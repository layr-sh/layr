package data

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestDataControlPlaneHandlerBaseScopeIntegration(t *testing.T) {
	db, cleanup := setupTestDataDatabase(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(db)
	eventBus := core.NewEventBus(db, nil)
	defer eventBus.Close()

	service := NewService(db)
	service.SetServiceAccountManager(serviceAccountManager)
	service.SetEventBus(eventBus)
	_ = service.Start(ctx)
	defer func() { _ = service.Stop() }()

	controlPlaneHandler := service.controlPlaneHandler

	// 1. Create a service account with "data:schema.read" scope
	serviceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Read Only SA",
		Scopes: []string{"data:schema.read"},
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}

	// 2. Test valid scope check
	validRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables", nil)
	validRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	if !controlPlaneHandler.checkScope(validRequest, "data:schema.read") {
		t.Fatal("expected checkScope to return true for matching scope")
	}

	// 3. Test invalid scope check
	invalidScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/data/tables", nil)
	invalidScopeRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	if controlPlaneHandler.checkScope(invalidScopeRequest, "data:schema.write") {
		t.Fatal("expected checkScope to return false for missing scope")
	}

	// 4. Test invalid API key token
	malformedTokenRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/data/tables", nil)
	malformedTokenRequest.Header.Set("Authorization", "Bearer invalid_api_key_token")
	if controlPlaneHandler.checkScope(malformedTokenRequest, "data:schema.read") {
		t.Fatal("expected checkScope to return false for invalid token")
	}

	// 5. Test cache invalidation
	controlPlaneHandler.invalidateCache(ctx)
}
