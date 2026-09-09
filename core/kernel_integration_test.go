package core

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCoreKernelFullLifecycleEmbeddedIntegration(t *testing.T) {
	temporaryDirectory := t.TempDir()
	dataDirectory := filepath.Join(temporaryDirectory, "data")

	config := DefaultConfig()
	config.Database.URL = dataDirectory
	config.Data.Enabled = true
	config.Analytics.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	kernel, err := NewKernel()
	if err != nil {
		t.Fatalf("NewKernel failed: %v", err)
	}

	errChannel := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		errChannel <- kernel.Start(ctx)
	}()

	// Wait up to 60s for boot
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(60 * time.Second)

	ready := false
	for !ready {
		select {
		case err := <-errChannel:
			if err != nil {
				t.Fatalf("kernel start returned error: %v", err)
			}
		case <-timeout:
			cancel()
			t.Fatal("kernel did not start within 60s")
		case <-ticker.C:
			if kernel.server != nil {
				ready = true
			}
		}
	}

	// Cancel context to test graceful shutdown
	cancel()
	select {
	case err := <-errChannel:
		if err != nil && !strings.Contains(err.Error(), "context canceled") {
			t.Fatalf("kernel exited with error after cancel: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("kernel did not stop within 15s after cancel")
	}
}

func TestCoreKernelFailureEmbeddedBadPathIntegration(t *testing.T) {
	config := DefaultConfig()
	config.Data.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	config.Database.URL = "/dev/null/forbidden/path"
	SetLoadedConfig(config)
	defer UnloadConfig()

	kernel, _ := NewKernel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := kernel.Start(ctx); err == nil {
		t.Fatal("expected failure on invalid embedded database path")
	}
}

func TestCoreKernelFailureExternalBadURLIntegration(t *testing.T) {
	config := DefaultConfig()
	config.Data.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	config.Database.URL = "postgres://invalid:invalid@127.0.0.1:19999/db?sslmode=disable"
	SetLoadedConfig(config)
	defer UnloadConfig()

	kernel, _ := NewKernel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := kernel.Start(ctx); err == nil {
		t.Fatal("expected failure on unreachable database URL")
	}
}

func TestCoreKernelWithTestcontainerIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Skip("docker not available")
		return
	}
	defer func() {
		_ = postgresContainer.Terminate(ctx)
	}()

	databaseURL, _ := postgresContainer.ConnectionString(ctx, "sslmode=disable")

	config := DefaultConfig()
	config.Database.URL = databaseURL
	config.Data.Enabled = true
	config.Auth.Enabled = true
	config.FileStorage.Enabled = true
	config.Tasks.Enabled = true
	config.Notification.Enabled = true
	config.Analytics.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	kernel, err := NewKernel()
	if err != nil {
		t.Fatalf("NewKernel failed: %v", err)
	}

	errChannel := make(chan error, 1)
	startCtx, startCancel := context.WithCancel(ctx)
	defer startCancel()

	go func() {
		errChannel <- kernel.Start(startCtx)
	}()

	// Wait for boot
	time.Sleep(2 * time.Second)

	// Check PublishableKey
	if publishableKey := kernel.PublishableKey(); len(publishableKey) != 64 {
		t.Fatalf("expected 64-char hex publishable key from kernel, got: %s", publishableKey)
	}

	// Cancel context to test graceful shutdown via ctx.Done()
	startCancel()

	select {
	case err := <-errChannel:
		if err != nil {
			t.Fatalf("kernel exited with error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("kernel did not stop within 10 seconds after context cancellation")
	}
}

func TestCoreKernelSignalInterruptIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Skip("docker not available")
		return
	}
	defer func() { _ = postgresContainer.Terminate(ctx) }()

	databaseURL, _ := postgresContainer.ConnectionString(ctx, "sslmode=disable")

	config := DefaultConfig()
	config.Database.URL = databaseURL
	config.Data.Enabled = true
	config.Auth.Enabled = true
	config.FileStorage.Enabled = true
	config.Tasks.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	kernel, err := NewKernel()
	if err != nil {
		t.Fatalf("NewKernel failed: %v", err)
	}

	errChannel := make(chan error, 1)
	go func() {
		errChannel <- kernel.Start(ctx)
	}()

	time.Sleep(2 * time.Second)

	// Send SIGINT to test signal handling path
	process, _ := os.FindProcess(os.Getpid())
	_ = process.Signal(os.Interrupt)

	select {
	case err := <-errChannel:
		if err != nil {
			t.Fatalf("kernel signal shutdown exited with error: %v", err)
		}
	case <-time.After(10 * time.Second):
		_ = kernel.Stop(ctx)
	}
}

func TestCoreKernelServerErrorIntegration(t *testing.T) {
	// Occupy a port, then configure the kernel to listen on that exact port
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("cannot bind listener")
	}
	defer func() { _ = listener.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Skip("docker not available")
		return
	}
	defer func() { _ = postgresContainer.Terminate(ctx) }()

	databaseURL, _ := postgresContainer.ConnectionString(ctx, "sslmode=disable")

	config := DefaultConfig()
	config.Database.URL = databaseURL
	config.Data.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	config.Server.ListenAddr = listener.Addr().String()
	SetLoadedConfig(config)
	defer UnloadConfig()

	kernel, _ := NewKernel()
	if err := kernel.Start(ctx); err == nil {
		t.Fatal("expected kernel start failure on occupied port")
	}
}

func TestCoreKernelMigrationErrorIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Skip("docker not available")
		return
	}
	defer func() { _ = postgresContainer.Terminate(ctx) }()

	databaseURL, _ := postgresContainer.ConnectionString(ctx, "sslmode=disable")

	config := DefaultConfig()
	config.Database.URL = databaseURL
	config.Data.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	kernel, _ := NewKernel()

	// Pre-create incompatible table to trigger migration error
	setupDB, dbErr := NewDatabasePool(ctx, databaseURL, DatabasePoolOptions{})
	if dbErr != nil {
		t.Fatalf("failed to connect setup pool: %v", dbErr)
	}
	_, _ = setupDB.Exec(ctx, "CREATE SCHEMA core; CREATE TABLE core.migrations (broken_column TEXT);")
	setupDB.Close()

	err = kernel.Start(ctx)
	if err == nil || !strings.Contains(err.Error(), "database migration failed") {
		t.Fatalf("expected database migration failed error, got: %v", err)
	}

	// Clean up broken table so migrations succeed for subsequent kv store failure test
	cleanupDB, dbErr := NewDatabasePool(ctx, databaseURL, DatabasePoolOptions{})
	if dbErr == nil {
		_, _ = cleanupDB.Exec(ctx, "DROP SCHEMA core CASCADE;")
		cleanupDB.Close()
	}

	// Test KV store error with invalid redis URL
	badKVConfig := DefaultConfig()
	badKVConfig.Database.URL = databaseURL
	badKVConfig.Data.Enabled = true
	badKVConfig.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	badKVConfig.KVStore.Backend = "redis"
	badKVConfig.KVStore.URL = "redis://invalid-host:9999"
	SetLoadedConfig(badKVConfig)
	defer UnloadConfig()
	badKVKernel, _ := NewKernel()
	if err := badKVKernel.Start(ctx); err == nil {
		t.Fatal("expected kv store initialization error on invalid redis")
	}
}

type failingServiceRunner struct{}

func (failingRunner *failingServiceRunner) Start(ctx context.Context) error {
	return errors.New("mock service start failed")
}
func (failingRunner *failingServiceRunner) RegisterRoutes(router *Router, controlPlaneRouter *Router) {
}
func (failingRunner *failingServiceRunner) Stop() error { return nil }

func TestCoreKernelServiceStartErrorIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Skip("docker not available")
		return
	}
	defer func() { _ = postgresContainer.Terminate(ctx) }()

	databaseURL, _ := postgresContainer.ConnectionString(ctx, "sslmode=disable")

	config := DefaultConfig()
	config.Database.URL = databaseURL
	config.Data.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	kernel, _ := NewKernel()
	kernel.RegisterService(&failingServiceRunner{})

	err = kernel.Start(ctx)
	if err == nil || !strings.Contains(err.Error(), "mock service start failed") {
		t.Fatalf("expected mock service start error, got: %v", err)
	}

	// Test successful service runner
	successRunner := &mockServiceRunner{}
	plainRunner := &plainMockRunner{}
	runnerKernel, _ := NewKernel()
	runnerKernel.RegisterService(successRunner)
	runnerKernel.RegisterService(plainRunner)
	runnerCtx, runnerCancel := context.WithCancel(ctx)
	errChannel := make(chan error, 1)
	go func() {
		errChannel <- runnerKernel.Start(runnerCtx)
	}()
	for i := 0; i < 60; i++ {
		if successRunner.started && successRunner.registered && successRunner.openAPIRegistered {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	runnerCancel()
	<-errChannel
	_ = runnerKernel.Stop(context.Background())
	if !successRunner.started || !successRunner.registered || !successRunner.openAPIRegistered {
		t.Fatalf("expected service runner to be started and registered, got: %+v", successRunner)
	}
}

type plainMockRunner struct{}

func (plainRunner *plainMockRunner) Start(ctx context.Context) error { return nil }
func (plainRunner *plainMockRunner) RegisterRoutes(router *Router, controlPlaneRouter *Router) {
}
func (plainRunner *plainMockRunner) Stop() error { return nil }

func TestCoreKernelStopWithFullSubsystemsIntegration(t *testing.T) {
	config := DefaultConfig()
	config.Data.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	kernel, _ := NewKernel()
	if err := kernel.Stop(context.Background()); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

type failingService struct {
	err error
}

func (failingService *failingService) Start(ctx context.Context) error {
	return failingService.err
}

func (failingService *failingService) Stop() error {
	return nil
}

func (failingService *failingService) RegisterRoutes(router *Router, controlPlaneRouter *Router) {
}

func TestCoreKernelModularServiceStartErrorIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Skip("docker not available")
		return
	}
	defer func() { _ = postgresContainer.Terminate(ctx) }()

	databaseURL, _ := postgresContainer.ConnectionString(ctx, "sslmode=disable")

	config := DefaultConfig()
	config.Database.URL = databaseURL
	config.Data.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	kernel, _ := NewKernel()
	kernel.RegisterService(&failingService{err: errors.New("modular service failed to start")})
	if err := kernel.Start(ctx); err == nil {
		t.Fatal("expected kernel start failure on failing modular service")
	}

	// Test failing service factory auto-instantiation
	oldServiceFactory, hadOldData := GetServiceFactory("data")
	RegisterServiceFactory("data", func(kernel *Kernel) (ServiceRunner, error) {
		return nil, errors.New("data factory failure")
	})
	defer func() {
		if hadOldData {
			RegisterServiceFactory("data", oldServiceFactory)
		}
	}()

	failingFactoryKernel, _ := NewKernel()
	if err := failingFactoryKernel.Start(ctx); err == nil {
		t.Fatal("expected kernel start failure on failing service factory")
	}

	// Test failing bootstrap root console user during kernel.Start
	if cleanupDB, err := NewDatabasePool(ctx, databaseURL); err == nil && cleanupDB != nil {
		_, _ = cleanupDB.Exec(ctx, "DELETE FROM console.users")
		cleanupDB.Close()
	}

	defaultPasswordHasher.SetRandomReader(&coreErrReader{})
	failingBootstrapKernel, _ := NewKernel()
	if err := failingBootstrapKernel.Start(ctx); err == nil {
		t.Fatal("expected kernel start failure on failing bootstrap user")
	}
	defaultPasswordHasher.SetRandomReader(nil)

	// Test failing bootstrap root service account during kernel.Start
	if cleanupDB, err := NewDatabasePool(ctx, databaseURL); err == nil && cleanupDB != nil {
		_, _ = cleanupDB.Exec(ctx, "DELETE FROM core.service_accounts")
		_, _ = cleanupDB.Exec(ctx, "ALTER TABLE core.service_accounts ADD CONSTRAINT fail_service_account_bootstrap CHECK (name != 'Root Service Account')")
		cleanupDB.Close()
	}

	failingServiceAccountBootstrapKernel, _ := NewKernel()
	if err := failingServiceAccountBootstrapKernel.Start(ctx); err == nil {
		t.Fatal("expected kernel start failure on failing bootstrap service account")
	}

	if cleanupDB, err := NewDatabasePool(ctx, databaseURL); err == nil && cleanupDB != nil {
		_, _ = cleanupDB.Exec(ctx, "ALTER TABLE core.service_accounts DROP CONSTRAINT fail_service_account_bootstrap")
		cleanupDB.Close()
	}

	// Test successful service factory auto-instantiation
	successAutoRunner := &mockServiceRunner{}
	RegisterServiceFactory("data", func(kernel *Kernel) (ServiceRunner, error) {
		return successAutoRunner, nil
	})
	successFactoryKernel, _ := NewKernel()
	factoryCtx, factoryCancel := context.WithCancel(ctx)
	autoErrorChannel := make(chan error, 1)
	go func() {
		autoErrorChannel <- successFactoryKernel.Start(factoryCtx)
	}()
	for i := 0; i < 60; i++ {
		if successAutoRunner.started && successAutoRunner.registered {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	factoryCancel()
	<-autoErrorChannel
	_ = successFactoryKernel.Stop(context.Background())
	if !successAutoRunner.started || !successAutoRunner.registered {
		t.Fatalf("expected auto-instantiated service to start and register, got: %+v", successAutoRunner)
	}
}

func TestCoreHTTPRoutesIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, containerErr := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if containerErr != nil {
		t.Skip("docker not available")
		return
	}
	defer func() { _ = postgresContainer.Terminate(ctx) }()

	databaseURL, _ := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	db, dbErr := NewDatabasePool(ctx, databaseURL)
	if dbErr != nil {
		t.Fatalf("failed to create db connection pool: %v", dbErr)
	}
	defer db.Close()

	if migrationErr := db.RunMigrations(ctx, SystemDatabaseMigrations); migrationErr != nil {
		t.Fatalf("failed to run migrations: %v", migrationErr)
	}

	config := DefaultConfig()
	config.Database.URL = databaseURL
	config.Data.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	kernel, err := NewKernel()
	if err != nil {
		t.Fatalf("failed to create kernel: %v", err)
	}
	kernel.db = db
	kernel.serviceAccountManager = NewServiceAccountManager(db)
	kernel.webhookEventBus = NewWebhookEventBus(db, kernel.cryptoKeyManager)
	defer kernel.webhookEventBus.Close()
	kernel.webhookManager = NewWebhookManager(db, kernel.cryptoKeyManager, kernel.webhookEventBus)

	server := NewServer(db, kernel.cryptoKeyManager)
	kernel.registerCoreRoutes(server)

	// 1. Service Accounts HTTP API
	// POST /api/v1/_/core/service-accounts
	createServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/service-accounts", strings.NewReader(`{"name":"HTTP Service Account","scopes":["*"]}`))
	createServiceAccountRequest.Header.Set("Content-Type", "application/json")
	createServiceAccountResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(createServiceAccountResponseRecorder, createServiceAccountRequest)
	if createServiceAccountResponseRecorder.Code != http.StatusOK && createServiceAccountResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 200/201 for POST service accounts, got %d: %s", createServiceAccountResponseRecorder.Code, createServiceAccountResponseRecorder.Body.String())
	}
	var createdServiceAccount ServiceAccount
	_ = json.Unmarshal(createServiceAccountResponseRecorder.Body.Bytes(), &createdServiceAccount)

	// GET /api/v1/_/core/service-accounts
	listServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/service-accounts", nil)
	listServiceAccountResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(listServiceAccountResponseRecorder, listServiceAccountRequest)
	if listServiceAccountResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET service accounts, got %d", listServiceAccountResponseRecorder.Code)
	}

	// GET /api/v1/_/core/service-accounts/:id
	getServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/service-accounts/"+createdServiceAccount.ID, nil)
	getServiceAccountResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(getServiceAccountResponseRecorder, getServiceAccountRequest)
	if getServiceAccountResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET service account by ID, got %d", getServiceAccountResponseRecorder.Code)
	}

	// PUT /api/v1/_/core/service-accounts/:id
	updateServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/service-accounts/"+createdServiceAccount.ID, strings.NewReader(`{"name":"Updated HTTP Service Account"}`))
	updateServiceAccountRequest.Header.Set("Content-Type", "application/json")
	updateServiceAccountResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(updateServiceAccountResponseRecorder, updateServiceAccountRequest)
	if updateServiceAccountResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for PUT service account by ID, got %d", updateServiceAccountResponseRecorder.Code)
	}

	// 2. Webhooks HTTP API
	// POST /api/v1/_/core/webhooks
	createWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/webhooks", strings.NewReader(`{"name":"HTTP Webhook","target_url":"http://localhost:8080/webhook","events":["*"],"signing_secret":"secret123"}`))
	createWebhookRequest.Header.Set("Content-Type", "application/json")
	createWebhookResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(createWebhookResponseRecorder, createWebhookRequest)
	if createWebhookResponseRecorder.Code != http.StatusOK && createWebhookResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 200/201 for POST webhooks, got %d: %s", createWebhookResponseRecorder.Code, createWebhookResponseRecorder.Body.String())
	}
	var webhook Webhook
	_ = json.Unmarshal(createWebhookResponseRecorder.Body.Bytes(), &webhook)

	// GET /api/v1/_/core/webhooks
	listWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks", nil)
	listWebhookResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(listWebhookResponseRecorder, listWebhookRequest)
	if listWebhookResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET webhooks, got %d", listWebhookResponseRecorder.Code)
	}

	// GET /api/v1/_/core/webhooks/:id
	getWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/"+webhook.ID, nil)
	getWebhookResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(getWebhookResponseRecorder, getWebhookRequest)
	if getWebhookResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET webhook by ID, got %d", getWebhookResponseRecorder.Code)
	}

	// PUT /api/v1/_/core/webhooks/:id
	updateWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/webhooks/"+webhook.ID, strings.NewReader(`{"name":"Updated HTTP Webhook"}`))
	updateWebhookRequest.Header.Set("Content-Type", "application/json")
	updateWebhookResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(updateWebhookResponseRecorder, updateWebhookRequest)
	if updateWebhookResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for PUT webhook by ID, got %d", updateWebhookResponseRecorder.Code)
	}

	// GET /api/v1/_/core/webhooks/:id/deliveries
	deliveriesRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/"+webhook.ID+"/deliveries", nil)
	deliveriesResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(deliveriesResponseRecorder, deliveriesRequest)
	if deliveriesResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET webhook deliveries, got %d", deliveriesResponseRecorder.Code)
	}

	// DELETE /api/v1/_/core/webhooks/:id
	deleteWebhookDeliveryRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/webhooks/"+webhook.ID, nil)
	deleteWebhookDeliveryResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(deleteWebhookDeliveryResponseRecorder, deleteWebhookDeliveryRequest)
	if deleteWebhookDeliveryResponseRecorder.Code != http.StatusNoContent && deleteWebhookDeliveryResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 204/200 for DELETE webhook, got %d", deleteWebhookDeliveryResponseRecorder.Code)
	}

	// Create 2nd root service account to allow deleting 1st
	secondRootServiceAccount, _ := kernel.serviceAccountManager.Create(ctx, CreateServiceAccountInput{Name: "2nd Root", Scopes: []string{ScopeRoot}})
	_ = secondRootServiceAccount

	// DELETE /api/v1/_/core/service-accounts/:id
	deleteServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/service-accounts/"+createdServiceAccount.ID, nil)
	deleteServiceAccountResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(deleteServiceAccountResponseRecorder, deleteServiceAccountRequest)
	if deleteServiceAccountResponseRecorder.Code != http.StatusNoContent && deleteServiceAccountResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 204/200 for DELETE service account, got %d", deleteServiceAccountResponseRecorder.Code)
	}

	// 3. Error handling & Edge Cases for service accounts and webhooks HTTP Handlers
	// Invalid JSON on POST /service-accounts
	badJSONServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/service-accounts", strings.NewReader(`{invalid-json`))
	badJSONServiceAccountRequest.Header.Set("Content-Type", "application/json")
	badJSONServiceAccountResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(badJSONServiceAccountResponseRecorder, badJSONServiceAccountRequest)
	if badJSONServiceAccountResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", badJSONServiceAccountResponseRecorder.Code)
	}

	// Empty name on POST /service-accounts
	emptyNameServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/service-accounts", strings.NewReader(`{"name":""}`))
	emptyNameServiceAccountRequest.Header.Set("Content-Type", "application/json")
	emptyNameServiceAccountResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(emptyNameServiceAccountResponseRecorder, emptyNameServiceAccountRequest)
	if emptyNameServiceAccountResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", emptyNameServiceAccountResponseRecorder.Code)
	}

	// Not Found on GET /service-accounts/:id
	notFoundServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/service-accounts/00000000-0000-0000-0000-000000000000", nil)
	notFoundServiceAccountResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(notFoundServiceAccountResponseRecorder, notFoundServiceAccountRequest)
	if notFoundServiceAccountResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", notFoundServiceAccountResponseRecorder.Code)
	}

	// Bad JSON on PUT /service-accounts/:id
	badJSONPutServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/service-accounts/"+secondRootServiceAccount.ID, strings.NewReader(`{bad`))
	badJSONPutServiceAccountRequest.Header.Set("Content-Type", "application/json")
	badJSONPutServiceAccountResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(badJSONPutServiceAccountResponseRecorder, badJSONPutServiceAccountRequest)
	if badJSONPutServiceAccountResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", badJSONPutServiceAccountResponseRecorder.Code)
	}

	// Root Protection on DELETE last root service account
	deleteLastRootServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/service-accounts/"+secondRootServiceAccount.ID, nil)
	deleteLastRootServiceAccountResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(deleteLastRootServiceAccountResponseRecorder, deleteLastRootServiceAccountRequest)
	if deleteLastRootServiceAccountResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for deleting last root service account, got %d", deleteLastRootServiceAccountResponseRecorder.Code)
	}

	// Bad JSON on POST /webhooks
	badJSONWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/webhooks", strings.NewReader(`{bad`))
	badJSONWebhookRequest.Header.Set("Content-Type", "application/json")
	badJSONWebhookResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(badJSONWebhookResponseRecorder, badJSONWebhookRequest)
	if badJSONWebhookResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", badJSONWebhookResponseRecorder.Code)
	}

	// Not Found on GET /webhooks/:id
	notFoundWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/00000000-0000-0000-0000-000000000000", nil)
	notFoundWebhookResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(notFoundWebhookResponseRecorder, notFoundWebhookRequest)
	if notFoundWebhookResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", notFoundWebhookResponseRecorder.Code)
	}

	// Bad JSON on PUT /webhooks/:id
	badJSONPutWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/webhooks/00000000-0000-0000-0000-000000000000", strings.NewReader(`{bad`))
	badJSONPutWebhookRequest.Header.Set("Content-Type", "application/json")
	badJSONPutWebhookResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(badJSONPutWebhookResponseRecorder, badJSONPutWebhookRequest)
	if badJSONPutWebhookResponseRecorder.Code != http.StatusBadRequest && badJSONPutWebhookResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 400/404, got %d", badJSONPutWebhookResponseRecorder.Code)
	}

	// Not Found on DELETE /webhooks/:id
	notFoundDelWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/webhooks/00000000-0000-0000-0000-000000000000", nil)
	notFoundDelWebhookResponseRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(notFoundDelWebhookResponseRecorder, notFoundDelWebhookRequest)
	if notFoundDelWebhookResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", notFoundDelWebhookResponseRecorder.Code)
	}
}

func TestCoreKernelOpenAPIControllersIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, containerErr := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if containerErr != nil {
		t.Skip("docker not available")
		return
	}
	defer func() { _ = postgresContainer.Terminate(ctx) }()

	databaseURL, _ := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	db, dbErr := NewDatabasePool(ctx, databaseURL)
	if dbErr != nil {
		t.Fatalf("failed to create db connection pool: %v", dbErr)
	}
	defer db.Close()

	if migrationErr := db.RunMigrations(ctx, GetRegisteredDatabaseMigrations()); migrationErr != nil {
		t.Fatalf("failed to run migrations: %v", migrationErr)
	}

	cryptoKeyManager, _ := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	config := DefaultConfig()
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	config.Data.Enabled = true
	SetLoadedConfig(config)
	defer UnloadConfig()

	kernel := &Kernel{
		cryptoKeyManager:      cryptoKeyManager,
		db:                    db,
		webhookEventBus:       NewWebhookEventBus(db, cryptoKeyManager),
		serviceAccountManager: NewServiceAccountManager(db),
		webhookManager:        NewWebhookManager(db, cryptoKeyManager, nil),
	}

	server := NewServer(db, cryptoKeyManager)
	kernel.registerCoreRoutes(server)

	// Test 1: List Service Accounts via Control Plane Router
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/service-accounts", nil)
	responseResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(responseResponseRecorder, request)
	if responseResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from control router list service accounts, got %d: %s", responseResponseRecorder.Code, responseResponseRecorder.Body.String())
	}

	// Test 2: Create Service Account via Control Plane Router
	createServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/service-accounts", strings.NewReader(`{"name":"Router Test Service Account","scopes":["*"]}`))
	createServiceAccountRequest.Header.Set("Content-Type", "application/json")
	createServiceAccountResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(createServiceAccountResponseRecorder, createServiceAccountRequest)
	if createServiceAccountResponseRecorder.Code != http.StatusOK && createServiceAccountResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 200/201 from control router create service account, got %d: %s", createServiceAccountResponseRecorder.Code, createServiceAccountResponseRecorder.Body.String())
	}
	var createdServiceAccount ServiceAccount
	_ = json.Unmarshal(createServiceAccountResponseRecorder.Body.Bytes(), &createdServiceAccount)

	// Test 3: Get Service Account by ID
	getServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/service-accounts/"+createdServiceAccount.ID, nil)
	getServiceAccountResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(getServiceAccountResponseRecorder, getServiceAccountRequest)
	if getServiceAccountResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from get service account by ID, got %d", getServiceAccountResponseRecorder.Code)
	}

	// Test 4: Update Service Account by ID
	updateServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/service-accounts/"+createdServiceAccount.ID, strings.NewReader(`{"name":"Updated Router Service Account"}`))
	updateServiceAccountRequest.Header.Set("Content-Type", "application/json")
	updateServiceAccountResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(updateServiceAccountResponseRecorder, updateServiceAccountRequest)
	if updateServiceAccountResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from update service account, got %d: %s", updateServiceAccountResponseRecorder.Code, updateServiceAccountResponseRecorder.Body.String())
	}

	// Test 5: List Webhooks
	webhookRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks", nil)
	webhookResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(webhookResponseRecorder, webhookRequest)
	if webhookResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from list webhooks, got %d", webhookResponseRecorder.Code)
	}

	// Test 6: Create Webhook
	createWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/webhooks", strings.NewReader(`{"name":"Router WH","target_url":"http://localhost:8080/hook","events":["*"],"signing_secret":"secret123"}`))
	createWebhookRequest.Header.Set("Content-Type", "application/json")
	createWebhookResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(createWebhookResponseRecorder, createWebhookRequest)
	if createWebhookResponseRecorder.Code != http.StatusOK && createWebhookResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 200/201 from create webhook, got %d: %s", createWebhookResponseRecorder.Code, createWebhookResponseRecorder.Body.String())
	}
	var webhook Webhook
	_ = json.Unmarshal(createWebhookResponseRecorder.Body.Bytes(), &webhook)

	// Test 7: Get Webhook by ID
	getWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/"+webhook.ID, nil)
	getWebhookResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(getWebhookResponseRecorder, getWebhookRequest)
	if getWebhookResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from get webhook by ID, got %d", getWebhookResponseRecorder.Code)
	}

	// Test 8: Update Webhook
	updateWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/webhooks/"+webhook.ID, strings.NewReader(`{"name":"Updated Router WH"}`))
	updateWebhookRequest.Header.Set("Content-Type", "application/json")
	updateWebhookResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(updateWebhookResponseRecorder, updateWebhookRequest)
	if updateWebhookResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from update webhook, got %d", updateWebhookResponseRecorder.Code)
	}

	// Test 9: List Webhook Deliveries
	webhookDeliveryRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/"+webhook.ID+"/deliveries", nil)
	webhookDeliveryResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(webhookDeliveryResponseRecorder, webhookDeliveryRequest)
	if webhookDeliveryResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from list deliveries, got %d", webhookDeliveryResponseRecorder.Code)
	}

	// Test 10: Delete Webhook
	deleteWebhookDeliveryRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/webhooks/"+webhook.ID, nil)
	deleteWebhookDeliveryResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(deleteWebhookDeliveryResponseRecorder, deleteWebhookDeliveryRequest)
	if deleteWebhookDeliveryResponseRecorder.Code != http.StatusNoContent && deleteWebhookDeliveryResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 204/200 from delete webhook, got %d", deleteWebhookDeliveryResponseRecorder.Code)
	}

	// Create 2nd root service account so we can delete the first
	_, _ = kernel.serviceAccountManager.Create(ctx, CreateServiceAccountInput{Name: "2nd Root", Scopes: []string{ScopeRoot}})

	// Test 11: Delete Service Account
	deleteServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/service-accounts/"+createdServiceAccount.ID, nil)
	deleteServiceAccountResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(deleteServiceAccountResponseRecorder, deleteServiceAccountRequest)
	if deleteServiceAccountResponseRecorder.Code != http.StatusNoContent && deleteServiceAccountResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 204/200 from delete service account, got %d", deleteServiceAccountResponseRecorder.Code)
	}

	// Test Error Branches
	// Invalid Create service account body (missing name)
	badCreateServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/service-accounts", strings.NewReader(`{"name":""}`))
	badCreateServiceAccountRequest.Header.Set("Content-Type", "application/json")
	badCreateServiceAccountResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(badCreateServiceAccountResponseRecorder, badCreateServiceAccountRequest)
	if badCreateServiceAccountResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name in create service account, got %d", badCreateServiceAccountResponseRecorder.Code)
	}

	// Invalid Create Webhook body (missing name)
	badCreateWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/webhooks", strings.NewReader(`{"name":""}`))
	badCreateWebhookRequest.Header.Set("Content-Type", "application/json")
	badCreateWebhookResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(badCreateWebhookResponseRecorder, badCreateWebhookRequest)
	if badCreateWebhookResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name in create webhook, got %d", badCreateWebhookResponseRecorder.Code)
	}

	// A. Not found service account
	notFoundServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/service-accounts/00000000-0000-0000-0000-000000000000", nil)
	notFoundServiceAccountResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(notFoundServiceAccountResponseRecorder, notFoundServiceAccountRequest)
	if notFoundServiceAccountResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for not found service account, got %d", notFoundServiceAccountResponseRecorder.Code)
	}

	// B. Not found Webhook
	notFoundWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/00000000-0000-0000-0000-000000000000", nil)
	notFoundWebhookResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(notFoundWebhookResponseRecorder, notFoundWebhookRequest)
	if notFoundWebhookResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for not found webhook, got %d", notFoundWebhookResponseRecorder.Code)
	}

	// C. Update not found service account
	updateNotFoundServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/service-accounts/00000000-0000-0000-0000-000000000000", strings.NewReader(`{"name":"NF"}`))
	updateNotFoundServiceAccountRequest.Header.Set("Content-Type", "application/json")
	updateNotFoundServiceAccountResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(updateNotFoundServiceAccountResponseRecorder, updateNotFoundServiceAccountRequest)
	if updateNotFoundServiceAccountResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for update not found service account, got %d", updateNotFoundServiceAccountResponseRecorder.Code)
	}

	// D. Update not found Webhook
	updateNotFoundWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/webhooks/00000000-0000-0000-0000-000000000000", strings.NewReader(`{"name":"NF"}`))
	updateNotFoundWebhookRequest.Header.Set("Content-Type", "application/json")
	updateNotFoundWebhookResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(updateNotFoundWebhookResponseRecorder, updateNotFoundWebhookRequest)
	if updateNotFoundWebhookResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for update not found webhook, got %d", updateNotFoundWebhookResponseRecorder.Code)
	}

	// E. Delete not found service account
	deleteNotFoundServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/service-accounts/00000000-0000-0000-0000-000000000000", nil)
	deleteNotFoundServiceAccountResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(deleteNotFoundServiceAccountResponseRecorder, deleteNotFoundServiceAccountRequest)
	if deleteNotFoundServiceAccountResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for delete not found service account, got %d", deleteNotFoundServiceAccountResponseRecorder.Code)
	}

	// H. Update Root Account Disabled -> 403 Forbidden
	singleRootServiceAccount, _ := kernel.serviceAccountManager.Create(ctx, CreateServiceAccountInput{Name: "Single Root", Scopes: []string{ScopeRoot}})
	// Delete any other root accounts if exist, leaving only singleRootServiceAccount
	listServiceAccounts, _ := kernel.serviceAccountManager.List(ctx)
	for _, account := range listServiceAccounts {
		if account.ID != singleRootServiceAccount.ID && HasScope(account.Scopes, ScopeRoot) {
			_ = kernel.serviceAccountManager.Delete(ctx, account.ID)
		}
	}
	updateRootServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/service-accounts/"+singleRootServiceAccount.ID, strings.NewReader(`{"is_enabled":false}`))
	updateRootServiceAccountRequest.Header.Set("Content-Type", "application/json")
	updateRootServiceAccountResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(updateRootServiceAccountResponseRecorder, updateRootServiceAccountRequest)
	if updateRootServiceAccountResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for root account update disabling, got: %d", updateRootServiceAccountResponseRecorder.Code)
	}

	// Additional error coverage for kernel handlers with canceled context
	testCanceledContextEndpoints(t, server, webhook.ID)

	// Create service account invalid JSON body
	invalidCreateServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/service-accounts", strings.NewReader(`invalid-json`))
	invalidCreateServiceAccountResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(invalidCreateServiceAccountResponseRecorder, invalidCreateServiceAccountRequest)
	if invalidCreateServiceAccountResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON create service account, got %d", invalidCreateServiceAccountResponseRecorder.Code)
	}

	// Update service account invalid JSON body
	invalidUpdateServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/service-accounts/"+singleRootServiceAccount.ID, strings.NewReader(`invalid-json`))
	invalidUpdateServiceAccountResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(invalidUpdateServiceAccountResponseRecorder, invalidUpdateServiceAccountRequest)
	if invalidUpdateServiceAccountResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON update service account, got %d", invalidUpdateServiceAccountResponseRecorder.Code)
	}

	// Create webhook invalid JSON body
	invalidCreateWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/webhooks", strings.NewReader(`invalid-json`))
	invalidCreateWebhookResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(invalidCreateWebhookResponseRecorder, invalidCreateWebhookRequest)
	if invalidCreateWebhookResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON create webhook, got %d", invalidCreateWebhookResponseRecorder.Code)
	}

	// Update webhook invalid JSON body
	invalidUpdateWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/webhooks/"+webhook.ID, strings.NewReader(`invalid-json`))
	invalidUpdateWebhookResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(invalidUpdateWebhookResponseRecorder, invalidUpdateWebhookRequest)
	if invalidUpdateWebhookResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON update webhook, got %d", invalidUpdateWebhookResponseRecorder.Code)
	}

	// Delete not found webhook
	deleteNotFoundWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/webhooks/00000000-0000-0000-0000-000000000000", nil)
	deleteNotFoundWebhookResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(deleteNotFoundWebhookResponseRecorder, deleteNotFoundWebhookRequest)
	if deleteNotFoundWebhookResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for delete not found webhook, got %d", deleteNotFoundWebhookResponseRecorder.Code)
	}

	// Test ServiceAccountAuthMiddleware & RequireScopeMiddleware
	serviceAccount, err := kernel.serviceAccountManager.Create(ctx, CreateServiceAccountInput{
		Name:   "Middleware Service Account",
		Scopes: []string{ScopeRoot},
	})
	if err != nil {
		t.Fatalf("failed to create middleware service account: %v", err)
	}

	testHandler := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		serviceAccount := GetServiceAccount(request.Context())
		if serviceAccount == nil {
			http.Error(responseWriter, "no service account in context", http.StatusUnauthorized)
			return
		}
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(serviceAccount.ID))
	})

	// 1. Nil manager pass-through
	nilServiceAccountManagerHandler := ServiceAccountAuthMiddleware(nil)(testHandler)
	nilServiceAccountManagerRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	nilServiceAccountManagerResponseRecorder := httptest.NewRecorder()
	nilServiceAccountManagerHandler.ServeHTTP(nilServiceAccountManagerResponseRecorder, nilServiceAccountManagerRequest)
	if nilServiceAccountManagerResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when serviceAccount not in context, got %d", nilServiceAccountManagerResponseRecorder.Code)
	}

	// 2. Empty key pass-through
	handler := ServiceAccountAuthMiddleware(kernel.serviceAccountManager)(testHandler)
	noKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	noKeyResponseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(noKeyResponseRecorder, noKeyRequest)
	if noKeyResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on empty key pass through, got %d", noKeyResponseRecorder.Code)
	}

	// 3. Invalid key -> 401
	badKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	badKeyRequest.Header.Set("X-Layr-Service-Account-Key", "invalid_short_key")
	badKeyResponseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(badKeyResponseRecorder, badKeyRequest)
	if badKeyResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid key, got %d", badKeyResponseRecorder.Code)
	}

	// 4. Valid Service Account Key via X-Forwarded-For IP & Bearer Header
	validKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	validKeyRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	validKeyRequest.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")
	validKeyResponseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(validKeyResponseRecorder, validKeyRequest)
	if validKeyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid service account key, got %d", validKeyResponseRecorder.Code)
	}

	// 5. Valid Service Account Key via X-Real-IP
	validXRealRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	validXRealRequest.Header.Set("X-Layr-Service-Account-Key", serviceAccount.SecretKey)
	validXRealRequest.Header.Set("X-Real-IP", "127.0.0.1")
	validXRealResponseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(validXRealResponseRecorder, validXRealRequest)
	if validXRealResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid service account key with X-Real-IP, got %d", validXRealResponseRecorder.Code)
	}

	// 5b. Valid Service Account Key with plain RemoteAddr
	plainRemoteRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	plainRemoteRequest.Header.Set("X-Layr-Service-Account-Key", serviceAccount.SecretKey)
	plainRemoteRequest.RemoteAddr = "127.0.0.1" // no port, tests SplitHostPort fallback
	plainRemoteResponseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(plainRemoteResponseRecorder, plainRemoteRequest)
	if plainRemoteResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid service account key with plain RemoteAddr, got %d", plainRemoteResponseRecorder.Code)
	}

	// 6. Test RequireScopeMiddleware
	scopeGuardedHandler := RequireScopeMiddleware("core:service-account.read")(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	}))

	// Without service account in context -> 401
	unauthScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	unauthScopeResponseRecorder := httptest.NewRecorder()
	scopeGuardedHandler.ServeHTTP(unauthScopeResponseRecorder, unauthScopeRequest)
	if unauthScopeResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without service account in context, got %d", unauthScopeResponseRecorder.Code)
	}

	// With service account lacking scope -> 403
	scopedServiceAccount := &ServiceAccount{ID: "test", Scopes: []string{"data:query.read"}}
	insufficientCtx := WithServiceAccount(ctx, scopedServiceAccount)
	insufficientRequest := httptest.NewRequestWithContext(insufficientCtx, http.MethodGet, "/test", nil)
	insufficientResponseRecorder := httptest.NewRecorder()
	scopeGuardedHandler.ServeHTTP(insufficientResponseRecorder, insufficientRequest)
	if insufficientResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on insufficient scope, got %d", insufficientResponseRecorder.Code)
	}

	// With service account holding wildcard scope -> 200
	rootServiceAccount := &ServiceAccount{ID: "root", Scopes: []string{ScopeRoot}}
	rootCtx := WithServiceAccount(ctx, rootServiceAccount)
	rootRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/test", nil)
	rootResponseRecorder := httptest.NewRecorder()
	scopeGuardedHandler.ServeHTTP(rootResponseRecorder, rootRequest)
	if rootResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 with root service account, got %d", rootResponseRecorder.Code)
	}
}

func testCanceledContextEndpoints(t *testing.T, server *Server, webhookID string) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// List service accounts canceled context
	listCanceledRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/service-accounts", nil)
	listCanceledResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(listCanceledResponseRecorder, listCanceledRequest)
	if listCanceledResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for canceled context on list service accounts, got %d", listCanceledResponseRecorder.Code)
	}

	// List webhooks canceled context
	listWebhooksCanceledRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks", nil)
	listWebhooksCanceledResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(listWebhooksCanceledResponseRecorder, listWebhooksCanceledRequest)
	if listWebhooksCanceledResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for canceled context on list webhooks, got %d", listWebhooksCanceledResponseRecorder.Code)
	}

	// List webhook deliveries canceled context
	listDeliveriesCanceledRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/"+webhookID+"/deliveries", nil)
	listDeliveriesCanceledResponseRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(listDeliveriesCanceledResponseRecorder, listDeliveriesCanceledRequest)
	if listDeliveriesCanceledResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for canceled context on list deliveries, got %d", listDeliveriesCanceledResponseRecorder.Code)
	}
}

func TestCoreKernelExtraCoreErrorBranchesIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Skip("docker not available")
		return
	}
	defer func() { _ = postgresContainer.Terminate(ctx) }()

	databaseURL, _ := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	db, err := NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create db connection pool: %v", err)
	}
	_ = db.RunMigrations(ctx, SystemDatabaseMigrations)

	cryptoKeyManager, _ := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	webhookEventBus := NewWebhookEventBus(db, cryptoKeyManager)
	defer webhookEventBus.Close()

	serviceAccountManager := NewServiceAccountManager(db)
	webhookManager := NewWebhookManager(db, cryptoKeyManager, webhookEventBus)

	serviceAccount, err := serviceAccountManager.Create(ctx, CreateServiceAccountInput{Name: "Target Service Account"})
	if err != nil {
		t.Fatalf("create serviceAccount failed: %v", err)
	}
	webhook, err := webhookManager.Create(ctx, CreateWebhookInput{Name: "Target Webhook", TargetURL: "http://localhost:19999", Events: []string{"*"}})
	if err != nil {
		t.Fatalf("create webhook failed: %v", err)
	}

	// Fill dispatchChannel to trigger buffer full drop path in Publish
	testWebhookEventBus := &WebhookEventBus{
		dispatchChannel: make(chan WebhookEventEnvelope, 1),
	}
	testWebhookEventBus.dispatchChannel <- WebhookEventEnvelope{ID: "dummy"}
	testWebhookEventBus.Publish(ctx, WebhookEventEnvelope{ID: "overflow", Event: "overflow"})

	// Close pool to trigger query error paths in service account and webhook managers
	db.Close()

	if _, err := serviceAccountManager.Create(ctx, CreateServiceAccountInput{Name: "Fail Service Account"}); err == nil {
		t.Fatal("expected error on Create service account with closed pool")
	}
	if _, err := serviceAccountManager.Update(ctx, serviceAccount.ID, UpdateServiceAccountInput{}); err == nil {
		t.Fatal("expected error on Update service account with closed pool")
	}
	if _, err := serviceAccountManager.List(ctx); err == nil {
		t.Fatal("expected error on List service accounts with closed pool")
	}
	if _, err := serviceAccountManager.Authenticate(ctx, serviceAccount.SecretKey, "127.0.0.1"); err == nil {
		t.Fatal("expected error on Authenticate service account with closed pool")
	}

	if _, err := webhookManager.Create(ctx, CreateWebhookInput{Name: "Fail Webhook", TargetURL: "http://example.com", Events: []string{"*"}}); err == nil {
		t.Fatal("expected error on Create webhook with closed pool")
	}
	if _, err := webhookManager.Update(ctx, webhook.ID, UpdateWebhookInput{}); err == nil {
		t.Fatal("expected error on Update webhook with closed pool")
	}
	if _, err := webhookManager.List(ctx); err == nil {
		t.Fatal("expected error on List webhooks with closed pool")
	}
	if _, err := webhookManager.ListDeliveries(ctx, webhook.ID); err == nil {
		t.Fatal("expected error on ListDeliveries with closed pool")
	}
	webhookEventBus.dispatch(ctx, WebhookEventEnvelope{Event: "test.event"})
}

type coreErrReader struct{}

func (coreErrReader) Read(destinationBuffer []byte) (int, error) {
	return 0, errors.New("simulated rand reader failure")
}

func TestCoreKernelBootstrapRootAccountIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	defer func() { _ = postgresContainer.Terminate(context.WithoutCancel(ctx)) }()

	databaseURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create db connection pool: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrations(ctx, GetRegisteredDatabaseMigrations()); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	serviceAccountManager := NewServiceAccountManager(db)

	// 1. Nil pool or nil manager error
	nilKernel := &Kernel{}
	if err := nilKernel.bootstrapRootAccount(ctx); err == nil {
		t.Fatal("expected error on nil db connection pool bootstrapRootAccount")
	}

	// 2. Count error on closed pool
	closedDB, _ := NewDatabasePool(ctx, databaseURL)
	closedDB.Close()
	closedKernel := &Kernel{db: closedDB, serviceAccountManager: NewServiceAccountManager(closedDB)}
	if err := closedKernel.bootstrapRootAccount(ctx); err == nil {
		t.Fatal("expected error on closed pool count in bootstrapRootAccount")
	}

	// 3. Hasher error
	_, _ = db.Exec(ctx, "DELETE FROM core.service_accounts")
	_, _ = db.Exec(ctx, "DELETE FROM console.users")
	defaultPasswordHasher.SetRandomReader(&coreErrReader{})
	kernel := &Kernel{db: db, serviceAccountManager: serviceAccountManager}
	if err := kernel.bootstrapRootAccount(ctx); err == nil {
		t.Fatal("expected error on hasher failure")
	}
	defaultPasswordHasher.SetRandomReader(nil)

	// 4. Insert console user failure
	_, _ = db.Exec(ctx, "ALTER TABLE console.users ADD CONSTRAINT test_bootstrap_fail CHECK (email != 'root@layr.local')")
	if err := kernel.bootstrapRootAccount(ctx); err == nil {
		t.Fatal("expected error on insert console user constraint failure")
	}
	_, _ = db.Exec(ctx, "ALTER TABLE console.users DROP CONSTRAINT test_bootstrap_fail")

	// 5. Create service account failure
	_, _ = db.Exec(ctx, "ALTER TABLE core.service_accounts ADD CONSTRAINT test_service_account_fail CHECK (name != 'Root Service Account')")
	if err := kernel.bootstrapRootAccount(ctx); err == nil {
		t.Fatal("expected error on create service account constraint failure")
	}
	_, _ = db.Exec(ctx, "ALTER TABLE core.service_accounts DROP CONSTRAINT test_service_account_fail")

	// Clean up tables for successful default run
	_, _ = db.Exec(ctx, "DELETE FROM core.service_accounts")
	_, _ = db.Exec(ctx, "DELETE FROM console.users")

	// 6. Successful default bootstrap (generates random password and linked service account)
	if err := kernel.bootstrapRootAccount(ctx); err != nil {
		t.Fatalf("failed default bootstrapRootAccount: %v", err)
	}

	// Verify linkage
	var serviceAccountCount int
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM core.service_accounts WHERE console_user_id IS NOT NULL").Scan(&serviceAccountCount)
	if serviceAccountCount != 1 {
		t.Fatalf("expected 1 linked service account, got %d", serviceAccountCount)
	}

	// 7. Idempotent re-run when count > 0
	if err := kernel.bootstrapRootAccount(ctx); err != nil {
		t.Fatalf("idempotent bootstrapRootAccount failed: %v", err)
	}

	// 8. Successful custom bootstrap
	_, _ = db.Exec(ctx, "DELETE FROM core.service_accounts")
	_, _ = db.Exec(ctx, "DELETE FROM console.users")
	customConfig := DefaultConfig()
	customConfig.Console.InitialUserEmail = "console_user@custom.org"
	customConfig.Console.InitialUserPassword = "MyCustomPassword123!"
	SetLoadedConfig(customConfig)
	defer UnloadConfig()
	customKernel := &Kernel{
		db:                    db,
		serviceAccountManager: serviceAccountManager,
	}
	if err := customKernel.bootstrapRootAccount(ctx); err != nil {
		t.Fatalf("custom bootstrapRootAccount failed: %v", err)
	}
}
