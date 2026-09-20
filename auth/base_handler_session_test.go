package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"layr.sh/core"
)

func TestAuthSessionHandlerUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	baseHandler := NewBaseHandler(nil, configManager, cryptoKeyManager)
	ctx := context.Background()

	validAccessToken, err := baseHandler.jwtSigner.GenerateAccessToken(core.JWTClaims{
		Subject: "user-123",
		Email:   "user@example.com",
		Role:    "authenticated",
	}, 900)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	authedCtx := core.WithAuthContext(ctx, core.AuthContext{
		UserID: "user-123",
		JWT: core.JWTClaims{
			Subject: "user-123",
			Email:   "user@example.com",
			Role:    "authenticated",
		},
	})

	// 1. handleListSessions on nil pool -> 500
	cookieRequest := httptest.NewRequestWithContext(authedCtx, http.MethodGet, "/v1/auth/user/sessions", nil)
	cookieRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	cookieRequest.AddCookie(&http.Cookie{Name: core.SessionCookieNameSecure, Value: "cookie_refresh_token"})
	listRequestResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListSessions(listRequestResponseRecorder, cookieRequest)
	if listRequestResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool list sessions, got: %d", listRequestResponseRecorder.Code)
	}

	// 2. handleListSessions unauthorized -> 401
	unauthorizedListRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/auth/user/sessions", nil)
	unauthorizedListRequestResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListSessions(unauthorizedListRequestResponseRecorder, unauthorizedListRequest)
	if unauthorizedListRequestResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthorized list sessions, got: %d", unauthorizedListRequestResponseRecorder.Code)
	}

	// 3. handleRevokeSession on nil pool -> 500
	revokeRequest := httptest.NewRequestWithContext(authedCtx, http.MethodDelete, "/v1/auth/user/sessions/01918a3f-1234-7000-8000-000000000001", nil)
	revokeRequestResponseRecorder := httptest.NewRecorder()
	revokeRequest.SetPathValue("session_id", "01918a3f-1234-7000-8000-000000000001")
	revokeRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	baseHandler.handleRevokeSession(revokeRequestResponseRecorder, revokeRequest)
	if revokeRequestResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool revoke session, got: %d", revokeRequestResponseRecorder.Code)
	}

	// 4. handleRevokeSession unauthorized -> 401
	unauthRevokeRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/auth/user/sessions/01918a3f-1234-7000-8000-000000000001", nil)
	unauthRevokeRequestResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRevokeSession(unauthRevokeRequestResponseRecorder, unauthRevokeRequest)
	if unauthRevokeRequestResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauth revoke session, got: %d", unauthRevokeRequestResponseRecorder.Code)
	}

	// 5. handleRevokeSession empty or bad path -> 400
	badPathRequest := httptest.NewRequestWithContext(authedCtx, http.MethodDelete, "/v1/auth/user/sessions/", nil)
	badPathRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	badPathRequestResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRevokeSession(badPathRequestResponseRecorder, badPathRequest)
	if badPathRequestResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty session id, got: %d", badPathRequestResponseRecorder.Code)
	}

	slashPathRequest := httptest.NewRequestWithContext(authedCtx, http.MethodDelete, "/v1/auth/user/sessions/abc/def", nil)
	slashPathRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	slashPathRequestResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRevokeSession(slashPathRequestResponseRecorder, slashPathRequest)
	if slashPathRequestResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on nested path, got: %d", slashPathRequestResponseRecorder.Code)
	}

	invalidUUIDRequest := httptest.NewRequestWithContext(authedCtx, http.MethodDelete, "/v1/auth/user/sessions/not-a-valid-uuid", nil)
	invalidUUIDRequest.SetPathValue("session_id", "not-a-valid-uuid")
	invalidUUIDRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	invalidUUIDRequestResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRevokeSession(invalidUUIDRequestResponseRecorder, invalidUUIDRequest)
	if invalidUUIDRequestResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid uuid, got: %d", invalidUUIDRequestResponseRecorder.Code)
	}

	// 6. handleRevokeOtherSessions on nil pool -> 500
	revokeOthersRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPost, "/v1/auth/user/sessions/revoke-others", nil)
	revokeOthersRequestResponseRecorder := httptest.NewRecorder()
	revokeOthersRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	baseHandler.handleRevokeOtherSessions(revokeOthersRequestResponseRecorder, revokeOthersRequest)
	if revokeOthersRequestResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool revoke others, got: %d", revokeOthersRequestResponseRecorder.Code)
	}

	// 7. handleRevokeOtherSessions unauthorized -> 401
	unauthRevokeOthersRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/user/sessions/revoke-others", nil)
	unauthRevokeOthersRequestResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRevokeOtherSessions(unauthRevokeOthersRequestResponseRecorder, unauthRevokeOthersRequest)
	if unauthRevokeOthersRequestResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthorized revoke others, got: %d", unauthRevokeOthersRequestResponseRecorder.Code)
	}

	// 8. Token Refresh validation errors
	badRefreshJSONRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/token/refresh", bytes.NewReader([]byte(`bad-json`)))
	badRefreshJSONResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRefreshToken(badRefreshJSONResponseRecorder, badRefreshJSONRequest)
	if badRefreshJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad refresh JSON, got: %d", badRefreshJSONResponseRecorder.Code)
	}

	missingTokenRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/token/refresh", strings.NewReader(`{}`))
	missingTokenRefreshResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRefreshToken(missingTokenRefreshResponseRecorder, missingTokenRefreshRequest)
	if missingTokenRefreshResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing refresh token, got: %d", missingTokenRefreshResponseRecorder.Code)
	}

	validRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/token/refresh", strings.NewReader(`{"refresh_token":"valid-refresh-token"}`))
	validRefreshResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRefreshToken(validRefreshResponseRecorder, validRefreshRequest)
	if validRefreshResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on refresh nil db pool, got: %d", validRefreshResponseRecorder.Code)
	}

	// Fast-path session cache hit with locked user -> 423
	sessionKVStore := newInMemoryKVStore()
	baseHandler.SetKVStore(sessionKVStore)
	fastPathConfig := DefaultConfig()
	fastPathConfig.Cache.FastPathSessionsEnabled = true
	configManager.Set(fastPathConfig)

	futureLockUntil := time.Now().UTC().Add(time.Hour)
	lockedCachedSession := CachedSession{
		User: User{
			ID:          "user-locked-123",
			Role:        "authenticated",
			LockedUntil: &futureLockUntil,
		},
	}
	lockedCachedBytes, _ := json.Marshal(lockedCachedSession)
	lockedRefreshToken := "cached-locked-token"
	lockedTokenHash := baseHandler.jwtSigner.HashRefreshToken(lockedRefreshToken)
	_ = sessionKVStore.Set(ctx, "auth:session:"+lockedTokenHash, string(lockedCachedBytes), time.Hour)

	lockedCacheRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/token/refresh", strings.NewReader(`{"refresh_token":"cached-locked-token"}`))
	lockedCacheRefreshResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRefreshToken(lockedCacheRefreshResponseRecorder, lockedCacheRefreshRequest)
	if lockedCacheRefreshResponseRecorder.Code != http.StatusLocked {
		t.Fatalf("expected 423 on locked cached session refresh, got: %d", lockedCacheRefreshResponseRecorder.Code)
	}

	// 9. SignOut
	signOutRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-out", nil)
	signOutResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignOut(signOutResponseRecorder, signOutRequest)
	if signOutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on sign out, got: %d", signOutResponseRecorder.Code)
	}

	signOutWithBodyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-out", strings.NewReader(`{"refresh_token":"dummy-refresh-token"}`))
	signOutWithBodyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignOut(signOutWithBodyResponseRecorder, signOutWithBodyRequest)
	if signOutWithBodyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on sign out with body, got: %d", signOutWithBodyResponseRecorder.Code)
	}
}
