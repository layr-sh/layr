package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/core"
)

func TestAuthControlPlaneHandlerIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	serviceAccountManager := kernel.ServiceAccountManager()

	createdServiceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Auth Base Control Plane Service Account",
		Scopes: []string{core.ScopeAuthUserRead},
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}

	// Valid scope check
	validRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users", nil)
	validRequest.Header.Set("Authorization", "Bearer "+createdServiceAccount.SecretKey)
	if !serviceAccountManager.CheckScope(validRequest, core.ScopeAuthUserRead) {
		t.Fatal("expected CheckScope to pass with valid service account and scope")
	}

	// Missing scope check
	if serviceAccountManager.CheckScope(validRequest, core.ScopeAuthUserWrite) {
		t.Fatal("expected CheckScope to fail when required scope is not granted")
	}

	// M2M AuthContext scope checks
	m2mAuthedCtx := core.WithAuthContext(ctx, core.AuthContext{
		ServiceAccountID: createdServiceAccount.ID,
		JWT: core.JWTClaims{
			Subject: createdServiceAccount.ID,
			Role:    "service_role",
			Scope:   core.ScopeAuthUserRead,
		},
	})
	m2mValidRequest := httptest.NewRequestWithContext(m2mAuthedCtx, http.MethodGet, "/v1/_/auth/users", nil)
	if !serviceAccountManager.CheckScope(m2mValidRequest, core.ScopeAuthUserRead) {
		t.Fatal("expected CheckScope to pass with M2M AuthContext containing auth:user.read")
	}
	if serviceAccountManager.CheckScope(m2mValidRequest, core.ScopeAuthUserWrite) {
		t.Fatal("expected CheckScope to fail with M2M AuthContext missing auth:user.write")
	}
}
