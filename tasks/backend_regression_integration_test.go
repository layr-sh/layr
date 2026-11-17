package tasks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

// Query tracing injects real PostgreSQL failures at exact transaction boundaries
// without changing production database interfaces or function signatures.
type tasksFaultQueryTracer struct {
	before func(context.Context, *pgx.Conn, string) context.Context
}

func (tracer *tasksFaultQueryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, traceQueryStartData pgx.TraceQueryStartData) context.Context {
	return tracer.before(ctx, conn, strings.TrimSpace(traceQueryStartData.SQL))
}
func (*tasksFaultQueryTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestTasksPersistenceFailuresIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	ctx := context.Background()
	newFaultKernel := func(t *testing.T, before func(context.Context, *pgx.Conn, string) context.Context) *core.Kernel {
		t.Helper()
		databaseConfig := kernel.DB().Config()
		databaseConfig.ConnConfig.Tracer = &tasksFaultQueryTracer{before: before}
		pool, err := pgxpool.NewWithConfig(ctx, databaseConfig)
		require.NoError(t, err)
		t.Cleanup(pool.Close)
		return core.NewTestKernel(&core.DatabasePool{Pool: pool})
	}
	jobManager := NewJobManager(kernel, NewConfigManager(kernel))
	for _, testCase := range []struct{ name, query string }{
		{"begin deletion", "begin"},
		{"lock job", "SELECT id FROM tasks.jobs WHERE id = $1 FOR UPDATE"},
		{"delete pending", "DELETE FROM tasks.executions"},
		{"delete job", "DELETE FROM tasks.jobs"},
		{"commit deletion", "commit"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			job, err := jobManager.CreateJob(ctx, CreateJobInput{Name: uuid.NewV7().String(), CronExpression: "0 0 * * *", TargetType: TargetTypeSQL, TargetPayload: json.RawMessage(`{"query":"SELECT 1"}`)})
			require.NoError(t, err)
			triggerExecutionResponse, err := jobManager.TriggerExecution(ctx, TriggerExecutionInput{JobID: &job.ID})
			require.NoError(t, err)
			injected := false
			faultKernel := newFaultKernel(t, func(queryCtx context.Context, _ *pgx.Conn, query string) context.Context {
				if strings.HasPrefix(query, testCase.query) {
					injected = true
					canceledCtx, cancel := context.WithCancel(queryCtx)
					cancel()
					return canceledCtx
				}
				return queryCtx
			})
			faultJobManager := NewJobManager(faultKernel, NewConfigManager(faultKernel))
			require.Error(t, faultJobManager.DeleteJob(ctx, job.ID))
			require.True(t, injected)
			_, err = jobManager.GetJob(ctx, job.ID)
			require.NoError(t, err)
			var status string
			require.NoError(t, kernel.DB().QueryRow(ctx, `SELECT status FROM tasks.executions WHERE id = $1`, triggerExecutionResponse.ExecutionID).Scan(&status))
			require.Equal(t, StatusPending, status)
			require.NoError(t, jobManager.DeleteJob(ctx, job.ID))
		})
	}
	t.Run("job disappears before deletion lock", func(t *testing.T) {
		job, err := jobManager.CreateJob(ctx, CreateJobInput{Name: uuid.NewV7().String(), CronExpression: "0 0 * * *", TargetType: TargetTypeSQL, TargetPayload: json.RawMessage(`{"query":"SELECT 1"}`)})
		require.NoError(t, err)
		faultKernel := newFaultKernel(t, func(queryCtx context.Context, _ *pgx.Conn, query string) context.Context {
			if strings.HasPrefix(query, "SELECT id FROM tasks.jobs WHERE id = $1 FOR UPDATE") {
				_, deleteErr := kernel.DB().Exec(queryCtx, `DELETE FROM tasks.jobs WHERE id = $1`, job.ID)
				require.NoError(t, deleteErr)
			}
			return queryCtx
		})
		require.ErrorIs(t, NewJobManager(faultKernel, NewConfigManager(faultKernel)).DeleteJob(ctx, job.ID), ErrJobNotFound)
	})
	t.Run("concurrent retry cannot reset a running execution", func(t *testing.T) {
		executionID := uuid.NewV7()
		_, err := kernel.DB().Exec(ctx, `INSERT INTO tasks.executions (id, status, run_at, attempts) VALUES ($1, 'failed', clock_timestamp(), 3)`, executionID)
		require.NoError(t, err)
		injected := false
		faultKernel := newFaultKernel(t, func(queryCtx context.Context, _ *pgx.Conn, query string) context.Context {
			if strings.HasPrefix(query, "UPDATE tasks.executions SET") {
				injected = true
				_, updateErr := kernel.DB().Exec(queryCtx, `UPDATE tasks.executions SET status = 'running' WHERE id = $1`, executionID)
				require.NoError(t, updateErr)
			}
			return queryCtx
		})
		_, err = NewJobManager(faultKernel, NewConfigManager(faultKernel)).RetryDLQExecution(ctx, executionID)
		require.ErrorIs(t, err, ErrDLQItemNotFound)
		require.True(t, injected)
		var status string
		var attempts int
		require.NoError(t, kernel.DB().QueryRow(ctx, `SELECT status, attempts FROM tasks.executions WHERE id = $1`, executionID).Scan(&status, &attempts))
		require.Equal(t, StatusRunning, status)
		require.Equal(t, 3, attempts)
	})

	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	for _, testCase := range []struct {
		name, query, payload string
		maxAttempts          int
	}{
		{"begin worker result", "begin", `{"query":"SELECT 1"}`, 2},
		{"completed update", "UPDATE tasks.executions SET", `{"query":"SELECT 1"}`, 2},
		{"success audit insert", "INSERT INTO tasks.execution_logs", `{"query":"SELECT 1"}`, 2},
		{"failure audit insert", "INSERT INTO tasks.execution_logs", `{"query":"SELECT 1/0"}`, 1},
		{"retry update", "UPDATE tasks.executions SET", `{"url":"` + server.URL + `"}`, 2},
		{"dead letter update", "UPDATE tasks.executions SET", `{"query":"SELECT 1/0"}`, 1},
		{"commit worker result", "commit", `{"query":"SELECT 1"}`, 2},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			execution := Execution{ID: uuid.NewV7(), Status: StatusRunning, Payload: json.RawMessage(testCase.payload), MaxAttempts: testCase.maxAttempts}
			_, err := kernel.DB().Exec(ctx, `INSERT INTO tasks.executions (id, status, run_at, payload, max_attempts) VALUES ($1, 'running', clock_timestamp(), $2, $3)`, execution.ID, execution.Payload, execution.MaxAttempts)
			require.NoError(t, err)
			injected := false
			faultKernel := newFaultKernel(t, func(queryCtx context.Context, _ *pgx.Conn, query string) context.Context {
				if strings.HasPrefix(query, testCase.query) {
					injected = true
					canceledCtx, cancel := context.WithCancel(queryCtx)
					cancel()
					return canceledCtx
				}
				return queryCtx
			})
			var resultEventsInt32 atomic.Int32
			faultKernel.EventBus().Subscribe("tasks.*", func(_ context.Context, event core.Event) error {
				if event.Type != "tasks.execution.started" {
					resultEventsInt32.Add(1)
				}
				return nil
			})
			workerQueue := NewWorkerQueue(faultKernel, NewConfigManager(faultKernel), NewDispatcher(faultKernel))
			workerQueue.processExecution(ctx, &execution)
			faultKernel.EventBus().Close()
			require.Zero(t, resultEventsInt32.Load())
			require.True(t, injected)
			var status string
			var logCount int
			require.NoError(t, kernel.DB().QueryRow(ctx, `SELECT status FROM tasks.executions WHERE id = $1`, execution.ID).Scan(&status))
			require.NoError(t, kernel.DB().QueryRow(ctx, `SELECT count(*) FROM tasks.execution_logs WHERE execution_id = $1`, execution.ID).Scan(&logCount))
			require.Equal(t, StatusRunning, status)
			require.Zero(t, logCount)
		})
	}
	t.Run("scheduler query and empty commit failures are returned", func(t *testing.T) {
		for _, queryPrefix := range []string{"SELECT id, name", "commit"} {
			t.Run(queryPrefix, func(t *testing.T) {
				injected := false
				faultKernel := newFaultKernel(t, func(queryCtx context.Context, _ *pgx.Conn, query string) context.Context {
					if strings.HasPrefix(query, queryPrefix) {
						injected = true
						canceledCtx, cancel := context.WithCancel(queryCtx)
						cancel()
						return canceledCtx
					}
					return queryCtx
				})
				count, err := NewJobPoller(faultKernel, NewConfigManager(faultKernel)).PollOnce(ctx)
				require.Error(t, err)
				require.Zero(t, count)
				require.True(t, injected)
			})
		}
	})
}
