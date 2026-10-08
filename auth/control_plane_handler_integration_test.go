package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestAuthControlPlaneHandlerBaseScopeIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(ctx))
	defer service.Stop()

	controlPlaneHandler := service.ControlPlaneHandler()
	serviceAccountManager := kernel.ServiceAccountManager()

	// 1. Create a service account with ScopeAuthConfigRead scope
	serviceAccount, accountErr := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Read Only Auth Service Account",
		Scopes: []string{core.ScopeAuthConfigRead, core.ScopeAuthUserRead},
	})
	require.NoError(t, accountErr)

	// 2. Test valid scope check
	validRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/config", nil)
	validRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	validResponseRecorder := httptest.NewRecorder()
	require.True(t, controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(validResponseRecorder, validRequest, core.ScopeAuthConfigRead))

	// 3. Test invalid scope check
	invalidScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/auth/config", nil)
	invalidScopeRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	invalidScopeResponseRecorder := httptest.NewRecorder()
	require.False(t, controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(invalidScopeResponseRecorder, invalidScopeRequest, core.ScopeAuthConfigWrite))

	// 4. Test invalid API key token
	malformedTokenRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/config", nil)
	malformedTokenRequest.Header.Set("Authorization", "Bearer invalid_api_key_token")
	malformedTokenResponseRecorder := httptest.NewRecorder()
	require.False(t, controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(malformedTokenResponseRecorder, malformedTokenRequest, core.ScopeAuthConfigRead))

	// 5. M2M AuthContext scope checks
	m2mAuthedCtx := core.WithAuthContext(ctx, core.AuthContext{
		ServiceAccountID: serviceAccount.ID,
		JWT: core.JWTClaims{
			Subject: serviceAccount.ID,
			Role:    "service_role",
			Scope:   core.ScopeAuthUserRead,
		},
	})
	m2mValidRequest := httptest.NewRequestWithContext(m2mAuthedCtx, http.MethodGet, "/v1/_/auth/users", nil)
	require.True(t, serviceAccountManager.CheckScope(m2mValidRequest, core.ScopeAuthUserRead))
	require.False(t, serviceAccountManager.CheckScope(m2mValidRequest, core.ScopeAuthUserWrite))
}
