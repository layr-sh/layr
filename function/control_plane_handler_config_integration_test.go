// Package function defines the serverless and edge function execution engine.
package function

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

func TestFunctionControlPlaneHandlerConfigIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	coreServer := core.NewServer(kernel)
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(testCtx, core.CreateServiceAccountInput{
		Name: "function-config-admin",
		Scopes: []string{
			core.ScopeFunctionConfigRead,
			core.ScopeFunctionConfigWrite,
		},
	})
	require.NoError(t, accountErr)
	authHeader := "Bearer " + serviceAccount.SecretKey

	t.Run("config control plane API lifecycle", func(t *testing.T) {
		// 1. GET /v1/_/function/config
		getRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/config", nil)
		getRequest.Header.Set("Authorization", authHeader)
		getResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(getResponseRecorder, getRequest)
		require.Equal(t, http.StatusOK, getResponseRecorder.Code)

		var currentConfig Config
		require.NoError(t, json.Unmarshal(getResponseRecorder.Body.Bytes(), &currentConfig))
		require.Equal(t, 128, currentConfig.DefaultMemoryLimitMB)

		// 2. PUT /v1/_/function/config
		currentConfig.DefaultMemoryLimitMB = 512
		updatePayload, marshalErr := json.Marshal(currentConfig)
		require.NoError(t, marshalErr)

		putRequest := httptest.NewRequestWithContext(testCtx, http.MethodPut, "/v1/_/function/config", bytes.NewReader(updatePayload))
		putRequest.Header.Set("Content-Type", "application/json")
		putRequest.Header.Set("Authorization", authHeader)
		putResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(putResponseRecorder, putRequest)
		require.Equal(t, http.StatusOK, putResponseRecorder.Code)

		var updatedConfig Config
		require.NoError(t, json.Unmarshal(putResponseRecorder.Body.Bytes(), &updatedConfig))
		require.Equal(t, 512, updatedConfig.DefaultMemoryLimitMB)
	})
}
