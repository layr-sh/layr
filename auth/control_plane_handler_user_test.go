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
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler
	controlPlaneHandler.hasher = password.NewHasher()

	// 1. handleCreateUser validation
	badJSONRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/auth/users", bytes.NewReader([]byte(`invalid-json`)))
	badJSONResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateUser(badJSONResponseRecorder, badJSONRequest)
	if badJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on invalid JSON, got: %d", badJSONResponseRecorder.Code)
	}

	emptyUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/auth/users", bytes.NewReader([]byte(`{}`)))
	emptyUserResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateUser(emptyUserResponseRecorder, emptyUserRequest)
	if emptyUserResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on empty user body, got: %d", emptyUserResponseRecorder.Code)
	}

	invalidPhoneUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/auth/users", bytes.NewReader([]byte(`{"phone":"invalid"}`)))
	invalidPhoneUserResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateUser(invalidPhoneUserResponseRecorder, invalidPhoneUserRequest)
	if invalidPhoneUserResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on invalid phone, got: %d", invalidPhoneUserResponseRecorder.Code)
	}

	// 2. handleListUsers with invalid pagination parameters
	paginationRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users?limit=bad&offset=bad", nil)
	paginationResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListUsers(paginationResponseRecorder, paginationRequest)
	if paginationResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error on nil db list, got: %d", paginationResponseRecorder.Code)
	}

	validPaginationRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users?limit=25&offset=10&role=authenticated&search=alice", nil)
	validPaginationResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListUsers(validPaginationResponseRecorder, validPaginationRequest)
	if validPaginationResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil db with valid pagination, got: %d", validPaginationResponseRecorder.Code)
	}

	// 3. handleLockUser bad JSON duration
	testUserID := uuid.NewV7().String()
	badLockRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/auth/users/"+testUserID+"/lock", bytes.NewReader([]byte(`invalid-json`)))
	badLockRequest.SetPathValue("user_id", testUserID)
	badLockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleLockUser(badLockResponseRecorder, badLockRequest)
	if badLockResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error on nil db lock, got: %d", badLockResponseRecorder.Code)
	}

	// 4. Test Forbidden Scope on all user handlers

	forbiddenRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users/"+testUserID, nil)
	forbiddenRequest.Header.Set("Authorization", "Bearer invalid-key")
	forbiddenRequest.SetPathValue("user_id", testUserID)

	forbiddenListResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListUsers(forbiddenListResponseRecorder, forbiddenRequest)
	if forbiddenListResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on handleListUsers, got: %d", forbiddenListResponseRecorder.Code)
	}

	forbiddenCreateResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateUser(forbiddenCreateResponseRecorder, forbiddenRequest)
	if forbiddenCreateResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on handleCreateUser, got: %d", forbiddenCreateResponseRecorder.Code)
	}

	forbiddenGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetUser(forbiddenGetResponseRecorder, forbiddenRequest)
	if forbiddenGetResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on handleGetUser, got: %d", forbiddenGetResponseRecorder.Code)
	}

	forbiddenDeleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteUser(forbiddenDeleteResponseRecorder, forbiddenRequest)
	if forbiddenDeleteResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on handleDeleteUser, got: %d", forbiddenDeleteResponseRecorder.Code)
	}

	forbiddenLockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleLockUser(forbiddenLockResponseRecorder, forbiddenRequest)
	if forbiddenLockResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on handleLockUser, got: %d", forbiddenLockResponseRecorder.Code)
	}

	forbiddenUnlockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUnlockUser(forbiddenUnlockResponseRecorder, forbiddenRequest)
	if forbiddenUnlockResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on handleUnlockUser, got: %d", forbiddenUnlockResponseRecorder.Code)
	}

	// 5. Test Invalid UUIDs on all single-user endpoints (with valid scope)
	invalidUUID := "not-a-valid-uuid"
	invalidUUIDRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users/"+invalidUUID, nil)
	invalidUUIDRequest.SetPathValue("user_id", invalidUUID)

	invalidUUIDGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetUser(invalidUUIDGetResponseRecorder, invalidUUIDRequest)
	if invalidUUIDGetResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on handleGetUser with bad UUID, got: %d", invalidUUIDGetResponseRecorder.Code)
	}

	invalidUUIDDeleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteUser(invalidUUIDDeleteResponseRecorder, invalidUUIDRequest)
	if invalidUUIDDeleteResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on handleDeleteUser with bad UUID, got: %d", invalidUUIDDeleteResponseRecorder.Code)
	}

	invalidUUIDLockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleLockUser(invalidUUIDLockResponseRecorder, invalidUUIDRequest)
	if invalidUUIDLockResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on handleLockUser with bad UUID, got: %d", invalidUUIDLockResponseRecorder.Code)
	}

	invalidUUIDUnlockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUnlockUser(invalidUUIDUnlockResponseRecorder, invalidUUIDRequest)
	if invalidUUIDUnlockResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on handleUnlockUser with bad UUID, got: %d", invalidUUIDUnlockResponseRecorder.Code)
	}

	// 6. Test Nil DB on Single-User handlers
	singleUserRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users/"+testUserID, nil)
	singleUserRequest.SetPathValue("user_id", testUserID)

	nilDBGetResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleGetUser(nilDBGetResponseRecorder, singleUserRequest)
	if nilDBGetResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil db handleGetUser, got: %d", nilDBGetResponseRecorder.Code)
	}

	nilDBDeleteResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleDeleteUser(nilDBDeleteResponseRecorder, singleUserRequest)
	if nilDBDeleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil db handleDeleteUser, got: %d", nilDBDeleteResponseRecorder.Code)
	}

	nilDBUnlockResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleUnlockUser(nilDBUnlockResponseRecorder, singleUserRequest)
	if nilDBUnlockResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil db handleUnlockUser, got: %d", nilDBUnlockResponseRecorder.Code)
	}

	// 7. Test handleCreateUser on nil db with valid payload -> 500
	validCreateUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/auth/users", bytes.NewReader([]byte(`{"email":"nildb@example.com","password":"Password123!"}`)))
	validCreateUserResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateUser(validCreateUserResponseRecorder, validCreateUserRequest)
	if validCreateUserResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on handleCreateUser with nil db, got: %d", validCreateUserResponseRecorder.Code)
	}

	// 8. Test handleLockUser with explicit locked_until timestamp on nil db -> 500
	futureTime := time.Now().Add(24 * time.Hour)
	lockWithTimeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/auth/users/"+testUserID+"/lock", bytes.NewReader([]byte(`{"locked_until":"`+futureTime.Format(time.RFC3339)+`"}`)))
	lockWithTimeRequest.SetPathValue("user_id", testUserID)
	lockWithTimeResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleLockUser(lockWithTimeResponseRecorder, lockWithTimeRequest)
	if lockWithTimeResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on handleLockUser with nil db and timestamp, got: %d", lockWithTimeResponseRecorder.Code)
	}
}
