package auth

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/core"
)

func TestAuthPasswordHandlerUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	baseHandler := NewBaseHandler(nil, configManager, cryptoKeyManager)
	testKVStore := newInMemoryKVStore()
	baseHandler.SetKVStore(testKVStore)
	ctx := context.Background()

	// 1. Password disabled -> 403 on sign up, sign in, and password reset
	disabledConfig := DefaultConfig()
	disabledConfig.Password.Enabled = false
	configManager.Set(disabledConfig)

	signupDisabledRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", strings.NewReader(`{"email":"test@example.com","password":"Password123!"}`))
	signupDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(signupDisabledResponseRecorder, signupDisabledRequest)
	if signupDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when password disabled, got: %d", signupDisabledResponseRecorder.Code)
	}

	resetDisabledRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/request", strings.NewReader(`{"email":"test@example.com"}`))
	resetDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestPasswordReset(resetDisabledResponseRecorder, resetDisabledRequest)
	if resetDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on password reset when password disabled, got: %d", resetDisabledResponseRecorder.Code)
	}

	confirmDisabledRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/confirm", strings.NewReader(`{"email":"test@example.com","code":"123456","password":"Password123!"}`))
	confirmDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPasswordReset(confirmDisabledResponseRecorder, confirmDisabledRequest)
	if confirmDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on password reset confirm when password disabled, got: %d", confirmDisabledResponseRecorder.Code)
	}

	// 2. Validation errors on SignUp
	configManager.Set(DefaultConfig())

	badJSONSignupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", bytes.NewReader([]byte(`bad-json`)))
	badJSONSignupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(badJSONSignupResponseRecorder, badJSONSignupRequest)
	if badJSONSignupResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON sign up, got: %d", badJSONSignupResponseRecorder.Code)
	}

	missingEmailSignupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", strings.NewReader(`{"password":"Password123!"}`))
	missingEmailSignupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(missingEmailSignupResponseRecorder, missingEmailSignupRequest)
	if missingEmailSignupResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing email/phone sign up, got: %d", missingEmailSignupResponseRecorder.Code)
	}

	shortPasswordSignupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", strings.NewReader(`{"email":"test@example.com","password":"short"}`))
	shortPasswordSignupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(shortPasswordSignupResponseRecorder, shortPasswordSignupRequest)
	if shortPasswordSignupResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on short password, got: %d", shortPasswordSignupResponseRecorder.Code)
	}

	invalidPhoneSignupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", strings.NewReader(`{"phone":"invalid","password":"Password123!"}`))
	invalidPhoneSignupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(invalidPhoneSignupResponseRecorder, invalidPhoneSignupRequest)
	if invalidPhoneSignupResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone sign up, got: %d", invalidPhoneSignupResponseRecorder.Code)
	}

	// 3. Validation errors on SignIn
	invalidPhoneSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", strings.NewReader(`{"phone":"invalid","password":"Password123!"}`))
	invalidPhoneSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(invalidPhoneSignInResponseRecorder, invalidPhoneSignInRequest)
	if invalidPhoneSignInResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone sign in, got: %d", invalidPhoneSignInResponseRecorder.Code)
	}

	badSignInJSONRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", bytes.NewReader([]byte(`bad-json`)))
	badSignInJSONResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(badSignInJSONResponseRecorder, badSignInJSONRequest)
	if badSignInJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON sign in, got: %d", badSignInJSONResponseRecorder.Code)
	}

	missingCredentialsSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", strings.NewReader(`{"email":"test@example.com"}`))
	missingCredentialsSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(missingCredentialsSignInResponseRecorder, missingCredentialsSignInRequest)
	if missingCredentialsSignInResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing password sign in, got: %d", missingCredentialsSignInResponseRecorder.Code)
	}

	// 4. Nil database pool -> 500 on valid inputs
	phoneSignupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", strings.NewReader(`{"phone":"+1234567890","password":"Password123!"}`))
	phoneSignupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(phoneSignupResponseRecorder, phoneSignupRequest)
	if phoneSignupResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on phone sign up nil db pool, got: %d", phoneSignupResponseRecorder.Code)
	}

	phoneSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", strings.NewReader(`{"phone":"+1234567890","password":"Password123!"}`))
	phoneSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(phoneSignInResponseRecorder, phoneSignInRequest)
	if phoneSignInResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on phone sign in nil db pool, got: %d", phoneSignInResponseRecorder.Code)
	}

	emailSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", strings.NewReader(`{"email":"alice@example.com","password":"Password123!"}`))
	emailSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(emailSignInResponseRecorder, emailSignInRequest)
	if emailSignInResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on email sign in nil db pool, got: %d", emailSignInResponseRecorder.Code)
	}

	// 5. Unconfigured email/SMS delivery -> 422
	activeConfig := configManager.Get()
	activeConfig.Password.Enabled = true
	configManager.Set(activeConfig)

	unconfEmailRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/request", strings.NewReader(`{"email":"user@example.com"}`))
	unconfEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestPasswordReset(unconfEmailResponseRecorder, unconfEmailRequest)
	if unconfEmailResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on unconfigured email delivery, got: %d", unconfEmailResponseRecorder.Code)
	}

	unconfSMSRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/request", strings.NewReader(`{"phone":"+15551234567"}`))
	unconfSMSResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestPasswordReset(unconfSMSResponseRecorder, unconfSMSRequest)
	if unconfSMSResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on unconfigured SMS delivery, got: %d", unconfSMSResponseRecorder.Code)
	}

	// 6. Configure Mock Delivery
	driverWebhook := "webhook"
	activeConfig.EmailDispatcher = EmailDispatcherConfig{
		Driver:  &driverWebhook,
		Webhook: EmailDispatcherWebhookConfig{URL: "http://localhost:9999/dummy"},
	}
	activeConfig.SMSDispatcher = SMSDispatcherConfig{
		Driver:  &driverWebhook,
		Webhook: SMSDispatcherWebhookConfig{URL: "http://localhost:9999/dummy"},
	}
	configManager.Set(activeConfig)
	baseHandler.SetEmailDispatcher(NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		emailDispatcherConfig := activeConfig.EmailDispatcher
		return &emailDispatcherConfig
	}, nil))
	baseHandler.SetSMSDispatcher(NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		smsDispatcherConfig := activeConfig.SMSDispatcher
		return &smsDispatcherConfig
	}, nil))

	// 7. Validation errors on Password Reset Request
	badResetJSONRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/request", strings.NewReader(`{invalid`))
	badResetJSONResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestPasswordReset(badResetJSONResponseRecorder, badResetJSONRequest)
	if badResetJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad reset JSON, got: %d", badResetJSONResponseRecorder.Code)
	}

	emptyRecipientResetRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/request", strings.NewReader(`{}`))
	emptyRecipientResetResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestPasswordReset(emptyRecipientResetResponseRecorder, emptyRecipientResetRequest)
	if emptyRecipientResetResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty recipient password reset, got: %d", emptyRecipientResetResponseRecorder.Code)
	}

	invalidPhoneResetRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/request", strings.NewReader(`{"phone":"invalid"}`))
	invalidPhoneResetResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestPasswordReset(invalidPhoneResetResponseRecorder, invalidPhoneResetRequest)
	if invalidPhoneResetResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone in password reset request, got: %d", invalidPhoneResetResponseRecorder.Code)
	}

	// Nil pool on valid request -> 500
	validResetRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/request", strings.NewReader(`{"email":"valid@example.com"}`))
	validResetResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestPasswordReset(validResetResponseRecorder, validResetRequest)
	if validResetResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool password reset request, got: %d", validResetResponseRecorder.Code)
	}

	// 8. Validation errors on Confirm
	badConfirmJSONRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/confirm", strings.NewReader(`{invalid`))
	badConfirmJSONResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPasswordReset(badConfirmJSONResponseRecorder, badConfirmJSONRequest)
	if badConfirmJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON in password reset confirm, got: %d", badConfirmJSONResponseRecorder.Code)
	}

	badConfirmRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/confirm", strings.NewReader(`{"email":"test@example.com"}`))
	badConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPasswordReset(badConfirmResponseRecorder, badConfirmRequest)
	if badConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing confirm fields, got: %d", badConfirmResponseRecorder.Code)
	}

	invalidPhoneConfirmRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/confirm", strings.NewReader(`{"phone":"invalid-phone","code":"123456","password":"ValidPassword123!"}`))
	invalidPhoneConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPasswordReset(invalidPhoneConfirmResponseRecorder, invalidPhoneConfirmRequest)
	if invalidPhoneConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone in confirm, got: %d", invalidPhoneConfirmResponseRecorder.Code)
	}

	shortConfirmPasswordRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/confirm", strings.NewReader(`{"email":"test@example.com","code":"123456","password":"short"}`))
	shortConfirmPasswordResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPasswordReset(shortConfirmPasswordResponseRecorder, shortConfirmPasswordRequest)
	if shortConfirmPasswordResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on short password in confirm, got: %d", shortConfirmPasswordResponseRecorder.Code)
	}

	// Nil pool on valid confirm -> 500
	validConfirmRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/confirm", strings.NewReader(`{"email":"test@example.com","code":"123456","password":"ValidPassword123!"}`))
	validConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPasswordReset(validConfirmResponseRecorder, validConfirmRequest)
	if validConfirmResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool confirm, got: %d", validConfirmResponseRecorder.Code)
	}
}

func TestAuthPasswordThreatValidationUnit(t *testing.T) {
	cryptoKeyManager, cryptoErr := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if cryptoErr != nil {
		t.Fatalf("failed to create crypto key manager: %v", cryptoErr)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	baseHandler := NewBaseHandler(nil, configManager, cryptoKeyManager)
	mockClient := &threatMockHTTPClient{}
	baseHandler.SetHTTPClient(mockClient)
	testKVStore := newInMemoryKVStore()
	baseHandler.SetKVStore(testKVStore)

	ctx := context.Background()

	// 1. Bot protection enabled in "always" mode -> missing CAPTCHA token returns 400
	botProtectionConfig := DefaultConfig()
	botProtectionConfig.Threat.BotProtection.Enabled = true
	botProtectionConfig.Threat.BotProtection.Mode = "always"
	configManager.Set(botProtectionConfig)

	// SignUp captcha check
	signupNoCaptchaRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", strings.NewReader(`{"email":"test@example.com","password":"Password123!"}`))
	signupNoCaptchaResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(signupNoCaptchaResponseRecorder, signupNoCaptchaRequest)
	if signupNoCaptchaResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on sign up without captcha, got: %d", signupNoCaptchaResponseRecorder.Code)
	}

	// SignIn captcha check
	signinNoCaptchaRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", strings.NewReader(`{"email":"test@example.com","password":"Password123!"}`))
	signinNoCaptchaResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(signinNoCaptchaResponseRecorder, signinNoCaptchaRequest)
	if signinNoCaptchaResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on sign in without captcha, got: %d", signinNoCaptchaResponseRecorder.Code)
	}

	// PasswordResetRequest captcha check
	resetNoCaptchaRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/request", strings.NewReader(`{"email":"test@example.com"}`))
	resetNoCaptchaResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestPasswordReset(resetNoCaptchaResponseRecorder, resetNoCaptchaRequest)
	if resetNoCaptchaResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on password reset request without captcha, got: %d", resetNoCaptchaResponseRecorder.Code)
	}

	// 2. Password breach detection enabled -> breached password returns 400
	passwordBreachConfig := DefaultConfig()
	passwordBreachConfig.Password.BreachCheck.Enabled = true
	passwordBreachConfig.Password.BreachCheck.FailOpen = false
	configManager.Set(passwordBreachConfig)

	// SHA-1 for "password" has prefix 5BAA6 and suffix 1E4C9B93F3F0682250B6CF8331B7EE68FD8
	mockClient.doFunc = func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("1E4C9B93F3F0682250B6CF8331B7EE68FD8:3861493\r\n")),
			Header:     make(http.Header),
		}, nil
	}

	signupBreachedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", strings.NewReader(`{"email":"test@example.com","password":"password"}`))
	signupBreachedResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(signupBreachedResponseRecorder, signupBreachedRequest)
	if signupBreachedResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(signupBreachedResponseRecorder.Body.String(), "breach") {
		t.Fatalf("expected 400 on breached password sign up, got: %d (%s)", signupBreachedResponseRecorder.Code, signupBreachedResponseRecorder.Body.String())
	}

	confirmBreachedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/confirm", strings.NewReader(`{"email":"test@example.com","code":"123456","password":"password"}`))
	confirmBreachedResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPasswordReset(confirmBreachedResponseRecorder, confirmBreachedRequest)
	if confirmBreachedResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(confirmBreachedResponseRecorder.Body.String(), "breach") {
		t.Fatalf("expected 400 on breached password reset confirm, got: %d (%s)", confirmBreachedResponseRecorder.Code, confirmBreachedResponseRecorder.Body.String())
	}
}
