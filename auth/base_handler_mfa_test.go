package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/core"
)

func TestAuthMFAHandlerUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := service.baseHandler
	configManager := service.configManager

	// 1. MFA disabled -> 403
	disabledConfig := DefaultConfig()
	disabledConfig.MFA.Enabled = false
	configManager.SetMemoryConfig(disabledConfig)

	mfaSetupDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/mfa/setup", strings.NewReader(`{}`))
	mfaSetupDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSetupMFA(mfaSetupDisabledResponseRecorder, mfaSetupDisabledRequest)
	if mfaSetupDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on MFA setup when MFA disabled, got: %d", mfaSetupDisabledResponseRecorder.Code)
	}

	mfaVerifyDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/mfa/verify", strings.NewReader(`{}`))
	mfaVerifyDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyMFA(mfaVerifyDisabledResponseRecorder, mfaVerifyDisabledRequest)
	if mfaVerifyDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on MFA verify when MFA disabled, got: %d", mfaVerifyDisabledResponseRecorder.Code)
	}

	// Enable MFA
	enabledConfig := DefaultConfig()
	enabledConfig.MFA.Enabled = true
	configManager.SetMemoryConfig(enabledConfig)

	// 2. Missing bearer -> 401
	missingBearerMFASetupRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/mfa/setup", nil)
	missingBearerMFASetupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSetupMFA(missingBearerMFASetupResponseRecorder, missingBearerMFASetupRequest)
	if missingBearerMFASetupResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on MFA setup without bearer, got: %d", missingBearerMFASetupResponseRecorder.Code)
	}

	missingBearerMFAVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/mfa/verify", nil)
	missingBearerMFAVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyMFA(missingBearerMFAVerifyResponseRecorder, missingBearerMFAVerifyRequest)
	if missingBearerMFAVerifyResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on MFA verify without bearer, got: %d", missingBearerMFAVerifyResponseRecorder.Code)
	}

	// 3. Invalid token -> 401
	invalidTokenMFASetupRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/mfa/setup", nil)
	invalidTokenMFASetupRequest.Header.Set("Authorization", "Bearer invalid-token")
	invalidTokenMFASetupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSetupMFA(invalidTokenMFASetupResponseRecorder, invalidTokenMFASetupRequest)
	if invalidTokenMFASetupResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid token MFA setup, got: %d", invalidTokenMFASetupResponseRecorder.Code)
	}

	invalidTokenMFAVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/mfa/verify", strings.NewReader(`{"code":"123456"}`))
	invalidTokenMFAVerifyRequest.Header.Set("Authorization", "Bearer invalid-token")
	invalidTokenMFAVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyMFA(invalidTokenMFAVerifyResponseRecorder, invalidTokenMFAVerifyRequest)
	if invalidTokenMFAVerifyResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid token MFA verify, got: %d", invalidTokenMFAVerifyResponseRecorder.Code)
	}

	// 4. Valid token on nil pool -> 500
	testUserUUID := "018f2234-5678-789a-bcde-f0123456789a"
	validToken := kernel.JWTSigner().GenerateAccessToken(core.JWTClaims{
		Subject: testUserUUID,
		Email:   "test@example.com",
		Role:    "authenticated",
	}, 900)
	bearerHeader := "Bearer " + validToken
	authContext := core.AuthContext{UserID: testUserUUID, JWT: core.JWTClaims{Subject: testUserUUID, Role: "authenticated"}}

	validTokenMFASetupRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), authContext), http.MethodPost, "/v1/auth/mfa/setup", nil)
	validTokenMFASetupRequest.Header.Set("Authorization", bearerHeader)
	validTokenMFASetupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSetupMFA(validTokenMFASetupResponseRecorder, validTokenMFASetupRequest)
	if validTokenMFASetupResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on valid token MFA setup with non-existent user, got: %d", validTokenMFASetupResponseRecorder.Code)
	}

	// 5. Valid token and missing code -> 400
	missingCodeMFAVerifyRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), authContext), http.MethodPost, "/v1/auth/mfa/verify", strings.NewReader(`{}`))
	missingCodeMFAVerifyRequest.Header.Set("Authorization", bearerHeader)
	missingCodeMFAVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyMFA(missingCodeMFAVerifyResponseRecorder, missingCodeMFAVerifyRequest)
	if missingCodeMFAVerifyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing code MFA verify, got: %d", missingCodeMFAVerifyResponseRecorder.Code)
	}

	// 6. Valid token and code on non-existent user -> 404
	validMFAVerifyRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), authContext), http.MethodPost, "/v1/auth/mfa/verify", strings.NewReader(`{"code":"123456"}`))
	validMFAVerifyRequest.Header.Set("Authorization", bearerHeader)
	validMFAVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyMFA(validMFAVerifyResponseRecorder, validMFAVerifyRequest)
	if validMFAVerifyResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on valid token MFA verify with non-existent user, got: %d", validMFAVerifyResponseRecorder.Code)
	}

	// 7. Custom Issuer
	customIssuerConfig := DefaultConfig()
	customIssuerConfig.MFA.Enabled = true
	customIssuerConfig.MFA.Issuer = "MyCustomIssuer"
	configManager.SetMemoryConfig(customIssuerConfig)

	customIssuerMFARequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), authContext), http.MethodPost, "/v1/auth/mfa/setup", nil)
	customIssuerMFARequest.Header.Set("Authorization", bearerHeader)
	customIssuerMFAResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSetupMFA(customIssuerMFAResponseRecorder, customIssuerMFARequest)
	if customIssuerMFAResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on custom issuer MFA setup with non-existent user, got: %d", customIssuerMFAResponseRecorder.Code)
	}

	// 8. Empty MFA Issuer defaults to "Layr"
	emptyIssuerConfig := DefaultConfig()
	emptyIssuerConfig.MFA.Issuer = ""
	emptyIssuerService := NewService(kernel)
	emptyIssuerService.configManager.SetMemoryConfig(emptyIssuerConfig)
	emptyIssuerBaseHandler := emptyIssuerService.baseHandler
	if emptyIssuerBaseHandler.totpManager == nil {
		t.Fatal("expected non-nil TOTP manager")
	}

	// 9. handleDisableMFA unit checks
	disabledConfig.MFA.Enabled = false
	configManager.SetMemoryConfig(disabledConfig)
	disableMFAForbiddenRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/v1/auth/mfa", nil)
	disableMFAForbiddenResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDisableMFA(disableMFAForbiddenResponseRecorder, disableMFAForbiddenRequest)
	if disableMFAForbiddenResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on MFA disable when disabled in config, got: %d", disableMFAForbiddenResponseRecorder.Code)
	}
	configManager.SetMemoryConfig(enabledConfig)

	missingBearerDisableRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/v1/auth/mfa", nil)
	missingBearerDisableResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDisableMFA(missingBearerDisableResponseRecorder, missingBearerDisableRequest)
	if missingBearerDisableResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on MFA disable without bearer, got: %d", missingBearerDisableResponseRecorder.Code)
	}

	invalidTokenDisableRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/v1/auth/mfa", nil)
	invalidTokenDisableRequest.Header.Set("Authorization", "Bearer invalid-token")
	invalidTokenDisableResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDisableMFA(invalidTokenDisableResponseRecorder, invalidTokenDisableRequest)
	if invalidTokenDisableResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid token MFA disable, got: %d", invalidTokenDisableResponseRecorder.Code)
	}

	validTokenDisableRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), authContext), http.MethodDelete, "/v1/auth/mfa", nil)
	validTokenDisableRequest.Header.Set("Authorization", bearerHeader)
	validTokenDisableResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDisableMFA(validTokenDisableResponseRecorder, validTokenDisableRequest)
	if validTokenDisableResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on valid token MFA disable with non-existent user, got: %d", validTokenDisableResponseRecorder.Code)
	}
}

func TestAuthMFAChallengeHandlerUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := service.baseHandler
	configManager := service.configManager

	// 1. MFA disabled -> 403
	disabledConfig := DefaultConfig()
	disabledConfig.MFA.Enabled = false
	configManager.SetMemoryConfig(disabledConfig)

	mfaChallengeDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/mfa/challenge", strings.NewReader(`{}`))
	mfaChallengeDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleChallengeMFA(mfaChallengeDisabledResponseRecorder, mfaChallengeDisabledRequest)
	if mfaChallengeDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on MFA challenge when MFA disabled, got: %d", mfaChallengeDisabledResponseRecorder.Code)
	}

	// Enable MFA
	enabledConfig := DefaultConfig()
	enabledConfig.MFA.Enabled = true
	configManager.SetMemoryConfig(enabledConfig)

	// 2. Bad JSON body -> 400
	badJSONRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/mfa/challenge", strings.NewReader(`{invalid`))
	badJSONResponseRecorder := httptest.NewRecorder()
	baseHandler.handleChallengeMFA(badJSONResponseRecorder, badJSONRequest)
	if badJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON in MFA challenge, got: %d", badJSONResponseRecorder.Code)
	}

	// 3. Missing ticket or code -> 400
	missingFieldsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/mfa/challenge", strings.NewReader(`{"mfa_ticket":"","code":""}`))
	missingFieldsResponseRecorder := httptest.NewRecorder()
	baseHandler.handleChallengeMFA(missingFieldsResponseRecorder, missingFieldsRequest)
	if missingFieldsResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing ticket/code in MFA challenge, got: %d", missingFieldsResponseRecorder.Code)
	}

	// 4. KV store missing ticket -> 401
	validPayloadRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/mfa/challenge", strings.NewReader(`{"mfa_ticket":"mfa_tk_test","code":"123456"}`))
	validPayloadResponseRecorder := httptest.NewRecorder()
	baseHandler.handleChallengeMFA(validPayloadResponseRecorder, validPayloadRequest)
	if validPayloadResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing ticket in MFA challenge, got: %d", validPayloadResponseRecorder.Code)
	}

	// 5. KV store with ticket on broken DB pool -> 401
	_ = kernel.KVStore().Set(context.Background(), "auth:mfa_ticket:mfa_tk_test", "test-user-id", 0)

	brokenDBRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/mfa/challenge", strings.NewReader(`{"mfa_ticket":"mfa_tk_test","code":"123456"}`))
	brokenDBResponseRecorder := httptest.NewRecorder()
	baseHandler.handleChallengeMFA(brokenDBResponseRecorder, brokenDBRequest)
	if brokenDBResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on broken DB pool in MFA challenge, got: %d", brokenDBResponseRecorder.Code)
	}
}
