package threat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"layr.sh/core"
)

type inMemoryKVDriver struct {
	rwMutex             sync.RWMutex
	storage             map[string]string
	shouldFailGet       bool
	shouldFailSet       bool
	shouldFailIncrement bool
	shouldFailDelete    bool
}

func newInMemoryKVDriver() *inMemoryKVDriver {
	return &inMemoryKVDriver{
		storage: make(map[string]string),
	}
}

func newTestKVStore() *core.KVStore {
	return core.NewKVStoreFromDriver(newInMemoryKVDriver())
}

var ErrSimulatedKV = errors.New("simulated kv error")

func (driver *inMemoryKVDriver) Get(ctx context.Context, key string) (string, error) {
	driver.rwMutex.RLock()
	defer driver.rwMutex.RUnlock()
	if driver.shouldFailGet {
		return "", ErrSimulatedKV
	}
	storedValue, exists := driver.storage[key]
	if !exists {
		return "", core.ErrKVStoreKeyNotFound
	}
	return storedValue, nil
}

func (driver *inMemoryKVDriver) MGet(ctx context.Context, keys []string) (map[string]string, error) {
	driver.rwMutex.RLock()
	defer driver.rwMutex.RUnlock()
	results := make(map[string]string, len(keys))
	for _, key := range keys {
		if storedValue, exists := driver.storage[key]; exists {
			results[key] = storedValue
		}
	}
	return results, nil
}

func (driver *inMemoryKVDriver) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	if driver.shouldFailSet {
		return ErrSimulatedKV
	}
	driver.storage[key] = value
	return nil
}

func (driver *inMemoryKVDriver) MSet(ctx context.Context, entries map[string]string, expiry time.Duration) error {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	for key, storedValue := range entries {
		driver.storage[key] = storedValue
	}
	return nil
}

func (driver *inMemoryKVDriver) SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error) {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	if _, exists := driver.storage[key]; exists {
		return false, nil
	}
	driver.storage[key] = value
	return true, nil
}

func (driver *inMemoryKVDriver) Delete(ctx context.Context, key string) error {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	if driver.shouldFailDelete {
		return ErrSimulatedKV
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
	if driver.shouldFailIncrement {
		return 0, ErrSimulatedKV
	}
	currentValue, _ := strconv.ParseInt(driver.storage[key], 10, 64)
	currentValue += delta
	driver.storage[key] = strconv.FormatInt(currentValue, 10)
	return currentValue, nil
}

func (driver *inMemoryKVDriver) Expire(ctx context.Context, key string, expiry time.Duration) error {
	return nil
}

func (driver *inMemoryKVDriver) Ping(ctx context.Context) error {
	return nil
}

func (driver *inMemoryKVDriver) Close() error {
	return nil
}

type mockHTTPClient struct {
	doFunc func(request *http.Request) (*http.Response, error)
}

func (client *mockHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return client.doFunc(request)
}

type failingBodyReader struct{}

func (reader *failingBodyReader) Read(buffer []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func (reader *failingBodyReader) Close() error {
	return nil
}

func TestThreatHelperMocksUnit(t *testing.T) {
	ctx := context.Background()
	driver := newInMemoryKVDriver()
	kvStore := core.NewKVStoreFromDriver(driver)

	_ = kvStore.Set(ctx, "k1", "v1", time.Minute)
	_ = kvStore.MSet(ctx, map[string]string{"k2": "v2"}, time.Minute)
	retrievedValue, _ := kvStore.Get(ctx, "k1")
	if retrievedValue != "v1" {
		t.Fatalf("expected v1, got %s", retrievedValue)
	}

	batchValues, _ := kvStore.MGet(ctx, []string{"k1", "k2", "k3"})
	if len(batchValues) != 2 {
		t.Fatalf("expected 2 items, got %d", len(batchValues))
	}

	_, _ = kvStore.SetNX(ctx, "k1", "ignored", time.Minute)
	_, _ = kvStore.SetNX(ctx, "k3", "v3", time.Minute)
	_, _ = kvStore.Increment(ctx, "counter", time.Minute)
	_ = kvStore.Expire(ctx, "k1", time.Minute)
	_ = kvStore.Ping(ctx)
	_ = kvStore.Delete(ctx, "k1")
	_ = kvStore.Close()

	driver.shouldFailGet = true
	if _, err := kvStore.Get(ctx, "k2"); err == nil {
		t.Fatal("expected get error")
	}

	driver.shouldFailSet = true
	if err := kvStore.Set(ctx, "k4", "v4", time.Minute); err == nil {
		t.Fatal("expected set error")
	}

	driver.shouldFailIncrement = true
	if _, err := kvStore.Increment(ctx, "counter", time.Minute); err == nil {
		t.Fatal("expected increment error")
	}

	driver.shouldFailDelete = true
	if err := kvStore.Delete(ctx, "k2"); err == nil {
		t.Fatal("expected delete error")
	}

	client := &mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}
	testRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
	testResponse, err := client.Do(testRequest)
	if err != nil || testResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected client response: %v, %v", testResponse, err)
	}
	defer func() {
		_ = testResponse.Body.Close()
	}()

	bodyReader := &failingBodyReader{}
	buffer := make([]byte, 16)
	_, readErr := bodyReader.Read(buffer)
	if !errors.Is(readErr, io.ErrUnexpectedEOF) {
		t.Fatalf("expected ErrUnexpectedEOF, got %v", readErr)
	}
	_ = bodyReader.Close()
}

const (
	testDatabaseStartupTimeout    = 60 * time.Second
	testDatabaseWaitLogOccurrence = 2
)

var threatDatabaseMigrations = []core.DatabaseMigration{
	{
		Service:     "auth",
		Version:     1,
		Description: "Auth schema and tables for threat testing",
		UpSQL: `
			CREATE SCHEMA IF NOT EXISTS auth;
			CREATE TABLE IF NOT EXISTS auth.users (
				id UUID PRIMARY KEY DEFAULT uuidv7(),
				email VARCHAR(255) UNIQUE
			);
			CREATE TABLE IF NOT EXISTS auth.sessions (
				id UUID PRIMARY KEY DEFAULT uuidv7(),
				user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
				client_id VARCHAR(255),
				refresh_token_hash VARCHAR(255) NOT NULL,
				ip_address INET,
				user_agent TEXT,
				expires_at TIMESTAMPTZ NOT NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
			);
		`,
	},
}

func setupTestThreatDatabase(t *testing.T) (*core.DatabasePool, func()) {
	ctx := context.Background()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("threat_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("secret"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(testDatabaseWaitLogOccurrence).
				WithStartupTimeout(testDatabaseStartupTimeout),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	databaseURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = postgresContainer.Terminate(ctx)
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, databaseErr := core.NewDatabasePool(ctx, databaseURL)
	if databaseErr != nil {
		_ = postgresContainer.Terminate(ctx)
		t.Fatalf("failed to create db pool: %v", databaseErr)
	}

	allMigrations := append([]core.DatabaseMigration{}, core.SystemDatabaseMigrations...)
	allMigrations = append(allMigrations, threatDatabaseMigrations...)
	if migrationErr := db.RunMigrations(ctx, allMigrations); migrationErr != nil {
		db.Close()
		_ = postgresContainer.Terminate(ctx)
		t.Fatalf("failed to run migrations: %v", migrationErr)
	}

	cleanup := func() {
		db.Close()
		_ = postgresContainer.Terminate(ctx)
	}

	return db, cleanup
}

func createBrokenThreatPool(t *testing.T) *core.DatabasePool {
	db, cleanup := setupTestThreatDatabase(t)
	cleanup()
	return db
}
