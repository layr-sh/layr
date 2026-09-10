package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCoreEmbeddedDatabaseLifecycleE2E(t *testing.T) {
	temporaryDirectory := t.TempDir()
	dataDirectory := filepath.Join(temporaryDirectory, "data")

	embeddedDatabase := NewEmbeddedDatabase(dataDirectory)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Provision and start embedded postgres 18+ daemon
	databaseURL, err := embeddedDatabase.Start(ctx)
	if err != nil {
		t.Skipf("embedded postgres start failed: %v", err)
		return
	}
	defer func() {
		_ = embeddedDatabase.Stop()
	}()

	db, err := NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create db connection pool: %v", err)
	}
	defer db.Close()

	// Apply foundational system migrations
	err = db.RunMigrations(ctx, SystemDatabaseMigrations)
	if err != nil {
		t.Fatalf("system migrations failed: %v", err)
	}

	// 1. Create initial console user (UUIDv7 generated automatically)
	var consoleUserID string
	err = db.QueryRow(ctx, `
		INSERT INTO console.users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id::text
	`, "console_user@layr.sh", "$argon2id$v=19$m=65536,t=3,p=4$dummyhash").Scan(&consoleUserID)
	if err != nil || consoleUserID == "" {
		t.Fatalf("failed to create console user: %v", err)
	}

	// 2. Simulate console user login session
	var sessionID string
	err = db.QueryRow(ctx, `
		INSERT INTO console.sessions (user_id, session_token_hash, ip_address, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, clock_timestamp() + interval '24 hours')
		RETURNING id::text
	`, consoleUserID, "hashed_token_value_abc", "127.0.0.1", "LayrConsole/1.0").Scan(&sessionID)
	if err != nil || sessionID == "" {
		t.Fatalf("failed to create console session: %v", err)
	}

	// 3. Record an event for console user action
	var eventID string
	err = db.QueryRow(ctx, `
		INSERT INTO core.events (type, action, resource_type, resource_id, actor_type, actor_id, payload, ip_address)
		VALUES ($1, $2, $3, $4, 'console_user', $5, $6::jsonb, $7)
		RETURNING id::text
	`, "core.service_account.created", "create", "service_account", "sa_01", consoleUserID, []byte(`{"name":"api-client"}`), "127.0.0.1").Scan(&eventID)
	if err != nil || eventID == "" {
		t.Fatalf("failed to record event: %v", err)
	}

	// 4. Create a service account tied to the console user
	var serviceAccountID string
	err = db.QueryRow(ctx, `
		INSERT INTO core.service_accounts (name, description, key_prefix, key_hash, console_user_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text
	`, "Backend Worker", "Background automation client", "a1b2c3d4", "hashed_secret_key", consoleUserID).Scan(&serviceAccountID)
	if err != nil || serviceAccountID == "" {
		t.Fatalf("failed to create service account: %v", err)
	}

	// 5. Register an event hook and record a delivery event
	var eventHookID string
	err = db.QueryRow(ctx, `
		INSERT INTO core.event_hooks (name, driver, http_target_url, event_types)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text
	`, "Slack Alerts", "http", "https://hooks.slack.com/services/test", []string{"user.created", "user.deleted"}).Scan(&eventHookID)
	if err != nil || eventHookID == "" {
		t.Fatalf("failed to create event hook: %v", err)
	}

	var deliveryID string
	err = db.QueryRow(ctx, `
		INSERT INTO core.event_hook_deliveries (event_hook_id, event_id, event_type, payload, http_response_status, is_delivered)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6)
		RETURNING id::text
	`, eventHookID, eventID, "user.created", []byte(`{"user_id":"123"}`), 200, true).Scan(&deliveryID)
	if err != nil || deliveryID == "" {
		t.Fatalf("failed to record event hook delivery: %v", err)
	}

	// 6. Test KV Store unlogged table operations
	_, err = db.Exec(ctx, `
		INSERT INTO core.kv_store (key, value, expires_at)
		VALUES ($1, $2, clock_timestamp() + interval '10 minutes')
	`, "session:active_tokens", []byte("sample_token_payload"))
	if err != nil {
		t.Fatalf("failed to insert kv item: %v", err)
	}

	var kvValue []byte
	err = db.QueryRow(ctx, "SELECT value FROM core.kv_store WHERE key = $1", "session:active_tokens").Scan(&kvValue)
	if err != nil || string(kvValue) != "sample_token_payload" {
		t.Fatalf("expected 'sample_token_payload', got %s, err: %v", string(kvValue), err)
	}

	// Multi-node cluster topology and worker lifecycle
	nodeRegistry := NewNodeRegistry(db, "primary-node-1", []string{"data", "auth", "storage", "console"})
	err = nodeRegistry.Register(ctx)
	if err != nil {
		t.Fatalf("primary node registration failed: %v", err)
	}

	secondNodeRegistry := NewNodeRegistry(db, "worker-node-2", []string{"scheduler", "notification"})
	err = secondNodeRegistry.Register(ctx)
	if err != nil {
		t.Fatalf("worker node registration failed: %v", err)
	}

	var registeredNodeCount int
	err = db.QueryRow(ctx, "SELECT count(*) FROM core.nodes").Scan(&registeredNodeCount)
	if err != nil || registeredNodeCount != 2 {
		t.Fatalf("expected 2 registered nodes in core.nodes, got %d, err: %v", registeredNodeCount, err)
	}

	// Graceful node worker decommissioning
	secondNodeRegistry.Close()
	secondNodeRegistry.Close() // Idempotent double close check

	err = db.QueryRow(ctx, "SELECT count(*) FROM core.nodes").Scan(&registeredNodeCount)
	if err != nil || registeredNodeCount != 1 {
		t.Fatalf("expected 1 remaining node after worker deregistration, got %d, err: %v", registeredNodeCount, err)
	}

	// Schema evolution upgrade and rollback flow
	evolutionDatabaseMigration := DatabaseMigration{
		Version:     2,
		Description: "Add custom domain items table",
		UpSQL: `
			CREATE TABLE IF NOT EXISTS core.custom_items (
				id UUID PRIMARY KEY DEFAULT uuidv7(),
				title TEXT NOT NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
			);
		`,
		DownSQL: `
			DROP TABLE IF EXISTS core.custom_items CASCADE;
		`,
	}

	fullMigrationsList := append(SystemDatabaseMigrations, evolutionDatabaseMigration)
	err = db.MigrateUp(ctx, fullMigrationsList, 2)
	if err != nil {
		t.Fatalf("migration upgrade failed: %v", err)
	}

	// Insert into new upgraded schema table
	var itemID string
	err = db.QueryRow(ctx, "INSERT INTO core.custom_items (title) VALUES ($1) RETURNING id::text", "Item 1").Scan(&itemID)
	if err != nil || itemID == "" {
		t.Fatalf("failed to insert into upgraded table: %v", err)
	}

	// Rollback migration to version 1
	err = db.MigrateDown(ctx, fullMigrationsList, 1)
	if err != nil {
		t.Fatalf("migration rollback to v1 failed: %v", err)
	}

	// Verify table is removed
	var tableExists bool
	err = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'core' AND table_name = 'custom_items'
		)
	`).Scan(&tableExists)
	if err != nil || tableExists {
		t.Fatalf("expected custom_items table to be dropped after rollback, tableExists=%v, err=%v", tableExists, err)
	}

	// Server shutdown, restart and data persistence verification
	nodeRegistry.Close()
	db.Close()

	err = embeddedDatabase.Stop()
	if err != nil {
		t.Fatalf("embedded postgres stop failed: %v", err)
	}

	// Idempotent Stop on already stopped embedded postgres
	err = embeddedDatabase.Stop()
	if err != nil {
		t.Fatalf("expected second Stop call to succeed idempotently, got: %v", err)
	}

	// Re-start embedded postgres from the existing data directory
	restartEmbeddedDatabase := NewEmbeddedDatabase(dataDirectory)
	reconnectedURL, err := restartEmbeddedDatabase.Start(ctx)
	if err != nil {
		t.Fatalf("failed to restart embedded postgres on existing data directory: %v", err)
	}
	defer func() {
		_ = restartEmbeddedDatabase.Stop()
	}()

	reconnectedDB, err := NewDatabasePool(ctx, reconnectedURL)
	if err != nil {
		t.Fatalf("failed to reconnect db to restarted database: %v", err)
	}
	defer reconnectedDB.Close()

	// Assert persisted console user survived restart
	var persistedEmail string
	err = reconnectedDB.QueryRow(ctx, "SELECT email FROM console.users WHERE id = $1", consoleUserID).Scan(&persistedEmail)
	if err != nil || persistedEmail != "console_user@layr.sh" {
		t.Fatalf("expected persisted user email 'console_user@layr.sh', got %s, err: %v", persistedEmail, err)
	}

	// Assert persisted service account survived restart
	var persistedServiceAccountName string
	err = reconnectedDB.QueryRow(ctx, "SELECT name FROM core.service_accounts WHERE id = $1", serviceAccountID).Scan(&persistedServiceAccountName)
	if err != nil || persistedServiceAccountName != "Backend Worker" {
		t.Fatalf("expected persisted service account name 'Backend Worker', got %s, err: %v", persistedServiceAccountName, err)
	}

	// Assert event hook and delivery survived restart
	var persistedEventHookName string
	err = reconnectedDB.QueryRow(ctx, "SELECT name FROM core.event_hooks WHERE id = $1", eventHookID).Scan(&persistedEventHookName)
	if err != nil || persistedEventHookName != "Slack Alerts" {
		t.Fatalf("expected persisted event hook 'Slack Alerts', got %s, err: %v", persistedEventHookName, err)
	}

	// Clean final shutdown
	reconnectedDB.Close()
	if err := restartEmbeddedDatabase.Stop(); err != nil {
		t.Fatalf("embedded postgres second stop failed: %v", err)
	}

	// Assert connection db ping fails after database is stopped
	{
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		if err := reconnectedDB.Ping(ctx); err == nil {
			t.Fatal("expected ping to fail on stopped database")
		}
	}
}
