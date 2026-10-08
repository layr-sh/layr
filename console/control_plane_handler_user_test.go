package console

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestConsoleControlPlaneHandlerUsersUnit(t *testing.T) {
	ctx := context.Background()
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler
	testUserID := uuid.NewV7().String()
	testSessionID := uuid.NewV7().String()

	// 1. Forbidden Scopes (Empty/Unauthorized scopes)
	unauthorizedCtx := core.WithAuthContext(ctx, core.AuthContext{
		ServiceAccountID: "sa-unauth",
		JWT: core.JWTClaims{
			Subject: "sa-unauth",
			Role:    "service_role",
			Scope:   "",
		},
	})

	t.Run("forbidden scope checks", func(t *testing.T) {
		listRequest := httptest.NewRequestWithContext(unauthorizedCtx, http.MethodGet, "/v1/_/console/users", nil)
		listResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListConsoleUsers(listResponseRecorder, listRequest)
		require.Equal(t, http.StatusForbidden, listResponseRecorder.Code)

		createRequest := httptest.NewRequestWithContext(unauthorizedCtx, http.MethodPost, "/v1/_/console/users", bytes.NewReader([]byte(`{}`)))
		createResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateConsoleUser(createResponseRecorder, createRequest)
		require.Equal(t, http.StatusForbidden, createResponseRecorder.Code)

		getUserRequest := httptest.NewRequestWithContext(unauthorizedCtx, http.MethodGet, "/v1/_/console/users/"+testUserID, nil)
		getUserRequest.SetPathValue("user_id", testUserID)
		getUserResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConsoleUser(getUserResponseRecorder, getUserRequest)
		require.Equal(t, http.StatusForbidden, getUserResponseRecorder.Code)

		updateRequest := httptest.NewRequestWithContext(unauthorizedCtx, http.MethodPut, "/v1/_/console/users/"+testUserID, bytes.NewReader([]byte(`{}`)))
		updateRequest.SetPathValue("user_id", testUserID)
		updateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConsoleUser(updateResponseRecorder, updateRequest)
		require.Equal(t, http.StatusForbidden, updateResponseRecorder.Code)

		deleteRequest := httptest.NewRequestWithContext(unauthorizedCtx, http.MethodDelete, "/v1/_/console/users/"+testUserID, nil)
		deleteRequest.SetPathValue("user_id", testUserID)
		deleteResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteConsoleUser(deleteResponseRecorder, deleteRequest)
		require.Equal(t, http.StatusForbidden, deleteResponseRecorder.Code)

		resetRequest := httptest.NewRequestWithContext(unauthorizedCtx, http.MethodPost, "/v1/_/console/users/"+testUserID+"/password", bytes.NewReader([]byte(`{}`)))
		resetRequest.SetPathValue("user_id", testUserID)
		resetResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleResetConsoleUserPassword(resetResponseRecorder, resetRequest)
		require.Equal(t, http.StatusForbidden, resetResponseRecorder.Code)

		listSessionsRequest := httptest.NewRequestWithContext(unauthorizedCtx, http.MethodGet, "/v1/_/console/users/"+testUserID+"/sessions", nil)
		listSessionsRequest.SetPathValue("user_id", testUserID)
		listSessionsResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListConsoleUserSessions(listSessionsResponseRecorder, listSessionsRequest)
		require.Equal(t, http.StatusForbidden, listSessionsResponseRecorder.Code)

		delSessionRequest := httptest.NewRequestWithContext(unauthorizedCtx, http.MethodDelete, "/v1/_/console/users/"+testUserID+"/sessions/"+testSessionID, nil)
		delSessionRequest.SetPathValue("user_id", testUserID)
		delSessionRequest.SetPathValue("session_id", testSessionID)
		delSessionResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteConsoleUserSession(delSessionResponseRecorder, delSessionRequest)
		require.Equal(t, http.StatusForbidden, delSessionResponseRecorder.Code)

		revokeRequest := httptest.NewRequestWithContext(unauthorizedCtx, http.MethodPost, "/v1/_/console/users/"+testUserID+"/sessions/revoke", nil)
		revokeRequest.SetPathValue("user_id", testUserID)
		revokeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRevokeConsoleUserSessions(revokeResponseRecorder, revokeRequest)
		require.Equal(t, http.StatusForbidden, revokeResponseRecorder.Code)
	})

	authorizedCtx := core.WithAuthContext(ctx, core.AuthContext{
		ServiceAccountID: "sa-auth",
		JWT: core.JWTClaims{
			Subject: "sa-auth",
			Role:    "service_role",
			Scope:   core.ScopeConsoleUserRead + " " + core.ScopeConsoleUserWrite,
		},
	})

	// 2. Bad UUIDs
	t.Run("bad UUID parameters", func(t *testing.T) {
		invalidUUID := "not-a-valid-uuid"

		getUserRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodGet, "/v1/_/console/users/"+invalidUUID, nil)
		getUserRequest.SetPathValue("user_id", invalidUUID)
		getUserResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConsoleUser(getUserResponseRecorder, getUserRequest)
		require.Equal(t, http.StatusBadRequest, getUserResponseRecorder.Code)

		updateRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPut, "/v1/_/console/users/"+invalidUUID, bytes.NewReader([]byte(`{}`)))
		updateRequest.SetPathValue("user_id", invalidUUID)
		updateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConsoleUser(updateResponseRecorder, updateRequest)
		require.Equal(t, http.StatusBadRequest, updateResponseRecorder.Code)

		deleteRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodDelete, "/v1/_/console/users/"+invalidUUID, nil)
		deleteRequest.SetPathValue("user_id", invalidUUID)
		deleteResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteConsoleUser(deleteResponseRecorder, deleteRequest)
		require.Equal(t, http.StatusBadRequest, deleteResponseRecorder.Code)

		resetRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPost, "/v1/_/console/users/"+invalidUUID+"/password", bytes.NewReader([]byte(`{}`)))
		resetRequest.SetPathValue("user_id", invalidUUID)
		resetResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleResetConsoleUserPassword(resetResponseRecorder, resetRequest)
		require.Equal(t, http.StatusBadRequest, resetResponseRecorder.Code)

		listSessionsRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodGet, "/v1/_/console/users/"+invalidUUID+"/sessions", nil)
		listSessionsRequest.SetPathValue("user_id", invalidUUID)
		listSessionsResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListConsoleUserSessions(listSessionsResponseRecorder, listSessionsRequest)
		require.Equal(t, http.StatusBadRequest, listSessionsResponseRecorder.Code)

		delSessionRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodDelete, "/v1/_/console/users/"+invalidUUID+"/sessions/"+testSessionID, nil)
		delSessionRequest.SetPathValue("user_id", invalidUUID)
		delSessionRequest.SetPathValue("session_id", testSessionID)
		delSessionResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteConsoleUserSession(delSessionResponseRecorder, delSessionRequest)
		require.Equal(t, http.StatusBadRequest, delSessionResponseRecorder.Code)

		delSessionOtherRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodDelete, "/v1/_/console/users/"+testUserID+"/sessions/"+invalidUUID, nil)
		delSessionOtherRequest.SetPathValue("user_id", testUserID)
		delSessionOtherRequest.SetPathValue("session_id", invalidUUID)
		delSessionOtherResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteConsoleUserSession(delSessionOtherResponseRecorder, delSessionOtherRequest)
		require.Equal(t, http.StatusBadRequest, delSessionOtherResponseRecorder.Code)

		revokeRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPost, "/v1/_/console/users/"+invalidUUID+"/sessions/revoke", nil)
		revokeRequest.SetPathValue("user_id", invalidUUID)
		revokeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRevokeConsoleUserSessions(revokeResponseRecorder, revokeRequest)
		require.Equal(t, http.StatusBadRequest, revokeResponseRecorder.Code)
	})

	// 3. Bad JSON and Validation Failures
	t.Run("bad JSON and validation failures", func(t *testing.T) {
		// Create bad JSON
		badJSONCreateRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPost, "/v1/_/console/users", bytes.NewReader([]byte(`{invalid`)))
		badJSONCreateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateConsoleUser(badJSONCreateResponseRecorder, badJSONCreateRequest)
		require.Equal(t, http.StatusBadRequest, badJSONCreateResponseRecorder.Code)

		// Create invalid email
		invalidEmailCreateRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPost, "/v1/_/console/users", bytes.NewReader([]byte(`{"email":"bademail","password":"password123"}`)))
		invalidEmailCreateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateConsoleUser(invalidEmailCreateResponseRecorder, invalidEmailCreateRequest)
		require.Equal(t, http.StatusBadRequest, invalidEmailCreateResponseRecorder.Code)

		// Create short password
		shortPasswordCreateRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPost, "/v1/_/console/users", bytes.NewReader([]byte(`{"email":"test@example.com","password":"short"}`)))
		shortPasswordCreateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateConsoleUser(shortPasswordCreateResponseRecorder, shortPasswordCreateRequest)
		require.Equal(t, http.StatusBadRequest, shortPasswordCreateResponseRecorder.Code)

		// Update bad JSON
		badJSONUpdateRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPut, "/v1/_/console/users/"+testUserID, bytes.NewReader([]byte(`{invalid`)))
		badJSONUpdateRequest.SetPathValue("user_id", testUserID)
		badJSONUpdateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConsoleUser(badJSONUpdateResponseRecorder, badJSONUpdateRequest)
		require.Equal(t, http.StatusBadRequest, badJSONUpdateResponseRecorder.Code)

		// Update invalid email
		invalidEmailUpdateRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPut, "/v1/_/console/users/"+testUserID, bytes.NewReader([]byte(`{"email":"bademail"}`)))
		invalidEmailUpdateRequest.SetPathValue("user_id", testUserID)
		invalidEmailUpdateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConsoleUser(invalidEmailUpdateResponseRecorder, invalidEmailUpdateRequest)
		require.Equal(t, http.StatusBadRequest, invalidEmailUpdateResponseRecorder.Code)

		// Reset password bad JSON
		badJSONResetRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPost, "/v1/_/console/users/"+testUserID+"/password", bytes.NewReader([]byte(`{invalid`)))
		badJSONResetRequest.SetPathValue("user_id", testUserID)
		badJSONResetResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleResetConsoleUserPassword(badJSONResetResponseRecorder, badJSONResetRequest)
		require.Equal(t, http.StatusBadRequest, badJSONResetResponseRecorder.Code)

		// Reset password short
		shortPasswordResetRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPost, "/v1/_/console/users/"+testUserID+"/password", bytes.NewReader([]byte(`{"new_password":"short"}`)))
		shortPasswordResetRequest.SetPathValue("user_id", testUserID)
		shortPasswordResetResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleResetConsoleUserPassword(shortPasswordResetResponseRecorder, shortPasswordResetRequest)
		require.Equal(t, http.StatusBadRequest, shortPasswordResetResponseRecorder.Code)
	})

	// 4. Broken Database (returns 500)
	t.Run("broken database errors", func(t *testing.T) {
		listRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodGet, "/v1/_/console/users", nil)
		listResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListConsoleUsers(listResponseRecorder, listRequest)
		require.Equal(t, http.StatusInternalServerError, listResponseRecorder.Code)

		createRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPost, "/v1/_/console/users", bytes.NewReader([]byte(`{"email":"test@example.com","password":"validpassword123"}`)))
		createResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleCreateConsoleUser(createResponseRecorder, createRequest)
		require.Equal(t, http.StatusInternalServerError, createResponseRecorder.Code)

		getUserRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodGet, "/v1/_/console/users/"+testUserID, nil)
		getUserRequest.SetPathValue("user_id", testUserID)
		getUserResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetConsoleUser(getUserResponseRecorder, getUserRequest)
		require.Equal(t, http.StatusInternalServerError, getUserResponseRecorder.Code)

		updateRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPut, "/v1/_/console/users/"+testUserID, bytes.NewReader([]byte(`{"email":"test2@example.com"}`)))
		updateRequest.SetPathValue("user_id", testUserID)
		updateResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleUpdateConsoleUser(updateResponseRecorder, updateRequest)
		require.Equal(t, http.StatusInternalServerError, updateResponseRecorder.Code)

		deleteRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodDelete, "/v1/_/console/users/"+testUserID, nil)
		deleteRequest.SetPathValue("user_id", testUserID)
		deleteResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteConsoleUser(deleteResponseRecorder, deleteRequest)
		require.Equal(t, http.StatusInternalServerError, deleteResponseRecorder.Code)

		resetRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPost, "/v1/_/console/users/"+testUserID+"/password", bytes.NewReader([]byte(`{"new_password":"newvalidpassword123"}`)))
		resetRequest.SetPathValue("user_id", testUserID)
		resetResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleResetConsoleUserPassword(resetResponseRecorder, resetRequest)
		require.Equal(t, http.StatusInternalServerError, resetResponseRecorder.Code)

		listSessionsRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodGet, "/v1/_/console/users/"+testUserID+"/sessions", nil)
		listSessionsRequest.SetPathValue("user_id", testUserID)
		listSessionsResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleListConsoleUserSessions(listSessionsResponseRecorder, listSessionsRequest)
		require.Equal(t, http.StatusInternalServerError, listSessionsResponseRecorder.Code)

		delSessionRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodDelete, "/v1/_/console/users/"+testUserID+"/sessions/"+testSessionID, nil)
		delSessionRequest.SetPathValue("user_id", testUserID)
		delSessionRequest.SetPathValue("session_id", testSessionID)
		delSessionResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleDeleteConsoleUserSession(delSessionResponseRecorder, delSessionRequest)
		require.Equal(t, http.StatusInternalServerError, delSessionResponseRecorder.Code)

		revokeRequest := httptest.NewRequestWithContext(authorizedCtx, http.MethodPost, "/v1/_/console/users/"+testUserID+"/sessions/revoke", nil)
		revokeRequest.SetPathValue("user_id", testUserID)
		revokeResponseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleRevokeConsoleUserSessions(revokeResponseRecorder, revokeRequest)
		require.Equal(t, http.StatusInternalServerError, revokeResponseRecorder.Code)
	})
}
