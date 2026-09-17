// Package auth provides authentication services, dispatchers, and configuration.
package auth

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

const (
	testDatabaseStartupTimeout    = 60 * time.Second
	testDatabaseWaitLogOccurrence = 2
	testMasterEncryptionKeyHex    = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

type inMemoryKVStore struct {
	rwMutex sync.RWMutex
	storage map[string]string
	setErr  error
}

func newInMemoryKVStore() *inMemoryKVStore {
	return &inMemoryKVStore{
		storage: make(map[string]string),
	}
}

func (store *inMemoryKVStore) Get(ctx context.Context, key string) (string, error) {
	store.rwMutex.RLock()
	defer store.rwMutex.RUnlock()
	value, exists := store.storage[key]
	if !exists {
		return "", core.ErrKVStoreKeyNotFound
	}
	return value, nil
}

func (store *inMemoryKVStore) MGet(ctx context.Context, keys []string) (map[string]string, error) {
	store.rwMutex.RLock()
	defer store.rwMutex.RUnlock()
	results := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, exists := store.storage[key]; exists {
			results[key] = value
		}
	}
	return results, nil
}

func (store *inMemoryKVStore) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	store.rwMutex.Lock()
	defer store.rwMutex.Unlock()
	if store.setErr != nil {
		return store.setErr
	}
	store.storage[key] = value
	return nil
}

func (store *inMemoryKVStore) MSet(ctx context.Context, entries map[string]string, expiry time.Duration) error {
	store.rwMutex.Lock()
	defer store.rwMutex.Unlock()
	for key, value := range entries {
		store.storage[key] = value
	}
	return nil
}

func (store *inMemoryKVStore) SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error) {
	store.rwMutex.Lock()
	defer store.rwMutex.Unlock()
	if _, exists := store.storage[key]; exists {
		return false, nil
	}
	store.storage[key] = value
	return true, nil
}

func (store *inMemoryKVStore) Delete(ctx context.Context, key string) error {
	store.rwMutex.Lock()
	defer store.rwMutex.Unlock()
	delete(store.storage, key)
	return nil
}

func (store *inMemoryKVStore) Increment(ctx context.Context, key string, expiry time.Duration) (int64, error) {
	return store.IncrementBy(ctx, key, 1, expiry)
}

func (store *inMemoryKVStore) IncrementBy(ctx context.Context, key string, delta int64, expiry time.Duration) (int64, error) {
	store.rwMutex.Lock()
	defer store.rwMutex.Unlock()
	currentValue, _ := strconv.ParseInt(store.storage[key], 10, 64)
	currentValue += delta
	store.storage[key] = strconv.FormatInt(currentValue, 10)
	return currentValue, nil
}

func (store *inMemoryKVStore) Expire(ctx context.Context, key string, expiry time.Duration) error {
	return nil
}

func (store *inMemoryKVStore) Ping(ctx context.Context) error {
	return nil
}

func (store *inMemoryKVStore) Close() error {
	return nil
}

type mockOAuthClient struct {
	doFunc func(request *http.Request) (*http.Response, error)
}

func (mock *mockOAuthClient) Do(request *http.Request) (*http.Response, error) {
	return mock.doFunc(request)
}

type simulatedEntropyErrorReader struct{}

func (reader *simulatedEntropyErrorReader) Read(buffer []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestAuthHarnessMocksUnit(t *testing.T) {
	ctx := context.Background()
	kvStore := newInMemoryKVStore()
	_ = kvStore.Set(ctx, "k1", "v1", time.Minute)
	_ = kvStore.MSet(ctx, map[string]string{"k2": "v2"}, time.Minute)
	storedValue, _ := kvStore.Get(ctx, "k1")
	if storedValue != "v1" {
		t.Fatalf("expected v1, got: %s", storedValue)
	}
	_, _ = kvStore.Get(ctx, "nonexistent")
	batchResults, _ := kvStore.MGet(ctx, []string{"k1", "k2", "missing"})
	if len(batchResults) != 2 {
		t.Fatalf("expected 2 items, got: %d", len(batchResults))
	}
	_, _ = kvStore.SetNX(ctx, "k1", "v1-new", time.Minute)
	_, _ = kvStore.SetNX(ctx, "k4", "v4", time.Minute)
	_, _ = kvStore.Increment(ctx, "counter", time.Minute)
	_ = kvStore.Expire(ctx, "k1", time.Hour)
	_ = kvStore.Ping(ctx)
	_ = kvStore.Delete(ctx, "k1")
	_ = kvStore.Close()

	oauthClient := &mockOAuthClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}
	testRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
	testResponse, _ := oauthClient.Do(testRequest)
	defer func() {
		if testResponse != nil && testResponse.Body != nil {
			_ = testResponse.Body.Close()
		}
	}()
	if testResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", testResponse.StatusCode)
	}

	entropyReader := &simulatedEntropyErrorReader{}
	entropyBuffer := make([]byte, 16)
	_, readErr := entropyReader.Read(entropyBuffer)
	if !errors.Is(readErr, io.ErrUnexpectedEOF) {
		t.Fatalf("expected ErrUnexpectedEOF, got %v", readErr)
	}
}

func setupTestDatabase(t *testing.T) (*core.DatabasePool, *core.CryptoKeyManager, func()) {
	ctx := context.Background()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("auth_test"),
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

	// Apply Core + Auth Migrations
	allMigrations := append([]core.DatabaseMigration{}, core.SystemDatabaseMigrations...)
	allMigrations = append(allMigrations, Migrations...)
	if migrationErr := db.RunMigrations(ctx, allMigrations); migrationErr != nil {
		db.Close()
		_ = postgresContainer.Terminate(ctx)
		t.Fatalf("failed to run migrations: %v", migrationErr)
	}

	cryptoKeyManager, keyManagerErr := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if keyManagerErr != nil {
		db.Close()
		_ = postgresContainer.Terminate(ctx)
		t.Fatalf("failed to create key manager: %v", keyManagerErr)
	}

	cleanup := func() {
		db.Close()
		_ = postgresContainer.Terminate(ctx)
	}

	return db, cryptoKeyManager, cleanup
}

func createBrokenPool(t *testing.T) *core.DatabasePool {
	db, _, cleanup := setupTestDatabase(t)
	cleanup()
	return db
}

func withUserAuth(request *http.Request, userID, role string, isAnon bool) *http.Request {
	jwtClaims := core.JWTClaims{
		Subject:     userID,
		Role:        role,
		IsAnonymous: isAnon,
	}
	authContext := core.AuthContext{
		UserID: userID,
		JWT:    jwtClaims,
	}
	return request.WithContext(core.WithAuthContext(request.Context(), authContext))
}

func withUserAuthClaims(request *http.Request, jwtClaims core.JWTClaims) *http.Request {
	authContext := core.AuthContext{
		UserID: jwtClaims.Subject,
		JWT:    jwtClaims,
	}
	return request.WithContext(core.WithAuthContext(request.Context(), authContext))
}
