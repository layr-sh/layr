package core

import (
	"context"
	"testing"
)

func TestCoreKVStoreFactoryValidationUnit(t *testing.T) {
	ctx := context.Background()
	defer UnloadConfig()

	// 1. Missing connection pool on database backend
	config := DefaultConfig()
	config.KVStore = KVStoreConfig{Backend: "database"}
	SetLoadedConfig(config)
	if _, err := NewKVStore(ctx, nil); err == nil {
		t.Fatal("expected error on nil db connection pool with database backend")
	}

	// 2. Missing connection pool on default empty backend
	config.KVStore = KVStoreConfig{Backend: ""}
	SetLoadedConfig(config)
	if _, err := NewKVStore(ctx, nil); err == nil {
		t.Fatal("expected error on nil db connection pool with default empty backend")
	}

	// 3. Unsupported backend
	config.KVStore = KVStoreConfig{Backend: "unsupported"}
	SetLoadedConfig(config)
	if _, err := NewKVStore(ctx, nil); err == nil {
		t.Fatal("expected error on unsupported backend")
	}
	config.KVStore = KVStoreConfig{Backend: "memcached"}
	SetLoadedConfig(config)
	if _, err := NewKVStore(ctx, nil); err == nil {
		t.Fatal("expected error on memcached backend")
	}

	// 4. Redis missing url and cluster urls
	config.KVStore = KVStoreConfig{Backend: "redis"}
	SetLoadedConfig(config)
	if _, err := NewKVStore(ctx, nil); err == nil {
		t.Fatal("expected error on redis missing url")
	}

	// 5. Redis invalid url
	config.KVStore = KVStoreConfig{Backend: "redis", URL: "://invalid-url"}
	SetLoadedConfig(config)
	if _, err := NewKVStore(ctx, nil); err == nil {
		t.Fatal("expected error on invalid redis url")
	}

	// 6. Redis unreachable endpoint
	config.KVStore = KVStoreConfig{Backend: "redis", URL: "redis://127.0.0.1:19999"}
	SetLoadedConfig(config)
	if _, err := NewKVStore(ctx, nil); err == nil {
		t.Fatal("expected connection error on unreachable redis")
	}

	// 7. Redis Cluster with unreachable endpoints
	config.KVStore = KVStoreConfig{
		Backend:     "redis",
		ClusterURLs: []string{"127.0.0.1:19998", "127.0.0.1:19999"},
	}
	SetLoadedConfig(config)
	if _, err := NewKVStore(ctx, nil); err == nil {
		t.Fatal("expected connection error on unreachable redis cluster")
	}
}
