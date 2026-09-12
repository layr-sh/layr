package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"layr.sh/auth/jwt"
	"layr.sh/core"
)

func TestAuthOutboundRateLimitingAndCooldownIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	driverWebhook := "webhook"
	authConfig := configManager.Get()
	authConfig.EmailOTP.Enabled = true
	authConfig.SMSOTP.Enabled = true
	authConfig.EmailDispatcher = EmailDispatcherConfig{
		Driver:      &driverWebhook,
		SenderEmail: "auth@layr.sh",
		SenderName:  "Layr",
		Webhook: EmailDispatcherWebhookConfig{
			URL: "http://localhost:9999/webhook",
		},
	}
	authConfig.SMSDispatcher = SMSDispatcherConfig{
		Driver: &driverWebhook,
		Webhook: SMSDispatcherWebhookConfig{
			URL: "http://localhost:9999/webhook",
		},
	}
	if err := configManager.Save(ctx, authConfig); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	databaseKVStore := core.NewDatabaseKVStore(ctx, db, 60*time.Second)
	defer func() { _ = databaseKVStore.Close() }()
	handler := NewHandler(db, configManager, cryptoKeyManager)
	handler.SetKVStore(databaseKVStore)

	emailDispatcher := NewEmailDispatcher(db, func() *EmailDispatcherConfig {
		emailDispatcherConfig := configManager.Get().EmailDispatcher
		return &emailDispatcherConfig
	}, cryptoKeyManager)
	smsDispatcher := NewSMSDispatcher(db, func() *SMSDispatcherConfig {
		smsDispatcherConfig := configManager.Get().SMSDispatcher
		return &smsDispatcherConfig
	}, cryptoKeyManager)
	handler.SetEmailDispatcher(emailDispatcher)
	handler.SetSMSDispatcher(smsDispatcher)

	targetEmail := "cooldown.user@example.com"

	// 1. Send first OTP request -> 204 No Content
	firstOTPPayload := map[string]any{
		"recipient": targetEmail,
		"purpose":   "signin",
	}
	encodedFirstOTP, _ := json.Marshal(firstOTPPayload)
	firstOTPRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/send", bytes.NewReader(encodedFirstOTP))
	firstOTPResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(firstOTPResponseRecorder, firstOTPRequest)
	if firstOTPResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on first OTP send, got: %d (body: %s)", firstOTPResponseRecorder.Code, firstOTPResponseRecorder.Body.String())
	}

	// 2. Send second OTP request within cooldown to same email -> 429 Too Many Requests (LAYR_AUTH_COOLDOWN)
	secondOTPPayload := map[string]any{
		"recipient": targetEmail,
		"purpose":   "signin",
	}
	encodedSecondOTP, _ := json.Marshal(secondOTPPayload)
	secondOTPRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/send", bytes.NewReader(encodedSecondOTP))
	secondOTPResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(secondOTPResponseRecorder, secondOTPRequest)
	if secondOTPResponseRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on second OTP send within cooldown, got: %d", secondOTPResponseRecorder.Code)
	}
	if !strings.Contains(secondOTPResponseRecorder.Body.String(), "LAYR_AUTH_COOLDOWN") || !strings.Contains(secondOTPResponseRecorder.Body.String(), "Please wait 60 seconds before requesting another code") {
		t.Fatalf("expected LAYR_AUTH_COOLDOWN and cooldown message, got: %s", secondOTPResponseRecorder.Body.String())
	}

	// 3. Fast-forward cooldown key expiration in database to simulate 60s passing -> request succeeds
	_, err := db.Exec(ctx, `
		UPDATE core.kv_store 
		SET expires_at = clock_timestamp() - interval '1 second' 
		WHERE key = $1
	`, fmt.Sprintf("layr:auth:cooldown:signin:%s", targetEmail))
	if err != nil {
		t.Fatalf("failed to expire cooldown key: %v", err)
	}

	thirdOTPPayload := map[string]any{
		"recipient": targetEmail,
		"purpose":   "signin",
	}
	encodedThirdOTP, _ := json.Marshal(thirdOTPPayload)
	thirdOTPRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/send", bytes.NewReader(encodedThirdOTP))
	thirdOTPResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(thirdOTPResponseRecorder, thirdOTPRequest)
	if thirdOTPResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on OTP send after cooldown expired, got: %d (body: %s)", thirdOTPResponseRecorder.Code, thirdOTPResponseRecorder.Body.String())
	}

	// 4. IP Rate Limit: 10 requests allowed per hour, 11th blocked with 429 LAYR_AUTH_RATE_LIMIT_EXCEEDED
	limitedClientIP := "198.51.100.77:12345"
	for counter := 1; counter <= 10; counter++ {
		loopPayload := map[string]any{
			"recipient": fmt.Sprintf("ip.user%d@example.com", counter),
			"purpose":   "signin",
		}
		encodedLoop, _ := json.Marshal(loopPayload)
		loopRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/send", bytes.NewReader(encodedLoop))
		loopRequest.RemoteAddr = limitedClientIP
		loopResponseRecorder := httptest.NewRecorder()
		handler.handleOTPSend(loopResponseRecorder, loopRequest)
		if loopResponseRecorder.Code != http.StatusNoContent {
			t.Fatalf("expected 204 for request %d within IP rate limit, got: %d", counter, loopResponseRecorder.Code)
		}
	}

	// 11th request from limitedClientIP -> 429
	eleventhPayload := map[string]any{
		"recipient": "ip.user11@example.com",
		"purpose":   "signin",
	}
	encodedEleventh, _ := json.Marshal(eleventhPayload)
	eleventhRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/send", bytes.NewReader(encodedEleventh))
	eleventhRequest.RemoteAddr = limitedClientIP
	eleventhResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(eleventhResponseRecorder, eleventhRequest)
	if eleventhResponseRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 11th request from same IP, got: %d", eleventhResponseRecorder.Code)
	}
	if !strings.Contains(eleventhResponseRecorder.Body.String(), "LAYR_AUTH_RATE_LIMIT_EXCEEDED") {
		t.Fatalf("expected LAYR_AUTH_RATE_LIMIT_EXCEEDED in error response, got: %s", eleventhResponseRecorder.Body.String())
	}
}

func TestAuthOTPFlowAndConversionIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	driverWebhook := "webhook"
	authConfig := configManager.Get()
	authConfig.Anonymous.Enabled = true
	authConfig.EmailOTP.Enabled = true
	authConfig.SMSOTP.Enabled = true
	authConfig.EmailDispatcher = EmailDispatcherConfig{
		Driver:      &driverWebhook,
		SenderEmail: "auth@layr.sh",
		SenderName:  "Layr",
		Webhook: EmailDispatcherWebhookConfig{
			URL: "http://localhost:9999/webhook",
		},
	}
	authConfig.SMSDispatcher = SMSDispatcherConfig{
		Driver: &driverWebhook,
		Webhook: SMSDispatcherWebhookConfig{
			URL: "http://localhost:9999/webhook",
		},
	}
	if err := configManager.Save(ctx, authConfig); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	databaseKVStore := core.NewDatabaseKVStore(ctx, db, 60*time.Second)
	defer func() { _ = databaseKVStore.Close() }()
	eventBus := core.NewEventBus(db, cryptoKeyManager)

	var capturedEvents []core.Event
	var eventsMutex sync.Mutex
	eventBus.Subscribe("auth.*", func(eventCtx context.Context, event core.Event) error {
		eventsMutex.Lock()
		defer eventsMutex.Unlock()
		capturedEvents = append(capturedEvents, event)
		return nil
	})

	handler := NewHandler(db, configManager, cryptoKeyManager)
	handler.SetKVStore(databaseKVStore)
	handler.SetEventBus(eventBus)
	emailDispatcher := NewEmailDispatcher(db, func() *EmailDispatcherConfig {
		emailDispatcherConfig := configManager.Get().EmailDispatcher
		return &emailDispatcherConfig
	}, cryptoKeyManager)
	smsDispatcher := NewSMSDispatcher(db, func() *SMSDispatcherConfig {
		smsDispatcherConfig := configManager.Get().SMSDispatcher
		return &smsDispatcherConfig
	}, cryptoKeyManager)
	handler.SetEmailDispatcher(emailDispatcher)
	handler.SetSMSDispatcher(smsDispatcher)

	userEmail := "otp.user@example.com"

	// 1. Send OTP for email
	otpSendPayload := map[string]any{
		"recipient": userEmail,
		"purpose":   "signin",
	}
	encodedOTPSend, _ := json.Marshal(otpSendPayload)
	otpSendRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/send", bytes.NewReader(encodedOTPSend))
	otpSendResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(otpSendResponseRecorder, otpSendRequest)
	if otpSendResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 from /otp/send, got: %d (%s)", otpSendResponseRecorder.Code, otpSendResponseRecorder.Body.String())
	}

	otpCode, err := databaseKVStore.Get(ctx, fmt.Sprintf("layr:auth:otp:signin:%s", userEmail))
	if err != nil || otpCode == "" {
		t.Fatalf("failed to retrieve OTP code from kvstore: %v", err)
	}

	// 2. Wrong OTP code -> 400
	wrongCodePayload := map[string]any{
		"recipient": userEmail,
		"code":      "000000",
		"purpose":   "signin",
	}
	encodedWrongCode, _ := json.Marshal(wrongCodePayload)
	wrongCodeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewReader(encodedWrongCode))
	wrongCodeResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(wrongCodeResponseRecorder, wrongCodeRequest)
	if wrongCodeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on wrong OTP code, got: %d", wrongCodeResponseRecorder.Code)
	}

	// 3. Valid OTP code -> 200 OK (creates user and session)
	otpVerifyPayload := map[string]any{
		"recipient": userEmail,
		"code":      otpCode,
		"purpose":   "signin",
	}
	encodedOTPVerify, _ := json.Marshal(otpVerifyPayload)
	otpVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewReader(encodedOTPVerify))
	otpVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(otpVerifyResponseRecorder, otpVerifyRequest)
	if otpVerifyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /otp/verify, got: %d (%s)", otpVerifyResponseRecorder.Code, otpVerifyResponseRecorder.Body.String())
	}

	var sessionResponse SessionResponse
	if decodeErr := json.NewDecoder(otpVerifyResponseRecorder.Body).Decode(&sessionResponse); decodeErr != nil {
		t.Fatalf("failed to decode verify response: %v", decodeErr)
	}
	if sessionResponse.User.Email == nil || *sessionResponse.User.Email != userEmail {
		t.Fatalf("expected user email %s, got: %v", userEmail, sessionResponse.User.Email)
	}

	// 4. Phone OTP flow
	phoneRecipient := "+1234567890"
	phoneOTPPayload := map[string]any{
		"recipient": phoneRecipient,
		"purpose":   "signin",
	}
	encodedPhoneOTP, _ := json.Marshal(phoneOTPPayload)
	phoneOTPRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/send", bytes.NewReader(encodedPhoneOTP))
	phoneOTPResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(phoneOTPResponseRecorder, phoneOTPRequest)
	if phoneOTPResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on phone OTP send: %d", phoneOTPResponseRecorder.Code)
	}

	phoneCode, phoneCodeErr := databaseKVStore.Get(ctx, fmt.Sprintf("layr:auth:otp:signin:%s", phoneRecipient))
	if phoneCodeErr != nil || phoneCode == "" {
		t.Fatalf("failed to retrieve phone OTP code: %v", phoneCodeErr)
	}

	phoneVerifyPayload := map[string]any{
		"recipient": phoneRecipient,
		"code":      phoneCode,
	}
	encodedPhoneVerify, _ := json.Marshal(phoneVerifyPayload)
	phoneVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewReader(encodedPhoneVerify))
	phoneVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(phoneVerifyResponseRecorder, phoneVerifyRequest)
	if phoneVerifyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on phone OTP verify: %d", phoneVerifyResponseRecorder.Code)
	}

	// 5. Convert anonymous user via OTP
	anonUserID := uuid.NewV7().String()
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, 'authenticated', true, clock_timestamp(), clock_timestamp())
	`, anonUserID)
	anonAccessToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     anonUserID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 900)

	convertEmail := "converted.otp@example.com"
	convertSendPayload, _ := json.Marshal(map[string]any{
		"recipient": convertEmail,
		"purpose":   "signin",
	})
	convertSendRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/send", bytes.NewReader(convertSendPayload))
	convertSendResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(convertSendResponseRecorder, convertSendRequest)
	if convertSendResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on convert OTP send: %d", convertSendResponseRecorder.Code)
	}

	convertCode, _ := databaseKVStore.Get(ctx, fmt.Sprintf("layr:auth:otp:signin:%s", convertEmail))

	convertVerifyPayload, _ := json.Marshal(map[string]any{
		"recipient": convertEmail,
		"code":      convertCode,
		"purpose":   "signin",
	})
	convertVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewReader(convertVerifyPayload))
	convertVerifyRequest.Header.Set("Authorization", "Bearer "+anonAccessToken)
	convertVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(convertVerifyResponseRecorder, convertVerifyRequest)
	if convertVerifyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on conversion verify: %d (%s)", convertVerifyResponseRecorder.Code, convertVerifyResponseRecorder.Body.String())
	}

	var convertedSessionResponse SessionResponse
	_ = json.NewDecoder(convertVerifyResponseRecorder.Body).Decode(&convertedSessionResponse)
	if convertedSessionResponse.User.ID != anonUserID {
		t.Fatalf("expected user ID %s to remain unchanged on conversion, got: %s", anonUserID, convertedSessionResponse.User.ID)
	}
	if convertedSessionResponse.User.IsAnonymous {
		t.Fatal("expected user to no longer be anonymous")
	}

	// 6. Verify non-existent/expired OTP code -> 400
	nonExistentVerifyPayload, _ := json.Marshal(map[string]any{
		"recipient": "never.requested@example.com",
		"code":      "123456",
		"purpose":   "signin",
	})
	nonExistentVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewReader(nonExistentVerifyPayload))
	nonExistentVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(nonExistentVerifyResponseRecorder, nonExistentVerifyRequest)
	if nonExistentVerifyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on non-existent OTP code verify, got: %d", nonExistentVerifyResponseRecorder.Code)
	}

	// 7. Anonymous user conversion conflict (email conflict -> 409, phone conflict -> 409)
	existingConflictEmail := "existing.conflict@example.com"
	existingConflictPhone := "+15553334444"
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (email, phone, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, $2, 'authenticated', false, clock_timestamp(), clock_timestamp())
	`, existingConflictEmail, existingConflictPhone)

	conflictAnonID := uuid.NewV7().String()
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, 'authenticated', true, clock_timestamp(), clock_timestamp())
	`, conflictAnonID)
	conflictAnonToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     conflictAnonID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 900)

	// Send OTP for conflicting email (existingConflictEmail already belongs to existing user)
	conflictEmailSendPayload, _ := json.Marshal(map[string]any{
		"recipient": existingConflictEmail,
		"purpose":   "signin",
	})
	conflictEmailSendRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/send", bytes.NewReader(conflictEmailSendPayload))
	conflictEmailSendResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(conflictEmailSendResponseRecorder, conflictEmailSendRequest)
	if conflictEmailSendResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on conflict email OTP send: %d", conflictEmailSendResponseRecorder.Code)
	}
	conflictEmailCode, _ := databaseKVStore.Get(ctx, fmt.Sprintf("layr:auth:otp:signin:%s", existingConflictEmail))

	conflictEmailVerifyPayload, _ := json.Marshal(map[string]any{
		"recipient": existingConflictEmail,
		"code":      conflictEmailCode,
		"purpose":   "signin",
	})
	conflictEmailVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewReader(conflictEmailVerifyPayload))
	conflictEmailVerifyRequest.Header.Set("Authorization", "Bearer "+conflictAnonToken)
	conflictEmailVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(conflictEmailVerifyResponseRecorder, conflictEmailVerifyRequest)
	if conflictEmailVerifyResponseRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on anonymous conversion to existing email, got: %d (%s)", conflictEmailVerifyResponseRecorder.Code, conflictEmailVerifyResponseRecorder.Body.String())
	}

	// Send OTP for conflicting phone (existingConflictPhone already belongs to existing user)
	conflictPhoneSendPayload, _ := json.Marshal(map[string]any{
		"recipient": existingConflictPhone,
		"purpose":   "signin",
	})
	conflictPhoneSendRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/send", bytes.NewReader(conflictPhoneSendPayload))
	conflictPhoneSendResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(conflictPhoneSendResponseRecorder, conflictPhoneSendRequest)
	if conflictPhoneSendResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on conflict phone OTP send: %d", conflictPhoneSendResponseRecorder.Code)
	}
	conflictPhoneCode, _ := databaseKVStore.Get(ctx, fmt.Sprintf("layr:auth:otp:signin:%s", existingConflictPhone))

	conflictPhoneVerifyPayload, _ := json.Marshal(map[string]any{
		"recipient": existingConflictPhone,
		"code":      conflictPhoneCode,
		"purpose":   "signin",
	})
	conflictPhoneVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewReader(conflictPhoneVerifyPayload))
	conflictPhoneVerifyRequest.Header.Set("Authorization", "Bearer "+conflictAnonToken)
	conflictPhoneVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(conflictPhoneVerifyResponseRecorder, conflictPhoneVerifyRequest)
	if conflictPhoneVerifyResponseRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on anonymous conversion to existing phone, got: %d (%s)", conflictPhoneVerifyResponseRecorder.Code, conflictPhoneVerifyResponseRecorder.Body.String())
	}

	// 8. Anonymous user conversion via phone -> 200 OK
	phoneAnonID := uuid.NewV7().String()
	_, _ = db.Exec(ctx, `
		INSERT INTO layr_auth.users (id, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, 'authenticated', true, clock_timestamp(), clock_timestamp())
	`, phoneAnonID)
	phoneAnonToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     phoneAnonID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 900)

	convertPhoneRecipient := "+15558765432"
	convertPhoneSendPayload, _ := json.Marshal(map[string]any{
		"recipient": convertPhoneRecipient,
		"purpose":   "signin",
	})
	convertPhoneSendRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/send", bytes.NewReader(convertPhoneSendPayload))
	convertPhoneSendResponseRecorder := httptest.NewRecorder()
	handler.handleOTPSend(convertPhoneSendResponseRecorder, convertPhoneSendRequest)
	convertPhoneCode, _ := databaseKVStore.Get(ctx, fmt.Sprintf("layr:auth:otp:signin:%s", convertPhoneRecipient))

	convertPhoneVerifyPayload, _ := json.Marshal(map[string]any{
		"recipient": convertPhoneRecipient,
		"code":      convertPhoneCode,
		"purpose":   "signin",
	})
	convertPhoneVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewReader(convertPhoneVerifyPayload))
	convertPhoneVerifyRequest.Header.Set("Authorization", "Bearer "+phoneAnonToken)
	convertPhoneVerifyResponseRecorder := httptest.NewRecorder()
	handler.handleOTPVerify(convertPhoneVerifyResponseRecorder, convertPhoneVerifyRequest)
	if convertPhoneVerifyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on anonymous phone conversion, got: %d (%s)", convertPhoneVerifyResponseRecorder.Code, convertPhoneVerifyResponseRecorder.Body.String())
	}
	var phoneConvertedSessionResponse SessionResponse
	_ = json.NewDecoder(convertPhoneVerifyResponseRecorder.Body).Decode(&phoneConvertedSessionResponse)
	if phoneConvertedSessionResponse.User.ID != phoneAnonID {
		t.Fatalf("expected user ID %s to remain unchanged on conversion, got: %s", phoneAnonID, phoneConvertedSessionResponse.User.ID)
	}
	if phoneConvertedSessionResponse.User.IsAnonymous {
		t.Fatal("expected user to no longer be anonymous")
	}

	// Verify events were dispatched
	eventsMutex.Lock()
	defer eventsMutex.Unlock()
	var otpSentFound, otpVerifiedFound, convertedFound bool
	for _, event := range capturedEvents {
		switch event.Type {
		case "auth.otp.sent":
			otpSentFound = true
		case "auth.otp.verified":
			otpVerifiedFound = true
		case "auth.user.converted":
			convertedFound = true
		}
	}
	if !otpSentFound || !otpVerifiedFound || !convertedFound {
		t.Fatalf("missing expected events: sent=%t, verified=%t, converted=%t", otpSentFound, otpVerifiedFound, convertedFound)
	}
}
