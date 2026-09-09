package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-fuego/fuego"
	"layr.sh/auth/jwt"
	"layr.sh/core"
)

func TestAuthHandlerUserUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}
	configManager := NewConfigManager(nil, cryptoKeyManager)
	handler := NewHandler(nil, configManager, cryptoKeyManager)

	// 1. Unauthenticated / Invalid / Expired Tokens on Guarded User Endpoints
	userHandlers := []struct {
		name    string
		method  string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"GetUser", http.MethodGet, handler.handleGetUser},
		{"UpdateUserProperties", http.MethodPatch, handler.handleUpdateUserProperties},
		{"UpdateUserPassword", http.MethodPatch, handler.handleUpdateUserPassword},
		{"DeleteUser", http.MethodDelete, handler.handleDeleteUser},
		{"UpdateUserEmail", http.MethodPatch, handler.handleUpdateUserEmail},
		{"UpdateUserPhone", http.MethodPatch, handler.handleUpdateUserPhone},
	}

	for _, userEndpoint := range userHandlers {
		// No auth header/cookie
		unauthenticatedRequest := httptest.NewRequestWithContext(context.Background(), userEndpoint.method, "/api/v1/auth/user", strings.NewReader(`{}`))
		unauthenticatedResponseRecorder := httptest.NewRecorder()
		userEndpoint.handler(unauthenticatedResponseRecorder, unauthenticatedRequest)
		if unauthenticatedResponseRecorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on %s without auth, got: %d", userEndpoint.name, unauthenticatedResponseRecorder.Code)
		}

		// Invalid bearer token
		invalidTokenRequest := httptest.NewRequestWithContext(context.Background(), userEndpoint.method, "/api/v1/auth/user", strings.NewReader(`{}`))
		invalidTokenRequest.Header.Set("Authorization", "Bearer invalid-token-string")
		invalidTokenResponseRecorder := httptest.NewRecorder()
		userEndpoint.handler(invalidTokenResponseRecorder, invalidTokenRequest)
		if invalidTokenResponseRecorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on %s with invalid token, got: %d", userEndpoint.name, invalidTokenResponseRecorder.Code)
		}

		// Expired bearer token
		expiredToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
			Subject:     "user-expired",
			Email:       "user@example.com",
			Role:        "authenticated",
			IsAnonymous: false,
		}, -10)
		expiredTokenRequest := httptest.NewRequestWithContext(context.Background(), userEndpoint.method, "/api/v1/auth/user", strings.NewReader(`{}`))
		expiredTokenRequest.Header.Set("Authorization", "Bearer "+expiredToken)
		expiredTokenResponseRecorder := httptest.NewRecorder()
		userEndpoint.handler(expiredTokenResponseRecorder, expiredTokenRequest)
		if expiredTokenResponseRecorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on %s with expired token, got: %d", userEndpoint.name, expiredTokenResponseRecorder.Code)
		}
	}

	// 2. Valid token for profile tests
	validToken, err := handler.signer.GenerateAccessToken(jwt.Claims{
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
	handler.handleGetUser(getUserResponseRecorder, getUserRequest)
	if getUserResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool GET user, got: %d", getUserResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/properties bad JSON -> 400
	badJSONPatchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/properties", strings.NewReader(`{invalid`))
	badJSONPatchRequest.Header.Set("Authorization", "Bearer "+validToken)
	badJSONPatchResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserProperties(badJSONPatchResponseRecorder, badJSONPatchRequest)
	if badJSONPatchResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON PATCH user properties, got: %d", badJSONPatchResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/properties on nil pool -> 500
	validPatchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/properties", strings.NewReader(`{"properties":{"tier":"gold"}}`))
	validPatchRequest.Header.Set("Authorization", "Bearer "+validToken)
	validPatchResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserProperties(validPatchResponseRecorder, validPatchRequest)
	if validPatchResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool PATCH user properties, got: %d", validPatchResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/password bad JSON -> 400
	badJSONPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/password", strings.NewReader(`{invalid`))
	badJSONPasswordRequest.Header.Set("Authorization", "Bearer "+validToken)
	badJSONPasswordResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPassword(badJSONPasswordResponseRecorder, badJSONPasswordRequest)
	if badJSONPasswordResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON PATCH user password, got: %d", badJSONPasswordResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/password on nil pool -> 500
	validPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/password", strings.NewReader(`{"new_password":"NewValidPassword123!"}`))
	validPasswordRequest.Header.Set("Authorization", "Bearer "+validToken)
	validPasswordResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPassword(validPasswordResponseRecorder, validPasswordRequest)
	if validPasswordResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool PATCH user password, got: %d", validPasswordResponseRecorder.Code)
	}

	// DELETE /api/v1/auth/user on nil pool -> 500
	deleteRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/user", nil)
	deleteRequest.Header.Set("Authorization", "Bearer "+validToken)
	deleteResponseRecorder := httptest.NewRecorder()
	handler.handleDeleteUser(deleteResponseRecorder, deleteRequest)
	if deleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool DELETE user, got: %d", deleteResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/email bad JSON -> 400
	badJSONEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/email", strings.NewReader(`{invalid`))
	badJSONEmailRequest.Header.Set("Authorization", "Bearer "+validToken)
	badJSONEmailResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserEmail(badJSONEmailResponseRecorder, badJSONEmailRequest)
	if badJSONEmailResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON PATCH user email, got: %d", badJSONEmailResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/email invalid email -> 400
	invalidEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/email", strings.NewReader(`{"email":"not-an-email"}`))
	invalidEmailRequest.Header.Set("Authorization", "Bearer "+validToken)
	invalidEmailResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserEmail(invalidEmailResponseRecorder, invalidEmailRequest)
	if invalidEmailResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid email PATCH user email, got: %d", invalidEmailResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/email delivery not ready -> 422
	deliveryNotReadyEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/email", strings.NewReader(`{"email":"valid@example.com"}`))
	deliveryNotReadyEmailRequest.Header.Set("Authorization", "Bearer "+validToken)
	deliveryNotReadyEmailResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserEmail(deliveryNotReadyEmailResponseRecorder, deliveryNotReadyEmailRequest)
	if deliveryNotReadyEmailResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on email delivery not ready, got: %d", deliveryNotReadyEmailResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/email delivery ready on nil pool -> 500
	driverWebhook := "webhook"
	mockEmailDispatcher := NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{Driver: &driverWebhook, Webhook: EmailDispatcherWebhookConfig{URL: "http://localhost"}}
	}, nil)
	handler.SetEmailDispatcher(mockEmailDispatcher)

	nilDBEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/email", strings.NewReader(`{"email":"valid@example.com"}`))
	nilDBEmailRequest.Header.Set("Authorization", "Bearer "+validToken)
	nilDBEmailResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserEmail(nilDBEmailResponseRecorder, nilDBEmailRequest)
	if nilDBEmailResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool PATCH user email, got: %d", nilDBEmailResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/phone bad JSON -> 400
	badJSONPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", strings.NewReader(`{invalid`))
	badJSONPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	badJSONPhoneResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPhone(badJSONPhoneResponseRecorder, badJSONPhoneRequest)
	if badJSONPhoneResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON PATCH user phone, got: %d", badJSONPhoneResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/phone empty phone -> 400
	emptyPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", strings.NewReader(`{"phone":""}`))
	emptyPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	emptyPhoneResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPhone(emptyPhoneResponseRecorder, emptyPhoneRequest)
	if emptyPhoneResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty phone PATCH user phone, got: %d", emptyPhoneResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/phone invalid phone format -> 400
	invalidPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", strings.NewReader(`{"phone":"invalid-format"}`))
	invalidPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	invalidPhoneResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPhone(invalidPhoneResponseRecorder, invalidPhoneRequest)
	if invalidPhoneResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid format PATCH user phone, got: %d", invalidPhoneResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/phone delivery not ready -> 422
	deliveryNotReadyPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", strings.NewReader(`{"phone":"+1234567890"}`))
	deliveryNotReadyPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	deliveryNotReadyPhoneResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPhone(deliveryNotReadyPhoneResponseRecorder, deliveryNotReadyPhoneRequest)
	if deliveryNotReadyPhoneResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on phone delivery not ready, got: %d", deliveryNotReadyPhoneResponseRecorder.Code)
	}

	// PATCH /api/v1/auth/user/phone delivery ready on nil pool -> 500
	mockSMSDispatcher := NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{Driver: &driverWebhook, Webhook: SMSDispatcherWebhookConfig{URL: "http://localhost"}}
	}, nil)
	handler.SetSMSDispatcher(mockSMSDispatcher)

	nilDBPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", strings.NewReader(`{"phone":"+1234567890"}`))
	nilDBPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	nilDBPhoneResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPhone(nilDBPhoneResponseRecorder, nilDBPhoneRequest)
	if nilDBPhoneResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool PATCH user phone, got: %d", nilDBPhoneResponseRecorder.Code)
	}

	// 3. Email & Phone Verification Unified Request and Confirm Tests
	missingEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/request", strings.NewReader(`{}`))
	missingEmailResponseRecorder := httptest.NewRecorder()
	handler.handleUserEmailVerificationRequest(missingEmailResponseRecorder, missingEmailRequest)
	if missingEmailResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing email, got: %d", missingEmailResponseRecorder.Code)
	}

	// Reset email dispatcher to nil to test unconfigured
	handler.SetEmailDispatcher(nil)
	unconfiguredEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/request", strings.NewReader(`{"email":"new@example.com"}`))
	unconfiguredEmailRequest.Header.Set("Authorization", "Bearer "+validToken)
	unconfiguredEmailResponseRecorder := httptest.NewRecorder()
	handler.handleUserEmailVerificationRequest(unconfiguredEmailResponseRecorder, unconfiguredEmailRequest)
	if unconfiguredEmailResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on unconfigured email delivery, got: %d", unconfiguredEmailResponseRecorder.Code)
	}

	missingPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/request", strings.NewReader(`{}`))
	missingPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	missingPhoneResponseRecorder := httptest.NewRecorder()
	handler.handleUserPhoneVerificationRequest(missingPhoneResponseRecorder, missingPhoneRequest)
	if missingPhoneResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing phone, got: %d", missingPhoneResponseRecorder.Code)
	}

	invalidPhoneVerificationRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/request", strings.NewReader(`{"phone":"12345"}`))
	invalidPhoneVerificationRequest.Header.Set("Authorization", "Bearer "+validToken)
	invalidPhoneVerificationResponseRecorder := httptest.NewRecorder()
	handler.handleUserPhoneVerificationRequest(invalidPhoneVerificationResponseRecorder, invalidPhoneVerificationRequest)
	if invalidPhoneVerificationResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone format, got: %d", invalidPhoneVerificationResponseRecorder.Code)
	}

	handler.SetSMSDispatcher(nil)
	unconfiguredPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/request", strings.NewReader(`{"phone":"+15551234567"}`))
	unconfiguredPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	unconfiguredPhoneResponseRecorder := httptest.NewRecorder()
	handler.handleUserPhoneVerificationRequest(unconfiguredPhoneResponseRecorder, unconfiguredPhoneRequest)
	if unconfiguredPhoneResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on unconfigured SMS delivery, got: %d", unconfiguredPhoneResponseRecorder.Code)
	}

	// Restore dispatchers for nil pool verification tests
	handler.SetEmailDispatcher(mockEmailDispatcher)
	handler.SetSMSDispatcher(mockSMSDispatcher)

	emailVerificationRequestRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/request", strings.NewReader(`{"email":"test@example.com"}`))
	emailVerificationRequestResponseRecorder := httptest.NewRecorder()
	handler.handleUserEmailVerificationRequest(emailVerificationRequestResponseRecorder, emailVerificationRequestRequest)
	if emailVerificationRequestResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool email verify request, got: %d", emailVerificationRequestResponseRecorder.Code)
	}

	phoneVerificationRequestRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/request", strings.NewReader(`{"phone":"+15551234567"}`))
	phoneVerificationRequestResponseRecorder := httptest.NewRecorder()
	handler.handleUserPhoneVerificationRequest(phoneVerificationRequestResponseRecorder, phoneVerificationRequestRequest)
	if phoneVerificationRequestResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool phone verify request, got: %d", phoneVerificationRequestResponseRecorder.Code)
	}

	// Verification Confirm Unit Tests
	badJSONEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(`{bad`))
	badJSONEmailConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(badJSONEmailConfirmResponseRecorder, badJSONEmailConfirmRequest)
	if badJSONEmailConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON email confirm, got: %d", badJSONEmailConfirmResponseRecorder.Code)
	}

	missingCodeEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(`{"email":"user@example.com"}`))
	missingCodeEmailConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(missingCodeEmailConfirmResponseRecorder, missingCodeEmailConfirmRequest)
	if missingCodeEmailConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing code email confirm, got: %d", missingCodeEmailConfirmResponseRecorder.Code)
	}

	validEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(`{"email":"user@example.com","code":"123456"}`))
	validEmailConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(validEmailConfirmResponseRecorder, validEmailConfirmRequest)
	if validEmailConfirmResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool email confirm, got: %d", validEmailConfirmResponseRecorder.Code)
	}

	badJSONPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(`{bad`))
	badJSONPhoneConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(badJSONPhoneConfirmResponseRecorder, badJSONPhoneConfirmRequest)
	if badJSONPhoneConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON phone confirm, got: %d", badJSONPhoneConfirmResponseRecorder.Code)
	}

	missingCodePhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"phone":"+15551234567"}`))
	missingCodePhoneConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(missingCodePhoneConfirmResponseRecorder, missingCodePhoneConfirmRequest)
	if missingCodePhoneConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing code phone confirm, got: %d", missingCodePhoneConfirmResponseRecorder.Code)
	}

	invalidPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"phone":"bad-phone","code":"123456"}`))
	invalidPhoneConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(invalidPhoneConfirmResponseRecorder, invalidPhoneConfirmRequest)
	if invalidPhoneConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone format confirm, got: %d", invalidPhoneConfirmResponseRecorder.Code)
	}

	validPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"phone":"+15551234567","code":"123456"}`))
	validPhoneConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(validPhoneConfirmResponseRecorder, validPhoneConfirmRequest)
	if validPhoneConfirmResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool phone confirm, got: %d", validPhoneConfirmResponseRecorder.Code)
	}

	// 4. Property Sanitization Test
	dirtyProperties := map[string]any{
		"mfa_secret_enc": "enc:v1:secret",
		"mfa_pending":    true,
		"mfa_enabled":    true,
		"display_name":   "Alice",
		"theme":          "dark",
	}
	cleanProperties := sanitizeUserProperties(dirtyProperties)
	if _, exists := cleanProperties["mfa_secret_enc"]; exists {
		t.Fatalf("expected mfa_secret_enc to be stripped")
	}
	if _, exists := cleanProperties["mfa_pending"]; exists {
		t.Fatalf("expected mfa_pending to be stripped")
	}
	if _, exists := cleanProperties["mfa_enabled"]; exists {
		t.Fatalf("expected mfa_enabled to be stripped from properties")
	}
	if cleanProperties["display_name"] != "Alice" || cleanProperties["theme"] != "dark" {
		t.Fatalf("expected display_name and theme preserved: %+v", cleanProperties)
	}

	// 5. Claims-based email and phone verification confirm
	tokenWithEmail, err := handler.signer.GenerateAccessToken(jwt.Claims{
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
	handler.handleUserEmailVerificationConfirm(claimsEmailResponseRecorder, claimsEmailRequest)
	if claimsEmailResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool claims email confirm, got: %d", claimsEmailResponseRecorder.Code)
	}

	tokenWithPhone, err := handler.signer.GenerateAccessToken(jwt.Claims{
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
	handler.handleUserPhoneVerificationConfirm(claimsPhoneResponseRecorder, claimsPhoneRequest)
	if claimsPhoneResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool claims phone confirm, got: %d", claimsPhoneResponseRecorder.Code)
	}

	// 6. RegisterUserRoutes
	fuegoEngine := fuego.NewServer()
	router := core.NewRouter(fuegoEngine)
	handler.RegisterUserRoutes(router)
}
