package tasks

import (
	"encoding/json"
	"time"

	"uuid"

	"layr.sh/core"
)

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

// ListExecutionsResponse represents the output of querying execution history logs.
type ListExecutionsResponse struct {
	Data []ExecutionLog `json:"data"`
	core.PaginationOutput
}

// ListDLQResponse represents the output of listing dead-letter queue tasks.
type ListDLQResponse struct {
	Data []DLQExecution `json:"data"`
	core.PaginationOutput
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
