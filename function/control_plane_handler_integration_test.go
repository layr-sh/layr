// Package function defines the serverless and edge function execution engine.
package function

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionControlPlaneHandlerBaseScopeIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	controlPlaneHandler := service.ControlPlaneHandler()
	serviceAccountManager := kernel.ServiceAccountManager()

	// 1. Create a service account with ScopeFunctionConfigRead scope
	serviceAccount, accountErr := serviceAccountManager.Create(testCtx, core.CreateServiceAccountInput{
		Name:   "Read Only Function Service Account",
		Scopes: []string{core.ScopeFunctionConfigRead},
	})
	require.NoError(t, accountErr)

	// 2. Test valid scope check
	validRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/config", nil)
	validRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	validResponseRecorder := httptest.NewRecorder()
	require.True(t, controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(validResponseRecorder, validRequest, core.ScopeFunctionConfigRead))

	// 3. Test invalid scope check
	invalidScopeRequest := httptest.NewRequestWithContext(testCtx, http.MethodPut, "/v1/_/function/config", nil)
	invalidScopeRequest.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	invalidScopeResponseRecorder := httptest.NewRecorder()
	require.False(t, controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(invalidScopeResponseRecorder, invalidScopeRequest, core.ScopeFunctionConfigWrite))

	// 4. Test invalid API key token
	malformedTokenRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/config", nil)
	malformedTokenRequest.Header.Set("Authorization", "Bearer invalid_api_key_token")
	malformedTokenResponseRecorder := httptest.NewRecorder()
	require.False(t, controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(malformedTokenResponseRecorder, malformedTokenRequest, core.ScopeFunctionConfigRead))
}
