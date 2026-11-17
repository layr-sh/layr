package tasks

import (
	"context"
	"encoding/json"
	"testing"

	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksListFailuresIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	ctx := context.Background()
	jobManager := NewJobManager(kernel, NewConfigManager(kernel))
	job, err := jobManager.CreateJob(ctx, CreateJobInput{Name: "list-failure", CronExpression: "0 0 * * *", TargetType: TargetTypeSQL, TargetPayload: json.RawMessage(`{"query":"SELECT 1"}`)})
	require.NoError(t, err)
	_, err = kernel.DB().Exec(ctx, `UPDATE tasks.jobs SET next_run_at = clock_timestamp() - interval '1 hour' WHERE id = $1`, job.ID)
	require.NoError(t, err)
	t.Run("invalid job rows return scan errors", func(t *testing.T) {
		_, setupErr := kernel.DB().Exec(ctx, `ALTER TABLE tasks.jobs RENAME TO jobs_backup;
   CREATE VIEW tasks.jobs AS SELECT 'invalid-uuid'::text AS id, name, cron_expression, timezone, target_type, target_payload, is_enabled, last_run_at, next_run_at, created_at, last_updated_at FROM tasks.jobs_backup`)
		require.NoError(t, setupErr)
		defer func() {
			_, restoreErr := kernel.DB().Exec(ctx, `DROP VIEW tasks.jobs; ALTER TABLE tasks.jobs_backup RENAME TO jobs`)
			require.NoError(t, restoreErr)
		}()
		jobs, listErr := jobManager.ListJobs(ctx)
		require.ErrorContains(t, listErr, "failed to scan job")
		require.Nil(t, jobs)
		count, pollErr := NewJobPoller(kernel, NewConfigManager(kernel)).PollOnce(ctx)
		require.ErrorContains(t, pollErr, "failed to scan due job")
		require.Zero(t, count)
	})
	t.Run("job stream errors are not returned as partial success", func(t *testing.T) {
		_, setupErr := kernel.DB().Exec(ctx, `CREATE FUNCTION tasks.fail_list_name(text) RETURNS text LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected row failure'; END $$;
   ALTER TABLE tasks.jobs RENAME TO jobs_backup;
   CREATE VIEW tasks.jobs AS SELECT id, tasks.fail_list_name(name) AS name, cron_expression, timezone, target_type, target_payload, is_enabled, last_run_at, next_run_at, created_at, last_updated_at FROM tasks.jobs_backup`)
		require.NoError(t, setupErr)
		defer func() {
			_, restoreErr := kernel.DB().Exec(ctx, `DROP VIEW tasks.jobs; ALTER TABLE tasks.jobs_backup RENAME TO jobs; DROP FUNCTION tasks.fail_list_name(text)`)
			require.NoError(t, restoreErr)
		}()
		jobs, listErr := jobManager.ListJobs(ctx)
		require.ErrorContains(t, listErr, "failed to read jobs")
		require.Nil(t, jobs)
	})
	t.Run("invalid execution log JSON is rejected", func(t *testing.T) {
		_, setupErr := kernel.DB().Exec(ctx, `INSERT INTO tasks.execution_logs (execution_id, status, execution_duration_ms, executed_at) VALUES ($1, 'success', 0, clock_timestamp())`, uuid.NewV7())
		require.NoError(t, setupErr)
		_, setupErr = kernel.DB().Exec(ctx, `ALTER TABLE tasks.execution_logs RENAME TO execution_logs_backup;
   CREATE VIEW tasks.execution_logs AS SELECT id, job_id, execution_id, status, response_status_code, execution_duration_ms, response_body, error_message, 'not-a-timestamp'::text AS executed_at FROM tasks.execution_logs_backup`)
		require.NoError(t, setupErr)
		defer func() {
			_, restoreErr := kernel.DB().Exec(ctx, `DROP VIEW tasks.execution_logs; ALTER TABLE tasks.execution_logs_backup RENAME TO execution_logs`)
			require.NoError(t, restoreErr)
		}()
		logs, count, listErr := jobManager.ListExecutions(ctx, nil, 10, 0)
		require.ErrorContains(t, listErr, "failed to decode execution logs")
		require.Nil(t, logs)
		require.Zero(t, count)
	})
	t.Run("invalid DLQ JSON is rejected", func(t *testing.T) {
		_, setupErr := kernel.DB().Exec(ctx, `INSERT INTO tasks.executions (status, run_at) VALUES ('failed', clock_timestamp())`)
		require.NoError(t, setupErr)
		_, setupErr = kernel.DB().Exec(ctx, `ALTER TABLE tasks.executions RENAME TO executions_backup;
   CREATE VIEW tasks.executions AS SELECT id, job_id, status, run_at, payload, attempts, max_attempts, last_error, 'not-a-timestamp'::text AS created_at FROM tasks.executions_backup`)
		require.NoError(t, setupErr)
		defer func() {
			_, restoreErr := kernel.DB().Exec(ctx, `DROP VIEW tasks.executions; ALTER TABLE tasks.executions_backup RENAME TO executions`)
			require.NoError(t, restoreErr)
		}()
		items, count, listErr := jobManager.ListDLQ(ctx, 10, 0)
		require.ErrorContains(t, listErr, "failed to decode dlq items")
		require.Nil(t, items)
		require.Zero(t, count)
	})
}
