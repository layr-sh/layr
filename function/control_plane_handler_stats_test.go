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

func TestFunctionControlPlaneHandlerStatsUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	testRunner := &mockFailingRunner{}
	service.Engine().RegisterRunner(testRunner)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	controlPlaneHandler := service.ControlPlaneHandler()

	rootCtx := core.WithAuthContext(testCtx, core.AuthContext{
		ServiceAccountID: "sa-root",
		JWT: core.JWTClaims{
			Subject:  "sa-root",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFunctionStatsRead,
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

	t.Run("Scope permissions and denials", func(t *testing.T) {
		noScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/function/stats", nil)
		noScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetStats(noScopeResponseRecorder, noScopeRequest)
		require.Equal(t, http.StatusForbidden, noScopeResponseRecorder.Code)
	})

	t.Run("Healthy and failing runner telemetry", func(t *testing.T) {
		// Healthy
		validRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/stats", nil)
		validResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetStats(validResponseRecorder, validRequest)
		require.Equal(t, http.StatusOK, validResponseRecorder.Code)

		var getStatsResponse GetStatsResponse
		require.NoError(t, json.Unmarshal(validResponseRecorder.Body.Bytes(), &getStatsResponse))
		require.NotNil(t, getStatsResponse.Runtimes["workerd"])
		require.True(t, getStatsResponse.Runtimes["workerd"].Available)

		// Runner health failure still returns 200 with unhealthy telemetry
		testRunner.healthFail = true
		failingStatsRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/stats", nil)
		failingStatsResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetStats(failingStatsResponseRecorder, failingStatsRequest)
		require.Equal(t, http.StatusOK, failingStatsResponseRecorder.Code)
		testRunner.healthFail = false
	})
}

func TestFunctionControlPlaneHandlerStatsDatabaseFailureUnit(t *testing.T) {
	brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	brokenService := NewService(brokenKernel)
	controlPlaneHandler := brokenService.ControlPlaneHandler()

	rootCtx := core.WithAuthContext(context.Background(), core.AuthContext{
		ServiceAccountID: "sa-root",
		JWT: core.JWTClaims{
			Subject:  "sa-root",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFunctionStatsRead,
		},
	})

	statsRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/stats", nil)
	statsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetStats(statsResponseRecorder, statsRequest)
	require.Equal(t, http.StatusInternalServerError, statsResponseRecorder.Code)
}
