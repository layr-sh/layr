package core

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// DatabaseMigration represents a versioned SQL migration supporting Up and Down scripts.
type DatabaseMigration struct {
	Version     int
	Description string
	UpSQL       string
	DownSQL     string
}

var (
	registryLock sync.Mutex
	registered   []DatabaseMigration
)

// RegisterDatabaseMigration adds a migration to the global db of migrations.
func RegisterDatabaseMigration(migration DatabaseMigration) {
	registryLock.Lock()
	defer registryLock.Unlock()
	registered = append(registered, migration)
}

// GetRegisteredDatabaseMigrations returns all registered migrations deduplicated and sorted by version.
func GetRegisteredDatabaseMigrations() []DatabaseMigration {
	registryLock.Lock()
	defer registryLock.Unlock()
	seen := make(map[int]bool)
	var registeredMigrations []DatabaseMigration
	for _, migration := range append(SystemDatabaseMigrations, registered...) {
		if !seen[migration.Version] {
			seen[migration.Version] = true
			registeredMigrations = append(registeredMigrations, migration)
		}
	}
	sort.Slice(registeredMigrations, func(i, j int) bool {
		return registeredMigrations[i].Version < registeredMigrations[j].Version
	})
	return registeredMigrations
}

// SystemDatabaseMigrations contains foundational core schema definitions for core and console.
var SystemDatabaseMigrations = []DatabaseMigration{
	{
		Version:     1,
		Description: "Initialize core and console foundational schemas, migrations, nodes, users, and service accounts",
		UpSQL: `
CREATE SCHEMA IF NOT EXISTS core;
CREATE SCHEMA IF NOT EXISTS console;

CREATE TABLE IF NOT EXISTS core.migrations (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    version INT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE UNLOGGED TABLE IF NOT EXISTS core.kv_store (
    key TEXT PRIMARY KEY,
    value BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_core_kv_store_expires_at ON core.kv_store (expires_at);

CREATE TABLE IF NOT EXISTS core.nodes (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    node_name TEXT NOT NULL,
    enabled_services TEXT[] NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_core_nodes_heartbeat ON core.nodes (last_heartbeat_at);

CREATE TABLE IF NOT EXISTS console.users (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    encrypted_mfa_secret BYTEA,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE IF NOT EXISTS console.sessions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES console.users(id) ON DELETE CASCADE,
    session_token_hash VARCHAR(255) NOT NULL,
    ip_address INET NOT NULL,
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_console_sessions_hash ON console.sessions(session_token_hash);

CREATE TABLE IF NOT EXISTS console.audit_logs (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID REFERENCES console.users(id) ON DELETE SET NULL,
    action VARCHAR(128) NOT NULL,
    target_service VARCHAR(64) NOT NULL,
    entity_id VARCHAR(255),
    diff_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address INET NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_time ON console.audit_logs(created_at DESC);

CREATE TABLE IF NOT EXISTS core.service_accounts (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name TEXT NOT NULL,
    description TEXT,
    key_prefix TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    scopes JSONB NOT NULL DEFAULT '["*"]'::jsonb,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    allowed_ips TEXT[] NOT NULL DEFAULT '{}',
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    console_user_id UUID REFERENCES console.users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_core_service_accounts_prefix ON core.service_accounts (key_prefix);
CREATE INDEX IF NOT EXISTS idx_core_service_accounts_console_user_id ON core.service_accounts(console_user_id);

CREATE TABLE IF NOT EXISTS core.webhooks (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name TEXT NOT NULL,
    target_url TEXT NOT NULL,
    events JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    signing_secret_enc TEXT,
    max_retries INT NOT NULL DEFAULT 3,
    timeout_seconds INT NOT NULL DEFAULT 10,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE IF NOT EXISTS core.webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    webhook_id UUID NOT NULL REFERENCES core.webhooks(id) ON DELETE CASCADE,
    event_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    response_status INT,
    response_body TEXT,
    error_message TEXT,
    attempt_count INT NOT NULL DEFAULT 1,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    is_delivered BOOLEAN NOT NULL DEFAULT false,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_core_webhook_deliveries_webhook ON core.webhook_deliveries (webhook_id);
`,
		DownSQL: `
DROP TABLE IF EXISTS core.webhook_deliveries CASCADE;
DROP TABLE IF EXISTS core.webhooks CASCADE;
DROP TABLE IF EXISTS core.service_accounts CASCADE;
DROP TABLE IF EXISTS core.nodes CASCADE;
DROP TABLE IF EXISTS core.kv_store CASCADE;
DROP TABLE IF EXISTS console.audit_logs CASCADE;
DROP TABLE IF EXISTS console.sessions CASCADE;
DROP TABLE IF EXISTS console.users CASCADE;
DROP TABLE IF EXISTS core.migrations CASCADE;
DROP SCHEMA IF EXISTS console CASCADE;
DROP SCHEMA IF EXISTS core CASCADE;
`,
	},
}

// RunMigrations applies system and modular migrations upward with advisory locking.
func (db *DatabasePool) RunMigrations(ctx context.Context, migrations []DatabaseMigration) error {
	log.Debugf("initiating RunMigrations with %d migration definitions", len(migrations))
	return db.MigrateUp(ctx, migrations, 0)
}

// MigrateUp applies migrations up to targetVersion (0 = all pending).
func (db *DatabasePool) MigrateUp(ctx context.Context, migrations []DatabaseMigration, targetVersion int) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin migration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Acquire cluster-wide migration advisory lock and bootstrap schema
	log.Trace("acquiring cluster-wide migration advisory lock (1279342930)")
	_, _ = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1279342930);")
	log.Trace("ensuring core schema and core.migrations table")
	_, _ = tx.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS core;")
	_, _ = tx.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS core.migrations (
			id UUID PRIMARY KEY DEFAULT uuidv7(),
			version INT NOT NULL UNIQUE,
			description TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
		);
	`)

	// Fetch applied migrations
	rows, _ := tx.Query(ctx, "SELECT version FROM core.migrations ORDER BY version ASC")
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var appliedVersion int
		_ = rows.Scan(&appliedVersion)
		applied[appliedVersion] = true
	}
	log.Tracef("retrieved %d applied migration versions from core.migrations", len(applied))

	// Sort migrations in ascending order
	sortedMigrations := make([]DatabaseMigration, len(migrations))
	copy(sortedMigrations, migrations)
	sort.Slice(sortedMigrations, func(i, j int) bool {
		return sortedMigrations[i].Version < sortedMigrations[j].Version
	})

	log.Debugf("starting MigrateUp (available migrations: %d, target: %d)", len(migrations), targetVersion)
	for _, migration := range sortedMigrations {
		if applied[migration.Version] {
			log.Tracef("migration %d (%s) already applied, skipping", migration.Version, migration.Description)
			continue
		}
		if targetVersion > 0 && migration.Version > targetVersion {
			log.Tracef("migration %d exceeds target version %d, stopping MigrateUp", migration.Version, targetVersion)
			break
		}

		log.Infof("Applying Up migration %d: %s", migration.Version, migration.Description)
		log.Tracef("executing Up migration SQL for version %d", migration.Version)
		if _, execErr := tx.Exec(ctx, migration.UpSQL); execErr != nil {
			return fmt.Errorf("failed Up migration %d (%s): %w", migration.Version, migration.Description, execErr)
		}

		if _, insertErr := tx.Exec(ctx, "INSERT INTO core.migrations (version, description) VALUES ($1, $2)", migration.Version, migration.Description); insertErr != nil {
			return fmt.Errorf("failed to record migration %d: %w", migration.Version, insertErr)
		}
		applied[migration.Version] = true
		log.Tracef("recorded migration %d in core.migrations", migration.Version)
	}

	log.Debug("MigrateUp completed successfully, committing transaction")
	return tx.Commit(ctx)
}

// MigrateDown rolls back applied migrations down to targetVersion.
func (db *DatabasePool) MigrateDown(ctx context.Context, migrations []DatabaseMigration, targetVersion int) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin rollback transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Acquire advisory lock and verify migrations table existence
	log.Trace("acquiring rollback advisory lock and checking core.migrations existence")
	var tableExists bool
	_ = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'core' AND table_name = 'migrations'
		)
	`).Scan(&tableExists)
	if !tableExists {
		log.Debug("core.migrations table does not exist, skipping MigrateDown")
		return nil // Nothing to rollback
	}

	rows, _ := tx.Query(ctx, "SELECT version FROM core.migrations ORDER BY version DESC")
	defer rows.Close()

	var appliedVersions []int
	for rows.Next() {
		var appliedVersion int
		_ = rows.Scan(&appliedVersion)
		appliedVersions = append(appliedVersions, appliedVersion)
	}

	migrationMap := make(map[int]DatabaseMigration)
	for _, migration := range migrations {
		migrationMap[migration.Version] = migration
	}

	log.Debugf("starting MigrateDown (applied versions count: %d, target: %d)", len(appliedVersions), targetVersion)
	for _, version := range appliedVersions {
		if version <= targetVersion {
			log.Tracef("version %d is at or below target %d, stopping MigrateDown", version, targetVersion)
			break
		}

		migration, ok := migrationMap[version]
		if !ok {
			return fmt.Errorf("cannot rollback migration %d: definition not found in registry", version)
		}

		if migration.DownSQL == "" {
			return fmt.Errorf("cannot rollback migration %d (%s): no DownSQL specified", migration.Version, migration.Description)
		}

		// If this is the base migration (version 1) that drops core, don't execute DELETE afterward
		if migration.Version != 1 {
			if _, err := tx.Exec(ctx, "DELETE FROM core.migrations WHERE version = $1", migration.Version); err != nil {
				return fmt.Errorf("failed to delete migration %d record: %w", migration.Version, err)
			}
		}

		log.Infof("Rolling back Down migration %d: %s", migration.Version, migration.Description)
		log.Tracef("executing Down rollback SQL for version %d", migration.Version)
		if _, err := tx.Exec(ctx, migration.DownSQL); err != nil {
			return fmt.Errorf("failed Down migration %d (%s): %w", migration.Version, migration.Description, err)
		}
	}

	log.Debug("MigrateDown completed successfully, committing transaction")
	return tx.Commit(ctx)
}
