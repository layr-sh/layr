// Package function defines the serverless and edge function execution engine.
package function

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionControlPlaneHandlerEndpointsUnit(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	testCtx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(testCtx))
	defer service.Stop()

	controlPlaneHandler := service.ControlPlaneHandler()

	readAuthContext := core.AuthContext{
		ServiceAccountID: "sa-endpoint-read",
		JWT: core.JWTClaims{
			Subject:  "sa-endpoint-read",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFunctionEndpointRead,
		},
	}
	readCtx := core.WithAuthContext(testCtx, readAuthContext)

	writeAuthContext := core.AuthContext{
		ServiceAccountID: "sa-endpoint-write",
		JWT: core.JWTClaims{
			Subject:  "sa-endpoint-write",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    core.ScopeFunctionEndpointWrite,
		},
	}
	writeCtx := core.WithAuthContext(testCtx, writeAuthContext)

	noScopeAuthContext := core.AuthContext{
		ServiceAccountID: "sa-no-scope",
		JWT: core.JWTClaims{
			Subject:  "sa-no-scope",
			Role:     "service_role",
			Audience: "test:service_account",
			Scope:    "",
		},
	}
	noScopeCtx := core.WithAuthContext(testCtx, noScopeAuthContext)

	randomUUID := "01923456-789a-7bc8-9def-0123456789ab"

	t.Run("Scope permissions and denials", func(t *testing.T) {
		// List without scope -> 403
		listNoScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/function/endpoints", nil)
		listNoScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListEndpoints(listNoScopeResponseRecorder, listNoScopeRequest)
		require.Equal(t, http.StatusForbidden, listNoScopeResponseRecorder.Code)

		// List with read scope -> 200
		listValidRequest := httptest.NewRequestWithContext(readCtx, http.MethodGet, "/v1/_/function/endpoints", nil)
		listValidResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListEndpoints(listValidResponseRecorder, listValidRequest)
		require.Equal(t, http.StatusOK, listValidResponseRecorder.Code)

		// Create without scope -> 403
		createNoScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPost, "/v1/_/function/endpoints", nil)
		createNoScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateEndpoint(createNoScopeResponseRecorder, createNoScopeRequest)
		require.Equal(t, http.StatusForbidden, createNoScopeResponseRecorder.Code)

		// Get without scope -> 403
		getNoScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodGet, "/v1/_/function/endpoints/"+randomUUID, nil)
		getNoScopeRequest.SetPathValue("id", randomUUID)
		getNoScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetEndpoint(getNoScopeResponseRecorder, getNoScopeRequest)
		require.Equal(t, http.StatusForbidden, getNoScopeResponseRecorder.Code)

		// Update without scope -> 403
		updateNoScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodPut, "/v1/_/function/endpoints/"+randomUUID, nil)
		updateNoScopeRequest.SetPathValue("id", randomUUID)
		updateNoScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateEndpoint(updateNoScopeResponseRecorder, updateNoScopeRequest)
		require.Equal(t, http.StatusForbidden, updateNoScopeResponseRecorder.Code)

		// Delete without scope -> 403
		deleteNoScopeRequest := httptest.NewRequestWithContext(noScopeCtx, http.MethodDelete, "/v1/_/function/endpoints/"+randomUUID, nil)
		deleteNoScopeRequest.SetPathValue("id", randomUUID)
		deleteNoScopeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteEndpoint(deleteNoScopeResponseRecorder, deleteNoScopeRequest)
		require.Equal(t, http.StatusForbidden, deleteNoScopeResponseRecorder.Code)
	})

	t.Run("Validations and edge cases", func(t *testing.T) {
		// Create - bad JSON
		badJSONRequest := httptest.NewRequestWithContext(writeCtx, http.MethodPost, "/v1/_/function/endpoints", bytes.NewReader([]byte("{invalid-json")))
		badJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateEndpoint(badJSONResponseRecorder, badJSONRequest)
		require.Equal(t, http.StatusBadRequest, badJSONResponseRecorder.Code)

		// Create - invalid name
		badNameCreateEndpointInput := CreateEndpointInput{Name: "INVALID NAME!"}
		badNameJSON, _ := json.Marshal(badNameCreateEndpointInput)
		badNameRequest := httptest.NewRequestWithContext(writeCtx, http.MethodPost, "/v1/_/function/endpoints", bytes.NewReader(badNameJSON))
		badNameResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateEndpoint(badNameResponseRecorder, badNameRequest)
		require.Equal(t, http.StatusBadRequest, badNameResponseRecorder.Code)

		// Create - valid
		validCreateEndpointInput := CreateEndpointInput{Name: "test-endpoint"}
		validJSON, _ := json.Marshal(validCreateEndpointInput)
		validCreateRequest := httptest.NewRequestWithContext(writeCtx, http.MethodPost, "/v1/_/function/endpoints", bytes.NewReader(validJSON))
		validCreateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateEndpoint(validCreateResponseRecorder, validCreateRequest)
		require.Equal(t, http.StatusCreated, validCreateResponseRecorder.Code)

		var storedEndpoint Endpoint
		require.NoError(t, json.Unmarshal(validCreateResponseRecorder.Body.Bytes(), &storedEndpoint))

		// Create - conflict
		conflictRequest := httptest.NewRequestWithContext(writeCtx, http.MethodPost, "/v1/_/function/endpoints", bytes.NewReader(validJSON))
		conflictResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateEndpoint(conflictResponseRecorder, conflictRequest)
		require.Equal(t, http.StatusConflict, conflictResponseRecorder.Code)

		// Get - invalid UUID
		getBadUUIDRequest := httptest.NewRequestWithContext(readCtx, http.MethodGet, "/v1/_/function/endpoints/invalid-uuid", nil)
		getBadUUIDRequest.SetPathValue("id", "invalid-uuid")
		getBadUUIDResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetEndpoint(getBadUUIDResponseRecorder, getBadUUIDRequest)
		require.Equal(t, http.StatusBadRequest, getBadUUIDResponseRecorder.Code)

		// Get - not found
		notFoundUUID := uuid.New().String()
		getNotFoundRequest := httptest.NewRequestWithContext(readCtx, http.MethodGet, "/v1/_/function/endpoints/"+notFoundUUID, nil)
		getNotFoundRequest.SetPathValue("id", notFoundUUID)
		getNotFoundResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetEndpoint(getNotFoundResponseRecorder, getNotFoundRequest)
		require.Equal(t, http.StatusNotFound, getNotFoundResponseRecorder.Code)

		// Get - valid
		getValidRequest := httptest.NewRequestWithContext(readCtx, http.MethodGet, "/v1/_/function/endpoints/"+storedEndpoint.ID.String(), nil)
		getValidRequest.SetPathValue("id", storedEndpoint.ID.String())
		getValidResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetEndpoint(getValidResponseRecorder, getValidRequest)
		require.Equal(t, http.StatusOK, getValidResponseRecorder.Code)

		// Update - invalid UUID
		updateBadUUIDRequest := httptest.NewRequestWithContext(writeCtx, http.MethodPut, "/v1/_/function/endpoints/invalid-uuid", nil)
		updateBadUUIDRequest.SetPathValue("id", "invalid-uuid")
		updateBadUUIDResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateEndpoint(updateBadUUIDResponseRecorder, updateBadUUIDRequest)
		require.Equal(t, http.StatusBadRequest, updateBadUUIDResponseRecorder.Code)

		// Update - bad JSON
		updateBadJSONRequest := httptest.NewRequestWithContext(writeCtx, http.MethodPut, "/v1/_/function/endpoints/"+storedEndpoint.ID.String(), bytes.NewReader([]byte("{invalid-json")))
		updateBadJSONRequest.SetPathValue("id", storedEndpoint.ID.String())
		updateBadJSONResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateEndpoint(updateBadJSONResponseRecorder, updateBadJSONRequest)
		require.Equal(t, http.StatusBadRequest, updateBadJSONResponseRecorder.Code)

		// Update - not found
		updateNotFoundRequest := httptest.NewRequestWithContext(writeCtx, http.MethodPut, "/v1/_/function/endpoints/"+notFoundUUID, bytes.NewReader([]byte("{}")))
		updateNotFoundRequest.SetPathValue("id", notFoundUUID)
		updateNotFoundResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateEndpoint(updateNotFoundResponseRecorder, updateNotFoundRequest)
		require.Equal(t, http.StatusNotFound, updateNotFoundResponseRecorder.Code)

		// Update - valid (updating all fields: description, runtime, entrypoint, memory_limit_mb, timeout_seconds, is_public)
		newDescription := "updated description"
		newRuntime := "workerd"
		newEntrypoint := "worker.js"
		newMemoryLimitMB := 256
		newTimeoutSeconds := 60
		newIsPublic := false
		updateEndpointInput := UpdateEndpointInput{
			Description:    &newDescription,
			Runtime:        &newRuntime,
			Entrypoint:     &newEntrypoint,
			MemoryLimitMB:  &newMemoryLimitMB,
			TimeoutSeconds: &newTimeoutSeconds,
			IsPublic:       &newIsPublic,
		}
		updatePayloadJSON, _ := json.Marshal(updateEndpointInput)
		updateValidRequest := httptest.NewRequestWithContext(writeCtx, http.MethodPut, "/v1/_/function/endpoints/"+storedEndpoint.ID.String(), bytes.NewReader(updatePayloadJSON))
		updateValidRequest.SetPathValue("id", storedEndpoint.ID.String())
		updateValidResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateEndpoint(updateValidResponseRecorder, updateValidRequest)
		require.Equal(t, http.StatusOK, updateValidResponseRecorder.Code)

		// Delete - invalid UUID
		deleteBadUUIDRequest := httptest.NewRequestWithContext(writeCtx, http.MethodDelete, "/v1/_/function/endpoints/invalid-uuid", nil)
		deleteBadUUIDRequest.SetPathValue("id", "invalid-uuid")
		deleteBadUUIDResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteEndpoint(deleteBadUUIDResponseRecorder, deleteBadUUIDRequest)
		require.Equal(t, http.StatusBadRequest, deleteBadUUIDResponseRecorder.Code)

		// Delete - not found
		deleteNotFoundRequest := httptest.NewRequestWithContext(writeCtx, http.MethodDelete, "/v1/_/function/endpoints/"+notFoundUUID, nil)
		deleteNotFoundRequest.SetPathValue("id", notFoundUUID)
		deleteNotFoundResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteEndpoint(deleteNotFoundResponseRecorder, deleteNotFoundRequest)
		require.Equal(t, http.StatusNotFound, deleteNotFoundResponseRecorder.Code)

		// Update - DB error during update (via trigger raising exception) (line 295)
		_, createUpdateTrgErr := kernel.DB().Exec(testCtx, `
			CREATE OR REPLACE FUNCTION function.trg_fail_update_endpoint() RETURNS trigger AS $$
			BEGIN
				RAISE EXCEPTION 'simulated update endpoint failure';
			END;
			$$ LANGUAGE plpgsql;
			CREATE TRIGGER trg_test_fail_update_endpoint BEFORE UPDATE ON function.endpoints
			FOR EACH ROW EXECUTE FUNCTION function.trg_fail_update_endpoint();
		`)
		require.NoError(t, createUpdateTrgErr)

		failUpdateRequest := httptest.NewRequestWithContext(writeCtx, http.MethodPut, "/v1/_/function/endpoints/"+storedEndpoint.ID.String(), strings.NewReader(`{"description":"fail-update"}`))
		failUpdateRequest.SetPathValue("id", storedEndpoint.ID.String())
		failUpdateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateEndpoint(failUpdateResponseRecorder, failUpdateRequest)
		require.Equal(t, http.StatusInternalServerError, failUpdateResponseRecorder.Code)

		_, dropUpdateTrgErr := kernel.DB().Exec(testCtx, `DROP TRIGGER trg_test_fail_update_endpoint ON function.endpoints;`)
		require.NoError(t, dropUpdateTrgErr)

		// Delete - DB error during delete (via trigger raising exception) (line 343)
		_, createDeleteTrgErr := kernel.DB().Exec(testCtx, `
			CREATE OR REPLACE FUNCTION function.trg_fail_delete_endpoint() RETURNS trigger AS $$
			BEGIN
				RAISE EXCEPTION 'simulated delete endpoint failure';
			END;
			$$ LANGUAGE plpgsql;
			CREATE TRIGGER trg_test_fail_delete_endpoint BEFORE DELETE ON function.endpoints
			FOR EACH ROW EXECUTE FUNCTION function.trg_fail_delete_endpoint();
		`)
		require.NoError(t, createDeleteTrgErr)

		failDeleteRequest := httptest.NewRequestWithContext(writeCtx, http.MethodDelete, "/v1/_/function/endpoints/"+storedEndpoint.ID.String(), nil)
		failDeleteRequest.SetPathValue("id", storedEndpoint.ID.String())
		failDeleteResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteEndpoint(failDeleteResponseRecorder, failDeleteRequest)
		require.Equal(t, http.StatusInternalServerError, failDeleteResponseRecorder.Code)

		_, dropDeleteTrgErr := kernel.DB().Exec(testCtx, `DROP TRIGGER trg_test_fail_delete_endpoint ON function.endpoints;`)
		require.NoError(t, dropDeleteTrgErr)

		// Delete - valid
		deleteValidRequest := httptest.NewRequestWithContext(writeCtx, http.MethodDelete, "/v1/_/function/endpoints/"+storedEndpoint.ID.String(), nil)
		deleteValidRequest.SetPathValue("id", storedEndpoint.ID.String())
		deleteValidResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteEndpoint(deleteValidResponseRecorder, deleteValidRequest)
		require.Equal(t, http.StatusNoContent, deleteValidResponseRecorder.Code)
	})
}

func TestFunctionControlPlaneHandlerEndpointsDatabaseFailureUnit(t *testing.T) {
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

	randomUUID := "01923456-789a-7bc8-9def-0123456789ab"

	// List DB failure -> 500
	listRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/endpoints", nil)
	listResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListEndpoints(listResponseRecorder, listRequest)
	require.Equal(t, http.StatusInternalServerError, listResponseRecorder.Code)

	// Create DB failure -> 500
	createRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPost, "/v1/_/function/endpoints", strings.NewReader(`{"name":"test-fn"}`))
	createResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateEndpoint(createResponseRecorder, createRequest)
	require.Equal(t, http.StatusInternalServerError, createResponseRecorder.Code)

	// Get DB failure -> 500
	getRequest := httptest.NewRequestWithContext(rootCtx, http.MethodGet, "/v1/_/function/endpoints/"+randomUUID, nil)
	getRequest.SetPathValue("id", randomUUID)
	getResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetEndpoint(getResponseRecorder, getRequest)
	require.Equal(t, http.StatusInternalServerError, getResponseRecorder.Code)

	// Update DB failure -> 500
	updateRequest := httptest.NewRequestWithContext(rootCtx, http.MethodPut, "/v1/_/function/endpoints/"+randomUUID, strings.NewReader(`{"entrypoint":"index.js"}`))
	updateRequest.SetPathValue("id", randomUUID)
	updateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUpdateEndpoint(updateResponseRecorder, updateRequest)
	require.Equal(t, http.StatusInternalServerError, updateResponseRecorder.Code)

	// Delete DB failure -> 500
	deleteRequest := httptest.NewRequestWithContext(rootCtx, http.MethodDelete, "/v1/_/function/endpoints/"+randomUUID, nil)
	deleteRequest.SetPathValue("id", randomUUID)
	deleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteEndpoint(deleteResponseRecorder, deleteRequest)
	require.Equal(t, http.StatusInternalServerError, deleteResponseRecorder.Code)
}
