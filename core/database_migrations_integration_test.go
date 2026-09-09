package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCoreDatabaseMigrationsExecutionAndRollbackIntegration(t *testing.T) {
	temporaryDirectory := t.TempDir()
	dataDirectory := filepath.Join(temporaryDirectory, "data")

	embeddedDatabase := NewEmbeddedDatabase(dataDirectory)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

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

	// 1. Run migrations up
	if err := db.RunMigrations(ctx, SystemDatabaseMigrations); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// 2. Idempotent re-run
	if err := db.RunMigrations(ctx, SystemDatabaseMigrations); err != nil {
		t.Fatalf("RunMigrations rerun failed: %v", err)
	}

	// 3. Rollback
	if err := db.MigrateDown(ctx, SystemDatabaseMigrations, 0); err != nil {
		t.Fatalf("MigrateDown rollback failed: %v", err)
	}
}
