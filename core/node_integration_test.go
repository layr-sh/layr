package core

import (
	"context"
	"testing"
	"time"
)

func TestCoreNodeHeartbeatTickIntegration(t *testing.T) {
	db, cleanup := startTestContainer(t)
	defer cleanup()

	ctx := context.Background()
	_ = db.RunMigrations(ctx, SystemDatabaseMigrations)

	// Create registry with fast heartbeat (50ms) and reaper (75ms)
	nodeManager := NewNodeManager(db, "heartbeat-test", []string{"data"})
	nodeManager.heartbeatInterval = 50 * time.Millisecond
	nodeManager.reaperInterval = 75 * time.Millisecond

	if err := nodeManager.Register(ctx); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Wait for at least 2 heartbeat ticks to fire
	time.Sleep(200 * time.Millisecond)
	nodeManager.Close()

	// Wait for the goroutine to exit
	time.Sleep(100 * time.Millisecond)
}

func TestCoreNodeRegisterErrorIntegration(t *testing.T) {
	db, cleanup := startTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Don't run migrations, so the nodes table doesn't exist
	nodeManager := NewNodeManager(db, "fail-node", []string{"data"})
	if err := nodeManager.Register(ctx); err == nil {
		t.Fatal("expected register failure on missing table")
	}
}

func TestCoreNodeStopCleanupIntegration(t *testing.T) {
	db, cleanup := startTestContainer(t)
	defer cleanup()

	ctx := context.Background()
	_ = db.RunMigrations(ctx, SystemDatabaseMigrations)

	// Register and immediately close (stopChannel path)
	nodeManager := NewNodeManager(db, "stop-test", []string{"auth"})
	nodeManager.heartbeatInterval = 50 * time.Millisecond
	if err := nodeManager.Register(ctx); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Close immediately without waiting for heartbeat (tests stopChannel race with ticker)
	nodeManager.Close()
	time.Sleep(100 * time.Millisecond)
}

func TestCoreNodeHeartbeatAndReaperErrorIntegration(t *testing.T) {
	db, cleanup := startTestContainer(t)
	defer cleanup()

	ctx := context.Background()
	_ = db.RunMigrations(ctx, SystemDatabaseMigrations)

	nodeManager := NewNodeManager(db, "error-heartbeat-node", []string{"data"})
	nodeManager.heartbeatInterval = 20 * time.Millisecond
	nodeManager.reaperInterval = 20 * time.Millisecond

	if err := nodeManager.Register(ctx); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Close database connection pool to force heartbeat, reaper, and unregister queries to fail
	db.Close()

	// Wait for ticker to fire and encounter error on heartbeat update and reaper
	time.Sleep(100 * time.Millisecond)

	// Close node to trigger unregister error logging with closed db
	nodeManager.Close()
}

func TestCoreNodeEventBusIntegration(t *testing.T) {
	db, cleanup := startTestContainer(t)
	defer cleanup()

	ctx := context.Background()
	_ = db.RunMigrations(ctx, SystemDatabaseMigrations)

	eventBus := NewEventBus(db, nil)
	defer eventBus.Close()

	var registeredReceived, unregisteredReceived bool
	eventBus.Subscribe("core.node.registered", func(eventCtx context.Context, event Event) error {
		if event.Type == "core.node.registered" && event.Data["node_name"] == "event-node" {
			registeredReceived = true
		}
		return nil
	})
	eventBus.Subscribe("core.node.unregistered", func(eventCtx context.Context, event Event) error {
		if event.Type == "core.node.unregistered" && event.Data["node_name"] == "event-node" {
			unregisteredReceived = true
		}
		return nil
	})

	nodeManager := NewNodeManager(db, "event-node", []string{"data", "auth"}).WithEventBus(eventBus)
	nodeManager.heartbeatInterval = 50 * time.Millisecond
	if err := nodeManager.Register(ctx); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if !registeredReceived {
		t.Fatal("expected core.node.registered event to be received")
	}

	nodeManager.Close()
	time.Sleep(100 * time.Millisecond)
	if !unregisteredReceived {
		t.Fatal("expected core.node.unregistered event to be received")
	}
}
