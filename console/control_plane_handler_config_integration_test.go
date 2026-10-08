package console

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

func TestConsoleControlPlaneHandlerConfigIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	consoleService := NewService(kernel)
	require.NoError(t, consoleService.Start(ctx))
	defer consoleService.Stop()

	coreServer := core.NewServer(kernel)
	consoleService.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "console-config-admin",
		Scopes: []string{
			core.ScopeConsoleConfigRead,
			core.ScopeConsoleConfigWrite,
		},
	})
	require.NoError(t, accountErr)
	authSecretKey := serviceAccount.SecretKey

	t.Run("config control plane API lifecycle", func(t *testing.T) {
		// 1. GET /v1/_/console/config
		getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/config", nil)
		getRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		getResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(getResponseRecorder, getRequest)
		require.Equal(t, http.StatusOK, getResponseRecorder.Code)

		var currentConfig Config
		require.NoError(t, json.Unmarshal(getResponseRecorder.Body.Bytes(), &currentConfig))
		require.Equal(t, DefaultSessionIdleTimeoutSeconds, currentConfig.SessionIdleTimeoutSeconds)

		// 2. PUT /v1/_/console/config
		updatePayload := `{"session_idle_timeout_seconds":1200,"session_max_lifetime_seconds":7200,"mfa_enforced":true,"allowed_cidrs":[],"audit_retention_days":60}`
		putRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/console/config", bytes.NewReader([]byte(updatePayload)))
		putRequest.Header.Set("Content-Type", "application/json")
		putRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		putResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(putResponseRecorder, putRequest)
		require.Equal(t, http.StatusOK, putResponseRecorder.Code)

		var updatedConfig Config
		require.NoError(t, json.Unmarshal(putResponseRecorder.Body.Bytes(), &updatedConfig))
		require.Equal(t, 1200, updatedConfig.SessionIdleTimeoutSeconds)
		require.True(t, updatedConfig.MFAEnforced)
	})
}
