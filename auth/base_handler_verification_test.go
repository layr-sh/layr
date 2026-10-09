package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/core"
)

func TestAuthBaseHandlerVerificationUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := service.baseHandler
	jwtSigner := kernel.JWTSigner()

	validToken := jwtSigner.GenerateAccessToken(core.JWTClaims{
		Subject:     "user-unit-123",
		Email:       "unit@example.com",
		Role:        "user",
		IsAnonymous: false,
	}, 3600)

	authedCtx := core.WithAuthContext(context.Background(), core.AuthContext{
		UserID: "user-unit-123",
		JWT: core.JWTClaims{
			Subject:     "user-unit-123",
			Email:       "unit@example.com",
			Role:        "user",
			IsAnonymous: false,
		},
	})

	driverWebhook := "webhook"
	mockEmailDispatcher := NewEmailDispatcher(kernel, func() *EmailDispatcherConfig {
		return &EmailDispatcherConfig{
			Driver:  &driverWebhook,
			Webhook: EmailDispatcherWebhookConfig{URL: "http://localhost"},
		}
	})
	mockSMSDispatcher := NewSMSDispatcher(kernel, func() *SMSDispatcherConfig {
		return &SMSDispatcherConfig{
			Driver:  &driverWebhook,
			Webhook: SMSDispatcherWebhookConfig{URL: "http://localhost"},
		}
	})

	// 1. Email & Phone Verification Unified Request Tests
	missingEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/user/email/verification/request", strings.NewReader(`{}`))
	missingEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestEmailVerification(missingEmailResponseRecorder, missingEmailRequest)
	if missingEmailResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing email, got: %d", missingEmailResponseRecorder.Code)
	}

	// Unconfigured email dispatcher readiness check
	unconfiguredEmailDispatcher := NewEmailDispatcher(kernel, func() *EmailDispatcherConfig { return &EmailDispatcherConfig{} })
	baseHandler.emailDispatcher = unconfiguredEmailDispatcher
	unconfiguredEmailRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPost, "/v1/auth/user/email/verification/request", strings.NewReader(`{"email":"new@example.com"}`))
	unconfiguredEmailRequest.Header.Set("Authorization", "Bearer "+validToken)
	unconfiguredEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestEmailVerification(unconfiguredEmailResponseRecorder, unconfiguredEmailRequest)
	if unconfiguredEmailResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on unconfigured email delivery, got: %d", unconfiguredEmailResponseRecorder.Code)
	}

	missingPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/user/phone/verification/request", strings.NewReader(`{}`))
	missingPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	missingPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestPhoneVerification(missingPhoneResponseRecorder, missingPhoneRequest)
	if missingPhoneResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing phone, got: %d", missingPhoneResponseRecorder.Code)
	}

	invalidPhoneVerificationRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPost, "/v1/auth/user/phone/verification/request", strings.NewReader(`{"phone":"12345"}`))
	invalidPhoneVerificationRequest.Header.Set("Authorization", "Bearer "+validToken)
	invalidPhoneVerificationResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestPhoneVerification(invalidPhoneVerificationResponseRecorder, invalidPhoneVerificationRequest)
	if invalidPhoneVerificationResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone format, got: %d", invalidPhoneVerificationResponseRecorder.Code)
	}

	unconfiguredSMSDispatcher := NewSMSDispatcher(kernel, func() *SMSDispatcherConfig { return &SMSDispatcherConfig{} })
	baseHandler.smsDispatcher = unconfiguredSMSDispatcher
	unconfiguredPhoneRequest := httptest.NewRequestWithContext(authedCtx, http.MethodPost, "/v1/auth/user/phone/verification/request", strings.NewReader(`{"phone":"+15551234567"}`))
	unconfiguredPhoneRequest.Header.Set("Authorization", "Bearer "+validToken)
	unconfiguredPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestPhoneVerification(unconfiguredPhoneResponseRecorder, unconfiguredPhoneRequest)
	if unconfiguredPhoneResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on unconfigured SMS delivery, got: %d", unconfiguredPhoneResponseRecorder.Code)
	}

	// Restore dispatchers
	baseHandler.emailDispatcher = mockEmailDispatcher
	baseHandler.smsDispatcher = mockSMSDispatcher

	emailVerificationRequestRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/user/email/verification/request", strings.NewReader(`{"email":"test@example.com"}`))
	emailVerificationRequestResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestEmailVerification(emailVerificationRequestResponseRecorder, emailVerificationRequestRequest)
	if emailVerificationRequestResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on email verify request with non-existent user, got: %d", emailVerificationRequestResponseRecorder.Code)
	}

	phoneVerificationRequestRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/user/phone/verification/request", strings.NewReader(`{"phone":"+15551234567"}`))
	phoneVerificationRequestResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestPhoneVerification(phoneVerificationRequestResponseRecorder, phoneVerificationRequestRequest)
	if phoneVerificationRequestResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on phone verify request with non-existent user, got: %d", phoneVerificationRequestResponseRecorder.Code)
	}

	// 2. Verification Confirm Unit Tests
	badJSONEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/user/email/verification/confirm", strings.NewReader(`{bad`))
	badJSONEmailConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmEmailVerification(badJSONEmailConfirmResponseRecorder, badJSONEmailConfirmRequest)
	if badJSONEmailConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON email confirm, got: %d", badJSONEmailConfirmResponseRecorder.Code)
	}

	missingCodeEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/user/email/verification/confirm", strings.NewReader(`{"email":"user@example.com"}`))
	missingCodeEmailConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmEmailVerification(missingCodeEmailConfirmResponseRecorder, missingCodeEmailConfirmRequest)
	if missingCodeEmailConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing code email confirm, got: %d", missingCodeEmailConfirmResponseRecorder.Code)
	}

	validEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/user/email/verification/confirm", strings.NewReader(`{"email":"user@example.com","code":"123456"}`))
	validEmailConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmEmailVerification(validEmailConfirmResponseRecorder, validEmailConfirmRequest)
	if validEmailConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on email confirm with non-existent code, got: %d", validEmailConfirmResponseRecorder.Code)
	}

	badJSONPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/user/phone/verification/confirm", strings.NewReader(`{bad`))
	badJSONPhoneConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPhoneVerification(badJSONPhoneConfirmResponseRecorder, badJSONPhoneConfirmRequest)
	if badJSONPhoneConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON phone confirm, got: %d", badJSONPhoneConfirmResponseRecorder.Code)
	}

	missingCodePhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"phone":"+15551234567"}`))
	missingCodePhoneConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPhoneVerification(missingCodePhoneConfirmResponseRecorder, missingCodePhoneConfirmRequest)
	if missingCodePhoneConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing code phone confirm, got: %d", missingCodePhoneConfirmResponseRecorder.Code)
	}

	invalidPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"phone":"bad-phone","code":"123456"}`))
	invalidPhoneConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPhoneVerification(invalidPhoneConfirmResponseRecorder, invalidPhoneConfirmRequest)
	if invalidPhoneConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone format confirm, got: %d", invalidPhoneConfirmResponseRecorder.Code)
	}

	validPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"phone":"+15551234567","code":"123456"}`))
	validPhoneConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPhoneVerification(validPhoneConfirmResponseRecorder, validPhoneConfirmRequest)
	if validPhoneConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on phone confirm with non-existent code, got: %d", validPhoneConfirmResponseRecorder.Code)
	}

	// 3. Claims-based email and phone verification confirm
	claimsEmailCtx := core.WithAuthContext(context.Background(), core.AuthContext{
		UserID: "user-claims-email",
		JWT: core.JWTClaims{
			Subject:     "user-claims-email",
			Email:       "claims.email@example.com",
			Role:        "authenticated",
			IsAnonymous: false,
		},
	})
	claimsEmailRequest := httptest.NewRequestWithContext(claimsEmailCtx, http.MethodPost, "/v1/auth/user/email/verification/confirm", strings.NewReader(`{"code":"123456"}`))
	claimsEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmEmailVerification(claimsEmailResponseRecorder, claimsEmailRequest)
	if claimsEmailResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on claims email confirm with non-existent code, got: %d", claimsEmailResponseRecorder.Code)
	}

	claimsPhoneCtx := core.WithAuthContext(context.Background(), core.AuthContext{
		UserID: "user-claims-phone",
		JWT: core.JWTClaims{
			Subject:     "user-claims-phone",
			Phone:       "+15551234567",
			Role:        "authenticated",
			IsAnonymous: false,
		},
	})
	claimsPhoneRequest := httptest.NewRequestWithContext(claimsPhoneCtx, http.MethodPost, "/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"code":"123456"}`))
	claimsPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPhoneVerification(claimsPhoneResponseRecorder, claimsPhoneRequest)
	if claimsPhoneResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on claims phone confirm with non-existent code, got: %d", claimsPhoneResponseRecorder.Code)
	}
}
