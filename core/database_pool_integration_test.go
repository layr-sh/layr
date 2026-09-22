package core

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startTestContainer(t *testing.T) (*DatabasePool, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	postgresContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		cancel()
		t.Skipf("docker not available: %v", err)
		return nil, nil
	}

	databaseURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cancel()
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := NewDatabasePool(ctx, databaseURL, DatabasePoolOptions{
		MaxConns:            10,
		MinConns:            1,
		ConnectionTimeoutMs: 5000,
		MaxConnLifetime:     10 * time.Minute,
		MaxConnIdleTime:     2 * time.Minute,
		HealthCheckPeriod:   10 * time.Second,
	})
	if err != nil {
		cancel()
		t.Fatalf("failed to create db connection pool: %v", err)
	}

	cleanup := func() {
		db.Close()
		_ = postgresContainer.Terminate(ctx)
		cancel()
	}

	return db, cleanup
}

func TestCoreDatabasePoolMigrateUpFullCoverageIntegration(t *testing.T) {
	db, cleanup := startTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// 1. RunMigrations (MigrateUp all)
	if err := db.RunMigrations(ctx, SystemDatabaseMigrations); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// 2. MigrateUp idempotent (already applied, continue branch)
	if err := db.MigrateUp(ctx, SystemDatabaseMigrations, 0); err != nil {
		t.Fatalf("MigrateUp idempotent failed: %v", err)
	}

	// 3. MigrateUp with targetVersion (break branch when m.Version > targetVersion)
	custom := []DatabaseMigration{
		{Version: 5, Description: "v5", UpSQL: "SELECT 1;", DownSQL: "SELECT 1;"},
		{Version: 6, Description: "v6", UpSQL: "SELECT 1;", DownSQL: "SELECT 1;"},
	}
	// Apply only v5, v6 should be skipped (break)
	if err := db.MigrateUp(ctx, custom, 5); err != nil {
		t.Fatalf("MigrateUp target=5 failed: %v", err)
	}

	// 4. MigrateUp with bad SQL (exec error)
	invalidMigrations := []DatabaseMigration{{Version: 99, Description: "bad", UpSQL: "INVALID SQL;", DownSQL: "SELECT 1;"}}
	if err := db.MigrateUp(ctx, invalidMigrations, 0); err == nil {
		t.Fatal("expected error on bad SQL migration")
	}

	// 5. MigrateUp with canceled context (tx.Begin error)
	{
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel() // immediately cancel
		if err := db.MigrateUp(canceledCtx, SystemDatabaseMigrations, 0); err == nil {
			t.Fatal("expected error on canceled context Begin")
		}
	}

	// cleanup
	_ = db.MigrateDown(ctx, append(SystemDatabaseMigrations, custom...), 0)
}

func TestCoreDatabasePoolMigrateDownFullCoverageIntegration(t *testing.T) {
	db, cleanup := startTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Setup: Apply system + custom migrations
	_ = db.RunMigrations(ctx, SystemDatabaseMigrations)
	custom := []DatabaseMigration{
		{Version: 2, Description: "v2", UpSQL: "SELECT 1;", DownSQL: "SELECT 1;"},
		{Version: 3, Description: "v3", UpSQL: "SELECT 1;", DownSQL: "SELECT 1;"},
	}
	_ = db.MigrateUp(ctx, custom, 0)
	allMigrations := append(SystemDatabaseMigrations, custom...)

	// 1. MigrateDown partial (v <= targetVersion break branch)
	if err := db.MigrateDown(ctx, allMigrations, 2); err != nil {
		t.Fatalf("MigrateDown to 2 failed: %v", err)
	}

	// 2. MigrateDown further (DELETE record for v != 1)
	if err := db.MigrateDown(ctx, allMigrations, 1); err != nil {
		t.Fatalf("MigrateDown to 1 failed: %v", err)
	}

	// 3. MigrateDown version 1 (special v==1 branch: skip DELETE, execute DownSQL with DROP SCHEMA)
	if err := db.MigrateDown(ctx, allMigrations, 0); err != nil {
		t.Fatalf("MigrateDown to 0 failed: %v", err)
	}

	// 4. MigrateDown on empty schema (tableExists=false branch)
	if err := db.MigrateDown(ctx, allMigrations, 0); err != nil {
		t.Fatalf("MigrateDown on empty schema failed: %v", err)
	}

	// 5. MigrateDown with missing definition
	_ = db.RunMigrations(ctx, SystemDatabaseMigrations)
	if err := db.MigrateDown(ctx, []DatabaseMigration{}, 0); err == nil {
		t.Fatal("expected error on missing definition")
	}

	// 6. MigrateDown with empty DownSQL
	noDown := []DatabaseMigration{{Version: 9991, Description: "nodown", UpSQL: "SELECT 1;", DownSQL: ""}}
	_ = db.MigrateUp(ctx, append(SystemDatabaseMigrations, noDown...), 0)
	if err := db.MigrateDown(ctx, append(SystemDatabaseMigrations, noDown...), 9990); err == nil {
		t.Fatal("expected error on empty DownSQL")
	}
	_, _ = db.Exec(ctx, "DELETE FROM core.migrations WHERE version = 9991")

	// 7. MigrateDown with bad DownSQL (exec error)
	badDown := []DatabaseMigration{{Version: 9992, Description: "bad", UpSQL: "SELECT 1;", DownSQL: "INVALID SQL STATEMENT;"}}
	_ = db.MigrateUp(ctx, append(SystemDatabaseMigrations, badDown...), 0)
	if err := db.MigrateDown(ctx, append(SystemDatabaseMigrations, badDown...), 9990); err == nil {
		t.Fatal("expected error on bad DownSQL")
	}
	_, _ = db.Exec(ctx, "DELETE FROM core.migrations WHERE version = 9992")

	// 8. MigrateDown with canceled context (tx.Begin error)
	{
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		if err := db.MigrateDown(canceledCtx, allMigrations, 0); err == nil {
			t.Fatal("expected error on canceled context Begin")
		}
	}

	// cleanup
	_ = db.MigrateDown(ctx, SystemDatabaseMigrations, 0)
}

func TestCoreDatabasePoolMigrateUpInsertRecordErrorIntegration(t *testing.T) {
	db, cleanup := startTestContainer(t)
	defer cleanup()

	ctx := context.Background()
	_ = db.RunMigrations(ctx, SystemDatabaseMigrations)

	// A migration that manually pre-inserts its version in UpSQL to cause INSERT record failure
	conflicting := []DatabaseMigration{
		{
			Version:     2,
			Description: "conflict",
			UpSQL:       "INSERT INTO core.migrations (version, description) VALUES (2, 'manual');",
			DownSQL:     "SELECT 1;",
		},
	}

	err := db.MigrateUp(ctx, conflicting, 0)
	if err == nil {
		t.Fatal("expected error on duplicate migration record insertion")
	}
}

func TestCoreDatabasePoolMigrateDownDeleteRecordErrorIntegration(t *testing.T) {
	db, cleanup := startTestContainer(t)
	defer cleanup()

	ctx := context.Background()
	_ = db.RunMigrations(ctx, SystemDatabaseMigrations)

	// Apply migration 2
	migration2 := []DatabaseMigration{
		{Version: 2, Description: "v2", UpSQL: "SELECT 1;", DownSQL: "SELECT 1;"},
	}
	_ = db.MigrateUp(ctx, migration2, 0)

	// Install a trigger on core.migrations that prevents DELETE
	_, err := db.Exec(ctx, `
		CREATE OR REPLACE FUNCTION core.block_delete() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'delete blocked for test';
		END;
		$$ LANGUAGE plpgsql;

		CREATE TRIGGER trg_block_delete BEFORE DELETE ON core.migrations
		FOR EACH ROW EXECUTE FUNCTION core.block_delete();
	`)
	if err != nil {
		t.Fatalf("failed to create trigger: %v", err)
	}

	// MigrateDown to 1 should attempt to DELETE version 2 and fail
	err = db.MigrateDown(ctx, append(SystemDatabaseMigrations, migration2...), 1)
	if err == nil {
		t.Fatal("expected error on blocked DELETE in MigrateDown")
	}
}
