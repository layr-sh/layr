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
	baseHandler := NewHandler(nil, configManager, cryptoKeyManager)

	// 1. MFA disabled -> 403
	disabledConfig := DefaultConfig()
	disabledConfig.MFA.Enabled = false
	configManager.Set(disabledConfig)

	mfaSetupDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/setup", strings.NewReader(`{}`))
	mfaSetupDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFASetup(mfaSetupDisabledResponseRecorder, mfaSetupDisabledRequest)
	if mfaSetupDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on MFA setup when MFA disabled, got: %d", mfaSetupDisabledResponseRecorder.Code)
	}

	mfaVerifyDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/verify", strings.NewReader(`{}`))
	mfaVerifyDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(mfaVerifyDisabledResponseRecorder, mfaVerifyDisabledRequest)
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
	baseHandler.handleMFASetup(missingBearerMFASetupResponseRecorder, missingBearerMFASetupRequest)
	if missingBearerMFASetupResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on MFA setup without bearer, got: %d", missingBearerMFASetupResponseRecorder.Code)
	}

	missingBearerMFAVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/verify", nil)
	missingBearerMFAVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(missingBearerMFAVerifyResponseRecorder, missingBearerMFAVerifyRequest)
	if missingBearerMFAVerifyResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on MFA verify without bearer, got: %d", missingBearerMFAVerifyResponseRecorder.Code)
	}

	// 3. Invalid token -> 401
	invalidTokenMFASetupRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/setup", nil)
	invalidTokenMFASetupRequest.Header.Set("Authorization", "Bearer invalid-token")
	invalidTokenMFASetupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFASetup(invalidTokenMFASetupResponseRecorder, invalidTokenMFASetupRequest)
	if invalidTokenMFASetupResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid token MFA setup, got: %d", invalidTokenMFASetupResponseRecorder.Code)
	}

	invalidTokenMFAVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/verify", strings.NewReader(`{"code":"123456"}`))
	invalidTokenMFAVerifyRequest.Header.Set("Authorization", "Bearer invalid-token")
	invalidTokenMFAVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(invalidTokenMFAVerifyResponseRecorder, invalidTokenMFAVerifyRequest)
	if invalidTokenMFAVerifyResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid token MFA verify, got: %d", invalidTokenMFAVerifyResponseRecorder.Code)
	}

	// 4. Valid token on nil pool -> 500
	testUserUUID := "018f2234-5678-789a-bcde-f0123456789a"
	validToken, err := baseHandler.signer.GenerateAccessToken(jwt.Claims{
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
	baseHandler.handleMFASetup(validTokenMFASetupResponseRecorder, validTokenMFASetupRequest)
	if validTokenMFASetupResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on valid token MFA setup with nil pool, got: %d", validTokenMFASetupResponseRecorder.Code)
	}

	// 5. Valid token and missing code -> 400
	missingCodeMFAVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/verify", strings.NewReader(`{}`))
	missingCodeMFAVerifyRequest.Header.Set("Authorization", bearerHeader)
	missingCodeMFAVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(missingCodeMFAVerifyResponseRecorder, missingCodeMFAVerifyRequest)
	if missingCodeMFAVerifyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing code MFA verify, got: %d", missingCodeMFAVerifyResponseRecorder.Code)
	}

	// 6. Valid token and code on nil pool -> 500
	validMFAVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/verify", strings.NewReader(`{"code":"123456"}`))
	validMFAVerifyRequest.Header.Set("Authorization", bearerHeader)
	validMFAVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(validMFAVerifyResponseRecorder, validMFAVerifyRequest)
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
	baseHandler.handleMFASetup(customIssuerMFAResponseRecorder, customIssuerMFARequest)
	if customIssuerMFAResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on custom issuer MFA setup nil pool, got: %d", customIssuerMFAResponseRecorder.Code)
	}

	// 8. Empty MFA Issuer defaults to "Layr"
	emptyIssuerConfig := DefaultConfig()
	emptyIssuerConfig.MFA.Issuer = ""
	emptyIssuerConfigManager := NewConfigManager(nil, cryptoKeyManager)
	emptyIssuerConfigManager.Set(emptyIssuerConfig)
	emptyIssuerBaseHandler := NewHandler(nil, emptyIssuerConfigManager, cryptoKeyManager)
	if emptyIssuerBaseHandler.GetTOTPManager() == nil {
		t.Fatal("expected non-nil TOTP manager")
	}

	// 9. handleMFADisable unit checks
	disabledConfig.MFA.Enabled = false
	configManager.Set(disabledConfig)
	disableMFAForbiddenRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/mfa", nil)
	disableMFAForbiddenResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFADisable(disableMFAForbiddenResponseRecorder, disableMFAForbiddenRequest)
	if disableMFAForbiddenResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on MFA disable when disabled in config, got: %d", disableMFAForbiddenResponseRecorder.Code)
	}
	configManager.Set(enabledConfig)

	missingBearerDisableRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/mfa", nil)
	missingBearerDisableResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFADisable(missingBearerDisableResponseRecorder, missingBearerDisableRequest)
	if missingBearerDisableResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on MFA disable without bearer, got: %d", missingBearerDisableResponseRecorder.Code)
	}

	invalidTokenDisableRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/mfa", nil)
	invalidTokenDisableRequest.Header.Set("Authorization", "Bearer invalid-token")
	invalidTokenDisableResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFADisable(invalidTokenDisableResponseRecorder, invalidTokenDisableRequest)
	if invalidTokenDisableResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid token MFA disable, got: %d", invalidTokenDisableResponseRecorder.Code)
	}

	validTokenDisableRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/mfa", nil)
	validTokenDisableRequest.Header.Set("Authorization", bearerHeader)
	validTokenDisableResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFADisable(validTokenDisableResponseRecorder, validTokenDisableRequest)
	if validTokenDisableResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on valid token MFA disable with nil pool, got: %d", validTokenDisableResponseRecorder.Code)
	}
}

func TestAuthMFAChallengeHandlerUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	baseHandler := NewHandler(nil, configManager, cryptoKeyManager)

	// 1. MFA disabled -> 403
	disabledConfig := DefaultConfig()
	disabledConfig.MFA.Enabled = false
	configManager.Set(disabledConfig)

	mfaChallengeDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/challenge", strings.NewReader(`{}`))
	mfaChallengeDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(mfaChallengeDisabledResponseRecorder, mfaChallengeDisabledRequest)
	if mfaChallengeDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on MFA challenge when MFA disabled, got: %d", mfaChallengeDisabledResponseRecorder.Code)
	}

	// Enable MFA
	enabledConfig := DefaultConfig()
	enabledConfig.MFA.Enabled = true
	configManager.Set(enabledConfig)

	// 2. Bad JSON body -> 400
	badJSONRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/challenge", strings.NewReader(`{invalid`))
	badJSONResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(badJSONResponseRecorder, badJSONRequest)
	if badJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON in MFA challenge, got: %d", badJSONResponseRecorder.Code)
	}

	// 3. Missing ticket or code -> 400
	missingFieldsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/challenge", strings.NewReader(`{"mfa_ticket":"","code":""}`))
	missingFieldsResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(missingFieldsResponseRecorder, missingFieldsRequest)
	if missingFieldsResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing ticket/code in MFA challenge, got: %d", missingFieldsResponseRecorder.Code)
	}

	// 4. KV store nil -> 500
	validPayloadRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/challenge", strings.NewReader(`{"mfa_ticket":"mfa_tk_test","code":"123456"}`))
	validPayloadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(validPayloadResponseRecorder, validPayloadRequest)
	if validPayloadResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil KV store in MFA challenge, got: %d", validPayloadResponseRecorder.Code)
	}

	// 5. KV store with ticket on nil DB pool -> 500
	testKVStore := newInMemoryKVStore()
	_ = testKVStore.Set(context.Background(), "auth:mfa_ticket:mfa_tk_test", "test-user-id", 0)
	baseHandler.SetKVStore(testKVStore)

	nilDBRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/mfa/challenge", strings.NewReader(`{"mfa_ticket":"mfa_tk_test","code":"123456"}`))
	nilDBResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(nilDBResponseRecorder, nilDBRequest)
	if nilDBResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil DB pool in MFA challenge, got: %d", nilDBResponseRecorder.Code)
	}
}
