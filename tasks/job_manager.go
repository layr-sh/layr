// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

// Standard job and execution errors.
var (
	ErrJobNotFound       = errors.New("job not found")
	ErrJobAlreadyExists  = errors.New("job with this name already exists")
	ErrExecutionNotFound = errors.New("execution not found")
	ErrDLQItemNotFound   = errors.New("dead-letter queue item not found")
)

// JobManager coordinates job registration, execution queues, and DLQ inspection.
type JobManager struct {
	kernel        *core.Kernel
	configManager *ConfigManager
}

// NewJobManager initializes a new JobManager.
func NewJobManager(kernel *core.Kernel, configManager *ConfigManager) *JobManager {
	return &JobManager{
		kernel:        kernel,
		configManager: configManager,
	}
}

// CreateJob validates and registers a new recurring cron job into tasks.jobs.
func (jobManager *JobManager) CreateJob(ctx context.Context, createJobInput CreateJobInput) (*Job, error) {
	name := strings.TrimSpace(createJobInput.Name)
	if name == "" {
		return nil, errors.New("job name is required")
	}

	cronExpr := strings.TrimSpace(createJobInput.CronExpression)
	cronSchedule, err := ParseCron(cronExpr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}

	timezone := strings.TrimSpace(createJobInput.Timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	targetType := strings.ToLower(strings.TrimSpace(createJobInput.TargetType))
	if targetType != TargetTypeHTTP && targetType != TargetTypeSQL {
		return nil, fmt.Errorf("invalid target_type: %s (must be 'http' or 'sql')", createJobInput.TargetType)
	}

	if validateErr := validateTargetPayload(targetType, createJobInput.TargetPayload); validateErr != nil {
		return nil, validateErr
	}

	isEnabled := true
	if createJobInput.IsEnabled != nil {
		isEnabled = *createJobInput.IsEnabled
	}

	now := time.Now()
	nextRunAt := cronSchedule.Next(now, location)

	var job Job
	err = jobManager.kernel.DB().QueryRow(ctx, `
		INSERT INTO tasks.jobs (
			name, cron_expression, timezone, target_type, target_payload, is_enabled, next_run_at, created_at, last_updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, clock_timestamp(), clock_timestamp()
		)
		RETURNING id, name, cron_expression, timezone, target_type, target_payload, is_enabled, last_run_at, next_run_at, created_at, last_updated_at
	`, name, cronExpr, timezone, targetType, createJobInput.TargetPayload, isEnabled, nextRunAt).Scan(
		&job.ID, &job.Name, &job.CronExpression, &job.Timezone, &job.TargetType, &job.TargetPayload,
		&job.IsEnabled, &job.LastRunAt, &job.NextRunAt, &job.CreatedAt, &job.LastUpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return nil, ErrJobAlreadyExists
		}
		return nil, fmt.Errorf("failed to insert job: %w", err)
	}

	jobManager.kernel.EventBus().Publish(ctx, NewJobCreatedEvent(job.ID.String(), JobCreatedEventData(job)))
	return &job, nil
}

// GetJob retrieves a cron job with its live countdown to next run.
func (jobManager *JobManager) GetJob(ctx context.Context, jobID uuid.UUID) (*GetJobResponse, error) {
	var job Job
	err := jobManager.kernel.DB().QueryRow(ctx, `
		SELECT id, name, cron_expression, timezone, target_type, target_payload, is_enabled, last_run_at, next_run_at, created_at, last_updated_at
		FROM tasks.jobs
		WHERE id = $1
	`, jobID).Scan(
		&job.ID, &job.Name, &job.CronExpression, &job.Timezone, &job.TargetType, &job.TargetPayload,
		&job.IsEnabled, &job.LastRunAt, &job.NextRunAt, &job.CreatedAt, &job.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to query job: %w", err)
	}

	countdown := int64(time.Until(job.NextRunAt).Seconds())
	if countdown < 0 {
		countdown = 0
	}

	return &GetJobResponse{
		Job:                     job,
		NextRunCountdownSeconds: countdown,
	}, nil
}

// ListJobs returns all configured recurring jobs.
func (jobManager *JobManager) ListJobs(ctx context.Context) ([]GetJobResponse, error) {
	rows, err := jobManager.kernel.DB().Query(ctx, `
		SELECT id, name, cron_expression, timezone, target_type, target_payload, is_enabled, last_run_at, next_run_at, created_at, last_updated_at
		FROM tasks.jobs
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []GetJobResponse
	for rows.Next() {
		var job Job
		_ = rows.Scan(
			&job.ID, &job.Name, &job.CronExpression, &job.Timezone, &job.TargetType, &job.TargetPayload,
			&job.IsEnabled, &job.LastRunAt, &job.NextRunAt, &job.CreatedAt, &job.LastUpdatedAt,
		)

		countdown := int64(time.Until(job.NextRunAt).Seconds())
		if countdown < 0 {
			countdown = 0
		}

		jobs = append(jobs, GetJobResponse{
			Job:                     job,
			NextRunCountdownSeconds: countdown,
		})
	}

	return jobs, nil
}

// UpdateJob updates an existing job's schedule, targets, or active state.
func (jobManager *JobManager) UpdateJob(ctx context.Context, jobID uuid.UUID, updateJobInput UpdateJobInput) (*Job, error) {
	getJobResponse, err := jobManager.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	name := getJobResponse.Name
	if updateJobInput.Name != nil && strings.TrimSpace(*updateJobInput.Name) != "" {
		name = strings.TrimSpace(*updateJobInput.Name)
	}

	cronExpr := getJobResponse.CronExpression
	if updateJobInput.CronExpression != nil && strings.TrimSpace(*updateJobInput.CronExpression) != "" {
		cronExpr = strings.TrimSpace(*updateJobInput.CronExpression)
	}

	timezone := getJobResponse.Timezone
	if updateJobInput.Timezone != nil && strings.TrimSpace(*updateJobInput.Timezone) != "" {
		timezone = strings.TrimSpace(*updateJobInput.Timezone)
	}

	cronSchedule, err := ParseCron(cronExpr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	targetType := getJobResponse.TargetType
	if updateJobInput.TargetType != nil && strings.TrimSpace(*updateJobInput.TargetType) != "" {
		targetType = strings.ToLower(strings.TrimSpace(*updateJobInput.TargetType))
		if targetType != TargetTypeHTTP && targetType != TargetTypeSQL {
			return nil, fmt.Errorf("invalid target_type: %s", *updateJobInput.TargetType)
		}
	}

	targetPayloadValue := getJobResponse.TargetPayload
	if updateJobInput.TargetPayload != nil && len(*updateJobInput.TargetPayload) > 0 {
		targetPayloadValue = *updateJobInput.TargetPayload
	}

	if validateErr := validateTargetPayload(targetType, targetPayloadValue); validateErr != nil {
		return nil, validateErr
	}

	isEnabled := getJobResponse.IsEnabled
	stateChanged := false
	if updateJobInput.IsEnabled != nil && *updateJobInput.IsEnabled != isEnabled {
		isEnabled = *updateJobInput.IsEnabled
		stateChanged = true
	}

	nextRunAt := cronSchedule.Next(time.Now(), location)

	var job Job
	err = jobManager.kernel.DB().QueryRow(ctx, `
		UPDATE tasks.jobs SET
			name = $1,
			cron_expression = $2,
			timezone = $3,
			target_type = $4,
			target_payload = $5,
			is_enabled = $6,
			next_run_at = $7,
			last_updated_at = clock_timestamp()
		WHERE id = $8
		RETURNING id, name, cron_expression, timezone, target_type, target_payload, is_enabled, last_run_at, next_run_at, created_at, last_updated_at
	`, name, cronExpr, timezone, targetType, targetPayloadValue, isEnabled, nextRunAt, jobID).Scan(
		&job.ID, &job.Name, &job.CronExpression, &job.Timezone, &job.TargetType, &job.TargetPayload,
		&job.IsEnabled, &job.LastRunAt, &job.NextRunAt, &job.CreatedAt, &job.LastUpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return nil, ErrJobAlreadyExists
		}
		return nil, fmt.Errorf("failed to update job: %w", err)
	}

	jobManager.kernel.EventBus().Publish(ctx, NewJobUpdatedEvent(job.ID.String(), JobUpdatedEventData(job)))

	if stateChanged {
		if isEnabled {
			jobManager.kernel.EventBus().Publish(ctx, NewJobResumedEvent(job.ID.String(), JobResumedEventData{
				JobID:     job.ID,
				JobName:   job.Name,
				NextRunAt: job.NextRunAt,
			}))
		} else {
			jobManager.kernel.EventBus().Publish(ctx, NewJobPausedEvent(job.ID.String(), JobPausedEventData{
				JobID:   job.ID,
				JobName: job.Name,
			}))
		}
	}

	return &job, nil
}

// DeleteJob removes a job and cancels all associated pending executions.
func (jobManager *JobManager) DeleteJob(ctx context.Context, jobID uuid.UUID) error {
	getJobResponse, err := jobManager.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	// Delete pending executions
	execResult, err := jobManager.kernel.DB().Exec(ctx, `
		DELETE FROM tasks.executions
		WHERE job_id = $1 AND status = 'pending'
	`, jobID)
	if err != nil {
		return fmt.Errorf("failed to cancel pending executions: %w", err)
	}
	deletedExecutionsCount := int(execResult.RowsAffected())

	// Delete job
	_, err = jobManager.kernel.DB().Exec(ctx, `DELETE FROM tasks.jobs WHERE id = $1`, jobID)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	jobManager.kernel.EventBus().Publish(ctx, NewJobDeletedEvent(jobID.String(), JobDeletedEventData{
		JobID:                  jobID,
		JobName:                getJobResponse.Name,
		DeletedExecutionsCount: deletedExecutionsCount,
	}))
	return nil
}

// TriggerExecution manually dispatches an immediate execution out-of-schedule.
func (jobManager *JobManager) TriggerExecution(ctx context.Context, triggerExecutionInput TriggerExecutionInput) (*TriggerExecutionResponse, error) {
	var targetPayloadValue json.RawMessage
	var jobName string
	targetType := TargetTypeHTTP

	if triggerExecutionInput.JobID != nil {
		getJobResponse, err := jobManager.GetJob(ctx, *triggerExecutionInput.JobID)
		if err != nil {
			return nil, err
		}
		targetPayloadValue = getJobResponse.TargetPayload
		jobName = getJobResponse.Name
		targetType = getJobResponse.TargetType
	}

	if triggerExecutionInput.Payload != nil && len(*triggerExecutionInput.Payload) > 0 {
		targetPayloadValue = *triggerExecutionInput.Payload
	}

	if len(targetPayloadValue) == 0 {
		targetPayloadValue = json.RawMessage("{}")
	}

	tasksConfig := jobManager.configManager.Get()
	maxAttempts := tasksConfig.RetryMaxAttempts

	var triggerExecutionResponse TriggerExecutionResponse
	triggerExecutionResponse.Status = StatusPending
	err := jobManager.kernel.DB().QueryRow(ctx, `
		INSERT INTO tasks.executions (
			job_id, status, run_at, payload, attempts, max_attempts, created_at
		) VALUES (
			$1, 'pending', clock_timestamp(), $2, 0, $3, clock_timestamp()
		)
		RETURNING id, run_at
	`, triggerExecutionInput.JobID, targetPayloadValue, maxAttempts).Scan(&triggerExecutionResponse.ExecutionID, &triggerExecutionResponse.RunAt)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue immediate execution: %w", err)
	}

	jobManager.kernel.EventBus().Publish(ctx, NewExecutionEnqueuedEvent(triggerExecutionResponse.ExecutionID.String(), ExecutionEnqueuedEventData{
		ExecutionID: triggerExecutionResponse.ExecutionID,
		JobID:       triggerExecutionInput.JobID,
		JobName:     jobName,
		RunAt:       triggerExecutionResponse.RunAt,
		TargetType:  targetType,
		IsImmediate: true,
	}))

	return &triggerExecutionResponse, nil
}

// ListExecutions queries execution history logs with optional job filtering and pagination.
func (jobManager *JobManager) ListExecutions(ctx context.Context, jobID *uuid.UUID, limit, offset int) ([]ExecutionLog, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT
			(SELECT count(*) FROM tasks.execution_logs WHERE ($1::uuid IS NULL OR job_id = $1)) AS total_count,
			COALESCE(
				json_agg(
					json_build_object(
						'id', id,
						'job_id', job_id,
						'execution_id', execution_id,
						'status', status,
						'response_status_code', response_status_code,
						'execution_duration_ms', execution_duration_ms,
						'response_body', response_body,
						'error_message', error_message,
						'executed_at', executed_at
					) ORDER BY executed_at DESC
				) FILTER (WHERE id IS NOT NULL),
				'[]'::json
			) AS logs
		FROM (
			SELECT id, job_id, execution_id, status, response_status_code, execution_duration_ms, response_body, error_message, executed_at
			FROM tasks.execution_logs
			WHERE ($1::uuid IS NULL OR job_id = $1)
			ORDER BY executed_at DESC
			LIMIT $2 OFFSET $3
		) sub
	`

	var totalCount int
	var rawJSON []byte
	if err := jobManager.kernel.DB().QueryRow(ctx, query, jobID, limit, offset).Scan(&totalCount, &rawJSON); err != nil {
		return nil, 0, fmt.Errorf("failed to query execution logs: %w", err)
	}

	logs := []ExecutionLog{}
	_ = json.Unmarshal(rawJSON, &logs)

	return logs, totalCount, nil
}

// ListDLQ returns failed executions isolated in the dead-letter queue.
func (jobManager *JobManager) ListDLQ(ctx context.Context, limit, offset int) ([]DLQExecution, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT
			(SELECT count(*) FROM tasks.executions WHERE status = 'failed') AS total_count,
			COALESCE(
				json_agg(
					json_build_object(
						'id', id,
						'job_id', job_id,
						'status', status,
						'run_at', run_at,
						'payload', payload,
						'attempts', attempts,
						'max_attempts', max_attempts,
						'last_error', last_error,
						'created_at', created_at
					) ORDER BY created_at DESC
				) FILTER (WHERE id IS NOT NULL),
				'[]'::json
			) AS items
		FROM (
			SELECT id, job_id, status, run_at, payload, attempts, max_attempts, last_error, created_at
			FROM tasks.executions
			WHERE status = 'failed'
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		) sub
	`

	var totalCount int
	var rawJSON []byte
	if err := jobManager.kernel.DB().QueryRow(ctx, query, limit, offset).Scan(&totalCount, &rawJSON); err != nil {
		return nil, 0, fmt.Errorf("failed to query dlq items: %w", err)
	}

	items := []DLQExecution{}
	_ = json.Unmarshal(rawJSON, &items)

	return items, totalCount, nil
}

// RetryDLQExecution resets a failed execution in the DLQ to pending for immediate re-attempt.
func (jobManager *JobManager) RetryDLQExecution(ctx context.Context, executionID uuid.UUID) (*RetryDLQResponse, error) {
	var jobID *uuid.UUID
	var payloadValue json.RawMessage
	err := jobManager.kernel.DB().QueryRow(ctx, `
		SELECT job_id, payload FROM tasks.executions WHERE id = $1 AND status = 'failed'
	`, executionID).Scan(&jobID, &payloadValue)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDLQItemNotFound
		}
		return nil, fmt.Errorf("failed to check dlq item: %w", err)
	}

	_, err = jobManager.kernel.DB().Exec(ctx, `
		UPDATE tasks.executions SET
			status = 'pending',
			run_at = clock_timestamp(),
			attempts = 0,
			last_error = NULL,
			locked_at = NULL,
			locked_by = NULL
		WHERE id = $1
	`, executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to re-queue dlq item: %w", err)
	}

	jobManager.kernel.EventBus().Publish(ctx, NewDLQRetriedEvent(executionID.String(), DLQRetriedEventData{
		ExecutionID: executionID,
		JobID:       jobID,
		TriggeredBy: "control_plane",
	}))

	jobManager.kernel.EventBus().Publish(ctx, NewExecutionEnqueuedEvent(executionID.String(), ExecutionEnqueuedEventData{
		ExecutionID: executionID,
		JobID:       jobID,
		RunAt:       time.Now().UTC(),
		IsImmediate: true,
	}))

	return &RetryDLQResponse{
		ExecutionID: executionID,
		Status:      StatusPending,
		Message:     "Execution successfully re-queued from DLQ",
	}, nil
}

// PurgeDLQExecution removes an execution permanently from the DLQ.
func (jobManager *JobManager) PurgeDLQExecution(ctx context.Context, executionID uuid.UUID) error {
	var jobID *uuid.UUID
	err := jobManager.kernel.DB().QueryRow(ctx, `
		SELECT job_id FROM tasks.executions WHERE id = $1 AND status = 'failed'
	`, executionID).Scan(&jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return ErrDLQItemNotFound
		}
		return fmt.Errorf("failed to find dlq item: %w", err)
	}

	_, err = jobManager.kernel.DB().Exec(ctx, "DELETE FROM tasks.executions WHERE id = $1 AND status = 'failed'", executionID)
	if err != nil {
		return fmt.Errorf("failed to purge dlq item: %w", err)
	}

	jobManager.kernel.EventBus().Publish(ctx, NewDLQPurgedEvent(executionID.String(), DLQPurgedEventData{
		ExecutionID: executionID,
		JobID:       jobID,
	}))
	return nil
}

// GetStats returns telemetry metrics across all jobs and executions.
func (jobManager *JobManager) GetStats(ctx context.Context) (*GetStatsResponse, error) {
	var getStatsResponse GetStatsResponse

	// Jobs stats
	if err := jobManager.kernel.DB().QueryRow(ctx, `
		SELECT
			count(*),
			count(*) FILTER (WHERE is_enabled = true),
			count(*) FILTER (WHERE is_enabled = false)
		FROM tasks.jobs
	`).Scan(&getStatsResponse.TotalJobs, &getStatsResponse.ActiveJobs, &getStatsResponse.PausedJobs); err != nil {
		return nil, fmt.Errorf("failed to query jobs stats: %w", err)
	}

	// Executions stats
	if err := jobManager.kernel.DB().QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status = 'pending'),
			count(*) FILTER (WHERE status = 'running'),
			count(*) FILTER (WHERE status = 'completed'),
			count(*) FILTER (WHERE status = 'failed')
		FROM tasks.executions
	`).Scan(&getStatsResponse.PendingExecutions, &getStatsResponse.RunningExecutions, &getStatsResponse.CompletedExecutions, &getStatsResponse.FailedExecutions); err != nil {
		return nil, fmt.Errorf("failed to query executions stats: %w", err)
	}

	totalFinished := getStatsResponse.CompletedExecutions + getStatsResponse.FailedExecutions
	if totalFinished > 0 {
		getStatsResponse.SuccessRate = float64(getStatsResponse.CompletedExecutions) / float64(totalFinished) * 100.0
	} else {
		getStatsResponse.SuccessRate = 100.0
	}

	return &getStatsResponse, nil
}

func validateTargetPayload(targetType string, payloadValue json.RawMessage) error {
	if targetType != TargetTypeHTTP && targetType != TargetTypeSQL {
		return fmt.Errorf("unsupported target_type: %s", targetType)
	}

	if len(payloadValue) == 0 || string(payloadValue) == "{}" {
		return fmt.Errorf("target_payload is required for target_type '%s'", targetType)
	}

	switch targetType {
	case TargetTypeHTTP:
		var httpPayload HTTPPayload
		if err := json.Unmarshal(payloadValue, &httpPayload); err != nil {
			return fmt.Errorf("invalid http target_payload: %w", err)
		}
		if strings.TrimSpace(httpPayload.URL) == "" {
			return errors.New("url is required in target_payload for http target")
		}
		targetParsedURL, err := url.Parse(httpPayload.URL)
		if err != nil || (targetParsedURL.Scheme != "http" && targetParsedURL.Scheme != "https") {
			return errors.New("valid http or https url is required in target_payload")
		}
	case TargetTypeSQL:
		var sqlPayload SQLPayload
		if err := json.Unmarshal(payloadValue, &sqlPayload); err != nil {
			return fmt.Errorf("invalid sql target_payload: %w", err)
		}
		if strings.TrimSpace(sqlPayload.Query) == "" {
			return errors.New("query is required in target_payload for sql target")
		}
	}
	return nil
}
