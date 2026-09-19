package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"layr.sh/core"
)

func TestAuthPasskeyCeremoniesIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanupDatabase := setupTestDatabase(t)
	defer cleanupDatabase()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load initial auth config: %v", err)
	}

	activeConfig := configManager.Get()
	activeConfig.Passkeys.Enabled = true
	configManager.Set(activeConfig)

	eventBus := core.NewEventBus(db, cryptoKeyManager)
	defer eventBus.Close()
	testKVStore := newInMemoryKVStore()

	baseHandler := NewBaseHandler(db, configManager, cryptoKeyManager)
	baseHandler.SetEventBus(eventBus)
	baseHandler.SetKVStore(testKVStore)

	emittedEvents := make([]core.Event, 0)
	eventBus.Subscribe("auth.passkey.created", func(eventCtx context.Context, event core.Event) error {
		emittedEvents = append(emittedEvents, event)
		return nil
	})
	eventBus.Subscribe("auth.user.converted", func(eventCtx context.Context, event core.Event) error {
		emittedEvents = append(emittedEvents, event)
		return nil
	})

	// 1. Begin Sign-up Flow
	newUserID := "01918a24-1111-7000-8000-000000000001"
	signUpPayload, _ := json.Marshal(PasskeySignUpRequest{
		UserID:   newUserID,
		UserName: "Alice",
	})
	signUpRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up", bytes.NewReader(signUpPayload))
	signUpResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignUp(signUpResponseRecorder, signUpRequest)
	if signUpResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey sign-up, got: %d (%s)", signUpResponseRecorder.Code, signUpResponseRecorder.Body.String())
	}

	var signUpResponse map[string]any
	if err := json.NewDecoder(signUpResponseRecorder.Body).Decode(&signUpResponse); err != nil {
		t.Fatalf("failed to decode passkey sign-up response: %v", err)
	}
	challenge, ok := signUpResponse["challenge"].(string)
	if !ok || challenge == "" {
		t.Fatalf("expected non-empty challenge in sign-up response: %+v", signUpResponse)
	}

	// 2. Complete Sign-up Flow (New User)
	credentialID := "credential-alice-12345"
	verifySignUpPayload, _ := json.Marshal(PasskeySignUpVerifyRequest{
		UserID:       newUserID,
		Challenge:    challenge,
		CredentialID: credentialID,
		PublicKey:    "public-key-alice-blob",
		FriendlyName: "MacBook TouchID",
		Transports:   []string{"internal"},
	})
	verifySignUpRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up/verify", bytes.NewReader(verifySignUpPayload))
	verifySignUpResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignUpVerify(verifySignUpResponseRecorder, verifySignUpRequest)
	if verifySignUpResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey sign-up verify, got: %d (%s)", verifySignUpResponseRecorder.Code, verifySignUpResponseRecorder.Body.String())
	}

	var signUpSessionResponse SessionResponse
	if err := json.NewDecoder(verifySignUpResponseRecorder.Body).Decode(&signUpSessionResponse); err != nil || signUpSessionResponse.AccessToken == "" {
		t.Fatalf("expected valid SessionResponse, got: %+v (err: %v)", signUpSessionResponse, err)
	}
	if signUpSessionResponse.User.ID != newUserID {
		t.Fatalf("expected user ID %s, got: %s", newUserID, signUpSessionResponse.User.ID)
	}

	// Verify DB record
	var passkeyCount int
	_ = db.QueryRow(ctx, "SELECT count(*) FROM auth.passkeys WHERE user_id = $1", newUserID).Scan(&passkeyCount)
	if passkeyCount != 1 {
		t.Fatalf("expected 1 passkey record in DB, got: %d", passkeyCount)
	}

	// 3. Begin Sign-in Flow
	signInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-in", strings.NewReader(`{}`))
	signInResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignIn(signInResponseRecorder, signInRequest)
	if signInResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey sign-in, got: %d", signInResponseRecorder.Code)
	}

	var signInResponse map[string]any
	_ = json.NewDecoder(signInResponseRecorder.Body).Decode(&signInResponse)
	signInChallenge, _ := signInResponse["challenge"].(string)
	if signInChallenge == "" {
		t.Fatalf("expected challenge in sign-in response")
	}

	// 4. Complete Sign-in Flow
	verifySignInPayload, _ := json.Marshal(PasskeySignInVerifyRequest{
		Challenge:    signInChallenge,
		CredentialID: credentialID,
	})
	verifySignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-in/verify", bytes.NewReader(verifySignInPayload))
	verifySignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignInVerify(verifySignInResponseRecorder, verifySignInRequest)
	if verifySignInResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey sign-in verify, got: %d (%s)", verifySignInResponseRecorder.Code, verifySignInResponseRecorder.Body.String())
	}

	var signInSessionResponse SessionResponse
	if err := json.NewDecoder(verifySignInResponseRecorder.Body).Decode(&signInSessionResponse); err != nil || signInSessionResponse.AccessToken == "" {
		t.Fatalf("expected valid session response on sign-in verify: %+v", signInSessionResponse)
	}

	// 5. Sign-in Verify with Non-existent Credential -> 401
	ghostChallenge, _ := baseHandler.passkeyManager.GenerateChallenge("")
	ghostVerifyPayload, _ := json.Marshal(PasskeySignInVerifyRequest{
		Challenge:    ghostChallenge,
		CredentialID: "unknown-credential-id",
	})
	ghostVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-in/verify", bytes.NewReader(ghostVerifyPayload))
	ghostVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignInVerify(ghostVerifyResponseRecorder, ghostVerifyRequest)
	if ghostVerifyResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unknown credential verify, got: %d", ghostVerifyResponseRecorder.Code)
	}

	// 6. Sign-up Verify for Anonymous User (Converts anonymous caller)
	anonUserID := "01918a24-7777-7000-8000-000000000007"
	_, _ = db.Exec(ctx, `
		INSERT INTO auth.users (id, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, 'authenticated', true, '{"tier":"free"}'::jsonb, clock_timestamp(), clock_timestamp())
	`, anonUserID)
	anonToken, _ := baseHandler.jwtSigner.GenerateAccessToken(core.JWTClaims{
		Subject:     anonUserID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 3600)
	anonAuthContext := core.AuthContext{UserID: anonUserID, JWT: core.JWTClaims{Subject: anonUserID, Role: "authenticated", IsAnonymous: true}}

	anonChallenge, _ := baseHandler.passkeyManager.GenerateChallenge(anonUserID)
	anonVerifyPayload, _ := json.Marshal(PasskeySignUpVerifyRequest{
		UserID:       anonUserID,
		Challenge:    anonChallenge,
		CredentialID: "credential-anon-999",
		PublicKey:    "public-key-anon-blob",
		FriendlyName: "YubiKey 5C",
		Transports:   []string{"usb", "nfc"},
	})
	anonVerifyRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), anonAuthContext), http.MethodPost, "/api/v1/auth/passkeys/sign-up/verify", bytes.NewReader(anonVerifyPayload))
	anonVerifyRequest.Header.Set("Authorization", "Bearer "+anonToken)
	anonVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignUpVerify(anonVerifyResponseRecorder, anonVerifyRequest)
	if anonVerifyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on anonymous passkey verify conversion, got: %d (%s)", anonVerifyResponseRecorder.Code, anonVerifyResponseRecorder.Body.String())
	}

	var isStillAnonymous bool
	_ = db.QueryRow(ctx, "SELECT is_anonymous FROM auth.users WHERE id = $1", anonUserID).Scan(&isStillAnonymous)
	if isStillAnonymous {
		t.Fatalf("expected anonymous user to be converted to non-anonymous after passkey registration")
	}

	// 7. Error branch: Sign-up verify with canceled context -> 500
	canceledChallenge, _ := baseHandler.passkeyManager.GenerateChallenge(newUserID)
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	canceledPayload, _ := json.Marshal(PasskeySignUpVerifyRequest{
		UserID:       newUserID,
		Challenge:    canceledChallenge,
		CredentialID: "cred-canceled",
		PublicKey:    "pub-canceled",
	})
	canceledVerifyRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodPost, "/api/v1/auth/passkeys/sign-up/verify", bytes.NewReader(canceledPayload))
	canceledVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignUpVerify(canceledVerifyResponseRecorder, canceledVerifyRequest)
	if canceledVerifyResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled context passkey sign-up verify, got: %d", canceledVerifyResponseRecorder.Code)
	}

	// 8. Error branch: Sign-in verify where user does not exist in users table -> 500
	ghostUserCredentialID := "cred-ghost-orphan"
	ghostUserID := "01918a24-8888-7000-8000-000000000088"
	var fkConstraintName string
	_ = db.QueryRow(ctx, `
		SELECT constraint_name FROM information_schema.table_constraints
		WHERE table_schema = 'auth' AND table_name = 'passkeys' AND constraint_type = 'FOREIGN KEY'
	`).Scan(&fkConstraintName)
	if fkConstraintName != "" {
		_, _ = db.Exec(ctx, "ALTER TABLE auth.passkeys DROP CONSTRAINT "+fkConstraintName)
	}
	_, insertErr := db.Exec(ctx, `
		INSERT INTO auth.passkeys (id, user_id, credential_id, public_key, counter, created_at, last_used_at)
		VALUES ('01918a24-8888-7000-8000-000000000099', $1, $2, 'mockpubkey', 0, clock_timestamp(), clock_timestamp())
	`, ghostUserID, []byte(ghostUserCredentialID))
	if insertErr != nil {
		t.Fatalf("failed to insert orphaned passkey: %v", insertErr)
	}

	ghostSignInChallenge, _ := baseHandler.passkeyManager.GenerateChallenge("")
	ghostSignInPayload, _ := json.Marshal(PasskeySignInVerifyRequest{
		Challenge:    ghostSignInChallenge,
		CredentialID: ghostUserCredentialID,
	})
	ghostSignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-in/verify", bytes.NewReader(ghostSignInPayload))
	ghostSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignInVerify(ghostSignInResponseRecorder, ghostSignInRequest)
	if ghostSignInResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on orphaned passkey user lookup, got: %d (%s)", ghostSignInResponseRecorder.Code, ghostSignInResponseRecorder.Body.String())
	}

	// 9. Error branch: passkey insert fails -> 500
	_, _ = db.Exec(ctx, `
		CREATE OR REPLACE FUNCTION auth.trg_fail_passkey_insert_fn() RETURNS trigger AS $$
		BEGIN
			IF NEW.friendly_name = 'fail_insert_trigger' THEN
				RAISE EXCEPTION 'simulated passkey insert failure';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		DROP TRIGGER IF EXISTS trg_fail_passkey_insert ON auth.passkeys;
		CREATE TRIGGER trg_fail_passkey_insert BEFORE INSERT ON auth.passkeys
		FOR EACH ROW EXECUTE FUNCTION auth.trg_fail_passkey_insert_fn();
	`)
	failChallenge, _ := baseHandler.passkeyManager.GenerateChallenge(newUserID)
	failVerifyPayload, _ := json.Marshal(PasskeySignUpVerifyRequest{
		UserID:       newUserID,
		Challenge:    failChallenge,
		CredentialID: "cred-fail-insert",
		PublicKey:    "pub-key-fail",
		FriendlyName: "fail_insert_trigger",
	})
	failVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up/verify", bytes.NewReader(failVerifyPayload))
	failVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignUpVerify(failVerifyResponseRecorder, failVerifyRequest)
	if failVerifyResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on passkey insert trigger failure, got: %d", failVerifyResponseRecorder.Code)
	}
	_, _ = db.Exec(ctx, `
		DROP TRIGGER IF EXISTS trg_fail_passkey_insert ON auth.passkeys;
		DROP FUNCTION IF EXISTS auth.trg_fail_passkey_insert_fn();
	`)
}

func TestAuthPasskeyManagementAndHardeningIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanupDatabase := setupTestDatabase(t)
	defer cleanupDatabase()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load initial auth config: %v", err)
	}

	activeConfig := configManager.Get()
	activeConfig.Passkeys.Enabled = true
	configManager.Set(activeConfig)

	eventBus := core.NewEventBus(db, cryptoKeyManager)
	defer eventBus.Close()
	testKVStore := newInMemoryKVStore()

	baseHandler := NewBaseHandler(db, configManager, cryptoKeyManager)
	baseHandler.SetEventBus(eventBus)
	baseHandler.SetKVStore(testKVStore)

	var emittedEvents []core.Event
	var eventsMutex sync.Mutex
	eventBus.Subscribe("auth.passkey.*", func(eventCtx context.Context, event core.Event) error {
		eventsMutex.Lock()
		defer eventsMutex.Unlock()
		emittedEvents = append(emittedEvents, event)
		return nil
	})

	// Create user
	userID := "01918a24-2222-7000-8000-000000000002"
	_, err := db.Exec(ctx, `
		INSERT INTO auth.users (id, role, properties, created_at, last_updated_at)
		VALUES ($1, 'authenticated', '{"tier":"pro"}', clock_timestamp(), clock_timestamp())
	`, userID)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	userToken, err := baseHandler.jwtSigner.GenerateAccessToken(core.JWTClaims{
		Subject: userID,
		Role:    "authenticated",
	}, 3600)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}
	bearerHeader := "Bearer " + userToken
	userAuthContext := core.AuthContext{UserID: userID, JWT: core.JWTClaims{Subject: userID, Role: "authenticated"}}

	// 1. Authenticated caller registers passkey using session token (empty user_id in payload)
	signUpPayload, _ := json.Marshal(PasskeySignUpRequest{
		UserName: "Bob Session",
	})
	signUpRequest := httptest.NewRequestWithContext(core.WithAuthContext(ctx, userAuthContext), http.MethodPost, "/api/v1/auth/passkeys/sign-up", bytes.NewReader(signUpPayload))
	signUpRequest.Header.Set("Authorization", bearerHeader)
	signUpResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignUp(signUpResponseRecorder, signUpRequest)
	if signUpResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on authenticated passkey sign-up, got: %d (%s)", signUpResponseRecorder.Code, signUpResponseRecorder.Body.String())
	}

	var signUpResponse map[string]any
	_ = json.NewDecoder(signUpResponseRecorder.Body).Decode(&signUpResponse)
	signUpChallenge := signUpResponse["challenge"].(string)

	credentialID1 := "cred-bob-session-1"
	verifySignUpPayload, _ := json.Marshal(PasskeySignUpVerifyRequest{
		Challenge:    signUpChallenge,
		CredentialID: credentialID1,
		PublicKey:    "public-key-blob-1",
		FriendlyName: "Work Laptop",
		Transports:   []string{"internal"},
	})
	verifySignUpRequest := httptest.NewRequestWithContext(core.WithAuthContext(ctx, userAuthContext), http.MethodPost, "/api/v1/auth/passkeys/sign-up/verify", bytes.NewReader(verifySignUpPayload))
	verifySignUpRequest.Header.Set("Authorization", bearerHeader)
	verifySignUpResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignUpVerify(verifySignUpResponseRecorder, verifySignUpRequest)
	if verifySignUpResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on authenticated passkey sign-up verify, got: %d (%s)", verifySignUpResponseRecorder.Code, verifySignUpResponseRecorder.Body.String())
	}

	// 2. Register a second passkey with empty public key (to test VerifySignature false branch)
	challenge2, _ := baseHandler.passkeyManager.GenerateChallenge(userID)
	credentialID2 := "cred-bob-session-2"
	verify2Payload, _ := json.Marshal(PasskeySignUpVerifyRequest{
		UserID:       userID,
		Challenge:    challenge2,
		CredentialID: credentialID2,
		PublicKey:    "",
		FriendlyName: "Backup Security Key",
		Transports:   []string{"usb"},
	})
	verify2Request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/passkeys/sign-up/verify", bytes.NewReader(verify2Payload))
	verify2ResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignUpVerify(verify2ResponseRecorder, verify2Request)
	if verify2ResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on second passkey verify, got: %d", verify2ResponseRecorder.Code)
	}

	// 3. List passkeys -> 200 with 2 items
	listRequest := httptest.NewRequestWithContext(core.WithAuthContext(ctx, userAuthContext), http.MethodGet, "/api/v1/auth/user/passkeys", nil)
	listRequest.Header.Set("Authorization", bearerHeader)
	listResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListUserPasskeys(listResponseRecorder, listRequest)
	if listResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on list user passkeys, got: %d (%s)", listResponseRecorder.Code, listResponseRecorder.Body.String())
	}

	var passkeyItems []UserPasskeyResponse
	if decodeErr := json.NewDecoder(listResponseRecorder.Body).Decode(&passkeyItems); decodeErr != nil {
		t.Fatalf("failed to decode list passkeys response: %v", decodeErr)
	}
	if len(passkeyItems) != 2 {
		t.Fatalf("expected 2 passkeys, got: %d", len(passkeyItems))
	}

	// 4. Sign-in verify with assertion signature:
	// 4a. Valid signature over non-empty public key -> 200
	signInChallenge1, _ := baseHandler.passkeyManager.GenerateChallenge("")
	verifySigPayload, _ := json.Marshal(PasskeySignInVerifyRequest{
		Challenge:         signInChallenge1,
		CredentialID:      credentialID1,
		ClientDataJSON:    `{"type":"webauthn.get"}`,
		AuthenticatorData: "authenticator-data",
		Signature:         "test-signature",
	})
	verifySigRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/passkeys/sign-in/verify", bytes.NewReader(verifySigPayload))
	verifySigResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignInVerify(verifySigResponseRecorder, verifySigRequest)
	if verifySigResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey verify with valid signature, got: %d (%s)", verifySigResponseRecorder.Code, verifySigResponseRecorder.Body.String())
	}

	// 4b. Signature over credential with empty public key -> 401
	signInChallenge2, _ := baseHandler.passkeyManager.GenerateChallenge("")
	invalidSigPayload, _ := json.Marshal(PasskeySignInVerifyRequest{
		Challenge:         signInChallenge2,
		CredentialID:      credentialID2,
		ClientDataJSON:    `{"type":"webauthn.get"}`,
		AuthenticatorData: "authenticator-data",
		Signature:         "test-signature",
	})
	invalidSigRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/passkeys/sign-in/verify", bytes.NewReader(invalidSigPayload))
	invalidSigResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignInVerify(invalidSigResponseRecorder, invalidSigRequest)
	if invalidSigResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid passkey assertion signature, got: %d", invalidSigResponseRecorder.Code)
	}

	// 5. Sign-in verify with locked account -> 423
	lockedUntil := time.Now().UTC().Add(time.Hour)
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = $1 WHERE id = $2", lockedUntil, userID)

	lockedSignInChallenge, _ := baseHandler.passkeyManager.GenerateChallenge("")
	lockedPayload, _ := json.Marshal(PasskeySignInVerifyRequest{
		Challenge:    lockedSignInChallenge,
		CredentialID: credentialID1,
	})
	lockedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/passkeys/sign-in/verify", bytes.NewReader(lockedPayload))
	lockedResponseRecorder := httptest.NewRecorder()
	baseHandler.handlePasskeySignInVerify(lockedResponseRecorder, lockedRequest)
	if lockedResponseRecorder.Code != http.StatusLocked {
		t.Fatalf("expected 423 on locked user passkey sign-in, got: %d", lockedResponseRecorder.Code)
	}
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = NULL WHERE id = $1", userID)

	// 6. Delete passkey -> 204 No Content
	passkeyToDeleteID := passkeyItems[0].ID
	deleteRequest := httptest.NewRequestWithContext(core.WithAuthContext(ctx, userAuthContext), http.MethodDelete, "/api/v1/auth/user/passkeys/"+passkeyToDeleteID, nil)
	deleteRequest.SetPathValue("id", passkeyToDeleteID)
	deleteRequest.Header.Set("Authorization", bearerHeader)
	deleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteUserPasskey(deleteResponseRecorder, deleteRequest)
	if deleteResponseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on delete passkey, got: %d (%s)", deleteResponseRecorder.Code, deleteResponseRecorder.Body.String())
	}

	// Verify auth.passkey.deleted event was emitted
	var deletedEventFound bool
	for attempt := 0; attempt < 50; attempt++ {
		eventsMutex.Lock()
		for _, evt := range emittedEvents {
			if evt.Type == "auth.passkey.deleted" {
				deletedEventFound = true
				if evt.Data["id"] != passkeyToDeleteID {
					eventsMutex.Unlock()
					t.Fatalf("expected passkey id %s in event, got: %v", passkeyToDeleteID, evt.Data["id"])
				}
				userMap, ok := evt.Data["user"].(map[string]any)
				if !ok || userMap["id"] != userID {
					eventsMutex.Unlock()
					t.Fatalf("expected nested user in passkey deleted event: %v", evt.Data["user"])
				}
				break
			}
		}
		eventsMutex.Unlock()
		if deletedEventFound {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !deletedEventFound {
		t.Fatal("expected auth.passkey.deleted event was published")
	}

	// 7. Delete non-existent passkey -> 404
	delete404Request := withUserAuth(httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/auth/user/passkeys/"+passkeyToDeleteID, nil), userID, "authenticated", false)
	delete404Request.SetPathValue("id", passkeyToDeleteID)
	delete404Request.Header.Set("Authorization", bearerHeader)
	delete404ResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteUserPasskey(delete404ResponseRecorder, delete404Request)
	if delete404ResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on deleting already deleted passkey, got: %d", delete404ResponseRecorder.Code)
	}

	// 8. Delete passkey with trigger/DB error -> 500
	_, _ = db.Exec(ctx, `
		CREATE OR REPLACE FUNCTION auth.trg_fail_passkey_delete_fn() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'simulated passkey delete failure';
		END;
		$$ LANGUAGE plpgsql;
		DROP TRIGGER IF EXISTS trg_fail_passkey_delete ON auth.passkeys;
		CREATE TRIGGER trg_fail_passkey_delete BEFORE DELETE ON auth.passkeys
		FOR EACH ROW EXECUTE FUNCTION auth.trg_fail_passkey_delete_fn();
	`)

	passkeyToFailDeleteID := passkeyItems[1].ID
	failDeleteRequest := withUserAuth(httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/auth/user/passkeys/"+passkeyToFailDeleteID, nil), userID, "authenticated", false)
	failDeleteRequest.SetPathValue("id", passkeyToFailDeleteID)
	failDeleteRequest.Header.Set("Authorization", bearerHeader)
	failDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeleteUserPasskey(failDeleteResponseRecorder, failDeleteRequest)
	if failDeleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on passkey delete failure trigger, got: %d", failDeleteResponseRecorder.Code)
	}

	_, _ = db.Exec(ctx, `
		DROP TRIGGER IF EXISTS trg_fail_passkey_delete ON auth.passkeys;
		DROP FUNCTION IF EXISTS auth.trg_fail_passkey_delete_fn();
	`)

	// 9. List passkeys with canceled context -> 500
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	canceledListRequest := withUserAuth(httptest.NewRequestWithContext(canceledCtx, http.MethodGet, "/api/v1/auth/user/passkeys", nil), userID, "authenticated", false)
	canceledListRequest.Header.Set("Authorization", bearerHeader)
	canceledListResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListUserPasskeys(canceledListResponseRecorder, canceledListRequest)
	if canceledListResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled context list passkeys, got: %d", canceledListResponseRecorder.Code)
	}
}
