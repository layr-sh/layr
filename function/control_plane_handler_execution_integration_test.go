// Package function defines the serverless and edge function execution engine.
package function

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionControlPlaneHandlerExecutionsIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	controlPlaneHandler := service.ControlPlaneHandler()

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(testCtx, core.CreateServiceAccountInput{
		Name: "executions-admin",
		Scopes: []string{
			core.ScopeFunctionEndpointRead,
		},
	})
	require.NoError(t, accountErr)
	authHeader := "Bearer " + serviceAccount.SecretKey

	// 1. Insert endpoint and executions in DB
	var endpoint Endpoint
	insertEndpointSQL := `
		INSERT INTO function.endpoints (name, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, created_at, updated_at)
		VALUES ('exec-integration-fn', 'workerd', 'index.js', 128, 30, true, clock_timestamp(), clock_timestamp())
		RETURNING id, name, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, created_at, updated_at;
	`
	require.NoError(t, kernel.DB().QueryRow(testCtx, insertEndpointSQL).Scan(
		&endpoint.ID,
		&endpoint.Name,
		&endpoint.Runtime,
		&endpoint.Entrypoint,
		&endpoint.MemoryLimitMB,
		&endpoint.TimeoutSeconds,
		&endpoint.IsPublic,
		&endpoint.CreatedAt,
		&endpoint.UpdatedAt,
	))

	insertExecutionSQL := `
		INSERT INTO function.executions (endpoint_id, method, path, status_code, duration_ms, stdout, stderr)
		VALUES ($1, 'POST', '/v1/function/exec-integration-fn', 201, 15, 'stdout message', 'stderr message');
	`
	_, insertExecErr := kernel.DB().Exec(testCtx, insertExecutionSQL, endpoint.ID)
	require.NoError(t, insertExecErr)

	t.Run("query execution logs via control plane", func(t *testing.T) {
		// List executions across all endpoints
		allRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/executions", nil)
		allRequest.Header.Set("Authorization", authHeader)
		allResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListExecutions(allResponseRecorder, allRequest)
		require.Equal(t, http.StatusOK, allResponseRecorder.Code)

		var allListExecutionsResponse ListExecutionsResponse
		require.NoError(t, json.Unmarshal(allResponseRecorder.Body.Bytes(), &allListExecutionsResponse))
		require.Equal(t, 1, allListExecutionsResponse.Count)
		require.Equal(t, "POST", allListExecutionsResponse.Executions[0].Method)
		require.Equal(t, "stdout message", allListExecutionsResponse.Executions[0].Stdout)

		// List executions by endpoint ID path
		byEndpointRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/executions", nil)
		byEndpointRequest.SetPathValue("id", endpoint.ID.String())
		byEndpointRequest.Header.Set("Authorization", authHeader)
		byEndpointResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListExecutions(byEndpointResponseRecorder, byEndpointRequest)
		require.Equal(t, http.StatusOK, byEndpointResponseRecorder.Code)

		var byEndpointListExecutionsResponse ListExecutionsResponse
		require.NoError(t, json.Unmarshal(byEndpointResponseRecorder.Body.Bytes(), &byEndpointListExecutionsResponse))
		require.Equal(t, 1, byEndpointListExecutionsResponse.Count)
		require.Equal(t, endpoint.ID, byEndpointListExecutionsResponse.Executions[0].EndpointID)
	})
}
