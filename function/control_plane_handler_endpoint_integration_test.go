// Package function defines the serverless and edge function execution engine.
package function

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionControlPlaneHandlerEndpointDeletionRegressionIntegration(t *testing.T) {
	ctx := context.Background()
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	service := NewService(kernel)
	service.engine.RegisterRunner(&mockRunner{name: "workerd", undeployErr: errors.New("audit undeploy failure")})
	serviceAccount, err := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{Name: "audit-delete", Scopes: []string{core.ScopeRoot}})
	require.NoError(t, err)
	endpoint := insertTestEndpoint(ctx, t, kernel, "audit-delete", true)
	request := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/", nil)
	request.SetPathValue("endpoint_id", endpoint.ID.String())
	request.Header.Set("Authorization", "Bearer "+serviceAccount.SecretKey)
	responseRecorder := httptest.NewRecorder()
	service.ControlPlaneHandler().handleDeleteEndpoint(responseRecorder, request)
	require.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	_, err = service.ControlPlaneHandler().fetchEndpointByID(ctx, endpoint.ID)
	require.NoError(t, err)
}

func TestFunctionControlPlaneHandlerEndpointTransactionsIntegration(t *testing.T) {
	ctx := context.Background()
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	endpoint := insertTestEndpoint(ctx, t, kernel, "delete-transaction-failures", true)
	for _, stage := range []string{"begin", "FOR UPDATE", "DELETE FROM function.endpoints", "commit"} {
		t.Run(stage, func(t *testing.T) {
			config := kernel.DB().Config()
			config.ConnConfig.Tracer = &queryFailureTracer{matches: func(query string) bool { return strings.Contains(query, stage) }}
			pool, connectionErr := pgxpool.NewWithConfig(ctx, config)
			require.NoError(t, connectionErr)
			defer pool.Close()
			service := NewService(core.NewTestKernel(&core.DatabasePool{Pool: pool}))
			service.engine.RegisterRunner(&mockRunner{name: "workerd"})
			authCtx := core.WithAuthContext(ctx, core.AuthContext{ServiceAccountID: "transaction-test", JWT: core.JWTClaims{Role: "service_role", Scope: core.ScopeRoot}})
			request := httptest.NewRequestWithContext(authCtx, http.MethodDelete, "/", nil)
			request.SetPathValue("endpoint_id", endpoint.ID.String())
			responseRecorder := httptest.NewRecorder()
			service.ControlPlaneHandler().handleDeleteEndpoint(responseRecorder, request)
			require.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		})
	}
}

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
		getRequest.SetPathValue("endpoint_id", createdEndpointID.String())
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
		updateRequest.SetPathValue("endpoint_id", createdEndpointID.String())
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
		deleteRequest.SetPathValue("endpoint_id", createdEndpointID.String())
		deleteRequest.Header.Set("Authorization", authHeader)
		deleteResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteEndpoint(deleteResponseRecorder, deleteRequest)
		require.Equal(t, http.StatusNoContent, deleteResponseRecorder.Code)
	})
}
