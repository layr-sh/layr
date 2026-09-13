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

func TestAuthHandlerUserUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}
	configManager := NewConfigManager(nil, cryptoKeyManager)
	baseHandler := NewHandler(nil, configManager, cryptoKeyManager)

	// 1. Unauthenticated / Invalid / Expired Tokens on Guarded User Endpoints
	userEndpoints := []struct {
		name   string
		method string
		run    func(http.ResponseWriter, *http.Request)
	}{
		{"GetUser", http.MethodGet, baseHandler.handleGetUser},
		{"UpdateUserProperties", http.MethodPatch, baseHandler.handleUpdateUserProperties},
		{"UpdateUserPassword", http.MethodPatch, baseHandler.handleUpdateUserPassword},
		{"DeleteUser", http.MethodDelete, baseHandler.handleDeleteUser},
		{"UpdateUserEmail", http.MethodPatch, baseHandler.handleUpdateUserEmail},
		{"UpdateUserPhone", http.MethodPatch, baseHandler.handleUpdateUserPhone},
	}

	for _, userEndpoint := range userEndpoints {
		// No auth header/cookie
		unauthenticatedRequest := httptest.NewRequestWithContext(context.Background(), userEndpoint.method, "/api/v1/auth/user", strings.NewReader(`{}`))
		unauthenticatedResponseRecorder := httptest.NewRecorder()
		userEndpoint.run(unauthenticatedResponseRecorder, unauthenticatedRequest)
		if unauthenticatedResponseRecorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on %s without auth, got: %d", userEndpoint.name, unauthenticatedResponseRecorder.Code)
		}

		// Invalid bearer token
		invalidTokenRequest := httptest.NewRequestWithContext(context.Background(), userEndpoint.method, "/api/v1/auth/user", strings.NewReader(`{}`))
		invalidTokenRequest.Header.Set("Authorization", "Bearer invalid-token-string")
		invalidTokenResponseRecorder := httptest.NewRecorder()
		userEndpoint.run(invalidTokenResponseRecorder, invalidTokenRequest)
		if invalidTokenResponseRecorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on %s with invalid token, got: %d", userEndpoint.name, invalidTokenResponseRecorder.Code)
		}

		// Expired bearer token
		expiredToken, _ := baseHandler.signer.GenerateAccessToken(jwt.Claims{
			Subject:     "user-expired",
			Email:       "user@example.com",
			Role:        "authenticated",
			IsAnonymous: false,
		}, -10)
		expiredTokenRequest := httptest.NewRequestWithContext(context.Background(), userEndpoint.method, "/api/v1/auth/user", strings.NewReader(`{}`))
		expiredTokenRequest.Header.Set("Authorization", "Bearer "+expiredToken)
		expiredTokenResponseRecorder := httptest.NewRecorder()
		userEndpoint.run(expiredTokenResponseRecorder, expiredTokenRequest)
		if expiredTokenResponseRecorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on %s with expired token, got: %d", userEndpoint.name, expiredTokenResponseRecorder.Code)
		}
	}

	// 2. Valid token for profile tests
	validToken, err := baseHandler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     "user-unit-123",
		Email:       "unit@example.com",
		Role:        "user",
		IsAnonymous: false,
	}, 3600)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	// GET /api/v1/auth/user on nil pool -> 500
	getUserRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	getUserRequest.Header.Set("Authorization", "Bearer "+validToken)
	getUserResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetUser(getUserResponseRecorder, getUserRequest)
	if getUserResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool GET user, got: %d", getUserResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/properties bad JSON -> 400
	badJSONPatchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/properties", strings.NewReader(`{invalid`))
	badJSONPatchRequest.Header.Set("Authorization", "Bearer "+validToken)
	badJSONPatchResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserProperties(badJSONPatchResponseRecorder, badJSONPatchRequest)
	if badJSONPatchResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON PATCH user properties, got: %d", badJSONPatchResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/properties on nil pool -> 500
	validPatchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/properties", strings.NewReader(`{"properties":{"tier":"gold"}}`))
	validPatchRequest.Header.Set("Authorization", "Bearer "+validToken)
	validPatchResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserProperties(validPatchResponseRecorder, validPatchRequest)
	if validPatchResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool PATCH user properties, got: %d", validPatchResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/password bad JSON -> 400
	badJSONPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/password", strings.NewReader(`{invalid`))
	badJSONPasswordRequest.Header.Set("Authorization", "Bearer "+validToken)
	badJSONPasswordResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserPassword(badJSONPasswordResponseRecorder, badJSONPasswordRequest)
	if badJSONPasswordResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON PATCH user password, got: %d", badJSONPasswordResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/password on nil pool -> 500
	validPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/password", strings.NewReader(`{"new_password":"NewValidPassword123!"}`))
	validPasswordRequest.Header.Set("Authorization", "Bearer "+validToken)
	validPasswordResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserPassword(validPasswordResponseRecorder, validPasswordRequest)
	if validPasswordResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool PATCH user password, got: %d", validPasswordResponseRecorder.Code)
	}

	// DELETE /api/v1/auth/user on nil pool -> 500
	deleteRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/user", nil)
	deleteRequest.Header.Set("Authorization", "Bearer "+validToken)
	deleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteUser(deleteResponseRecorder, deleteRequest)
	if deleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool DELETE user, got: %d", deleteResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/email bad JSON -> 400
	badJSONEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/email", strings.NewReader(`{invalid`))
	badJSONEmailRequest.Header.Set("Authorization", "Bearer "+validToken)
	badJSONEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserEmail(badJSONEmailResponseRecorder, badJSONEmailRequest)
	if badJSONEmailResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON PATCH user email, got: %d", badJSONEmailResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/email invalid email -> 400
	invalidEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/email", strings.NewReader(`{"email":"not-an-email"}`))
	invalidEmailRequest.Header.Set("Authorization", "Bearer "+validToken)
	invalidEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserEmail(invalidEmailResponseRecorder, invalidEmailRequest)
	if invalidEmailResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid email PATCH user email, got: %d", invalidEmailResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/email delivery not ready -> 422
	deliveryNotReadyEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/email", strings.NewReader(`{"email":"valid@example.com"}`))
	deliveryNotReadyEmailRequest.Header.Set("Authorization", "Bearer "+validToken)
	deliveryNotReadyEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserEmail(deliveryNotReadyEmailResponseRecorder, deliveryNotReadyEmailRequest)
	if deliveryNotReadyEmailResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on email delivery not ready, got: %d", deliveryNotReadyEmailResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/email delivery ready on nil pool -> 500
	driverWebhook := "webhook"
	mockEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{Driver: &driverWebhook, Webhook: EmailDispatcherWebhookConfig{URL: "http://localhost"}}
	}, nil)
	baseHandler.SetEmailDispatcher(mockEmailDispatcher)

	nilDBEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/email", strings.NewReader(`{"email":"valid@example.com"}`))
	nilDBEmailRequest.Header.Set("Authorization", "Bearer "+validToken)
	nilDBEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserEmail(nilDBEmailResponseRecorder, nilDBEmailRequest)
	if nilDBEmailResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool PATCH user email, got: %d", nilDBEmailResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/phone bad JSON -> 400
	badJSONPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", strings.NewReader(`{invalid`))
	badJSONPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	badJSONPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserPhone(badJSONPhoneResponseRecorder, badJSONPhoneRequest)
	if badJSONPhoneResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON PATCH user phone, got: %d", badJSONPhoneResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/phone empty phone -> 400
	emptyPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", strings.NewReader(`{"phone":""}`))
	emptyPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	emptyPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserPhone(emptyPhoneResponseRecorder, emptyPhoneRequest)
	if emptyPhoneResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty phone PATCH user phone, got: %d", emptyPhoneResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/phone invalid phone format -> 400
	invalidPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", strings.NewReader(`{"phone":"invalid-format"}`))
	invalidPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	invalidPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserPhone(invalidPhoneResponseRecorder, invalidPhoneRequest)
	if invalidPhoneResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid format PATCH user phone, got: %d", invalidPhoneResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/phone delivery not ready -> 422
	deliveryNotReadyPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", strings.NewReader(`{"phone":"+1234567890"}`))
	deliveryNotReadyPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	deliveryNotReadyPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserPhone(deliveryNotReadyPhoneResponseRecorder, deliveryNotReadyPhoneRequest)
	if deliveryNotReadyPhoneResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on phone delivery not ready, got: %d", deliveryNotReadyPhoneResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/phone delivery ready on nil pool -> 500
	mockSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{Driver: &driverWebhook, Webhook: SMSDispatcherWebhookConfig{URL: "http://localhost"}}
	}, nil)
	baseHandler.SetSMSDispatcher(mockSMSDispatcher)

	nilDBPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", strings.NewReader(`{"phone":"+1234567890"}`))
	nilDBPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	nilDBPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserPhone(nilDBPhoneResponseRecorder, nilDBPhoneRequest)
	if nilDBPhoneResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool PATCH user phone, got: %d", nilDBPhoneResponseRecorder.Code)
	}

	// 3. Email & Phone Verification Unified Request and Confirm Tests
	missingEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/request", strings.NewReader(`{}`))
	missingEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserEmailVerificationRequest(missingEmailResponseRecorder, missingEmailRequest)
	if missingEmailResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing email, got: %d", missingEmailResponseRecorder.Code)
	}

	// Reset email dispatcher to nil to test unconfigured
	baseHandler.SetEmailDispatcher(nil)
	unconfiguredEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/request", strings.NewReader(`{"email":"new@example.com"}`))
	unconfiguredEmailRequest.Header.Set("Authorization", "Bearer "+validToken)
	unconfiguredEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserEmailVerificationRequest(unconfiguredEmailResponseRecorder, unconfiguredEmailRequest)
	if unconfiguredEmailResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on unconfigured email delivery, got: %d", unconfiguredEmailResponseRecorder.Code)
	}

	missingPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/request", strings.NewReader(`{}`))
	missingPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	missingPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserPhoneVerificationRequest(missingPhoneResponseRecorder, missingPhoneRequest)
	if missingPhoneResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing phone, got: %d", missingPhoneResponseRecorder.Code)
	}

	invalidPhoneVerificationRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/request", strings.NewReader(`{"phone":"12345"}`))
	invalidPhoneVerificationRequest.Header.Set("Authorization", "Bearer "+validToken)
	invalidPhoneVerificationResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserPhoneVerificationRequest(invalidPhoneVerificationResponseRecorder, invalidPhoneVerificationRequest)
	if invalidPhoneVerificationResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone format, got: %d", invalidPhoneVerificationResponseRecorder.Code)
	}

	baseHandler.SetSMSDispatcher(nil)
	unconfiguredPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/request", strings.NewReader(`{"phone":"+15551234567"}`))
	unconfiguredPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	unconfiguredPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserPhoneVerificationRequest(unconfiguredPhoneResponseRecorder, unconfiguredPhoneRequest)
	if unconfiguredPhoneResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on unconfigured SMS delivery, got: %d", unconfiguredPhoneResponseRecorder.Code)
	}

	// Restore dispatchers for nil pool verification tests
	baseHandler.SetEmailDispatcher(mockEmailDispatcher)
	baseHandler.SetSMSDispatcher(mockSMSDispatcher)

	emailVerificationRequestRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/request", strings.NewReader(`{"email":"test@example.com"}`))
	emailVerificationRequestResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserEmailVerificationRequest(emailVerificationRequestResponseRecorder, emailVerificationRequestRequest)
	if emailVerificationRequestResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool email verify request, got: %d", emailVerificationRequestResponseRecorder.Code)
	}

	phoneVerificationRequestRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/request", strings.NewReader(`{"phone":"+15551234567"}`))
	phoneVerificationRequestResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserPhoneVerificationRequest(phoneVerificationRequestResponseRecorder, phoneVerificationRequestRequest)
	if phoneVerificationRequestResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool phone verify request, got: %d", phoneVerificationRequestResponseRecorder.Code)
	}

	// Verification Confirm Unit Tests
	badJSONEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(`{bad`))
	badJSONEmailConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserEmailVerificationConfirm(badJSONEmailConfirmResponseRecorder, badJSONEmailConfirmRequest)
	if badJSONEmailConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON email confirm, got: %d", badJSONEmailConfirmResponseRecorder.Code)
	}

	missingCodeEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(`{"email":"user@example.com"}`))
	missingCodeEmailConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserEmailVerificationConfirm(missingCodeEmailConfirmResponseRecorder, missingCodeEmailConfirmRequest)
	if missingCodeEmailConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing code email confirm, got: %d", missingCodeEmailConfirmResponseRecorder.Code)
	}

	validEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(`{"email":"user@example.com","code":"123456"}`))
	validEmailConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserEmailVerificationConfirm(validEmailConfirmResponseRecorder, validEmailConfirmRequest)
	if validEmailConfirmResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool email confirm, got: %d", validEmailConfirmResponseRecorder.Code)
	}

	badJSONPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(`{bad`))
	badJSONPhoneConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserPhoneVerificationConfirm(badJSONPhoneConfirmResponseRecorder, badJSONPhoneConfirmRequest)
	if badJSONPhoneConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON phone confirm, got: %d", badJSONPhoneConfirmResponseRecorder.Code)
	}

	missingCodePhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"phone":"+15551234567"}`))
	missingCodePhoneConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserPhoneVerificationConfirm(missingCodePhoneConfirmResponseRecorder, missingCodePhoneConfirmRequest)
	if missingCodePhoneConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing code phone confirm, got: %d", missingCodePhoneConfirmResponseRecorder.Code)
	}

	invalidPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"phone":"bad-phone","code":"123456"}`))
	invalidPhoneConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserPhoneVerificationConfirm(invalidPhoneConfirmResponseRecorder, invalidPhoneConfirmRequest)
	if invalidPhoneConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone format confirm, got: %d", invalidPhoneConfirmResponseRecorder.Code)
	}

	validPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"phone":"+15551234567","code":"123456"}`))
	validPhoneConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserPhoneVerificationConfirm(validPhoneConfirmResponseRecorder, validPhoneConfirmRequest)
	if validPhoneConfirmResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool phone confirm, got: %d", validPhoneConfirmResponseRecorder.Code)
	}

	// 4. User Properties Clean Pass-through Test
	userProps := map[string]any{
		"display_name": "Alice",
		"theme":        "dark",
	}
	cleanProps := sanitizeUserProperties(userProps)
	if cleanProps["display_name"] != "Alice" || cleanProps["theme"] != "dark" {
		t.Fatalf("expected display_name and theme preserved: %+v", cleanProps)
	}

	// 5. Claims-based email and phone verification confirm
	tokenWithEmail, err := baseHandler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     "user-claims-email",
		Email:       "claims.email@example.com",
		Role:        "authenticated",
		IsAnonymous: false,
	}, 3600)
	if err != nil {
		t.Fatalf("failed to sign token with email: %v", err)
	}
	claimsEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(`{"code":"123456"}`))
	claimsEmailRequest.Header.Set("Authorization", "Bearer "+tokenWithEmail)
	claimsEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserEmailVerificationConfirm(claimsEmailResponseRecorder, claimsEmailRequest)
	if claimsEmailResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool claims email confirm, got: %d", claimsEmailResponseRecorder.Code)
	}

	tokenWithPhone, err := baseHandler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     "user-claims-phone",
		Phone:       "+15551234567",
		Role:        "authenticated",
		IsAnonymous: false,
	}, 3600)
	if err != nil {
		t.Fatalf("failed to sign token with phone: %v", err)
	}
	claimsPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"code":"123456"}`))
	claimsPhoneRequest.Header.Set("Authorization", "Bearer "+tokenWithPhone)
	claimsPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUserPhoneVerificationConfirm(claimsPhoneResponseRecorder, claimsPhoneRequest)
	if claimsPhoneResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool claims phone confirm, got: %d", claimsPhoneResponseRecorder.Code)
	}

	// 6. Nil database pool error handling in fetchUserRecordByID and fetchUserRecordByRecipient
	nilDBUserRecord, nilDBErr := fetchUserRecordByID(context.Background(), nil, "user-nil-db")
	if nilDBErr == nil || nilDBUserRecord.ID != "user-nil-db" {
		t.Fatalf("expected error and fallback user record with nil DB, got: %v, %v", nilDBUserRecord, nilDBErr)
	}

	nilDBRecipientUserRecord, nilDBRecipientErr := fetchUserRecordByRecipient(context.Background(), nil, "user@example.com")
	if nilDBRecipientErr == nil || nilDBRecipientUserRecord.ID != "" {
		t.Fatalf("expected error and empty user record with nil DB, got: %v, %v", nilDBRecipientUserRecord, nilDBRecipientErr)
	}

	// 7. Sanitization of user properties and system property key check
	testProps := map[string]any{
		"theme":                "dark",
		"encrypted_secret":     "malicious_value",
		"user_data_enc":        "malicious_value",
		"role":                 "admin",
		"locked_until":         "2099-01-01",
		"is_anonymous":         true,
		"email_verified_at":    "2025-01-01",
		"phone_verified_at":    "2025-01-01",
		"password_hash":        "hash",
		"mfa_enabled":          true,
		"mfa_pending":          true,
		"encrypted_mfa_secret": "secret",
		"id":                   "hacked-id",
		"email":                "hacked@example.com",
		"phone":                "+19999999999",
		"created_at":           "timestamp",
		"last_updated_at":      "timestamp",
	}
	cleaned := sanitizeUserProperties(testProps)
	if len(cleaned) != 1 || cleaned["theme"] != "dark" {
		t.Fatalf("expected only non-system properties to remain, got: %v", cleaned)
	}
	if !isSystemPropertyKey("encrypted_token") || !isSystemPropertyKey("data_enc") || !isSystemPropertyKey("role") || isSystemPropertyKey("displayName") {
		t.Fatalf("unexpected isSystemPropertyKey behavior")
	}
}
