package data

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestDataControlPlaneHandlerBaseScopeIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	_ = service.Start(ctx)
	defer func() { service.Stop() }()

	controlPlaneHandler := service.controlPlaneHandler
	serviceAccountManager := kernel.ServiceAccountManager()

	// 1. Create a service account with ScopeDataSchemaRead scope
	serviceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Read Only SA",
		Scopes: []string{core.ScopeDataSchemaRead},
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}

	// 2. Test valid scope check
	validRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables", nil)
	validRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	validResponseRecorder := httptest.NewRecorder()
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(validResponseRecorder, validRequest, core.ScopeDataSchemaRead) {
		t.Fatal("expected RequireScope to return true for matching scope")
	}

	// 3. Test invalid scope check
	invalidScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables", nil)
	invalidScopeRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	invalidScopeResponseRecorder := httptest.NewRecorder()
	if controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(invalidScopeResponseRecorder, invalidScopeRequest, core.ScopeDataSchemaWrite) {
		t.Fatal("expected RequireScope to return false for missing scope")
	}

	// 4. Test invalid API key token
	malformedTokenRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables", nil)
	malformedTokenRequest.Header.Set("Authorization", "Bearer invalid_api_key_token")
	malformedTokenResponseRecorder := httptest.NewRecorder()
	if controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(malformedTokenResponseRecorder, malformedTokenRequest, core.ScopeDataSchemaRead) {
		t.Fatal("expected RequireScope to return false for invalid token")
	}

	// 5. Test cache invalidation
	controlPlaneHandler.InvalidateCache(ctx, InvalidateCacheInput{All: true})
}
