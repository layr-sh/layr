// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"encoding/json"
	"time"

	"uuid"
)

// Supported job target types.
const (
	TargetTypeHTTP = "http"
	TargetTypeSQL  = "sql"
)

// Supported execution statuses.
const (
	StatusPending    = "pending"
	StatusRunning    = "running"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusSuccess    = "success"
	StatusTimeout    = "timeout"
	StatusDeadLetter = "dead_letter"
)

// Job represents a registered recurring cron job entity in tasks.jobs.
type Job struct {
	ID             uuid.UUID       `json:"id"`
	Name           string          `json:"name"`
	CronExpression string          `json:"cron_expression"`
	Timezone       string          `json:"timezone"`
	TargetType     string          `json:"target_type"`
	TargetPayload  json.RawMessage `json:"target_payload"`
	IsEnabled      bool            `json:"is_enabled"`
	LastRunAt      *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt      time.Time       `json:"next_run_at"`
	CreatedAt      time.Time       `json:"created_at"`
	LastUpdatedAt  time.Time       `json:"last_updated_at"`
}

// GetJobResponse augments Job with next run countdown in seconds.
type GetJobResponse struct {
	Job
	NextRunCountdownSeconds int64 `json:"next_run_countdown_seconds"`
}

// CreateJobInput defines the request body for registering a new cron job.
type CreateJobInput struct {
	Name           string          `json:"name"`
	CronExpression string          `json:"cron_expression"`
	Timezone       string          `json:"timezone,omitempty"`
	TargetType     string          `json:"target_type"`
	TargetPayload  json.RawMessage `json:"target_payload"`
	IsEnabled      *bool           `json:"is_enabled,omitempty"`
}

// UpdateJobInput defines the request body for updating an existing job.
type UpdateJobInput struct {
	Name           *string          `json:"name,omitempty"`
	CronExpression *string          `json:"cron_expression,omitempty"`
	Timezone       *string          `json:"timezone,omitempty"`
	TargetType     *string          `json:"target_type,omitempty"`
	TargetPayload  *json.RawMessage `json:"target_payload,omitempty"`
	IsEnabled      *bool            `json:"is_enabled,omitempty"`
}

// ListJobsResponse represents the output of listing cron jobs.
type ListJobsResponse struct {
	Jobs  []GetJobResponse `json:"jobs"`
	Count int              `json:"count"`
}

// Execution represents an execution task queued in tasks.executions.
type Execution struct {
	ID          uuid.UUID       `json:"id"`
	JobID       *uuid.UUID      `json:"job_id,omitempty"`
	Status      string          `json:"status"`
	RunAt       time.Time       `json:"run_at"`
	Payload     json.RawMessage `json:"payload"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	LockedAt    *time.Time      `json:"locked_at,omitempty"`
	LockedBy    *string         `json:"locked_by,omitempty"`
	LastError   *string         `json:"last_error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// TriggerExecutionInput defines the request body for triggering an immediate execution.
type TriggerExecutionInput struct {
	JobID   *uuid.UUID       `json:"job_id,omitempty"`
	Payload *json.RawMessage `json:"payload,omitempty"`
}

// TriggerExecutionResponse defines the response after triggering an execution.
type TriggerExecutionResponse struct {
	ExecutionID uuid.UUID `json:"execution_id"`
	Status      string    `json:"status"`
	RunAt       time.Time `json:"run_at"`
}

// ExecutionLog represents a historical audit record in tasks.execution_logs.
type ExecutionLog struct {
	ID                  uuid.UUID  `json:"id"`
	JobID               *uuid.UUID `json:"job_id,omitempty"`
	ExecutionID         uuid.UUID  `json:"execution_id"`
	Status              string     `json:"status"`
	ResponseStatusCode  *int       `json:"response_status_code,omitempty"`
	ExecutionDurationMs int        `json:"execution_duration_ms"`
	ResponseBody        *string    `json:"response_body,omitempty"`
	ErrorMessage        *string    `json:"error_message,omitempty"`
	ExecutedAt          time.Time  `json:"executed_at"`
}

// ListExecutionsResponse represents the output of querying execution history logs.
type ListExecutionsResponse struct {
	Executions []ExecutionLog `json:"executions"`
	Count      int            `json:"count"`
}

// DLQExecution represents a failed execution isolated in the dead-letter queue.
type DLQExecution struct {
	ID          uuid.UUID       `json:"id"`
	JobID       *uuid.UUID      `json:"job_id,omitempty"`
	Status      string          `json:"status"`
	RunAt       time.Time       `json:"run_at"`
	Payload     json.RawMessage `json:"payload"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	LastError   *string         `json:"last_error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// ListDLQResponse represents the output of listing dead-letter queue tasks.
type ListDLQResponse struct {
	DLQ   []DLQExecution `json:"dlq"`
	Count int            `json:"count"`
}

// RetryDLQResponse defines the output after re-queuing a dead-lettered execution.
type RetryDLQResponse struct {
	ExecutionID uuid.UUID `json:"execution_id"`
	Status      string    `json:"status"`
	Message     string    `json:"message"`
}

// GetStatsResponse represents service operational telemetry.
type GetStatsResponse struct {
	TotalJobs           int     `json:"total_jobs"`
	ActiveJobs          int     `json:"active_jobs"`
	PausedJobs          int     `json:"paused_jobs"`
	PendingExecutions   int     `json:"pending_executions"`
	RunningExecutions   int     `json:"running_executions"`
	CompletedExecutions int     `json:"completed_executions"`
	FailedExecutions    int     `json:"failed_executions"`
	SuccessRate         float64 `json:"success_rate"`
}

// HTTPPayload defines target payload fields when TargetType is "http".
type HTTPPayload struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// SQLPayload defines target payload fields when TargetType is "sql".
type SQLPayload struct {
	Query  string `json:"query"`
	Params []any  `json:"params,omitempty"`
}
