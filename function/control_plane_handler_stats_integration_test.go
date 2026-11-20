// Package function defines the serverless and edge function execution engine.
package function

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	require.Zero(t, getStatsResponse.TotalExecutions)
	require.Zero(t, getStatsResponse.ExecutionsLast24Hours)
	require.Nil(t, getStatsResponse.AverageDurationMs)
	require.Nil(t, getStatsResponse.P95DurationMs)
	require.Nil(t, getStatsResponse.LastExecutedAt)
	require.Empty(t, getStatsResponse.Endpoints)

	var endpointID, idleEndpointID, deploymentID string
	require.NoError(t, kernel.DB().QueryRow(testCtx, `INSERT INTO function.endpoints (name, is_public) VALUES ('stats-live', true) RETURNING id`).Scan(&endpointID))
	require.NoError(t, kernel.DB().QueryRow(testCtx, `INSERT INTO function.endpoints (name, is_public) VALUES ('stats-idle', false) RETURNING id`).Scan(&idleEndpointID))
	require.NoError(t, kernel.DB().QueryRow(testCtx, `INSERT INTO function.deployments (endpoint_id, version, bundle_content, bundle_hash) VALUES ($1, 1, 'abc'::bytea, 'one') RETURNING id`, endpointID).Scan(&deploymentID))
	_, err := kernel.DB().Exec(testCtx, `INSERT INTO function.deployments (endpoint_id, version, bundle_content, bundle_hash) VALUES ($1, 2, 'abcd'::bytea, 'two')`, endpointID)
	require.NoError(t, err)
	_, err = kernel.DB().Exec(testCtx, `UPDATE function.endpoints SET active_deployment_id = $1 WHERE id = $2`, deploymentID, endpointID)
	require.NoError(t, err)
	_, err = kernel.DB().Exec(testCtx, `
		INSERT INTO function.executions (endpoint_id, method, path, status_code, duration_ms, error_message, created_at)
		SELECT $1, 'GET', '/', status_code, duration_ms, error_message, CURRENT_TIMESTAMP + age
		FROM (VALUES
			(200, 10, NULL::text, INTERVAL '-1 hour'),
			(302, 30, '', INTERVAL '-2 hours'),
			(404, 50, NULL, INTERVAL '-3 hours'),
			(500, 100, 'server error', INTERVAL '-4 hours'),
			(200, 200, 'runtime error', INTERVAL '-5 hours'),
			(500, 9999, 'old error', INTERVAL '-25 hours'),
			(500, 900, 'future error', INTERVAL '1 hour')
		) AS sample(status_code, duration_ms, error_message, age)`, endpointID)
	require.NoError(t, err)
	_, err = kernel.DB().Exec(testCtx, `
		INSERT INTO function.custom_domains (domain, tls_enabled, tls_status, tls_expires_at) VALUES
		('healthy.example.com', true, 'active', CURRENT_TIMESTAMP + INTERVAL '30 days'),
		('failed.example.com', true, 'failed', NULL),
		('expired.example.com', true, 'active', CURRENT_TIMESTAMP - INTERVAL '1 day'),
		('plain.example.com', false, 'disabled', NULL)`)
	require.NoError(t, err)
	_, err = kernel.DB().Exec(testCtx, `INSERT INTO function.custom_domain_routes (custom_domain_id, endpoint_id, path_prefix) SELECT id, $1, '/' FROM function.custom_domains WHERE domain IN ('healthy.example.com', 'plain.example.com')`, endpointID)
	require.NoError(t, err)

	statsResponseRecorder = httptest.NewRecorder()
	controlPlaneHandler.handleGetStats(statsResponseRecorder, statsRequest)
	require.Equal(t, http.StatusOK, statsResponseRecorder.Code, statsResponseRecorder.Body.String())
	require.NoError(t, json.Unmarshal(statsResponseRecorder.Body.Bytes(), &getStatsResponse))
	require.Equal(t, 2, getStatsResponse.TotalEndpoints)
	require.Equal(t, 1, getStatsResponse.PublicEndpoints)
	require.Equal(t, 1, getStatsResponse.PrivateEndpoints)
	require.Equal(t, 1, getStatsResponse.DeployedEndpoints)
	require.Equal(t, int64(2), getStatsResponse.TotalDeployments)
	require.Equal(t, int64(7), getStatsResponse.BundleSizeBytes)
	require.Equal(t, int64(4), getStatsResponse.TotalDomains)
	require.Equal(t, int64(2), getStatsResponse.DomainsNeedingTLS)
	require.Equal(t, int64(2), getStatsResponse.DomainRoutes)
	require.Equal(t, int64(7), getStatsResponse.TotalExecutions)
	require.Equal(t, int64(5), getStatsResponse.ExecutionsLast24Hours)
	require.Equal(t, int64(3), getStatsResponse.FailedExecutionsLast24Hours)
	require.Equal(t, int64(1), getStatsResponse.ClientErrorsLast24Hours)
	require.Equal(t, int64(1), getStatsResponse.ServerErrorsLast24Hours)
	require.InDelta(t, 78, *getStatsResponse.AverageDurationMs, 0.001)
	require.InDelta(t, 180, *getStatsResponse.P95DurationMs, 0.001)
	require.Equal(t, 24*time.Hour, getStatsResponse.GeneratedAt.Sub(getStatsResponse.WindowStartedAt))
	require.NotNil(t, getStatsResponse.LastExecutedAt)
	require.Len(t, getStatsResponse.Endpoints, 2)
	require.Equal(t, "stats-live", getStatsResponse.Endpoints[0].Name)
	require.Equal(t, int64(5), getStatsResponse.Endpoints[0].Executions)
	require.Equal(t, int64(3), getStatsResponse.Endpoints[0].FailedExecutions)
	require.InDelta(t, 78, *getStatsResponse.Endpoints[0].AverageDurationMs, 0.001)
	require.InDelta(t, 180, *getStatsResponse.Endpoints[0].P95DurationMs, 0.001)
	require.True(t, getStatsResponse.Endpoints[0].IsDeployed)
	require.True(t, getStatsResponse.Endpoints[0].IsPublic)
	require.Equal(t, "stats-idle", getStatsResponse.Endpoints[1].Name)
	require.Nil(t, getStatsResponse.Endpoints[1].AverageDurationMs)
	require.Nil(t, getStatsResponse.Endpoints[1].P95DurationMs)
	require.Nil(t, getStatsResponse.Endpoints[1].LastExecutedAt)
	require.False(t, getStatsResponse.Endpoints[1].IsDeployed)
}
