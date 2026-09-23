package image

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageControlPlaneHandlerBaseScopeIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(ctx))
	defer service.Stop()

	controlPlaneHandler := service.ControlPlaneHandler()
	serviceAccountManager := kernel.ServiceAccountManager()

	// 1. Create a service account with ScopeImageConfigRead scope
	serviceAccount, accountErr := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Read Only Image Service Account",
		Scopes: []string{core.ScopeImageConfigRead},
	})
	require.NoError(t, accountErr)

	// 2. Test valid scope check
	validRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/image/config", nil)
	validRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	validResponseRecorder := httptest.NewRecorder()
	require.True(t, controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(validResponseRecorder, validRequest, core.ScopeImageConfigRead))

	// 3. Test invalid scope check
	invalidScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/image/config", nil)
	invalidScopeRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	invalidScopeResponseRecorder := httptest.NewRecorder()
	require.False(t, controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(invalidScopeResponseRecorder, invalidScopeRequest, core.ScopeImageConfigWrite))

	// 4. Test invalid API key token
	malformedTokenRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/image/config", nil)
	malformedTokenRequest.Header.Set("Authorization", "Bearer invalid_api_key_token")
	malformedTokenResponseRecorder := httptest.NewRecorder()
	require.False(t, controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(malformedTokenResponseRecorder, malformedTokenRequest, core.ScopeImageConfigRead))
}
