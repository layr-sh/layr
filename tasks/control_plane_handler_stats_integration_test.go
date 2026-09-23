// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksControlPlaneHandlerStatsIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(ctx))
	defer service.Stop()

	coreServer := core.NewServer(kernel)
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "tasks-stats-reader",
		Scopes: []string{
			core.ScopeTasksStatsRead,
		},
	})
	require.NoError(t, accountErr)
	authKey := serviceAccount.SecretKey

	t.Run("stats integration", func(t *testing.T) {
		statsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/tasks/stats", nil)
		statsRequest.Header.Set("X-Service-Account-Key", authKey)
		statsResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(statsResponseRecorder, statsRequest)
		require.Equal(t, http.StatusOK, statsResponseRecorder.Code)

		var statsResponse StatsResponse
		require.NoError(t, json.Unmarshal(statsResponseRecorder.Body.Bytes(), &statsResponse))
		require.GreaterOrEqual(t, statsResponse.TotalJobs, 0)
	})
}
