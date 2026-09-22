package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"layr.sh/core"
)

func TestAuthControlPlaneHandlerSessionUnit(t *testing.T) {
	ctx := context.Background()
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	controlPlaneHandler := service.controlPlaneHandler
	testUserID := uuid.NewV7().String()

	// 1. Test Forbidden Scope on session handlers

	forbiddenRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users/"+testUserID+"/sessions", nil)
	forbiddenRequest.Header.Set("Authorization", "Bearer invalid-key")
	forbiddenRequest.SetPathValue("user_id", testUserID)

	forbiddenSessionsListResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListUserSessions(forbiddenSessionsListResponseRecorder, forbiddenRequest)
	if forbiddenSessionsListResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on handleListUserSessions, got: %d", forbiddenSessionsListResponseRecorder.Code)
	}

	forbiddenRevokeSessionsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleRevokeUserSessions(forbiddenRevokeSessionsResponseRecorder, forbiddenRequest)
	if forbiddenRevokeSessionsResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on handleRevokeUserSessions, got: %d", forbiddenRevokeSessionsResponseRecorder.Code)
	}

	// 2. Test Invalid UUIDs on session endpoints (with valid scope)
	invalidUUID := "not-a-valid-uuid"
	invalidUUIDRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users/"+invalidUUID+"/sessions", nil)
	invalidUUIDRequest.SetPathValue("user_id", invalidUUID)

	invalidUUIDSessionsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListUserSessions(invalidUUIDSessionsResponseRecorder, invalidUUIDRequest)
	if invalidUUIDSessionsResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on handleListUserSessions with bad UUID, got: %d", invalidUUIDSessionsResponseRecorder.Code)
	}

	invalidUUIDRevokeResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleRevokeUserSessions(invalidUUIDRevokeResponseRecorder, invalidUUIDRequest)
	if invalidUUIDRevokeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on handleRevokeUserSessions with bad UUID, got: %d", invalidUUIDRevokeResponseRecorder.Code)
	}

	// 3. Test Nil DB on Session handlers
	singleUserRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/users/"+testUserID+"/sessions", nil)
	singleUserRequest.SetPathValue("user_id", testUserID)

	nilDBSessionsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleListUserSessions(nilDBSessionsResponseRecorder, singleUserRequest)
	if nilDBSessionsResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil db handleListUserSessions, got: %d", nilDBSessionsResponseRecorder.Code)
	}

	nilDBRevokeResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleRevokeUserSessions(nilDBRevokeResponseRecorder, singleUserRequest)
	if nilDBRevokeResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil db handleRevokeUserSessions, got: %d", nilDBRevokeResponseRecorder.Code)
	}
}
