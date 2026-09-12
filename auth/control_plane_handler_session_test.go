package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/go-fuego/fuego"
	"layr.sh/core"
)

func TestAuthControlPlaneHandlerSessionUnit(t *testing.T) {
	ctx := context.Background()
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	controlPlaneHandler := NewControlPlaneHandler(nil, configManager)
	testUserID := uuid.NewV7().String()

	// 1. Test Forbidden Scope on session handlers
	serviceAccountManager := core.NewServiceAccountManager(nil)
	controlPlaneHandler.SetServiceAccountManager(serviceAccountManager)

	forbiddenRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users/"+testUserID+"/sessions", nil)
	forbiddenRequest.Header.Set("Authorization", "Bearer invalid-key")
	forbiddenRequest.SetPathValue("user_id", testUserID)

	forbiddenSessionsListResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleListUserSessions(forbiddenSessionsListResponseRecorder, forbiddenRequest)
	if forbiddenSessionsListResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on HandleListUserSessions, got: %d", forbiddenSessionsListResponseRecorder.Code)
	}

	forbiddenRevokeSessionsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleRevokeUserSessions(forbiddenRevokeSessionsResponseRecorder, forbiddenRequest)
	if forbiddenRevokeSessionsResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on HandleRevokeUserSessions, got: %d", forbiddenRevokeSessionsResponseRecorder.Code)
	}

	// 2. Test Invalid UUIDs on session endpoints (with valid scope)
	controlPlaneHandler.SetServiceAccountManager(nil)
	invalidUUID := "not-a-valid-uuid"
	invalidUUIDRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users/"+invalidUUID+"/sessions", nil)
	invalidUUIDRequest.SetPathValue("user_id", invalidUUID)

	invalidUUIDSessionsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleListUserSessions(invalidUUIDSessionsResponseRecorder, invalidUUIDRequest)
	if invalidUUIDSessionsResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on HandleListUserSessions with bad UUID, got: %d", invalidUUIDSessionsResponseRecorder.Code)
	}

	invalidUUIDRevokeResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleRevokeUserSessions(invalidUUIDRevokeResponseRecorder, invalidUUIDRequest)
	if invalidUUIDRevokeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on HandleRevokeUserSessions with bad UUID, got: %d", invalidUUIDRevokeResponseRecorder.Code)
	}

	// 3. Test Nil DB on Session handlers
	singleUserRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/_/auth/users/"+testUserID+"/sessions", nil)
	singleUserRequest.SetPathValue("user_id", testUserID)

	nilDBSessionsResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleListUserSessions(nilDBSessionsResponseRecorder, singleUserRequest)
	if nilDBSessionsResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil db HandleListUserSessions, got: %d", nilDBSessionsResponseRecorder.Code)
	}

	nilDBRevokeResponseRecorder := httptest.NewRecorder()
	controlPlaneHandler.HandleRevokeUserSessions(nilDBRevokeResponseRecorder, singleUserRequest)
	if nilDBRevokeResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil db HandleRevokeUserSessions, got: %d", nilDBRevokeResponseRecorder.Code)
	}

	// 4. Test RegisterSessionRoutes wiring
	fuegoEngine := fuego.NewServer()
	router := core.NewRouter(fuegoEngine)
	controlPlaneHandler.RegisterSessionRoutes(router)
}
