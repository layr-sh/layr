package image

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageControlPlaneHandlerUnit(t *testing.T) {
	t.Parallel()

	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	controlPlaneHandler := service.ControlPlaneHandler()
	require.NotNil(t, controlPlaneHandler)

	serviceAccountManager := controlPlaneHandler.kernel.ServiceAccountManager()

	t.Run("RequireScope with service account auth context", func(t *testing.T) {
		t.Parallel()
		matchingAuthContext := core.AuthContext{
			ServiceAccountID: "sa-1",
			JWT: core.JWTClaims{
				Subject:  "sa-1",
				Role:     "service_role",
				Audience: "test:service_account",
				Scope:    core.ScopeImagePresetRead,
			},
		}
		matchingCtx := core.WithAuthContext(context.Background(), matchingAuthContext)
		matchingRequest := httptest.NewRequestWithContext(matchingCtx, http.MethodGet, "/test", nil)
		matchingResponseRecorder := httptest.NewRecorder()
		require.True(t, serviceAccountManager.RequireScope(matchingResponseRecorder, matchingRequest, core.ScopeImagePresetRead))

		failingResponseRecorder := httptest.NewRecorder()
		require.False(t, serviceAccountManager.RequireScope(failingResponseRecorder, matchingRequest, core.ScopeImagePresetWrite))
	})

	t.Run("RequireScope without service account key header", func(t *testing.T) {
		t.Parallel()
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		responseRecorder := httptest.NewRecorder()
		require.True(t, serviceAccountManager.RequireScope(responseRecorder, request, core.ScopeImagePresetRead))
	})

	t.Run("RequireScope with invalid service account key", func(t *testing.T) {
		t.Parallel()
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		request.Header.Set("Authorization", "Bearer invalid-sa-key")
		responseRecorder := httptest.NewRecorder()
		require.False(t, serviceAccountManager.RequireScope(responseRecorder, request, core.ScopeImagePresetRead))
	})
}
