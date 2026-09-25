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

func TestFunctionControlPlaneHandlerDomainsIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	controlPlaneHandler := service.ControlPlaneHandler()

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(testCtx, core.CreateServiceAccountInput{
		Name: "domains-admin",
		Scopes: []string{
			core.ScopeFunctionEndpointRead,
			core.ScopeFunctionEndpointWrite,
		},
	})
	require.NoError(t, accountErr)
	authHeader := "Bearer " + serviceAccount.SecretKey

	// Create endpoint
	createEndpointInput := CreateEndpointInput{
		Name: "domains-integration-fn",
	}
	createEpJSON, _ := json.Marshal(createEndpointInput)
	createEpRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints", bytes.NewReader(createEpJSON))
	createEpRequest.Header.Set("Authorization", authHeader)
	createEpResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateEndpoint(createEpResponseRecorder, createEpRequest)
	require.Equal(t, http.StatusCreated, createEpResponseRecorder.Code)

	var targetEndpoint Endpoint
	require.NoError(t, json.Unmarshal(createEpResponseRecorder.Body.Bytes(), &targetEndpoint))

	t.Run("custom domains and routes lifecycle", func(t *testing.T) {
		// 1. Create domain
		createCustomDomainInput := CreateCustomDomainInput{
			Domain: "integration.myworker.com",
		}
		createDomainJSON, _ := json.Marshal(createCustomDomainInput)
		createDomainRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/domains", bytes.NewReader(createDomainJSON))
		createDomainRequest.Header.Set("Authorization", authHeader)
		createDomainResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateCustomDomain(createDomainResponseRecorder, createDomainRequest)
		require.Equal(t, http.StatusCreated, createDomainResponseRecorder.Code)

		var customDomain CustomDomain
		require.NoError(t, json.Unmarshal(createDomainResponseRecorder.Body.Bytes(), &customDomain))
		require.Equal(t, "integration.myworker.com", customDomain.Domain)

		// 2. List domains
		listDomainsRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/domains", nil)
		listDomainsRequest.Header.Set("Authorization", authHeader)
		listDomainsResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListCustomDomains(listDomainsResponseRecorder, listDomainsRequest)
		require.Equal(t, http.StatusOK, listDomainsResponseRecorder.Code)

		var listCustomDomainsResponse ListCustomDomainsResponse
		require.NoError(t, json.Unmarshal(listDomainsResponseRecorder.Body.Bytes(), &listCustomDomainsResponse))
		require.Equal(t, 1, listCustomDomainsResponse.Total)

		// 3. Create route on endpoint
		defaultPathPrefix := "/api"
		createCustomDomainRouteInput := CreateCustomDomainRouteInput{
			CustomDomainID: customDomain.ID,
			PathPrefix:     &defaultPathPrefix,
		}
		createRouteJSON, _ := json.Marshal(createCustomDomainRouteInput)
		createRouteRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", bytes.NewReader(createRouteJSON))
		createRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
		createRouteRequest.Header.Set("Authorization", authHeader)
		createRouteResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateCustomDomainRoute(createRouteResponseRecorder, createRouteRequest)
		require.Equal(t, http.StatusCreated, createRouteResponseRecorder.Code)

		var customDomainRoute CustomDomainRoute
		require.NoError(t, json.Unmarshal(createRouteResponseRecorder.Body.Bytes(), &customDomainRoute))
		require.Equal(t, "/api", customDomainRoute.PathPrefix)

		// 4. List routes on endpoint
		listRoutesRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains", nil)
		listRoutesRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
		listRoutesRequest.Header.Set("Authorization", authHeader)
		listRoutesResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListCustomDomainRoutes(listRoutesResponseRecorder, listRoutesRequest)
		require.Equal(t, http.StatusOK, listRoutesResponseRecorder.Code)

		var listCustomDomainRoutesResponse ListCustomDomainRoutesResponse
		require.NoError(t, json.Unmarshal(listRoutesResponseRecorder.Body.Bytes(), &listCustomDomainRoutesResponse))
		require.Equal(t, 1, listCustomDomainRoutesResponse.Total)

		// 5. Delete route
		deleteRouteRequest := httptest.NewRequestWithContext(testCtx, http.MethodDelete, "/v1/_/function/endpoints/"+targetEndpoint.ID.String()+"/domains/"+customDomainRoute.ID.String(), nil)
		deleteRouteRequest.SetPathValue("endpoint_id", targetEndpoint.ID.String())
		deleteRouteRequest.SetPathValue("route_id", customDomainRoute.ID.String())
		deleteRouteRequest.Header.Set("Authorization", authHeader)
		deleteRouteResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteCustomDomainRoute(deleteRouteResponseRecorder, deleteRouteRequest)
		require.Equal(t, http.StatusNoContent, deleteRouteResponseRecorder.Code)

		// 6. Delete domain
		deleteDomainRequest := httptest.NewRequestWithContext(testCtx, http.MethodDelete, "/v1/_/function/domains/"+customDomain.ID.String(), nil)
		deleteDomainRequest.SetPathValue("domain_id", customDomain.ID.String())
		deleteDomainRequest.Header.Set("Authorization", authHeader)
		deleteDomainResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteCustomDomain(deleteDomainResponseRecorder, deleteDomainRequest)
		require.Equal(t, http.StatusNoContent, deleteDomainResponseRecorder.Code)
	})
}
