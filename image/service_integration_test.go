package image

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestImageServiceIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()

	// 1. Initialize Service with real kernel
	service := NewService(kernel)

	// 2. Start service (loads or seeds config in real DB)
	if err := service.Start(ctx); err != nil {
		t.Fatalf("expected Service.Start to succeed: %v", err)
	}
	defer service.Stop()

	// 3. Create service account in DB and test CheckScope
	serviceAccountManager := kernel.ServiceAccountManager()
	createdServiceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Image Service Scope Verification Account",
		Scopes: []string{core.ScopeImagePresetRead, core.ScopeImageConfigRead},
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}

	// Authorized scope
	validScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/image/presets", nil)
	validScopeRequest.Header.Set("Authorization", "Bearer "+createdServiceAccount.SecretKey)
	if !serviceAccountManager.CheckScope(validScopeRequest, core.ScopeImagePresetRead) {
		t.Fatal("expected CheckScope to return true for granted scope")
	}

	// Unauthorized scope
	missingScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/image/presets", nil)
	missingScopeRequest.Header.Set("Authorization", "Bearer "+createdServiceAccount.SecretKey)
	if serviceAccountManager.CheckScope(missingScopeRequest, core.ScopeImagePresetWrite) {
		t.Fatal("expected CheckScope to return false for ungranted scope")
	}

	// 4. Test ServiceFactory with core.Kernel
	serviceFactory, exists := core.GetServiceFactory("image")
	if !exists {
		t.Fatal("expected 'image' service factory to exist")
	}

	_, factoryErr := serviceFactory(kernel)
	if factoryErr != nil {
		t.Fatalf("expected serviceFactory to return runner without error: %v", factoryErr)
	}
}
