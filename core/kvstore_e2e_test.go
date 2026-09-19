package core

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestCoreKVStoreLifecycleE2E(t *testing.T) {
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

	if err = db.RunMigrations(ctx, SystemDatabaseMigrations); err != nil {
		t.Fatalf("system migrations failed: %v", err)
	}

	// Initialize KVStore backed by DB
	config := DefaultConfig()
	config.KVStore = KVStoreConfig{Backend: "database"}
	SetLoadedConfig(config)
	defer UnloadConfig()

	kvStore, err := NewKVStore(ctx, db)
	if err != nil {
		t.Fatalf("failed to initialize KV store: %v", err)
	}
	defer func() { _ = kvStore.Close() }()

	// 1. Set key with expiration
	if err = kvStore.Set(ctx, "session:e2e-user", "session-data-token", 1*time.Minute); err != nil {
		t.Fatalf("failed to set key: %v", err)
	}

	// 2. Fetch and assert
	storedValue, err := kvStore.Get(ctx, "session:e2e-user")
	if err != nil || storedValue != "session-data-token" {
		t.Fatalf("expected 'session-data-token', got '%s', err: %v", storedValue, err)
	}

	// 3. Batch operations: MSet and MGet
	batchEntries := map[string]string{
		"config:feature_flags": "enabled",
		"config:maintenance":   "false",
	}
	if err = kvStore.MSet(ctx, batchEntries, 5*time.Minute); err != nil {
		t.Fatalf("failed to MSet batch entries: %v", err)
	}

	fetchedBatch, err := kvStore.MGet(ctx, []string{"config:feature_flags", "config:maintenance", "missing:key"})
	if err != nil {
		t.Fatalf("failed to MGet batch entries: %v", err)
	}
	if fetchedBatch["config:feature_flags"] != "enabled" || fetchedBatch["config:maintenance"] != "false" || len(fetchedBatch) != 2 {
		t.Fatalf("unexpected MGet result: %v", fetchedBatch)
	}

	// 4. Distributed lock simulation via SetNX
	lockAcquired, setNXErr := kvStore.SetNX(ctx, "lock:cron:sync", "node-1", 10*time.Second)
	if setNXErr != nil || !lockAcquired {
		t.Fatalf("expected first SetNX to acquire lock, ok=%v, err=%v", lockAcquired, setNXErr)
	}
	secondLockAttempt, secondLockErr := kvStore.SetNX(ctx, "lock:cron:sync", "node-2", 10*time.Second)
	if secondLockErr != nil || secondLockAttempt {
		t.Fatalf("expected second SetNX to fail acquiring existing lock, ok=%v, err=%v", secondLockAttempt, secondLockErr)
	}

	// 5. Atomically increment rate limit counter
	count, err := kvStore.Increment(ctx, "ratelimit:ip:127.0.0.1", 1*time.Minute)
	if err != nil || count != 1 {
		t.Fatalf("expected count 1, got %d, err: %v", count, err)
	}
	count, err = kvStore.Increment(ctx, "ratelimit:ip:127.0.0.1", 1*time.Minute)
	if err != nil || count != 2 {
		t.Fatalf("expected count 2, got %d, err: %v", count, err)
	}

	// 6. Expire update on existing key
	if err = kvStore.Expire(ctx, "session:e2e-user", 2*time.Minute); err != nil {
		t.Fatalf("failed to update expiry on session: %v", err)
	}

	// 7. Delete key and verify ErrKVStoreKeyNotFound
	if err = kvStore.Delete(ctx, "session:e2e-user"); err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}
	if _, getErr := kvStore.Get(ctx, "session:e2e-user"); !errors.Is(getErr, ErrKVStoreKeyNotFound) {
		t.Fatalf("expected ErrKVStoreKeyNotFound, got: %v", getErr)
	}

	// 8. Idempotent Close and post-pool-close behavior
	if err = kvStore.Close(); err != nil {
		t.Fatalf("expected clean close: %v", err)
	}
	if err = kvStore.Close(); err != nil {
		t.Fatalf("expected idempotent second close: %v", err)
	}

	db.Close()
	if _, getErr := kvStore.Get(ctx, "lock:cron:sync"); getErr == nil {
		t.Fatal("expected error on Get after db connection pool closed")
	}
}
