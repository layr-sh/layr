package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCoreEmbeddedDatabaseFullLifecycleAndMigrationsIntegration(t *testing.T) {
	temporaryDirectory := t.TempDir()
	dataDirectory := filepath.Join(temporaryDirectory, "data")

	embeddedDatabase := NewEmbeddedDatabase(dataDirectory)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 1. Start embedded postgres
	databaseURL, err := embeddedDatabase.Start(ctx)
	if err != nil {
		t.Skipf("embedded postgres binary download/start failed: %v", err)
		return
	}
	defer func() {
		_ = embeddedDatabase.Stop()
	}()

	if databaseURL == "" {
		t.Fatal("expected non-empty databaseURL")
	}

	// 2. Connect db
	db, err := NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect pgxpool to embedded db: %v", err)
	}
	defer db.Close()

	// 3. Test MigrateUp (All)
	if err := db.RunMigrations(ctx, SystemDatabaseMigrations); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// 4. Test Node Register and Heartbeat
	node := NewNode(db, "test-node", []string{"data", "auth"})
	if err := node.Register(ctx); err != nil {
		t.Fatalf("Node Register failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	node.Close()

	// 5. Test MigrateDown (Rollback to 0)
	if err := db.MigrateDown(ctx, SystemDatabaseMigrations, 0); err != nil {
		t.Fatalf("MigrateDown failed: %v", err)
	}

	// 6. Test MigrateDown with no tables (idempotent)
	if err := db.MigrateDown(ctx, SystemDatabaseMigrations, 0); err != nil {
		t.Fatalf("MigrateDown idempotent run failed: %v", err)
	}
}
