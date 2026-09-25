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

func newMissingScopeContext(ctx context.Context) context.Context {
	authContext := core.AuthContext{
		ServiceAccountID: "unauthorized-account",
		JWT: core.JWTClaims{
			Subject:  "unauthorized-account",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "unrelated:scope",
		},
	}
	return core.WithAuthContext(ctx, authContext)
}

func TestFunctionControlPlaneHandlerUnit(t *testing.T) {
	t.Parallel()

	kernel := core.NewTestKernel(nil)
	functionService := NewService(kernel)
	controlPlaneHandler := functionService.ControlPlaneHandler()
	require.NotNil(t, controlPlaneHandler)

	t.Run("scope check failures across control plane handlers", func(t *testing.T) {
		t.Parallel()
		ctx := newMissingScopeContext(context.Background())

		// Config
		getConfigRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/function/config", nil)
		getConfigResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConfig(getConfigResponseRecorder, getConfigRequest)
		require.Equal(t, http.StatusForbidden, getConfigResponseRecorder.Code)

		updateConfigRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/function/config", nil)
		updateConfigResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConfig(updateConfigResponseRecorder, updateConfigRequest)
		require.Equal(t, http.StatusForbidden, updateConfigResponseRecorder.Code)

		// Endpoints
		listEndpointsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/function/endpoints", nil)
		listEndpointsResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListEndpoints(listEndpointsResponseRecorder, listEndpointsRequest)
		require.Equal(t, http.StatusForbidden, listEndpointsResponseRecorder.Code)

		createEndpointRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/function/endpoints", nil)
		createEndpointResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateEndpoint(createEndpointResponseRecorder, createEndpointRequest)
		require.Equal(t, http.StatusForbidden, createEndpointResponseRecorder.Code)

		getEndpointRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/function/endpoints/abc", nil)
		getEndpointRequest.SetPathValue("id", "abc")
		getEndpointResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetEndpoint(getEndpointResponseRecorder, getEndpointRequest)
		require.Equal(t, http.StatusForbidden, getEndpointResponseRecorder.Code)

		updateEndpointRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/function/endpoints/abc", nil)
		updateEndpointRequest.SetPathValue("id", "abc")
		updateEndpointResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateEndpoint(updateEndpointResponseRecorder, updateEndpointRequest)
		require.Equal(t, http.StatusForbidden, updateEndpointResponseRecorder.Code)

		deleteEndpointRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/function/endpoints/abc", nil)
		deleteEndpointRequest.SetPathValue("id", "abc")
		deleteEndpointResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteEndpoint(deleteEndpointResponseRecorder, deleteEndpointRequest)
		require.Equal(t, http.StatusForbidden, deleteEndpointResponseRecorder.Code)

		// Deployments
		deployRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/function/endpoints/abc/deploy", nil)
		deployRequest.SetPathValue("id", "abc")
		deployResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateDeployment(deployResponseRecorder, deployRequest)
		require.Equal(t, http.StatusForbidden, deployResponseRecorder.Code)

		listDeploymentsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/function/endpoints/abc/deployments", nil)
		listDeploymentsRequest.SetPathValue("id", "abc")
		listDeploymentsResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListDeployments(listDeploymentsResponseRecorder, listDeploymentsRequest)
		require.Equal(t, http.StatusForbidden, listDeploymentsResponseRecorder.Code)

		rollbackRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/function/endpoints/abc/rollback", nil)
		rollbackRequest.SetPathValue("id", "abc")
		rollbackResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRollbackDeployment(rollbackResponseRecorder, rollbackRequest)
		require.Equal(t, http.StatusForbidden, rollbackResponseRecorder.Code)

		// Stats
		getStatsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/function/stats", nil)
		getStatsResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetStats(getStatsResponseRecorder, getStatsRequest)
		require.Equal(t, http.StatusForbidden, getStatsResponseRecorder.Code)

		// Executions
		listExecutionsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/function/executions", nil)
		listExecutionsResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListExecutions(listExecutionsResponseRecorder, listExecutionsRequest)
		require.Equal(t, http.StatusForbidden, listExecutionsResponseRecorder.Code)

		// Domains
		createDomainRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/function/domains", nil)
		createDomainResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateCustomDomain(createDomainResponseRecorder, createDomainRequest)
		require.Equal(t, http.StatusForbidden, createDomainResponseRecorder.Code)

		listDomainsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/function/domains", nil)
		listDomainsResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListCustomDomains(listDomainsResponseRecorder, listDomainsRequest)
		require.Equal(t, http.StatusForbidden, listDomainsResponseRecorder.Code)

		deleteDomainRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/function/domains/abc", nil)
		deleteDomainRequest.SetPathValue("id", "abc")
		deleteDomainResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteCustomDomain(deleteDomainResponseRecorder, deleteDomainRequest)
		require.Equal(t, http.StatusForbidden, deleteDomainResponseRecorder.Code)

		// Domain routes
		createRouteRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/function/endpoints/abc/domains", nil)
		createRouteRequest.SetPathValue("id", "abc")
		createRouteResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateCustomDomainRoute(createRouteResponseRecorder, createRouteRequest)
		require.Equal(t, http.StatusForbidden, createRouteResponseRecorder.Code)

		listRoutesRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/function/endpoints/abc/domains", nil)
		listRoutesRequest.SetPathValue("id", "abc")
		listRoutesResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListCustomDomainRoutes(listRoutesResponseRecorder, listRoutesRequest)
		require.Equal(t, http.StatusForbidden, listRoutesResponseRecorder.Code)

		deleteRouteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/function/endpoints/abc/domains/def", nil)
		deleteRouteRequest.SetPathValue("id", "abc")
		deleteRouteRequest.SetPathValue("route_id", "def")
		deleteRouteResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteCustomDomainRoute(deleteRouteResponseRecorder, deleteRouteRequest)
		require.Equal(t, http.StatusForbidden, deleteRouteResponseRecorder.Code)
	})
}
