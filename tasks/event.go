// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"time"

	"uuid"

	"layr.sh/core"
)

// --- 1. Job Lifecycle Events ---

// JobCreatedEventData represents payload for tasks.job.created.
type JobCreatedEventData Job

// NewJobCreatedEvent creates a typed event when a recurring cron job is created.
func NewJobCreatedEvent(resourceID string, jobCreatedEventData JobCreatedEventData) core.Event {
	return core.NewEvent("tasks.job.created", jobCreatedEventData).WithResourceID(resourceID)
}

// JobUpdatedEventData represents payload for tasks.job.updated.
type JobUpdatedEventData Job

// NewJobUpdatedEvent creates a typed event when a recurring cron job is updated.
func NewJobUpdatedEvent(resourceID string, jobUpdatedEventData JobUpdatedEventData) core.Event {
	return core.NewEvent("tasks.job.updated", jobUpdatedEventData).WithResourceID(resourceID)
}

// JobDeletedEventData represents payload for tasks.job.deleted.
type JobDeletedEventData struct {
	JobID                  uuid.UUID `json:"job_id"`
	JobName                string    `json:"job_name"`
	DeletedExecutionsCount int       `json:"deleted_executions_count"`
}

// NewJobDeletedEvent creates a typed event when a recurring cron job is deleted.
func NewJobDeletedEvent(resourceID string, jobDeletedEventData JobDeletedEventData) core.Event {
	return core.NewEvent("tasks.job.deleted", jobDeletedEventData).WithResourceID(resourceID)
}

// JobPausedEventData represents payload for tasks.job.paused.
type JobPausedEventData struct {
	JobID   uuid.UUID `json:"job_id"`
	JobName string    `json:"job_name"`
}

// NewJobPausedEvent creates a typed event when a job is paused.
func NewJobPausedEvent(resourceID string, jobPausedEventData JobPausedEventData) core.Event {
	return core.NewEvent("tasks.job.paused", jobPausedEventData).WithResourceID(resourceID)
}

// JobResumedEventData represents payload for tasks.job.resumed.
type JobResumedEventData struct {
	JobID     uuid.UUID `json:"job_id"`
	JobName   string    `json:"job_name"`
	NextRunAt time.Time `json:"next_run_at"`
}

// NewJobResumedEvent creates a typed event when a job is resumed.
func NewJobResumedEvent(resourceID string, jobResumedEventData JobResumedEventData) core.Event {
	return core.NewEvent("tasks.job.resumed", jobResumedEventData).WithResourceID(resourceID)
}

// JobScheduledEventData represents payload for tasks.job.scheduled.
type JobScheduledEventData struct {
	JobID         uuid.UUID  `json:"job_id"`
	JobName       string     `json:"job_name"`
	PreviousRunAt *time.Time `json:"previous_run_at,omitempty"`
	NextRunAt     time.Time  `json:"next_run_at"`
}

// NewJobScheduledEvent creates a typed event when a job trigger advances.
func NewJobScheduledEvent(resourceID string, jobScheduledEventData JobScheduledEventData) core.Event {
	return core.NewEvent("tasks.job.scheduled", jobScheduledEventData).WithResourceID(resourceID)
}

// --- 2. Execution Queue Events ---

// ExecutionEnqueuedEventData represents payload for tasks.execution.enqueued.
type ExecutionEnqueuedEventData struct {
	ExecutionID uuid.UUID  `json:"execution_id"`
	JobID       *uuid.UUID `json:"job_id,omitempty"`
	JobName     string     `json:"job_name,omitempty"`
	RunAt       time.Time  `json:"run_at"`
	TargetType  string     `json:"target_type"`
	IsImmediate bool       `json:"is_immediate"`
}

// NewExecutionEnqueuedEvent creates a typed event when an execution is enqueued.
func NewExecutionEnqueuedEvent(resourceID string, executionEnqueuedEventData ExecutionEnqueuedEventData) core.Event {
	return core.NewEvent("tasks.execution.enqueued", executionEnqueuedEventData).WithResourceID(resourceID)
}

// ExecutionStartedEventData represents payload for tasks.execution.started.
type ExecutionStartedEventData struct {
	ExecutionID uuid.UUID  `json:"execution_id"`
	JobID       *uuid.UUID `json:"job_id,omitempty"`
	JobName     string     `json:"job_name,omitempty"`
	Attempt     int        `json:"attempt"`
	LockedBy    string     `json:"locked_by"`
}

// NewExecutionStartedEvent creates a typed event when an execution begins processing.
func NewExecutionStartedEvent(resourceID string, executionStartedEventData ExecutionStartedEventData) core.Event {
	return core.NewEvent("tasks.execution.started", executionStartedEventData).WithResourceID(resourceID)
}

// ExecutionCompletedEventData represents payload for tasks.execution.completed.
type ExecutionCompletedEventData struct {
	ExecutionID        uuid.UUID  `json:"execution_id"`
	JobID              *uuid.UUID `json:"job_id,omitempty"`
	JobName            string     `json:"job_name,omitempty"`
	DurationMs         int64      `json:"duration_ms"`
	ResponseStatusCode *int       `json:"response_status_code,omitempty"`
	Attempt            int        `json:"attempt"`
}

// NewExecutionCompletedEvent creates a typed event when an execution succeeds.
func NewExecutionCompletedEvent(resourceID string, executionCompletedEventData ExecutionCompletedEventData) core.Event {
	return core.NewEvent("tasks.execution.completed", executionCompletedEventData).WithResourceID(resourceID)
}

// ExecutionFailedEventData represents payload for tasks.execution.failed.
type ExecutionFailedEventData struct {
	ExecutionID        uuid.UUID  `json:"execution_id"`
	JobID              *uuid.UUID `json:"job_id,omitempty"`
	JobName            string     `json:"job_name,omitempty"`
	DurationMs         int64      `json:"duration_ms"`
	ResponseStatusCode *int       `json:"response_status_code,omitempty"`
	Attempt            int        `json:"attempt"`
	Error              string     `json:"error"`
}

// NewExecutionFailedEvent creates a typed event when an execution fails before DLQ threshold.
func NewExecutionFailedEvent(resourceID string, executionFailedEventData ExecutionFailedEventData) core.Event {
	return core.NewEvent("tasks.execution.failed", executionFailedEventData).WithResourceID(resourceID)
}

// ExecutionTimedOutEventData represents payload for tasks.execution.timed_out.
type ExecutionTimedOutEventData struct {
	ExecutionID         uuid.UUID  `json:"execution_id"`
	JobID               *uuid.UUID `json:"job_id,omitempty"`
	JobName             string     `json:"job_name,omitempty"`
	DurationMs          int64      `json:"duration_ms"`
	TimeoutLimitSeconds int        `json:"timeout_limit_seconds"`
	Attempt             int        `json:"attempt"`
}

// NewExecutionTimedOutEvent creates a typed event when an execution exceeds timeout limit.
func NewExecutionTimedOutEvent(resourceID string, executionTimedOutEventData ExecutionTimedOutEventData) core.Event {
	return core.NewEvent("tasks.execution.timed_out", executionTimedOutEventData).WithResourceID(resourceID)
}

// ExecutionRetriedEventData represents payload for tasks.execution.retried.
type ExecutionRetriedEventData struct {
	ExecutionID    uuid.UUID  `json:"execution_id"`
	JobID          *uuid.UUID `json:"job_id,omitempty"`
	JobName        string     `json:"job_name,omitempty"`
	Attempt        int        `json:"attempt"`
	MaxAttempts    int        `json:"max_attempts"`
	NextRunAt      time.Time  `json:"next_run_at"`
	BackoffDelayMs int64      `json:"backoff_delay_ms"`
	Error          string     `json:"error"`
}

// NewExecutionRetriedEvent creates a typed event when an execution is rescheduled with backoff.
func NewExecutionRetriedEvent(resourceID string, executionRetriedEventData ExecutionRetriedEventData) core.Event {
	return core.NewEvent("tasks.execution.retried", executionRetriedEventData).WithResourceID(resourceID)
}

// ExecutionCancelledEventData represents payload for tasks.execution.cancelled.
type ExecutionCancelledEventData struct {
	ExecutionID uuid.UUID  `json:"execution_id"`
	JobID       *uuid.UUID `json:"job_id,omitempty"`
	Reason      string     `json:"reason"`
}

// NewExecutionCancelledEvent creates a typed event when pending executions are cancelled.
func NewExecutionCancelledEvent(resourceID string, executionCancelledEventData ExecutionCancelledEventData) core.Event {
	return core.NewEvent("tasks.execution.cancelled", executionCancelledEventData).WithResourceID(resourceID)
}

// --- 3. Dead-Letter Queue (DLQ) Events ---

// DLQIsolatedEventData represents payload for tasks.dlq.isolated.
type DLQIsolatedEventData struct {
	ExecutionID   uuid.UUID  `json:"execution_id"`
	JobID         *uuid.UUID `json:"job_id,omitempty"`
	JobName       string     `json:"job_name,omitempty"`
	TotalAttempts int        `json:"total_attempts"`
	LastError     string     `json:"last_error"`
}

// NewDLQIsolatedEvent creates a typed event when an execution transitions to DLQ.
func NewDLQIsolatedEvent(resourceID string, dlqIsolatedEventData DLQIsolatedEventData) core.Event {
	return core.NewEvent("tasks.dlq.isolated", dlqIsolatedEventData).WithResourceID(resourceID)
}

// DLQRetriedEventData represents payload for tasks.dlq.retried.
type DLQRetriedEventData struct {
	ExecutionID uuid.UUID  `json:"execution_id"`
	JobID       *uuid.UUID `json:"job_id,omitempty"`
	JobName     string     `json:"job_name,omitempty"`
	TriggeredBy string     `json:"triggered_by"`
}

// NewDLQRetriedEvent creates a typed event when a dead-lettered execution is manually re-queued.
func NewDLQRetriedEvent(resourceID string, dlqRetriedEventData DLQRetriedEventData) core.Event {
	return core.NewEvent("tasks.dlq.retried", dlqRetriedEventData).WithResourceID(resourceID)
}

// DLQPurgedEventData represents payload for tasks.dlq.purged.
type DLQPurgedEventData struct {
	ExecutionID uuid.UUID  `json:"execution_id"`
	JobID       *uuid.UUID `json:"job_id,omitempty"`
}

// NewDLQPurgedEvent creates a typed event when a DLQ execution is purged.
func NewDLQPurgedEvent(resourceID string, dlqPurgedEventData DLQPurgedEventData) core.Event {
	return core.NewEvent("tasks.dlq.purged", dlqPurgedEventData).WithResourceID(resourceID)
}

// --- 4. Security & Threat Mitigation Events ---

// ThreatSSRFBlockedEventData represents payload for tasks.threat.ssrf_blocked.
type ThreatSSRFBlockedEventData struct {
	TargetURL  string `json:"target_url"`
	ResolvedIP string `json:"resolved_ip,omitempty"`
	Reason     string `json:"reason"`
}

// NewThreatSSRFBlockedEvent creates a typed event when a webhook target is blocked by SSRF protection.
func NewThreatSSRFBlockedEvent(resourceID string, threatSSRFBlockedEventData ThreatSSRFBlockedEventData) core.Event {
	return core.NewEvent("tasks.threat.ssrf_blocked", threatSSRFBlockedEventData).WithResourceID(resourceID)
}

// ThreatSignatureFailedEventData represents payload for tasks.threat.signature_failed.
type ThreatSignatureFailedEventData struct {
	TargetURL string `json:"target_url"`
	Reason    string `json:"reason"`
	Timestamp string `json:"timestamp,omitempty"`
}

// NewThreatSignatureFailedEvent creates a typed event when webhook signature verification fails.
func NewThreatSignatureFailedEvent(resourceID string, threatSignatureFailedEventData ThreatSignatureFailedEventData) core.Event {
	return core.NewEvent("tasks.threat.signature_failed", threatSignatureFailedEventData).WithResourceID(resourceID)
}

// --- 5. Runtime Configuration Events ---

// ConfigUpdatedEventData represents payload for tasks.config.updated.
type ConfigUpdatedEventData Config

// NewConfigUpdatedEvent creates a typed event when tasks runtime config is updated.
func NewConfigUpdatedEvent(resourceID string, configUpdatedEventData ConfigUpdatedEventData) core.Event {
	return core.NewEvent("tasks.config.updated", configUpdatedEventData).WithResourceID(resourceID)
}
