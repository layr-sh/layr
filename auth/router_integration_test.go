package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-fuego/fuego"
	"layr.sh/core"
)

func TestAuthRouterIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	service := NewService(db, cryptoKeyManager)
	if err := service.Start(ctx); err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	fuegoEngine := fuego.NewServer()
	publicRouter := core.NewRouter(fuegoEngine)

	controlPlaneFuegoEngine := fuego.NewServer()
	controlPlaneRouter := core.NewRouter(controlPlaneFuegoEngine)

	service.RegisterRoutes(publicRouter, controlPlaneRouter)

	// 1. Verify Public OIDC Discovery endpoint
	discoveryRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/.well-known/openid-configuration", nil)
	discoveryResponseRecorder := httptest.NewRecorder()
	publicRouter.Mux().ServeHTTP(discoveryResponseRecorder, discoveryRequest)
	if discoveryResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from openid-configuration, got: %d", discoveryResponseRecorder.Code)
	}
	if !strings.Contains(discoveryResponseRecorder.Body.String(), "issuer") {
		t.Fatalf("expected openid-configuration body to contain 'issuer', got: %s", discoveryResponseRecorder.Body.String())
	}

	// 2. Verify Public JWKS endpoint
	jwksRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/.well-known/jwks.json", nil)
	jwksResponseRecorder := httptest.NewRecorder()
	publicRouter.Mux().ServeHTTP(jwksResponseRecorder, jwksRequest)
	if jwksResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from jwks.json, got: %d", jwksResponseRecorder.Code)
	}
	if !strings.Contains(jwksResponseRecorder.Body.String(), "keys") {
		t.Fatalf("expected jwks.json body to contain 'keys', got: %s", jwksResponseRecorder.Body.String())
	}

	// 3. Verify Control Plane Config endpoint
	configRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/config", nil)
	configResponseRecorder := httptest.NewRecorder()
	controlPlaneRouter.Mux().ServeHTTP(configResponseRecorder, configRequest)
	if configResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from control plane config endpoint, got: %d", configResponseRecorder.Code)
	}

	// 4. Verify Control Plane Users endpoint
	usersRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users", nil)
	usersResponseRecorder := httptest.NewRecorder()
	controlPlaneRouter.Mux().ServeHTTP(usersResponseRecorder, usersRequest)
	if usersResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from control plane users endpoint, got: %d", usersResponseRecorder.Code)
	}

	// 5. Verify session user-prefixed alias route on public router
	sessionAliasRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/auth/user/sessions", nil)
	sessionAliasResponseRecorder := httptest.NewRecorder()
	publicRouter.Mux().ServeHTTP(sessionAliasResponseRecorder, sessionAliasRequest)
	if sessionAliasResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized on unauthenticated session alias, got: %d", sessionAliasResponseRecorder.Code)
	}
}
