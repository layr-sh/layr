// Package tasks provides distributed cron and background task orchestration.
package tasks

import "layr.sh/core"

func init() {
	for _, migration := range Migrations {
		log.Tracef("registering database migration v%d (%s)", migration.Version, migration.Description)
		core.RegisterDatabaseMigration(migration)
	}
}

// TasksDatabaseMigration defines the database schema for the tasks subsystem.
var TasksDatabaseMigration = core.DatabaseMigration{
	Service:     "tasks",
	Version:     1,
	Description: "Initialize tasks schema, dynamic config, jobs, executions, and execution logs",
	UpSQL: `
CREATE SCHEMA IF NOT EXISTS tasks;

-- 1. Dynamic Runtime Configuration
CREATE TABLE IF NOT EXISTS tasks.config (
    key VARCHAR(128) PRIMARY KEY,
    value JSONB NOT NULL,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- 2. Registered Recurring Cron Jobs
CREATE TABLE IF NOT EXISTS tasks.jobs (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name VARCHAR(128) UNIQUE NOT NULL,
    cron_expression VARCHAR(64) NOT NULL,
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
    target_type VARCHAR(32) NOT NULL,
    target_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_layr_jobs_enabled_next ON tasks.jobs(next_run_at) WHERE is_enabled = true;

-- 3. Execution Queue
CREATE TABLE IF NOT EXISTS tasks.executions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    job_id UUID REFERENCES tasks.jobs(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    run_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    locked_at TIMESTAMPTZ,
    locked_by VARCHAR(128),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_layr_executions_dispatch ON tasks.executions(status, run_at) WHERE status = 'pending';

-- 4. Historical Execution Audit Logs
CREATE TABLE IF NOT EXISTS tasks.execution_logs (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    job_id UUID REFERENCES tasks.jobs(id) ON DELETE SET NULL,
    execution_id UUID NOT NULL,
    status VARCHAR(32) NOT NULL,
    response_status_code INT,
    execution_duration_ms INT NOT NULL,
    response_body TEXT,
    error_message TEXT,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_layr_execution_logs_time ON tasks.execution_logs(job_id, executed_at DESC);
`,
	DownSQL: `
DROP SCHEMA IF EXISTS tasks CASCADE;
`,
}

// Migrations is the list of all tasks migrations.
var Migrations = []core.DatabaseMigration{
	TasksDatabaseMigration,
}
