package auth

import (
	"bytes"
	"context"
	"layr.sh/auth/password"
	"layr.sh/core"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"
)

func TestAuthControlPlaneHandlerUserUnit(t *testing.T) {
	ctx := context.Background()
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	controlPlaneHandler := NewControlPlaneHandler(nil, configManager)
	controlPlaneHandler.SetHasher(password.NewHasher())

	// 1. HandleCreateUser validation
	badJSONRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users", bytes.NewReader([]byte(`invalid-json`)))
	badJSONResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleCreateUser(badJSONResponseRecorder, badJSONRequest)
	if badJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on invalid JSON, got: %d", badJSONResponseRecorder.Code)
	}

	emptyUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users", bytes.NewReader([]byte(`{}`)))
	emptyUserResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleCreateUser(emptyUserResponseRecorder, emptyUserRequest)
	if emptyUserResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on empty user body, got: %d", emptyUserResponseRecorder.Code)
	}

	invalidPhoneUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users", bytes.NewReader([]byte(`{"phone":"invalid"}`)))
	invalidPhoneUserResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleCreateUser(invalidPhoneUserResponseRecorder, invalidPhoneUserRequest)
	if invalidPhoneUserResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on invalid phone, got: %d", invalidPhoneUserResponseRecorder.Code)
	}

	// 2. HandleListUsers with invalid pagination parameters
	paginationRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users?limit=bad&offset=bad", nil)
	paginationResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleListUsers(paginationResponseRecorder, paginationRequest)
	if paginationResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error on nil db list, got: %d", paginationResponseRecorder.Code)
	}

	validPaginationRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users?limit=25&offset=10&role=authenticated&search=alice", nil)
	validPaginationResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleListUsers(validPaginationResponseRecorder, validPaginationRequest)
	if validPaginationResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil db with valid pagination, got: %d", validPaginationResponseRecorder.Code)
	}

	// 3. HandleLockUser bad JSON duration
	testUserID := uuid.NewV7().String()
	badLockRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users/"+testUserID+"/lock", bytes.NewReader([]byte(`invalid-json`)))
	badLockRequest.SetPathValue("user_id", testUserID)
	badLockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleLockUser(badLockResponseRecorder, badLockRequest)
	if badLockResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error on nil db lock, got: %d", badLockResponseRecorder.Code)
	}

	// 4. Test Forbidden Scope on all user handlers
	serviceAccountManager := core.NewServiceAccountManager(nil)
	controlPlaneHandler.SetServiceAccountManager(serviceAccountManager)

	forbiddenRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users/"+testUserID, nil)
	forbiddenRequest.Header.Set("Authorization", "Bearer invalid-key")
	forbiddenRequest.SetPathValue("user_id", testUserID)

	forbiddenListResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleListUsers(forbiddenListResponseRecorder, forbiddenRequest)
	if forbiddenListResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on HandleListUsers, got: %d", forbiddenListResponseRecorder.Code)
	}

	forbiddenCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleCreateUser(forbiddenCreateResponseRecorder, forbiddenRequest)
	if forbiddenCreateResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on HandleCreateUser, got: %d", forbiddenCreateResponseRecorder.Code)
	}

	forbiddenGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleGetUser(forbiddenGetResponseRecorder, forbiddenRequest)
	if forbiddenGetResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on HandleGetUser, got: %d", forbiddenGetResponseRecorder.Code)
	}

	forbiddenDeleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleDeleteUser(forbiddenDeleteResponseRecorder, forbiddenRequest)
	if forbiddenDeleteResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on HandleDeleteUser, got: %d", forbiddenDeleteResponseRecorder.Code)
	}

	forbiddenLockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleLockUser(forbiddenLockResponseRecorder, forbiddenRequest)
	if forbiddenLockResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on HandleLockUser, got: %d", forbiddenLockResponseRecorder.Code)
	}

	forbiddenUnlockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleUnlockUser(forbiddenUnlockResponseRecorder, forbiddenRequest)
	if forbiddenUnlockResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on HandleUnlockUser, got: %d", forbiddenUnlockResponseRecorder.Code)
	}

	// 5. Test Invalid UUIDs on all single-user endpoints (with valid scope)
	controlPlaneHandler.SetServiceAccountManager(nil)
	invalidUUID := "not-a-valid-uuid"
	invalidUUIDRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users/"+invalidUUID, nil)
	invalidUUIDRequest.SetPathValue("user_id", invalidUUID)

	invalidUUIDGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleGetUser(invalidUUIDGetResponseRecorder, invalidUUIDRequest)
	if invalidUUIDGetResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on HandleGetUser with bad UUID, got: %d", invalidUUIDGetResponseRecorder.Code)
	}

	invalidUUIDDeleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleDeleteUser(invalidUUIDDeleteResponseRecorder, invalidUUIDRequest)
	if invalidUUIDDeleteResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on HandleDeleteUser with bad UUID, got: %d", invalidUUIDDeleteResponseRecorder.Code)
	}

	invalidUUIDLockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleLockUser(invalidUUIDLockResponseRecorder, invalidUUIDRequest)
	if invalidUUIDLockResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on HandleLockUser with bad UUID, got: %d", invalidUUIDLockResponseRecorder.Code)
	}

	invalidUUIDUnlockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleUnlockUser(invalidUUIDUnlockResponseRecorder, invalidUUIDRequest)
	if invalidUUIDUnlockResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on HandleUnlockUser with bad UUID, got: %d", invalidUUIDUnlockResponseRecorder.Code)
	}

	// 6. Test Nil DB on Single-User handlers
	singleUserRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users/"+testUserID, nil)
	singleUserRequest.SetPathValue("user_id", testUserID)

	nilDBGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleGetUser(nilDBGetResponseRecorder, singleUserRequest)
	if nilDBGetResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil db HandleGetUser, got: %d", nilDBGetResponseRecorder.Code)
	}

	nilDBDeleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleDeleteUser(nilDBDeleteResponseRecorder, singleUserRequest)
	if nilDBDeleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil db HandleDeleteUser, got: %d", nilDBDeleteResponseRecorder.Code)
	}

	nilDBUnlockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleUnlockUser(nilDBUnlockResponseRecorder, singleUserRequest)
	if nilDBUnlockResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil db HandleUnlockUser, got: %d", nilDBUnlockResponseRecorder.Code)
	}

	// 7. Test HandleCreateUser on nil db with valid payload -> 500
	validCreateUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users", bytes.NewReader([]byte(`{"email":"nildb@example.com","password":"Password123!"}`)))
	validCreateUserResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleCreateUser(validCreateUserResponseRecorder, validCreateUserRequest)
	if validCreateUserResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on HandleCreateUser with nil db, got: %d", validCreateUserResponseRecorder.Code)
	}

	// 8. Test HandleLockUser with explicit locked_until timestamp on nil db -> 500
	futureTime := time.Now().Add(24 * time.Hour)
	lockWithTimeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/_/auth/users/"+testUserID+"/lock", bytes.NewReader([]byte(`{"locked_until":"`+futureTime.Format(time.RFC3339)+`"}`)))
	lockWithTimeRequest.SetPathValue("user_id", testUserID)
	lockWithTimeResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleLockUser(lockWithTimeResponseRecorder, lockWithTimeRequest)
	if lockWithTimeResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on HandleLockUser with nil db and timestamp, got: %d", lockWithTimeResponseRecorder.Code)
	}
}
