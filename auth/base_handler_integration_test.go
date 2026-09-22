package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"layr.sh/auth/oauth"
	"layr.sh/auth/otp"
	"layr.sh/core"
)

func TestAuthHandlerFullLifecycleIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	db := kernel.DB()

	ctx := context.Background()
	service := NewService(kernel)
	configManager := service.configManager
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

	baseHandler := service.baseHandler
	baseHandler.emailDispatcher = NewEmailDispatcher(kernel, func() *EmailDispatcherConfig { return &authConfig.EmailDispatcher })
	baseHandler.smsDispatcher = NewSMSDispatcher(kernel, func() *SMSDispatcherConfig { return &authConfig.SMSDispatcher })
	databaseKVStore := kernel.KVStore()

	userEmail := "alice.handler@example.com"
	userPassword := "SuperStrongPassword123!"

	// 1. Sign Up via POST /v1/auth/sign-up
	signupPayload := map[string]any{
		"email":    userEmail,
		"password": userPassword,
		"properties": map[string]any{
			"plan": "pro",
		},
	}
	encodedSignup, _ := json.Marshal(signupPayload)
	signupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", bytes.NewReader(encodedSignup))
	signupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(signupResponseRecorder, signupRequest)

	if signupResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from /sign-up, got: %d (body: %s)", signupResponseRecorder.Code, signupResponseRecorder.Body.String())
	}

	var signupAuthTokenResponse AuthTokenResponse
	if decodeErr := json.NewDecoder(signupResponseRecorder.Body).Decode(&signupAuthTokenResponse); decodeErr != nil {
		t.Fatalf("failed to decode sign up response: %v", decodeErr)
	}
	if signupAuthTokenResponse.User.ID == "" || signupAuthTokenResponse.AccessToken == "" {
		t.Fatalf("invalid sign up response: %+v", signupAuthTokenResponse)
	}

	// 2. Sign In via POST /v1/auth/sign-in
	loginPayload := map[string]any{
		"email":    userEmail,
		"password": userPassword,
	}
	encodedLogin, _ := json.Marshal(loginPayload)
	loginRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", bytes.NewReader(encodedLogin))
	loginResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(loginResponseRecorder, loginRequest)

	if loginResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /sign-in, got: %d (body: %s)", loginResponseRecorder.Code, loginResponseRecorder.Body.String())
	}

	var loginAuthTokenResponse AuthTokenResponse
	_ = json.NewDecoder(loginResponseRecorder.Body).Decode(&loginAuthTokenResponse)

	// 3. Token Refresh via POST /v1/auth/token/refresh
	refreshPayload := map[string]any{
		"refresh_token": loginAuthTokenResponse.RefreshToken,
	}
	encodedRefresh, _ := json.Marshal(refreshPayload)
	refreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/token/refresh", bytes.NewReader(encodedRefresh))
	refreshResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRefreshToken(refreshResponseRecorder, refreshRequest)

	if refreshResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /token/refresh, got: %d (body: %s)", refreshResponseRecorder.Code, refreshResponseRecorder.Body.String())
	}

	var refreshAuthTokenResponse AuthTokenResponse
	if decodeErr := json.NewDecoder(refreshResponseRecorder.Body).Decode(&refreshAuthTokenResponse); decodeErr != nil {
		t.Fatalf("failed to decode refresh response: %v", decodeErr)
	}

	// 4. OTP Send & Verify (Email & Phone)
	otpSendPayload := map[string]any{
		"recipient": userEmail,
		"purpose":   "sign_in",
	}
	encodedOTPSend, _ := json.Marshal(otpSendPayload)
	otpSendRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/otp/send", bytes.NewReader(encodedOTPSend))
	otpSendResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSendOTP(otpSendResponseRecorder, otpSendRequest)

	if otpSendResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content from /otp send, got: %d", otpSendResponseRecorder.Code)
	}

	otpCode, err := databaseKVStore.Get(ctx, fmt.Sprintf("auth:otp:sign_in:%s", userEmail))
	if err != nil || otpCode == "" {
		t.Fatalf("failed to get OTP code from kvstore: %v", err)
	}

	// 1. Wrong OTP code for existing OTP -> 400 (increments attempts)
	wrongCodePayload := map[string]any{
		"recipient": userEmail,
		"code":      "000000",
		"purpose":   "sign_in",
	}
	encodedWrongCode, _ := json.Marshal(wrongCodePayload)
	wrongCodeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/otp/verify", bytes.NewReader(encodedWrongCode))
	wrongCodeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyOTP(wrongCodeResponseRecorder, wrongCodeRequest)
	if wrongCodeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on wrong OTP code, got: %d", wrongCodeResponseRecorder.Code)
	}

	// 2. Valid OTP code -> 200 (deletes OTP)
	otpVerifyPayload := map[string]any{
		"recipient": userEmail,
		"code":      otpCode,
		"purpose":   "sign_in",
	}
	encodedOTPVerify, _ := json.Marshal(otpVerifyPayload)
	otpVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/otp/verify", bytes.NewReader(encodedOTPVerify))
	otpVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyOTP(otpVerifyResponseRecorder, otpVerifyRequest)
	if otpVerifyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /otp/verify, got: %d", otpVerifyResponseRecorder.Code)
	}

	// 3. Non-existent / already deleted OTP -> 400
	missingOTPPayload := map[string]any{
		"recipient": "nonexistent@example.com",
		"code":      "123456",
		"purpose":   "sign_in",
	}
	encodedMissingOTP, _ := json.Marshal(missingOTPPayload)
	missingOTPRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/otp/verify", bytes.NewReader(encodedMissingOTP))
	missingOTPResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyOTP(missingOTPResponseRecorder, missingOTPRequest)
	if missingOTPResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on non-existent OTP, got: %d", missingOTPResponseRecorder.Code)
	}

	// OTP with Phone Number
	phoneOTPPayload := map[string]any{
		"recipient": "+1234567890",
		"purpose":   "sign_in",
	}
	encodedPhoneOTP, _ := json.Marshal(phoneOTPPayload)
	phoneOTPRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/otp/send", bytes.NewReader(encodedPhoneOTP))
	phoneOTPResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSendOTP(phoneOTPResponseRecorder, phoneOTPRequest)
	if phoneOTPResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on phone OTP send: %d", phoneOTPResponseRecorder.Code)
	}

	phoneCode, err := databaseKVStore.Get(ctx, fmt.Sprintf("auth:otp:sign_in:%s", "+1234567890"))
	if err != nil || phoneCode == "" {
		t.Fatalf("failed to get phone OTP code from kvstore: %v", err)
	}

	phoneVerifyPayload := map[string]any{
		"recipient": "+1234567890",
		"code":      phoneCode,
	}
	encodedPhoneVerify, _ := json.Marshal(phoneVerifyPayload)
	phoneVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/otp/verify", bytes.NewReader(encodedPhoneVerify))
	phoneVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyOTP(phoneVerifyResponseRecorder, phoneVerifyRequest)
	if phoneVerifyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on phone OTP verify: %d", phoneVerifyResponseRecorder.Code)
	}

	// 5. Password Reset Request & Confirm
	resetPayload := map[string]any{
		"email": userEmail,
	}
	encodedReset, _ := json.Marshal(resetPayload)
	resetRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/request", bytes.NewReader(encodedReset))
	resetResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRequestPasswordReset(resetResponseRecorder, resetRequest)
	if resetResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content from /password-reset/request, got: %d", resetResponseRecorder.Code)
	}

	resetCode, err := databaseKVStore.Get(ctx, fmt.Sprintf("auth:otp:password_reset:%s", userEmail))
	if err != nil || resetCode == "" {
		t.Fatalf("failed to get password reset code from kvstore: %v", err)
	}

	// Confirm with wrong code
	wrongConfirmPayload := map[string]any{
		"email":    userEmail,
		"code":     "000000",
		"password": "NewSuperPassword123!",
	}
	encodedWrongConfirm, _ := json.Marshal(wrongConfirmPayload)
	wrongConfirmRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/confirm", bytes.NewReader(encodedWrongConfirm))
	wrongConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPasswordReset(wrongConfirmResponseRecorder, wrongConfirmRequest)
	if wrongConfirmResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on wrong reset code, got: %d", wrongConfirmResponseRecorder.Code)
	}

	// Confirm with valid code and new password
	validConfirmPayload := map[string]any{
		"email":    userEmail,
		"code":     resetCode,
		"password": "NewSuperPassword123!",
	}
	encodedValidConfirm, _ := json.Marshal(validConfirmPayload)
	validConfirmRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/password-reset/confirm", bytes.NewReader(encodedValidConfirm))
	validConfirmResponseRecorder := httptest.NewRecorder()
	baseHandler.handleConfirmPasswordReset(validConfirmResponseRecorder, validConfirmRequest)
	if validConfirmResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on valid password reset confirm, got: %d (body: %s)", validConfirmResponseRecorder.Code, validConfirmResponseRecorder.Body.String())
	}

	// 6. Passkey Sign Up & Verify
	passkeyOptionsPayload := map[string]any{
		"user_id":   signupAuthTokenResponse.User.ID,
		"user_name": "Alice",
	}
	encodedPasskeyOptions, _ := json.Marshal(passkeyOptionsPayload)
	passkeyOptionsRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/passkeys/sign-up", bytes.NewReader(encodedPasskeyOptions))
	passkeyOptionsResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignUp(passkeyOptionsResponseRecorder, passkeyOptionsRequest)
	if passkeyOptionsResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /passkeys/sign-up, got: %d", passkeyOptionsResponseRecorder.Code)
	}

	var passkeyOptions struct {
		Challenge string `json:"challenge"`
	}
	_ = json.NewDecoder(passkeyOptionsResponseRecorder.Body).Decode(&passkeyOptions)

	passkeyVerifyPayload := map[string]any{
		"user_id":       signupAuthTokenResponse.User.ID,
		"challenge":     passkeyOptions.Challenge,
		"credential_id": "cred-id-12345",
		"public_key":    "public-key-es256",
		"friendly_name": "Alice MacBook",
	}
	encodedPasskeyVerify, _ := json.Marshal(passkeyVerifyPayload)
	passkeyVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/passkeys/sign-up/verify", bytes.NewReader(encodedPasskeyVerify))
	passkeyVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignUp(passkeyVerifyResponseRecorder, passkeyVerifyRequest)
	if passkeyVerifyResponseRecorder.Code != http.StatusOK && passkeyVerifyResponseRecorder.Code != http.StatusCreated {
		t.Fatalf("expected 200/201 on /passkeys/sign-up/verify, got: %d (body: %s)", passkeyVerifyResponseRecorder.Code, passkeyVerifyResponseRecorder.Body.String())
	}

	// 7. Passkey Sign In Options & Verify
	passkeySignInOptionsRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/passkeys/sign-in", strings.NewReader(`{}`))
	passkeySignInOptionsResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignIn(passkeySignInOptionsResponseRecorder, passkeySignInOptionsRequest)
	if passkeySignInOptionsResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /passkeys/sign-in, got: %d", passkeySignInOptionsResponseRecorder.Code)
	}

	var passkeySignInOptions struct {
		Challenge string `json:"challenge"`
	}
	_ = json.NewDecoder(passkeySignInOptionsResponseRecorder.Body).Decode(&passkeySignInOptions)

	passkeySignInVerifyPayload := map[string]any{
		"challenge":     passkeySignInOptions.Challenge,
		"credential_id": "cred-id-12345",
	}
	encodedSignInVerify, _ := json.Marshal(passkeySignInVerifyPayload)
	passkeySignInVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/passkeys/sign-in/verify", bytes.NewReader(encodedSignInVerify))
	passkeySignInVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignIn(passkeySignInVerifyResponseRecorder, passkeySignInVerifyRequest)
	if passkeySignInVerifyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /passkeys/sign-in/verify, got: %d (body: %s)", passkeySignInVerifyResponseRecorder.Code, passkeySignInVerifyResponseRecorder.Body.String())
	}

	// 8. TOTP MFA Setup & Verify
	mfaSetupPayload := map[string]any{
		"user_id": signupAuthTokenResponse.User.ID,
	}
	encodedMFASetup, _ := json.Marshal(mfaSetupPayload)
	mfaSetupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/mfa/setup", bytes.NewReader(encodedMFASetup))
	mfaSetupRequest.Header.Set("Authorization", "Bearer "+loginAuthTokenResponse.AccessToken)
	mfaSetupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSetupMFA(mfaSetupResponseRecorder, mfaSetupRequest)

	if mfaSetupResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /mfa/setup, got: %d (body: %s)", mfaSetupResponseRecorder.Code, mfaSetupResponseRecorder.Body.String())
	}

	var mfaSetupResponse struct {
		Secret string `json:"secret"`
	}
	_ = json.NewDecoder(mfaSetupResponseRecorder.Body).Decode(&mfaSetupResponse)

	currentTOTPCode, err := baseHandler.totpManager.GenerateCode(mfaSetupResponse.Secret, time.Now())
	if err == nil && currentTOTPCode != "" {
		mfaVerifyPayload := map[string]any{
			"user_id": signupAuthTokenResponse.User.ID,
			"code":    currentTOTPCode,
		}
		encodedMFAVerify, _ := json.Marshal(mfaVerifyPayload)
		mfaVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/mfa/verify", bytes.NewReader(encodedMFAVerify))
		mfaVerifyRequest.Header.Set("Authorization", "Bearer "+loginAuthTokenResponse.AccessToken)
		mfaVerifyResponseRecorder := httptest.NewRecorder()
		baseHandler.handleVerifyMFA(mfaVerifyResponseRecorder, mfaVerifyRequest)
		if mfaVerifyResponseRecorder.Code != http.StatusOK {
			t.Fatalf("expected 200 OK on /mfa/verify, got: %d (body: %s)", mfaVerifyResponseRecorder.Code, mfaVerifyResponseRecorder.Body.String())
		}
	}

	// 9. OAuth Authorize, Callback, Token, and UserInfo
	oauth.SetHTTPClient(&mockOAuthClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			if strings.Contains(request.URL.Path, "token") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"mock-oauth-token","token_type":"Bearer"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"sub":"oauth-user-777","email":"oauth777@gmail.com","name":"OAuth User"}`)),
			}, nil
		},
	})
	defer oauth.SetHTTPClient(nil)

	activeConfig := configManager.Get()
	activeConfig.OAuthProviders["google"] = OAuthProviderConfig{
		Enabled:      true,
		Preset:       "google",
		ClientID:     "google-client-id",
		ClientSecret: "google-secret",
	}
	configManager.Set(activeConfig)

	oauthAuthRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/auth/oauth/google/authorize?redirect_uri=http://localhost:8080/callback&state=valid-state", nil)
	oauthAuthRequest.SetPathValue("provider", "google")
	oauthAuthResponseRecorder := httptest.NewRecorder()
	baseHandler.handleAuthorizeOAuth(oauthAuthResponseRecorder, oauthAuthRequest)
	if oauthAuthResponseRecorder.Code != http.StatusFound && oauthAuthResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 302 redirect on /oauth/google/authorize, got: %d", oauthAuthResponseRecorder.Code)
	}

	oauthCallbackRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/auth/oauth/google/callback?code=valid-code&state=valid-state", nil)
	oauthCallbackRequest.SetPathValue("provider", "google")
	oauthCallbackResponseRecorder := httptest.NewRecorder()
	baseHandler.handleProcessOAuthCallback(oauthCallbackResponseRecorder, oauthCallbackRequest)
	if oauthCallbackResponseRecorder.Code != http.StatusOK && oauthCallbackResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected successful oauth callback, got: %d (body: %s)", oauthCallbackResponseRecorder.Code, oauthCallbackResponseRecorder.Body.String())
	}

	// 10. User Export via POST /v1/auth/users/{user_id}/export
	_, err = db.Exec(ctx, `
		INSERT INTO auth.identities (user_id, provider, provider_user_id, properties, created_at, last_sign_in_at)
		VALUES ($1, 'github', 'gh_user_123', '{"login":"alice"}'::jsonb, clock_timestamp(), clock_timestamp())
	`, signupAuthTokenResponse.User.ID)
	if err != nil {
		t.Fatalf("failed to insert test identity for export: %v", err)
	}

	exportRequest := core.WithTestAuthContext(httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/users/"+signupAuthTokenResponse.User.ID+"/export", nil), signupAuthTokenResponse.User.ID, "authenticated", false)
	exportRequest.SetPathValue("user_id", signupAuthTokenResponse.User.ID)
	exportRequest.Header.Set("Authorization", "Bearer "+loginAuthTokenResponse.AccessToken)
	exportResponseRecorder := httptest.NewRecorder()
	baseHandler.handleExportUser(exportResponseRecorder, exportRequest)
	if exportResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on user export, got: %d (body: %s)", exportResponseRecorder.Code, exportResponseRecorder.Body.String())
	}
	var exportUserResponse ExportUserResponse
	if decodeErr := json.NewDecoder(exportResponseRecorder.Body).Decode(&exportUserResponse); decodeErr != nil {
		t.Fatalf("failed to decode user export response: %v", decodeErr)
	}
	if len(exportUserResponse.Identities) == 0 {
		t.Fatal("expected exported identities to be populated")
	}

	// Test non-existent user export -> 404 Not Found
	nonExistentUserID := "018f2234-5678-789a-bcde-f0123456789a"
	nonExistentToken := kernel.JWTSigner().GenerateAccessToken(core.JWTClaims{
		Subject: nonExistentUserID,
		Role:    "authenticated",
	}, 900)
	nonExistentCtx := core.WithAuthContext(ctx, core.AuthContext{
		UserID: nonExistentUserID,
		JWT: core.JWTClaims{
			Subject: nonExistentUserID,
			Role:    "authenticated",
		},
	})
	nonExistentExportRequest := httptest.NewRequestWithContext(nonExistentCtx, http.MethodPost, "/v1/auth/users/"+nonExistentUserID+"/export", nil)
	nonExistentExportRequest.SetPathValue("user_id", nonExistentUserID)
	nonExistentExportRequest.Header.Set("Authorization", "Bearer "+nonExistentToken)
	nonExistentExportResponseRecorder := httptest.NewRecorder()
	baseHandler.handleExportUser(nonExistentExportResponseRecorder, nonExistentExportRequest)
	if nonExistentExportResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found on non-existent user export, got: %d", nonExistentExportResponseRecorder.Code)
	}

	// 11. Sign Out via POST /v1/auth/sign-out with active refresh token to trigger SessionDeletedEvent
	_, _ = db.Exec(ctx, "UPDATE auth.sessions SET client_id = 'client-signout-test'")
	signOutPayload, _ := json.Marshal(RefreshTokenInput{RefreshToken: refreshAuthTokenResponse.RefreshToken})
	signOutRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-out", bytes.NewReader(signOutPayload))
	signOutResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignOut(signOutResponseRecorder, signOutRequest)
	if signOutResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content from /sign-out, got: %d", signOutResponseRecorder.Code)
	}
}

func TestAuthAnonymousSignInAndInPlaceConversionIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	db := kernel.DB()

	ctx := context.Background()
	service := NewService(kernel)
	configManager := service.configManager
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

	var capturedEvents []core.Event
	var eventsMutex sync.Mutex
	kernel.EventBus().Subscribe("auth.user.*", func(eventCtx context.Context, event core.Event) error {
		eventsMutex.Lock()
		defer eventsMutex.Unlock()
		capturedEvents = append(capturedEvents, event)
		return nil
	})

	baseHandler := service.baseHandler
	baseHandler.emailDispatcher = NewEmailDispatcher(kernel, func() *EmailDispatcherConfig { return &authConfig.EmailDispatcher })
	baseHandler.smsDispatcher = NewSMSDispatcher(kernel, func() *SMSDispatcherConfig { return &authConfig.SMSDispatcher })

	// 1. Anonymous Sign-In (POST /v1/auth/anonymous)
	anonymousPayload, _ := json.Marshal(map[string]any{
		"properties": map[string]any{
			"theme":      "dark",
			"cart_count": 3,
		},
	})
	anonymousRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/anonymous", bytes.NewReader(anonymousPayload))
	anonymousResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignInAnonymous(anonymousResponseRecorder, anonymousRequest)

	if anonymousResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on anonymous sign-in, got: %d (%s)", anonymousResponseRecorder.Code, anonymousResponseRecorder.Body.String())
	}

	var anonymousAuthTokenResponse AuthTokenResponse
	if err := json.NewDecoder(anonymousResponseRecorder.Body).Decode(&anonymousAuthTokenResponse); err != nil {
		t.Fatalf("failed to decode anonymous sign-in response: %v", err)
	}

	anonymousUserID := anonymousAuthTokenResponse.User.ID
	if anonymousUserID == "" {
		t.Fatal("expected non-empty anonymous user ID")
	}
	if anonymousAuthTokenResponse.User.Email != nil {
		t.Fatalf("expected nil email for anonymous user, got: %v", *anonymousAuthTokenResponse.User.Email)
	}
	if anonymousAuthTokenResponse.User.Phone != nil {
		t.Fatalf("expected nil phone for anonymous user, got: %v", *anonymousAuthTokenResponse.User.Phone)
	}
	if !anonymousAuthTokenResponse.User.IsAnonymous {
		t.Fatal("expected user.is_anonymous to be true")
	}
	if anonymousAuthTokenResponse.User.Role != "authenticated" {
		t.Fatalf("expected role authenticated, got: %s", anonymousAuthTokenResponse.User.Role)
	}
	if anonymousAuthTokenResponse.User.Properties["theme"] != "dark" {
		t.Fatalf("expected theme property 'dark', got: %v", anonymousAuthTokenResponse.User.Properties["theme"])
	}

	// Verify Ed25519 JWT claims
	anonymousJWTClaims, claimErr := kernel.JWTSigner().VerifyAccessToken(anonymousAuthTokenResponse.AccessToken)
	if claimErr != nil {
		t.Fatalf("failed to verify anonymous access token: %v", claimErr)
	}
	if !anonymousJWTClaims.IsAnonymous {
		t.Fatal("expected JWT claims is_anonymous to be true")
	}
	if anonymousJWTClaims.Subject != anonymousUserID {
		t.Fatalf("expected JWT subject %s, got: %s", anonymousUserID, anonymousJWTClaims.Subject)
	}

	// Verify auth.user.signed_up event emitted with is_anonymous: true
	eventsMutex.Lock()
	var createdEventFound bool
	for _, capturedEvent := range capturedEvents {
		if capturedEvent.Type == "auth.user.signed_up" {
			if capturedEvent.ResourceID != nil && *capturedEvent.ResourceID == anonymousUserID {
				createdEventFound = true
				break
			}
		}
	}
	eventsMutex.Unlock()
	if !createdEventFound {
		t.Fatal("expected auth.user.signed_up event to be emitted with anonymous user id")
	}

	// 2. In-Place Auto-Conversion via Sign-Up (POST /v1/auth/sign-up)
	targetEmail := fmt.Sprintf("converted_%d@example.com", time.Now().UnixNano())
	targetPassword := "ConvertedSecurePassword123!"

	signupConversionPayload, _ := json.Marshal(map[string]any{
		"email":    targetEmail,
		"password": targetPassword,
		"properties": map[string]any{
			"subscribed": true,
		},
	})
	signupConversionRequest := core.WithTestAuthContext(httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", bytes.NewReader(signupConversionPayload)), anonymousUserID, "anon", true)
	signupConversionRequest.Header.Set("Authorization", "Bearer "+anonymousAuthTokenResponse.AccessToken)
	signupConversionResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(signupConversionResponseRecorder, signupConversionRequest)

	if signupConversionResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on conversion sign up, got: %d (%s)", signupConversionResponseRecorder.Code, signupConversionResponseRecorder.Body.String())
	}

	var signupConversionAuthTokenResponse AuthTokenResponse
	if err := json.NewDecoder(signupConversionResponseRecorder.Body).Decode(&signupConversionAuthTokenResponse); err != nil {
		t.Fatalf("failed to decode conversion sign up response: %v", err)
	}

	if signupConversionAuthTokenResponse.User.ID != anonymousUserID {
		t.Fatalf("expected user ID to remain %s after conversion, got: %s", anonymousUserID, signupConversionAuthTokenResponse.User.ID)
	}
	if signupConversionAuthTokenResponse.User.IsAnonymous {
		t.Fatal("expected user.is_anonymous to be false after conversion")
	}
	if signupConversionAuthTokenResponse.User.Email == nil || *signupConversionAuthTokenResponse.User.Email != targetEmail {
		t.Fatalf("expected email %s, got: %v", targetEmail, signupConversionAuthTokenResponse.User.Email)
	}

	// Asserts properties are preserved and merged
	if signupConversionAuthTokenResponse.User.Properties["theme"] != "dark" || signupConversionAuthTokenResponse.User.Properties["subscribed"] != true {
		t.Fatalf("expected merged properties containing both theme and subscribed, got: %+v", signupConversionAuthTokenResponse.User.Properties)
	}

	convertedJWTClaims, tokenErr := kernel.JWTSigner().VerifyAccessToken(signupConversionAuthTokenResponse.AccessToken)
	if tokenErr != nil {
		t.Fatalf("failed to verify converted access token: %v", tokenErr)
	}
	if convertedJWTClaims.IsAnonymous {
		t.Fatal("expected converted JWT claims is_anonymous to be false")
	}
	if convertedJWTClaims.Subject != anonymousUserID {
		t.Fatalf("expected converted JWT subject %s, got: %s", anonymousUserID, convertedJWTClaims.Subject)
	}

	// Verify auth.user.converted event emitted
	eventsMutex.Lock()
	var convertedEventFound bool
	for _, capturedEvent := range capturedEvents {
		if capturedEvent.Type == "auth.user.converted" {
			if capturedEvent.ResourceID != nil && *capturedEvent.ResourceID == anonymousUserID {
				convertedEventFound = true
				break
			}
		}
	}
	eventsMutex.Unlock()
	if !convertedEventFound {
		t.Fatal("expected auth.user.converted event to be emitted")
	}

	// 3. Unauthenticated Sign-Up creates a brand new user as normal
	brandNewEmail := fmt.Sprintf("brand_new_%d@example.com", time.Now().UnixNano())
	brandNewPassword := "BrandNewPassword123!"

	brandNewPayload, _ := json.Marshal(map[string]any{
		"email":    brandNewEmail,
		"password": brandNewPassword,
	})
	brandNewRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", bytes.NewReader(brandNewPayload))
	brandNewResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(brandNewResponseRecorder, brandNewRequest)

	if brandNewResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on brand new sign up, got: %d (%s)", brandNewResponseRecorder.Code, brandNewResponseRecorder.Body.String())
	}

	var brandNewAuthTokenResponse AuthTokenResponse
	if err := json.NewDecoder(brandNewResponseRecorder.Body).Decode(&brandNewAuthTokenResponse); err != nil {
		t.Fatalf("failed to decode brand new sign up response: %v", err)
	}

	if brandNewAuthTokenResponse.User.ID == anonymousUserID {
		t.Fatalf("expected brand new user ID, got same anonymous user ID: %s", brandNewAuthTokenResponse.User.ID)
	}
	if brandNewAuthTokenResponse.User.IsAnonymous {
		t.Fatal("expected brand new user is_anonymous to be false")
	}
	if brandNewAuthTokenResponse.User.Email == nil || *brandNewAuthTokenResponse.User.Email != brandNewEmail {
		t.Fatalf("expected email %s, got: %v", brandNewEmail, brandNewAuthTokenResponse.User.Email)
	}

	// 4. In-Place Auto-Conversion via OTP Verify (POST /v1/auth/otp/verify)
	secondAnonymousRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/anonymous", nil)
	secondAnonymousResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignInAnonymous(secondAnonymousResponseRecorder, secondAnonymousRequest)
	if secondAnonymousResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on second anonymous sign-in, got: %d", secondAnonymousResponseRecorder.Code)
	}

	var secondAnonymousAuthTokenResponse AuthTokenResponse
	if err := json.NewDecoder(secondAnonymousResponseRecorder.Body).Decode(&secondAnonymousAuthTokenResponse); err != nil {
		t.Fatalf("failed to decode second anonymous response: %v", err)
	}
	secondAnonymousUserID := secondAnonymousAuthTokenResponse.User.ID

	otpRecipient := fmt.Sprintf("otp_converted_%d@example.com", time.Now().UnixNano())
	otpCode := "778899"
	otpHash := otp.HashCode(otpCode)
	otpExpiresAt := time.Now().UTC().Add(15 * time.Minute)

	_, execErr := db.Exec(ctx, `
		INSERT INTO auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'sign_in', 0, $3, clock_timestamp())
	`, otpRecipient, otpHash, otpExpiresAt)
	if execErr != nil {
		t.Fatalf("failed to insert test OTP: %v", execErr)
	}

	otpVerifyPayload, _ := json.Marshal(map[string]any{
		"recipient": otpRecipient,
		"code":      otpCode,
		"purpose":   "sign_in",
	})
	otpVerifyRequest := core.WithTestAuthContext(httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/otp/verify", bytes.NewReader(otpVerifyPayload)), secondAnonymousUserID, "anon", true)
	otpVerifyRequest.Header.Set("Authorization", "Bearer "+secondAnonymousAuthTokenResponse.AccessToken)
	otpVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyOTP(otpVerifyResponseRecorder, otpVerifyRequest)

	if otpVerifyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on OTP conversion verify, got: %d (%s)", otpVerifyResponseRecorder.Code, otpVerifyResponseRecorder.Body.String())
	}

	var otpVerifyAuthTokenResponse AuthTokenResponse
	if err := json.NewDecoder(otpVerifyResponseRecorder.Body).Decode(&otpVerifyAuthTokenResponse); err != nil {
		t.Fatalf("failed to decode OTP conversion verify response: %v", err)
	}

	if otpVerifyAuthTokenResponse.User.ID != secondAnonymousUserID {
		t.Fatalf("expected user ID %s to remain unchanged on OTP conversion, got: %s", secondAnonymousUserID, otpVerifyAuthTokenResponse.User.ID)
	}
	if otpVerifyAuthTokenResponse.User.IsAnonymous {
		t.Fatal("expected user.is_anonymous to be false after OTP conversion")
	}
	if otpVerifyAuthTokenResponse.User.Email == nil || *otpVerifyAuthTokenResponse.User.Email != otpRecipient {
		t.Fatalf("expected verified email %s, got: %v", otpRecipient, otpVerifyAuthTokenResponse.User.Email)
	}

	var emailVerifiedAt *time.Time
	queryEmailErr := db.QueryRow(ctx, "SELECT email_verified_at FROM auth.users WHERE id = $1", secondAnonymousUserID).Scan(&emailVerifiedAt)
	if queryEmailErr != nil || emailVerifiedAt == nil {
		t.Fatalf("expected email_verified_at to be populated in DB after OTP verify: %v", queryEmailErr)
	}

	// 5. Updating User Email converts anonymous user to regular user
	thirdAnonymousRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/anonymous", nil)
	thirdAnonymousResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignInAnonymous(thirdAnonymousResponseRecorder, thirdAnonymousRequest)
	var thirdAnonymousAuthTokenResponse AuthTokenResponse
	_ = json.NewDecoder(thirdAnonymousResponseRecorder.Body).Decode(&thirdAnonymousAuthTokenResponse)
	thirdAnonymousUserID := thirdAnonymousAuthTokenResponse.User.ID

	updateEmail := fmt.Sprintf("update_email_%d@example.com", time.Now().UnixNano())
	patchEmailPayload, _ := json.Marshal(UpdateUserEmailInput{Email: updateEmail})
	patchEmailRequest := core.WithTestAuthContext(httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/auth/user/email", bytes.NewReader(patchEmailPayload)), thirdAnonymousUserID, "anon", true)
	patchEmailRequest.Header.Set("Authorization", "Bearer "+thirdAnonymousAuthTokenResponse.AccessToken)
	patchEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserEmail(patchEmailResponseRecorder, patchEmailRequest)

	if patchEmailResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on update user email, got: %d (%s)", patchEmailResponseRecorder.Code, patchEmailResponseRecorder.Body.String())
	}

	var isAnonymousAfterEmailUpdate bool
	var emailAfterUpdate *string
	queryUpdateErr := db.QueryRow(ctx, "SELECT is_anonymous, email FROM auth.users WHERE id = $1", thirdAnonymousUserID).Scan(&isAnonymousAfterEmailUpdate, &emailAfterUpdate)
	if queryUpdateErr != nil {
		t.Fatalf("failed to query user after email update: %v", queryUpdateErr)
	}
	if isAnonymousAfterEmailUpdate {
		t.Fatal("expected is_anonymous=false after updating user email")
	}
	if emailAfterUpdate == nil || *emailAfterUpdate != updateEmail {
		t.Fatalf("expected email %s, got: %v", updateEmail, emailAfterUpdate)
	}

	// 6. Updating User Phone converts anonymous user to regular user
	fourthAnonymousRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/anonymous", nil)
	fourthAnonymousResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignInAnonymous(fourthAnonymousResponseRecorder, fourthAnonymousRequest)
	var fourthAnonymousAuthTokenResponse AuthTokenResponse
	_ = json.NewDecoder(fourthAnonymousResponseRecorder.Body).Decode(&fourthAnonymousAuthTokenResponse)
	fourthAnonymousUserID := fourthAnonymousAuthTokenResponse.User.ID

	updatePhone := fmt.Sprintf("+1415%07d", time.Now().UnixNano()%10000000)
	patchPhonePayload, _ := json.Marshal(UpdateUserPhoneInput{Phone: updatePhone})
	patchPhoneRequest := core.WithTestAuthContext(httptest.NewRequestWithContext(ctx, http.MethodPatch, "/v1/auth/user/phone", bytes.NewReader(patchPhonePayload)), fourthAnonymousUserID, "anon", true)
	patchPhoneRequest.Header.Set("Authorization", "Bearer "+fourthAnonymousAuthTokenResponse.AccessToken)
	patchPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleUpdateUserPhone(patchPhoneResponseRecorder, patchPhoneRequest)

	if patchPhoneResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on update user phone, got: %d (%s)", patchPhoneResponseRecorder.Code, patchPhoneResponseRecorder.Body.String())
	}

	var isAnonymousAfterPhoneUpdate bool
	var phoneAfterUpdate *string
	queryPhoneErr := db.QueryRow(ctx, "SELECT is_anonymous, phone FROM auth.users WHERE id = $1", fourthAnonymousUserID).Scan(&isAnonymousAfterPhoneUpdate, &phoneAfterUpdate)
	if queryPhoneErr != nil {
		t.Fatalf("failed to query user after phone update: %v", queryPhoneErr)
	}
	if isAnonymousAfterPhoneUpdate {
		t.Fatal("expected is_anonymous=false after updating user phone")
	}
	if phoneAfterUpdate == nil || *phoneAfterUpdate != updatePhone {
		t.Fatalf("expected phone %s, got: %v", updatePhone, phoneAfterUpdate)
	}
}

func TestAuthHandlerCredentialsAndSessionFlowsIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	db := kernel.DB()

	ctx := context.Background()
	service := NewService(kernel)
	configManager := service.configManager
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	authConfig := configManager.Get()
	authConfig.RateLimiting.Enabled = true
	authConfig.RateLimiting.MaxSignInAttempts = 2
	authConfig.RateLimiting.WindowDurationSeconds = 60
	if err := configManager.Save(ctx, authConfig); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	baseHandler := service.baseHandler

	// 1. Phone Sign-Up and Phone Sign-In
	phoneUser := "+12025550199"
	phonePassword := "SuperSecretPassword123!"

	phoneSignupPayload, _ := json.Marshal(SignUpInput{
		Phone:    phoneUser,
		Password: phonePassword,
	})
	phoneSignupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", bytes.NewReader(phoneSignupPayload))
	phoneSignupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(phoneSignupResponseRecorder, phoneSignupRequest)
	if phoneSignupResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on phone sign-up, got: %d (%s)", phoneSignupResponseRecorder.Code, phoneSignupResponseRecorder.Body.String())
	}

	// Sign-in with wrong password -> 401
	wrongPassPayload, _ := json.Marshal(SignInInput{
		Phone:    phoneUser,
		Password: "WrongPassword999!",
	})
	wrongPassRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", bytes.NewReader(wrongPassPayload))
	wrongPassResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(wrongPassResponseRecorder, wrongPassRequest)
	if wrongPassResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on wrong password phone sign-in, got: %d", wrongPassResponseRecorder.Code)
	}

	// Sign-in with non-existent phone/user -> 401
	missingUserPayload, _ := json.Marshal(SignInInput{
		Phone:    "+12025550100",
		Password: "Password123!",
	})
	missingUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", bytes.NewReader(missingUserPayload))
	missingUserResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(missingUserResponseRecorder, missingUserRequest)
	if missingUserResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing user phone sign-in, got: %d", missingUserResponseRecorder.Code)
	}

	// Sign-in with valid phone and password -> 200
	validPhonePayload, _ := json.Marshal(SignInInput{
		Phone:    phoneUser,
		Password: phonePassword,
	})
	validPhoneRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", bytes.NewReader(validPhonePayload))
	validPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(validPhoneResponseRecorder, validPhoneRequest)
	if validPhoneResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on valid phone sign-in, got: %d", validPhoneResponseRecorder.Code)
	}

	// 2. Sign-in Rate Limiting (exceed MaxSigninAttempts)
	rateLimitedEmail := "ratelimited@example.com"
	for i := 0; i < 2; i++ {
		rateAttemptPayload, _ := json.Marshal(SignInInput{
			Email:    rateLimitedEmail,
			Password: "WrongPassword123!",
		})
		rateAttemptRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", bytes.NewReader(rateAttemptPayload))
		rateAttemptResponseRecorder := httptest.NewRecorder()
		baseHandler.handleSignIn(rateAttemptResponseRecorder, rateAttemptRequest)
	}
	// 3rd attempt exceeds limit of 2
	rateBlockedPayload, _ := json.Marshal(SignInInput{
		Email:    rateLimitedEmail,
		Password: "WrongPassword123!",
	})
	rateBlockedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", bytes.NewReader(rateBlockedPayload))
	rateBlockedResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(rateBlockedResponseRecorder, rateBlockedRequest)
	if rateBlockedResponseRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests on rate limit exceeded, got: %d (%s)", rateBlockedResponseRecorder.Code, rateBlockedResponseRecorder.Body.String())
	}

	// 3. Locked User Account Sign-In
	var phoneUserID string
	err := db.QueryRow(ctx, "SELECT id FROM auth.users WHERE phone = $1", phoneUser).Scan(&phoneUserID)
	if err != nil {
		t.Fatalf("failed to query phone user ID: %v", err)
	}
	_, err = db.Exec(ctx, "UPDATE auth.users SET locked_until = clock_timestamp() + interval '1 hour' WHERE id = $1", phoneUserID)
	if err != nil {
		t.Fatalf("failed to lock user account: %v", err)
	}

	lockedSignInPayload, _ := json.Marshal(SignInInput{
		Phone:    phoneUser,
		Password: phonePassword,
	})
	lockedSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", bytes.NewReader(lockedSignInPayload))
	lockedSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(lockedSignInResponseRecorder, lockedSignInRequest)
	if lockedSignInResponseRecorder.Code != http.StatusLocked {
		t.Fatalf("expected 423 Locked on locked user sign-in, got: %d (%s)", lockedSignInResponseRecorder.Code, lockedSignInResponseRecorder.Body.String())
	}

	// Reset locked_until
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = NULL WHERE id = $1", phoneUserID)

	// 4. In-Place Conversion Conflicts (Email & Phone)
	conflictEmail := "existing_conflict@example.com"
	conflictSignupPayload, _ := json.Marshal(SignUpInput{
		Email:    conflictEmail,
		Password: "Password123!",
	})
	conflictSignupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", bytes.NewReader(conflictSignupPayload))
	conflictSignupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(conflictSignupResponseRecorder, conflictSignupRequest)

	// Create anonymous user
	anonRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/anonymous", nil)
	anonResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignInAnonymous(anonResponseRecorder, anonRequest)
	var anonAuthTokenResponse AuthTokenResponse
	_ = json.NewDecoder(anonResponseRecorder.Body).Decode(&anonAuthTokenResponse)

	// Try to convert with existing email -> 409 Conflict
	conflictEmailConvertPayload, _ := json.Marshal(SignUpInput{
		Email:    conflictEmail,
		Password: "NewPassword123!",
	})
	conflictEmailRequest := core.WithTestAuthContext(httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", bytes.NewReader(conflictEmailConvertPayload)), anonAuthTokenResponse.User.ID, "anon", true)
	conflictEmailRequest.Header.Set("Authorization", "Bearer "+anonAuthTokenResponse.AccessToken)
	conflictEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(conflictEmailResponseRecorder, conflictEmailRequest)
	if conflictEmailResponseRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on conversion with duplicate email, got: %d (%s)", conflictEmailResponseRecorder.Code, conflictEmailResponseRecorder.Body.String())
	}

	// Try to convert with existing phone -> 409 Conflict
	conflictPhoneConvertPayload, _ := json.Marshal(SignUpInput{
		Phone:    phoneUser,
		Password: "NewPassword123!",
	})
	conflictPhoneRequest := core.WithTestAuthContext(httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", bytes.NewReader(conflictPhoneConvertPayload)), anonAuthTokenResponse.User.ID, "anon", true)
	conflictPhoneRequest.Header.Set("Authorization", "Bearer "+anonAuthTokenResponse.AccessToken)
	conflictPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(conflictPhoneResponseRecorder, conflictPhoneRequest)
	if conflictPhoneResponseRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on conversion with duplicate phone, got: %d (%s)", conflictPhoneResponseRecorder.Code, conflictPhoneResponseRecorder.Body.String())
	}

	// 5. Conversion DB Update Error via CHECK constraint
	_, err = db.Exec(ctx, "ALTER TABLE auth.users ADD CONSTRAINT layr_test_block_convert CHECK (email != 'block_convert@example.com')")
	if err != nil {
		t.Fatalf("failed to add test constraint: %v", err)
	}
	defer func() {
		_, _ = db.Exec(context.Background(), "ALTER TABLE auth.users DROP CONSTRAINT IF EXISTS layr_test_block_convert")
	}()

	blockConvertPayload, _ := json.Marshal(SignUpInput{
		Email:    "block_convert@example.com",
		Password: "Password123!",
	})
	blockConvertRequest := core.WithTestAuthContext(httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-up", bytes.NewReader(blockConvertPayload)), anonAuthTokenResponse.User.ID, "anon", true)
	blockConvertRequest.Header.Set("Authorization", "Bearer "+anonAuthTokenResponse.AccessToken)
	blockConvertResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(blockConvertResponseRecorder, blockConvertRequest)
	if blockConvertResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on conversion failure via DB constraint, got: %d (%s)", blockConvertResponseRecorder.Code, blockConvertResponseRecorder.Body.String())
	}
	_, _ = db.Exec(ctx, "ALTER TABLE auth.users DROP CONSTRAINT IF EXISTS layr_test_block_convert")

	// 6. DB Scan Errors on Anonymous Sign-In and Direct Sign-Up via canceled context
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	canceledAnonRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodPost, "/v1/auth/anonymous", nil)
	canceledAnonResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignInAnonymous(canceledAnonResponseRecorder, canceledAnonRequest)
	if canceledAnonResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled anonymous sign-in, got: %d", canceledAnonResponseRecorder.Code)
	}

	canceledSignupPayload, _ := json.Marshal(SignUpInput{
		Email:    "canceled_signup@example.com",
		Password: "Password123!",
	})
	canceledSignupRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodPost, "/v1/auth/sign-up", bytes.NewReader(canceledSignupPayload))
	canceledSignupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignUp(canceledSignupResponseRecorder, canceledSignupRequest)
	if canceledSignupResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled sign-up, got: %d", canceledSignupResponseRecorder.Code)
	}

	// 7. Token Refresh via Database (FastPath Disabled)
	authConfig.Cache.FastPathSessionsEnabled = false
	if saveConfigErr := configManager.Save(ctx, authConfig); saveConfigErr != nil {
		t.Fatalf("failed to update config: %v", saveConfigErr)
	}

	// A) Missing / revoked refresh token in DB -> 401
	revokedRefreshPayload, _ := json.Marshal(RefreshTokenInput{RefreshToken: "revoked-or-missing-token"})
	revokedRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/token/refresh", bytes.NewReader(revokedRefreshPayload))
	revokedRefreshResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRefreshToken(revokedRefreshResponseRecorder, revokedRefreshRequest)
	if revokedRefreshResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing/revoked token refresh in DB, got: %d (%s)", revokedRefreshResponseRecorder.Code, revokedRefreshResponseRecorder.Body.String())
	}

	// B) Expired session in DB -> 401
	var conflictUserID string
	_ = db.QueryRow(ctx, "SELECT id FROM auth.users WHERE email = $1", conflictEmail).Scan(&conflictUserID)

	expiredToken := "expired-token-val"
	expiredHash := kernel.JWTSigner().HashRefreshToken(expiredToken)
	_, err = db.Exec(ctx, `
		INSERT INTO auth.sessions (user_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1, $2, clock_timestamp() - interval '10 minutes', clock_timestamp() - interval '1 hour')
	`, conflictUserID, expiredHash)
	if err != nil {
		t.Fatalf("failed to insert expired session: %v", err)
	}

	expiredRefreshPayload, _ := json.Marshal(RefreshTokenInput{RefreshToken: expiredToken})
	expiredRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/token/refresh", bytes.NewReader(expiredRefreshPayload))
	expiredRefreshResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRefreshToken(expiredRefreshResponseRecorder, expiredRefreshRequest)
	if expiredRefreshResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on expired token refresh in DB, got: %d (%s)", expiredRefreshResponseRecorder.Code, expiredRefreshResponseRecorder.Body.String())
	}

	// C) Session referencing deleted user -> 401
	_, err = db.Exec(ctx, "ALTER TABLE auth.sessions DROP CONSTRAINT IF EXISTS sessions_user_id_fkey")
	if err != nil {
		t.Fatalf("failed to drop foreign key constraint: %v", err)
	}
	defer func() {
		_, _ = db.Exec(context.Background(), `
			ALTER TABLE auth.sessions 
			ADD CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE
		`)
	}()

	deletedUserToken := "deleted-user-refresh-token"
	deletedUserHash := kernel.JWTSigner().HashRefreshToken(deletedUserToken)
	nonExistentUserID := "018f2234-5678-789a-bcde-f0123456789b"
	_, err = db.Exec(ctx, `
		INSERT INTO auth.sessions (user_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1, $2, clock_timestamp() + interval '1 hour', clock_timestamp())
	`, nonExistentUserID, deletedUserHash)
	if err != nil {
		t.Fatalf("failed to insert session for non-existent user: %v", err)
	}

	deletedUserRefreshPayload, _ := json.Marshal(RefreshTokenInput{RefreshToken: deletedUserToken})
	deletedUserRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/token/refresh", bytes.NewReader(deletedUserRefreshPayload))
	deletedUserRefreshResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRefreshToken(deletedUserRefreshResponseRecorder, deletedUserRefreshRequest)
	if deletedUserRefreshResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on non-existent user token refresh in DB, got: %d (%s)", deletedUserRefreshResponseRecorder.Code, deletedUserRefreshResponseRecorder.Body.String())
	}

	_, _ = db.Exec(ctx, "DELETE FROM auth.sessions WHERE refresh_token_hash = $1", deletedUserHash)
	_, _ = db.Exec(ctx, `
		ALTER TABLE auth.sessions 
		ADD CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE
	`)

	// D) Valid DB session refresh -> 200 OK
	validDBSignInPayload, _ := json.Marshal(SignInInput{
		Email:    conflictEmail,
		Password: "Password123!",
	})
	validDBSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/sign-in", bytes.NewReader(validDBSignInPayload))
	validDBSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(validDBSignInResponseRecorder, validDBSignInRequest)
	if validDBSignInRecCode := validDBSignInResponseRecorder.Code; validDBSignInRecCode != http.StatusOK {
		t.Fatalf("expected 200 OK on sign-in before DB refresh, got: %d", validDBSignInRecCode)
	}

	var validDBAuthTokenResponse AuthTokenResponse
	if err := json.NewDecoder(validDBSignInResponseRecorder.Body).Decode(&validDBAuthTokenResponse); err != nil {
		t.Fatalf("failed to decode sign-in response: %v", err)
	}

	validDBRefreshPayload, _ := json.Marshal(RefreshTokenInput{RefreshToken: validDBAuthTokenResponse.RefreshToken})
	validDBRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/token/refresh", bytes.NewReader(validDBRefreshPayload))
	validDBRefreshResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRefreshToken(validDBRefreshResponseRecorder, validDBRefreshRequest)
	if validDBRefreshResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on valid DB token refresh, got: %d (%s)", validDBRefreshResponseRecorder.Code, validDBRefreshResponseRecorder.Body.String())
	}

	var rotatedAuthTokenResponse AuthTokenResponse
	_ = json.NewDecoder(validDBRefreshResponseRecorder.Body).Decode(&rotatedAuthTokenResponse)

	// Locked user DB token refresh -> 423
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = clock_timestamp() + interval '1 hour' WHERE email = $1", conflictEmail)
	lockedDBRefreshPayload, _ := json.Marshal(RefreshTokenInput{RefreshToken: rotatedAuthTokenResponse.RefreshToken})
	lockedDBRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/token/refresh", bytes.NewReader(lockedDBRefreshPayload))
	lockedDBRefreshResponseRecorder := httptest.NewRecorder()
	baseHandler.handleRefreshToken(lockedDBRefreshResponseRecorder, lockedDBRefreshRequest)
	if lockedDBRefreshResponseRecorder.Code != http.StatusLocked {
		t.Fatalf("expected 423 StatusLocked on locked DB token refresh, got: %d (%s)", lockedDBRefreshResponseRecorder.Code, lockedDBRefreshResponseRecorder.Body.String())
	}
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = NULL WHERE email = $1", conflictEmail)
}
