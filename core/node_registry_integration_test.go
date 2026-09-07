package core

import (
	"context"
	"testing"
	"time"
)

func TestCoreNodeRegistryHeartbeatTickIntegration(t *testing.T) {
	db, cleanup := startTestContainer(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	_ = db.RunMigrations(ctx, SystemDatabaseMigrations)

	// Create registry with fast heartbeat (50ms)
	nodeRegistry := NewNodeRegistry(db, "heartbeat-test", []string{"data"})
	nodeRegistry.heartbeatInterval = 50 * time.Millisecond

	if err := nodeRegistry.Register(ctx); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Wait for at least 2 heartbeat ticks to fire
	time.Sleep(200 * time.Millisecond)
	nodeRegistry.Close()

	// Wait for the goroutine to exit
	time.Sleep(100 * time.Millisecond)
}

func TestCoreNodeRegistryRegisterErrorIntegration(t *testing.T) {
	db, cleanup := startTestContainer(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Don't run migrations, so the nodes table doesn't exist
	nodeRegistry := NewNodeRegistry(db, "fail-node", []string{"data"})
	if err := nodeRegistry.Register(ctx); err == nil {
		t.Fatal("expected register failure on missing table")
	}
}

func TestCoreNodeRegistryStopCleanupIntegration(t *testing.T) {
	db, cleanup := startTestContainer(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	_ = db.RunMigrations(ctx, SystemDatabaseMigrations)

	// Register and immediately close (stopChannel path)
	nodeRegistry := NewNodeRegistry(db, "stop-test", []string{"auth"})
	nodeRegistry.heartbeatInterval = 50 * time.Millisecond
	if err := nodeRegistry.Register(ctx); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Close immediately without waiting for heartbeat (tests stopChannel race with ticker)
	nodeRegistry.Close()
	time.Sleep(100 * time.Millisecond)
}
