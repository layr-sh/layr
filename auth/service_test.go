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

	// 2. Test Service initialization with dependencies
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	if service == nil {
		t.Fatal("expected NewService with dependencies to return a non-nil Service instance")
	}

	// 3. Test Getters
	service.Stop()
	if service.Kernel() == nil {
		t.Fatal("expected Kernel to return non-nil Kernel")
	}
	if service.ConfigManager() == nil {
		t.Fatal("expected ConfigManager to return a non-nil ConfigManager")
	}
	if service.ControlPlaneHandler() == nil {
		t.Fatal("expected ControlPlaneHandler to return a non-nil ControlPlaneHandler")
	}
	if service.BaseHandler() == nil {
		t.Fatal("expected BaseHandler to return a non-nil Handler")
	}
	if service.Hasher() == nil {
		t.Fatal("expected Hasher to return a non-nil Hasher")
	}
	if service.PasskeyManager() == nil {
		t.Fatal("expected PasskeyManager to return a non-nil PasskeyManager")
	}
	if service.TOTPManager() == nil {
		t.Fatal("expected TOTPManager to return a non-nil TOTPManager")
	}
	if service.EmailDispatcher() == nil {
		t.Fatal("expected EmailDispatcher to return a non-nil EmailDispatcher")
	}
	if service.SMSDispatcher() == nil {
		t.Fatal("expected SMSDispatcher to return a non-nil SMSDispatcher")
	}
	if service.HTTPClient() == nil {
		t.Fatal("expected HTTPClient to return a non-nil HTTPClient")
	}

	// 4. Test CheckScope branches
	// Branch A0: AuthContext is a service account -> evaluates HasScope directly
	m2mAuthedCtx := core.WithAuthContext(ctx, core.AuthContext{
		ServiceAccountID: "sa_worker_123",
		JWT: core.JWTClaims{
			Subject: "sa_worker_123",
			Role:    "service_role",
			Scope:   core.ScopeAuthUserRead,
		},
	})
	m2mValidRequest := httptest.NewRequestWithContext(m2mAuthedCtx, http.MethodGet, "/v1/test", nil)
	if !service.kernel.ServiceAccountManager().CheckScope(m2mValidRequest, core.ScopeAuthUserRead) {
		t.Fatal("expected CheckScope to return true when M2M AuthContext has required scope")
	}
	if service.kernel.ServiceAccountManager().CheckScope(m2mValidRequest, core.ScopeAuthUserWrite) {
		t.Fatal("expected CheckScope to return false when M2M AuthContext lacks required scope")
	}

	// Branch B: serviceAccountManager present, but request has no service account key -> returns true
	noKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/test", nil)
	if !service.kernel.ServiceAccountManager().CheckScope(noKeyRequest, "auth:test.scope") {
		t.Fatal("expected CheckScope to return true when request has no service account key")
	}

	// Branch C: serviceAccountManager present, request has invalid secret key -> Authenticate fails -> returns false
	invalidKeyRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/test", nil)
	invalidKeyRequest.Header.Set("Authorization", "Bearer invalid_secret_key_value")
	if service.kernel.ServiceAccountManager().CheckScope(invalidKeyRequest, "auth:test.scope") {
		t.Fatal("expected CheckScope to return false when service account authentication fails")
	}

	// 5. Test Start with broken pool (returns error)
	if err := service.Start(ctx); err == nil {
		t.Fatal("expected Start with broken pool to fail")
	}

	// 8. Test Stop
	service.Stop()
}
