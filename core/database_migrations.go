package core

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// DatabaseMigration represents a versioned SQL migration supporting Up and Down scripts.
type DatabaseMigration struct {
	Service     string
	Version     int
	Description string
	UpSQL       string
	DownSQL     string
}

var (
	registryMutex sync.Mutex
	registered    []DatabaseMigration
)

// RegisterDatabaseMigration adds a migration to the global db of migrations.
func RegisterDatabaseMigration(databaseMigration DatabaseMigration) {
	if databaseMigration.Service == "" {
		databaseMigration.Service = "core"
	}
	registryMutex.Lock()
	defer registryMutex.Unlock()
	registered = append(registered, databaseMigration)
}

func sortDatabaseMigrations(migrations []DatabaseMigration) {
	sort.Slice(migrations, func(indexI, indexJ int) bool {
		if migrations[indexI].Service != migrations[indexJ].Service {
			if migrations[indexI].Service == "core" {
				return true
			}
			if migrations[indexJ].Service == "core" {
				return false
			}
			return migrations[indexI].Service < migrations[indexJ].Service
		}
		return migrations[indexI].Version < migrations[indexJ].Version
	})
}

// GetRegisteredDatabaseMigrations returns all registered migrations deduplicated and sorted by service and version.
func GetRegisteredDatabaseMigrations() []DatabaseMigration {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	seen := make(map[string]bool)
	var registeredMigrations []DatabaseMigration
	for _, migration := range append(SystemDatabaseMigrations, registered...) {
		key := fmt.Sprintf("%s:%d", migration.Service, migration.Version)
		if !seen[key] {
			seen[key] = true
			registeredMigrations = append(registeredMigrations, migration)
		}
	}
	sortDatabaseMigrations(registeredMigrations)
	return registeredMigrations
}

// SystemDatabaseMigrations contains foundational core schema definitions for core and console.
var SystemDatabaseMigrations = []DatabaseMigration{
	{
		Service:     "core",
		Version:     1,
		Description: "Initialize core and console foundational schemas, migrations, nodes, users, and service accounts",
		UpSQL: `
CREATE SCHEMA IF NOT EXISTS core;
CREATE SCHEMA IF NOT EXISTS console;

CREATE TABLE IF NOT EXISTS core.migrations (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    service VARCHAR(64) NOT NULL DEFAULT 'core',
    version INT NOT NULL,
    description TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT core_migrations_service_version_key UNIQUE (service, version)
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

CREATE TABLE IF NOT EXISTS core.events (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    type TEXT NOT NULL,
    actor_id UUID,
    actor_type TEXT NOT NULL DEFAULT 'system',
    ip_address INET,
    user_agent TEXT,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_core_events_created_at ON core.events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_core_events_type ON core.events(type);
CREATE INDEX IF NOT EXISTS idx_core_events_actor ON core.events(actor_type, actor_id);
CREATE INDEX IF NOT EXISTS idx_core_events_resource ON core.events(resource_type, resource_id);

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

CREATE TABLE IF NOT EXISTS core.event_hooks (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name TEXT NOT NULL,
    driver TEXT NOT NULL,
    sql_function_name TEXT,
    http_target_url TEXT,
    http_encrypted_signing_secret TEXT,
    event_types TEXT[] NOT NULL DEFAULT '{}'::text[],
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    max_retries INT NOT NULL DEFAULT 3,
    timeout_seconds INT NOT NULL DEFAULT 10,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_core_event_hooks_is_enabled ON core.event_hooks(is_enabled);
CREATE INDEX IF NOT EXISTS idx_core_event_hooks_event_types ON core.event_hooks USING gin(event_types);

CREATE TABLE IF NOT EXISTS core.event_hook_deliveries (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    event_hook_id UUID NOT NULL REFERENCES core.event_hooks(id) ON DELETE CASCADE,
    event_id UUID NOT NULL REFERENCES core.events(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    http_response_status INT,
    result TEXT,
    error_message TEXT,
    attempt_count INT NOT NULL DEFAULT 1,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    is_delivered BOOLEAN NOT NULL DEFAULT false,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_core_event_hook_deliveries_hook_created ON core.event_hook_deliveries (event_hook_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_core_event_hook_deliveries_event_id ON core.event_hook_deliveries (event_id);
`,

		DownSQL: `
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
			service VARCHAR(64) NOT NULL DEFAULT 'core',
			version INT NOT NULL,
			description TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			CONSTRAINT core_migrations_service_version_key UNIQUE (service, version)
		);
	`)

	// Fetch applied migrations
	rows, _ := tx.Query(ctx, "SELECT service, version FROM core.migrations ORDER BY service ASC, version ASC")
	if rows != nil {
		defer rows.Close()
	}

	applied := make(map[string]bool)
	for rows.Next() {
		var appliedService string
		var appliedVersion int
		_ = rows.Scan(&appliedService, &appliedVersion)
		applied[fmt.Sprintf("%s:%d", appliedService, appliedVersion)] = true
	}
	log.Tracef("retrieved %d applied migration versions from core.migrations", len(applied))

	// Sort migrations in ascending order: "core" first, then service alphabetically, then version ascending
	sortedMigrations := make([]DatabaseMigration, len(migrations))
	copy(sortedMigrations, migrations)
	for index := range sortedMigrations {
		if sortedMigrations[index].Service == "" {
			sortedMigrations[index].Service = "core"
		}
	}
	sortDatabaseMigrations(sortedMigrations)

	log.Debugf("starting MigrateUp (available migrations: %d, target: %d)", len(migrations), targetVersion)
	for _, migration := range sortedMigrations {
		key := fmt.Sprintf("%s:%d", migration.Service, migration.Version)
		if applied[key] {
			log.Tracef("migration %s (%s) already applied, skipping", key, migration.Description)
			continue
		}
		if targetVersion > 0 && migration.Version > targetVersion {
			log.Tracef("migration %s exceeds target version %d, stopping MigrateUp", key, targetVersion)
			break
		}

		log.Infof("Applying Up migration %s: %s", key, migration.Description)
		log.Tracef("executing Up migration SQL for version %s", key)
		if _, execErr := tx.Exec(ctx, migration.UpSQL); execErr != nil {
			return fmt.Errorf("failed Up migration %s (%s): %w", key, migration.Description, execErr)
		}

		if _, insertErr := tx.Exec(ctx, "INSERT INTO core.migrations (service, version, description) VALUES ($1, $2, $3)", migration.Service, migration.Version, migration.Description); insertErr != nil {
			return fmt.Errorf("failed to record migration %s: %w", key, insertErr)
		}
		applied[key] = true
		log.Tracef("recorded migration %s in core.migrations", key)
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

	// Order applied migrations in reverse dependency order: non-core services first (descending), core last; version descending
	rows, _ := tx.Query(ctx, "SELECT service, version FROM core.migrations ORDER BY (CASE WHEN service = 'core' THEN 1 ELSE 0 END) ASC, service DESC, version DESC")
	if rows != nil {
		defer rows.Close()
	}

	type appliedMigrationEntry struct {
		Service string
		Version int
	}
	var appliedMigrations []appliedMigrationEntry
	for rows.Next() {
		var currentAppliedMigrationEntry appliedMigrationEntry
		_ = rows.Scan(&currentAppliedMigrationEntry.Service, &currentAppliedMigrationEntry.Version)
		appliedMigrations = append(appliedMigrations, currentAppliedMigrationEntry)
	}

	targetServices := make(map[string]bool)
	migrationMap := make(map[string]DatabaseMigration)
	for _, migration := range migrations {
		svc := migration.Service
		if svc == "" {
			svc = "core"
		}
		targetServices[svc] = true
		migrationMap[fmt.Sprintf("%s:%d", svc, migration.Version)] = migration
	}

	log.Debugf("starting MigrateDown (applied count: %d, target: %d)", len(appliedMigrations), targetVersion)
	for _, currentAppliedMigrationEntry := range appliedMigrations {
		key := fmt.Sprintf("%s:%d", currentAppliedMigrationEntry.Service, currentAppliedMigrationEntry.Version)
		if len(migrations) == 0 {
			return fmt.Errorf("cannot rollback migration %s: definition not found in registry", key)
		}
		if !targetServices[currentAppliedMigrationEntry.Service] {
			continue
		}

		databaseMigration, ok := migrationMap[key]
		if !ok {
			return fmt.Errorf("cannot rollback migration %s: definition not found in registry", key)
		}

		if targetVersion > 0 && currentAppliedMigrationEntry.Version <= targetVersion {
			log.Tracef("migration %s is at or below target %d, skipping MigrateDown", key, targetVersion)
			continue
		}

		if databaseMigration.DownSQL == "" {
			return fmt.Errorf("cannot rollback migration %s (%s): no DownSQL specified", key, databaseMigration.Description)
		}

		// If this is the base migration (core version 1) that drops core schema, don't execute DELETE afterward
		if databaseMigration.Service != "core" || databaseMigration.Version != 1 {
			if _, err := tx.Exec(ctx, "DELETE FROM core.migrations WHERE service = $1 AND version = $2", databaseMigration.Service, databaseMigration.Version); err != nil {
				return fmt.Errorf("failed to delete migration %s record: %w", key, err)
			}
		}

		log.Infof("Rolling back Down migration %s: %s", key, databaseMigration.Description)
		log.Tracef("executing Down rollback SQL for %s", key)
		if _, err := tx.Exec(ctx, databaseMigration.DownSQL); err != nil {
			return fmt.Errorf("failed Down migration %s (%s): %w", key, databaseMigration.Description, err)
		}
	}

	log.Debug("MigrateDown completed successfully, committing transaction")
	return tx.Commit(ctx)
}
