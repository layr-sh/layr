package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"layr.sh/core"
)

func TestAuthOTPHandlerUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	handler := NewHandler(nil, configManager, cryptoKeyManager)
	testKVStore := newInMemoryKVStore()
	handler.SetKVStore(testKVStore)

	// 1. OTP disabled -> 403
	disabledConfig := DefaultConfig()
	disabledConfig.EmailOTP.Enabled = false
	disabledConfig.SMSOTP.Enabled = false
	configManager.Set(disabledConfig)

	disabledSendRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/send", strings.NewReader(`{"recipient":"test@example.com","purpose":"signin"}`))
	disabledSendResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(disabledSendResponseRecorder, disabledSendRequest)
	if disabledSendResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when OTP disabled, got: %d", disabledSendResponseRecorder.Code)
	}

	verifyDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/verify", strings.NewReader(`{"recipient":"test@example.com","code":"123456"}`))
	verifyDisabledResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(verifyDisabledResponseRecorder, verifyDisabledRequest)
	if verifyDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on OTP verify when OTP disabled, got: %d", verifyDisabledResponseRecorder.Code)
	}

	// 2. Enable OTP and configure delivery
	activeConfig := DefaultConfig()
	driverWebhook := "webhook"
	activeConfig.EmailOTP.Enabled = true
	activeConfig.SMSOTP.Enabled = true
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

	// 3. Validation errors
	missingRecipientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/send", strings.NewReader(`{}`))
	missingRecipientResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(missingRecipientResponseRecorder, missingRecipientRequest)
	if missingRecipientResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing OTP recipient, got: %d", missingRecipientResponseRecorder.Code)
	}

	missingCodeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/verify", strings.NewReader(`{"recipient":"test@example.com"}`))
	missingCodeResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(missingCodeResponseRecorder, missingCodeRequest)
	if missingCodeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing OTP verify code, got: %d", missingCodeResponseRecorder.Code)
	}

	invalidPhoneSendRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/send", strings.NewReader(`{"recipient":"invalid"}`))
	invalidPhoneSendResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(invalidPhoneSendResponseRecorder, invalidPhoneSendRequest)
	if invalidPhoneSendResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone in OTP send, got: %d", invalidPhoneSendResponseRecorder.Code)
	}

	invalidPhoneVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/verify", strings.NewReader(`{"recipient":"invalid","code":"123456"}`))
	invalidPhoneVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(invalidPhoneVerifyResponseRecorder, invalidPhoneVerifyRequest)
	if invalidPhoneVerifyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone in OTP verify, got: %d", invalidPhoneVerifyResponseRecorder.Code)
	}

	// 4. Channel specific disabled branches
	emailOnlyConfig := activeConfig
	emailOnlyConfig.EmailOTP.Enabled = true
	emailOnlyConfig.SMSOTP.Enabled = false
	configManager.Set(emailOnlyConfig)

	smsDisabledSendRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/send", strings.NewReader(`{"recipient":"+15551112222"}`))
	smsDisabledSendResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(smsDisabledSendResponseRecorder, smsDisabledSendRequest)
	if smsDisabledSendResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on SMS OTP send when SMS OTP disabled, got: %d", smsDisabledSendResponseRecorder.Code)
	}

	smsDisabledVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/verify", strings.NewReader(`{"recipient":"+15551112222","code":"123456"}`))
	smsDisabledVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(smsDisabledVerifyResponseRecorder, smsDisabledVerifyRequest)
	if smsDisabledVerifyResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on SMS OTP verify when SMS OTP disabled, got: %d", smsDisabledVerifyResponseRecorder.Code)
	}

	smsOnlyConfig := activeConfig
	smsOnlyConfig.EmailOTP.Enabled = false
	smsOnlyConfig.SMSOTP.Enabled = true
	configManager.Set(smsOnlyConfig)

	emailDisabledSendRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/send", strings.NewReader(`{"recipient":"user@example.com"}`))
	emailDisabledSendResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(emailDisabledSendResponseRecorder, emailDisabledSendRequest)
	if emailDisabledSendResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on email OTP send when email OTP disabled, got: %d", emailDisabledSendResponseRecorder.Code)
	}

	emailDisabledVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/verify", strings.NewReader(`{"recipient":"user@example.com","code":"123456"}`))
	emailDisabledVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(emailDisabledVerifyResponseRecorder, emailDisabledVerifyRequest)
	if emailDisabledVerifyResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on email OTP verify when email OTP disabled, got: %d", emailDisabledVerifyResponseRecorder.Code)
	}

	// 5. Unconfigured delivery -> 422
	unconfiguredConfig := DefaultConfig()
	unconfiguredConfig.EmailOTP.Enabled = true
	unconfiguredConfig.SMSOTP.Enabled = true
	unconfiguredConfig.EmailDispatcher.Driver = nil
	unconfiguredConfig.SMSDispatcher.Driver = nil
	configManager.Set(unconfiguredConfig)
	handler.SetEmailDispatcher(nil)
	handler.SetSMSDispatcher(nil)

	unconfiguredEmailOTPResponseRecorder := httptest.NewRecorder()
	unconfiguredEmailOTPRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/send", strings.NewReader(`{"recipient":"user@example.com","purpose":"signin"}`))
	handler.handleOTPSend(unconfiguredEmailOTPResponseRecorder, unconfiguredEmailOTPRequest)
	if unconfiguredEmailOTPResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on unconfigured email OTP, got: %d (%s)", unconfiguredEmailOTPResponseRecorder.Code, unconfiguredEmailOTPResponseRecorder.Body.String())
	}

	unconfiguredPhoneOTPResponseRecorder := httptest.NewRecorder()
	unconfiguredPhoneOTPRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/send", strings.NewReader(`{"recipient":"+15551112222","purpose":"signin"}`))
	handler.handleOTPSend(unconfiguredPhoneOTPResponseRecorder, unconfiguredPhoneOTPRequest)
	if unconfiguredPhoneOTPResponseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on unconfigured phone OTP, got: %d (%s)", unconfiguredPhoneOTPResponseRecorder.Code, unconfiguredPhoneOTPResponseRecorder.Body.String())
	}

	// Restore configured delivery
	configManager.Set(activeConfig)
	handler.SetEmailDispatcher(NewEmailDispatcher(nil, func() *EmailDispatcherConfig {
		emailDispatcherConfig := activeConfig.EmailDispatcher
		return &emailDispatcherConfig
	}, nil))
	handler.SetSMSDispatcher(NewSMSDispatcher(nil, func() *SMSDispatcherConfig {
		smsDispatcherConfig := activeConfig.SMSDispatcher
		return &smsDispatcherConfig
	}, nil))

	// 6. Rate limits & Cooldown
	_ = testKVStore.Set(context.Background(), "auth:cooldown:signin:cooldown_user@example.com", "1", 60*time.Second)
	cooldownOTPRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/send", strings.NewReader(`{"recipient":"cooldown_user@example.com","purpose":"signin"}`))
	cooldownOTPResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(cooldownOTPResponseRecorder, cooldownOTPRequest)
	if cooldownOTPResponseRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on cooldown OTP request, got: %d", cooldownOTPResponseRecorder.Code)
	}

	_ = testKVStore.Set(context.Background(), "auth:ratelimit:otp:ip:203.0.113.50", "10", time.Hour)
	rateLimitedOTPRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/send", strings.NewReader(`{"recipient":"fresh_user@example.com","purpose":"signin"}`))
	rateLimitedOTPRequest.RemoteAddr = "203.0.113.50:1234"
	rateLimitedOTPResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(rateLimitedOTPResponseRecorder, rateLimitedOTPRequest)
	if rateLimitedOTPResponseRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on IP rate limited OTP request, got: %d", rateLimitedOTPResponseRecorder.Code)
	}

	// 7. Nil database pool -> 500
	otpSignupSendRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/send", strings.NewReader(`{"recipient":"+1234567890","purpose":"signup"}`))
	otpSignupSendResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(otpSignupSendResponseRecorder, otpSignupSendRequest)
	if otpSignupSendResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on OTP signup send nil pool, got: %d", otpSignupSendResponseRecorder.Code)
	}

	otpSignupVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/verify", strings.NewReader(`{"recipient":"+1234567890","code":"123456","purpose":"signup"}`))
	otpSignupVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(otpSignupVerifyResponseRecorder, otpSignupVerifyRequest)
	if otpSignupVerifyResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on OTP signup verify nil pool, got: %d", otpSignupVerifyResponseRecorder.Code)
	}

	// 8. Event Bus on Nil database pool -> 500
	eventBus := core.NewEventBus(nil, cryptoKeyManager)
	handler.SetEventBus(eventBus)
	eventOTPSendRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/otp/send", strings.NewReader(`{"recipient":"event_user@example.com"}`))
	eventOTPSendResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(eventOTPSendResponseRecorder, eventOTPSendRequest)
	if eventOTPSendResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on event OTP send nil pool, got: %d", eventOTPSendResponseRecorder.Code)
	}
}
