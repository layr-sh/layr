package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/core"
)

func TestAuthPasswordResetHandlerUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	handler := NewHandler(nil, configManager, cryptoKeyManager)
	testKVStore := newInMemoryKVStore()
	handler.SetKVStore(testKVStore)

	// 1. Password disabled -> 403
	disabledConfig := configManager.Get()
	disabledConfig.Password.Enabled = false
	configManager.Set(disabledConfig)

	resetDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/request", strings.NewReader(`{"email":"test@example.com"}`))
	resetDisabledResponseRecorder := httptest.NewRecorder()
	handler.handlePasswordResetRequest(resetDisabledResponseRecorder, resetDisabledRequest)
	if resetDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on password reset when password disabled, got: %d", resetDisabledResponseRecorder.Code)
	}

	confirmDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/confirm", strings.NewReader(`{"email":"test@example.com","code":"123456","password":"Password123!"}`))
	confirmDisabledResponseRecorder := httptest.NewRecorder()
	handler.handlePasswordResetConfirm(confirmDisabledResponseRecorder, confirmDisabledRequest)
	if confirmDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on password reset confirm when password disabled, got: %d", confirmDisabledResponseRecorder.Code)
	}

	// 2. Enable Password
	activeConfig := configManager.Get()
	activeConfig.Password.Enabled = true
	configManager.Set(activeConfig)

	// 3. Unconfigured email/SMS delivery -> 422
	unconfEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/request", strings.NewReader(`{"email":"user@example.com"}`))
	unconfEmailResponseRecorder := httptest.NewRecorder()
	handler.handlePasswordResetRequest(unconfEmailResponseRecorder, unconfEmailRequest)
	if unconfEmailResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on unconfigured email delivery, got: %d", unconfEmailResponseRecorder.Code)
	}

	unconfSMSRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/request", strings.NewReader(`{"phone":"+15551234567"}`))
	unconfSMSResponseRecorder := httptest.NewRecorder()
	handler.handlePasswordResetRequest(unconfSMSResponseRecorder, unconfSMSRequest)
	if unconfSMSResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on unconfigured SMS delivery, got: %d", unconfSMSResponseRecorder.Code)
	}

	// 4. Configure Mock Delivery
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
	handler.SetEmailDispatcher(NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		emailDispatcherConfig := activeConfig.EmailDispatcher
		return &emailDispatcherConfig
	}, nil))
	handler.SetSMSDispatcher(NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		smsDispatcherConfig := activeConfig.SMSDispatcher
		return &smsDispatcherConfig
	}, nil))

	// 5. Validation errors on Request
	badResetJSONRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/request", strings.NewReader(`{invalid`))
	badResetJSONResponseRecorder := httptest.NewRecorder()
	handler.handlePasswordResetRequest(badResetJSONResponseRecorder, badResetJSONRequest)
	if badResetJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad reset JSON, got: %d", badResetJSONResponseRecorder.Code)
	}

	emptyRecipientResetRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/request", strings.NewReader(`{}`))
	emptyRecipientResetResponseRecorder := httptest.NewRecorder()
	handler.handlePasswordResetRequest(emptyRecipientResetResponseRecorder, emptyRecipientResetRequest)
	if emptyRecipientResetResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty recipient password reset, got: %d", emptyRecipientResetResponseRecorder.Code)
	}

	invalidPhoneResetRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/request", strings.NewReader(`{"phone":"invalid"}`))
	invalidPhoneResetResponseRecorder := httptest.NewRecorder()
	handler.handlePasswordResetRequest(invalidPhoneResetResponseRecorder, invalidPhoneResetRequest)
	if invalidPhoneResetResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone in password reset request, got: %d", invalidPhoneResetResponseRecorder.Code)
	}

	// Nil pool on valid request -> 500
	validResetRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/request", strings.NewReader(`{"email":"valid@example.com"}`))
	validResetResponseRecorder := httptest.NewRecorder()
	handler.handlePasswordResetRequest(validResetResponseRecorder, validResetRequest)
	if validResetResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool password reset request, got: %d", validResetResponseRecorder.Code)
	}

	// 6. Validation errors on Confirm
	badConfirmJSONRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/confirm", strings.NewReader(`{invalid`))
	badConfirmJSONResponseRecorder := httptest.NewRecorder()
	handler.handlePasswordResetConfirm(badConfirmJSONResponseRecorder, badConfirmJSONRequest)
	if badConfirmJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON in password reset confirm, got: %d", badConfirmJSONResponseRecorder.Code)
	}

	badConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/confirm", strings.NewReader(`{"email":"test@example.com"}`))
	badConfirmResponseRecorder := httptest.NewRecorder()
	handler.handlePasswordResetConfirm(badConfirmResponseRecorder, badConfirmRequest)
	if badConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing confirm fields, got: %d", badConfirmResponseRecorder.Code)
	}

	invalidPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/confirm", strings.NewReader(`{"phone":"invalid-phone","code":"123456","password":"ValidPassword123!"}`))
	invalidPhoneConfirmResponseRecorder := httptest.NewRecorder()
	handler.handlePasswordResetConfirm(invalidPhoneConfirmResponseRecorder, invalidPhoneConfirmRequest)
	if invalidPhoneConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone in confirm, got: %d", invalidPhoneConfirmResponseRecorder.Code)
	}

	shortConfirmPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/confirm", strings.NewReader(`{"email":"test@example.com","code":"123456","password":"short"}`))
	shortConfirmPasswordResponseRecorder := httptest.NewRecorder()
	handler.handlePasswordResetConfirm(shortConfirmPasswordResponseRecorder, shortConfirmPasswordRequest)
	if shortConfirmPasswordResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on short password in confirm, got: %d", shortConfirmPasswordResponseRecorder.Code)
	}

	// Nil pool on valid confirm -> 500
	validConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/confirm", strings.NewReader(`{"email":"test@example.com","code":"123456","password":"ValidPassword123!"}`))
	validConfirmResponseRecorder := httptest.NewRecorder()
	handler.handlePasswordResetConfirm(validConfirmResponseRecorder, validConfirmRequest)
	if validConfirmResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil pool confirm, got: %d", validConfirmResponseRecorder.Code)
	}
}
