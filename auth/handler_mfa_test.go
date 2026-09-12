package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/auth/jwt"
	"layr.sh/core"
)

func TestAuthMFAHandlerUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	handler := NewHandler(nil, configManager, cryptoKeyManager)

	// 1. MFA disabled -> 403
	disabledConfig := DefaultConfig()
	disabledConfig.MFA.Enabled = false
	configManager.Set(disabledConfig)

	mfaSetupDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/setup", strings.NewReader(`{}`))
	mfaSetupDisabledResponseRecorder := httptest.NewRecorder()
	handler.handleMFASetup(mfaSetupDisabledResponseRecorder, mfaSetupDisabledRequest)
	if mfaSetupDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on MFA setup when MFA disabled, got: %d", mfaSetupDisabledResponseRecorder.Code)
	}

	mfaVerifyDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/verify", strings.NewReader(`{}`))
	mfaVerifyDisabledResponseRecorder := httptest.NewRecorder()
	handler.handleMFAVerify(mfaVerifyDisabledResponseRecorder, mfaVerifyDisabledRequest)
	if mfaVerifyDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on MFA verify when MFA disabled, got: %d", mfaVerifyDisabledResponseRecorder.Code)
	}

	// Enable MFA
	enabledConfig := DefaultConfig()
	enabledConfig.MFA.Enabled = true
	configManager.Set(enabledConfig)

	// 2. Missing bearer -> 401
	missingBearerMFASetupRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/setup", nil)
	missingBearerMFASetupResponseRecorder := httptest.NewRecorder()
	handler.handleMFASetup(missingBearerMFASetupResponseRecorder, missingBearerMFASetupRequest)
	if missingBearerMFASetupResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on MFA setup without bearer, got: %d", missingBearerMFASetupResponseRecorder.Code)
	}

	missingBearerMFAVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/verify", nil)
	missingBearerMFAVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleMFAVerify(missingBearerMFAVerifyResponseRecorder, missingBearerMFAVerifyRequest)
	if missingBearerMFAVerifyResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on MFA verify without bearer, got: %d", missingBearerMFAVerifyResponseRecorder.Code)
	}

	// 3. Invalid token -> 401
	invalidTokenMFASetupRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/setup", nil)
	invalidTokenMFASetupRequest.Header.Set("Authorization", "Bearer invalid-token")
	invalidTokenMFASetupResponseRecorder := httptest.NewRecorder()
	handler.handleMFASetup(invalidTokenMFASetupResponseRecorder, invalidTokenMFASetupRequest)
	if invalidTokenMFASetupResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid token MFA setup, got: %d", invalidTokenMFASetupResponseRecorder.Code)
	}

	invalidTokenMFAVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/verify", strings.NewReader(`{"code":"123456"}`))
	invalidTokenMFAVerifyRequest.Header.Set("Authorization", "Bearer invalid-token")
	invalidTokenMFAVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleMFAVerify(invalidTokenMFAVerifyResponseRecorder, invalidTokenMFAVerifyRequest)
	if invalidTokenMFAVerifyResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid token MFA verify, got: %d", invalidTokenMFAVerifyResponseRecorder.Code)
	}

	// 4. Valid token on nil pool -> 500
	testUserUUID := "018f2234-5678-789a-bcde-f0123456789a"
	validToken, err := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject: testUserUUID,
		Email:   "test@example.com",
		Role:    "authenticated",
	}, 900)
	if err != nil {
		t.Fatalf("failed to sign access token: %v", err)
	}
	bearerHeader := "Bearer " + validToken

	validTokenMFASetupRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/setup", nil)
	validTokenMFASetupRequest.Header.Set("Authorization", bearerHeader)
	validTokenMFASetupResponseRecorder := httptest.NewRecorder()
	handler.handleMFASetup(validTokenMFASetupResponseRecorder, validTokenMFASetupRequest)
	if validTokenMFASetupResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on valid token MFA setup with nil pool, got: %d", validTokenMFASetupResponseRecorder.Code)
	}

	// 5. Valid token and missing code -> 400
	missingCodeMFAVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/verify", strings.NewReader(`{}`))
	missingCodeMFAVerifyRequest.Header.Set("Authorization", bearerHeader)
	missingCodeMFAVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleMFAVerify(missingCodeMFAVerifyResponseRecorder, missingCodeMFAVerifyRequest)
	if missingCodeMFAVerifyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing code MFA verify, got: %d", missingCodeMFAVerifyResponseRecorder.Code)
	}

	// 6. Valid token and code on nil pool -> 500
	validMFAVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/verify", strings.NewReader(`{"code":"123456"}`))
	validMFAVerifyRequest.Header.Set("Authorization", bearerHeader)
	validMFAVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleMFAVerify(validMFAVerifyResponseRecorder, validMFAVerifyRequest)
	if validMFAVerifyResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on valid token MFA verify with nil pool, got: %d", validMFAVerifyResponseRecorder.Code)
	}

	// 7. Custom Issuer
	customIssuerConfig := DefaultConfig()
	customIssuerConfig.MFA.Enabled = true
	customIssuerConfig.MFA.Issuer = "MyCustomIssuer"
	configManager.Set(customIssuerConfig)

	customIssuerMFARequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/setup", nil)
	customIssuerMFARequest.Header.Set("Authorization", bearerHeader)
	customIssuerMFAResponseRecorder := httptest.NewRecorder()
	handler.handleMFASetup(customIssuerMFAResponseRecorder, customIssuerMFARequest)
	if customIssuerMFAResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on custom issuer MFA setup nil pool, got: %d", customIssuerMFAResponseRecorder.Code)
	}

	// 8. Empty MFA Issuer defaults to "Layr"
	emptyIssuerConfig := DefaultConfig()
	emptyIssuerConfig.MFA.Issuer = ""
	emptyIssuerConfigManager := NewConfigManager(nil, cryptoKeyManager)
	emptyIssuerConfigManager.Set(emptyIssuerConfig)
	emptyIssuerHandler := NewHandler(nil, emptyIssuerConfigManager, cryptoKeyManager)
	if emptyIssuerHandler.GetTOTPManager() == nil {
		t.Fatal("expected non-nil TOTP manager")
	}
}
