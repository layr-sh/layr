package filestorage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFilestorageControlPlaneHandlerUnit(t *testing.T) {
	t.Parallel()

	cryptoKeyManager, keyErr := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	require.NoError(t, keyErr)

	configManager := NewConfigManager(nil)
	controlPlaneHandler := NewControlPlaneHandler(nil, configManager, cryptoKeyManager)

	t.Run("checkScope with service account auth context", func(t *testing.T) {
		t.Parallel()
		matchingAuthContext := core.AuthContext{
			ServiceAccountID: "sa-1",
			JWT: core.JWTClaims{
				Subject:  "sa-1",
				Role:     "service_role",
				Audience: "test:service_account",
				Scope:    core.ScopeFileStorageBucketRead,
			},
		}
		matchingCtx := core.WithAuthContext(context.Background(), matchingAuthContext)
		matchingRequest := httptest.NewRequestWithContext(matchingCtx, http.MethodGet, "/test", nil)
		require.True(t, controlPlaneHandler.checkScope(matchingRequest, core.ScopeFileStorageBucketRead))
		require.False(t, controlPlaneHandler.checkScope(matchingRequest, core.ScopeFileStorageBucketWrite))
	})

	t.Run("checkScope with nil service account manager", func(t *testing.T) {
		t.Parallel()
		nilManagerControlPlaneHandler := NewControlPlaneHandler(nil, configManager, cryptoKeyManager)
		nilManagerControlPlaneHandler.SetServiceAccountManager(nil)

		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		require.True(t, nilManagerControlPlaneHandler.checkScope(request, core.ScopeFileStorageBucketRead))
	})

	t.Run("checkScope without service account key header", func(t *testing.T) {
		t.Parallel()
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		require.True(t, controlPlaneHandler.checkScope(request, core.ScopeFileStorageBucketRead))
	})

	t.Run("checkScope with invalid service account key", func(t *testing.T) {
		t.Parallel()
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		request.Header.Set("Authorization", "Bearer invalid-sa-key")
		require.False(t, controlPlaneHandler.checkScope(request, core.ScopeFileStorageBucketRead))
	})

	t.Run("writeJSON writes valid json", func(t *testing.T) {
		t.Parallel()
		responseRecorder := httptest.NewRecorder()
		controlPlaneHandler.writeJSON(responseRecorder, http.StatusOK, map[string]string{"status": "ok"})
		require.Equal(t, http.StatusOK, responseRecorder.Code)
		require.Contains(t, responseRecorder.Body.String(), `"status":"ok"`)
	})
}
