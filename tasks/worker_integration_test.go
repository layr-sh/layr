package tasks

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksWorkerIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(kernel)
	dispatcher := NewDispatcher(kernel)
	workerQueue := NewWorkerQueue(kernel, configManager, dispatcher)
	workerQueue.SetNodeID("integration-worker-node")

	jobManager := NewJobManager(kernel, configManager)

	t.Run("processes SQL task to completion and logs audit record", func(t *testing.T) {
		sqlPayload, _ := json.Marshal(map[string]any{"query": "SELECT 1"})
		job, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "worker-sql-job",
			CronExpression: "0 0 1 1 *",
			TargetType:     TargetTypeSQL,
			TargetPayload:  sqlPayload,
		})
		require.NoError(t, err)

		triggerExecutionResponse, err := jobManager.TriggerExecution(ctx, TriggerExecutionInput{JobID: &job.ID})
		require.NoError(t, err)

		execution, err := workerQueue.dequeueNextExecution(ctx)
		require.NoError(t, err)
		require.NotNil(t, execution)
		require.Equal(t, triggerExecutionResponse.ExecutionID, execution.ID)

		// Process execution directly
		workerQueue.processExecution(ctx, execution)

		// Verify execution is completed in DB
		var status string
		err = kernel.DB().QueryRow(ctx, "SELECT status FROM tasks.executions WHERE id = $1", execution.ID).Scan(&status)
		require.NoError(t, err)
		require.Equal(t, "completed", status)

		// Verify audit log exists
		var logCount int
		err = kernel.DB().QueryRow(ctx, "SELECT count(*) FROM tasks.execution_logs WHERE execution_id = $1", execution.ID).Scan(&logCount)
		require.NoError(t, err)
		require.Equal(t, 1, logCount)
	})

	t.Run("processes HTTP task to completion and logs audit record", func(t *testing.T) {
		var webhookInvokedInt32 atomic.Int32
		testServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			webhookInvokedInt32.Add(1)
			_, _ = io.ReadAll(request.Body)
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte(`{"status":"ok"}`))
		}))
		defer testServer.Close()

		httpPayload, _ := json.Marshal(map[string]any{
			"url":    testServer.URL,
			"method": "POST",
			"body":   map[string]string{"foo": "bar"},
		})

		job, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "worker-http-job",
			CronExpression: "0 0 1 1 *",
			TargetType:     TargetTypeHTTP,
			TargetPayload:  httpPayload,
		})
		require.NoError(t, err)

		triggerExecutionResponse, err := jobManager.TriggerExecution(ctx, TriggerExecutionInput{JobID: &job.ID})
		require.NoError(t, err)

		execution, err := workerQueue.dequeueNextExecution(ctx)
		require.NoError(t, err)
		require.NotNil(t, execution)
		require.Equal(t, triggerExecutionResponse.ExecutionID, execution.ID)

		workerQueue.processExecution(ctx, execution)

		var status string
		err = kernel.DB().QueryRow(ctx, "SELECT status FROM tasks.executions WHERE id = $1", execution.ID).Scan(&status)
		require.NoError(t, err)
		require.Equal(t, "completed", status)
		require.Equal(t, int32(1), webhookInvokedInt32.Load())
	})

	t.Run("client error transitions directly to DLQ", func(t *testing.T) {
		clientErrServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			http.Error(responseWriter, "bad request", http.StatusBadRequest)
		}))
		defer clientErrServer.Close()

		httpPayload, _ := json.Marshal(map[string]any{"url": clientErrServer.URL})
		job, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "worker-client-err-job",
			CronExpression: "0 0 1 1 *",
			TargetType:     TargetTypeHTTP,
			TargetPayload:  httpPayload,
		})
		require.NoError(t, err)

		triggerExecutionResponse, err := jobManager.TriggerExecution(ctx, TriggerExecutionInput{JobID: &job.ID})
		require.NoError(t, err)

		execution, err := workerQueue.dequeueNextExecution(ctx)
		require.NoError(t, err)

		workerQueue.processExecution(ctx, execution)

		var status string
		err = kernel.DB().QueryRow(ctx, "SELECT status FROM tasks.executions WHERE id = $1", triggerExecutionResponse.ExecutionID).Scan(&status)
		require.NoError(t, err)
		require.Equal(t, "failed", status)
	})

	t.Run("server error retries with backoff until max attempts reached", func(t *testing.T) {
		serverErrServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			http.Error(responseWriter, "server error", http.StatusInternalServerError)
		}))
		defer serverErrServer.Close()

		httpPayload, _ := json.Marshal(map[string]any{"url": serverErrServer.URL})
		job, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "worker-server-err-job",
			CronExpression: "0 0 1 1 *",
			TargetType:     TargetTypeHTTP,
			TargetPayload:  httpPayload,
		})
		require.NoError(t, err)

		triggerExecutionResponse, err := jobManager.TriggerExecution(ctx, TriggerExecutionInput{JobID: &job.ID})
		require.NoError(t, err)
		require.NotNil(t, triggerExecutionResponse)

		execution, err := workerQueue.dequeueNextExecution(ctx)
		require.NoError(t, err)

		// Set max_attempts to 2 to test transition on attempt 2
		_, err = kernel.DB().Exec(ctx, "UPDATE tasks.executions SET max_attempts = 2 WHERE id = $1", execution.ID)
		require.NoError(t, err)
		execution.MaxAttempts = 2

		// 1st attempt: should remain pending and advance run_at
		workerQueue.processExecution(ctx, execution)

		var status string
		var attempts int
		err = kernel.DB().QueryRow(ctx, "SELECT status, attempts FROM tasks.executions WHERE id = $1", execution.ID).Scan(&status, &attempts)
		require.NoError(t, err)
		require.Equal(t, "pending", status)
		require.Equal(t, 1, attempts)

		// Set run_at to past and dequeue for 2nd attempt
		_, err = kernel.DB().Exec(ctx, "UPDATE tasks.executions SET run_at = clock_timestamp() - interval '1 second' WHERE id = $1", execution.ID)
		require.NoError(t, err)

		retryExecution, err := workerQueue.dequeueNextExecution(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, retryExecution.Attempts)

		// 2nd attempt: attempts reaches max_attempts (2) -> transitions to DLQ ('failed')
		workerQueue.processExecution(ctx, retryExecution)

		err = kernel.DB().QueryRow(ctx, "SELECT status, attempts FROM tasks.executions WHERE id = $1", execution.ID).Scan(&status, &attempts)
		require.NoError(t, err)
		require.Equal(t, "failed", status)
		require.Equal(t, 2, attempts)
	})

	t.Run("timeout limit enforcement aborts and records timeout", func(t *testing.T) {
		timeoutServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			time.Sleep(2 * time.Second)
			responseWriter.WriteHeader(http.StatusOK)
		}))
		defer timeoutServer.Close()

		testConfig := DefaultConfig()
		testConfig.TimeoutSeconds = 1
		configManager.SetMemoryConfig(testConfig)

		httpPayload, _ := json.Marshal(map[string]any{"url": timeoutServer.URL})
		executionID := uuid.NewV7()
		_, err := kernel.DB().Exec(ctx, `
			INSERT INTO tasks.executions (
				id, payload, status, run_at, attempts, max_attempts, created_at
			) VALUES ($1, $2, 'pending', clock_timestamp(), 0, 1, clock_timestamp())
		`, executionID, httpPayload)
		require.NoError(t, err)

		execution, err := workerQueue.dequeueNextExecution(ctx)
		require.NoError(t, err)
		require.Equal(t, executionID, execution.ID)

		workerQueue.processExecution(ctx, execution)

		var status string
		var lastError *string
		err = kernel.DB().QueryRow(ctx, "SELECT status, last_error FROM tasks.executions WHERE id = $1", executionID).Scan(&status, &lastError)
		require.NoError(t, err)
		require.Equal(t, "failed", status)
		require.NotNil(t, lastError)
		require.Contains(t, *lastError, "deadline exceeded")
	})

	t.Run("dequeue returns ErrNoPendingExecutions when no tasks are due", func(t *testing.T) {
		_, err := workerQueue.dequeueNextExecution(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNoPendingExecutions)
	})

	t.Run("ad-hoc task with query in payload executes as SQL target", func(t *testing.T) {
		adhocExecutionID := uuid.NewV7()
		_, err := kernel.DB().Exec(ctx, `
			INSERT INTO tasks.executions (id, payload, status, run_at, attempts, max_attempts, created_at)
			VALUES ($1, '{"query":"SELECT 1"}', 'pending', clock_timestamp(), 0, 3, clock_timestamp())
		`, adhocExecutionID)
		require.NoError(t, err)

		execution, err := workerQueue.dequeueNextExecution(ctx)
		require.NoError(t, err)
		require.Equal(t, adhocExecutionID, execution.ID)

		workerQueue.processExecution(ctx, execution)

		var status string
		err = kernel.DB().QueryRow(ctx, "SELECT status FROM tasks.executions WHERE id = $1", adhocExecutionID).Scan(&status)
		require.NoError(t, err)
		require.Equal(t, "completed", status)
	})

	t.Run("SQL execution with syntax error triggers permanent error DLQ transition", func(t *testing.T) {
		sqlJob, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "sql-syntax-err-job",
			CronExpression: "0 0 1 1 *",
			TargetType:     TargetTypeSQL,
			TargetPayload:  []byte(`{"query":"SELECT 1"}`),
		})
		require.NoError(t, err)

		syntaxErrExecID := uuid.NewV7()
		_, err = kernel.DB().Exec(ctx, `
			INSERT INTO tasks.executions (id, job_id, payload, status, run_at, attempts, max_attempts, created_at)
			VALUES ($1, $2, '{"query":"SELECT syntax error from"}', 'pending', clock_timestamp(), 0, 5, clock_timestamp())
		`, syntaxErrExecID, sqlJob.ID)
		require.NoError(t, err)

		execution, err := workerQueue.dequeueNextExecution(ctx)
		require.NoError(t, err)
		require.Equal(t, syntaxErrExecID, execution.ID)

		workerQueue.processExecution(ctx, execution)

		var status string
		err = kernel.DB().QueryRow(ctx, "SELECT status FROM tasks.executions WHERE id = $1", syntaxErrExecID).Scan(&status)
		require.NoError(t, err)
		require.Equal(t, "failed", status)
	})

	t.Run("SQL execution with invalid json payload triggers permanent error", func(t *testing.T) {
		sqlJob, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "sql-bad-json-job",
			CronExpression: "0 0 1 1 *",
			TargetType:     TargetTypeSQL,
			TargetPayload:  []byte(`{"query":"SELECT 1"}`),
		})
		require.NoError(t, err)

		badJSONExecID := uuid.NewV7()
		_, err = kernel.DB().Exec(ctx, `
			INSERT INTO tasks.executions (id, job_id, payload, status, run_at, attempts, max_attempts, created_at)
			VALUES ($1, $2, '123', 'pending', clock_timestamp(), 0, 5, clock_timestamp())
		`, badJSONExecID, sqlJob.ID)
		require.NoError(t, err)

		execution, err := workerQueue.dequeueNextExecution(ctx)
		require.NoError(t, err)
		require.Equal(t, badJSONExecID, execution.ID)

		workerQueue.processExecution(ctx, execution)

		var status string
		err = kernel.DB().QueryRow(ctx, "SELECT status FROM tasks.executions WHERE id = $1", badJSONExecID).Scan(&status)
		require.NoError(t, err)
		require.Equal(t, "failed", status)
	})

	t.Run("HTTP execution with invalid json payload triggers permanent error", func(t *testing.T) {
		httpJob, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "http-bad-json-job",
			CronExpression: "0 0 1 1 *",
			TargetType:     TargetTypeHTTP,
			TargetPayload:  []byte(`{"url":"https://example.com"}`),
		})
		require.NoError(t, err)

		badHTTPJSONExecID := uuid.NewV7()
		_, err = kernel.DB().Exec(ctx, `
			INSERT INTO tasks.executions (id, job_id, payload, status, run_at, attempts, max_attempts, created_at)
			VALUES ($1, $2, '123', 'pending', clock_timestamp(), 0, 5, clock_timestamp())
		`, badHTTPJSONExecID, httpJob.ID)
		require.NoError(t, err)

		execution, err := workerQueue.dequeueNextExecution(ctx)
		require.NoError(t, err)
		require.Equal(t, badHTTPJSONExecID, execution.ID)

		workerQueue.processExecution(ctx, execution)

		var status string
		err = kernel.DB().QueryRow(ctx, "SELECT status FROM tasks.executions WHERE id = $1", badHTTPJSONExecID).Scan(&status)
		require.NoError(t, err)
		require.Equal(t, "failed", status)
	})

	t.Run("workerLoop handles concurrency limit backoff and context cancel", func(t *testing.T) {
		concurrencyWorkerQueue := NewWorkerQueue(kernel, configManager, dispatcher)
		concurrencyWorkerQueue.activeWorkersInt32.Store(1000)

		loopCtx, cancel := context.WithCancel(context.Background())
		concurrencyWorkerQueue.Start(loopCtx)
		time.Sleep(100 * time.Millisecond)
		cancel()
		concurrencyWorkerQueue.Stop()
	})

	t.Run("workerLoop idle sleep returns on context cancellation", func(t *testing.T) {
		idleWorkerQueue := NewWorkerQueue(kernel, configManager, dispatcher)
		loopCtx, cancel := context.WithCancel(context.Background())
		idleWorkerQueue.Start(loopCtx)
		time.Sleep(50 * time.Millisecond)
		cancel()
		idleWorkerQueue.Stop()
	})
}
