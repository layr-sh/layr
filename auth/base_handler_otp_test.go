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
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := service.baseHandler
	configManager := service.configManager
	testKVStore := kernel.KVStore()

	// 1. OTP disabled -> 403
	disabledConfig := DefaultConfig()
	disabledConfig.EmailOTP.Enabled = false
	disabledConfig.SMSOTP.Enabled = false
	configManager.SetMemoryConfig(disabledConfig)

	disabledSendRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/send", strings.NewReader(`{"recipient":"test@example.com","purpose":"sign_in"}`))
	disabledSendResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSendOTP(disabledSendResponseRecorder, disabledSendRequest)
	if disabledSendResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when OTP disabled, got: %d", disabledSendResponseRecorder.Code)
	}

	verifyDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/verify", strings.NewReader(`{"recipient":"test@example.com","code":"123456"}`))
	verifyDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyOTP(verifyDisabledResponseRecorder, verifyDisabledRequest)
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
	configManager.SetMemoryConfig(activeConfig)
	baseHandler.emailDispatcher = NewEmailDispatcher(kernel, func() *EmailDispatcherConfig {
		emailDispatcherConfig := configManager.Get().EmailDispatcher
		return &emailDispatcherConfig
	})
	baseHandler.smsDispatcher = NewSMSDispatcher(kernel, func() *SMSDispatcherConfig {
		smsDispatcherConfig := configManager.Get().SMSDispatcher
		return &smsDispatcherConfig
	})

	// 3. Validation errors
	missingRecipientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/send", strings.NewReader(`{}`))
	missingRecipientResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSendOTP(missingRecipientResponseRecorder, missingRecipientRequest)
	if missingRecipientResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing OTP recipient, got: %d", missingRecipientResponseRecorder.Code)
	}

	missingCodeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/verify", strings.NewReader(`{"recipient":"test@example.com"}`))
	missingCodeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyOTP(missingCodeResponseRecorder, missingCodeRequest)
	if missingCodeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing OTP verify code, got: %d", missingCodeResponseRecorder.Code)
	}

	invalidPhoneSendRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/send", strings.NewReader(`{"recipient":"invalid"}`))
	invalidPhoneSendResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSendOTP(invalidPhoneSendResponseRecorder, invalidPhoneSendRequest)
	if invalidPhoneSendResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone in OTP send, got: %d", invalidPhoneSendResponseRecorder.Code)
	}

	invalidPhoneVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/verify", strings.NewReader(`{"recipient":"invalid","code":"123456"}`))
	invalidPhoneVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyOTP(invalidPhoneVerifyResponseRecorder, invalidPhoneVerifyRequest)
	if invalidPhoneVerifyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone in OTP verify, got: %d", invalidPhoneVerifyResponseRecorder.Code)
	}

	// 4. Channel specific disabled branches
	emailOnlyConfig := activeConfig
	emailOnlyConfig.EmailOTP.Enabled = true
	emailOnlyConfig.SMSOTP.Enabled = false
	configManager.SetMemoryConfig(emailOnlyConfig)

	smsDisabledSendRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/send", strings.NewReader(`{"recipient":"+15551112222"}`))
	smsDisabledSendResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSendOTP(smsDisabledSendResponseRecorder, smsDisabledSendRequest)
	if smsDisabledSendResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on SMS OTP send when SMS OTP disabled, got: %d", smsDisabledSendResponseRecorder.Code)
	}

	smsDisabledVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/verify", strings.NewReader(`{"recipient":"+15551112222","code":"123456"}`))
	smsDisabledVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyOTP(smsDisabledVerifyResponseRecorder, smsDisabledVerifyRequest)
	if smsDisabledVerifyResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on SMS OTP verify when SMS OTP disabled, got: %d", smsDisabledVerifyResponseRecorder.Code)
	}

	smsOnlyConfig := activeConfig
	smsOnlyConfig.EmailOTP.Enabled = false
	smsOnlyConfig.SMSOTP.Enabled = true
	configManager.SetMemoryConfig(smsOnlyConfig)

	emailDisabledSendRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/send", strings.NewReader(`{"recipient":"user@example.com"}`))
	emailDisabledSendResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSendOTP(emailDisabledSendResponseRecorder, emailDisabledSendRequest)
	if emailDisabledSendResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on email OTP send when email OTP disabled, got: %d", emailDisabledSendResponseRecorder.Code)
	}

	emailDisabledVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/verify", strings.NewReader(`{"recipient":"user@example.com","code":"123456"}`))
	emailDisabledVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyOTP(emailDisabledVerifyResponseRecorder, emailDisabledVerifyRequest)
	if emailDisabledVerifyResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on email OTP verify when email OTP disabled, got: %d", emailDisabledVerifyResponseRecorder.Code)
	}

	// 5. Unconfigured delivery -> 422
	unconfiguredConfig := DefaultConfig()
	unconfiguredConfig.EmailOTP.Enabled = true
	unconfiguredConfig.SMSOTP.Enabled = true
	unconfiguredConfig.EmailDispatcher.Driver = nil
	unconfiguredConfig.SMSDispatcher.Driver = nil
	configManager.SetMemoryConfig(unconfiguredConfig)

	unconfiguredEmailOTPResponseRecorder := httptest.NewRecorder()
	unconfiguredEmailOTPRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/send", strings.NewReader(`{"recipient":"user@example.com","purpose":"sign_in"}`))
	baseHandler.handleSendOTP(unconfiguredEmailOTPResponseRecorder, unconfiguredEmailOTPRequest)
	if unconfiguredEmailOTPResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on unconfigured email OTP, got: %d (%s)", unconfiguredEmailOTPResponseRecorder.Code, unconfiguredEmailOTPResponseRecorder.Body.String())
	}

	unconfiguredPhoneOTPResponseRecorder := httptest.NewRecorder()
	unconfiguredPhoneOTPRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/send", strings.NewReader(`{"recipient":"+15551112222","purpose":"sign_in"}`))
	baseHandler.handleSendOTP(unconfiguredPhoneOTPResponseRecorder, unconfiguredPhoneOTPRequest)
	if unconfiguredPhoneOTPResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on unconfigured phone OTP, got: %d (%s)", unconfiguredPhoneOTPResponseRecorder.Code, unconfiguredPhoneOTPResponseRecorder.Body.String())
	}

	// Restore configured delivery
	configManager.SetMemoryConfig(activeConfig)

	// 6. Rate limits & Cooldown
	_ = testKVStore.Set(context.Background(), "auth:cooldown:sign_in:cooldown_user@example.com", "1", 60*time.Second)
	cooldownOTPRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/send", strings.NewReader(`{"recipient":"cooldown_user@example.com","purpose":"sign_in"}`))
	cooldownOTPResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSendOTP(cooldownOTPResponseRecorder, cooldownOTPRequest)
	if cooldownOTPResponseRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on cooldown OTP request, got: %d", cooldownOTPResponseRecorder.Code)
	}

	_ = testKVStore.Set(context.Background(), "auth:ratelimit:otp:ip:203.0.113.50", "10", time.Hour)
	rateLimitedOTPRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/send", strings.NewReader(`{"recipient":"fresh_user@example.com","purpose":"sign_in"}`))
	rateLimitedOTPRequest.RemoteAddr = "203.0.113.50:1234"
	rateLimitedOTPResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSendOTP(rateLimitedOTPResponseRecorder, rateLimitedOTPRequest)
	if rateLimitedOTPResponseRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on IP rate limited OTP request, got: %d", rateLimitedOTPResponseRecorder.Code)
	}

	// 7. OTP send on broken pool -> 204 (db exec error is non-fatal for send)
	otpSignUpSendRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/send", strings.NewReader(`{"recipient":"+1234567890","purpose":"sign_up"}`))
	otpSignUpSendResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSendOTP(otpSignUpSendResponseRecorder, otpSignUpSendRequest)
	if otpSignUpSendResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on OTP sign up send, got: %d", otpSignUpSendResponseRecorder.Code)
	}

	otpSignUpVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/verify", strings.NewReader(`{"recipient":"+1234567890","code":"123456","purpose":"sign_up"}`))
	otpSignUpVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyOTP(otpSignUpVerifyResponseRecorder, otpSignUpVerifyRequest)
	if otpSignUpVerifyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on OTP sign up verify with broken pool, got: %d", otpSignUpVerifyResponseRecorder.Code)
	}

	// 8. Event Bus on broken pool -> 204
	eventOTPSendRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/otp/send", strings.NewReader(`{"recipient":"event_user@example.com"}`))
	eventOTPSendResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSendOTP(eventOTPSendResponseRecorder, eventOTPSendRequest)
	if eventOTPSendResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on event OTP send, got: %d", eventOTPSendResponseRecorder.Code)
	}
}

func TestAuthOTPThreatValidationUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := service.baseHandler
	configManager := service.configManager

	ctx := context.Background()

	// 1. Bot protection enabled in "always" mode -> missing CAPTCHA token returns 400
	botProtectionConfig := DefaultConfig()
	botProtectionConfig.EmailOTP.Enabled = true
	botProtectionConfig.Threat.BotProtection.Enabled = true
	botProtectionConfig.Threat.BotProtection.Mode = "always"
	configManager.SetMemoryConfig(botProtectionConfig)

	// OTP Send captcha check
	otpSendNoCaptchaRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/otp/send", strings.NewReader(`{"recipient":"test@example.com","purpose":"sign_in"}`))
	otpSendNoCaptchaResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSendOTP(otpSendNoCaptchaResponseRecorder, otpSendNoCaptchaRequest)
	if otpSendNoCaptchaResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on OTP send without captcha, got: %d", otpSendNoCaptchaResponseRecorder.Code)
	}

	// OTP Verify captcha check
	otpVerifyNoCaptchaRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/otp/verify", strings.NewReader(`{"recipient":"test@example.com","code":"123456"}`))
	otpVerifyNoCaptchaResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyOTP(otpVerifyNoCaptchaResponseRecorder, otpVerifyNoCaptchaRequest)
	if otpVerifyNoCaptchaResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on OTP verify without captcha, got: %d", otpVerifyNoCaptchaResponseRecorder.Code)
	}
}
