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

func TestFunctionControlPlaneHandlerStatsIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	controlPlaneHandler := service.ControlPlaneHandler()

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(testCtx, core.CreateServiceAccountInput{
		Name: "stats-admin",
		Scopes: []string{
			core.ScopeFunctionStatsRead,
		},
	})
	require.NoError(t, accountErr)
	authHeader := "Bearer " + serviceAccount.SecretKey

	statsRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/stats", nil)
	statsRequest.Header.Set("Authorization", authHeader)
	statsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetStats(statsResponseRecorder, statsRequest)
	require.Equal(t, http.StatusOK, statsResponseRecorder.Code)

	var getStatsResponse GetStatsResponse
	require.NoError(t, json.Unmarshal(statsResponseRecorder.Body.Bytes(), &getStatsResponse))
	require.NotNil(t, getStatsResponse.Runtimes)
}
