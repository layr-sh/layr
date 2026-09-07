package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCoreRedisKVStoreIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 1. Boot Redis testcontainer using testcontainers-go/modules/redis
	redisContainer, err := tcredis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Skipf("docker not available: %v", err)
		return
	}
	defer func() { _ = redisContainer.Terminate(ctx) }()

	redisURI, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("failed to get redis connection string: %v", err)
	}

	// 2. Initialize RedisStore via NewKVStore
	config := DefaultConfig()
	config.KVStore = KVStoreConfig{
		Backend: "redis",
		URL:     redisURI,
	}
	SetLoadedConfig(config)
	defer UnloadConfig()

	kvStore, err := NewKVStore(ctx, nil)
	if err != nil {
		t.Fatalf("failed to initialize redis kv store: %v", err)
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

	// Set & Get
	if setErr := kvStore.Set(ctx, "redis:session:1", "data_123", 10*time.Minute); setErr != nil {
		t.Fatalf("failed to set redis key: %v", setErr)
	}
	value, getValErr := kvStore.Get(ctx, "redis:session:1")
	if getValErr != nil || value != "data_123" {
		t.Fatalf("expected 'data_123', got '%s', err: %v", value, getValErr)
	}

	// MSet & MGet
	if msetErr := kvStore.MSet(ctx, map[string]string{}, 0); msetErr != nil {
		t.Fatalf("expected nil on empty MSet: %v", msetErr)
	}
	emptyMGet, mgetErr := kvStore.MGet(ctx, []string{})
	if mgetErr != nil || len(emptyMGet) != 0 {
		t.Fatalf("expected empty map on empty MGet: %v", mgetErr)
	}
	if msetErr := kvStore.MSet(ctx, map[string]string{"rk1": "rv1", "rk2": "rv2"}, 10*time.Minute); msetErr != nil {
		t.Fatalf("failed to MSet redis: %v", msetErr)
	}
	multiGetResult, mgetErr := kvStore.MGet(ctx, []string{"rk1", "rk2", "rmissing"})
	if mgetErr != nil || multiGetResult["rk1"] != "rv1" || multiGetResult["rk2"] != "rv2" || len(multiGetResult) != 2 {
		t.Fatalf("unexpected redis MGet: %v, err: %v", multiGetResult, mgetErr)
	}

	// SetNX
	ok, setNxErr := kvStore.SetNX(ctx, "rnx:key", "first", 5*time.Minute)
	if setNxErr != nil || !ok {
		t.Fatalf("expected redis SetNX true for fresh key: ok=%v, err=%v", ok, setNxErr)
	}
	ok, setNxErr = kvStore.SetNX(ctx, "rnx:key", "second", 5*time.Minute)
	if setNxErr != nil || ok {
		t.Fatalf("expected redis SetNX false for existing key: ok=%v, err=%v", ok, setNxErr)
	}

	// Expire
	if expireErr := kvStore.Expire(ctx, "rnx:key", 10*time.Minute); expireErr != nil {
		t.Fatalf("expected redis Expire success: %v", expireErr)
	}
	if expireErr := kvStore.Expire(ctx, "rmissing_key", 10*time.Minute); !errors.Is(expireErr, ErrKVStoreKeyNotFound) {
		t.Fatalf("expected ErrKVStoreKeyNotFound on redis Expire missing: %v", expireErr)
	}

	// Increment
	count, incrErr := kvStore.Increment(ctx, "redis:rate:1", 5*time.Minute)
	if incrErr != nil || count != 1 {
		t.Fatalf("expected count 1, got %d, err: %v", count, incrErr)
	}
	count, incrErr = kvStore.Increment(ctx, "redis:rate:1", 0)
	if incrErr != nil || count != 2 {
		t.Fatalf("expected count 2, got %d, err: %v", count, incrErr)
	}

	// Delete
	if delErr := kvStore.Delete(ctx, "redis:session:1"); delErr != nil {
		t.Fatalf("failed to delete redis key: %v", delErr)
	}
	if _, getErr := kvStore.Get(ctx, "redis:session:1"); !errors.Is(getErr, ErrKVStoreKeyNotFound) {
		t.Fatalf("expected ErrKVStoreKeyNotFound after delete, got: %v", getErr)
	}

	// Closed Redis error branches
	_ = kvStore.Close()
	if _, closedErr := kvStore.Get(ctx, "k"); closedErr == nil {
		t.Fatal("expected error on Get with closed redis")
	}
	if _, closedErr := kvStore.MGet(ctx, []string{"k"}); closedErr == nil {
		t.Fatal("expected error on MGet with closed redis")
	}
	if closedErr := kvStore.Set(ctx, "k", "v", 0); closedErr == nil {
		t.Fatal("expected error on Set with closed redis")
	}
	if closedErr := kvStore.MSet(ctx, map[string]string{"k": "v"}, 0); closedErr == nil {
		t.Fatal("expected error on MSet with closed redis")
	}
	if _, closedErr := kvStore.SetNX(ctx, "k", "v", 0); closedErr == nil {
		t.Fatal("expected error on SetNX with closed redis")
	}
	if closedErr := kvStore.Expire(ctx, "k", 0); closedErr == nil {
		t.Fatal("expected error on Expire with closed redis")
	}
	if closedErr := kvStore.Delete(ctx, "k"); closedErr == nil {
		t.Fatal("expected error on Delete with closed redis")
	}
	if _, closedErr := kvStore.Increment(ctx, "k", 0); closedErr == nil {
		t.Fatal("expected error on Increment with closed redis")
	}
}
