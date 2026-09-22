package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCoreKVStoreFactoryValidationUnit(t *testing.T) {
	ctx := context.Background()
	defer UnloadConfig()

	config := DefaultConfig()

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

type mockUnitTestKVDriver struct {
	storage map[string]string
	err     error
}

func (driver *mockUnitTestKVDriver) Get(ctx context.Context, key string) (string, error) {
	if driver.err != nil {
		return "", driver.err
	}
	entryValue, exists := driver.storage[key]
	if !exists {
		return "", ErrKVStoreKeyNotFound
	}
	return entryValue, nil
}

func (driver *mockUnitTestKVDriver) MGet(ctx context.Context, keys []string) (map[string]string, error) {
	if driver.err != nil {
		return nil, driver.err
	}
	result := make(map[string]string)
	for _, key := range keys {
		if entryValue, exists := driver.storage[key]; exists {
			result[key] = entryValue
		}
	}
	return result, nil
}

func (driver *mockUnitTestKVDriver) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	if driver.err != nil {
		return driver.err
	}
	driver.storage[key] = value
	return nil
}

func (driver *mockUnitTestKVDriver) MSet(ctx context.Context, entries map[string]string, expiry time.Duration) error {
	if driver.err != nil {
		return driver.err
	}
	for key, value := range entries {
		driver.storage[key] = value
	}
	return nil
}

func (driver *mockUnitTestKVDriver) SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error) {
	if driver.err != nil {
		return false, driver.err
	}
	if _, exists := driver.storage[key]; exists {
		return false, nil
	}
	driver.storage[key] = value
	return true, nil
}

func (driver *mockUnitTestKVDriver) Delete(ctx context.Context, key string) error {
	if driver.err != nil {
		return driver.err
	}
	delete(driver.storage, key)
	return nil
}

func (driver *mockUnitTestKVDriver) Increment(ctx context.Context, key string, expiry time.Duration) (int64, error) {
	return driver.IncrementBy(ctx, key, 1, expiry)
}

func (driver *mockUnitTestKVDriver) IncrementBy(ctx context.Context, key string, delta int64, expiry time.Duration) (int64, error) {
	if driver.err != nil {
		return 0, driver.err
	}
	driver.storage[key] = "1"
	return delta, nil
}

func (driver *mockUnitTestKVDriver) Expire(ctx context.Context, key string, expiry time.Duration) error {
	if driver.err != nil {
		return driver.err
	}
	return nil
}

func (driver *mockUnitTestKVDriver) Ping(ctx context.Context) error {
	if driver.err != nil {
		return driver.err
	}
	return nil
}

func (driver *mockUnitTestKVDriver) Close() error {
	if driver.err != nil {
		return driver.err
	}
	return nil
}

type mockUnitTestSweeperDriver struct {
	mockUnitTestKVDriver
}

func (driver *mockUnitTestSweeperDriver) Sweep(ctx context.Context) (int64, error) {
	if driver.err != nil {
		return 0, driver.err
	}
	return 42, nil
}

func TestCoreKVStoreWrapperUnit(t *testing.T) {
	ctx := context.Background()

	// 4. Valid driver without Sweeper
	mockDriver := &mockUnitTestKVDriver{storage: make(map[string]string)}
	kvStore := NewKVStoreFromDriver(mockDriver)
	if kvStore.Driver() != mockDriver {
		t.Fatal("expected store with mock driver")
	}
	if err := kvStore.Set(ctx, "key1", "val1", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if retrievedValue, err := kvStore.Get(ctx, "key1"); err != nil || retrievedValue != "val1" {
		t.Fatalf("Get failed: %v, retrievedValue=%s", err, retrievedValue)
	}
	if err := kvStore.MSet(ctx, map[string]string{"key2": "val2"}, 0); err != nil {
		t.Fatalf("MSet failed: %v", err)
	}
	if multiGetEntries, err := kvStore.MGet(ctx, []string{"key1", "key2"}); err != nil || len(multiGetEntries) != 2 {
		t.Fatalf("MGet failed: %v", err)
	}
	if ok, err := kvStore.SetNX(ctx, "key1", "val1", 0); err != nil || ok {
		t.Fatalf("SetNX expected false for existing key, got %v, err=%v", ok, err)
	}
	if ok, err := kvStore.SetNX(ctx, "key3", "val3", 0); err != nil || !ok {
		t.Fatalf("SetNX expected true for new key, got %v, err=%v", ok, err)
	}
	if incrementResult, err := kvStore.Increment(ctx, "counter", 0); err != nil || incrementResult != 1 {
		t.Fatalf("Increment failed: %v, val=%d", err, incrementResult)
	}
	if incrementByResult, err := kvStore.IncrementBy(ctx, "counter", 10, 0); err != nil || incrementByResult != 10 {
		t.Fatalf("IncrementBy failed: %v, val=%d", err, incrementByResult)
	}
	if err := kvStore.Expire(ctx, "key1", 10*time.Second); err != nil {
		t.Fatalf("Expire failed: %v", err)
	}
	if err := kvStore.Ping(ctx); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	if swept, err := kvStore.Sweep(ctx); err != nil || swept != 0 {
		t.Fatalf("expected (0, nil) for driver not implementing Sweeper, got (%d, %v)", swept, err)
	}
	if err := kvStore.Delete(ctx, "key1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if err := kvStore.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 5. Driver implementing Sweeper
	sweeperDriver := &mockUnitTestSweeperDriver{mockUnitTestKVDriver: mockUnitTestKVDriver{storage: make(map[string]string)}}
	sweeperKVStore := NewKVStoreFromDriver(sweeperDriver)
	if swept, err := sweeperKVStore.Sweep(ctx); err != nil || swept != 42 {
		t.Fatalf("expected (42, nil) from sweeper driver, got (%d, %v)", swept, err)
	}

	// 6. DatabaseKVStore.KVStore() and RedisKVStore.KVStore() helper methods
	databaseKVStore := &DatabaseKVStore{}
	if dbKVStore := databaseKVStore.KVStore(); dbKVStore.Driver() != databaseKVStore {
		t.Fatal("expected DatabaseKVStore.KVStore() to wrap databaseKVStore")
	}

	redisKVStore := &RedisKVStore{}
	if rKVStore := redisKVStore.KVStore(); rKVStore.Driver() != redisKVStore {
		t.Fatal("expected RedisKVStore.KVStore() to wrap redisKVStore")
	}
	if kvStore.KVStore() != kvStore {
		t.Fatal("expected KVStore.KVStore() to return self")
	}

	// 7. Driver error propagation
	failingMockUnitTestKVDriver := &mockUnitTestKVDriver{err: errors.New("driver error")}
	failingKVStore := NewKVStoreFromDriver(failingMockUnitTestKVDriver)
	if _, err := failingKVStore.Get(ctx, "k"); err == nil {
		t.Fatal("expected error on Get")
	}
	if _, err := failingKVStore.MGet(ctx, []string{"k"}); err == nil {
		t.Fatal("expected error on MGet")
	}
	if err := failingKVStore.Set(ctx, "k", "v", 0); err == nil {
		t.Fatal("expected error on Set")
	}
	if err := failingKVStore.MSet(ctx, map[string]string{"k": "v"}, 0); err == nil {
		t.Fatal("expected error on MSet")
	}
	if _, err := failingKVStore.SetNX(ctx, "k", "v", 0); err == nil {
		t.Fatal("expected error on SetNX")
	}
	if err := failingKVStore.Delete(ctx, "k"); err == nil {
		t.Fatal("expected error on Delete")
	}
	if _, err := failingKVStore.Increment(ctx, "k", 0); err == nil {
		t.Fatal("expected error on Increment")
	}
	if _, err := failingKVStore.IncrementBy(ctx, "k", 5, 0); err == nil {
		t.Fatal("expected error on IncrementBy")
	}
	if err := failingKVStore.Expire(ctx, "k", 0); err == nil {
		t.Fatal("expected error on Expire")
	}
	if err := failingKVStore.Ping(ctx); err == nil {
		t.Fatal("expected error on Ping")
	}
	if err := failingKVStore.Close(); err == nil {
		t.Fatal("expected error on Close")
	}

	failingMockUnitTestSweeperDriver := &mockUnitTestSweeperDriver{mockUnitTestKVDriver: mockUnitTestKVDriver{err: errors.New("sweep failure")}}
	failingSweeperKVStore := NewKVStoreFromDriver(failingMockUnitTestSweeperDriver)
	if _, err := failingSweeperKVStore.Sweep(ctx); err == nil {
		t.Fatal("expected error on Sweep")
	}
}
