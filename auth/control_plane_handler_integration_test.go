package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestAuthControlPlaneHandlerIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	serviceAccountManager := core.NewServiceAccountManager(db)
	eventBus := core.NewEventBus(db, cryptoKeyManager)
	kvStore := newInMemoryKVStore()

	createdServiceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Auth Base Control Plane Service Account",
		Scopes: []string{"auth:user.read"},
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}

	configManager := NewConfigManager(db, cryptoKeyManager)
	controlPlaneHandler := NewControlPlaneHandler(db, configManager)
	controlPlaneHandler.SetKVStore(kvStore)
	controlPlaneHandler.SetEventBus(eventBus)
	controlPlaneHandler.SetServiceAccountManager(serviceAccountManager)

	// Valid scope check
	validRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users", nil)
	validRequest.Header.Set("Authorization", "Bearer "+createdServiceAccount.SecretKey)
	if !controlPlaneHandler.checkScope(validRequest, "auth:user.read") {
		t.Fatal("expected checkScope to pass with valid service account and scope")
	}

	// Missing scope check
	if controlPlaneHandler.checkScope(validRequest, "auth:user.write") {
		t.Fatal("expected checkScope to fail when required scope is not granted")
	}

	// M2M AuthContext scope checks
	m2mAuthedCtx := core.WithAuthContext(ctx, core.AuthContext{
		ServiceAccountID: createdServiceAccount.ID,
		JWT: core.JWTClaims{
			Subject: createdServiceAccount.ID,
			Role:    "service_role",
			Scope:   "auth:user.read",
		},
	})
	m2mValidRequest := httptest.NewRequestWithContext(m2mAuthedCtx, http.MethodGet, "/v1/_/auth/users", nil)
	if !controlPlaneHandler.checkScope(m2mValidRequest, "auth:user.read") {
		t.Fatal("expected checkScope to pass with M2M AuthContext containing auth:user.read")
	}
	if controlPlaneHandler.checkScope(m2mValidRequest, "auth:user.write") {
		t.Fatal("expected checkScope to fail with M2M AuthContext missing auth:user.write")
	}
}
