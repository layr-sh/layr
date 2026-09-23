package tasks

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksJobManagerIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(kernel)
	jobManager := NewJobManager(kernel, configManager)
	require.NotNil(t, jobManager)

	// --- 1. CreateJob Validations and Errors ---
	t.Run("CreateJob validates input", func(t *testing.T) {
		validSQLPayload, _ := json.Marshal(map[string]any{"query": "SELECT 1"})

		// Empty name
		_, err := jobManager.CreateJob(ctx, CreateJobInput{Name: ""})
		require.Error(t, err)
		require.Contains(t, err.Error(), "job name is required")

		// Invalid cron
		_, err = jobManager.CreateJob(ctx, CreateJobInput{Name: "job-1", CronExpression: "bad-cron"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid cron expression")

		// Invalid timezone
		_, err = jobManager.CreateJob(ctx, CreateJobInput{Name: "job-1", CronExpression: "0 0 * * *", Timezone: "Invalid/TZ"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid timezone")

		// Invalid target_type
		_, err = jobManager.CreateJob(ctx, CreateJobInput{Name: "job-1", CronExpression: "0 0 * * *", TargetType: "unsupported"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid target_type")

		// Invalid payload for sql
		_, err = jobManager.CreateJob(ctx, CreateJobInput{Name: "job-1", CronExpression: "0 0 * * *", TargetType: TargetTypeSQL, TargetPayload: []byte(`{"params":[]}`)})
		require.Error(t, err)
		require.Contains(t, err.Error(), "query is required")

		// Successful creation with nil IsEnabled defaults to true
		createdJob, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "job-valid-1",
			CronExpression: "0 12 * * *",
			TargetType:     TargetTypeSQL,
			TargetPayload:  validSQLPayload,
		})
		require.NoError(t, err)
		require.NotNil(t, createdJob)
		require.True(t, createdJob.IsEnabled)

		// Duplicate job creation fails with ErrJobAlreadyExists
		_, duplicateErr := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "job-valid-1",
			CronExpression: "0 12 * * *",
			TargetType:     TargetTypeSQL,
			TargetPayload:  validSQLPayload,
		})
		require.Error(t, duplicateErr)
		require.ErrorIs(t, duplicateErr, ErrJobAlreadyExists)
	})

	// --- 2. GetJob and ListJobs ---
	t.Run("GetJob and ListJobs functionality", func(t *testing.T) {
		validSQLPayload, _ := json.Marshal(map[string]any{"query": "SELECT 1"})
		isEnabledFalse := false
		createdJob, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "job-paused-1",
			CronExpression: "0 0 1 1 *",
			TargetType:     TargetTypeSQL,
			TargetPayload:  validSQLPayload,
			IsEnabled:      &isEnabledFalse,
		})
		require.NoError(t, err)

		// GetJob returns job with countdown
		jobWithCountdown, err := jobManager.GetJob(ctx, createdJob.ID)
		require.NoError(t, err)
		require.Equal(t, createdJob.ID, jobWithCountdown.ID)
		require.False(t, jobWithCountdown.IsEnabled)
		require.Positive(t, jobWithCountdown.NextRunCountdownSeconds)

		// GetJob for past next_run_at returns countdown 0
		_, err = kernel.DB().Exec(ctx, "UPDATE tasks.jobs SET next_run_at = $1 WHERE id = $2", time.Now().UTC().Add(-time.Hour), createdJob.ID)
		require.NoError(t, err)

		zeroCountdownJobWithCountdown, err := jobManager.GetJob(ctx, createdJob.ID)
		require.NoError(t, err)
		require.Equal(t, int64(0), zeroCountdownJobWithCountdown.NextRunCountdownSeconds)

		// GetJob non-existent ID
		_, notFoundErr := jobManager.GetJob(ctx, uuid.NewV7())
		require.Error(t, notFoundErr)
		require.ErrorIs(t, notFoundErr, ErrJobNotFound)

		// ListJobs returns jobs
		jobs, err := jobManager.ListJobs(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, jobs)
	})

	// --- 3. UpdateJob ---
	t.Run("UpdateJob validations and state transitions", func(t *testing.T) {
		validSQLPayload, _ := json.Marshal(map[string]any{"query": "SELECT 1"})
		job, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "job-to-update",
			CronExpression: "0 1 * * *",
			TargetType:     TargetTypeSQL,
			TargetPayload:  validSQLPayload,
		})
		require.NoError(t, err)

		// Non-existent job
		_, notFoundErr := jobManager.UpdateJob(ctx, uuid.NewV7(), UpdateJobInput{})
		require.Error(t, notFoundErr)
		require.ErrorIs(t, notFoundErr, ErrJobNotFound)

		// Invalid cron
		badCron := "invalid"
		_, err = jobManager.UpdateJob(ctx, job.ID, UpdateJobInput{CronExpression: &badCron})
		require.Error(t, err)

		// Invalid timezone
		badTimezone := "bad/tz"
		_, err = jobManager.UpdateJob(ctx, job.ID, UpdateJobInput{Timezone: &badTimezone})
		require.Error(t, err)

		// Invalid target type
		badTarget := "unsupported"
		_, err = jobManager.UpdateJob(ctx, job.ID, UpdateJobInput{TargetType: &badTarget})
		require.Error(t, err)

		// Invalid payload
		badPayloadValue := json.RawMessage(`{"wrong": 123}`)
		badTargetSQL := TargetTypeSQL
		_, err = jobManager.UpdateJob(ctx, job.ID, UpdateJobInput{TargetType: &badTargetSQL, TargetPayload: &badPayloadValue})
		require.Error(t, err)

		// Update name conflict
		conflictName := "job-valid-1"
		_, err = jobManager.UpdateJob(ctx, job.ID, UpdateJobInput{Name: &conflictName})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrJobAlreadyExists)

		// Pause job
		pausedState := false
		updatedJob, err := jobManager.UpdateJob(ctx, job.ID, UpdateJobInput{IsEnabled: &pausedState})
		require.NoError(t, err)
		require.False(t, updatedJob.IsEnabled)

		// Resume job
		resumedState := true
		resumedJob, err := jobManager.UpdateJob(ctx, job.ID, UpdateJobInput{IsEnabled: &resumedState})
		require.NoError(t, err)
		require.True(t, resumedJob.IsEnabled)
	})

	// --- 4. TriggerExecution and ListExecutions ---
	t.Run("TriggerExecution ad-hoc and job-based", func(t *testing.T) {
		validSQLPayload, _ := json.Marshal(map[string]any{"query": "SELECT 1"})
		job, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "job-for-trigger",
			CronExpression: "0 2 * * *",
			TargetType:     TargetTypeSQL,
			TargetPayload:  validSQLPayload,
		})
		require.NoError(t, err)

		// Trigger non-existent job
		missingJobID := uuid.NewV7()
		_, err = jobManager.TriggerExecution(ctx, TriggerExecutionInput{JobID: &missingJobID})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrJobNotFound)

		// Trigger from existing job
		triggerExecutionResponse, err := jobManager.TriggerExecution(ctx, TriggerExecutionInput{JobID: &job.ID})
		require.NoError(t, err)
		require.NotNil(t, triggerExecutionResponse)
		require.Equal(t, "pending", triggerExecutionResponse.Status)

		// Trigger with payload override
		overridePayloadValue := json.RawMessage(`{"query": "SELECT 2"}`)
		overrideTriggerExecutionResponse, err := jobManager.TriggerExecution(ctx, TriggerExecutionInput{
			JobID:   &job.ID,
			Payload: &overridePayloadValue,
		})
		require.NoError(t, err)
		require.NotNil(t, overrideTriggerExecutionResponse)

		// Ad-hoc trigger without job ID
		adHocPayloadValue := json.RawMessage(`{"query": "SELECT 3"}`)
		adHocTriggerExecutionResponse, err := jobManager.TriggerExecution(ctx, TriggerExecutionInput{
			Payload: &adHocPayloadValue,
		})
		require.NoError(t, err)
		require.NotNil(t, adHocTriggerExecutionResponse)

		// Insert dummy execution log to test ListExecutions
		_, err = kernel.DB().Exec(ctx, `
			INSERT INTO tasks.execution_logs (
				job_id, execution_id, status, execution_duration_ms, executed_at
			) VALUES ($1, $2, 'completed', 45, clock_timestamp())
		`, job.ID, triggerExecutionResponse.ExecutionID)
		require.NoError(t, err)

		// ListExecutions filtered by job_id
		executionsByJob, countByJob, err := jobManager.ListExecutions(ctx, &job.ID, 10, 0)
		require.NoError(t, err)
		require.Positive(t, countByJob)
		require.NotEmpty(t, executionsByJob)

		// ListExecutions unfiltered
		executionsAll, countAll, err := jobManager.ListExecutions(ctx, nil, 10, 0)
		require.NoError(t, err)
		require.Positive(t, countAll)
		require.NotEmpty(t, executionsAll)
	})

	// --- 5. DLQ Management ---
	t.Run("DLQ List, Retry, and Purge", func(t *testing.T) {
		validSQLPayload, _ := json.Marshal(map[string]any{"query": "SELECT 1"})
		job, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "job-for-dlq",
			CronExpression: "0 3 * * *",
			TargetType:     TargetTypeSQL,
			TargetPayload:  validSQLPayload,
		})
		require.NoError(t, err)

		executionID := uuid.NewV7()
		_, err = kernel.DB().Exec(ctx, `
			INSERT INTO tasks.executions (
				id, job_id, status, run_at, payload, attempts, max_attempts, last_error, created_at
			) VALUES ($1, $2, 'failed', clock_timestamp(), $3, 5, 5, 'fatal crash', clock_timestamp())
		`, executionID, job.ID, validSQLPayload)
		require.NoError(t, err)

		// List DLQ
		dlqList, totalDLQ, err := jobManager.ListDLQ(ctx, 10, 0)
		require.NoError(t, err)
		require.Positive(t, totalDLQ)
		require.NotEmpty(t, dlqList)

		// Retry non-existent DLQ execution
		_, notFoundRetryErr := jobManager.RetryDLQExecution(ctx, uuid.NewV7())
		require.Error(t, notFoundRetryErr)
		require.ErrorIs(t, notFoundRetryErr, ErrDLQItemNotFound)

		// Retry existing DLQ execution
		retryDLQResponse, err := jobManager.RetryDLQExecution(ctx, executionID)
		require.NoError(t, err)
		require.Equal(t, executionID, retryDLQResponse.ExecutionID)
		require.Equal(t, "pending", retryDLQResponse.Status)

		// Purge non-existent DLQ execution
		notFoundPurgeErr := jobManager.PurgeDLQExecution(ctx, uuid.NewV7())
		require.Error(t, notFoundPurgeErr)
		require.ErrorIs(t, notFoundPurgeErr, ErrDLQItemNotFound)

		// Purge item that is not failed returns ErrDLQItemNotFound
		notFailedPurgeErr := jobManager.PurgeDLQExecution(ctx, executionID)
		require.Error(t, notFailedPurgeErr)
		require.ErrorIs(t, notFailedPurgeErr, ErrDLQItemNotFound)

		// Set status back to failed and purge
		_, err = kernel.DB().Exec(ctx, "UPDATE tasks.executions SET status = 'failed' WHERE id = $1", executionID)
		require.NoError(t, err)

		purgeErr := jobManager.PurgeDLQExecution(ctx, executionID)
		require.NoError(t, purgeErr)
	})

	// --- 6. Telemetry Stats ---
	t.Run("GetStats telemetry metrics calculation", func(t *testing.T) {
		statsResponse, err := jobManager.GetStats(ctx)
		require.NoError(t, err)
		require.NotNil(t, statsResponse)
		require.Positive(t, statsResponse.TotalJobs)
	})

	// --- 7. DeleteJob ---
	t.Run("DeleteJob deletes job and cancels pending executions", func(t *testing.T) {
		validSQLPayload, _ := json.Marshal(map[string]any{"query": "SELECT 1"})
		job, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "job-to-delete",
			CronExpression: "0 4 * * *",
			TargetType:     TargetTypeSQL,
			TargetPayload:  validSQLPayload,
		})
		require.NoError(t, err)

		// Enqueue a pending execution
		_, err = jobManager.TriggerExecution(ctx, TriggerExecutionInput{JobID: &job.ID})
		require.NoError(t, err)

		// Delete job
		deleteErr := jobManager.DeleteJob(ctx, job.ID)
		require.NoError(t, deleteErr)

		// Delete non-existent job
		missingDeleteErr := jobManager.DeleteJob(ctx, uuid.NewV7())
		require.Error(t, missingDeleteErr)
		require.ErrorIs(t, missingDeleteErr, ErrJobNotFound)

		// Delete failure paths via triggers
		failJob, createFailJobErr := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "job-fail-del",
			CronExpression: "0 4 * * *",
			TargetType:     TargetTypeSQL,
			TargetPayload:  validSQLPayload,
		})
		require.NoError(t, createFailJobErr)

		_, _ = kernel.DB().Exec(ctx, `
			CREATE OR REPLACE FUNCTION fail_exec_delete() RETURNS trigger AS $$
			BEGIN
				RAISE EXCEPTION 'cannot delete execution';
			END;
			$$ LANGUAGE plpgsql;
			CREATE TRIGGER trig_fail_exec_del BEFORE DELETE ON tasks.executions
			FOR EACH ROW EXECUTE FUNCTION fail_exec_delete();
		`)
		_, _ = jobManager.TriggerExecution(ctx, TriggerExecutionInput{JobID: &failJob.ID})
		failDelErr := jobManager.DeleteJob(ctx, failJob.ID)
		require.Error(t, failDelErr)
		require.Contains(t, failDelErr.Error(), "failed to cancel pending executions")
		_, _ = kernel.DB().Exec(ctx, "DROP TRIGGER trig_fail_exec_del ON tasks.executions")

		_, _ = kernel.DB().Exec(ctx, `
			CREATE OR REPLACE FUNCTION fail_job_delete() RETURNS trigger AS $$
			BEGIN
				RAISE EXCEPTION 'cannot delete job';
			END;
			$$ LANGUAGE plpgsql;
			CREATE TRIGGER trig_fail_job_del BEFORE DELETE ON tasks.jobs
			FOR EACH ROW EXECUTE FUNCTION fail_job_delete();
		`)
		failJobDelErr := jobManager.DeleteJob(ctx, failJob.ID)
		require.Error(t, failJobDelErr)
		require.Contains(t, failJobDelErr.Error(), "failed to delete job")
		_, _ = kernel.DB().Exec(ctx, "DROP TRIGGER trig_fail_job_del ON tasks.jobs")
	})

	// --- 8. Fallback inputs, constraint failures, and DLQ errors ---
	t.Run("JobManager edge cases and error triggers", func(t *testing.T) {
		validSQLPayload, _ := json.Marshal(map[string]any{"query": "SELECT 1"})
		job, err := jobManager.CreateJob(ctx, CreateJobInput{
			Name:           "job-edge-cases",
			CronExpression: "0 5 * * *",
			TargetType:     TargetTypeSQL,
			TargetPayload:  validSQLPayload,
		})
		require.NoError(t, err)

		// TriggerExecution with nil payload falls back to default empty JSON
		noPayloadTriggerExecutionResponse, err := jobManager.TriggerExecution(ctx, TriggerExecutionInput{JobID: nil, Payload: nil})
		require.NoError(t, err)
		require.NotEmpty(t, noPayloadTriggerExecutionResponse.ExecutionID)

		// ListExecutions with limit <= 0 and offset < 0 falls back to defaults
		_, count, err := jobManager.ListExecutions(ctx, nil, -1, -1)
		require.NoError(t, err)
		require.GreaterOrEqual(t, count, 0)

		// ListDLQ with limit <= 0 and offset < 0 falls back to defaults
		_, dlqCount, err := jobManager.ListDLQ(ctx, -1, -1)
		require.NoError(t, err)
		require.GreaterOrEqual(t, dlqCount, 0)

		// UpdateJob non-duplicate DB error via custom check constraint
		_, _ = kernel.DB().Exec(ctx, "ALTER TABLE tasks.jobs ADD CONSTRAINT check_name_forbidden CHECK (name != 'forbidden-name')")
		forbiddenName := "forbidden-name"
		_, err = jobManager.UpdateJob(ctx, job.ID, UpdateJobInput{Name: &forbiddenName})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to update job")
		_, _ = kernel.DB().Exec(ctx, "ALTER TABLE tasks.jobs DROP CONSTRAINT check_name_forbidden")

		// RetryDLQExecution and PurgeDLQExecution DB error triggers
		failExecID := uuid.NewV7()
		_, _ = kernel.DB().Exec(ctx, "INSERT INTO tasks.executions (id, status, run_at, payload) VALUES ($1, 'failed', clock_timestamp(), '{}'::jsonb)", failExecID)

		_, _ = kernel.DB().Exec(ctx, `
			CREATE OR REPLACE FUNCTION fail_update_exec() RETURNS trigger AS $$
			BEGIN
				RAISE EXCEPTION 'cannot update execution';
			END;
			$$ LANGUAGE plpgsql;
			CREATE TRIGGER trig_fail_exec_upd BEFORE UPDATE ON tasks.executions
			FOR EACH ROW EXECUTE FUNCTION fail_update_exec();
		`)
		_, retryErr := jobManager.RetryDLQExecution(ctx, failExecID)
		require.Error(t, retryErr)
		require.Contains(t, retryErr.Error(), "failed to re-queue dlq item")
		_, _ = kernel.DB().Exec(ctx, "DROP TRIGGER trig_fail_exec_upd ON tasks.executions")

		_, _ = kernel.DB().Exec(ctx, `
			CREATE TRIGGER trig_fail_exec_del_item BEFORE DELETE ON tasks.executions
			FOR EACH ROW EXECUTE FUNCTION fail_exec_delete();
		`)
		purgeErr := jobManager.PurgeDLQExecution(ctx, failExecID)
		require.Error(t, purgeErr)
		require.Contains(t, purgeErr.Error(), "failed to purge dlq item")
		_, _ = kernel.DB().Exec(ctx, "DROP TRIGGER trig_fail_exec_del_item ON tasks.executions")

		// GetStats with completed executions calculates positive success rate
		_, _ = kernel.DB().Exec(ctx, "INSERT INTO tasks.executions (status, run_at, payload) VALUES ('completed', clock_timestamp(), '{}'::jsonb)")
		statsResponse, err := jobManager.GetStats(ctx)
		require.NoError(t, err)
		require.NotNil(t, statsResponse)
		require.Positive(t, statsResponse.CompletedExecutions)
		require.Positive(t, statsResponse.SuccessRate)

		// GetStats executions query failure
		_, _ = kernel.DB().Exec(ctx, "ALTER TABLE tasks.executions RENAME TO executions_backup")
		_, err = jobManager.GetStats(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to query executions stats")
		_, _ = kernel.DB().Exec(ctx, "ALTER TABLE tasks.executions_backup RENAME TO executions")
	})
}

func TestTasksJobManagerBrokenDatabaseIntegration(t *testing.T) {
	brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	brokenConfigManager := NewConfigManager(brokenKernel)
	brokenJobManager := NewJobManager(brokenKernel, brokenConfigManager)
	ctx := context.Background()

	_, err := brokenJobManager.ListJobs(ctx)
	require.Error(t, err)

	_, err = brokenJobManager.TriggerExecution(ctx, TriggerExecutionInput{})
	require.Error(t, err)

	_, _, err = brokenJobManager.ListExecutions(ctx, nil, 10, 0)
	require.Error(t, err)

	_, _, err = brokenJobManager.ListDLQ(ctx, 10, 0)
	require.Error(t, err)

	_, err = brokenJobManager.GetStats(ctx)
	require.Error(t, err)

	_, err = brokenJobManager.RetryDLQExecution(ctx, uuid.NewV7())
	require.Error(t, err)

	err = brokenJobManager.PurgeDLQExecution(ctx, uuid.NewV7())
	require.Error(t, err)
}
