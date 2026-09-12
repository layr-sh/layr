package auth

import (
	"context"
	"layr.sh/auth/jwt"
	"layr.sh/core"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthHandlerExportUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	baseHandler := NewHandler(nil, configManager, cryptoKeyManager)
	ctx := context.Background()

	// 1. Missing user_id path parameter -> 400
	missingPathRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/users//export", nil)
	missingPathResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserExport(missingPathResponseRecorder, missingPathRequest)
	if missingPathResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing user_id path parameter, got: %d", missingPathResponseRecorder.Code)
	}

	// 2. Missing bearer token -> 401
	missingBearerRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/users/user-123/export", nil)
	missingBearerRequest.SetPathValue("user_id", "user-123")
	missingBearerResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserExport(missingBearerResponseRecorder, missingBearerRequest)
	if missingBearerResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing bearer token, got: %d", missingBearerResponseRecorder.Code)
	}

	testUserUUID := "018f2234-5678-789a-bcde-f0123456789a"
	validToken, err := baseHandler.signer.GenerateAccessToken(jwt.Claims{
		Subject: testUserUUID,
		Email:   "test@example.com",
		Role:    "authenticated",
	}, 900)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}
	bearerHeader := "Bearer " + validToken

	// 3. Mismatched subject -> 401
	mismatchedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/users/other-uuid/export", nil)
	mismatchedRequest.SetPathValue("user_id", "other-uuid")
	mismatchedRequest.Header.Set("Authorization", bearerHeader)
	mismatchedResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserExport(mismatchedResponseRecorder, mismatchedRequest)
	if mismatchedResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on mismatched subject export, got: %d", mismatchedResponseRecorder.Code)
	}

	// 4. Matched subject on nil db pool -> 500
	matchedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/users/"+testUserUUID+"/export", nil)
	matchedRequest.SetPathValue("user_id", testUserUUID)
	matchedRequest.Header.Set("Authorization", bearerHeader)
	matchedResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserExport(matchedResponseRecorder, matchedRequest)
	if matchedResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on matched export with nil db pool, got: %d", matchedResponseRecorder.Code)
	}
}
