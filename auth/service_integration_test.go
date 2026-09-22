package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestAuthServiceIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()

	// 1. Initialize Service with real kernel
	service := NewService(kernel)

	// 2. Start service (loads or seeds config in real DB)
	if err := service.Start(ctx); err != nil {
		t.Fatalf("expected Service.Start to succeed: %v", err)
	}

	// 3. Create service account in DB and test CheckScope
	serviceAccountManager := kernel.ServiceAccountManager()
	createdServiceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Service Scope Verification Account",
		Scopes: []string{core.ScopeAuthUserRead, core.ScopeAuthConfigRead},
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}

	// Authorized scope
	validScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users", nil)
	validScopeRequest.Header.Set("Authorization", "Bearer "+createdServiceAccount.SecretKey)
	if !serviceAccountManager.CheckScope(validScopeRequest, core.ScopeAuthUserRead) {
		t.Fatal("expected CheckScope to return true for granted scope")
	}

	// Unauthorized scope
	missingScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users", nil)
	missingScopeRequest.Header.Set("Authorization", "Bearer "+createdServiceAccount.SecretKey)
	if serviceAccountManager.CheckScope(missingScopeRequest, core.ScopeAuthUserWrite) {
		t.Fatal("expected CheckScope to return false for ungranted scope")
	}

	// 4. Test ServiceFactory with core.Kernel
	serviceFactory, exists := core.GetServiceFactory("auth")
	if !exists || serviceFactory == nil {
		t.Fatal("expected 'auth' service factory to exist")
	}

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
	serviceRunner.Stop()
}

func TestAuthServiceBrokenPoolIntegration(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	ctx := context.Background()

	brokenService := NewService(kernel)
	err := brokenService.Start(ctx)
	if err == nil {
		t.Fatal("expected Service.Start to fail with broken DB pool")
	}
}
