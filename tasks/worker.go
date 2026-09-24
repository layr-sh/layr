// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

const (
	workerIdlePollDuration     = 250 * time.Millisecond
	workerErrorBackoffDuration = 200 * time.Millisecond
	workerBusyBackoffDuration  = 50 * time.Millisecond
)

// ErrNoPendingExecutions indicates no execution is due to be dequeued.
var ErrNoPendingExecutions = errors.New("no pending executions")

// WorkerQueue consumes tasks.executions using row-level locking and executes them with concurrency limits.
type WorkerQueue struct {
	kernel             *core.Kernel
	configManager      *ConfigManager
	dispatcher         *Dispatcher
	stopChannel        chan struct{}
	stoppedChannel     chan struct{}
	syncWaitGroup      sync.WaitGroup
	syncMutex          sync.Mutex
	activeWorkersInt32 atomic.Int32
	isRunning          bool
	nodeID             string
}

// NewWorkerQueue initializes a new WorkerQueue.
func NewWorkerQueue(kernel *core.Kernel, configManager *ConfigManager, dispatcher *Dispatcher) *WorkerQueue {
	hostname, _ := os.Hostname()

	return &WorkerQueue{
		kernel:         kernel,
		configManager:  configManager,
		dispatcher:     dispatcher,
		stopChannel:    make(chan struct{}),
		stoppedChannel: make(chan struct{}),
		nodeID:         hostname,
	}
}

// SetNodeID overrides the worker node identifier (useful for multi-node tests).
func (workerQueue *WorkerQueue) SetNodeID(nodeID string) {
	workerQueue.syncMutex.Lock()
	defer workerQueue.syncMutex.Unlock()
	workerQueue.nodeID = nodeID
}

// Start launches the worker dequeue loop.
func (workerQueue *WorkerQueue) Start(ctx context.Context) {
	workerQueue.syncMutex.Lock()
	if workerQueue.isRunning {
		workerQueue.syncMutex.Unlock()
		return
	}
	workerQueue.isRunning = true
	workerQueue.stopChannel = make(chan struct{})
	workerQueue.stoppedChannel = make(chan struct{})
	workerQueue.syncMutex.Unlock()

	go workerQueue.workerLoop(ctx)
}

// Stop terminates worker processing gracefully, waiting for in-flight tasks to conclude.
func (workerQueue *WorkerQueue) Stop() {
	workerQueue.syncMutex.Lock()
	if !workerQueue.isRunning {
		workerQueue.syncMutex.Unlock()
		return
	}
	workerQueue.isRunning = false
	close(workerQueue.stopChannel)
	workerQueue.syncMutex.Unlock()

	<-workerQueue.stoppedChannel
	workerQueue.syncWaitGroup.Wait()
}

func (workerQueue *WorkerQueue) workerLoop(ctx context.Context) {
	defer close(workerQueue.stoppedChannel)
	log.Debug("tasks worker processing loop started")

	for {
		select {
		case <-workerQueue.stopChannel:
			log.Debug("tasks worker loop stopping")
			return
		case <-ctx.Done():
			log.Debug("tasks worker context canceled")
			return
		default:
		}

		tasksConfig := workerQueue.configManager.Get()
		concurrencyLimit := tasksConfig.ConcurrencyLimit
		if concurrencyLimit < 1 {
			concurrencyLimit = defaultConcurrencyLimit
		}

		if workerQueue.activeWorkersInt32.Load() >= int32(concurrencyLimit) {
			if !sleepWithContext(ctx, workerBusyBackoffDuration) {
				return
			}
			continue
		}

		execution, err := workerQueue.dequeueNextExecution(ctx)
		if err != nil {
			if errors.Is(err, ErrNoPendingExecutions) {
				// No pending executions due right now
				if !sleepWithContext(ctx, workerIdlePollDuration) {
					return
				}
				continue
			}
			if !errors.Is(err, context.Canceled) {
				log.Errorf("error dequeuing next execution: %v", err)
			}
			if !sleepWithContext(ctx, workerErrorBackoffDuration) {
				return
			}
			continue
		}

		workerQueue.activeWorkersInt32.Add(1)
		workerQueue.syncWaitGroup.Add(1)
		go func(targetExecution *Execution) {
			defer func() {
				workerQueue.activeWorkersInt32.Add(-1)
				workerQueue.syncWaitGroup.Done()
			}()
			workerQueue.processExecution(ctx, targetExecution)
		}(execution)
	}
}

func (workerQueue *WorkerQueue) dequeueNextExecution(ctx context.Context) (*Execution, error) {
	workerQueue.syncMutex.Lock()
	currentNodeID := workerQueue.nodeID
	workerQueue.syncMutex.Unlock()

	query := `
		WITH next_task AS (
			SELECT id FROM tasks.executions
			WHERE status = 'pending' AND run_at <= clock_timestamp()
			ORDER BY run_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE tasks.executions
		SET status = 'running', locked_at = clock_timestamp(), locked_by = $1
		WHERE id = (SELECT id FROM next_task)
		RETURNING id, job_id, status, run_at, payload, attempts, max_attempts, locked_at, locked_by, last_error, created_at
	`

	var execution Execution
	err := workerQueue.kernel.DB().QueryRow(ctx, query, currentNodeID).Scan(
		&execution.ID, &execution.JobID, &execution.Status, &execution.RunAt, &execution.Payload,
		&execution.Attempts, &execution.MaxAttempts, &execution.LockedAt, &execution.LockedBy,
		&execution.LastError, &execution.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoPendingExecutions
		}
		return nil, err
	}

	return &execution, nil
}

func (workerQueue *WorkerQueue) processExecution(ctx context.Context, execution *Execution) {
	start := time.Now()
	tasksConfig := workerQueue.configManager.Get()
	timeoutSeconds := tasksConfig.TimeoutSeconds

	var jobName string
	targetType := TargetTypeHTTP

	if execution.JobID != nil {
		_ = workerQueue.kernel.DB().QueryRow(ctx, `
			SELECT name, target_type FROM tasks.jobs WHERE id = $1
		`, *execution.JobID).Scan(&jobName, &targetType)
	}

	// Detect target type from payload if not bound to a job
	if execution.JobID == nil {
		var probeMap map[string]any
		if err := json.Unmarshal(execution.Payload, &probeMap); err == nil {
			if _, hasQuery := probeMap["query"]; hasQuery {
				targetType = TargetTypeSQL
			}
		}
	}

	workerQueue.kernel.EventBus().Publish(ctx, NewExecutionStartedEvent(execution.ID.String(), ExecutionStartedEventData{
		ExecutionID: execution.ID,
		JobID:       execution.JobID,
		JobName:     jobName,
		Attempt:     execution.Attempts + 1,
		LockedBy:    workerQueue.nodeID,
	}))
	log.Tracef("starting execution %s for job %s (attempt %d)", execution.ID, jobName, execution.Attempts+1)

	var responseStatusCode *int
	var responseBody *string
	var execErr error
	var isPermanentClientError bool

	if targetType == TargetTypeSQL {
		var sqlPayload SQLPayload
		if err := json.Unmarshal(execution.Payload, &sqlPayload); err != nil {
			execErr = fmt.Errorf("invalid sql payload: %w", err)
			isPermanentClientError = true
		} else {
			_, resultText, isTransient, err := workerQueue.dispatcher.ExecuteSQLAttempt(ctx, sqlPayload.Query, sqlPayload.Params, timeoutSeconds)
			responseBody = resultText
			execErr = err
			if err != nil && !isTransient {
				isPermanentClientError = true
			}
		}
	} else {
		var httpPayload HTTPPayload
		if err := json.Unmarshal(execution.Payload, &httpPayload); err != nil {
			execErr = fmt.Errorf("invalid http payload: %w", err)
			isPermanentClientError = true
		} else {
			status, bodyText, isClientErr, err := workerQueue.dispatcher.ExecuteHTTPAttempt(ctx, httpPayload.URL, httpPayload.Headers, httpPayload.Body, execution.JobID, execution.ID, timeoutSeconds)
			responseStatusCode = status
			responseBody = bodyText
			isPermanentClientError = isClientErr
			execErr = err
			if err == nil && status != nil && (*status < 200 || *status >= 300) {
				execErr = fmt.Errorf("http error: status %d", *status)
			}
		}
	}

	durationMs := time.Since(start).Milliseconds()
	historyCtx := context.WithoutCancel(ctx)

	if execErr == nil {
		// --- Success ---
		_, _ = workerQueue.kernel.DB().Exec(historyCtx, `
			UPDATE tasks.executions SET
				status = 'completed',
				locked_at = NULL,
				locked_by = NULL
			WHERE id = $1
		`, execution.ID)

		_, _ = workerQueue.kernel.DB().Exec(historyCtx, `
			INSERT INTO tasks.execution_logs (
				job_id, execution_id, status, response_status_code, execution_duration_ms, response_body, executed_at
			) VALUES (
				$1, $2, 'success', $3, $4, $5, clock_timestamp()
			)
		`, execution.JobID, execution.ID, responseStatusCode, durationMs, responseBody)

		workerQueue.kernel.EventBus().Publish(ctx, NewExecutionCompletedEvent(execution.ID.String(), ExecutionCompletedEventData{
			ExecutionID:        execution.ID,
			JobID:              execution.JobID,
			JobName:            jobName,
			DurationMs:         durationMs,
			ResponseStatusCode: responseStatusCode,
			Attempt:            execution.Attempts + 1,
		}))
		log.Debugf("execution %s completed successfully in %dms", execution.ID, durationMs)
	} else {
		// --- Failure ---
		errMsg := execErr.Error()
		logStatus := StatusFailed
		if errors.Is(execErr, context.DeadlineExceeded) || strings.Contains(strings.ToLower(errMsg), "deadline exceeded") {
			logStatus = StatusTimeout
			workerQueue.kernel.EventBus().Publish(ctx, NewExecutionTimedOutEvent(execution.ID.String(), ExecutionTimedOutEventData{
				ExecutionID:         execution.ID,
				JobID:               execution.JobID,
				JobName:             jobName,
				DurationMs:          durationMs,
				TimeoutLimitSeconds: timeoutSeconds,
				Attempt:             execution.Attempts + 1,
			}))
			log.Warnf("execution %s timed out after %ds", execution.ID, timeoutSeconds)
		} else {
			workerQueue.kernel.EventBus().Publish(ctx, NewExecutionFailedEvent(execution.ID.String(), ExecutionFailedEventData{
				ExecutionID:        execution.ID,
				JobID:              execution.JobID,
				JobName:            jobName,
				DurationMs:         durationMs,
				ResponseStatusCode: responseStatusCode,
				Attempt:            execution.Attempts + 1,
				Error:              errMsg,
			}))
		}

		_, _ = workerQueue.kernel.DB().Exec(historyCtx, `
			INSERT INTO tasks.execution_logs (
				job_id, execution_id, status, response_status_code, execution_duration_ms, response_body, error_message, executed_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, clock_timestamp()
			)
		`, execution.JobID, execution.ID, logStatus, responseStatusCode, durationMs, responseBody, errMsg)

		nextAttempt := execution.Attempts + 1
		// Check if we should retry or isolate in DLQ
		if !isPermanentClientError && nextAttempt < execution.MaxAttempts {
			delay := CalculateTaskBackoff(nextAttempt, tasksConfig.RetryInitialDelaySeconds, tasksConfig.RetryMaxDelaySeconds)
			nextRunAt := time.Now().Add(delay)

			_, _ = workerQueue.kernel.DB().Exec(historyCtx, `
				UPDATE tasks.executions SET
					status = 'pending',
					attempts = $1,
					run_at = $2,
					locked_at = NULL,
					locked_by = NULL,
					last_error = $3
				WHERE id = $4
			`, nextAttempt, nextRunAt, errMsg, execution.ID)

			workerQueue.kernel.EventBus().Publish(ctx, NewExecutionRetriedEvent(execution.ID.String(), ExecutionRetriedEventData{
				ExecutionID:    execution.ID,
				JobID:          execution.JobID,
				JobName:        jobName,
				Attempt:        nextAttempt,
				MaxAttempts:    execution.MaxAttempts,
				NextRunAt:      nextRunAt,
				BackoffDelayMs: delay.Milliseconds(),
				Error:          errMsg,
			}))
			log.Warnf("execution %s failed: %v (retrying in %ds)", execution.ID, execErr, int(delay.Seconds()))
		} else {
			// Move to Dead-Letter Queue
			_, _ = workerQueue.kernel.DB().Exec(historyCtx, `
				UPDATE tasks.executions SET
					status = 'failed',
					attempts = $1,
					locked_at = NULL,
					locked_by = NULL,
					last_error = $2
				WHERE id = $3
			`, nextAttempt, errMsg, execution.ID)

			workerQueue.kernel.EventBus().Publish(ctx, NewDLQIsolatedEvent(execution.ID.String(), DLQIsolatedEventData{
				ExecutionID:   execution.ID,
				JobID:         execution.JobID,
				JobName:       jobName,
				TotalAttempts: nextAttempt,
				LastError:     errMsg,
			}))
			log.Errorf("execution %s failed permanently: %v (moved to DLQ)", execution.ID, execErr)
		}
	}
}
