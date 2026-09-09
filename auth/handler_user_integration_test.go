package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"layr.sh/auth/jwt"
	"layr.sh/auth/otp"
	"layr.sh/core"
)

func TestAuthUserVerificationIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanupDatabase := setupTestDatabase(t)
	defer cleanupDatabase()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load initial auth config: %v", err)
	}

	webhookEventBus := core.NewWebhookEventBus(db, cryptoKeyManager)
	defer webhookEventBus.Close()
	testKVStore := newInMemoryKVStore()

	handler := NewHandler(db, configManager, cryptoKeyManager)
	handler.SetWebhookEventBus(webhookEventBus)
	handler.SetKVStore(testKVStore)

	// Capture emitted events
	emittedEvents := make([]core.WebhookEventEnvelope, 0)
	webhookEventBus.Subscribe("auth.user.email_verified", func(eventCtx context.Context, webhookEventEnvelope core.WebhookEventEnvelope) error {
		emittedEvents = append(emittedEvents, webhookEventEnvelope)
		return nil
	})
	webhookEventBus.Subscribe("auth.user.phone_verified", func(eventCtx context.Context, webhookEventEnvelope core.WebhookEventEnvelope) error {
		emittedEvents = append(emittedEvents, webhookEventEnvelope)
		return nil
	})

	// Create test user in layr_auth.users
	testUserID := "01918a24-1111-7000-8000-000000000001"
	testEmail := "verifyuser@example.com"
	testPhone := "+15554443333"

	insertUserQuery := `
		INSERT INTO layr_auth.users (id, email, phone, role, email_verified_at, phone_verified_at, created_at, last_updated_at)
		VALUES ($1, $2, $3, 'user', NULL, NULL, clock_timestamp(), clock_timestamp())
	`
	if _, err := db.Exec(ctx, insertUserQuery, testUserID, testEmail, testPhone); err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	// 1. Email Verification Request - Unconfigured Dispatcher Failure (HTTP 422)
	requestBodyBytes, _ := json.Marshal(map[string]any{"email": testEmail})
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/request", bytes.NewReader(requestBodyBytes))
	responseRecorder := httptest.NewRecorder()
	handler.handleUserEmailVerificationRequest(responseRecorder, request)

	if responseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity when email unconfigured, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !strings.Contains(responseRecorder.Body.String(), "LAYR_AUTH_EMAIL_UNAVAILABLE") {
		t.Fatalf("expected LAYR_AUTH_EMAIL_UNAVAILABLE, got: %s", responseRecorder.Body.String())
	}

	// 2. Configure mock Email and SMS Webhook Servers
	var dispatchedEmailSubject string
	var dispatchedEmailCode string
	emailWebhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(request.Body).Decode(&payload)
		if subjectValue, ok := payload["subject"].(string); ok {
			dispatchedEmailSubject = subjectValue
		}
		if codeValue, ok := payload["code"].(string); ok {
			dispatchedEmailCode = codeValue
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer emailWebhookServer.Close()

	var dispatchedSMSText string
	var dispatchedSMSCode string
	smsWebhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(request.Body).Decode(&payload)
		if textValue, ok := payload["text"].(string); ok {
			dispatchedSMSText = textValue
		}
		if codeValue, ok := payload["code"].(string); ok {
			dispatchedSMSCode = codeValue
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer smsWebhookServer.Close()

	// Update auth config with configured email and SMS
	driverWebhook := "webhook"
	activeConfig := configManager.Get()
	activeConfig.EmailDispatcher = EmailDispatcherConfig{
		Driver: &driverWebhook,
		Webhook: EmailDispatcherWebhookConfig{
			URL: emailWebhookServer.URL,
		},
	}
	activeConfig.SMSDispatcher = SMSDispatcherConfig{
		Driver: &driverWebhook,
		Webhook: SMSDispatcherWebhookConfig{
			URL: smsWebhookServer.URL,
		},
	}
	configManager.Set(activeConfig)

	// 3. Email Verification Request - Success
	request = httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/user/email/verification/request", bytes.NewReader(requestBodyBytes))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserEmailVerificationRequest(responseRecorder, request)
	if responseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on valid email verification request, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}

	if dispatchedEmailSubject == "" || dispatchedEmailCode == "" {
		t.Fatalf("expected email dispatcher to receive webhook dispatch with subject and code")
	}

	// 4. Phone Verification Request - Success
	phoneBodyBytes, _ := json.Marshal(map[string]any{"phone": testPhone})
	phoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/request", bytes.NewReader(phoneBodyBytes))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserPhoneVerificationRequest(responseRecorder, phoneRequest)
	if responseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on valid phone verification request, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}

	if dispatchedSMSText == "" || dispatchedSMSCode == "" {
		t.Fatalf("expected SMS dispatcher to receive webhook dispatch with text and code")
	}

	// 5. Email Verification Confirm - Success (Unauthenticated Caller)
	confirmEmailPayload, _ := json.Marshal(map[string]any{
		"email": testEmail,
		"code":  dispatchedEmailCode,
	})
	confirmEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", bytes.NewReader(confirmEmailPayload))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(responseRecorder, confirmEmailRequest)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on valid email verification confirm, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}

	// Verify database was updated
	var emailVerifiedAt *time.Time
	err := db.QueryRow(ctx, "SELECT email_verified_at FROM layr_auth.users WHERE id = $1", testUserID).Scan(&emailVerifiedAt)
	if err != nil || emailVerifiedAt == nil {
		t.Fatalf("expected user email_verified_at to be populated in database: %v", err)
	}

	// 6. Phone Verification Confirm - Success (Unauthenticated Caller)
	confirmPhonePayload, _ := json.Marshal(map[string]any{
		"phone": testPhone,
		"code":  dispatchedSMSCode,
	})
	confirmPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", bytes.NewReader(confirmPhonePayload))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(responseRecorder, confirmPhoneRequest)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on valid phone verification confirm, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}

	var phoneVerifiedAt *time.Time
	err = db.QueryRow(ctx, "SELECT phone_verified_at FROM layr_auth.users WHERE id = $1", testUserID).Scan(&phoneVerifiedAt)
	if err != nil || phoneVerifiedAt == nil {
		t.Fatalf("expected user phone_verified_at to be populated in database: %v", err)
	}

	// 7. Requesting verification when already verified returns 200 already_verified
	request = httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/user/email/verification/request", bytes.NewReader(requestBodyBytes))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserEmailVerificationRequest(responseRecorder, request)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK when email already verified, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !strings.Contains(responseRecorder.Body.String(), "already_verified") {
		t.Fatalf("expected status already_verified in response, got: %s", responseRecorder.Body.String())
	}

	phoneRequest = httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/user/phone/verification/request", bytes.NewReader(phoneBodyBytes))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserPhoneVerificationRequest(responseRecorder, phoneRequest)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK when phone already verified, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !strings.Contains(responseRecorder.Body.String(), "already_verified") {
		t.Fatalf("expected status already_verified in response, got: %s", responseRecorder.Body.String())
	}

	// 8. Authenticated Caller omitting explicit email/phone (uses Claims)
	userAccessToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     testUserID,
		Email:       testEmail,
		Phone:       testPhone,
		Role:        "user",
		IsAnonymous: false,
	}, 3600)

	authenticatedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/request", strings.NewReader(`{}`))
	authenticatedRequest.Header.Set("Authorization", "Bearer "+userAccessToken)
	responseRecorder = httptest.NewRecorder()
	handler.handleUserEmailVerificationRequest(responseRecorder, authenticatedRequest)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for already verified authenticated email request, got: %d", responseRecorder.Code)
	}

	authenticatedPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/request", strings.NewReader(`{}`))
	authenticatedPhoneRequest.Header.Set("Authorization", "Bearer "+userAccessToken)
	responseRecorder = httptest.NewRecorder()
	handler.handleUserPhoneVerificationRequest(responseRecorder, authenticatedPhoneRequest)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for already verified authenticated phone request, got: %d", responseRecorder.Code)
	}

	// 9. Confirm verification with authenticated caller extracts session and issues session response
	anonUserID := "01918a24-9999-7000-8000-000000000009"
	anonEmail := "anonconverted@example.com"
	anonPhone := "+15558887777"
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, email, phone, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, NULL, NULL, 'authenticated', true, clock_timestamp(), clock_timestamp())
	`, anonUserID)

	anonAccessToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     anonUserID,
		Email:       "",
		Role:        "authenticated",
		IsAnonymous: true,
	}, 3600)

	// Insert OTPs for anon verification
	anonEmailCode := "987654"
	anonEmailHash := otp.HashCode(anonEmailCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'email_verification', 0, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, anonEmail, anonEmailHash)

	anonPhoneCode := "456789"
	anonPhoneHash := otp.HashCode(anonPhoneCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'phone_verification', 0, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, anonPhone, anonPhoneHash)

	tokenEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(fmt.Sprintf(`{"email":"%s","code":"%s"}`, anonEmail, anonEmailCode)))
	tokenEmailConfirmRequest.Header.Set("Authorization", "Bearer "+anonAccessToken)
	responseRecorder = httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(responseRecorder, tokenEmailConfirmRequest)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on authenticated email confirm, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}
	var sessionResponse SessionResponse
	if decodeErr := json.NewDecoder(responseRecorder.Body).Decode(&sessionResponse); decodeErr != nil || sessionResponse.AccessToken == "" {
		t.Fatalf("expected valid SessionResponse issued on authenticated confirm: %v", decodeErr)
	}

	tokenPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(fmt.Sprintf(`{"phone":"%s","code":"%s"}`, anonPhone, anonPhoneCode)))
	tokenPhoneConfirmRequest.Header.Set("Authorization", "Bearer "+anonAccessToken)
	responseRecorder = httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(responseRecorder, tokenPhoneConfirmRequest)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on authenticated phone confirm, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}

	// 10. Invalid JSON, Bad Phone formats, Missing codes
	invalidJSONEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(`{invalid`))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(responseRecorder, invalidJSONEmailRequest)
	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON email confirm, got: %d", responseRecorder.Code)
	}

	emptyEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(`{"email":""}`))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(responseRecorder, emptyEmailConfirmRequest)
	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty email confirm, got: %d", responseRecorder.Code)
	}

	invalidJSONPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(`{invalid`))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(responseRecorder, invalidJSONPhoneRequest)
	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON phone confirm, got: %d", responseRecorder.Code)
	}

	emptyPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"phone":""}`))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(responseRecorder, emptyPhoneConfirmRequest)
	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty phone confirm, got: %d", responseRecorder.Code)
	}

	emptyPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/request", strings.NewReader(`{"phone":""}`))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserPhoneVerificationRequest(responseRecorder, emptyPhoneRequest)
	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty phone verify request, got: %d", responseRecorder.Code)
	}

	nonExistentPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/request", strings.NewReader(`{"phone":"+15550001111"}`))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserPhoneVerificationRequest(responseRecorder, nonExistentPhoneRequest)
	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on unauthenticated non-existent phone request, got: %d", responseRecorder.Code)
	}

	// 11. Non-existent OTP code verification -> 400
	nonExistentOTPConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(`{"email":"nobody@example.com","code":"123456"}`))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(responseRecorder, nonExistentOTPConfirmRequest)
	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on non-existent OTP confirm, got: %d", responseRecorder.Code)
	}

	nonExistentPhoneOTPConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"phone":"+15550001111","code":"123456"}`))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(responseRecorder, nonExistentPhoneOTPConfirmRequest)
	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on non-existent phone OTP confirm, got: %d", responseRecorder.Code)
	}

	// 12. Expired OTP code -> 400
	expiredCode := "111222"
	expiredHash := otp.HashCode(expiredCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ('expired@example.com', $1, 'email_verification', 0, clock_timestamp() - interval '10 minutes', clock_timestamp() - interval '15 minutes')
	`, expiredHash)
	expiredEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(fmt.Sprintf(`{"email":"expired@example.com","code":"%s"}`, expiredCode)))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(responseRecorder, expiredEmailRequest)
	if responseRecorder.Code != http.StatusBadRequest || !strings.Contains(responseRecorder.Body.String(), "expired") {
		t.Fatalf("expected 400 with expired message on expired OTP, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}

	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ('+15559992222', $1, 'phone_verification', 0, clock_timestamp() - interval '10 minutes', clock_timestamp() - interval '15 minutes')
	`, expiredHash)
	expiredPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(fmt.Sprintf(`{"phone":"+15559992222","code":"%s"}`, expiredCode)))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(responseRecorder, expiredPhoneRequest)
	if responseRecorder.Code != http.StatusBadRequest || !strings.Contains(responseRecorder.Body.String(), "expired") {
		t.Fatalf("expected 400 with expired message on expired phone OTP, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}

	// 13. Max attempts exceeded -> 400
	maxAttemptsCode := "333444"
	maxAttemptsHash := otp.HashCode(maxAttemptsCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ('maxattempts@example.com', $1, 'email_verification', 5, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, maxAttemptsHash)
	maxAttemptsEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(fmt.Sprintf(`{"email":"maxattempts@example.com","code":"%s"}`, maxAttemptsCode)))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(responseRecorder, maxAttemptsEmailRequest)
	if responseRecorder.Code != http.StatusBadRequest || !strings.Contains(responseRecorder.Body.String(), "Maximum attempts exceeded") {
		t.Fatalf("expected 400 with Maximum attempts exceeded, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}

	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ('+15559993333', $1, 'phone_verification', 5, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, maxAttemptsHash)
	maxAttemptsPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(fmt.Sprintf(`{"phone":"+15559993333","code":"%s"}`, maxAttemptsCode)))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(responseRecorder, maxAttemptsPhoneRequest)
	if responseRecorder.Code != http.StatusBadRequest || !strings.Contains(responseRecorder.Body.String(), "Maximum attempts exceeded") {
		t.Fatalf("expected 400 with Maximum attempts exceeded, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}

	// 14. Wrong code increments attempt counter -> 400
	wrongCode := "555666"
	wrongCodeHash := otp.HashCode(wrongCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ('+15559994444', $1, 'phone_verification', 1, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, wrongCodeHash)
	wrongCodePhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(`{"phone":"+15559994444","code":"999999"}`))
	responseRecorder = httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(responseRecorder, wrongCodePhoneRequest)
	if responseRecorder.Code != http.StatusBadRequest || !strings.Contains(responseRecorder.Body.String(), "Invalid verification code") {
		t.Fatalf("expected 400 on invalid code, got: %d (%s)", responseRecorder.Code, responseRecorder.Body.String())
	}
	var attemptsAfter int
	_ = db.QueryRow(ctx, "SELECT attempts FROM layr_auth.otps WHERE recipient = '+15559994444'").Scan(&attemptsAfter)
	if attemptsAfter != 2 {
		t.Fatalf("expected attempts to increment to 2, got: %d", attemptsAfter)
	}

	// 15. Unauthenticated non-existent email verification request -> 404
	nonExistentEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/request", strings.NewReader(`{"email":"nonexistent.user@example.com"}`))
	nonExistentEmailResponseRecorder := httptest.NewRecorder()
	handler.handleUserEmailVerificationRequest(nonExistentEmailResponseRecorder, nonExistentEmailRequest)
	if nonExistentEmailResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on unauthenticated non-existent email request, got: %d", nonExistentEmailResponseRecorder.Code)
	}

	// 16. Wrong code for email verification increments attempts and returns 400
	wrongEmailCode := "777888"
	wrongEmailHash := otp.HashCode(wrongEmailCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ('wrongcode@example.com', $1, 'email_verification', 1, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, wrongEmailHash)
	wrongCodeEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(`{"email":"wrongcode@example.com","code":"000000"}`))
	wrongCodeEmailResponseRecorder := httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(wrongCodeEmailResponseRecorder, wrongCodeEmailRequest)
	if wrongCodeEmailResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(wrongCodeEmailResponseRecorder.Body.String(), "Invalid verification code") {
		t.Fatalf("expected 400 on invalid email verification code, got: %d (%s)", wrongCodeEmailResponseRecorder.Code, wrongCodeEmailResponseRecorder.Body.String())
	}
	var emailAttemptsAfter int
	_ = db.QueryRow(ctx, "SELECT attempts FROM layr_auth.otps WHERE recipient = 'wrongcode@example.com'").Scan(&emailAttemptsAfter)
	if emailAttemptsAfter != 2 {
		t.Fatalf("expected email attempts to increment to 2, got: %d", emailAttemptsAfter)
	}

	// 17. Unauthenticated email confirm for orphan recipient (no user row) -> 500
	orphanEmail := "orphan.email@example.com"
	orphanEmailCode := "654987"
	orphanEmailHash := otp.HashCode(orphanEmailCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'email_verification', 0, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, orphanEmail, orphanEmailHash)
	orphanEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", strings.NewReader(fmt.Sprintf(`{"email":"%s","code":"%s"}`, orphanEmail, orphanEmailCode)))
	orphanEmailConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(orphanEmailConfirmResponseRecorder, orphanEmailConfirmRequest)
	if orphanEmailConfirmResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on unauthenticated email verify confirm for non-existent user, got: %d", orphanEmailConfirmResponseRecorder.Code)
	}

	// 18. Unauthenticated phone confirm for orphan recipient (no user row) -> 500
	orphanPhone := "+15559990022"
	orphanPhoneCode := "987654"
	orphanPhoneHash := otp.HashCode(orphanPhoneCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'phone_verification', 0, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, orphanPhone, orphanPhoneHash)
	orphanPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(fmt.Sprintf(`{"phone":"%s","code":"%s"}`, orphanPhone, orphanPhoneCode)))
	orphanPhoneConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(orphanPhoneConfirmResponseRecorder, orphanPhoneConfirmRequest)
	if orphanPhoneConfirmResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on unauthenticated phone verify confirm for non-existent user, got: %d", orphanPhoneConfirmResponseRecorder.Code)
	}

	// 19. Anonymous user phone verification confirmation converts user and publishes event
	anonPhoneUserID := "01918a24-9999-7000-8000-000000000088"
	anonPhoneRecipient := "+15554449988"
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, 'authenticated', true, clock_timestamp(), clock_timestamp())
	`, anonPhoneUserID)
	anonPhoneToken, err := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     anonPhoneUserID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 3600)
	if err != nil {
		t.Fatalf("failed to generate anon phone token: %v", err)
	}

	anonPhoneConfirmCode := "123789"
	anonPhoneConfirmHash := otp.HashCode(anonPhoneConfirmCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'phone_verification', 0, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, anonPhoneRecipient, anonPhoneConfirmHash)

	anonPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", strings.NewReader(fmt.Sprintf(`{"phone":"%s","code":"%s"}`, anonPhoneRecipient, anonPhoneConfirmCode)))
	anonPhoneConfirmRequest.Header.Set("Authorization", "Bearer "+anonPhoneToken)
	anonPhoneConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(anonPhoneConfirmResponseRecorder, anonPhoneConfirmRequest)
	if anonPhoneConfirmResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on anonymous phone verification confirm, got: %d (%s)", anonPhoneConfirmResponseRecorder.Code, anonPhoneConfirmResponseRecorder.Body.String())
	}
}

func TestAuthUserSelfServiceIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanupDatabase := setupTestDatabase(t)
	defer cleanupDatabase()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load auth configuration: %v", err)
	}

	emailWebhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer emailWebhookServer.Close()

	smsWebhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer smsWebhookServer.Close()

	driverWebhook := "webhook"
	activeConfig := configManager.Get()
	activeConfig.EmailDispatcher = EmailDispatcherConfig{
		Driver: &driverWebhook,
		Webhook: EmailDispatcherWebhookConfig{
			URL: emailWebhookServer.URL,
		},
	}
	activeConfig.SMSDispatcher = SMSDispatcherConfig{
		Driver: &driverWebhook,
		Webhook: SMSDispatcherWebhookConfig{
			URL: smsWebhookServer.URL,
		},
	}
	configManager.Set(activeConfig)

	testKVStore := newInMemoryKVStore()
	webhookEventBus := core.NewWebhookEventBus(db, cryptoKeyManager)
	defer webhookEventBus.Close()
	serviceAccountManager := core.NewServiceAccountManager(db)

	handler := NewHandler(db, configManager, cryptoKeyManager)
	handler.SetKVStore(testKVStore)
	handler.SetWebhookEventBus(webhookEventBus)
	handler.SetServiceAccountManager(serviceAccountManager)

	userEmail := "selfservice.user@example.com"
	originalPassword := "InitialSecurePassword123!"

	// 1. Seed user directly in database
	userID := "01918a24-1111-7000-8000-000000000010"
	hashedPassword, _ := handler.hasher.Hash(originalPassword)
	propertiesJSON, _ := json.Marshal(map[string]any{
		"company": "Layr Inc",
		"tier":    "starter",
	})
	_, err := db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, email, password_hash, role, properties, is_anonymous, created_at, last_updated_at)
		VALUES ($1, $2, $3, 'user', $4, false, clock_timestamp(), clock_timestamp())
	`, userID, userEmail, hashedPassword, propertiesJSON)
	if err != nil {
		t.Fatalf("failed to insert initial user: %v", err)
	}

	accessToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     userID,
		Email:       userEmail,
		Role:        "user",
		IsAnonymous: false,
	}, 3600)

	// 2. GET /api/v1/auth/user - Inspect user profile
	profileRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	profileRequest.Header.Set("Authorization", "Bearer "+accessToken)
	profileResponseRecorder := httptest.NewRecorder()
	handler.handleGetUser(profileResponseRecorder, profileRequest)
	if profileResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get user profile, got: %d (%s)", profileResponseRecorder.Code, profileResponseRecorder.Body.String())
	}

	var userResponse UserResponse
	if decodeErr := json.NewDecoder(profileResponseRecorder.Body).Decode(&userResponse); decodeErr != nil {
		t.Fatalf("failed to decode user profile: %v", decodeErr)
	}
	if userResponse.Email == nil || *userResponse.Email != userEmail {
		t.Fatalf("expected email %s, got: %v", userEmail, userResponse.Email)
	}
	if userResponse.Properties["company"] != "Layr Inc" || userResponse.Properties["tier"] != "starter" {
		t.Fatalf("expected custom properties, got: %+v", userResponse.Properties)
	}
	if userResponse.MFAEnabled {
		t.Fatalf("expected MFAEnabled to be false initially")
	}

	// 3. User with enrolled MFA Totp
	_, _ = db.Exec(ctx, `
		UPDATE layr_auth.users 
		SET properties = properties || '{"mfa_secret_enc": "enc_totp_secret", "mfa_pending": false}'::jsonb
		WHERE id = $1
	`, userID)
	mfaRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	mfaRequest.Header.Set("Authorization", "Bearer "+accessToken)
	mfaResponseRecorder := httptest.NewRecorder()
	handler.handleGetUser(mfaResponseRecorder, mfaRequest)
	var mfaUserResponse UserResponse
	_ = json.NewDecoder(mfaResponseRecorder.Body).Decode(&mfaUserResponse)
	if !mfaUserResponse.MFAEnabled {
		t.Fatalf("expected MFAEnabled to be true after TOTP enrollment")
	}
	if _, exists := mfaUserResponse.Properties["mfa_secret_enc"]; exists {
		t.Fatalf("expected sensitive property mfa_secret_enc to be stripped from profile")
	}

	// 4. PATCH /api/v1/auth/user/properties - Merge custom properties
	patchPayload, _ := json.Marshal(UpdateUserPropertiesRequest{
		Properties: map[string]any{
			"theme":          "nord",
			"tier":           "enterprise",
			"mfa_secret_enc": "attacker_override_attempt",
		},
	})
	patchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/properties", bytes.NewReader(patchPayload))
	patchRequest.Header.Set("Authorization", "Bearer "+accessToken)
	patchResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserProperties(patchResponseRecorder, patchRequest)
	if patchResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on user properties update, got: %d (%s)", patchResponseRecorder.Code, patchResponseRecorder.Body.String())
	}

	var updateUserPropertiesResponse UpdateUserPropertiesResponse
	_ = json.NewDecoder(patchResponseRecorder.Body).Decode(&updateUserPropertiesResponse)
	if updateUserPropertiesResponse.Properties["theme"] != "nord" || updateUserPropertiesResponse.Properties["tier"] != "enterprise" {
		t.Fatalf("expected updated properties in response, got: %+v", updateUserPropertiesResponse.Properties)
	}
	if _, exists := updateUserPropertiesResponse.Properties["mfa_secret_enc"]; exists {
		t.Fatalf("expected mfa_secret_enc to be stripped from sanitized response")
	}

	// 5. PATCH /api/v1/auth/user/password - Validations
	// Wrong current password -> 400
	wrongPasswordPayload, _ := json.Marshal(UpdateUserPasswordRequest{
		CurrentPassword: "WrongPassword999!",
		NewPassword:     "BrandNewPassword123!",
	})
	wrongPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/password", bytes.NewReader(wrongPasswordPayload))
	wrongPasswordRequest.Header.Set("Authorization", "Bearer "+accessToken)
	wrongPasswordResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPassword(wrongPasswordResponseRecorder, wrongPasswordRequest)
	if wrongPasswordResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on wrong current password, got: %d (%s)", wrongPasswordResponseRecorder.Code, wrongPasswordResponseRecorder.Body.String())
	}

	// Missing current password -> 400
	missingCurrentPasswordPayload, _ := json.Marshal(UpdateUserPasswordRequest{
		NewPassword: "BrandNewPassword123!",
	})
	missingCurrentPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/password", bytes.NewReader(missingCurrentPasswordPayload))
	missingCurrentPasswordRequest.Header.Set("Authorization", "Bearer "+accessToken)
	missingCurrentPasswordResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPassword(missingCurrentPasswordResponseRecorder, missingCurrentPasswordRequest)
	if missingCurrentPasswordResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing current password, got: %d (%s)", missingCurrentPasswordResponseRecorder.Code, missingCurrentPasswordResponseRecorder.Body.String())
	}

	// Weak new password (below min length) -> 400
	weakPasswordPayload, _ := json.Marshal(UpdateUserPasswordRequest{
		CurrentPassword: originalPassword,
		NewPassword:     "short",
	})
	weakPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/password", bytes.NewReader(weakPasswordPayload))
	weakPasswordRequest.Header.Set("Authorization", "Bearer "+accessToken)
	weakPasswordResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPassword(weakPasswordResponseRecorder, weakPasswordRequest)
	if weakPasswordResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on password shorter than min length, got: %d (%s)", weakPasswordResponseRecorder.Code, weakPasswordResponseRecorder.Body.String())
	}

	// Successful password update -> 204
	newValidPassword := "UpdatedSecurePassword456!@"
	validPasswordPayload, _ := json.Marshal(UpdateUserPasswordRequest{
		CurrentPassword: originalPassword,
		NewPassword:     newValidPassword,
	})
	validPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/password", bytes.NewReader(validPasswordPayload))
	validPasswordRequest.Header.Set("Authorization", "Bearer "+accessToken)
	validPasswordResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPassword(validPasswordResponseRecorder, validPasswordRequest)
	if validPasswordResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on password update, got: %d (%s)", validPasswordResponseRecorder.Code, validPasswordResponseRecorder.Body.String())
	}

	// Verify updated password hash matches
	var updatedHash string
	_ = db.QueryRow(ctx, "SELECT password_hash FROM layr_auth.users WHERE id = $1", userID).Scan(&updatedHash)
	valid, verifyErr := handler.hasher.Verify(newValidPassword, updatedHash)
	if verifyErr != nil || !valid {
		t.Fatalf("expected new password to verify against stored hash")
	}

	// 6. Prohibit setting password on anonymous accounts
	anonUserID := "01918a24-7777-7000-8000-000000000007"
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, 'authenticated', true, clock_timestamp(), clock_timestamp())
	`, anonUserID)
	anonAccessToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     anonUserID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 3600)

	anonPasswordPayload, _ := json.Marshal(UpdateUserPasswordRequest{
		NewPassword: "SomeNewPassword123!",
	})
	anonPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/password", bytes.NewReader(anonPasswordPayload))
	anonPasswordRequest.Header.Set("Authorization", "Bearer "+anonAccessToken)
	anonPasswordResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPassword(anonPasswordResponseRecorder, anonPasswordRequest)
	if anonPasswordResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on setting password for anonymous account, got: %d (%s)", anonPasswordResponseRecorder.Code, anonPasswordResponseRecorder.Body.String())
	}

	// 7. Prohibit setting password on accounts without email and phone
	noIdentifierUserID := "01918a24-8888-7000-8000-000000000008"
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, 'user', false, clock_timestamp(), clock_timestamp())
	`, noIdentifierUserID)
	noIdentifierToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     noIdentifierUserID,
		Role:        "user",
		IsAnonymous: false,
	}, 3600)

	noIdentifierPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/password", bytes.NewReader(anonPasswordPayload))
	noIdentifierPasswordRequest.Header.Set("Authorization", "Bearer "+noIdentifierToken)
	noIdentifierPasswordResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPassword(noIdentifierPasswordResponseRecorder, noIdentifierPasswordRequest)
	if noIdentifierPasswordResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on setting password without email or phone, got: %d (%s)", noIdentifierPasswordResponseRecorder.Code, noIdentifierPasswordResponseRecorder.Body.String())
	}

	// 8. User with passkey enrolled marks MFAEnabled = true
	passkeyUserID := "01918a24-9999-7000-8000-000000000009"
	passkeyEmail := "passkey.user@example.com"
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, email, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, $2, 'user', false, '{}', clock_timestamp(), clock_timestamp())
	`, passkeyUserID, passkeyEmail)
	passkeyToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     passkeyUserID,
		Email:       passkeyEmail,
		Role:        "user",
		IsAnonymous: false,
	}, 3600)

	passkeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	passkeyRequest.Header.Set("Authorization", "Bearer "+passkeyToken)
	passkeyResponseRecorder := httptest.NewRecorder()
	handler.handleGetUser(passkeyResponseRecorder, passkeyRequest)
	var initialPasskeyUserResponse UserResponse
	_ = json.NewDecoder(passkeyResponseRecorder.Body).Decode(&initialPasskeyUserResponse)
	if initialPasskeyUserResponse.MFAEnabled {
		t.Fatalf("expected passkey user to have MFAEnabled=false before enrolling passkey")
	}

	// Insert passkey
	_, err = db.Exec(ctx, `
		INSERT INTO layr_auth.passkeys (id, user_id, credential_id, public_key, counter, created_at, last_used_at)
		VALUES ('01918a24-9999-7000-8000-000000000099', $1, 'credential-1', 'mockpubkey', 0, clock_timestamp(), clock_timestamp())
	`, passkeyUserID)
	if err != nil {
		t.Fatalf("failed to insert passkey: %v", err)
	}

	verifyPasskeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	verifyPasskeyRequest.Header.Set("Authorization", "Bearer "+passkeyToken)
	verifyPasskeyResponseRecorder := httptest.NewRecorder()
	handler.handleGetUser(verifyPasskeyResponseRecorder, verifyPasskeyRequest)
	var finalPasskeyUserResponse UserResponse
	_ = json.NewDecoder(verifyPasskeyResponseRecorder.Body).Decode(&finalPasskeyUserResponse)
	if !finalPasskeyUserResponse.MFAEnabled {
		t.Fatalf("expected passkey user to have MFAEnabled=true after passkey inserted")
	}

	// 9. Converted user without existing password can set initial password directly
	convertedUserID := "01918a24-2222-7000-8000-000000000002"
	convertedUserEmail := "converted.user@example.com"
	_, err = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, email, password_hash, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, $2, NULL, 'user', false, '{}', clock_timestamp(), clock_timestamp())
	`, convertedUserID, convertedUserEmail)
	if err != nil {
		t.Fatalf("failed to insert converted user without password: %v", err)
	}

	convertedAccessToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     convertedUserID,
		Email:       convertedUserEmail,
		Role:        "user",
		IsAnonymous: false,
	}, 3600)
	initialPasswordPayload, _ := json.Marshal(UpdateUserPasswordRequest{
		NewPassword: "InitialSetPassword123!#",
	})
	initialPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/password", bytes.NewReader(initialPasswordPayload))
	initialPasswordRequest.Header.Set("Authorization", "Bearer "+convertedAccessToken)
	initialPasswordResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPassword(initialPasswordResponseRecorder, initialPasswordRequest)
	if initialPasswordResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content when setting password on account without previous password, got: %d (%s)", initialPasswordResponseRecorder.Code, initialPasswordResponseRecorder.Body.String())
	}

	// 10. Email & Phone Conflict Checks for Authenticated User
	otherUserID := "01918a24-3333-7000-8000-000000000003"
	otherUserEmail := "other.user.existing@example.com"
	otherUserPhone := "+15557778888"
	_, err = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, email, phone, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, $2, $3, 'user', false, '{}', clock_timestamp(), clock_timestamp())
	`, otherUserID, otherUserEmail, otherUserPhone)
	if err != nil {
		t.Fatalf("failed to insert other user: %v", err)
	}

	emailConflictPayload, _ := json.Marshal(map[string]any{"email": otherUserEmail})
	emailConflictRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/request", bytes.NewReader(emailConflictPayload))
	emailConflictRequest.Header.Set("Authorization", "Bearer "+convertedAccessToken)
	emailConflictResponseRecorder := httptest.NewRecorder()
	handler.handleUserEmailVerificationRequest(emailConflictResponseRecorder, emailConflictRequest)
	if emailConflictResponseRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict when requesting already taken email, got: %d (%s)", emailConflictResponseRecorder.Code, emailConflictResponseRecorder.Body.String())
	}

	phoneConflictPayload, _ := json.Marshal(map[string]any{"phone": otherUserPhone})
	phoneConflictRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/request", bytes.NewReader(phoneConflictPayload))
	phoneConflictRequest.Header.Set("Authorization", "Bearer "+convertedAccessToken)
	phoneConflictResponseRecorder := httptest.NewRecorder()
	handler.handleUserPhoneVerificationRequest(phoneConflictResponseRecorder, phoneConflictRequest)
	if phoneConflictResponseRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict when requesting already taken phone, got: %d (%s)", phoneConflictResponseRecorder.Code, phoneConflictResponseRecorder.Body.String())
	}

	// 10b. Update User Email (PATCH /api/v1/auth/user/email) - Conflict and Success
	updateEmailConflictPayload, _ := json.Marshal(UpdateUserEmailRequest{Email: otherUserEmail})
	updateEmailConflictRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/email", bytes.NewReader(updateEmailConflictPayload))
	updateEmailConflictRequest.Header.Set("Authorization", "Bearer "+convertedAccessToken)
	updateEmailConflictResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserEmail(updateEmailConflictResponseRecorder, updateEmailConflictRequest)
	if updateEmailConflictResponseRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 on email update conflict, got: %d (%s)", updateEmailConflictResponseRecorder.Code, updateEmailConflictResponseRecorder.Body.String())
	}

	updateEmailSuccessPayload, _ := json.Marshal(UpdateUserEmailRequest{Email: "unique.new.email@example.com"})
	updateEmailSuccessRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/email", bytes.NewReader(updateEmailSuccessPayload))
	updateEmailSuccessRequest.Header.Set("Authorization", "Bearer "+convertedAccessToken)
	updateEmailSuccessResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserEmail(updateEmailSuccessResponseRecorder, updateEmailSuccessRequest)
	if updateEmailSuccessResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on valid email update, got: %d (%s)", updateEmailSuccessResponseRecorder.Code, updateEmailSuccessResponseRecorder.Body.String())
	}

	// 10c. Update User Phone (PATCH /api/v1/auth/user/phone) - Conflict and Success
	updatePhoneConflictPayload, _ := json.Marshal(UpdateUserPhoneRequest{Phone: otherUserPhone})
	updatePhoneConflictRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", bytes.NewReader(updatePhoneConflictPayload))
	updatePhoneConflictRequest.Header.Set("Authorization", "Bearer "+convertedAccessToken)
	updatePhoneConflictResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPhone(updatePhoneConflictResponseRecorder, updatePhoneConflictRequest)
	if updatePhoneConflictResponseRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 on phone update conflict, got: %d (%s)", updatePhoneConflictResponseRecorder.Code, updatePhoneConflictResponseRecorder.Body.String())
	}

	updatePhoneSuccessPayload, _ := json.Marshal(UpdateUserPhoneRequest{Phone: "+15558889999"})
	updatePhoneSuccessRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", bytes.NewReader(updatePhoneSuccessPayload))
	updatePhoneSuccessRequest.Header.Set("Authorization", "Bearer "+convertedAccessToken)
	updatePhoneSuccessResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPhone(updatePhoneSuccessResponseRecorder, updatePhoneSuccessRequest)
	if updatePhoneSuccessResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on valid phone update, got: %d (%s)", updatePhoneSuccessResponseRecorder.Code, updatePhoneSuccessResponseRecorder.Body.String())
	}

	// 10d. Anonymous user conversion error paths
	anonFailUserID := "01918a24-4444-7000-8000-000000000005"
	_, err = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, email, phone, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, NULL, NULL, 'authenticated', true, '{}', clock_timestamp(), clock_timestamp())
	`, anonFailUserID)
	if err != nil {
		t.Fatalf("failed to insert anonymous user: %v", err)
	}

	anonFailToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     anonFailUserID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 900)

	// Oversized email (>255 chars) causes UPDATE error -> 500
	oversizedEmail := strings.Repeat("a", 250) + "@test.com"
	updateEmailOversizedPayload, _ := json.Marshal(UpdateUserEmailRequest{Email: oversizedEmail})
	updateEmailOversizedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/email", bytes.NewReader(updateEmailOversizedPayload))
	updateEmailOversizedRequest.Header.Set("Authorization", "Bearer "+anonFailToken)
	updateEmailOversizedResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserEmail(updateEmailOversizedResponseRecorder, updateEmailOversizedRequest)
	if updateEmailOversizedResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on oversized email update for anonymous user, got: %d (%s)", updateEmailOversizedResponseRecorder.Code, updateEmailOversizedResponseRecorder.Body.String())
	}

	// Trigger DB update error -> 500 on phone update
	_, err = db.Exec(ctx, `
		CREATE OR REPLACE FUNCTION layr_auth.trg_fail_update_phone_fn() RETURNS trigger AS $$
		BEGIN
			IF NEW.phone = '+15559990000' THEN
				RAISE EXCEPTION 'simulated update failure';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		DROP TRIGGER IF EXISTS trg_fail_update_phone ON layr_auth.users;
		CREATE TRIGGER trg_fail_update_phone BEFORE UPDATE ON layr_auth.users
		FOR EACH ROW EXECUTE FUNCTION layr_auth.trg_fail_update_phone_fn();
	`)
	if err != nil {
		t.Fatalf("failed to create fail trigger: %v", err)
	}

	updatePhoneErrPayload, _ := json.Marshal(UpdateUserPhoneRequest{Phone: "+15559990000"})
	updatePhoneErrRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", bytes.NewReader(updatePhoneErrPayload))
	updatePhoneErrRequest.Header.Set("Authorization", "Bearer "+anonFailToken)
	updatePhoneErrResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPhone(updatePhoneErrResponseRecorder, updatePhoneErrRequest)
	if updatePhoneErrResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on phone update failure for anonymous user, got: %d (%s)", updatePhoneErrResponseRecorder.Code, updatePhoneErrResponseRecorder.Body.String())
	}

	_, _ = db.Exec(ctx, `
		DROP TRIGGER IF EXISTS trg_fail_update_phone ON layr_auth.users;
		DROP FUNCTION IF EXISTS layr_auth.trg_fail_update_phone_fn();
	`)

	// 11. Authenticate User via Insecure Cookie, X-Refresh-Token, and X-Session-Token
	testSessionRefreshToken := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testSessionRefreshHash := jwt.HashRefreshToken(testSessionRefreshToken)
	_, err = db.Exec(ctx, `
		INSERT INTO layr_auth.sessions (user_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1, $2, clock_timestamp() + interval '1 hour', clock_timestamp())
	`, convertedUserID, testSessionRefreshHash)
	if err != nil {
		t.Fatalf("failed to insert session: %v", err)
	}

	// Insecure cookie
	insecureCookieRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	insecureCookieRequest.AddCookie(&http.Cookie{Name: AuthSessionInsecureCookieName, Value: testSessionRefreshToken})
	insecureCookieResponseRecorder := httptest.NewRecorder()
	handler.handleGetUser(insecureCookieResponseRecorder, insecureCookieRequest)
	if insecureCookieResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 via insecure cookie auth, got: %d", insecureCookieResponseRecorder.Code)
	}

	// X-Refresh-Token header
	refreshHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	refreshHeaderRequest.Header.Set("X-Refresh-Token", testSessionRefreshToken)
	refreshHeaderResponseRecorder := httptest.NewRecorder()
	handler.handleGetUser(refreshHeaderResponseRecorder, refreshHeaderRequest)
	if refreshHeaderResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 via X-Refresh-Token header, got: %d", refreshHeaderResponseRecorder.Code)
	}

	// X-Session-Token header
	sessionHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	sessionHeaderRequest.Header.Set("X-Session-Token", testSessionRefreshToken)
	sessionHeaderResponseRecorder := httptest.NewRecorder()
	handler.handleGetUser(sessionHeaderResponseRecorder, sessionHeaderRequest)
	if sessionHeaderResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 via X-Session-Token header, got: %d", sessionHeaderResponseRecorder.Code)
	}

	// Unknown refresh token in header -> 401
	unknownHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	unknownHeaderRequest.Header.Set("X-Refresh-Token", "unknown-token-hash-12345678901234567890")
	unknownHeaderResponseRecorder := httptest.NewRecorder()
	handler.handleGetUser(unknownHeaderResponseRecorder, unknownHeaderRequest)
	if unknownHeaderResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unknown refresh token header, got: %d", unknownHeaderResponseRecorder.Code)
	}

	// 12. Deletion of user with phone cleans up phone and email OTPs
	phoneUserID := "01918a24-4444-7000-8000-000000000004"
	phoneUserEmail := "phone.delete@example.com"
	phoneUserPhone := "+15554321098"
	_, err = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, email, phone, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, $2, $3, 'user', false, '{}', clock_timestamp(), clock_timestamp())
	`, phoneUserID, phoneUserEmail, phoneUserPhone)
	if err != nil {
		t.Fatalf("failed to insert phone user: %v", err)
	}
	_, _ = db.Exec(ctx, "INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at) VALUES ($1, 'dummyhash', 'phone_verification', 0, clock_timestamp() + interval '10 minutes', clock_timestamp())", phoneUserPhone)
	_, _ = db.Exec(ctx, "INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at) VALUES ($1, 'dummyhash', 'email_verification', 0, clock_timestamp() + interval '10 minutes', clock_timestamp())", phoneUserEmail)

	phoneUserSessionRefresh := "111122223333444455556666777788889999aaaabbbbccccddddeeeeffff0000"
	phoneUserSessionHash := jwt.HashRefreshToken(phoneUserSessionRefresh)
	_, err = db.Exec(ctx, `
		INSERT INTO layr_auth.sessions (user_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1, $2, clock_timestamp() + interval '1 hour', clock_timestamp())
	`, phoneUserID, phoneUserSessionHash)
	if err != nil {
		t.Fatalf("failed to insert test session for phone user: %v", err)
	}
	_ = testKVStore.Set(ctx, "layr:auth:session:"+phoneUserSessionHash, "active", 3600*time.Second)

	phoneUserAccessToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     phoneUserID,
		Email:       phoneUserEmail,
		Phone:       phoneUserPhone,
		Role:        "user",
		IsAnonymous: false,
	}, 3600)
	deletePhoneUserRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/user", nil)
	deletePhoneUserRequest.Header.Set("Authorization", "Bearer "+phoneUserAccessToken)
	deletePhoneUserResponseRecorder := httptest.NewRecorder()
	handler.handleDeleteUser(deletePhoneUserResponseRecorder, deletePhoneUserRequest)
	if deletePhoneUserResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on phone user deletion, got: %d", deletePhoneUserResponseRecorder.Code)
	}
	var remainingOTPs int
	_ = db.QueryRow(ctx, "SELECT count(*) FROM layr_auth.otps WHERE recipient = $1 OR recipient = $2", phoneUserPhone, phoneUserEmail).Scan(&remainingOTPs)
	if remainingOTPs != 0 {
		t.Fatalf("expected all OTPs for deleted user to be removed, got: %d", remainingOTPs)
	}

	// 13. DELETE /api/v1/auth/user - Self-deletion of original user
	deleteRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/user", nil)
	deleteRequest.Header.Set("Authorization", "Bearer "+accessToken)
	deleteResponseRecorder := httptest.NewRecorder()
	handler.handleDeleteUser(deleteResponseRecorder, deleteRequest)
	if deleteResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on account deletion, got: %d (%s)", deleteResponseRecorder.Code, deleteResponseRecorder.Body.String())
	}

	var userCount int
	_ = db.QueryRow(ctx, "SELECT count(*) FROM layr_auth.users WHERE id = $1", userID).Scan(&userCount)
	if userCount != 0 {
		t.Fatalf("expected user to be completely removed from database")
	}

	subsequentProfileRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	subsequentProfileRequest.Header.Set("Authorization", "Bearer "+accessToken)
	subsequentProfileResponseRecorder := httptest.NewRecorder()
	handler.handleGetUser(subsequentProfileResponseRecorder, subsequentProfileRequest)
	if subsequentProfileResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found after user deletion, got: %d", subsequentProfileResponseRecorder.Code)
	}

	// 14. Authenticated Email Verification Confirm
	authEmailVerificationUser := "authemail.user@example.com"
	authEmailVerificationCode := "876543"
	authEmailVerificationHash := otp.HashCode(authEmailVerificationCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'email_verification', 0, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, authEmailVerificationUser, authEmailVerificationHash)

	authEmailConfirmPayload, _ := json.Marshal(map[string]any{
		"email": authEmailVerificationUser,
		"code":  authEmailVerificationCode,
	})
	authEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", bytes.NewReader(authEmailConfirmPayload))
	authEmailConfirmRequest.Header.Set("Authorization", "Bearer "+convertedAccessToken)
	authEmailConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(authEmailConfirmResponseRecorder, authEmailConfirmRequest)
	if authEmailConfirmResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on authenticated email verification confirm, got: %d (%s)", authEmailConfirmResponseRecorder.Code, authEmailConfirmResponseRecorder.Body.String())
	}

	// 15. Authenticated Phone Verification Confirm
	authPhoneVerificationUser := "+15552223333"
	authPhoneVerificationCode := "654321"
	authPhoneVerificationHash := otp.HashCode(authPhoneVerificationCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'phone_verification', 0, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, authPhoneVerificationUser, authPhoneVerificationHash)

	authPhoneConfirmPayload, _ := json.Marshal(map[string]any{
		"phone": authPhoneVerificationUser,
		"code":  authPhoneVerificationCode,
	})
	authPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", bytes.NewReader(authPhoneConfirmPayload))
	authPhoneConfirmRequest.Header.Set("Authorization", "Bearer "+convertedAccessToken)
	authPhoneConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(authPhoneConfirmResponseRecorder, authPhoneConfirmRequest)
	if authPhoneConfirmResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on authenticated phone verification confirm, got: %d (%s)", authPhoneConfirmResponseRecorder.Code, authPhoneConfirmResponseRecorder.Body.String())
	}

	// 16. User with NULL properties and mfa_secret_enc without pending
	nullPropsUserID := "01918a24-5555-7000-8000-000000000005"
	nullPropsEmail := "nullprops@example.com"
	_, err = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, email, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, $2, 'user', false, '{}', clock_timestamp(), clock_timestamp())
	`, nullPropsUserID, nullPropsEmail)
	if err != nil {
		t.Fatalf("failed to insert null properties user: %v", err)
	}
	nullPropsToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     nullPropsUserID,
		Email:       nullPropsEmail,
		Role:        "user",
		IsAnonymous: false,
	}, 3600)

	nullPropsProfileRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	nullPropsProfileRequest.Header.Set("Authorization", "Bearer "+nullPropsToken)
	nullPropsProfileResponseRecorder := httptest.NewRecorder()
	handler.handleGetUser(nullPropsProfileResponseRecorder, nullPropsProfileRequest)
	if nullPropsProfileResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for user with NULL properties, got: %d", nullPropsProfileResponseRecorder.Code)
	}

	// Update user to have mfa_secret_enc without mfa_pending
	_, _ = db.Exec(ctx, `
		UPDATE layr_auth.users
		SET properties = '{"mfa_secret_enc": "enc_secret_value"}'::jsonb
		WHERE id = $1
	`, nullPropsUserID)
	mfaSecretOnlyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	mfaSecretOnlyRequest.Header.Set("Authorization", "Bearer "+nullPropsToken)
	mfaSecretOnlyResponseRecorder := httptest.NewRecorder()
	handler.handleGetUser(mfaSecretOnlyResponseRecorder, mfaSecretOnlyRequest)
	var mfaSecretOnlyUserResponse UserResponse
	_ = json.NewDecoder(mfaSecretOnlyResponseRecorder.Body).Decode(&mfaSecretOnlyUserResponse)
	if !mfaSecretOnlyUserResponse.MFAEnabled {
		t.Fatalf("expected MFAEnabled to be true with mfa_secret_enc and no pending")
	}

	// User with boolean mfa_enabled
	boolMFAUserID := "01918a24-5555-7000-8000-000000000055"
	boolMFAEmail := "boolmfa@example.com"
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, email, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, $2, 'user', false, '{"mfa_enabled": true}'::jsonb, clock_timestamp(), clock_timestamp())
	`, boolMFAUserID, boolMFAEmail)
	boolMFAToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{Subject: boolMFAUserID, Email: boolMFAEmail, Role: "user"}, 3600)
	boolMFARequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	boolMFARequest.Header.Set("Authorization", "Bearer "+boolMFAToken)
	boolMFAResponseRecorder := httptest.NewRecorder()
	handler.handleGetUser(boolMFAResponseRecorder, boolMFARequest)
	var boolMFAUserResponse UserResponse
	_ = json.NewDecoder(boolMFAResponseRecorder.Body).Decode(&boolMFAUserResponse)
	if !boolMFAUserResponse.MFAEnabled {
		t.Fatalf("expected MFAEnabled to be true for boolean mfa_enabled")
	}

	// User with string mfa_enabled "true"
	stringMFAUserID := "01918a24-5555-7000-8000-000000000056"
	stringMFAEmail := "strmfa@example.com"
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, email, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, $2, 'user', false, '{"mfa_enabled": "true"}'::jsonb, clock_timestamp(), clock_timestamp())
	`, stringMFAUserID, stringMFAEmail)
	stringMFAToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{Subject: stringMFAUserID, Email: stringMFAEmail, Role: "user"}, 3600)
	stringMFARequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	stringMFARequest.Header.Set("Authorization", "Bearer "+stringMFAToken)
	stringMFAResponseRecorder := httptest.NewRecorder()
	handler.handleGetUser(stringMFAResponseRecorder, stringMFARequest)
	var stringMFAUserResponse UserResponse
	_ = json.NewDecoder(stringMFAResponseRecorder.Body).Decode(&stringMFAUserResponse)
	if !stringMFAUserResponse.MFAEnabled {
		t.Fatalf("expected MFAEnabled to be true for string mfa_enabled")
	}

	// 17. Non-existent user in valid token -> Update fails with 500, Password change fails with 404
	ghostUserID := "01918a24-6666-7000-8000-000000000006"
	ghostToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     ghostUserID,
		Email:       "ghost@example.com",
		Role:        "user",
		IsAnonymous: false,
	}, 3600)

	ghostUpdateRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/properties", strings.NewReader(`{"properties":{"tier":"ghost"}}`))
	ghostUpdateRequest.Header.Set("Authorization", "Bearer "+ghostToken)
	ghostUpdateResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserProperties(ghostUpdateResponseRecorder, ghostUpdateRequest)
	if ghostUpdateResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on updating non-existent user properties, got: %d", ghostUpdateResponseRecorder.Code)
	}

	ghostPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/password", strings.NewReader(`{"new_password":"ValidNewPassword123!"}`))
	ghostPasswordRequest.Header.Set("Authorization", "Bearer "+ghostToken)
	ghostPasswordResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPassword(ghostPasswordResponseRecorder, ghostPasswordRequest)
	if ghostPasswordResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on password change for non-existent user, got: %d", ghostPasswordResponseRecorder.Code)
	}

	// 18. Authenticated Confirm with non-existent user ID -> 500
	ghostEmailCode := "112233"
	ghostEmailHash := otp.HashCode(ghostEmailCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ('ghost.confirm@example.com', $1, 'email_verification', 0, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, ghostEmailHash)
	ghostEmailConfirmPayload, _ := json.Marshal(map[string]any{
		"email": "ghost.confirm@example.com",
		"code":  ghostEmailCode,
	})
	ghostEmailConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/email/verification/confirm", bytes.NewReader(ghostEmailConfirmPayload))
	ghostEmailConfirmRequest.Header.Set("Authorization", "Bearer "+ghostToken)
	ghostEmailConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserEmailVerificationConfirm(ghostEmailConfirmResponseRecorder, ghostEmailConfirmRequest)
	if ghostEmailConfirmResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on authenticated email confirm for non-existent user, got: %d", ghostEmailConfirmResponseRecorder.Code)
	}

	ghostPhoneCode := "332211"
	ghostPhoneHash := otp.HashCode(ghostPhoneCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ('+15559997777', $1, 'phone_verification', 0, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, ghostPhoneHash)
	ghostPhoneConfirmPayload, _ := json.Marshal(map[string]any{
		"phone": "+15559997777",
		"code":  ghostPhoneCode,
	})
	ghostPhoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/user/phone/verification/confirm", bytes.NewReader(ghostPhoneConfirmPayload))
	ghostPhoneConfirmRequest.Header.Set("Authorization", "Bearer "+ghostToken)
	ghostPhoneConfirmResponseRecorder := httptest.NewRecorder()
	handler.handleUserPhoneVerificationConfirm(ghostPhoneConfirmResponseRecorder, ghostPhoneConfirmRequest)
	if ghostPhoneConfirmResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on authenticated phone confirm for non-existent user, got: %d", ghostPhoneConfirmResponseRecorder.Code)
	}

	// 19. Canceled Context on Password change and Deletion -> 500 / 404
	canceledPasswordRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/password", strings.NewReader(`{"new_password":"ValidNewPassword123!"}`))
	{
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		canceledPasswordRequest = canceledPasswordRequest.WithContext(canceledCtx)
	}
	canceledPasswordRequest.Header.Set("Authorization", "Bearer "+convertedAccessToken)
	canceledPasswordResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPassword(canceledPasswordResponseRecorder, canceledPasswordRequest)
	if canceledPasswordResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on canceled context password change, got: %d", canceledPasswordResponseRecorder.Code)
	}

	canceledDeleteRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/auth/user", nil)
	{
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		canceledDeleteRequest = canceledDeleteRequest.WithContext(canceledCtx)
	}
	canceledDeleteRequest.Header.Set("Authorization", "Bearer "+convertedAccessToken)
	canceledDeleteResponseRecorder := httptest.NewRecorder()
	handler.handleDeleteUser(canceledDeleteResponseRecorder, canceledDeleteRequest)
	if canceledDeleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled context user delete, got: %d", canceledDeleteResponseRecorder.Code)
	}

	// 20. Anonymous user converts email and phone via PATCH (publishes event)
	anonPatchEmailUserID := "01918a24-7777-7000-8000-000000000077"
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, 'authenticated', true, clock_timestamp(), clock_timestamp())
	`, anonPatchEmailUserID)
	anonPatchEmailToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     anonPatchEmailUserID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 3600)
	anonPatchEmailPayload, _ := json.Marshal(UpdateUserEmailRequest{Email: "anon.converted.email@example.com"})
	anonPatchEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/email", bytes.NewReader(anonPatchEmailPayload))
	anonPatchEmailRequest.Header.Set("Authorization", "Bearer "+anonPatchEmailToken)
	anonPatchEmailResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserEmail(anonPatchEmailResponseRecorder, anonPatchEmailRequest)
	if anonPatchEmailResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on anonymous user email conversion, got: %d (%s)", anonPatchEmailResponseRecorder.Code, anonPatchEmailResponseRecorder.Body.String())
	}

	anonPatchPhoneUserID := "01918a24-7777-7000-8000-000000000078"
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, 'authenticated', true, clock_timestamp(), clock_timestamp())
	`, anonPatchPhoneUserID)
	anonPatchPhoneToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     anonPatchPhoneUserID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 3600)
	anonPatchPhonePayload, _ := json.Marshal(UpdateUserPhoneRequest{Phone: "+15553334444"})
	anonPatchPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", bytes.NewReader(anonPatchPhonePayload))
	anonPatchPhoneRequest.Header.Set("Authorization", "Bearer "+anonPatchPhoneToken)
	anonPatchPhoneResponseRecorder := httptest.NewRecorder()
	handler.handleUpdateUserPhone(anonPatchPhoneResponseRecorder, anonPatchPhoneRequest)
	if anonPatchPhoneResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on anonymous user phone conversion, got: %d (%s)", anonPatchPhoneResponseRecorder.Code, anonPatchPhoneResponseRecorder.Body.String())
	}

	// 21. Canceled context on UpdateUserEmail and UpdateUserPhone -> 500
	{
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		canceledEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/email", strings.NewReader(`{"email":"canceled.email@example.com"}`))
		canceledEmailRequest = canceledEmailRequest.WithContext(canceledCtx)
		canceledEmailRequest.Header.Set("Authorization", "Bearer "+anonAccessToken)
		canceledEmailResponseRecorder := httptest.NewRecorder()
		handler.handleUpdateUserEmail(canceledEmailResponseRecorder, canceledEmailRequest)
		if canceledEmailResponseRecorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on canceled context user email update, got: %d", canceledEmailResponseRecorder.Code)
		}

		canceledPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/auth/user/phone", strings.NewReader(`{"phone":"+15551239999"}`))
		canceledPhoneRequest = canceledPhoneRequest.WithContext(canceledCtx)
		canceledPhoneRequest.Header.Set("Authorization", "Bearer "+anonAccessToken)
		canceledPhoneResponseRecorder := httptest.NewRecorder()
		handler.handleUpdateUserPhone(canceledPhoneResponseRecorder, canceledPhoneRequest)
		if canceledPhoneResponseRecorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on canceled context user phone update, got: %d", canceledPhoneResponseRecorder.Code)
		}
	}

	// 22. resolveAnonymousCaller with non-existent user ID in DB (hits ErrNoRows branch)
	ghostSubjectToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     "01918a24-0000-7000-8000-000000000000",
		Role:        "authenticated",
		IsAnonymous: true,
	}, 3600)
	ghostAnonRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/user", nil)
	ghostAnonRequest.Header.Set("Authorization", "Bearer "+ghostSubjectToken)
	ghostAnonUserRecord, ghostAnonUserRecordErr := handler.resolveAnonymousCaller(ghostAnonRequest)
	if ghostAnonUserRecord != nil || !errors.Is(ghostAnonUserRecordErr, ErrAnonymousSessionNotFound) {
		t.Fatalf("expected ErrAnonymousSessionNotFound for ghost subject, got: %v", ghostAnonUserRecordErr)
	}

	// 23. issueSessionResponse with canceled context (DB session insert fails gracefully)
	{
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		dummyUserRecord := UserRecord{
			ID:   userID,
			Role: "authenticated",
		}
		canceledSessionRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil).WithContext(canceledCtx)
		canceledSessionResponseRecorder := httptest.NewRecorder()
		handler.issueSessionResponse(canceledSessionResponseRecorder, canceledSessionRequest, dummyUserRecord)
		if canceledSessionResponseRecorder.Code != http.StatusOK {
			t.Fatalf("expected 200 even if session db insert fails, got: %d", canceledSessionResponseRecorder.Code)
		}
	}
}
