package console

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestConsoleControlPlaneHandlerUsersIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(ctx))
	defer service.Stop()

	coreServer := core.NewServer(kernel)
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	serviceAccount, err := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "console-users-operator",
		Scopes: []string{
			core.ScopeConsoleUserRead,
			core.ScopeConsoleUserWrite,
		},
	})
	require.NoError(t, err)
	serviceAccountKey := serviceAccount.SecretKey

	var createdUserID string

	t.Run("create console user", func(t *testing.T) {
		payload := `{"email":"operator@example.com","password":"ValidPassword123!"}`
		createRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/users", bytes.NewReader([]byte(payload)))
		createRequest.Header.Set("Content-Type", "application/json")
		createRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		createResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(createResponseRecorder, createRequest)
		require.Equal(t, http.StatusCreated, createResponseRecorder.Code)

		var user User
		require.NoError(t, json.Unmarshal(createResponseRecorder.Body.Bytes(), &user))
		require.Equal(t, "operator@example.com", user.Email)
		require.True(t, user.IsEnabled)
		require.NotEmpty(t, user.ID)
		createdUserID = user.ID

		// Duplicate email conflict
		conflictResponseRecorder := httptest.NewRecorder()
		conflictRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/users", bytes.NewReader([]byte(payload)))
		conflictRequest.Header.Set("Content-Type", "application/json")
		conflictRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		coreServer.Handler().ServeHTTP(conflictResponseRecorder, conflictRequest)
		require.Equal(t, http.StatusConflict, conflictResponseRecorder.Code)
	})

	t.Run("list console users", func(t *testing.T) {
		listRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/users", nil)
		listRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		listResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(listResponseRecorder, listRequest)
		require.Equal(t, http.StatusOK, listResponseRecorder.Code)

		var listConsoleUsersResponse ListConsoleUsersResponse
		require.NoError(t, json.Unmarshal(listResponseRecorder.Body.Bytes(), &listConsoleUsersResponse))
		require.NotEmpty(t, listConsoleUsersResponse)
	})

	t.Run("get console user", func(t *testing.T) {
		getUserRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/users/"+createdUserID, nil)
		getUserRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		getUserResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(getUserResponseRecorder, getUserRequest)
		require.Equal(t, http.StatusOK, getUserResponseRecorder.Code)

		var user User
		require.NoError(t, json.Unmarshal(getUserResponseRecorder.Body.Bytes(), &user))
		require.Equal(t, createdUserID, user.ID)

		// Not found
		missingID := uuid.NewV7().String()
		notFoundRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/users/"+missingID, nil)
		notFoundRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		notFoundResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(notFoundResponseRecorder, notFoundRequest)
		require.Equal(t, http.StatusNotFound, notFoundResponseRecorder.Code)
	})

	t.Run("update console user", func(t *testing.T) {
		payload := `{"email":"operator_updated@example.com","is_enabled":false}`
		updateRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/console/users/"+createdUserID, bytes.NewReader([]byte(payload)))
		updateRequest.Header.Set("Content-Type", "application/json")
		updateRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		updateResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(updateResponseRecorder, updateRequest)
		require.Equal(t, http.StatusOK, updateResponseRecorder.Code)

		var user User
		require.NoError(t, json.Unmarshal(updateResponseRecorder.Body.Bytes(), &user))
		require.Equal(t, "operator_updated@example.com", user.Email)
		require.False(t, user.IsEnabled)

		// Create another user to test unique constraint collision
		secondPayload := `{"email":"second_operator@example.com","password":"ValidPassword123!"}`
		secondRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/users", bytes.NewReader([]byte(secondPayload)))
		secondRequest.Header.Set("Content-Type", "application/json")
		secondRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		secondResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(secondResponseRecorder, secondRequest)
		require.Equal(t, http.StatusCreated, secondResponseRecorder.Code)

		// Update to existing email -> 409
		conflictPayload := `{"email":"second_operator@example.com"}`
		conflictUpdateRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/console/users/"+createdUserID, bytes.NewReader([]byte(conflictPayload)))
		conflictUpdateRequest.Header.Set("Content-Type", "application/json")
		conflictUpdateRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		conflictUpdateResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(conflictUpdateResponseRecorder, conflictUpdateRequest)
		require.Equal(t, http.StatusConflict, conflictUpdateResponseRecorder.Code)

		// Not found
		missingID := uuid.NewV7().String()
		notFoundUpdateRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/console/users/"+missingID, bytes.NewReader([]byte(payload)))
		notFoundUpdateRequest.Header.Set("Content-Type", "application/json")
		notFoundUpdateRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		notFoundUpdateResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(notFoundUpdateResponseRecorder, notFoundUpdateRequest)
		require.Equal(t, http.StatusNotFound, notFoundUpdateResponseRecorder.Code)
	})

	t.Run("reset console user password", func(t *testing.T) {
		payload := `{"new_password":"BrandNewPassword123!"}`
		resetRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/users/"+createdUserID+"/password", bytes.NewReader([]byte(payload)))
		resetRequest.Header.Set("Content-Type", "application/json")
		resetRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		resetResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(resetResponseRecorder, resetRequest)
		require.Equal(t, http.StatusOK, resetResponseRecorder.Code)

		// Not found
		missingID := uuid.NewV7().String()
		missingResetRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/users/"+missingID+"/password", bytes.NewReader([]byte(payload)))
		missingResetRequest.Header.Set("Content-Type", "application/json")
		missingResetRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		missingResetResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(missingResetResponseRecorder, missingResetRequest)
		require.Equal(t, http.StatusNotFound, missingResetResponseRecorder.Code)
	})

	t.Run("session management lifecycle", func(t *testing.T) {
		// Insert test sessions into DB
		rawToken1 := kernel.JWTSigner().GenerateRefreshToken()
		hash1 := kernel.JWTSigner().HashRefreshToken(rawToken1)
		rawToken2 := kernel.JWTSigner().GenerateRefreshToken()
		hash2 := kernel.JWTSigner().HashRefreshToken(rawToken2)

		var sessionID1, sessionID2 string
		insertSQL := `
			INSERT INTO console.sessions (user_id, session_token_hash, ip_address, user_agent, expires_at)
			VALUES ($1, $2, '127.0.0.1'::inet, 'Go-Test', $3)
			RETURNING id
		`
		expiresAt := time.Now().UTC().Add(time.Hour)
		require.NoError(t, kernel.DB().QueryRow(ctx, insertSQL, createdUserID, hash1, expiresAt).Scan(&sessionID1))
		require.NoError(t, kernel.DB().QueryRow(ctx, insertSQL, createdUserID, hash2, expiresAt).Scan(&sessionID2))

		// List sessions
		listSessionsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/users/"+createdUserID+"/sessions", nil)
		listSessionsRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		listSessionsResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(listSessionsResponseRecorder, listSessionsRequest)
		require.Equal(t, http.StatusOK, listSessionsResponseRecorder.Code)

		var listConsoleUserSessionsResponse ListConsoleUserSessionsResponse
		require.NoError(t, json.Unmarshal(listSessionsResponseRecorder.Body.Bytes(), &listConsoleUserSessionsResponse))
		require.Len(t, listConsoleUserSessionsResponse, 2)

		// Delete specific session
		delSessionRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/console/users/"+createdUserID+"/sessions/"+sessionID1, nil)
		delSessionRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		delSessionResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(delSessionResponseRecorder, delSessionRequest)
		require.Equal(t, http.StatusNoContent, delSessionResponseRecorder.Code)

		// Delete nonexistent session -> 404
		missingSessionID := uuid.NewV7().String()
		delMissingSessionRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/console/users/"+createdUserID+"/sessions/"+missingSessionID, nil)
		delMissingSessionRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		delMissingSessionResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(delMissingSessionResponseRecorder, delMissingSessionRequest)
		require.Equal(t, http.StatusNotFound, delMissingSessionResponseRecorder.Code)

		// Revoke all remaining sessions
		revokeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/console/users/"+createdUserID+"/sessions/revoke", nil)
		revokeRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		revokeResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(revokeResponseRecorder, revokeRequest)
		require.Equal(t, http.StatusNoContent, revokeResponseRecorder.Code)

		// Verify 0 sessions remaining
		listSessionsAfterRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/console/users/"+createdUserID+"/sessions", nil)
		listSessionsAfterRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		listSessionsAfterResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(listSessionsAfterResponseRecorder, listSessionsAfterRequest)
		require.Equal(t, http.StatusOK, listSessionsAfterResponseRecorder.Code)

		var emptyListConsoleUserSessionsResponse ListConsoleUserSessionsResponse
		require.NoError(t, json.Unmarshal(listSessionsAfterResponseRecorder.Body.Bytes(), &emptyListConsoleUserSessionsResponse))
		require.Empty(t, emptyListConsoleUserSessionsResponse)
	})

	t.Run("delete console user", func(t *testing.T) {
		deleteUserRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/console/users/"+createdUserID, nil)
		deleteUserRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		deleteUserResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(deleteUserResponseRecorder, deleteUserRequest)
		require.Equal(t, http.StatusNoContent, deleteUserResponseRecorder.Code)

		// Delete again -> 404
		secondDelResponseRecorder := httptest.NewRecorder()
		secondDelRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/console/users/"+createdUserID, nil)
		secondDelRequest.Header.Set("X-Service-Account-Key", serviceAccountKey)
		coreServer.Handler().ServeHTTP(secondDelResponseRecorder, secondDelRequest)
		require.Equal(t, http.StatusNotFound, secondDelResponseRecorder.Code)
	})
}
