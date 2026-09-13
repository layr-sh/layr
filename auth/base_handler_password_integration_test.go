package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"layr.sh/auth/otp"
	"layr.sh/core"
)

func TestAuthPasswordResetFlowIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanupDatabase := setupTestDatabase(t)
	defer cleanupDatabase()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load initial auth config: %v", err)
	}

	// Configure mock dispatchers
	var dispatchedEmailCode, dispatchedSMSCode string
	emailWebhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(request.Body).Decode(&payload)
		if code, ok := payload["code"].(string); ok {
			dispatchedEmailCode = code
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer emailWebhookServer.Close()

	smsWebhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(request.Body).Decode(&payload)
		if code, ok := payload["code"].(string); ok {
			dispatchedSMSCode = code
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer smsWebhookServer.Close()

	driverWebhook := "webhook"
	activeConfig := configManager.Get()
	activeConfig.Password.Enabled = true
	activeConfig.EmailDispatcher = EmailDispatcherConfig{
		Driver:  &driverWebhook,
		Webhook: EmailDispatcherWebhookConfig{URL: emailWebhookServer.URL},
	}
	activeConfig.SMSDispatcher = SMSDispatcherConfig{
		Driver:  &driverWebhook,
		Webhook: SMSDispatcherWebhookConfig{URL: smsWebhookServer.URL},
	}
	configManager.Set(activeConfig)

	eventBus := core.NewEventBus(db, cryptoKeyManager)
	defer eventBus.Close()
	testKVStore := newInMemoryKVStore()

	baseHandler := NewBaseHandler(db, configManager, cryptoKeyManager)
	baseHandler.SetEventBus(eventBus)
	baseHandler.SetKVStore(testKVStore)
	baseHandler.SetEmailDispatcher(NewEmailDispatcher(db, func() *EmailDispatcherConfig {
		emailDispatcherConfig := activeConfig.EmailDispatcher
		return &emailDispatcherConfig
	}, cryptoKeyManager))
	baseHandler.SetSMSDispatcher(NewSMSDispatcher(db, func() *SMSDispatcherConfig {
		smsDispatcherConfig := activeConfig.SMSDispatcher
		return &smsDispatcherConfig
	}, cryptoKeyManager))

	// Seed test user with email and phone
	resetUserID := "01918a24-2222-7000-8000-000000000002"
	resetEmail := "reset.user@example.com"
	resetPhone := "+15553332222"
	initialPassword := "InitialPassword123!"
	initialHash, _ := baseHandler.hasher.Hash(initialPassword)

	_, err := db.Exec(ctx, `
		INSERT INTO auth.users (id, email, phone, password_hash, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, $2, $3, $4, 'authenticated', false, clock_timestamp(), clock_timestamp())
	`, resetUserID, resetEmail, resetPhone, initialHash)
	if err != nil {
		t.Fatalf("failed to insert test user for password reset: %v", err)
	}

	// 1. Password Reset Request via Email -> 204
	resetEmailRequestPayload, _ := json.Marshal(PasswordResetRequest{Email: resetEmail})
	resetEmailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewReader(resetEmailRequestPayload))
	resetEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasswordResetRequest(resetEmailResponseRecorder, resetEmailRequest)
	if resetEmailResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on email password reset request, got: %d (%s)", resetEmailResponseRecorder.Code, resetEmailResponseRecorder.Body.String())
	}
	if dispatchedEmailCode == "" {
		t.Fatalf("expected email code to be dispatched via webhook")
	}

	// 2. Cooldown active -> 429
	cooldownRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewReader(resetEmailRequestPayload))
	cooldownResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasswordResetRequest(cooldownResponseRecorder, cooldownRequest)
	if cooldownResponseRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on cooldown active, got: %d", cooldownResponseRecorder.Code)
	}
	// Clear cooldown for further tests
	_ = testKVStore.Delete(ctx, "auth:cooldown:password_reset:"+resetEmail)

	// 3. IP Rate Limit Exceeded -> 429
	clientIP := core.ExtractRequestClientIP(resetEmailRequest)
	ipRateKey := fmt.Sprintf("auth:ratelimit:otp:ip:%s", clientIP)
	for i := 0; i < 11; i++ {
		_, _ = testKVStore.Increment(ctx, ipRateKey, time.Hour)
	}
	rateLimitRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewReader(resetEmailRequestPayload))
	rateLimitResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasswordResetRequest(rateLimitResponseRecorder, rateLimitRequest)
	if rateLimitResponseRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on IP rate limit exceeded, got: %d", rateLimitResponseRecorder.Code)
	}
	_ = testKVStore.Delete(ctx, ipRateKey)

	// 4. Request for Non-Existent Recipient -> 404
	ghostResetPayload, _ := json.Marshal(PasswordResetRequest{Email: "nonexistent@example.com"})
	ghostResetRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewReader(ghostResetPayload))
	ghostResetResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasswordResetRequest(ghostResetResponseRecorder, ghostResetRequest)
	if ghostResetResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on non-existent recipient reset request, got: %d", ghostResetResponseRecorder.Code)
	}

	// 5. Confirm with Wrong Code increments attempts -> 400
	wrongConfirmPayload, _ := json.Marshal(PasswordResetConfirmRequest{
		Email:    resetEmail,
		Code:     "000000",
		Password: "NewSecurePassword123!@",
	})
	wrongConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/confirm", bytes.NewReader(wrongConfirmPayload))
	wrongConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasswordResetConfirm(wrongConfirmResponseRecorder, wrongConfirmRequest)
	if wrongConfirmResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(wrongConfirmResponseRecorder.Body.String(), "Invalid reset code") {
		t.Fatalf("expected 400 on invalid reset code, got: %d (%s)", wrongConfirmResponseRecorder.Code, wrongConfirmResponseRecorder.Body.String())
	}

	// 6. Confirm with Valid Code -> 200 and issues session
	newPassword := "BrandNewPassword456!#"
	validConfirmPayload, _ := json.Marshal(PasswordResetConfirmRequest{
		Email:    resetEmail,
		Code:     dispatchedEmailCode,
		Password: newPassword,
	})
	validConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/confirm", bytes.NewReader(validConfirmPayload))
	validConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasswordResetConfirm(validConfirmResponseRecorder, validConfirmRequest)
	if validConfirmResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid password reset confirm, got: %d (%s)", validConfirmResponseRecorder.Code, validConfirmResponseRecorder.Body.String())
	}

	// Verify updated password in DB
	var updatedPasswordHash string
	_ = db.QueryRow(ctx, "SELECT password_hash FROM auth.users WHERE id = $1", resetUserID).Scan(&updatedPasswordHash)
	valid, _ := baseHandler.hasher.Verify(newPassword, updatedPasswordHash)
	if !valid {
		t.Fatalf("expected new password to match stored hash in database")
	}

	// 6b. Locked user password reset confirm -> 423
	lockedResetRequestPayload, _ := json.Marshal(PasswordResetRequest{Email: resetEmail})
	lockedResetRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewReader(lockedResetRequestPayload))
	lockedResetResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasswordResetRequest(lockedResetResponseRecorder, lockedResetRequest)
	if lockedResetResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on password reset request, got: %d", lockedResetResponseRecorder.Code)
	}

	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = clock_timestamp() + interval '1 hour' WHERE id = $1", resetUserID)
	lockedConfirmPayload, _ := json.Marshal(PasswordResetConfirmRequest{
		Email:    resetEmail,
		Code:     dispatchedEmailCode,
		Password: "YetAnotherPassword789!",
	})
	lockedConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/confirm", bytes.NewReader(lockedConfirmPayload))
	lockedConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasswordResetConfirm(lockedConfirmResponseRecorder, lockedConfirmRequest)
	if lockedConfirmResponseRecorder.Code != http.StatusLocked {
		t.Fatalf("expected 423 StatusLocked on locked user password reset confirm, got: %d", lockedConfirmResponseRecorder.Code)
	}
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = NULL WHERE id = $1", resetUserID)

	// 7. Password Reset Request & Confirm via Phone
	resetPhoneRequestPayload, _ := json.Marshal(PasswordResetRequest{Phone: resetPhone})
	resetPhoneRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewReader(resetPhoneRequestPayload))
	resetPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasswordResetRequest(resetPhoneResponseRecorder, resetPhoneRequest)
	if resetPhoneResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on phone reset request, got: %d (%s)", resetPhoneResponseRecorder.Code, resetPhoneResponseRecorder.Body.String())
	}
	if dispatchedSMSCode == "" {
		t.Fatalf("expected SMS code to be dispatched")
	}

	phoneConfirmPayload, _ := json.Marshal(PasswordResetConfirmRequest{
		Phone:    resetPhone,
		Code:     dispatchedSMSCode,
		Password: "PhoneUpdatedPassword789!$",
	})
	phoneConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/confirm", bytes.NewReader(phoneConfirmPayload))
	phoneConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasswordResetConfirm(phoneConfirmResponseRecorder, phoneConfirmRequest)
	if phoneConfirmResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on phone reset confirm, got: %d (%s)", phoneConfirmResponseRecorder.Code, phoneConfirmResponseRecorder.Body.String())
	}

	// 8. Max Attempts Exceeded on Confirm -> 400
	maxAttemptsCode := "999888"
	maxAttemptsHash := otp.HashCode(maxAttemptsCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'password_reset', 5, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, resetEmail, maxAttemptsHash)
	maxAttemptsPayload, _ := json.Marshal(PasswordResetConfirmRequest{
		Email:    resetEmail,
		Code:     maxAttemptsCode,
		Password: "AnotherPassword123!",
	})
	maxAttemptsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/confirm", bytes.NewReader(maxAttemptsPayload))
	maxAttemptsResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasswordResetConfirm(maxAttemptsResponseRecorder, maxAttemptsRequest)
	if maxAttemptsResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(maxAttemptsResponseRecorder.Body.String(), "Maximum attempts exceeded") {
		t.Fatalf("expected 400 with Maximum attempts exceeded, got: %d (%s)", maxAttemptsResponseRecorder.Code, maxAttemptsResponseRecorder.Body.String())
	}

	// 9. Confirm for Expired / Non-Existent OTP -> 400
	ghostConfirmPayload, _ := json.Marshal(PasswordResetConfirmRequest{
		Email:    "nobody@example.com",
		Code:     "123456",
		Password: "ValidPassword123!",
	})
	ghostConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/confirm", bytes.NewReader(ghostConfirmPayload))
	ghostConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasswordResetConfirm(ghostConfirmResponseRecorder, ghostConfirmRequest)
	if ghostConfirmResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(ghostConfirmResponseRecorder.Body.String(), "Invalid or expired") {
		t.Fatalf("expected 400 on non-existent OTP confirm, got: %d (%s)", ghostConfirmResponseRecorder.Code, ghostConfirmResponseRecorder.Body.String())
	}

	// 10. Confirm with Orphan OTP (user row was deleted) -> 404
	orphanRecipient := "orphan.reset@example.com"
	orphanCode := "333222"
	orphanHash := otp.HashCode(orphanCode)
	_, _ = db.Exec(ctx, `
		INSERT INTO auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'password_reset', 0, clock_timestamp() + interval '10 minutes', clock_timestamp())
	`, orphanRecipient, orphanHash)
	orphanConfirmPayload, _ := json.Marshal(PasswordResetConfirmRequest{
		Email:    orphanRecipient,
		Code:     orphanCode,
		Password: "ValidPassword123!",
	})
	orphanConfirmRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/password-reset/confirm", bytes.NewReader(orphanConfirmPayload))
	orphanConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasswordResetConfirm(orphanConfirmResponseRecorder, orphanConfirmRequest)
	if orphanConfirmResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on orphan reset confirm, got: %d (%s)", orphanConfirmResponseRecorder.Code, orphanConfirmResponseRecorder.Body.String())
	}
}
