package core

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCoreKVStoreBackendResolutionIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 1. Boot Postgres testcontainer
	postgresContainer, err := postgres.Run(ctx,
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
	defer func() { _ = postgresContainer.Terminate(ctx) }()

	databaseURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create db connection pool: %v", err)
	}
	defer db.Close()

	if err = db.RunMigrations(ctx, SystemDatabaseMigrations); err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	// 2. Resolve default backend ("") -> DatabaseStore
	config := DefaultConfig()
	config.KVStore = KVStoreConfig{Backend: ""}
	SetLoadedConfig(config)
	defer UnloadConfig()

	defaultKVStore, err := NewKVStore(ctx, db)
	if err != nil {
		t.Fatalf("failed to resolve default database kv store: %v", err)
	}
	defer func() { _ = defaultKVStore.Close() }()

	if err = defaultKVStore.Ping(ctx); err != nil {
		t.Fatalf("expected default store ping success: %v", err)
	}

	// 3. Resolve explicit database backend ("database") -> DatabaseStore
	config.KVStore = KVStoreConfig{Backend: "database"}
	SetLoadedConfig(config)

	databaseKVStore, err := NewKVStore(ctx, db)
	if err != nil {
		t.Fatalf("failed to resolve explicit database kv store: %v", err)
	}
	defer func() { _ = databaseKVStore.Close() }()

	if err = databaseKVStore.Set(ctx, "common:key", "val-db", time.Minute); err != nil {
		t.Fatalf("failed to set key on database store: %v", err)
	}
	databaseValue, err := databaseKVStore.Get(ctx, "common:key")
	if err != nil || databaseValue != "val-db" {
		t.Fatalf("expected val-db, got %s, err: %v", databaseValue, err)
	}

	// 4. Boot Redis testcontainer
	redisContainer, err := tcredis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Skipf("docker not available for redis: %v", err)
		return
	}
	defer func() { _ = redisContainer.Terminate(ctx) }()

	redisURI, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("failed to get redis connection string: %v", err)
	}

	// 5. Resolve Redis backend ("redis") -> RedisStore
	config.KVStore = KVStoreConfig{
		Backend: "redis",
		URL:     redisURI,
	}
	SetLoadedConfig(config)

	redisKVStore, err := NewKVStore(ctx, db)
	if err != nil {
		t.Fatalf("failed to resolve redis store: %v", err)
	}
	defer func() { _ = redisKVStore.Close() }()

	if err = redisKVStore.Ping(ctx); err != nil {
		t.Fatalf("expected redis store ping success: %v", err)
	}
	if err = redisKVStore.Set(ctx, "common:key", "val-redis", time.Minute); err != nil {
		t.Fatalf("failed to set key on redis store: %v", err)
	}
	redisValue, err := redisKVStore.Get(ctx, "common:key")
	if err != nil || redisValue != "val-redis" {
		t.Fatalf("expected val-redis, got %s, err: %v", redisValue, err)
	}
}
