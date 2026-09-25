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

func TestFunctionControlPlaneHandlerExecutionsUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	controlPlaneHandler := service.ControlPlaneHandler()

	rootCtx := core.WithAuthContext(testCtx, core.AuthContext{
		ServiceAccountID: "sa-root",
		JWT: core.JWTClaims{
			Subject:  "sa-root",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "*",
		},
	})

	noScopeCtx := core.WithAuthContext(testCtx, core.AuthContext{
		ServiceAccountID: "sa-none",
		JWT: core.JWTClaims{
			Subject:  "sa-none",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "unrelated:scope",
		},
	})

	t.Run("Scope permissions and validations", func(t *testing.T) {
		// Missing scope -> 403
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/function/executions", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListExecutions(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)

		// Invalid path ID -> 400
		invalidPathRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/endpoints/invalid-uuid/executions", nil)
		invalidPathRequest.SetPathValue("endpoint_id", "invalid-uuid")
		invalidPathResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListExecutions(invalidPathResponseRecorder, invalidPathRequest)
		require.Equal(t, http.StatusBadRequest, invalidPathResponseRecorder.Code)

		// Invalid query endpoint_id -> 400
		invalidQueryRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/executions?endpoint_id=not-a-uuid", nil)
		invalidQueryResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListExecutions(invalidQueryResponseRecorder, invalidQueryRequest)
		require.Equal(t, http.StatusBadRequest, invalidQueryResponseRecorder.Code)
	})

	t.Run("Queries and filters", func(t *testing.T) {
		var endpoint Endpoint
		createErr := kernel.DB().QueryRow(testCtx, `
			INSERT INTO function.endpoints (name, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, created_at, updated_at)
			VALUES ('worker-exec-unit', 'workerd', 'index.js', 128, 30, true, clock_timestamp(), clock_timestamp())
			RETURNING id, name, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public, created_at, updated_at;
		`).Scan(&endpoint.ID, &endpoint.Name, &endpoint.Runtime, &endpoint.Entrypoint, &endpoint.MemoryLimitMB, &endpoint.TimeoutSeconds, &endpoint.IsPublic, &endpoint.CreatedAt, &endpoint.UpdatedAt)
		require.NoError(t, createErr)

		_, recordErr := kernel.DB().Exec(testCtx, `
			INSERT INTO function.executions (endpoint_id, method, path, status_code, duration_ms, stdout, stderr)
			VALUES ($1, 'GET', '/v1/function/worker-exec-unit', 200, 12, 'captured log message', '');
		`, endpoint.ID)
		require.NoError(t, recordErr)

		// Query all executions with pagination
		allRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/executions?limit=10&offset=0", nil)
		allResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListExecutions(allResponseRecorder, allRequest)
		require.Equal(t, http.StatusOK, allResponseRecorder.Code)

		// Query executions for specific endpoint via query parameter
		byQueryRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/executions?endpoint_id="+endpoint.ID.String(), nil)
		byQueryResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListExecutions(byQueryResponseRecorder, byQueryRequest)
		require.Equal(t, http.StatusOK, byQueryResponseRecorder.Code)

		// Query executions for specific endpoint via path parameter
		byPathRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/endpoints/"+endpoint.ID.String()+"/executions", nil)
		byPathRequest.SetPathValue("endpoint_id", endpoint.ID.String())
		byPathResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListExecutions(byPathResponseRecorder, byPathRequest)
		require.Equal(t, http.StatusOK, byPathResponseRecorder.Code)
	})
}

func TestFunctionControlPlaneHandlerExecutionsDatabaseFailureUnit(t *testing.T) {
	brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	brokenService := NewService(brokenKernel)
	controlPlaneHandler := brokenService.ControlPlaneHandler()

	rootCtx := core.WithAuthContext(context.Background(), core.AuthContext{
		ServiceAccountID: "sa-root",
		JWT: core.JWTClaims{
			Subject:  "sa-root",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "*",
		},
	})

	executionsRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/executions", nil)
	executionsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListExecutions(executionsResponseRecorder, executionsRequest)
	require.Equal(t, http.StatusInternalServerError, executionsResponseRecorder.Code)
}
