package tasks

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

func TestTasksControlPlaneHandlerConfigIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	tasksService := NewService(kernel)
	require.NoError(t, tasksService.Start(ctx))
	defer tasksService.Stop()

	coreServer := core.NewServer(kernel)
	tasksService.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "tasks-config-admin",
		Scopes: []string{
			core.ScopeTasksConfigRead,
			core.ScopeTasksConfigWrite,
		},
	})
	require.NoError(t, accountErr)
	authSecretKey := serviceAccount.SecretKey

	t.Run("config control plane API lifecycle", func(t *testing.T) {
		// 1. GET /v1/_/tasks/config
		getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/config", nil)
		getRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		getResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(getResponseRecorder, getRequest)
		require.Equal(t, http.StatusOK, getResponseRecorder.Code)

		var currentConfig Config
		require.NoError(t, json.Unmarshal(getResponseRecorder.Body.Bytes(), &currentConfig))
		require.Equal(t, defaultConcurrencyLimit, currentConfig.ConcurrencyLimit)

		// 2. PUT /v1/_/tasks/config
		updatePayload := `{"concurrency_limit":40,"timeout_seconds":45,"poll_interval_ms":500,"retry_initial_delay_seconds":2,"retry_max_delay_seconds":120,"retry_max_attempts":8}`
		putRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/tasks/config", bytes.NewReader([]byte(updatePayload)))
		putRequest.Header.Set("Content-Type", "application/json")
		putRequest.Header.Set("X-Service-Account-Key", authSecretKey)
		putResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(putResponseRecorder, putRequest)
		require.Equal(t, http.StatusOK, putResponseRecorder.Code)

		var updatedConfig Config
		require.NoError(t, json.Unmarshal(putResponseRecorder.Body.Bytes(), &updatedConfig))
		require.Equal(t, 40, updatedConfig.ConcurrencyLimit)
		require.Equal(t, 45, updatedConfig.TimeoutSeconds)
	})
}
