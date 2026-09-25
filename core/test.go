package core

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	testcontainers "github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestMasterEncryptionKeyHex is a deterministic 32-byte hex key for unit and integration testing.
const TestMasterEncryptionKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

const (
	defaultTestDatabaseSetupTimeout   = 120 * time.Second
	defaultTestDatabaseStartupTimeout = 60 * time.Second
	masterEncryptionKeyHexLength      = 64
)

// SetupTestDB starts a PostgreSQL container, runs system and service migrations, and returns the DatabasePool.
func SetupTestDB(t *testing.T, serviceMigrations []DatabaseMigration) (*DatabasePool, func()) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTestDatabaseSetupTimeout)
	defer cancel()

	postgresContainer, _ := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(defaultTestDatabaseStartupTimeout),
		),
	)

	databaseURL, _ := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	db, _ := NewDatabasePool(ctx, databaseURL)
	_ = db.RunMigrations(ctx, SystemDatabaseMigrations)
	if len(serviceMigrations) > 0 {
		_ = db.RunMigrations(ctx, serviceMigrations)
	}

	cleanup := func() {
		db.Close()
		_ = postgresContainer.Terminate(context.Background())
	}

	return db, cleanup
}

// SetupBrokenTestDB launches a real PostgreSQL container, applies migrations, and immediately terminates it.
// Returns a closed DatabasePool for asserting database query failure code paths.
func SetupBrokenTestDB(t *testing.T, serviceMigrations []DatabaseMigration) *DatabasePool {
	t.Helper()

	db, cleanup := SetupTestDB(t, serviceMigrations)
	cleanup()
	return db
}

// TestInMemoryKVDriver provides a thread-safe, in-memory implementation of KVDriver for testing.
type TestInMemoryKVDriver struct {
	rwMutex sync.RWMutex
	storage map[string]string
	expires map[string]time.Time
}

var (
	_ KVDriver = (*TestInMemoryKVDriver)(nil)
	_ Sweeper  = (*TestInMemoryKVDriver)(nil)
)

// NewTestInMemoryKVDriver creates a new in-memory KVDriver instance.
func NewTestInMemoryKVDriver() *TestInMemoryKVDriver {
	return &TestInMemoryKVDriver{
		storage: make(map[string]string),
		expires: make(map[string]time.Time),
	}
}

// NewInMemoryKVStore creates a new KVStore backed by an TestInMemoryKVDriver.
func NewInMemoryKVStore() *KVStore {
	return NewKVStoreFromDriver(NewTestInMemoryKVDriver())
}

func (driver *TestInMemoryKVDriver) isExpired(key string) bool {
	if expiresAt, hasExpiry := driver.expires[key]; hasExpiry && time.Now().UTC().After(expiresAt) {
		delete(driver.storage, key)
		delete(driver.expires, key)
		return true
	}
	return false
}

// Get retrieves a value by key.
func (driver *TestInMemoryKVDriver) Get(_ context.Context, key string) (string, error) {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	if driver.isExpired(key) {
		return "", ErrKVStoreKeyNotFound
	}
	value, exists := driver.storage[key]
	if !exists {
		return "", ErrKVStoreKeyNotFound
	}
	return value, nil
}

// MGet retrieves multiple values by keys.
func (driver *TestInMemoryKVDriver) MGet(_ context.Context, keys []string) (map[string]string, error) {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	results := make(map[string]string, len(keys))
	for _, key := range keys {
		if !driver.isExpired(key) {
			if value, exists := driver.storage[key]; exists {
				results[key] = value
			}
		}
	}
	return results, nil
}

// Set stores a key-value pair with optional TTL.
func (driver *TestInMemoryKVDriver) Set(_ context.Context, key string, value string, expiry time.Duration) error {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	driver.storage[key] = value
	if expiry > 0 {
		driver.expires[key] = time.Now().UTC().Add(expiry)
	} else {
		delete(driver.expires, key)
	}
	return nil
}

// MSet stores multiple key-value pairs with optional TTL.
func (driver *TestInMemoryKVDriver) MSet(_ context.Context, entries map[string]string, expiry time.Duration) error {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	now := time.Now().UTC()
	for key, value := range entries {
		driver.storage[key] = value
		if expiry > 0 {
			driver.expires[key] = now.Add(expiry)
		} else {
			delete(driver.expires, key)
		}
	}
	return nil
}

// SetNX stores a key-value pair only if the key does not already exist.
func (driver *TestInMemoryKVDriver) SetNX(_ context.Context, key string, value string, expiry time.Duration) (bool, error) {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	if !driver.isExpired(key) {
		if _, exists := driver.storage[key]; exists {
			return false, nil
		}
	}
	driver.storage[key] = value
	if expiry > 0 {
		driver.expires[key] = time.Now().UTC().Add(expiry)
	} else {
		delete(driver.expires, key)
	}
	return true, nil
}

// Delete removes a key from storage.
func (driver *TestInMemoryKVDriver) Delete(_ context.Context, key string) error {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	delete(driver.storage, key)
	delete(driver.expires, key)
	return nil
}

// Increment increments an integer value by 1.
func (driver *TestInMemoryKVDriver) Increment(ctx context.Context, key string, expiry time.Duration) (int64, error) {
	return driver.IncrementBy(ctx, key, 1, expiry)
}

// IncrementBy increments an integer value by delta.
func (driver *TestInMemoryKVDriver) IncrementBy(_ context.Context, key string, delta int64, expiry time.Duration) (int64, error) {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	var currentValue int64
	if !driver.isExpired(key) {
		if rawValue, exists := driver.storage[key]; exists {
			currentValue, _ = strconv.ParseInt(rawValue, 10, 64)
		}
	}
	currentValue += delta
	driver.storage[key] = strconv.FormatInt(currentValue, 10)
	if expiry > 0 {
		driver.expires[key] = time.Now().UTC().Add(expiry)
	}
	return currentValue, nil
}

// Expire sets an expiration deadline on an existing key.
func (driver *TestInMemoryKVDriver) Expire(_ context.Context, key string, expiry time.Duration) error {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	if driver.isExpired(key) {
		return ErrKVStoreKeyNotFound
	}
	if _, exists := driver.storage[key]; !exists {
		return ErrKVStoreKeyNotFound
	}
	if expiry > 0 {
		driver.expires[key] = time.Now().UTC().Add(expiry)
	} else {
		delete(driver.expires, key)
	}
	return nil
}

// Ping checks health of in-memory driver (always succeeds).
func (driver *TestInMemoryKVDriver) Ping(_ context.Context) error {
	return nil
}

// Close closes the in-memory driver.
func (driver *TestInMemoryKVDriver) Close() error {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	driver.storage = make(map[string]string)
	driver.expires = make(map[string]time.Time)
	return nil
}

// Sweep removes all expired keys from storage and returns the count of purged items.
func (driver *TestInMemoryKVDriver) Sweep(_ context.Context) (int64, error) {
	driver.rwMutex.Lock()
	defer driver.rwMutex.Unlock()
	var purgedCount int64
	now := time.Now().UTC()
	for key, expiresAt := range driver.expires {
		if now.After(expiresAt) {
			delete(driver.storage, key)
			delete(driver.expires, key)
			purgedCount++
		}
	}
	return purgedCount, nil
}

// TestKernelOption defines a functional option for configuring a test Kernel.
type TestKernelOption func(kernel *Kernel)

// WithDB configures a custom DatabasePool on the test Kernel.
func WithDB(db *DatabasePool) TestKernelOption {
	return func(kernel *Kernel) {
		kernel.db = db
	}
}

// WithKVStore configures a custom KVStore on the test Kernel.
func WithKVStore(kvStore *KVStore) TestKernelOption {
	return func(kernel *Kernel) {
		kernel.kvStore = kvStore
	}
}

// WithCryptoKeyManager configures a custom CryptoKeyManager on the test Kernel.
func WithCryptoKeyManager(cryptoKeyManager *CryptoKeyManager) TestKernelOption {
	return func(kernel *Kernel) {
		kernel.cryptoKeyManager = cryptoKeyManager
	}
}

// WithJWTSigner configures a custom JWTSigner on the test Kernel.
func WithJWTSigner(jwtSigner *JWTSigner) TestKernelOption {
	return func(kernel *Kernel) {
		kernel.jwtSigner = jwtSigner
	}
}

// WithServiceAccountManager configures a custom ServiceAccountManager on the test Kernel.
func WithServiceAccountManager(serviceAccountManager *ServiceAccountManager) TestKernelOption {
	return func(kernel *Kernel) {
		kernel.serviceAccountManager = serviceAccountManager
	}
}

// WithEventBus configures a custom EventBus on the test Kernel.
func WithEventBus(eventBus *EventBus) TestKernelOption {
	return func(kernel *Kernel) {
		kernel.eventBus = eventBus
		if eventBus != nil {
			kernel.eventManager = eventBus.EventManager()
			kernel.eventHookManager = eventBus.EventHookManager()
		}
	}
}

// WithEventManager configures a custom EventManager on the test Kernel.
func WithEventManager(eventManager *EventManager) TestKernelOption {
	return func(kernel *Kernel) {
		kernel.eventManager = eventManager
	}
}

// WithEventHookManager configures a custom EventHookManager on the test Kernel.
func WithEventHookManager(eventHookManager *EventHookManager) TestKernelOption {
	return func(kernel *Kernel) {
		kernel.eventHookManager = eventHookManager
	}
}

// WithNodeManager configures a custom NodeManager on the test Kernel.
func WithNodeManager(nodeManager *NodeManager) TestKernelOption {
	return func(kernel *Kernel) {
		kernel.nodeManager = nodeManager
	}
}

// WithServer configures a custom Server on the test Kernel.
func WithServer(server *Server) TestKernelOption {
	return func(kernel *Kernel) {
		kernel.server = server
	}
}

// NewTestKernel constructs a Kernel with initialized subsystems for unit and integration testing.
func NewTestKernel(db *DatabasePool, options ...TestKernelOption) *Kernel {
	config := GetConfig()
	masterKey := config.Security.MasterEncryptionKey
	if len(masterKey) != masterEncryptionKeyHexLength {
		masterKey = TestMasterEncryptionKeyHex
	}

	cryptoKeyManager, _ := NewCryptoKeyManager(masterKey)
	jwtSigner := NewJWTSigner(cryptoKeyManager)

	kernel := &Kernel{
		db:               db,
		cryptoKeyManager: cryptoKeyManager,
		jwtSigner:        jwtSigner,
		kvStore:          NewInMemoryKVStore(),
	}

	if db != nil {
		kernel.serviceAccountManager = NewServiceAccountManager(db)
		kernel.eventBus = NewEventBus(db, cryptoKeyManager)
		kernel.eventManager = kernel.eventBus.EventManager()
		kernel.eventHookManager = kernel.eventBus.EventHookManager()
	} else {
		kernel.eventBus = NewEventBus(nil, cryptoKeyManager)
	}

	for _, option := range options {
		option(kernel)
	}

	return kernel
}

// SetupTestKernel spins up a live PostgreSQL testcontainer, applies migrations, and returns a fully initialized Kernel.
func SetupTestKernel(t *testing.T, serviceMigrations []DatabaseMigration, options ...TestKernelOption) (*Kernel, func()) {
	t.Helper()

	testConfig := DefaultConfig()
	testConfig.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	SetLoadedConfig(testConfig)

	db, cleanupDB := SetupTestDB(t, serviceMigrations)
	kernel := NewTestKernel(db, options...)
	kernel.server = NewServer(kernel)

	cleanup := func() {
		UnloadConfig()
		cleanupDB()
	}

	return kernel, cleanup
}

// SetupTestKernelWithBrokenDB spins up a test database, runs migrations, closes the pool, and returns a Kernel with broken DB for testing failure handling.
func SetupTestKernelWithBrokenDB(t *testing.T, serviceMigrations []DatabaseMigration, options ...TestKernelOption) *Kernel {
	t.Helper()

	testConfig := DefaultConfig()
	testConfig.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	SetLoadedConfig(testConfig)

	brokenDB := SetupBrokenTestDB(t, serviceMigrations)
	kernel := NewTestKernel(brokenDB, options...)
	kernel.server = NewServer(kernel)
	return kernel
}

// WithTestAuthContext attaches an AuthContext to an HTTP request for testing.
func WithTestAuthContext(request *http.Request, userID, role string, isAnonymous bool) *http.Request {
	authContext := AuthContext{
		UserID: userID,
		JWT: JWTClaims{
			Subject:     userID,
			Role:        role,
			IsAnonymous: isAnonymous,
		},
	}
	return request.WithContext(WithAuthContext(request.Context(), authContext))
}

// WithTestAuthClaims attaches arbitrary JWT claims to an HTTP request context for testing.
func WithTestAuthClaims(request *http.Request, jwtClaims JWTClaims) *http.Request {
	authContext := AuthContext{
		UserID: jwtClaims.Subject,
		JWT:    jwtClaims,
	}
	return request.WithContext(WithAuthContext(request.Context(), authContext))
}
