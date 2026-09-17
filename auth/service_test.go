package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestAuthServiceUnit(t *testing.T) {
	ctx := context.Background()

	// 1. Verify global service factory registration via init()
	serviceFactory, exists := core.GetServiceFactory("auth")
	if !exists || serviceFactory == nil {
		t.Fatal("expected 'auth' service factory to be registered in core")
	}

	// 2. Test Service initialization with and without cryptoKeyManager
	nilKeyService := NewService(nil, nil)
	if nilKeyService == nil {
		t.Fatal("expected NewService to return a non-nil Service instance")
	}
	if nilKeyService.GetHandler() != nil {
		t.Fatal("expected nil Handler when cryptoKeyManager is nil")
	}

	cryptoKeyManager, _ := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	service := NewService(nil, cryptoKeyManager)
	if service == nil {
		t.Fatal("expected NewService with cryptoKeyManager to return a non-nil Service instance")
	}

	// 3. Test Getters
	if service.GetConfigManager() == nil {
		t.Fatal("expected GetConfigManager to return a non-nil ConfigManager")
	}
	if service.GetControlPlaneHandler() == nil {
		t.Fatal("expected GetControlPlaneHandler to return a non-nil ControlPlaneHandler")
	}
	if service.GetHandler() == nil {
		t.Fatal("expected GetHandler to return a non-nil Handler")
	}

	// 4. Test Setters on initialized service
	kvStore := newInMemoryKVStore()
	serviceAccountManager := core.NewServiceAccountManager(nil)
	eventBus := core.NewEventBus(nil, nil)

	service.SetKVStore(kvStore)
	service.SetServiceAccountManager(serviceAccountManager)
	service.SetEventBus(eventBus)

	// 5. Test Setters on empty Service struct to cover nil receiver fields
	emptyService := &Service{}
	emptyService.SetKVStore(kvStore)
	emptyService.SetServiceAccountManager(serviceAccountManager)
	emptyService.SetEventBus(eventBus)

	// 6. Test CheckScope branches
	// Branch A0: AuthContext is a service account -> evaluates HasScope directly
	m2mAuthedCtx := core.WithAuthContext(ctx, core.AuthContext{
		ServiceAccountID: "sa_worker_123",
		JWT: core.JWTClaims{
			Subject: "sa_worker_123",
			Role:    "service_role",
			Scope:   "auth:user.read",
		},
	})
	m2mValidRequest := httptest.NewRequestWithContext(m2mAuthedCtx, http.MethodGet, "/api/v1/test", nil)
	if !service.CheckScope(m2mValidRequest, "auth:user.read") {
		t.Fatal("expected CheckScope to return true when M2M AuthContext has required scope")
	}
	if service.CheckScope(m2mValidRequest, "auth:user.write") {
		t.Fatal("expected CheckScope to return false when M2M AuthContext lacks required scope")
	}

	// Branch A: serviceAccountManager is nil -> returns true
	noManagerService := &Service{}
	noManagerRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/test", nil)
	if !noManagerService.CheckScope(noManagerRequest, "auth:test.scope") {
		t.Fatal("expected CheckScope to return true when serviceAccountManager is nil")
	}

	// Branch B: serviceAccountManager present, but request has no service account key -> returns true
	noKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/test", nil)
	if !service.CheckScope(noKeyRequest, "auth:test.scope") {
		t.Fatal("expected CheckScope to return true when request has no service account key")
	}

	// Branch C: serviceAccountManager present, request has invalid secret key -> Authenticate fails -> returns false
	invalidKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/test", nil)
	invalidKeyRequest.Header.Set("Authorization", "Bearer layr_sec_invalid_key_value")
	if service.CheckScope(invalidKeyRequest, "auth:test.scope") {
		t.Fatal("expected CheckScope to return false when service account authentication fails")
	}

	// 7. Test Start with nil db (loads defaults cleanly)
	if err := service.Start(ctx); err != nil {
		t.Fatalf("expected Start with nil db to succeed, got: %v", err)
	}

	// 8. Test Stop
	if err := service.Stop(); err != nil {
		t.Fatalf("expected Stop to return nil, got: %v", err)
	}
}
