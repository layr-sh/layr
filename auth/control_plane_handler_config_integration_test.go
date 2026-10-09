package auth

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

func TestAuthControlPlaneHandlerConfigIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	authService := NewService(kernel)
	require.NoError(t, authService.Start(ctx))
	defer authService.Stop()

	coreServer := core.NewServer(kernel)
	authService.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "auth-config-admin",
		Scopes: []string{
			core.ScopeAuthConfigRead,
			core.ScopeAuthConfigWrite,
		},
	})
	require.NoError(t, accountErr)
	authSecretKey := serviceAccount.SecretKey

	t.Run("config control plane API lifecycle", func(t *testing.T) {
		// 1. GET /v1/_/auth/config
		getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/config", nil)
		getRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		getResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(getResponseRecorder, getRequest)
		require.Equal(t, http.StatusOK, getResponseRecorder.Code)

		var currentConfig Config
		require.NoError(t, json.Unmarshal(getResponseRecorder.Body.Bytes(), &currentConfig))
		require.True(t, currentConfig.Password.Enabled)

		// 2. PUT /v1/_/auth/config
		currentConfig.Sessions.AccessTokenExpirySeconds = 7200
		putBody, marshalErr := json.Marshal(currentConfig)
		require.NoError(t, marshalErr)

		putRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/auth/config", bytes.NewReader(putBody))
		putRequest.Header.Set("Content-Type", "application/json")
		putRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		putResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(putResponseRecorder, putRequest)
		require.Equal(t, http.StatusOK, putResponseRecorder.Code)

		var updatedConfig Config
		require.NoError(t, json.Unmarshal(putResponseRecorder.Body.Bytes(), &updatedConfig))
		require.Equal(t, 7200, updatedConfig.Sessions.AccessTokenExpirySeconds)
	})
}
