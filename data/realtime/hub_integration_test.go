package realtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"layr.sh/core"
)

func startRealtimeTestContainer(t *testing.T) (*core.DatabasePool, string, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	postgresContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr_rt_integration"),
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
		return nil, "", nil
	}

	databaseURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cancel()
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := core.NewDatabasePool(ctx, databaseURL)
	if err != nil {
		cancel()
		t.Fatalf("failed to connect pool: %v", err)
	}

	cleanup := func() {
		db.Close()
		_ = postgresContainer.Terminate(ctx)
		cancel()
	}

	return db, databaseURL, cleanup
}

func TestRealtimeHubPostgresIntegration(t *testing.T) {
	db, databaseURL, cleanup := startRealtimeTestContainer(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Initialize CDC schema, trigger function, and test tables
	_, err := db.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS data;

		CREATE OR REPLACE FUNCTION data.notify_cdc() RETURNS trigger AS $$
		DECLARE
			payload JSONB;
			rec_record JSONB;
			old_rec_record JSONB;
		BEGIN
			IF (TG_OP = 'DELETE') THEN
				rec_record := to_jsonb(OLD);
				old_rec_record := NULL;
			ELSIF (TG_OP = 'UPDATE') THEN
				rec_record := to_jsonb(NEW);
				old_rec_record := to_jsonb(OLD);
			ELSE
				rec_record := to_jsonb(NEW);
				old_rec_record := NULL;
			END IF;
			
			payload := json_build_object(
				'schema', TG_TABLE_SCHEMA,
				'table', TG_TABLE_NAME,
				'event', TG_OP,
				'commit_timestamp', clock_timestamp(),
				'record', rec_record,
				'old_record', old_rec_record
			);
			
			IF octet_length(payload::text) > 7800 THEN
				payload := json_build_object(
					'schema', TG_TABLE_SCHEMA,
					'table', TG_TABLE_NAME,
					'event', TG_OP,
					'commit_timestamp', clock_timestamp(),
					'record', jsonb_build_object('id', COALESCE(rec_record->>'id', rec_record->>'uuid')),
					'old_record', CASE WHEN old_rec_record IS NOT NULL THEN jsonb_build_object('id', COALESCE(old_rec_record->>'id', old_rec_record->>'uuid')) ELSE NULL END,
					'truncated', true
				);
			END IF;
			
			PERFORM pg_notify('cdc', payload::text);
			IF (TG_OP = 'DELETE') THEN
				RETURN OLD;
			ELSE
				RETURN NEW;
			END IF;
		END;
		$$ LANGUAGE plpgsql;

		CREATE TABLE IF NOT EXISTS public.hub_items (
			id UUID PRIMARY KEY DEFAULT uuidv7(),
			title TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("failed to setup CDC schema: %v", err)
	}

	hub := NewHub(db)

	// 1. EnsureTableTrigger on real PostgreSQL table
	if err = hub.EnsureTableTrigger(ctx, "public", "hub_items"); err != nil {
		t.Fatalf("failed to install trigger: %v", err)
	}

	// Repeated call uses in-memory cache
	if err = hub.EnsureTableTrigger(ctx, "public", "hub_items"); err != nil {
		t.Fatalf("failed on cached table trigger call: %v", err)
	}

	// EnsureTableTrigger with empty schema defaults to public
	if err = hub.EnsureTableTrigger(ctx, "", "hub_items"); err != nil {
		t.Fatalf("failed on empty schema trigger call: %v", err)
	}

	// EnsureTableTrigger error on non-existent table
	if err = hub.EnsureTableTrigger(ctx, "public", "nonexistent_table_for_error_test"); err == nil {
		t.Fatal("expected error on non-existent table trigger call")
	}

	// 2. Start Hub and verify LISTEN loop receives pg_notify events
	if err = hub.Start(ctx); err != nil {
		t.Fatalf("failed to start hub: %v", err)
	}
	// Second start is idempotent
	if err = hub.Start(ctx); err != nil {
		t.Fatalf("second start should be idempotent: %v", err)
	}

	time.Sleep(100 * time.Millisecond) // Allow LISTEN loop to establish connection

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	testClient := NewClient(hub, nil, 10)
	testClient.subscriptions["test_channel"] = Subscription{
		Channel: "test_channel",
		Schema:  "public",
		Table:   "hub_items",
		Event:   "INSERT",
	}
	hub.RegisterClient(testClient)

	go func() {
		defer waitGroup.Done()
		select {
		case <-testClient.sendChannel:
			// Received event
		case <-time.After(5 * time.Second):
			t.Errorf("timed out waiting for event on testClient")
		}
	}()

	// Trigger real database INSERT to produce cdc notification
	_, err = db.Exec(ctx, "INSERT INTO public.hub_items (title) VALUES ('Integration CDC Item')")
	if err != nil {
		t.Fatalf("failed to insert hub item: %v", err)
	}

	waitGroup.Wait()
	hub.Stop()

	// Test listenLoop cancellation via context
	cancelableCtx, hubCancel := context.WithCancel(ctx)
	hubCancel()
	canceledHub := NewHub(db)
	_ = canceledHub.Start(cancelableCtx)
	time.Sleep(20 * time.Millisecond)
	canceledHub.Stop()

	// Test listenLoop reconnection backoff on acquire failure
	closedDB, err := core.NewDatabasePool(ctx, databaseURL)
	if err == nil && closedDB != nil {
		closedDB.Close()
		reconnectHub := NewHub(closedDB)
		reconnectHub.initialBackoff = 5 * time.Millisecond
		reconnectHub.maxBackoff = 8 * time.Millisecond
		_ = reconnectHub.Start(ctx)
		time.Sleep(25 * time.Millisecond)
		reconnectHub.Stop()

		// Exercise cancellation while waiting in backoff
		reconnectCtx, reconnectCancel := context.WithCancel(ctx)
		canceledReconnectHub := NewHub(closedDB)
		canceledReconnectHub.initialBackoff = 200 * time.Millisecond
		_ = canceledReconnectHub.Start(reconnectCtx)
		time.Sleep(5 * time.Millisecond)
		reconnectCancel()
		canceledReconnectHub.Stop()
	}
}
