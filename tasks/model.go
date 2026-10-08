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

// Supported execution and log statuses.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusSuccess   = "success"
	StatusTimeout   = "timeout"
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
