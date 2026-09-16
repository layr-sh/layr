package data

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"layr.sh/core"
)

const testMasterEncryptionKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func setupTestDataDatabase(t *testing.T) (*core.DatabasePool, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr_data_test"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Skipf("failed to start postgres container (skipping integration test): %v", err)
		return nil, func() {}
	}

	databaseURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = postgresContainer.Terminate(context.Background())
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := core.NewDatabasePool(ctx, databaseURL)
	if err != nil {
		_ = postgresContainer.Terminate(context.Background())
		t.Fatalf("failed to create db pool: %v", err)
	}

	if err := db.RunMigrations(ctx, core.SystemDatabaseMigrations); err != nil {
		db.Close()
		_ = postgresContainer.Terminate(context.Background())
		t.Fatalf("failed to run system migrations: %v", err)
	}
	if err := db.RunMigrations(ctx, Migrations); err != nil {
		db.Close()
		_ = postgresContainer.Terminate(context.Background())
		t.Fatalf("failed to run data migrations: %v", err)
	}

	cleanup := func() {
		db.Close()
		_ = postgresContainer.Terminate(context.Background())
	}

	return db, cleanup
}

type inMemoryKVStore struct {
	rwMutex   sync.RWMutex
	storage   map[string]string
	setErr    error
	setNXErr  error
	msetErr   error
	expireErr error
}

func newInMemoryKVStore() *inMemoryKVStore {
	return &inMemoryKVStore{
		storage: make(map[string]string),
	}
}

func (kvStore *inMemoryKVStore) Get(ctx context.Context, key string) (string, error) {
	kvStore.rwMutex.RLock()
	defer kvStore.rwMutex.RUnlock()
	value, exists := kvStore.storage[key]
	if !exists {
		return "", core.ErrKVStoreKeyNotFound
	}
	return value, nil
}

func (kvStore *inMemoryKVStore) MGet(ctx context.Context, keys []string) (map[string]string, error) {
	kvStore.rwMutex.RLock()
	defer kvStore.rwMutex.RUnlock()
	results := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, exists := kvStore.storage[key]; exists {
			results[key] = value
		}
	}
	return results, nil
}

func (kvStore *inMemoryKVStore) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	kvStore.rwMutex.Lock()
	defer kvStore.rwMutex.Unlock()
	if kvStore.setErr != nil {
		return kvStore.setErr
	}
	kvStore.storage[key] = value
	return nil
}

func (kvStore *inMemoryKVStore) MSet(ctx context.Context, entries map[string]string, expiry time.Duration) error {
	kvStore.rwMutex.Lock()
	defer kvStore.rwMutex.Unlock()
	if kvStore.msetErr != nil {
		return kvStore.msetErr
	}
	for key, value := range entries {
		kvStore.storage[key] = value
	}
	return nil
}

func (kvStore *inMemoryKVStore) SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error) {
	kvStore.rwMutex.Lock()
	defer kvStore.rwMutex.Unlock()
	if kvStore.setNXErr != nil {
		return false, kvStore.setNXErr
	}
	if _, exists := kvStore.storage[key]; exists {
		return false, nil
	}
	kvStore.storage[key] = value
	return true, nil
}

func (kvStore *inMemoryKVStore) Delete(ctx context.Context, key string) error {
	kvStore.rwMutex.Lock()
	defer kvStore.rwMutex.Unlock()
	if strings.HasSuffix(key, "*") {
		prefix := strings.TrimSuffix(key, "*")
		for k := range kvStore.storage {
			if strings.HasPrefix(k, prefix) {
				delete(kvStore.storage, k)
			}
		}
		return nil
	}
	delete(kvStore.storage, key)
	return nil
}

func (kvStore *inMemoryKVStore) Increment(ctx context.Context, key string, expiry time.Duration) (int64, error) {
	return kvStore.IncrementBy(ctx, key, 1, expiry)
}

func (kvStore *inMemoryKVStore) IncrementBy(ctx context.Context, key string, delta int64, expiry time.Duration) (int64, error) {
	kvStore.rwMutex.Lock()
	defer kvStore.rwMutex.Unlock()
	currentValue, _ := strconv.ParseInt(kvStore.storage[key], 10, 64)
	currentValue += delta
	kvStore.storage[key] = strconv.FormatInt(currentValue, 10)
	return currentValue, nil
}

func (kvStore *inMemoryKVStore) Expire(ctx context.Context, key string, expiry time.Duration) error {
	kvStore.rwMutex.Lock()
	defer kvStore.rwMutex.Unlock()
	if kvStore.expireErr != nil {
		return kvStore.expireErr
	}
	if _, exists := kvStore.storage[key]; !exists {
		return core.ErrKVStoreKeyNotFound
	}
	return nil
}

func (kvStore *inMemoryKVStore) Ping(ctx context.Context) error {
	return nil
}

func (kvStore *inMemoryKVStore) Close() error {
	return nil
}
