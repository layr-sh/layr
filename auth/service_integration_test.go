package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestAuthServiceIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Initialize Service with real DB and CryptoKeyManager
	service := NewService(db, cryptoKeyManager)
	serviceAccountManager := core.NewServiceAccountManager(db)
	eventBus := core.NewEventBus(db, cryptoKeyManager)
	kvStore := newInMemoryKVStore()

	service.SetKVStore(kvStore)
	service.SetServiceAccountManager(serviceAccountManager)
	service.SetEventBus(eventBus)

	// 2. Start service (loads or seeds config in real DB)
	if err := service.Start(ctx); err != nil {
		t.Fatalf("expected Service.Start to succeed: %v", err)
	}

	// 3. Create service account in DB and test CheckScope
	createdServiceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Service Scope Verification Account",
		Scopes: []string{"auth:user.read", "auth:config.read"},
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}

	// Authorized scope
	validScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users", nil)
	validScopeRequest.Header.Set("Authorization", "Bearer "+createdServiceAccount.SecretKey)
	if !service.CheckScope(validScopeRequest, "auth:user.read") {
		t.Fatal("expected CheckScope to return true for granted scope")
	}

	// Unauthorized scope
	missingScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users", nil)
	missingScopeRequest.Header.Set("Authorization", "Bearer "+createdServiceAccount.SecretKey)
	if service.CheckScope(missingScopeRequest, "auth:user.write") {
		t.Fatal("expected CheckScope to return false for ungranted scope")
	}

	// 4. Test ServiceFactory with core.Kernel
	serviceFactory, exists := core.GetServiceFactory("auth")
	if !exists || serviceFactory == nil {
		t.Fatal("expected 'auth' service factory to exist")
	}

	kernel := &core.Kernel{}
	kernel.SetDB(db)
	kernel.SetKVStore(kvStore)
	kernel.SetEventBus(eventBus)
	kernel.SetServiceAccountManager(serviceAccountManager)

	serviceRunner, err := serviceFactory(kernel)
	if err != nil {
		t.Fatalf("expected serviceFactory to return runner without error: %v", err)
	}
	if serviceRunner == nil {
		t.Fatal("expected non-nil serviceRunner from factory")
	}
	if err := serviceRunner.Start(ctx); err != nil {
		t.Fatalf("expected factory runner Start to succeed: %v", err)
	}
	if err := serviceRunner.Stop(); err != nil {
		t.Fatalf("expected factory runner Stop to succeed: %v", err)
	}
}

func TestAuthServiceBrokenPoolIntegration(t *testing.T) {
	brokenDB := createBrokenPool(t)
	cryptoKeyManager, _ := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	ctx := context.Background()

	brokenService := NewService(brokenDB, cryptoKeyManager)
	err := brokenService.Start(ctx)
	if err == nil {
		t.Fatal("expected Service.Start to fail with broken DB pool")
	}
}
