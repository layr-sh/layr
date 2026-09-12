package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"layr.sh/auth/jwt"
	"layr.sh/core"
)

func TestAuthSessionHandlerUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	handler := NewHandler(nil, configManager, cryptoKeyManager)

	validAccessToken, err := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject: "user-123",
		Email:   "user@example.com",
		Role:    "authenticated",
	}, 900)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	// 1. handleListSessions on nil pool -> 500
	cookieRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user/sessions", nil)
	cookieRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	cookieRequest.AddCookie(&http.Cookie{Name: AuthSessionCookieName, Value: "cookie_refresh_token"})
	listRequestResponseRecorder := httptest.NewRecorder()
	handler.handleListSessions(listRequestResponseRecorder, cookieRequest)
	if listRequestResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool list sessions, got: %d", listRequestResponseRecorder.Code)
	}

	// 2. handleListSessions unauthorized -> 401
	unauthorizedListRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user/sessions", nil)
	unauthorizedListRequestResponseRecorder := httptest.NewRecorder()
	handler.handleListSessions(unauthorizedListRequestResponseRecorder, unauthorizedListRequest)
	if unauthorizedListRequestResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthorized list sessions, got: %d", unauthorizedListRequestResponseRecorder.Code)
	}

	// 3. handleRevokeSession on nil pool -> 500
	revokeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/user/sessions/01918a3f-1234-7000-8000-000000000001", nil)
	revokeRequestResponseRecorder := httptest.NewRecorder()
	revokeRequest.SetPathValue("session_id", "01918a3f-1234-7000-8000-000000000001")
	revokeRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	handler.handleRevokeSession(revokeRequestResponseRecorder, revokeRequest)
	if revokeRequestResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool revoke session, got: %d", revokeRequestResponseRecorder.Code)
	}

	// 4. handleRevokeSession unauthorized -> 401
	unauthRevokeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/user/sessions/01918a3f-1234-7000-8000-000000000001", nil)
	unauthRevokeRequestResponseRecorder := httptest.NewRecorder()
	handler.handleRevokeSession(unauthRevokeRequestResponseRecorder, unauthRevokeRequest)
	if unauthRevokeRequestResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauth revoke session, got: %d", unauthRevokeRequestResponseRecorder.Code)
	}

	// 5. handleRevokeSession empty or bad path -> 400
	badPathRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/user/sessions/", nil)
	badPathRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	badPathRequestResponseRecorder := httptest.NewRecorder()
	handler.handleRevokeSession(badPathRequestResponseRecorder, badPathRequest)
	if badPathRequestResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty session id, got: %d", badPathRequestResponseRecorder.Code)
	}

	slashPathRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/user/sessions/abc/def", nil)
	slashPathRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	slashPathRequestResponseRecorder := httptest.NewRecorder()
	handler.handleRevokeSession(slashPathRequestResponseRecorder, slashPathRequest)
	if slashPathRequestResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on nested path, got: %d", slashPathRequestResponseRecorder.Code)
	}

	invalidUUIDRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/user/sessions/not-a-valid-uuid", nil)
	invalidUUIDRequest.SetPathValue("session_id", "not-a-valid-uuid")
	invalidUUIDRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	invalidUUIDRequestResponseRecorder := httptest.NewRecorder()
	handler.handleRevokeSession(invalidUUIDRequestResponseRecorder, invalidUUIDRequest)
	if invalidUUIDRequestResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid uuid, got: %d", invalidUUIDRequestResponseRecorder.Code)
	}

	// 6. handleRevokeOtherSessions on nil pool -> 500
	revokeOthersRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/sessions/revoke-others", nil)
	revokeOthersRequestResponseRecorder := httptest.NewRecorder()
	revokeOthersRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	handler.handleRevokeOtherSessions(revokeOthersRequestResponseRecorder, revokeOthersRequest)
	if revokeOthersRequestResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool revoke others, got: %d", revokeOthersRequestResponseRecorder.Code)
	}

	// 7. handleRevokeOtherSessions unauthorized -> 401
	unauthRevokeOthersRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/sessions/revoke-others", nil)
	unauthRevokeOthersRequestResponseRecorder := httptest.NewRecorder()
	handler.handleRevokeOtherSessions(unauthRevokeOthersRequestResponseRecorder, unauthRevokeOthersRequest)
	if unauthRevokeOthersRequestResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthorized revoke others, got: %d", unauthRevokeOthersRequestResponseRecorder.Code)
	}
}
