package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/auth/otp"
	"layr.sh/auth/threat"
	"layr.sh/core"
)

func TestAuthSignInThreatAndAdaptiveMFAIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanupDatabase := setupTestDatabase(t)
	defer cleanupDatabase()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if loadErr := configManager.Load(ctx); loadErr != nil {
		t.Fatalf("failed to load initial auth config: %v", loadErr)
	}

	var suspiciousAlertDispatched bool
	var dispatchedSuspiciousEmail string
	var dispatchedSuspiciousIP string
	emailWebhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(request.Body).Decode(&payload)
		if messageKind, ok := payload["message_kind"].(string); ok && messageKind == "suspicious_activity" {
			suspiciousAlertDispatched = true
			if to, ok := payload["to"].(string); ok {
				dispatchedSuspiciousEmail = to
			}
			if text, ok := payload["text"].(string); ok {
				dispatchedSuspiciousIP = text
			}
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer emailWebhookServer.Close()

	driverWebhook := "webhook"
	activeConfig := configManager.Get()
	activeConfig.Password.Enabled = true
	activeConfig.Threat.NotifyOnNewDevice = true
	activeConfig.Threat.KnownDevicesMaxDays = 30
	activeConfig.MFA.Enabled = true
	activeConfig.MFA.Policy = "adaptive"
	activeConfig.MFA.RiskTriggers = []string{"new_device", "new_ip"}
	activeConfig.EmailDispatcher = EmailDispatcherConfig{
		Driver:  &driverWebhook,
		Webhook: EmailDispatcherWebhookConfig{URL: emailWebhookServer.URL},
	}
	configManager.Set(activeConfig)

	eventBus := core.NewEventBus(db, cryptoKeyManager)
	defer eventBus.Close()
	testKVStore := newInMemoryKVStore()

	baseHandler := NewBaseHandler(db, configManager, cryptoKeyManager)
	baseHandler.SetEventBus(eventBus)
	baseHandler.SetKVStore(testKVStore)
	mockHTTPClient := &threatMockHTTPClient{}
	baseHandler.SetHTTPClient(mockHTTPClient)
	baseHandler.SetEmailDispatcher(NewEmailDispatcher(db, func() *EmailDispatcherConfig {
		emailDispatcherConfig := activeConfig.EmailDispatcher
		return &emailDispatcherConfig
	}, cryptoKeyManager))

	// Seed User A with MFA enabled and a valid TOTP secret
	userAID := "01918a24-1111-7000-8000-000000000001"
	userAEmail := "usera.mfa@example.com"
	userAPassword := "UserAPassword123!"
	userAHash, _ := baseHandler.hasher.Hash(userAPassword)
	encryptedTOTPSecret, _ := cryptoKeyManager.EncryptField([]byte("JBSWY3DPEHPK3PXP"))

	_, insertUserAErr := db.Exec(ctx, `
		INSERT INTO auth.users (id, email, password_hash, mfa_enabled, encrypted_mfa_secret, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, $2, $3, true, $4, 'authenticated', false, clock_timestamp(), clock_timestamp())
	`, userAID, userAEmail, userAHash, encryptedTOTPSecret)
	if insertUserAErr != nil {
		t.Fatalf("failed to insert user A: %v", insertUserAErr)
	}

	// Seed a prior known session for User A from a known device/IP
	_, insertPriorSessionErr := db.Exec(ctx, `
		INSERT INTO auth.sessions (user_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at)
		VALUES ($1, 'prior_refresh_hash', '10.0.0.1', 'KnownBrowser/1.0', clock_timestamp() + interval '1 day', clock_timestamp())
	`, userAID)
	if insertPriorSessionErr != nil {
		t.Fatalf("failed to insert prior session: %v", insertPriorSessionErr)
	}

	// 1. Failed sign-in with wrong password increments failed attempts
	wrongPasswordPayload, _ := json.Marshal(map[string]any{
		"email":    userAEmail,
		"password": "WrongPassword123!",
	})
	failedSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(wrongPasswordPayload))
	failedSignInRequest.Header.Set("X-Forwarded-For", "203.0.113.50")
	failedSignInRequest.Header.Set("User-Agent", "TestBrowser/1.0")
	failedSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(failedSignInResponseRecorder, failedSignInRequest)
	if failedSignInResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on wrong password, got: %d", failedSignInResponseRecorder.Code)
	}
	if attempts := threat.GetFailedAttempts(ctx, testKVStore, "203.0.113.50"); attempts != 1 {
		t.Fatalf("expected 1 failed attempt for IP, got: %d", attempts)
	}
	if attempts := threat.GetFailedAttempts(ctx, testKVStore, userAID); attempts != 1 {
		t.Fatalf("expected 1 failed attempt for user ID, got: %d", attempts)
	}

	// 2. Successful sign-in on new device triggers Adaptive MFA challenge ticket, resets attempts, and dispatches suspicious alert
	validPasswordPayload, _ := json.Marshal(map[string]any{
		"email":    userAEmail,
		"password": userAPassword,
	})
	adaptiveSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(validPasswordPayload))
	adaptiveSignInRequest.Header.Set("X-Forwarded-For", "203.0.113.50")
	adaptiveSignInRequest.Header.Set("User-Agent", "TestBrowser/1.0")
	adaptiveSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(adaptiveSignInResponseRecorder, adaptiveSignInRequest)
	if adaptiveSignInResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from adaptive sign in, got: %d (%s)", adaptiveSignInResponseRecorder.Code, adaptiveSignInResponseRecorder.Body.String())
	}

	var adaptiveSignInResponse SignInResponse
	if decodeErr := json.NewDecoder(adaptiveSignInResponseRecorder.Body).Decode(&adaptiveSignInResponse); decodeErr != nil {
		t.Fatalf("failed to decode adaptive sign in response: %v", decodeErr)
	}
	if !adaptiveSignInResponse.MFARequired || !strings.HasPrefix(adaptiveSignInResponse.MFATicket, "mfa_tk_") {
		t.Fatalf("expected MFA challenge ticket for new device under adaptive policy, got: %+v", adaptiveSignInResponse)
	}
	if !suspiciousAlertDispatched || dispatchedSuspiciousEmail != userAEmail {
		t.Fatalf("expected suspicious activity alert dispatched for new device login, got: %v (to: %s)", suspiciousAlertDispatched, dispatchedSuspiciousEmail)
	}
	if !strings.Contains(dispatchedSuspiciousIP, "203.0.113.50") {
		t.Fatalf("expected suspicious email body to mention IP 203.0.113.50, got: %s", dispatchedSuspiciousIP)
	}
	// Failed attempts should now be reset to 0
	if attempts := threat.GetFailedAttempts(ctx, testKVStore, "203.0.113.50"); attempts != 0 {
		t.Fatalf("expected 0 failed attempts after successful sign in, got: %d", attempts)
	}

	// 3. Second sign-in from known device/IP (10.0.0.1, KnownBrowser/1.0): low risk -> Adaptive MFA is bypassed!
	knownDeviceSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(validPasswordPayload))
	knownDeviceSignInRequest.Header.Set("X-Forwarded-For", "10.0.0.1")
	knownDeviceSignInRequest.Header.Set("User-Agent", "KnownBrowser/1.0")
	knownDeviceSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(knownDeviceSignInResponseRecorder, knownDeviceSignInRequest)
	if knownDeviceSignInResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from known device sign in, got: %d (%s)", knownDeviceSignInResponseRecorder.Code, knownDeviceSignInResponseRecorder.Body.String())
	}

	var knownDeviceSessionResponse SessionResponse
	if decodeErr := json.NewDecoder(knownDeviceSignInResponseRecorder.Body).Decode(&knownDeviceSessionResponse); decodeErr != nil {
		t.Fatalf("failed to decode known device session response: %v", decodeErr)
	}
	if knownDeviceSessionResponse.AccessToken == "" || knownDeviceSessionResponse.User.ID != userAID {
		t.Fatalf("expected session response without MFA challenge for known device, got: %+v", knownDeviceSessionResponse)
	}

	// 3b. Sign-in with low risk (known User-Agent, but new IP):
	// Evaluates to RiskLevelLow with reason "new_ip". When RiskTriggers contains "new_ip", adaptive MFA triggers.
	lowRiskMatchedSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(validPasswordPayload))
	lowRiskMatchedSignInRequest.Header.Set("X-Forwarded-For", "198.51.100.22")
	lowRiskMatchedSignInRequest.Header.Set("User-Agent", "KnownBrowser/1.0")
	lowRiskMatchedResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(lowRiskMatchedResponseRecorder, lowRiskMatchedSignInRequest)
	if lowRiskMatchedResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from low risk matched sign in, got: %d (%s)", lowRiskMatchedResponseRecorder.Code, lowRiskMatchedResponseRecorder.Body.String())
	}
	var lowRiskMatchedSignInResponse SignInResponse
	if decodeErr := json.NewDecoder(lowRiskMatchedResponseRecorder.Body).Decode(&lowRiskMatchedSignInResponse); decodeErr != nil {
		t.Fatalf("failed to decode low risk matched sign in response: %v", decodeErr)
	}
	if !lowRiskMatchedSignInResponse.MFARequired || !strings.HasPrefix(lowRiskMatchedSignInResponse.MFATicket, "mfa_tk_") {
		t.Fatalf("expected MFA challenge ticket when new_ip matches RiskTriggers, got: %+v", lowRiskMatchedSignInResponse)
	}

	// 3c. When RiskTriggers does NOT match the assessed reason (only triggers on excessive_failed_attempts):
	customRiskConfig := activeConfig
	customRiskConfig.MFA.RiskTriggers = []string{"excessive_failed_attempts"}
	configManager.Set(customRiskConfig)

	lowRiskUnmatchedSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(validPasswordPayload))
	lowRiskUnmatchedSignInRequest.Header.Set("X-Forwarded-For", "198.51.100.23")
	lowRiskUnmatchedSignInRequest.Header.Set("User-Agent", "KnownBrowser/1.0")
	lowRiskUnmatchedResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(lowRiskUnmatchedResponseRecorder, lowRiskUnmatchedSignInRequest)
	if lowRiskUnmatchedResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from low risk unmatched sign in, got: %d (%s)", lowRiskUnmatchedResponseRecorder.Code, lowRiskUnmatchedResponseRecorder.Body.String())
	}
	var lowRiskUnmatchedSessionResponse SessionResponse
	if decodeErr := json.NewDecoder(lowRiskUnmatchedResponseRecorder.Body).Decode(&lowRiskUnmatchedSessionResponse); decodeErr != nil {
		t.Fatalf("failed to decode low risk unmatched session response: %v", decodeErr)
	}
	if lowRiskUnmatchedSessionResponse.AccessToken == "" {
		t.Fatalf("expected session response when risk triggers do not match, got: %+v", lowRiskUnmatchedSessionResponse)
	}

	// 4. Switching MFA Policy to "always" triggers MFA ticket even for known device
	alwaysMFAConfig := activeConfig
	alwaysMFAConfig.MFA.Policy = "always"
	configManager.Set(alwaysMFAConfig)

	alwaysPolicySignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(validPasswordPayload))
	alwaysPolicySignInRequest.Header.Set("X-Forwarded-For", "10.0.0.1")
	alwaysPolicySignInRequest.Header.Set("User-Agent", "KnownBrowser/1.0")
	alwaysPolicySignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(alwaysPolicySignInResponseRecorder, alwaysPolicySignInRequest)
	if alwaysPolicySignInResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from always policy sign in, got: %d (%s)", alwaysPolicySignInResponseRecorder.Code, alwaysPolicySignInResponseRecorder.Body.String())
	}

	var alwaysSignInResponse SignInResponse
	if decodeErr := json.NewDecoder(alwaysPolicySignInResponseRecorder.Body).Decode(&alwaysSignInResponse); decodeErr != nil {
		t.Fatalf("failed to decode always sign in response: %v", decodeErr)
	}
	if !alwaysSignInResponse.MFARequired || !strings.HasPrefix(alwaysSignInResponse.MFATicket, "mfa_tk_") {
		t.Fatalf("expected MFA ticket when policy is always, got: %+v", alwaysSignInResponse)
	}

	// 6. Account locked user sign-in: returns 423 Locked and records failed attempt
	lockedUserID := "01918a24-10cc-7000-8000-000000000002"
	lockedUserEmail := "locked.user@example.com"
	lockedHash, _ := baseHandler.hasher.Hash("AnyPassword123!")
	_, insertLockedErr := db.Exec(ctx, `
		INSERT INTO auth.users (id, email, password_hash, locked_until, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, $2, $3, clock_timestamp() + interval '1 hour', 'authenticated', false, clock_timestamp(), clock_timestamp())
	`, lockedUserID, lockedUserEmail, lockedHash)
	if insertLockedErr != nil {
		t.Fatalf("failed to insert locked user: %v", insertLockedErr)
	}

	lockedPayload, _ := json.Marshal(map[string]any{
		"email":    lockedUserEmail,
		"password": "AnyPassword123!",
	})
	lockedSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(lockedPayload))
	lockedSignInRequest.Header.Set("X-Forwarded-For", "203.0.113.99")
	lockedSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(lockedSignInResponseRecorder, lockedSignInRequest)
	if lockedSignInResponseRecorder.Code != http.StatusLocked {
		t.Fatalf("expected 423 Locked on locked account sign-in, got: %d", lockedSignInResponseRecorder.Code)
	}
	if attempts := threat.GetFailedAttempts(ctx, testKVStore, "203.0.113.99"); attempts != 1 {
		t.Fatalf("expected 1 failed attempt recorded for locked sign-in IP, got: %d", attempts)
	}

	// 7. Update user password with breach checking
	breachCheckConfig := activeConfig
	breachCheckConfig.Password.BreachCheck.Enabled = true
	breachCheckConfig.Password.BreachCheck.FailOpen = false
	configManager.Set(breachCheckConfig)

	// Mock HTTP client returns breached response for "password"
	mockHTTPClient.doFunc = func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("1E4C9B93F3F0682250B6CF8331B7EE68FD8:3861493\r\n")),
			Header:     make(http.Header),
		}, nil
	}

	breachedUpdatePayload, _ := json.Marshal(UpdateUserPasswordRequest{
		CurrentPassword: userAPassword,
		NewPassword:     "password",
	})
	breachedUpdateRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/auth/user/password", bytes.NewReader(breachedUpdatePayload))
	breachedUpdateRequest = withUserAuth(breachedUpdateRequest, userAID, "authenticated", false)
	breachedUpdateResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserPassword(breachedUpdateResponseRecorder, breachedUpdateRequest)
	if breachedUpdateResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(breachedUpdateResponseRecorder.Body.String(), "breach") {
		t.Fatalf("expected 400 on updating to breached password, got: %d (%s)", breachedUpdateResponseRecorder.Code, breachedUpdateResponseRecorder.Body.String())
	}

	// Safe password update succeeds
	mockHTTPClient.doFunc = func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("DIFFERENT_SUFFIX:1\r\n")),
			Header:     make(http.Header),
		}, nil
	}

	safeUpdatePayload, _ := json.Marshal(UpdateUserPasswordRequest{
		CurrentPassword: userAPassword,
		NewPassword:     "NewCleanPassword123!#",
	})
	safeUpdateRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/api/v1/auth/user/password", bytes.NewReader(safeUpdatePayload))
	safeUpdateRequest = withUserAuth(safeUpdateRequest, userAID, "authenticated", false)
	safeUpdateResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserPassword(safeUpdateResponseRecorder, safeUpdateRequest)
	if safeUpdateResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on updating to clean password, got: %d (%s)", safeUpdateResponseRecorder.Code, safeUpdateResponseRecorder.Body.String())
	}

	// 8. OTP Verify with threat tracking and adaptive MFA
	activeOTPConfig := activeConfig
	activeOTPConfig.EmailOTP.Enabled = true
	activeOTPConfig.MFA.Policy = "adaptive"
	activeOTPConfig.MFA.RiskTriggers = []string{"new_device", "new_ip"}
	configManager.Set(activeOTPConfig)

	otpCode := "123456"
	otpCodeHash := otp.HashCode(otpCode)
	_, insertOTPErr := db.Exec(ctx, `
		INSERT INTO auth.otps (id, recipient, purpose, code_hash, attempts, expires_at, created_at)
		VALUES ($1, $2, 'sign_in', $3, 0, clock_timestamp() + interval '15 minutes', clock_timestamp())
	`, "01918a24-2222-7000-8000-000000000001", userAEmail, otpCodeHash)
	if insertOTPErr != nil {
		t.Fatalf("failed to insert test otp: %v", insertOTPErr)
	}

	// 8a. Wrong OTP code records failed attempt
	wrongOTPPayload, _ := json.Marshal(OTPVerifyRequest{
		Recipient: userAEmail,
		Code:      "999999",
	})
	otpVerifyWrongRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewReader(wrongOTPPayload))
	otpVerifyWrongRequest.Header.Set("X-Forwarded-For", "198.51.100.77")
	otpVerifyWrongResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOTPVerify(otpVerifyWrongResponseRecorder, otpVerifyWrongRequest)
	if otpVerifyWrongResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on wrong otp code, got: %d", otpVerifyWrongResponseRecorder.Code)
	}
	if attempts := threat.GetFailedAttempts(ctx, testKVStore, "198.51.100.77"); attempts != 1 {
		t.Fatalf("expected 1 failed attempt recorded for wrong OTP IP, got: %d", attempts)
	}

	// 8b. Valid OTP verification from known device/IP: Adaptive MFA is bypassed
	validOTPPayload, _ := json.Marshal(OTPVerifyRequest{
		Recipient: userAEmail,
		Code:      otpCode,
	})
	otpVerifyKnownRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewReader(validOTPPayload))
	otpVerifyKnownRequest.Header.Set("X-Forwarded-For", "10.0.0.1")
	otpVerifyKnownRequest.Header.Set("User-Agent", "KnownBrowser/1.0")
	otpVerifyKnownResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOTPVerify(otpVerifyKnownResponseRecorder, otpVerifyKnownRequest)
	if otpVerifyKnownResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid otp from known device, got: %d (%s)", otpVerifyKnownResponseRecorder.Code, otpVerifyKnownResponseRecorder.Body.String())
	}
	var otpVerifyKnownSessionResponse SessionResponse
	if decodeErr := json.NewDecoder(otpVerifyKnownResponseRecorder.Body).Decode(&otpVerifyKnownSessionResponse); decodeErr != nil {
		t.Fatalf("failed to decode known device otp session response: %v", decodeErr)
	}
	if otpVerifyKnownSessionResponse.AccessToken == "" {
		t.Fatalf("expected session response without MFA challenge for known device OTP, got: %+v", otpVerifyKnownSessionResponse)
	}

	// 8c. Valid OTP verification from new device: triggers Adaptive MFA challenge ticket
	_, insertNewOTPErr := db.Exec(ctx, `
		INSERT INTO auth.otps (id, recipient, purpose, code_hash, attempts, expires_at, created_at)
		VALUES ($1, $2, 'sign_in', $3, 0, clock_timestamp() + interval '15 minutes', clock_timestamp())
	`, "01918a24-2222-7000-8000-000000000002", userAEmail, otpCodeHash)
	if insertNewOTPErr != nil {
		t.Fatalf("failed to insert second test otp: %v", insertNewOTPErr)
	}

	otpVerifyNewDeviceRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewReader(validOTPPayload))
	otpVerifyNewDeviceRequest.Header.Set("X-Forwarded-For", "203.0.113.88")
	otpVerifyNewDeviceRequest.Header.Set("User-Agent", "BrandNewOTPBrowser/1.0")
	otpVerifyNewDeviceResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOTPVerify(otpVerifyNewDeviceResponseRecorder, otpVerifyNewDeviceRequest)
	if otpVerifyNewDeviceResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on otp from new device, got: %d (%s)", otpVerifyNewDeviceResponseRecorder.Code, otpVerifyNewDeviceResponseRecorder.Body.String())
	}
	var otpVerifyNewDeviceSignInResponse SignInResponse
	if decodeErr := json.NewDecoder(otpVerifyNewDeviceResponseRecorder.Body).Decode(&otpVerifyNewDeviceSignInResponse); decodeErr != nil {
		t.Fatalf("failed to decode new device otp response: %v", decodeErr)
	}
	if !otpVerifyNewDeviceSignInResponse.MFARequired || !strings.HasPrefix(otpVerifyNewDeviceSignInResponse.MFATicket, "mfa_tk_") {
		t.Fatalf("expected MFA ticket for new device OTP under adaptive policy, got: %+v", otpVerifyNewDeviceSignInResponse)
	}

	// 9. OAuth Flow with threat tracking and adaptive MFA
	userAUserRecord := UserRecord{
		ID:         userAID,
		Email:      &userAEmail,
		Role:       "authenticated",
		MFAEnabled: true,
	}

	// 9a. Direct OAuth callback from known device: Adaptive MFA is bypassed
	oauthKnownRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/auth/oauth/google/callback", nil)
	oauthKnownRequest.Header.Set("X-Forwarded-For", "10.0.0.1")
	oauthKnownRequest.Header.Set("User-Agent", "KnownBrowser/1.0")
	oauthKnownRequest.SetPathValue("provider", "google")
	oauthKnownResponseRecorder := httptest.NewRecorder()
	baseHandler.CompleteOAuthFlow(oauthKnownResponseRecorder, oauthKnownRequest, userAUserRecord, OAuthStatePayload{}, false)
	if oauthKnownResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on oauth from known device, got: %d (%s)", oauthKnownResponseRecorder.Code, oauthKnownResponseRecorder.Body.String())
	}
	var oauthKnownSessionResponse SessionResponse
	if decodeErr := json.NewDecoder(oauthKnownResponseRecorder.Body).Decode(&oauthKnownSessionResponse); decodeErr != nil {
		t.Fatalf("failed to decode known device oauth session response: %v", decodeErr)
	}
	if oauthKnownSessionResponse.AccessToken == "" {
		t.Fatalf("expected session response without MFA challenge for known device OAuth, got: %+v", oauthKnownSessionResponse)
	}

	// 9b. Direct OAuth callback from new device: triggers Adaptive MFA challenge ticket & alert
	suspiciousAlertDispatched = false
	dispatchedSuspiciousEmail = ""
	oauthNewDeviceRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/auth/oauth/google/callback", nil)
	oauthNewDeviceRequest.Header.Set("X-Forwarded-For", "203.0.113.77")
	oauthNewDeviceRequest.Header.Set("User-Agent", "BrandNewOAuthBrowser/1.0")
	oauthNewDeviceRequest.SetPathValue("provider", "google")
	oauthNewDeviceResponseRecorder := httptest.NewRecorder()
	baseHandler.CompleteOAuthFlow(oauthNewDeviceResponseRecorder, oauthNewDeviceRequest, userAUserRecord, OAuthStatePayload{}, false)
	if oauthNewDeviceResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on oauth from new device, got: %d (%s)", oauthNewDeviceResponseRecorder.Code, oauthNewDeviceResponseRecorder.Body.String())
	}
	var oauthNewDeviceSignInResponse SignInResponse
	if decodeErr := json.NewDecoder(oauthNewDeviceResponseRecorder.Body).Decode(&oauthNewDeviceSignInResponse); decodeErr != nil {
		t.Fatalf("failed to decode new device oauth response: %v", decodeErr)
	}
	if !oauthNewDeviceSignInResponse.MFARequired || !strings.HasPrefix(oauthNewDeviceSignInResponse.MFATicket, "mfa_tk_") {
		t.Fatalf("expected MFA challenge ticket for new device OAuth, got: %+v", oauthNewDeviceSignInResponse)
	}
	if !suspiciousAlertDispatched || dispatchedSuspiciousEmail != userAEmail {
		t.Fatalf("expected suspicious activity alert for new device OAuth sign-in, got: %v (to: %s)", suspiciousAlertDispatched, dispatchedSuspiciousEmail)
	}
}
