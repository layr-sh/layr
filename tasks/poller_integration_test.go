package tasks

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksPollerIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(kernel)
	jobManager := NewJobManager(kernel, configManager)
	jobPoller := NewJobPoller(kernel, configManager)
	require.NotNil(t, jobPoller)

	sqlPayload, err := json.Marshal(map[string]any{"query": "SELECT 1"})
	require.NoError(t, err)

	// 1. Poll with no jobs -> returns 0
	count, err := jobPoller.PollOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// 2. Create job with next_run_at in past
	job, err := jobManager.CreateJob(ctx, CreateJobInput{
		Name:           "poller-due-job",
		CronExpression: "*/5 * * * *",
		TargetType:     TargetTypeSQL,
		TargetPayload:  sqlPayload,
	})
	require.NoError(t, err)

	pastTime := time.Now().UTC().Add(-10 * time.Minute)
	_, err = kernel.DB().Exec(ctx, "UPDATE tasks.jobs SET next_run_at = $1 WHERE id = $2", pastTime, job.ID)
	require.NoError(t, err)

	// Poll should discover 1 due job and advance schedule
	count, err = jobPoller.PollOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Verify an execution was enqueued in tasks.executions
	var executionCount int
	err = kernel.DB().QueryRow(ctx, "SELECT count(*) FROM tasks.executions WHERE job_id = $1", job.ID).Scan(&executionCount)
	require.NoError(t, err)
	require.Equal(t, 1, executionCount)

	// Verify next_run_at was advanced into the future
	var nextRunAt time.Time
	err = kernel.DB().QueryRow(ctx, "SELECT next_run_at FROM tasks.jobs WHERE id = $1", job.ID).Scan(&nextRunAt)
	require.NoError(t, err)
	require.True(t, nextRunAt.After(time.Now().UTC()))

	// 3. Polling again immediately finds 0 due jobs
	count, err = jobPoller.PollOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// 4. Inactive/paused job with past next_run_at is NOT polled
	isEnabledFalse := false
	_, err = jobManager.UpdateJob(ctx, job.ID, UpdateJobInput{IsEnabled: &isEnabledFalse})
	require.NoError(t, err)

	_, err = kernel.DB().Exec(ctx, "UPDATE tasks.jobs SET next_run_at = $1 WHERE id = $2", pastTime, job.ID)
	require.NoError(t, err)

	count, err = jobPoller.PollOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// 5. Job with invalid cron expression in DB is skipped
	_, err = kernel.DB().Exec(ctx, `
		INSERT INTO tasks.jobs (name, cron_expression, timezone, target_type, target_payload, next_run_at)
		VALUES ('bad-cron-job', 'not a cron', 'UTC', 'sql', '{"query":"SELECT 1"}', $1)
	`, pastTime)
	require.NoError(t, err)

	count, err = jobPoller.PollOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, count)
	_, _ = kernel.DB().Exec(ctx, "DELETE FROM tasks.jobs WHERE name = 'bad-cron-job'")

	// 6. Job with invalid timezone in DB falls back to UTC
	_, err = kernel.DB().Exec(ctx, `
		INSERT INTO tasks.jobs (name, cron_expression, timezone, target_type, target_payload, next_run_at)
		VALUES ('bad-tz-job', '*/5 * * * *', 'Invalid/Timezone', 'sql', '{"query":"SELECT 1"}', $1)
	`, pastTime)
	require.NoError(t, err)

	count, err = jobPoller.PollOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	_, _ = kernel.DB().Exec(ctx, "DELETE FROM tasks.jobs WHERE name = 'bad-tz-job'")

	// 7. Enqueue execution error trigger
	_, err = kernel.DB().Exec(ctx, `
		INSERT INTO tasks.jobs (name, cron_expression, timezone, target_type, target_payload, next_run_at)
		VALUES ('enqueue-err-job', '*/5 * * * *', 'UTC', 'sql', '{"query":"SELECT 1"}', $1)
	`, pastTime)
	require.NoError(t, err)

	_, _ = kernel.DB().Exec(ctx, `
		CREATE OR REPLACE FUNCTION fail_exec_insert() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'cannot insert execution';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER trig_fail_exec_ins BEFORE INSERT ON tasks.executions
		FOR EACH ROW EXECUTE FUNCTION fail_exec_insert();
	`)
	_, insertPollErr := jobPoller.PollOnce(ctx)
	require.Error(t, insertPollErr)
	_, _ = kernel.DB().Exec(ctx, "DROP TRIGGER trig_fail_exec_ins ON tasks.executions")

	// 8. Update job next_run_at error trigger
	_, _ = kernel.DB().Exec(ctx, `
		CREATE OR REPLACE FUNCTION fail_job_update() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'cannot update job';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER trig_fail_job_upd BEFORE UPDATE ON tasks.jobs
		FOR EACH ROW EXECUTE FUNCTION fail_job_update();
	`)
	_, updatePollErr := jobPoller.PollOnce(ctx)
	require.Error(t, updatePollErr)
	_, _ = kernel.DB().Exec(ctx, "DROP TRIGGER trig_fail_job_upd ON tasks.jobs")
	_, _ = kernel.DB().Exec(ctx, "DELETE FROM tasks.jobs WHERE name = 'enqueue-err-job'")

	// 9. Query due jobs error handling
	_, alterErr := kernel.DB().Exec(ctx, "ALTER TABLE tasks.jobs RENAME COLUMN next_run_at TO next_run_at_backup")
	require.NoError(t, alterErr)
	_, queryJobsErr := jobPoller.PollOnce(ctx)
	require.Error(t, queryJobsErr)
	require.Contains(t, queryJobsErr.Error(), "failed to query due jobs")
	_, restoreErr := kernel.DB().Exec(ctx, "ALTER TABLE tasks.jobs RENAME COLUMN next_run_at_backup TO next_run_at")
	require.NoError(t, restoreErr)
}
