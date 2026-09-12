package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"layr.sh/auth/jwt"
	"layr.sh/core"
)

func TestAuthMFAFlowIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	authConfig := configManager.Get()
	authConfig.MFA.Enabled = true
	authConfig.MFA.Issuer = "LayrAuthIntegration"
	if err := configManager.Save(ctx, authConfig); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	eventBus := core.NewEventBus(db, cryptoKeyManager)
	var capturedEvents []core.Event
	var eventsMutex sync.Mutex
	eventBus.Subscribe("auth.*", func(eventCtx context.Context, event core.Event) error {
		eventsMutex.Lock()
		defer eventsMutex.Unlock()
		capturedEvents = append(capturedEvents, event)
		return nil
	})

	baseHandler := NewHandler(db, configManager, cryptoKeyManager)
	baseHandler.SetEventBus(eventBus)

	// Create test user
	userEmail := "mfa.user@example.com"
	var userID string
	err := db.QueryRow(ctx, `
		INSERT INTO auth.users (email, role, properties, created_at, last_updated_at)
		VALUES ($1, 'authenticated', '{}', clock_timestamp(), clock_timestamp())
		RETURNING id
	`, userEmail).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	validAccessToken, err := baseHandler.signer.GenerateAccessToken(jwt.Claims{
		Subject: userID,
		Email:   userEmail,
		Role:    "authenticated",
	}, 900)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	// 1. Setup MFA
	mfaSetupPayload := map[string]any{
		"user_id": userID,
	}
	encodedMFASetup, _ := json.Marshal(mfaSetupPayload)
	mfaSetupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/setup", bytes.NewReader(encodedMFASetup))
	mfaSetupRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	mfaSetupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFASetup(mfaSetupResponseRecorder, mfaSetupRequest)

	if mfaSetupResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on MFA setup, got: %d (%s)", mfaSetupResponseRecorder.Code, mfaSetupResponseRecorder.Body.String())
	}

	var mfaSetupResponse MFASetupResponse
	if decodeErr := json.NewDecoder(mfaSetupResponseRecorder.Body).Decode(&mfaSetupResponse); decodeErr != nil {
		t.Fatalf("failed to decode MFA setup response: %v", decodeErr)
	}
	if mfaSetupResponse.Secret == "" || mfaSetupResponse.AuthURL == "" {
		t.Fatalf("expected non-empty secret and auth URL, got: %+v", mfaSetupResponse)
	}
	if mfaSetupResponse.Issuer != "LayrAuthIntegration" {
		t.Fatalf("expected issuer LayrAuthIntegration, got: %s", mfaSetupResponse.Issuer)
	}

	// Verify properties in DB
	var rawProperties []byte
	err = db.QueryRow(ctx, "SELECT properties FROM auth.users WHERE id = $1", userID).Scan(&rawProperties)
	if err != nil {
		t.Fatalf("failed to query user properties: %v", err)
	}
	var properties map[string]any
	_ = json.Unmarshal(rawProperties, &properties)
	if properties["mfa_pending"] != true || properties["mfa_secret_enc"] == nil {
		t.Fatalf("expected mfa_pending=true and mfa_secret_enc present, got: %v", properties)
	}

	// 2. Verify with wrong code -> 401
	wrongVerifyPayload := map[string]any{
		"user_id": userID,
		"code":    "000000",
	}
	encodedWrongVerify, _ := json.Marshal(wrongVerifyPayload)
	wrongVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(encodedWrongVerify))
	wrongVerifyRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	wrongVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(wrongVerifyResponseRecorder, wrongVerifyRequest)
	if wrongVerifyResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on wrong MFA code, got: %d", wrongVerifyResponseRecorder.Code)
	}

	// 3. Verify with valid TOTP code -> 200 OK
	currentTOTPCode, generateCodeErr := baseHandler.GetTOTPManager().GenerateCode(mfaSetupResponse.Secret, time.Now())
	if generateCodeErr != nil {
		t.Fatalf("failed to generate TOTP code: %v", generateCodeErr)
	}

	validVerifyPayload := map[string]any{
		"code": currentTOTPCode,
	}
	encodedValidVerify, _ := json.Marshal(validVerifyPayload)
	validVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(encodedValidVerify))
	validVerifyRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	validVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(validVerifyResponseRecorder, validVerifyRequest)
	if validVerifyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on valid MFA verify, got: %d (%s)", validVerifyResponseRecorder.Code, validVerifyResponseRecorder.Body.String())
	}

	// Verify user properties updated in DB
	properties = make(map[string]any)
	err = db.QueryRow(ctx, "SELECT properties FROM auth.users WHERE id = $1", userID).Scan(&rawProperties)
	if err != nil {
		t.Fatalf("failed to query updated user properties: %v", err)
	}
	_ = json.Unmarshal(rawProperties, &properties)
	if properties["mfa_enabled"] != true {
		t.Fatal("expected mfa_enabled to be true in user properties")
	}
	if _, exists := properties["mfa_pending"]; exists {
		t.Fatal("expected mfa_pending to be removed from user properties")
	}

	// Verify events
	eventsMutex.Lock()
	var otpVerifiedFound, userUpdatedFound bool
	for _, event := range capturedEvents {
		if event.Type == "auth.otp.verified" {
			otpVerifiedFound = true
		}
		if event.Type == "auth.user.updated" {
			userUpdatedFound = true
		}
	}
	eventsMutex.Unlock()
	if !otpVerifiedFound || !userUpdatedFound {
		t.Fatalf("expected auth.otp.verified and auth.user.updated events, got otpVerified=%t userUpdated=%t", otpVerifiedFound, userUpdatedFound)
	}

	// 4. Test locked user -> 423
	lockedUntil := time.Now().UTC().Add(time.Hour)
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = $1 WHERE id = $2", lockedUntil, userID)
	lockedVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(encodedValidVerify))
	lockedVerifyRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	lockedVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(lockedVerifyResponseRecorder, lockedVerifyRequest)
	if lockedVerifyResponseRecorder.Code != http.StatusLocked {
		t.Fatalf("expected 423 StatusLocked on locked user verify, got: %d", lockedVerifyResponseRecorder.Code)
	}
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = NULL WHERE id = $1", userID)

	// 5. Test corrupted encrypted secret -> 500
	_, _ = db.Exec(ctx, `UPDATE auth.users SET properties = '{"mfa_secret_enc":"invalid-secret"}'::jsonb WHERE id = $1`, userID)
	corruptedVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(encodedValidVerify))
	corruptedVerifyRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	corruptedVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(corruptedVerifyResponseRecorder, corruptedVerifyRequest)
	if corruptedVerifyResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on corrupted secret decrypt, got: %d", corruptedVerifyResponseRecorder.Code)
	}

	// 6. Test missing MFA secret in properties -> 400
	_, _ = db.Exec(ctx, "UPDATE auth.users SET properties = '{}'::jsonb WHERE id = $1", userID)
	noSecretVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(encodedValidVerify))
	noSecretVerifyRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	noSecretVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(noSecretVerifyResponseRecorder, noSecretVerifyRequest)
	if noSecretVerifyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on MFA not set up, got: %d", noSecretVerifyResponseRecorder.Code)
	}

	// 7. Non-existent user -> 404
	nonExistentVerifyPayload := map[string]any{
		"user_id": "01918a24-9999-7000-8000-000000000099",
		"code":    "123456",
	}
	encodedNonExistent, _ := json.Marshal(nonExistentVerifyPayload)
	nonExistentVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(encodedNonExistent))
	nonExistentVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(nonExistentVerifyResponseRecorder, nonExistentVerifyRequest)
	if nonExistentVerifyResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on non-existent user verify, got: %d", nonExistentVerifyResponseRecorder.Code)
	}

	nonExistentSetupPayload := map[string]any{
		"user_id": "01918a24-9999-7000-8000-000000000099",
	}
	encodedNonExistentSetup, _ := json.Marshal(nonExistentSetupPayload)
	nonExistentSetupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/setup", bytes.NewReader(encodedNonExistentSetup))
	nonExistentSetupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFASetup(nonExistentSetupResponseRecorder, nonExistentSetupRequest)
	if nonExistentSetupResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on non-existent user setup, got: %d", nonExistentSetupResponseRecorder.Code)
	}

	// 8. Phone-only user MFA setup & verify with empty issuer fallback to "Layr"
	phoneUserMFAConfig := configManager.Get()
	phoneUserMFAConfig.MFA.Issuer = ""
	_ = configManager.Save(ctx, phoneUserMFAConfig)

	var phoneUserID string
	err = db.QueryRow(ctx, `
		INSERT INTO auth.users (phone, role, is_anonymous, created_at, last_updated_at)
		VALUES ('+15554321098', 'authenticated', false, clock_timestamp(), clock_timestamp())
		RETURNING id
	`).Scan(&phoneUserID)
	if err != nil {
		t.Fatalf("failed to insert phone-only test user: %v", err)
	}

	phoneSetupPayload := map[string]any{
		"user_id": phoneUserID,
	}
	encodedPhoneSetup, _ := json.Marshal(phoneSetupPayload)
	phoneSetupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/setup", bytes.NewReader(encodedPhoneSetup))
	phoneSetupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFASetup(phoneSetupResponseRecorder, phoneSetupRequest)
	if phoneSetupResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on phone-only user MFA setup, got: %d", phoneSetupResponseRecorder.Code)
	}
	var phoneMFASetupResponse MFASetupResponse
	_ = json.NewDecoder(phoneSetupResponseRecorder.Body).Decode(&phoneMFASetupResponse)
	if phoneMFASetupResponse.Issuer != "Layr" {
		t.Fatalf("expected default issuer Layr, got: %s", phoneMFASetupResponse.Issuer)
	}

	phoneTOTPCode, _ := baseHandler.GetTOTPManager().GenerateCode(phoneMFASetupResponse.Secret, time.Now())
	phoneVerifyPayload := map[string]any{
		"user_id": phoneUserID,
		"code":    phoneTOTPCode,
	}
	encodedPhoneVerify, _ := json.Marshal(phoneVerifyPayload)
	phoneVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(encodedPhoneVerify))
	phoneVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(phoneVerifyResponseRecorder, phoneVerifyRequest)
	if phoneVerifyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on phone-only user MFA verify, got: %d", phoneVerifyResponseRecorder.Code)
	}
}
