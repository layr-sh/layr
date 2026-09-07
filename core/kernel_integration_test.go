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

	pgContainer, err := tcpostgres.Run(ctx,
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
		_ = pgContainer.Terminate(ctx)
	}()

	databaseURL, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")

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
	startContext, startCancel := context.WithCancel(ctx)
	defer startCancel()

	go func() {
		errChannel <- kernel.Start(startContext)
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

	pgContainer, err := tcpostgres.Run(ctx,
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
	defer func() { _ = pgContainer.Terminate(ctx) }()

	databaseURL, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")

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
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(os.Interrupt)

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

	pgContainer, err := tcpostgres.Run(ctx,
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
	defer func() { _ = pgContainer.Terminate(ctx) }()

	databaseURL, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")

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

	pgContainer, err := tcpostgres.Run(ctx,
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
	defer func() { _ = pgContainer.Terminate(ctx) }()

	databaseURL, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")

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

	pgContainer, err := tcpostgres.Run(ctx,
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
	defer func() { _ = pgContainer.Terminate(ctx) }()

	databaseURL, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")

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
	runnerContext, runnerCancel := context.WithCancel(ctx)
	errChannel := make(chan error, 1)
	go func() {
		errChannel <- runnerKernel.Start(runnerContext)
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

	pgContainer, err := tcpostgres.Run(ctx,
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
	defer func() { _ = pgContainer.Terminate(ctx) }()

	databaseURL, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")

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
	oldDataFactory, hadOldData := GetServiceFactory("data")
	RegisterServiceFactory("data", func(k *Kernel) (ServiceRunner, error) {
		return nil, errors.New("data factory failure")
	})
	defer func() {
		if hadOldData {
			RegisterServiceFactory("data", oldDataFactory)
		}
	}()

	failingFactoryKernel, _ := NewKernel()
	if err := failingFactoryKernel.Start(ctx); err == nil {
		t.Fatal("expected kernel start failure on failing service factory")
	}

	// Test failing bootstrap root console user during kernel.Start
	if cleanupPool, err := NewDatabasePool(ctx, databaseURL); err == nil && cleanupPool != nil {
		_, _ = cleanupPool.Exec(ctx, "DELETE FROM console.users")
		cleanupPool.Close()
	}

	defaultPasswordHasher.SetRandomReader(&coreErrReader{})
	failingBootstrapKernel, _ := NewKernel()
	if err := failingBootstrapKernel.Start(ctx); err == nil {
		t.Fatal("expected kernel start failure on failing bootstrap user")
	}
	defaultPasswordHasher.SetRandomReader(nil)

	// Test failing bootstrap root service account during kernel.Start
	if cleanupPool, err := NewDatabasePool(ctx, databaseURL); err == nil && cleanupPool != nil {
		_, _ = cleanupPool.Exec(ctx, "DELETE FROM core.service_accounts")
		_, _ = cleanupPool.Exec(ctx, "ALTER TABLE core.service_accounts ADD CONSTRAINT fail_service_account_bootstrap CHECK (name != 'Root Service Account')")
		cleanupPool.Close()
	}

	failingServiceAccountBootstrapKernel, _ := NewKernel()
	if err := failingServiceAccountBootstrapKernel.Start(ctx); err == nil {
		t.Fatal("expected kernel start failure on failing bootstrap service account")
	}

	if cleanupPool, err := NewDatabasePool(ctx, databaseURL); err == nil && cleanupPool != nil {
		_, _ = cleanupPool.Exec(ctx, "ALTER TABLE core.service_accounts DROP CONSTRAINT fail_service_account_bootstrap")
		cleanupPool.Close()
	}

	// Test successful service factory auto-instantiation
	successAutoRunner := &mockServiceRunner{}
	RegisterServiceFactory("data", func(kernel *Kernel) (ServiceRunner, error) {
		return successAutoRunner, nil
	})
	successFactoryKernel, _ := NewKernel()
	factoryContext, factoryCancel := context.WithCancel(ctx)
	autoErrorChannel := make(chan error, 1)
	go func() {
		autoErrorChannel <- successFactoryKernel.Start(factoryContext)
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

	pgContainer, containerErr := tcpostgres.Run(ctx,
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
	defer func() { _ = pgContainer.Terminate(ctx) }()

	databaseURL, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
	db, poolErr := NewDatabasePool(ctx, databaseURL)
	if poolErr != nil {
		t.Fatalf("failed to create db connection pool: %v", poolErr)
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
	createServiceAccountRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(createServiceAccountRecorder, createServiceAccountRequest)
	if createServiceAccountRecorder.Code != http.StatusOK && createServiceAccountRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 200/201 for POST service accounts, got %d: %s", createServiceAccountRecorder.Code, createServiceAccountRecorder.Body.String())
	}
	var serviceAccountRes CreateServiceAccountResult
	_ = json.Unmarshal(createServiceAccountRecorder.Body.Bytes(), &serviceAccountRes)

	// GET /api/v1/_/core/service-accounts
	listServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/service-accounts", nil)
	listServiceAccountRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(listServiceAccountRecorder, listServiceAccountRequest)
	if listServiceAccountRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET service accounts, got %d", listServiceAccountRecorder.Code)
	}

	// GET /api/v1/_/core/service-accounts/:id
	getServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/service-accounts/"+serviceAccountRes.ID, nil)
	getServiceAccountRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(getServiceAccountRecorder, getServiceAccountRequest)
	if getServiceAccountRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET service account by ID, got %d", getServiceAccountRecorder.Code)
	}

	// PUT /api/v1/_/core/service-accounts/:id
	updateServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/service-accounts/"+serviceAccountRes.ID, strings.NewReader(`{"name":"Updated HTTP Service Account"}`))
	updateServiceAccountRequest.Header.Set("Content-Type", "application/json")
	updateServiceAccountRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(updateServiceAccountRecorder, updateServiceAccountRequest)
	if updateServiceAccountRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for PUT service account by ID, got %d", updateServiceAccountRecorder.Code)
	}

	// 2. Webhooks HTTP API
	// POST /api/v1/_/core/webhooks
	createWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/webhooks", strings.NewReader(`{"name":"HTTP Webhook","target_url":"http://localhost:8080/webhook","events":["*"],"signing_secret":"secret123"}`))
	createWebhookRequest.Header.Set("Content-Type", "application/json")
	createWebhookRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(createWebhookRecorder, createWebhookRequest)
	if createWebhookRecorder.Code != http.StatusOK && createWebhookRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 200/201 for POST webhooks, got %d: %s", createWebhookRecorder.Code, createWebhookRecorder.Body.String())
	}
	var webhookRes WebhookSubscription
	_ = json.Unmarshal(createWebhookRecorder.Body.Bytes(), &webhookRes)

	// GET /api/v1/_/core/webhooks
	listWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks", nil)
	listWebhookRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(listWebhookRecorder, listWebhookRequest)
	if listWebhookRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET webhooks, got %d", listWebhookRecorder.Code)
	}

	// GET /api/v1/_/core/webhooks/:id
	getWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/"+webhookRes.ID, nil)
	getWebhookRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(getWebhookRecorder, getWebhookRequest)
	if getWebhookRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET webhook by ID, got %d", getWebhookRecorder.Code)
	}

	// PUT /api/v1/_/core/webhooks/:id
	updateWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/webhooks/"+webhookRes.ID, strings.NewReader(`{"name":"Updated HTTP Webhook"}`))
	updateWebhookRequest.Header.Set("Content-Type", "application/json")
	updateWebhookRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(updateWebhookRecorder, updateWebhookRequest)
	if updateWebhookRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for PUT webhook by ID, got %d", updateWebhookRecorder.Code)
	}

	// GET /api/v1/_/core/webhooks/:id/deliveries
	deliveriesRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/"+webhookRes.ID+"/deliveries", nil)
	deliveriesRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(deliveriesRecorder, deliveriesRequest)
	if deliveriesRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET webhook deliveries, got %d", deliveriesRecorder.Code)
	}

	// DELETE /api/v1/_/core/webhooks/:id
	deleteWebhookDeliveryRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/webhooks/"+webhookRes.ID, nil)
	deleteWebhookDeliveryRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(deleteWebhookDeliveryRecorder, deleteWebhookDeliveryRequest)
	if deleteWebhookDeliveryRecorder.Code != http.StatusNoContent && deleteWebhookDeliveryRecorder.Code != http.StatusOK {
		t.Fatalf("expected 204/200 for DELETE webhook, got %d", deleteWebhookDeliveryRecorder.Code)
	}

	// Create 2nd root service account to allow deleting 1st
	secondRoot, _ := kernel.serviceAccountManager.Create(ctx, CreateServiceAccountInput{Name: "2nd Root", Scopes: []string{ScopeRoot}})
	_ = secondRoot

	// DELETE /api/v1/_/core/service-accounts/:id
	deleteServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/service-accounts/"+serviceAccountRes.ID, nil)
	deleteServiceAccountRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(deleteServiceAccountRecorder, deleteServiceAccountRequest)
	if deleteServiceAccountRecorder.Code != http.StatusNoContent && deleteServiceAccountRecorder.Code != http.StatusOK {
		t.Fatalf("expected 204/200 for DELETE service account, got %d", deleteServiceAccountRecorder.Code)
	}

	// 3. Error handling & Edge Cases for service accounts and webhooks HTTP Handlers
	// Invalid JSON on POST /service-accounts
	badJSONServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/service-accounts", strings.NewReader(`{invalid-json`))
	badJSONServiceAccountRequest.Header.Set("Content-Type", "application/json")
	badJSONServiceAccountRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(badJSONServiceAccountRecorder, badJSONServiceAccountRequest)
	if badJSONServiceAccountRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", badJSONServiceAccountRecorder.Code)
	}

	// Empty name on POST /service-accounts
	emptyNameServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/service-accounts", strings.NewReader(`{"name":""}`))
	emptyNameServiceAccountRequest.Header.Set("Content-Type", "application/json")
	emptyNameServiceAccountRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(emptyNameServiceAccountRecorder, emptyNameServiceAccountRequest)
	if emptyNameServiceAccountRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", emptyNameServiceAccountRecorder.Code)
	}

	// Not Found on GET /service-accounts/:id
	notFoundServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/service-accounts/00000000-0000-0000-0000-000000000000", nil)
	notFoundServiceAccountRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(notFoundServiceAccountRecorder, notFoundServiceAccountRequest)
	if notFoundServiceAccountRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", notFoundServiceAccountRecorder.Code)
	}

	// Bad JSON on PUT /service-accounts/:id
	badJSONPutServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/service-accounts/"+secondRoot.ID, strings.NewReader(`{bad`))
	badJSONPutServiceAccountRequest.Header.Set("Content-Type", "application/json")
	badJSONPutServiceAccountRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(badJSONPutServiceAccountRecorder, badJSONPutServiceAccountRequest)
	if badJSONPutServiceAccountRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", badJSONPutServiceAccountRecorder.Code)
	}

	// Root Protection on DELETE last root service account
	deleteLastRootServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/service-accounts/"+secondRoot.ID, nil)
	deleteLastRootServiceAccountRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(deleteLastRootServiceAccountRecorder, deleteLastRootServiceAccountRequest)
	if deleteLastRootServiceAccountRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for deleting last root service account, got %d", deleteLastRootServiceAccountRecorder.Code)
	}

	// Bad JSON on POST /webhooks
	badJSONWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/webhooks", strings.NewReader(`{bad`))
	badJSONWebhookRequest.Header.Set("Content-Type", "application/json")
	badJSONWebhookRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(badJSONWebhookRecorder, badJSONWebhookRequest)
	if badJSONWebhookRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", badJSONWebhookRecorder.Code)
	}

	// Not Found on GET /webhooks/:id
	notFoundWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/00000000-0000-0000-0000-000000000000", nil)
	notFoundWebhookRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(notFoundWebhookRecorder, notFoundWebhookRequest)
	if notFoundWebhookRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", notFoundWebhookRecorder.Code)
	}

	// Bad JSON on PUT /webhooks/:id
	badJSONPutWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/webhooks/00000000-0000-0000-0000-000000000000", strings.NewReader(`{bad`))
	badJSONPutWebhookRequest.Header.Set("Content-Type", "application/json")
	badJSONPutWebhookRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(badJSONPutWebhookRecorder, badJSONPutWebhookRequest)
	if badJSONPutWebhookRecorder.Code != http.StatusBadRequest && badJSONPutWebhookRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 400/404, got %d", badJSONPutWebhookRecorder.Code)
	}

	// Not Found on DELETE /webhooks/:id
	notFoundDelWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/webhooks/00000000-0000-0000-0000-000000000000", nil)
	notFoundDelWebhookRecorder := httptest.NewRecorder()
	server.Mux().ServeHTTP(notFoundDelWebhookRecorder, notFoundDelWebhookRequest)
	if notFoundDelWebhookRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", notFoundDelWebhookRecorder.Code)
	}
}

func TestCoreKernelOpenAPIControllersIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	pgContainer, containerErr := tcpostgres.Run(ctx,
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
	defer func() { _ = pgContainer.Terminate(ctx) }()

	databaseURL, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
	db, poolErr := NewDatabasePool(ctx, databaseURL)
	if poolErr != nil {
		t.Fatalf("failed to create db connection pool: %v", poolErr)
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
	recorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from control router list service accounts, got %d: %s", recorder.Code, recorder.Body.String())
	}

	// Test 2: Create Service Account via Control Plane Router
	createServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/service-accounts", strings.NewReader(`{"name":"Router Test Service Account","scopes":["*"]}`))
	createServiceAccountRequest.Header.Set("Content-Type", "application/json")
	createServiceAccountRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(createServiceAccountRecorder, createServiceAccountRequest)
	if createServiceAccountRecorder.Code != http.StatusOK && createServiceAccountRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 200/201 from control router create service account, got %d: %s", createServiceAccountRecorder.Code, createServiceAccountRecorder.Body.String())
	}
	var serviceAccountRes CreateServiceAccountResult
	_ = json.Unmarshal(createServiceAccountRecorder.Body.Bytes(), &serviceAccountRes)

	// Test 3: Get Service Account by ID
	getServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/service-accounts/"+serviceAccountRes.ID, nil)
	getServiceAccountRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(getServiceAccountRecorder, getServiceAccountRequest)
	if getServiceAccountRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from get service account by ID, got %d", getServiceAccountRecorder.Code)
	}

	// Test 4: Update Service Account by ID
	updateServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/service-accounts/"+serviceAccountRes.ID, strings.NewReader(`{"name":"Updated Router Service Account"}`))
	updateServiceAccountRequest.Header.Set("Content-Type", "application/json")
	updateServiceAccountRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(updateServiceAccountRecorder, updateServiceAccountRequest)
	if updateServiceAccountRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from update service account, got %d: %s", updateServiceAccountRecorder.Code, updateServiceAccountRecorder.Body.String())
	}

	// Test 5: List Webhooks
	webhookRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks", nil)
	webhookRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(webhookRecorder, webhookRequest)
	if webhookRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from list webhooks, got %d", webhookRecorder.Code)
	}

	// Test 6: Create Webhook
	createWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/webhooks", strings.NewReader(`{"name":"Router WH","target_url":"http://localhost:8080/hook","events":["*"],"signing_secret":"secret123"}`))
	createWebhookRequest.Header.Set("Content-Type", "application/json")
	createWebhookRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(createWebhookRecorder, createWebhookRequest)
	if createWebhookRecorder.Code != http.StatusOK && createWebhookRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 200/201 from create webhook, got %d: %s", createWebhookRecorder.Code, createWebhookRecorder.Body.String())
	}
	var webhookRes WebhookSubscription
	_ = json.Unmarshal(createWebhookRecorder.Body.Bytes(), &webhookRes)

	// Test 7: Get Webhook by ID
	getWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/"+webhookRes.ID, nil)
	getWebhookRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(getWebhookRecorder, getWebhookRequest)
	if getWebhookRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from get webhook by ID, got %d", getWebhookRecorder.Code)
	}

	// Test 8: Update Webhook
	updateWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/webhooks/"+webhookRes.ID, strings.NewReader(`{"name":"Updated Router WH"}`))
	updateWebhookRequest.Header.Set("Content-Type", "application/json")
	updateWebhookRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(updateWebhookRecorder, updateWebhookRequest)
	if updateWebhookRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from update webhook, got %d", updateWebhookRecorder.Code)
	}

	// Test 9: List Webhook Deliveries
	webhookDeliveryRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/"+webhookRes.ID+"/deliveries", nil)
	webhookDeliveryRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(webhookDeliveryRecorder, webhookDeliveryRequest)
	if webhookDeliveryRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from list deliveries, got %d", webhookDeliveryRecorder.Code)
	}

	// Test 10: Delete Webhook
	deleteWebhookDeliveryRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/webhooks/"+webhookRes.ID, nil)
	deleteWebhookDeliveryRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(deleteWebhookDeliveryRecorder, deleteWebhookDeliveryRequest)
	if deleteWebhookDeliveryRecorder.Code != http.StatusNoContent && deleteWebhookDeliveryRecorder.Code != http.StatusOK {
		t.Fatalf("expected 204/200 from delete webhook, got %d", deleteWebhookDeliveryRecorder.Code)
	}

	// Create 2nd root service account so we can delete the first
	_, _ = kernel.serviceAccountManager.Create(ctx, CreateServiceAccountInput{Name: "2nd Root", Scopes: []string{ScopeRoot}})

	// Test 11: Delete Service Account
	deleteServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/service-accounts/"+serviceAccountRes.ID, nil)
	deleteServiceAccountRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(deleteServiceAccountRecorder, deleteServiceAccountRequest)
	if deleteServiceAccountRecorder.Code != http.StatusNoContent && deleteServiceAccountRecorder.Code != http.StatusOK {
		t.Fatalf("expected 204/200 from delete service account, got %d", deleteServiceAccountRecorder.Code)
	}

	// Test Error Branches
	// Invalid Create service account body (missing name)
	badCreateServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/service-accounts", strings.NewReader(`{"name":""}`))
	badCreateServiceAccountRequest.Header.Set("Content-Type", "application/json")
	badCreateServiceAccountRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(badCreateServiceAccountRecorder, badCreateServiceAccountRequest)
	if badCreateServiceAccountRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name in create service account, got %d", badCreateServiceAccountRecorder.Code)
	}

	// Invalid Create Webhook body (missing name)
	badCreateWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/webhooks", strings.NewReader(`{"name":""}`))
	badCreateWebhookRequest.Header.Set("Content-Type", "application/json")
	badCreateWebhookRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(badCreateWebhookRecorder, badCreateWebhookRequest)
	if badCreateWebhookRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name in create webhook, got %d", badCreateWebhookRecorder.Code)
	}

	// A. Not found service account
	notFoundServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/service-accounts/00000000-0000-0000-0000-000000000000", nil)
	notFoundServiceAccountRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(notFoundServiceAccountRecorder, notFoundServiceAccountRequest)
	if notFoundServiceAccountRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for not found service account, got %d", notFoundServiceAccountRecorder.Code)
	}

	// B. Not found Webhook
	notFoundWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/00000000-0000-0000-0000-000000000000", nil)
	notFoundWebhookRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(notFoundWebhookRecorder, notFoundWebhookRequest)
	if notFoundWebhookRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for not found webhook, got %d", notFoundWebhookRecorder.Code)
	}

	// C. Update not found service account
	updateNotFoundServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/service-accounts/00000000-0000-0000-0000-000000000000", strings.NewReader(`{"name":"NF"}`))
	updateNotFoundServiceAccountRequest.Header.Set("Content-Type", "application/json")
	updateNotFoundServiceAccountRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(updateNotFoundServiceAccountRecorder, updateNotFoundServiceAccountRequest)
	if updateNotFoundServiceAccountRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for update not found service account, got %d", updateNotFoundServiceAccountRecorder.Code)
	}

	// D. Update not found Webhook
	updateNotFoundWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/webhooks/00000000-0000-0000-0000-000000000000", strings.NewReader(`{"name":"NF"}`))
	updateNotFoundWebhookRequest.Header.Set("Content-Type", "application/json")
	updateNotFoundWebhookRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(updateNotFoundWebhookRecorder, updateNotFoundWebhookRequest)
	if updateNotFoundWebhookRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for update not found webhook, got %d", updateNotFoundWebhookRecorder.Code)
	}

	// E. Delete not found service account
	deleteNotFoundServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/service-accounts/00000000-0000-0000-0000-000000000000", nil)
	deleteNotFoundServiceAccountRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(deleteNotFoundServiceAccountRecorder, deleteNotFoundServiceAccountRequest)
	if deleteNotFoundServiceAccountRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for delete not found service account, got %d", deleteNotFoundServiceAccountRecorder.Code)
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
	updateRootServiceAccountRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(updateRootServiceAccountRecorder, updateRootServiceAccountRequest)
	if updateRootServiceAccountRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for root account update disabling, got: %d", updateRootServiceAccountRecorder.Code)
	}

	// Additional error coverage for kernel handlers with canceled context
	testCanceledContextEndpoints(t, server, webhookRes.ID)

	// Create service account invalid JSON body
	invalidCreateServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/service-accounts", strings.NewReader(`invalid-json`))
	invalidCreateServiceAccountRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(invalidCreateServiceAccountRecorder, invalidCreateServiceAccountRequest)
	if invalidCreateServiceAccountRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON create service account, got %d", invalidCreateServiceAccountRecorder.Code)
	}

	// Update service account invalid JSON body
	invalidUpdateServiceAccountRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/service-accounts/"+singleRootServiceAccount.ID, strings.NewReader(`invalid-json`))
	invalidUpdateServiceAccountRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(invalidUpdateServiceAccountRecorder, invalidUpdateServiceAccountRequest)
	if invalidUpdateServiceAccountRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON update service account, got %d", invalidUpdateServiceAccountRecorder.Code)
	}

	// Create webhook invalid JSON body
	invalidCreateWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/core/webhooks", strings.NewReader(`invalid-json`))
	invalidCreateWebhookRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(invalidCreateWebhookRecorder, invalidCreateWebhookRequest)
	if invalidCreateWebhookRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON create webhook, got %d", invalidCreateWebhookRecorder.Code)
	}

	// Update webhook invalid JSON body
	invalidUpdateWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/_/core/webhooks/"+webhookRes.ID, strings.NewReader(`invalid-json`))
	invalidUpdateWebhookRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(invalidUpdateWebhookRecorder, invalidUpdateWebhookRequest)
	if invalidUpdateWebhookRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON update webhook, got %d", invalidUpdateWebhookRecorder.Code)
	}

	// Delete not found webhook
	deleteNotFoundWebhookRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/_/core/webhooks/00000000-0000-0000-0000-000000000000", nil)
	deleteNotFoundWebhookRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(deleteNotFoundWebhookRecorder, deleteNotFoundWebhookRequest)
	if deleteNotFoundWebhookRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for delete not found webhook, got %d", deleteNotFoundWebhookRecorder.Code)
	}

	// Test ServiceAccountAuthMiddleware & RequireScopeMiddleware
	serviceAccount, err := kernel.serviceAccountManager.Create(ctx, CreateServiceAccountInput{
		Name:   "Middleware Service Account",
		Scopes: []string{ScopeRoot},
	})
	if err != nil {
		t.Fatalf("failed to create middleware service account: %v", err)
	}

	testHandler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serviceAccount := GetServiceAccount(request.Context())
		if serviceAccount == nil {
			http.Error(writer, "no service account in context", http.StatusUnauthorized)
			return
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(serviceAccount.ID))
	})

	// 1. Nil manager pass-through
	nilServiceAccountManagerChain := ServiceAccountAuthMiddleware(nil)(testHandler)
	nilServiceAccountManagerRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	nilServiceAccountManagerRecorder := httptest.NewRecorder()
	nilServiceAccountManagerChain.ServeHTTP(nilServiceAccountManagerRecorder, nilServiceAccountManagerRequest)
	if nilServiceAccountManagerRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when serviceAccount not in context, got %d", nilServiceAccountManagerRecorder.Code)
	}

	// 2. Empty key pass-through
	chain := ServiceAccountAuthMiddleware(kernel.serviceAccountManager)(testHandler)
	noKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	noKeyRecorder := httptest.NewRecorder()
	chain.ServeHTTP(noKeyRecorder, noKeyRequest)
	if noKeyRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on empty key pass through, got %d", noKeyRecorder.Code)
	}

	// 3. Invalid key -> 401
	badKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	badKeyRequest.Header.Set("X-Layr-Service-Account-Key", "invalid_short_key")
	badKeyRecorder := httptest.NewRecorder()
	chain.ServeHTTP(badKeyRecorder, badKeyRequest)
	if badKeyRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid key, got %d", badKeyRecorder.Code)
	}

	// 4. Valid Service Account Key via X-Forwarded-For IP & Bearer Header
	validKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	validKeyRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	validKeyRequest.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")
	validKeyRecorder := httptest.NewRecorder()
	chain.ServeHTTP(validKeyRecorder, validKeyRequest)
	if validKeyRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid service account key, got %d", validKeyRecorder.Code)
	}

	// 5. Valid Service Account Key via X-Real-IP
	validXRealRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	validXRealRequest.Header.Set("X-Layr-Service-Account-Key", serviceAccount.SecretKey)
	validXRealRequest.Header.Set("X-Real-IP", "127.0.0.1")
	validXRealRecorder := httptest.NewRecorder()
	chain.ServeHTTP(validXRealRecorder, validXRealRequest)
	if validXRealRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid service account key with X-Real-IP, got %d", validXRealRecorder.Code)
	}

	// 5b. Valid Service Account Key with plain RemoteAddr
	plainRemoteRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	plainRemoteRequest.Header.Set("X-Layr-Service-Account-Key", serviceAccount.SecretKey)
	plainRemoteRequest.RemoteAddr = "127.0.0.1" // no port, tests SplitHostPort fallback
	plainRemoteRecorder := httptest.NewRecorder()
	chain.ServeHTTP(plainRemoteRecorder, plainRemoteRequest)
	if plainRemoteRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid service account key with plain RemoteAddr, got %d", plainRemoteRecorder.Code)
	}

	// 6. Test RequireScopeMiddleware
	scopeGuardedHandler := RequireScopeMiddleware("core:service-account.read")(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))

	// Without service account in context -> 401
	unauthScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	unauthScopeRecorder := httptest.NewRecorder()
	scopeGuardedHandler.ServeHTTP(unauthScopeRecorder, unauthScopeRequest)
	if unauthScopeRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without service account in context, got %d", unauthScopeRecorder.Code)
	}

	// With service account lacking scope -> 403
	scopedServiceAccount := &ServiceAccount{ID: "test", Scopes: []string{"data:query.read"}}
	insufficientContext := WithServiceAccount(ctx, scopedServiceAccount)
	insufficientRequest := httptest.NewRequestWithContext(insufficientContext, http.MethodGet, "/test", nil)
	insufficientRecorder := httptest.NewRecorder()
	scopeGuardedHandler.ServeHTTP(insufficientRecorder, insufficientRequest)
	if insufficientRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on insufficient scope, got %d", insufficientRecorder.Code)
	}

	// With service account holding wildcard scope -> 200
	rootServiceAccount := &ServiceAccount{ID: "root", Scopes: []string{ScopeRoot}}
	rootContext := WithServiceAccount(ctx, rootServiceAccount)
	rootRequest := httptest.NewRequestWithContext(rootContext, http.MethodGet, "/test", nil)
	rootRecorder := httptest.NewRecorder()
	scopeGuardedHandler.ServeHTTP(rootRecorder, rootRequest)
	if rootRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 with root service account, got %d", rootRecorder.Code)
	}
}

func testCanceledContextEndpoints(t *testing.T, server *HTTPServer, webhookID string) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// List service accounts canceled context
	listCanceledRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/service-accounts", nil)
	listCanceledRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(listCanceledRecorder, listCanceledRequest)
	if listCanceledRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for canceled context on list service accounts, got %d", listCanceledRecorder.Code)
	}

	// List webhooks canceled context
	listWebhooksCanceledRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks", nil)
	listWebhooksCanceledRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(listWebhooksCanceledRecorder, listWebhooksCanceledRequest)
	if listWebhooksCanceledRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for canceled context on list webhooks, got %d", listWebhooksCanceledRecorder.Code)
	}

	// List webhook deliveries canceled context
	listDeliveriesCanceledRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/core/webhooks/"+webhookID+"/deliveries", nil)
	listDeliveriesCanceledRecorder := httptest.NewRecorder()
	server.ControlPlaneRouter().Mux().ServeHTTP(listDeliveriesCanceledRecorder, listDeliveriesCanceledRequest)
	if listDeliveriesCanceledRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for canceled context on list deliveries, got %d", listDeliveriesCanceledRecorder.Code)
	}
}

func TestCoreKernelExtraCoreErrorBranchesIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	pgContainer, err := tcpostgres.Run(ctx,
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
	defer func() { _ = pgContainer.Terminate(ctx) }()

	databaseURL, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
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

func (coreErrReader) Read(p []byte) (int, error) {
	return 0, errors.New("simulated rand reader failure")
}

func TestCoreKernelBootstrapRootAccountIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	pgContainer, err := tcpostgres.Run(ctx,
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
	defer func() { _ = pgContainer.Terminate(context.WithoutCancel(ctx)) }()

	databaseURL, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
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
	kernelInstance := &Kernel{db: db, serviceAccountManager: serviceAccountManager}
	if err := kernelInstance.bootstrapRootAccount(ctx); err == nil {
		t.Fatal("expected error on hasher failure")
	}
	defaultPasswordHasher.SetRandomReader(nil)

	// 4. Insert console user failure
	_, _ = db.Exec(ctx, "ALTER TABLE console.users ADD CONSTRAINT test_bootstrap_fail CHECK (email != 'root@layr.local')")
	if err := kernelInstance.bootstrapRootAccount(ctx); err == nil {
		t.Fatal("expected error on insert console user constraint failure")
	}
	_, _ = db.Exec(ctx, "ALTER TABLE console.users DROP CONSTRAINT test_bootstrap_fail")

	// 5. Create service account failure
	_, _ = db.Exec(ctx, "ALTER TABLE core.service_accounts ADD CONSTRAINT test_service_account_fail CHECK (name != 'Root Service Account')")
	if err := kernelInstance.bootstrapRootAccount(ctx); err == nil {
		t.Fatal("expected error on create service account constraint failure")
	}
	_, _ = db.Exec(ctx, "ALTER TABLE core.service_accounts DROP CONSTRAINT test_service_account_fail")

	// Clean up tables for successful default run
	_, _ = db.Exec(ctx, "DELETE FROM core.service_accounts")
	_, _ = db.Exec(ctx, "DELETE FROM console.users")

	// 6. Successful default bootstrap (generates random password and linked service account)
	if err := kernelInstance.bootstrapRootAccount(ctx); err != nil {
		t.Fatalf("failed default bootstrapRootAccount: %v", err)
	}

	// Verify linkage
	var serviceAccountCount int
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM core.service_accounts WHERE console_user_id IS NOT NULL").Scan(&serviceAccountCount)
	if serviceAccountCount != 1 {
		t.Fatalf("expected 1 linked service account, got %d", serviceAccountCount)
	}

	// 7. Idempotent re-run when count > 0
	if err := kernelInstance.bootstrapRootAccount(ctx); err != nil {
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
