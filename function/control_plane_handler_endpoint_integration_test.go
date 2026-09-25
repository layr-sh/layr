// Package function defines the serverless and edge function execution engine.
package function

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionControlPlaneHandlerEndpointsIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	controlPlaneHandler := service.ControlPlaneHandler()

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(testCtx, core.CreateServiceAccountInput{
		Name: "endpoints-admin",
		Scopes: []string{
			core.ScopeFunctionEndpointRead,
			core.ScopeFunctionEndpointWrite,
		},
	})
	require.NoError(t, accountErr)
	authHeader := "Bearer " + serviceAccount.SecretKey

	var createdEndpointID uuid.UUID

	t.Run("endpoint lifecycle CRUD and visibility", func(t *testing.T) {
		// 1. Create public endpoint
		createEndpointInput := CreateEndpointInput{
			Name: "integration-fn",
		}
		createJSON, _ := json.Marshal(createEndpointInput)
		createRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/_/function/endpoints", bytes.NewReader(createJSON))
		createRequest.Header.Set("Authorization", authHeader)
		createResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateEndpoint(createResponseRecorder, createRequest)
		require.Equal(t, http.StatusCreated, createResponseRecorder.Code)

		var storedEndpoint Endpoint
		require.NoError(t, json.Unmarshal(createResponseRecorder.Body.Bytes(), &storedEndpoint))
		createdEndpointID = storedEndpoint.ID
		require.True(t, storedEndpoint.IsPublic)

		// 2. List endpoints
		listRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/endpoints", nil)
		listRequest.Header.Set("Authorization", authHeader)
		listResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListEndpoints(listResponseRecorder, listRequest)
		require.Equal(t, http.StatusOK, listResponseRecorder.Code)

		// 3. Get endpoint
		getRequest := httptest.NewRequestWithContext(testCtx, http.MethodGet, "/v1/_/function/endpoints/"+createdEndpointID.String(), nil)
		getRequest.SetPathValue("id", createdEndpointID.String())
		getRequest.Header.Set("Authorization", authHeader)
		getResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetEndpoint(getResponseRecorder, getRequest)
		require.Equal(t, http.StatusOK, getResponseRecorder.Code)

		// 4. Update endpoint metadata and visibility to private
		newDescription := "Updated function description"
		isPublicFalse := false
		updateEndpointInput := UpdateEndpointInput{
			Description: &newDescription,
			IsPublic:    &isPublicFalse,
		}
		updateJSON, _ := json.Marshal(updateEndpointInput)
		updateRequest := httptest.NewRequestWithContext(testCtx, http.MethodPut, "/v1/_/function/endpoints/"+createdEndpointID.String(), bytes.NewReader(updateJSON))
		updateRequest.SetPathValue("id", createdEndpointID.String())
		updateRequest.Header.Set("Authorization", authHeader)
		updateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateEndpoint(updateResponseRecorder, updateRequest)
		require.Equal(t, http.StatusOK, updateResponseRecorder.Code)

		var updatedEndpoint Endpoint
		require.NoError(t, json.Unmarshal(updateResponseRecorder.Body.Bytes(), &updatedEndpoint))
		require.False(t, updatedEndpoint.IsPublic)
		require.Equal(t, "Updated function description", *updatedEndpoint.Description)

		// 5. Delete endpoint
		deleteRequest := httptest.NewRequestWithContext(testCtx, http.MethodDelete, "/v1/_/function/endpoints/"+createdEndpointID.String(), nil)
		deleteRequest.SetPathValue("id", createdEndpointID.String())
		deleteRequest.Header.Set("Authorization", authHeader)
		deleteResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteEndpoint(deleteResponseRecorder, deleteRequest)
		require.Equal(t, http.StatusNoContent, deleteResponseRecorder.Code)
	})
}
