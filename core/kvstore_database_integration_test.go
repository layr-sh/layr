package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCoreDatabaseKVStoreIntegration(t *testing.T) {
	ctx := context.Background()

	// 1. Boot Postgres testcontainer
	pgContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("layr"),
		postgres.WithUsername("layr"),
		postgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Skipf("docker not available: %v", err)
		return
	}
	defer func() { _ = pgContainer.Terminate(ctx) }()

	databaseURL, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create db connection pool: %v", err)
	}
	defer db.Close()

	if migrationErr := db.RunMigrations(ctx, SystemDatabaseMigrations); migrationErr != nil {
		t.Fatalf("failed to apply migrations: %v", migrationErr)
	}

	// 2. Initialize DatabaseKVStore via NewKVStore (default backend)
	config := DefaultConfig()
	SetLoadedConfig(config)
	defer UnloadConfig()

	kvStore, err := NewKVStore(ctx, db)
	if err != nil {
		t.Fatalf("failed to initialize db kv store: %v", err)
	}
	defer func() { _ = kvStore.Close() }()

	// Ping
	if pingErr := kvStore.Ping(ctx); pingErr != nil {
		t.Fatalf("expected ping success: %v", pingErr)
	}

	// Get Nonexistent Key -> ErrKVStoreKeyNotFound
	if _, getErr := kvStore.Get(ctx, "missing"); !errors.Is(getErr, ErrKVStoreKeyNotFound) {
		t.Fatalf("expected ErrKVStoreKeyNotFound, got: %v", getErr)
	}

	// Set with default TTL & Get
	if setErr := kvStore.Set(ctx, "session:1", "user_123", 0); setErr != nil {
		t.Fatalf("failed to set key: %v", setErr)
	}
	value, getValErr := kvStore.Get(ctx, "session:1")
	if getValErr != nil || value != "user_123" {
		t.Fatalf("expected 'user_123', got '%s', err: %v", value, getValErr)
	}

	// Overwrite Set
	if setErr := kvStore.Set(ctx, "session:1", "user_456", 5*time.Second); setErr != nil {
		t.Fatalf("failed to overwrite key: %v", setErr)
	}
	value, getValErr = kvStore.Get(ctx, "session:1")
	if getValErr != nil || value != "user_456" {
		t.Fatalf("expected 'user_456', got '%s', err: %v", value, getValErr)
	}

	// MSet & MGet
	if msetErr := kvStore.MSet(ctx, map[string]string{}, 0); msetErr != nil {
		t.Fatalf("expected nil on empty MSet: %v", msetErr)
	}
	emptyMGet, mgetErr := kvStore.MGet(ctx, []string{})
	if mgetErr != nil || len(emptyMGet) != 0 {
		t.Fatalf("expected empty map on empty MGet: %v", mgetErr)
	}
	if msetErr := kvStore.MSet(ctx, map[string]string{"m1": "v1", "m2": "v2"}, 0); msetErr != nil {
		t.Fatalf("failed to MSet: %v", msetErr)
	}
	multiGetResult, mgetErr := kvStore.MGet(ctx, []string{"m1", "m2", "missing"})
	if mgetErr != nil || multiGetResult["m1"] != "v1" || multiGetResult["m2"] != "v2" || len(multiGetResult) != 2 {
		t.Fatalf("unexpected MGet result: %v, err: %v", multiGetResult, mgetErr)
	}

	// SetNX
	ok, setNxErr := kvStore.SetNX(ctx, "nx:key", "first", 0)
	if setNxErr != nil || !ok {
		t.Fatalf("expected SetNX true for fresh key: ok=%v, err=%v", ok, setNxErr)
	}
	ok, setNxErr = kvStore.SetNX(ctx, "nx:key", "second", 10*time.Second)
	if setNxErr != nil || ok {
		t.Fatalf("expected SetNX false for existing key: ok=%v, err=%v", ok, setNxErr)
	}

	// Expire
	if expireErr := kvStore.Expire(ctx, "nx:key", 0); expireErr != nil {
		t.Fatalf("expected Expire success: %v", expireErr)
	}
	if expireErr := kvStore.Expire(ctx, "missing_expire", 10*time.Second); !errors.Is(expireErr, ErrKVStoreKeyNotFound) {
		t.Fatalf("expected ErrKVStoreKeyNotFound on Expire missing: %v", expireErr)
	}

	// Increment Counter (fresh key)
	count, incrErr := kvStore.Increment(ctx, "rate:login:ip1", 0)
	if incrErr != nil || count != 1 {
		t.Fatalf("expected count 1, got %d, err: %v", count, incrErr)
	}
	// Increment Counter again
	count, incrErr = kvStore.Increment(ctx, "rate:login:ip1", 10*time.Second)
	if incrErr != nil || count != 2 {
		t.Fatalf("expected count 2, got %d, err: %v", count, incrErr)
	}

	// Delete Key
	if delErr := kvStore.Delete(ctx, "session:1"); delErr != nil {
		t.Fatalf("failed to delete key: %v", delErr)
	}
	if _, getErr := kvStore.Get(ctx, "session:1"); !errors.Is(getErr, ErrKVStoreKeyNotFound) {
		t.Fatalf("expected ErrKVStoreKeyNotFound after delete, got: %v", getErr)
	}

	// Fast expiration & Sweeper loop test
	databaseKVStore := NewDatabaseKVStore(ctx, db, 10*time.Millisecond)
	_ = databaseKVStore.Set(ctx, "expiring_key", "bye", 10*time.Millisecond)
	time.Sleep(60 * time.Millisecond)

	// Get expired key -> ErrKVStoreKeyNotFound
	if _, getErr := databaseKVStore.Get(ctx, "expiring_key"); !errors.Is(getErr, ErrKVStoreKeyNotFound) {
		t.Fatalf("expected ErrKVStoreKeyNotFound on expired key, got: %v", getErr)
	}

	// Manual Sweep
	pruned, sweepErr := databaseKVStore.Sweep(ctx)
	if sweepErr != nil {
		t.Fatalf("unexpected error on sweep: %v", sweepErr)
	}
	_ = pruned
	_ = databaseKVStore.Close()

	// Test NewDatabaseKVStore with 0 sweepInterval
	zeroSweepStore := NewDatabaseKVStore(ctx, db, 0)
	_ = zeroSweepStore.Close()

	// Closed DB branches
	db.Close()
	if _, closedGetErr := kvStore.Get(ctx, "k"); closedGetErr == nil {
		t.Fatal("expected error on Get with closed db connection pool")
	}
	if _, closedMGetErr := kvStore.MGet(ctx, []string{"k"}); closedMGetErr == nil {
		t.Fatal("expected error on MGet with closed db connection pool")
	}
	if closedSetErr := kvStore.Set(ctx, "k", "v", 0); closedSetErr == nil {
		t.Fatal("expected error on Set with closed db connection pool")
	}
	if closedMSetErr := kvStore.MSet(ctx, map[string]string{"k": "v"}, 0); closedMSetErr == nil {
		t.Fatal("expected error on MSet with closed db connection pool")
	}
	if _, closedSetNxErr := kvStore.SetNX(ctx, "k", "v", 0); closedSetNxErr == nil {
		t.Fatal("expected error on SetNX with closed db connection pool")
	}
	if closedExpireErr := kvStore.Expire(ctx, "k", 0); closedExpireErr == nil {
		t.Fatal("expected error on Expire with closed db connection pool")
	}
	if closedDeleteErr := kvStore.Delete(ctx, "k"); closedDeleteErr == nil {
		t.Fatal("expected error on Delete with closed db connection pool")
	}
	if _, closedIncrErr := kvStore.Increment(ctx, "k", 0); closedIncrErr == nil {
		t.Fatal("expected error on Increment with closed db connection pool")
	}
	databaseKVStoreInstance := kvStore.(*DatabaseKVStore)
	if _, closedSweepErr := databaseKVStoreInstance.Sweep(ctx); closedSweepErr == nil {
		t.Fatal("expected error on Sweep with closed db connection pool")
	}

	// Double close is safe
	_ = kvStore.Close()
	_ = kvStore.Close()
}
