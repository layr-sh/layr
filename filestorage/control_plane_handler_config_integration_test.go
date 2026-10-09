package filestorage

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFilestorageControlPlaneHandlerConfigIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(ctx))
	defer service.Stop()

	coreServer := core.NewServer(kernel)
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "filestorage-config-admin",
		Scopes: []string{
			core.ScopeFileStorageConfigRead,
			core.ScopeFileStorageConfigWrite,
		},
	})
	require.NoError(t, accountErr)
	authSecretKey := serviceAccount.SecretKey

	t.Run("config control plane API lifecycle", func(t *testing.T) {
		// 1. GET /v1/_/file-storage/config
		getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/file-storage/config", nil)
		getRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		getResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(getResponseRecorder, getRequest)
		require.Equal(t, http.StatusOK, getResponseRecorder.Code)

		var currentConfig Config
		require.NoError(t, json.Unmarshal(getResponseRecorder.Body.Bytes(), &currentConfig))
		require.Equal(t, DefaultMaxFileSizeBytes, currentConfig.DefaultMaxFileSizeBytes)

		// 2. PUT /v1/_/file-storage/config
		updatePayload := `{"enabled":true,"default_max_file_size_bytes":104857600,"default_allowed_mime_types":["image/png"],"chunk_size_bytes":1048576,"presign_token_expiry_seconds":7200}`
		putRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/file-storage/config", bytes.NewReader([]byte(updatePayload)))
		putRequest.Header.Set("Content-Type", "application/json")
		putRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		putResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(putResponseRecorder, putRequest)
		require.Equal(t, http.StatusOK, putResponseRecorder.Code)

		var updatedConfig Config
		require.NoError(t, json.Unmarshal(putResponseRecorder.Body.Bytes(), &updatedConfig))
		require.Equal(t, int64(104857600), updatedConfig.DefaultMaxFileSizeBytes)
		require.Equal(t, 1048576, updatedConfig.ChunkSizeBytes)
		require.Equal(t, 7200, updatedConfig.PresignTokenExpirySeconds)
	})
}
