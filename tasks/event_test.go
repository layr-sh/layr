package tasks

import (
	"net/http"
	"testing"
	"time"

	"uuid"

	"github.com/stretchr/testify/require"
)

func TestTasksEventUnit(t *testing.T) {
	t.Parallel()

	testJobID := uuid.NewV7()
	testExecutionID := uuid.NewV7()
	now := time.Now().UTC()

	t.Run("creates job created event", func(t *testing.T) {
		t.Parallel()
		jobCreatedEventData := JobCreatedEventData{
			ID:             testJobID,
			Name:           "nightly-sync",
			CronExpression: "0 0 * * *",
			TargetType:     TargetTypeSQL,
			NextRunAt:      now,
		}
		event := NewJobCreatedEvent(testJobID.String(), jobCreatedEventData)
		require.Equal(t, "tasks.job.created", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testJobID.String(), *event.ResourceID)
	})

	t.Run("creates job updated event", func(t *testing.T) {
		t.Parallel()
		jobUpdatedEventData := JobUpdatedEventData{
			ID:             testJobID,
			Name:           "nightly-sync-renamed",
			CronExpression: "0 1 * * *",
			TargetType:     TargetTypeSQL,
			NextRunAt:      now,
		}
		event := NewJobUpdatedEvent(testJobID.String(), jobUpdatedEventData)
		require.Equal(t, "tasks.job.updated", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testJobID.String(), *event.ResourceID)
	})

	t.Run("creates job deleted event", func(t *testing.T) {
		t.Parallel()
		jobDeletedEventData := JobDeletedEventData{
			JobID:                  testJobID,
			JobName:                "nightly-sync",
			DeletedExecutionsCount: 3,
		}
		event := NewJobDeletedEvent(testJobID.String(), jobDeletedEventData)
		require.Equal(t, "tasks.job.deleted", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testJobID.String(), *event.ResourceID)
	})

	t.Run("creates job paused event", func(t *testing.T) {
		t.Parallel()
		jobPausedEventData := JobPausedEventData{
			JobID:   testJobID,
			JobName: "nightly-sync",
		}
		event := NewJobPausedEvent(testJobID.String(), jobPausedEventData)
		require.Equal(t, "tasks.job.paused", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testJobID.String(), *event.ResourceID)
	})

	t.Run("creates job resumed event", func(t *testing.T) {
		t.Parallel()
		jobResumedEventData := JobResumedEventData{
			JobID:     testJobID,
			JobName:   "nightly-sync",
			NextRunAt: now,
		}
		event := NewJobResumedEvent(testJobID.String(), jobResumedEventData)
		require.Equal(t, "tasks.job.resumed", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testJobID.String(), *event.ResourceID)
	})

	t.Run("creates job scheduled event", func(t *testing.T) {
		t.Parallel()
		jobScheduledEventData := JobScheduledEventData{
			JobID:         testJobID,
			JobName:       "nightly-sync",
			PreviousRunAt: &now,
			NextRunAt:     now.Add(time.Hour),
		}
		event := NewJobScheduledEvent(testJobID.String(), jobScheduledEventData)
		require.Equal(t, "tasks.job.scheduled", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testJobID.String(), *event.ResourceID)
	})

	t.Run("creates execution enqueued event", func(t *testing.T) {
		t.Parallel()
		executionEnqueuedEventData := ExecutionEnqueuedEventData{
			ExecutionID: testExecutionID,
			JobID:       &testJobID,
			JobName:     "nightly-sync",
			RunAt:       now,
			TargetType:  TargetTypeHTTP,
			IsImmediate: true,
		}
		event := NewExecutionEnqueuedEvent(testExecutionID.String(), executionEnqueuedEventData)
		require.Equal(t, "tasks.execution.enqueued", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testExecutionID.String(), *event.ResourceID)
	})

	t.Run("creates execution started event", func(t *testing.T) {
		t.Parallel()
		executionStartedEventData := ExecutionStartedEventData{
			ExecutionID: testExecutionID,
			JobID:       &testJobID,
			JobName:     "nightly-sync",
			Attempt:     1,
			LockedBy:    "worker-node-1",
		}
		event := NewExecutionStartedEvent(testExecutionID.String(), executionStartedEventData)
		require.Equal(t, "tasks.execution.started", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testExecutionID.String(), *event.ResourceID)
	})

	t.Run("creates execution completed event", func(t *testing.T) {
		t.Parallel()
		statusCode := http.StatusOK
		executionCompletedEventData := ExecutionCompletedEventData{
			ExecutionID:        testExecutionID,
			JobID:              &testJobID,
			JobName:            "nightly-sync",
			DurationMs:         150,
			ResponseStatusCode: &statusCode,
			Attempt:            1,
		}
		event := NewExecutionCompletedEvent(testExecutionID.String(), executionCompletedEventData)
		require.Equal(t, "tasks.execution.completed", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testExecutionID.String(), *event.ResourceID)
	})

	t.Run("creates execution failed event", func(t *testing.T) {
		t.Parallel()
		executionFailedEventData := ExecutionFailedEventData{
			ExecutionID: testExecutionID,
			JobID:       &testJobID,
			JobName:     "nightly-sync",
			DurationMs:  200,
			Attempt:     2,
			Error:       "connection timeout",
		}
		event := NewExecutionFailedEvent(testExecutionID.String(), executionFailedEventData)
		require.Equal(t, "tasks.execution.failed", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testExecutionID.String(), *event.ResourceID)
	})

	t.Run("creates execution timed out event", func(t *testing.T) {
		t.Parallel()
		executionTimedOutEventData := ExecutionTimedOutEventData{
			ExecutionID:         testExecutionID,
			JobID:               &testJobID,
			JobName:             "nightly-sync",
			DurationMs:          30000,
			TimeoutLimitSeconds: 30,
			Attempt:             1,
		}
		event := NewExecutionTimedOutEvent(testExecutionID.String(), executionTimedOutEventData)
		require.Equal(t, "tasks.execution.timed_out", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testExecutionID.String(), *event.ResourceID)
	})

	t.Run("creates execution retried event", func(t *testing.T) {
		t.Parallel()
		executionRetriedEventData := ExecutionRetriedEventData{
			ExecutionID:    testExecutionID,
			JobID:          &testJobID,
			JobName:        "nightly-sync",
			Attempt:        1,
			MaxAttempts:    3,
			NextRunAt:      now.Add(time.Second),
			BackoffDelayMs: 1000,
			Error:          "temporary failure",
		}
		event := NewExecutionRetriedEvent(testExecutionID.String(), executionRetriedEventData)
		require.Equal(t, "tasks.execution.retried", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testExecutionID.String(), *event.ResourceID)
	})

	t.Run("creates execution cancelled event", func(t *testing.T) {
		t.Parallel()
		executionCancelledEventData := ExecutionCancelledEventData{
			ExecutionID: testExecutionID,
			JobID:       &testJobID,
			Reason:      "parent job deleted",
		}
		event := NewExecutionCancelledEvent(testExecutionID.String(), executionCancelledEventData)
		require.Equal(t, "tasks.execution.cancelled", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testExecutionID.String(), *event.ResourceID)
	})

	t.Run("creates dlq isolated event", func(t *testing.T) {
		t.Parallel()
		dlqIsolatedEventData := DLQIsolatedEventData{
			ExecutionID:   testExecutionID,
			JobID:         &testJobID,
			JobName:       "nightly-sync",
			TotalAttempts: 3,
			LastError:     "fatal error",
		}
		event := NewDLQIsolatedEvent(testExecutionID.String(), dlqIsolatedEventData)
		require.Equal(t, "tasks.dlq.isolated", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testExecutionID.String(), *event.ResourceID)
	})

	t.Run("creates dlq retried event", func(t *testing.T) {
		t.Parallel()
		dlqRetriedEventData := DLQRetriedEventData{
			ExecutionID: testExecutionID,
			JobID:       &testJobID,
			JobName:     "nightly-sync",
			TriggeredBy: "control_plane",
		}
		event := NewDLQRetriedEvent(testExecutionID.String(), dlqRetriedEventData)
		require.Equal(t, "tasks.dlq.retried", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testExecutionID.String(), *event.ResourceID)
	})

	t.Run("creates dlq purged event", func(t *testing.T) {
		t.Parallel()
		dlqPurgedEventData := DLQPurgedEventData{
			ExecutionID: testExecutionID,
			JobID:       &testJobID,
		}
		event := NewDLQPurgedEvent(testExecutionID.String(), dlqPurgedEventData)
		require.Equal(t, "tasks.dlq.purged", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, testExecutionID.String(), *event.ResourceID)
	})

	t.Run("creates threat ssrf blocked event", func(t *testing.T) {
		t.Parallel()
		threatSSRFBlockedEventData := ThreatSSRFBlockedEventData{
			TargetURL:  "http://127.0.0.1:8080/hook",
			ResolvedIP: "127.0.0.1",
			Reason:     "loopback destination forbidden",
		}
		event := NewThreatSSRFBlockedEvent("127.0.0.1", threatSSRFBlockedEventData)
		require.Equal(t, "tasks.threat.ssrf_blocked", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "127.0.0.1", *event.ResourceID)
	})

	t.Run("creates threat signature failed event", func(t *testing.T) {
		t.Parallel()
		threatSignatureFailedEventData := ThreatSignatureFailedEventData{
			TargetURL: "https://example.com/webhook",
			Reason:    "missing secret key",
			Timestamp: "1234567890",
		}
		event := NewThreatSignatureFailedEvent("https://example.com/webhook", threatSignatureFailedEventData)
		require.Equal(t, "tasks.threat.signature_failed", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "https://example.com/webhook", *event.ResourceID)
	})

	t.Run("creates config updated event", func(t *testing.T) {
		t.Parallel()
		configUpdatedEventData := ConfigUpdatedEventData(DefaultConfig())
		event := NewConfigUpdatedEvent("tasks.config", configUpdatedEventData)
		require.Equal(t, "tasks.config.updated", event.Type)
		require.NotNil(t, event.ResourceID)
		require.Equal(t, "tasks.config", *event.ResourceID)
	})
}
