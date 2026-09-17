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

type inMemoryKVDriver struct {
	rwMutex   sync.RWMutex
	storage   map[string]string
	setErr    error
	setNXErr  error
	msetErr   error
	expireErr error
}

func newInMemoryKVDriver() *inMemoryKVDriver {
	return &inMemoryKVDriver{
		storage: make(map[string]string),
	}
}

func newInMemoryKVStore() *core.KVStore {
	return core.NewKVStoreFromDriver(newInMemoryKVDriver())
}

func (driver *inMemoryKVDriver) Get(ctx context.Context, key string) (string, error) {
	driver.rwMutex.RLock()
	defer driver.rwMutex.RUnlock()
	value, exists := driver.storage[key]
	if !exists {
		return "", core.ErrKVStoreKeyNotFound
	}
	return value, nil
}

func (driver *inMemoryKVDriver) MGet(ctx context.Context, keys []string) (map[string]string, error) {
	driver.rwMutex.RLock()
	defer driver.rwMutex.RUnlock()
	results := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, exists := driver.storage[key]; exists {
			results[key] = value
		}
	}
	return results, nil
}

func (driver *inMemoryKVDriver) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	if driver.setErr != nil {
		return driver.setErr
	}
	driver.storage[key] = value
	return nil
}

func (driver *inMemoryKVDriver) MSet(ctx context.Context, entries map[string]string, expiry time.Duration) error {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	if driver.msetErr != nil {
		return driver.msetErr
	}
	for key, value := range entries {
		driver.storage[key] = value
	}
	return nil
}

func (driver *inMemoryKVDriver) SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error) {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	if driver.setNXErr != nil {
		return false, driver.setNXErr
	}
	if _, exists := driver.storage[key]; exists {
		return false, nil
	}
	driver.storage[key] = value
	return true, nil
}

func (driver *inMemoryKVDriver) Delete(ctx context.Context, key string) error {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	if strings.HasSuffix(key, "*") {
		prefix := strings.TrimSuffix(key, "*")
		for k := range driver.storage {
			if strings.HasPrefix(k, prefix) {
				delete(driver.storage, k)
			}
		}
		return nil
	}
	delete(driver.storage, key)
	return nil
}

func (driver *inMemoryKVDriver) Increment(ctx context.Context, key string, expiry time.Duration) (int64, error) {
	return driver.IncrementBy(ctx, key, 1, expiry)
}

func (driver *inMemoryKVDriver) IncrementBy(ctx context.Context, key string, delta int64, expiry time.Duration) (int64, error) {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	currentValue, _ := strconv.ParseInt(driver.storage[key], 10, 64)
	currentValue += delta
	driver.storage[key] = strconv.FormatInt(currentValue, 10)
	return currentValue, nil
}

func (driver *inMemoryKVDriver) Expire(ctx context.Context, key string, expiry time.Duration) error {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	if driver.expireErr != nil {
		return driver.expireErr
	}
	if _, exists := driver.storage[key]; !exists {
		return core.ErrKVStoreKeyNotFound
	}
	return nil
}

func (driver *inMemoryKVDriver) Ping(ctx context.Context) error {
	return nil
}

func (driver *inMemoryKVDriver) Close() error {
	return nil
}
