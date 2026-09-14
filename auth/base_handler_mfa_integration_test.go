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

	// Verify MFA columns in DB
	var dbEncryptedMFASecret *string
	var dbMFAEnabled bool
	err = db.QueryRow(ctx, "SELECT encrypted_mfa_secret, mfa_enabled FROM auth.users WHERE id = $1", userID).Scan(&dbEncryptedMFASecret, &dbMFAEnabled)
	if err != nil {
		t.Fatalf("failed to query user mfa state: %v", err)
	}
	if dbMFAEnabled || dbEncryptedMFASecret == nil || *dbEncryptedMFASecret == "" {
		t.Fatalf("expected mfa_enabled=false and encrypted_mfa_secret present, got: enabled=%v, secret=%v", dbMFAEnabled, dbEncryptedMFASecret)
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

	// Verify user mfa_enabled updated in DB
	err = db.QueryRow(ctx, "SELECT mfa_enabled FROM auth.users WHERE id = $1", userID).Scan(&dbMFAEnabled)
	if err != nil {
		t.Fatalf("failed to query updated user mfa status: %v", err)
	}
	if !dbMFAEnabled {
		t.Fatal("expected mfa_enabled to be true in database")
	}

	// Verify events
	eventsMutex.Lock()
	var mfaEnabledFound, userUpdatedFound bool
	for _, event := range capturedEvents {
		if event.Type == "auth.mfa.enabled" {
			mfaEnabledFound = true
		}
		if event.Type == "auth.user.updated" {
			userUpdatedFound = true
		}
	}
	eventsMutex.Unlock()
	if !mfaEnabledFound || !userUpdatedFound {
		t.Fatalf("expected auth.mfa.enabled and auth.user.updated events, got mfaEnabled=%t userUpdated=%t", mfaEnabledFound, userUpdatedFound)
	}

	// 4. Test locked user -> 423
	lockedUntil := time.Now().UTC().Add(time.Hour)
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = $1 WHERE id = $2", lockedUntil, userID)
	lockedSetupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/setup", nil)
	lockedSetupRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	lockedSetupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFASetup(lockedSetupResponseRecorder, lockedSetupRequest)
	if lockedSetupResponseRecorder.Code != http.StatusLocked {
		t.Fatalf("expected 423 StatusLocked on locked user setup, got: %d", lockedSetupResponseRecorder.Code)
	}

	lockedVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(encodedValidVerify))
	lockedVerifyRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	lockedVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(lockedVerifyResponseRecorder, lockedVerifyRequest)
	if lockedVerifyResponseRecorder.Code != http.StatusLocked {
		t.Fatalf("expected 423 StatusLocked on locked user verify, got: %d", lockedVerifyResponseRecorder.Code)
	}
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = NULL WHERE id = $1", userID)

	// 5. Test corrupted encrypted secret -> 500
	_, _ = db.Exec(ctx, "UPDATE auth.users SET encrypted_mfa_secret = 'invalid-secret' WHERE id = $1", userID)
	corruptedVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(encodedValidVerify))
	corruptedVerifyRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	corruptedVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(corruptedVerifyResponseRecorder, corruptedVerifyRequest)
	if corruptedVerifyResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on corrupted secret decrypt, got: %d", corruptedVerifyResponseRecorder.Code)
	}

	// 6. Test missing MFA secret in DB -> 400
	_, _ = db.Exec(ctx, "UPDATE auth.users SET encrypted_mfa_secret = NULL, mfa_enabled = false WHERE id = $1", userID)
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
	expectedIssuer := core.GetConfig().Project.Name
	if expectedIssuer == "" {
		expectedIssuer = "Layr Auth"
	}
	if phoneMFASetupResponse.Issuer != expectedIssuer {
		t.Fatalf("expected default issuer %s, got: %s", expectedIssuer, phoneMFASetupResponse.Issuer)
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

	// 9. Test handleMFADisable
	phoneAccessToken, err := baseHandler.signer.GenerateAccessToken(jwt.Claims{
		Subject: phoneUserID,
		Phone:   "+15554321098",
		Role:    "authenticated",
	}, 3600)
	if err != nil {
		t.Fatalf("failed to generate access token for phone user: %v", err)
	}

	// Non-existent user -> 404
	nonExistentToken, _ := baseHandler.signer.GenerateAccessToken(jwt.Claims{
		Subject: "01918a24-9999-7000-8000-000000000099",
		Role:    "authenticated",
	}, 3600)
	nonExistentDisableRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/auth/mfa", nil)
	nonExistentDisableRequest.Header.Set("Authorization", "Bearer "+nonExistentToken)
	nonExistentDisableResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFADisable(nonExistentDisableResponseRecorder, nonExistentDisableRequest)
	if nonExistentDisableResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on non-existent user MFA disable, got: %d", nonExistentDisableResponseRecorder.Code)
	}

	// Locked user -> 423
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = clock_timestamp() + interval '1 hour' WHERE id = $1", phoneUserID)
	lockedDisableRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/auth/mfa", nil)
	lockedDisableRequest.Header.Set("Authorization", "Bearer "+phoneAccessToken)
	lockedDisableResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFADisable(lockedDisableResponseRecorder, lockedDisableRequest)
	if lockedDisableResponseRecorder.Code != http.StatusLocked {
		t.Fatalf("expected 423 StatusLocked on locked user MFA disable, got: %d", lockedDisableResponseRecorder.Code)
	}
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = NULL WHERE id = $1", phoneUserID)

	// Successful MFA disable -> 204
	validDisableRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/auth/mfa", nil)
	validDisableRequest.Header.Set("Authorization", "Bearer "+phoneAccessToken)
	validDisableResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFADisable(validDisableResponseRecorder, validDisableRequest)
	if validDisableResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 StatusNoContent on MFA disable, got: %d", validDisableResponseRecorder.Code)
	}

	// Assert DB mfa_enabled = false, encrypted_mfa_secret IS NULL
	var phoneDBMFAEnabled bool
	var phoneDBEncryptedSecret *string
	_ = db.QueryRow(ctx, "SELECT mfa_enabled, encrypted_mfa_secret FROM auth.users WHERE id = $1", phoneUserID).Scan(&phoneDBMFAEnabled, &phoneDBEncryptedSecret)
	if phoneDBMFAEnabled || phoneDBEncryptedSecret != nil {
		t.Fatalf("expected MFA to be disabled in DB, got enabled=%t, secret=%v", phoneDBMFAEnabled, phoneDBEncryptedSecret)
	}

	// Verify events
	eventsMutex.Lock()
	var mfaDisabledFound bool
	for _, event := range capturedEvents {
		if event.Type == "auth.mfa.disabled" {
			mfaDisabledFound = true
		}
	}
	eventsMutex.Unlock()
	if !mfaDisabledFound {
		t.Fatal("expected auth.mfa.disabled event to be published")
	}
}

func TestAuthMFAChallengeFlowIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	authConfig := configManager.Get()
	authConfig.MFA.Enabled = true
	authConfig.Password.Enabled = true
	if err := configManager.Save(ctx, authConfig); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	testKVStore := newInMemoryKVStore()
	eventBus := core.NewEventBus(db, cryptoKeyManager)
	defer eventBus.Close()

	baseHandler := NewHandler(db, configManager, cryptoKeyManager)
	baseHandler.SetEventBus(eventBus)
	baseHandler.SetKVStore(testKVStore)

	// Create user with password
	userEmail := "mfa.challenge.user@example.com"
	rawPassword := "SecurePassword123!"
	passwordHash, hashErr := baseHandler.hasher.Hash(rawPassword)
	if hashErr != nil {
		t.Fatalf("failed to hash password: %v", hashErr)
	}

	var userID string
	err := db.QueryRow(ctx, `
		INSERT INTO auth.users (email, password_hash, role, properties, created_at, last_updated_at)
		VALUES ($1, $2, 'authenticated', '{}', clock_timestamp(), clock_timestamp())
		RETURNING id
	`, userEmail, passwordHash).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	// 1. Password sign-in BEFORE MFA enabled -> issues session directly
	signInPayload, _ := json.Marshal(SignInRequest{
		Email:    userEmail,
		Password: rawPassword,
	})
	preMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(signInPayload))
	preMFAResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(preMFAResponseRecorder, preMFARequest)
	if preMFAResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on pre-MFA sign in, got: %d (%s)", preMFAResponseRecorder.Code, preMFAResponseRecorder.Body.String())
	}
	var preMFASessionResponse SessionResponse
	if decodeErr := json.NewDecoder(preMFAResponseRecorder.Body).Decode(&preMFASessionResponse); decodeErr != nil || preMFASessionResponse.AccessToken == "" {
		t.Fatalf("expected valid SessionResponse before MFA enabled, got: %+v", preMFASessionResponse)
	}

	// 2. Setup and enable MFA on user
	validAccessToken, _ := baseHandler.signer.GenerateAccessToken(jwt.Claims{
		Subject: userID,
		Email:   userEmail,
		Role:    "authenticated",
	}, 900)

	setupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/setup", bytes.NewReader([]byte(`{}`)))
	setupRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	setupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFASetup(setupResponseRecorder, setupRequest)
	if setupResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on MFA setup, got: %d", setupResponseRecorder.Code)
	}

	var mfaSetupResponse MFASetupResponse
	_ = json.NewDecoder(setupResponseRecorder.Body).Decode(&mfaSetupResponse)

	totpCode, _ := baseHandler.GetTOTPManager().GenerateCode(mfaSetupResponse.Secret, time.Now())
	verifyPayload, _ := json.Marshal(map[string]any{"code": totpCode})
	verifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(verifyPayload))
	verifyRequest.Header.Set("Authorization", "Bearer "+validAccessToken)
	verifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAVerify(verifyResponseRecorder, verifyRequest)
	if verifyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on MFA verify, got: %d", verifyResponseRecorder.Code)
	}

	// 3. Password sign-in AFTER MFA enabled -> intercepted, issues MFA ticket
	postMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(signInPayload))
	postMFAResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(postMFAResponseRecorder, postMFARequest)
	if postMFAResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on MFA-intercepted sign in, got: %d (%s)", postMFAResponseRecorder.Code, postMFAResponseRecorder.Body.String())
	}

	var signInResponse SignInResponse
	if decodeErr := json.NewDecoder(postMFAResponseRecorder.Body).Decode(&signInResponse); decodeErr != nil {
		t.Fatalf("failed to decode SignInResponse: %v", decodeErr)
	}
	if !signInResponse.MFARequired || signInResponse.MFATicket == "" || signInResponse.Factor != "totp" {
		t.Fatalf("expected MFA required with ticket, got: %+v", signInResponse)
	}

	// Sign-in with nil KV store when MFA enabled -> 200
	baseHandler.SetKVStore(nil)
	nilKVRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(signInPayload))
	nilKVResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(nilKVResponseRecorder, nilKVRequest)
	if nilKVResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on sign-in with nil kvStore, got: %d", nilKVResponseRecorder.Code)
	}
	baseHandler.SetKVStore(testKVStore)

	// 4. MFA Challenge: Invalid/missing fields -> 400
	emptyChallengePayload, _ := json.Marshal(MFAChallengeRequest{})
	emptyChallengeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/challenge", bytes.NewReader(emptyChallengePayload))
	emptyChallengeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(emptyChallengeResponseRecorder, emptyChallengeRequest)
	if emptyChallengeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty MFA challenge, got: %d", emptyChallengeResponseRecorder.Code)
	}

	// 5. MFA Challenge: Non-existent ticket -> 401
	ghostChallengePayload, _ := json.Marshal(MFAChallengeRequest{
		MFATicket: "mfa_tk_non_existent",
		Code:      "123456",
	})
	ghostChallengeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/challenge", bytes.NewReader(ghostChallengePayload))
	ghostChallengeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(ghostChallengeResponseRecorder, ghostChallengeRequest)
	if ghostChallengeResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on non-existent MFA ticket, got: %d", ghostChallengeResponseRecorder.Code)
	}

	// 6. MFA Challenge: Wrong TOTP code -> 401
	wrongCodePayload, _ := json.Marshal(MFAChallengeRequest{
		MFATicket: signInResponse.MFATicket,
		Code:      "000000",
	})
	wrongCodeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/challenge", bytes.NewReader(wrongCodePayload))
	wrongCodeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(wrongCodeResponseRecorder, wrongCodeRequest)
	if wrongCodeResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on wrong TOTP code in MFA challenge, got: %d", wrongCodeResponseRecorder.Code)
	}

	// 7. Replay attack: The previous ticket was consumed on attempt -> 401
	replayRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/challenge", bytes.NewReader(wrongCodePayload))
	replayResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(replayResponseRecorder, replayRequest)
	if replayResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on replayed MFA ticket, got: %d", replayResponseRecorder.Code)
	}

	// 8. Generate fresh ticket via sign-in and succeed with valid TOTP code
	freshSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader(signInPayload))
	freshSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignIn(freshSignInResponseRecorder, freshSignInRequest)
	var freshSignInResponse SignInResponse
	_ = json.NewDecoder(freshSignInResponseRecorder.Body).Decode(&freshSignInResponse)

	currentCode, _ := baseHandler.GetTOTPManager().GenerateCode(mfaSetupResponse.Secret, time.Now())
	validChallengePayload, _ := json.Marshal(MFAChallengeRequest{
		MFATicket: freshSignInResponse.MFATicket,
		Code:      currentCode,
	})
	validChallengeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/challenge", bytes.NewReader(validChallengePayload))
	validChallengeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(validChallengeResponseRecorder, validChallengeRequest)
	if validChallengeResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on valid MFA challenge, got: %d (%s)", validChallengeResponseRecorder.Code, validChallengeResponseRecorder.Body.String())
	}

	var mfaSessionResponse SessionResponse
	if decodeErr := json.NewDecoder(validChallengeResponseRecorder.Body).Decode(&mfaSessionResponse); decodeErr != nil || mfaSessionResponse.AccessToken == "" {
		t.Fatalf("expected valid SessionResponse from MFA challenge: %+v", mfaSessionResponse)
	}
	if mfaSessionResponse.User.ID != userID {
		t.Fatalf("expected user ID %s, got: %s", userID, mfaSessionResponse.User.ID)
	}

	// 9. Locked account branch: locked user -> 423
	lockedTicket := "mfa_tk_locked"
	_ = testKVStore.Set(ctx, "auth:mfa_ticket:"+lockedTicket, userID, 5*time.Minute)
	lockedUntil := time.Now().UTC().Add(time.Hour)
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = $1 WHERE id = $2", lockedUntil, userID)

	lockedChallengePayload, _ := json.Marshal(MFAChallengeRequest{
		MFATicket: lockedTicket,
		Code:      currentCode,
	})
	lockedChallengeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/challenge", bytes.NewReader(lockedChallengePayload))
	lockedChallengeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(lockedChallengeResponseRecorder, lockedChallengeRequest)
	if lockedChallengeResponseRecorder.Code != http.StatusLocked {
		t.Fatalf("expected 423 on locked user in MFA challenge, got: %d", lockedChallengeResponseRecorder.Code)
	}
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = NULL WHERE id = $1", userID)

	// 10. Missing MFA secret branch -> 400
	noSecretTicket := "mfa_tk_no_secret"
	_ = testKVStore.Set(ctx, "auth:mfa_ticket:"+noSecretTicket, userID, 5*time.Minute)
	var backupEncryptedSecret string
	_ = db.QueryRow(ctx, "SELECT encrypted_mfa_secret FROM auth.users WHERE id = $1", userID).Scan(&backupEncryptedSecret)
	_, _ = db.Exec(ctx, "UPDATE auth.users SET encrypted_mfa_secret = NULL WHERE id = $1", userID)

	noSecretPayload, _ := json.Marshal(MFAChallengeRequest{
		MFATicket: noSecretTicket,
		Code:      currentCode,
	})
	noSecretRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/challenge", bytes.NewReader(noSecretPayload))
	noSecretResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(noSecretResponseRecorder, noSecretRequest)
	if noSecretResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing secret in MFA challenge, got: %d", noSecretResponseRecorder.Code)
	}

	// 11. Corrupted MFA secret branch -> 500
	corruptSecretTicket := "mfa_tk_corrupt_secret"
	_ = testKVStore.Set(ctx, "auth:mfa_ticket:"+corruptSecretTicket, userID, 5*time.Minute)
	_, _ = db.Exec(ctx, "UPDATE auth.users SET encrypted_mfa_secret = 'corrupt-secret' WHERE id = $1", userID)

	corruptSecretPayload, _ := json.Marshal(MFAChallengeRequest{
		MFATicket: corruptSecretTicket,
		Code:      currentCode,
	})
	corruptSecretRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/challenge", bytes.NewReader(corruptSecretPayload))
	corruptSecretResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(corruptSecretResponseRecorder, corruptSecretRequest)
	if corruptSecretResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on corrupted secret in MFA challenge, got: %d", corruptSecretResponseRecorder.Code)
	}
	_, _ = db.Exec(ctx, "UPDATE auth.users SET encrypted_mfa_secret = $1 WHERE id = $2", backupEncryptedSecret, userID)

	// 12. Orphaned ticket for deleted user -> 401
	ghostUserTicket := "mfa_tk_ghost_user"
	ghostUserID := "01918a24-9999-7000-8000-000000000009"
	_ = testKVStore.Set(ctx, "auth:mfa_ticket:"+ghostUserTicket, ghostUserID, 5*time.Minute)

	ghostUserPayload, _ := json.Marshal(MFAChallengeRequest{
		MFATicket: ghostUserTicket,
		Code:      currentCode,
	})
	ghostUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/mfa/challenge", bytes.NewReader(ghostUserPayload))
	ghostUserResponseRecorder := httptest.NewRecorder()
	baseHandler.handleMFAChallenge(ghostUserResponseRecorder, ghostUserRequest)
	if ghostUserResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on non-existent user for MFA ticket, got: %d", ghostUserResponseRecorder.Code)
	}
}
