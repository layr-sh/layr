package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/auth/jwt"
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

	handler := NewHandler(db, configManager, cryptoKeyManager)
	handler.SetEventBus(eventBus)
	handler.SetKVStore(testKVStore)

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
	handler.handlePasskeySignUp(signUpResponseRecorder, signUpRequest)
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
	handler.handlePasskeySignUpVerify(verifySignUpResponseRecorder, verifySignUpRequest)
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
	handler.handlePasskeySignIn(signInResponseRecorder, signInRequest)
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
	handler.handlePasskeySignInVerify(verifySignInResponseRecorder, verifySignInRequest)
	if verifySignInResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey sign-in verify, got: %d (%s)", verifySignInResponseRecorder.Code, verifySignInResponseRecorder.Body.String())
	}

	var signInSessionResponse SessionResponse
	if err := json.NewDecoder(verifySignInResponseRecorder.Body).Decode(&signInSessionResponse); err != nil || signInSessionResponse.AccessToken == "" {
		t.Fatalf("expected valid session response on sign-in verify: %+v", signInSessionResponse)
	}

	// 5. Sign-in Verify with Non-existent Credential -> 401
	ghostChallenge, _ := handler.passkeyManager.GenerateChallenge("")
	ghostVerifyPayload, _ := json.Marshal(PasskeySignInVerifyRequest{
		Challenge:    ghostChallenge,
		CredentialID: "unknown-credential-id",
	})
	ghostVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-in/verify", bytes.NewReader(ghostVerifyPayload))
	ghostVerifyResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignInVerify(ghostVerifyResponseRecorder, ghostVerifyRequest)
	if ghostVerifyResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unknown credential verify, got: %d", ghostVerifyResponseRecorder.Code)
	}

	// 6. Sign-up Verify for Anonymous User (Converts anonymous caller)
	anonUserID := "01918a24-7777-7000-8000-000000000007"
	_, _ = db.Exec(ctx, `
		INSERT INTO auth.users (id, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, 'authenticated', true, '{"tier":"free"}'::jsonb, clock_timestamp(), clock_timestamp())
	`, anonUserID)
	anonToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     anonUserID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 3600)

	anonChallenge, _ := handler.passkeyManager.GenerateChallenge(anonUserID)
	anonVerifyPayload, _ := json.Marshal(PasskeySignUpVerifyRequest{
		UserID:       anonUserID,
		Challenge:    anonChallenge,
		CredentialID: "credential-anon-999",
		PublicKey:    "public-key-anon-blob",
		FriendlyName: "YubiKey 5C",
		Transports:   []string{"usb", "nfc"},
	})
	anonVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up/verify", bytes.NewReader(anonVerifyPayload))
	anonVerifyRequest.Header.Set("Authorization", "Bearer "+anonToken)
	anonVerifyResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignUpVerify(anonVerifyResponseRecorder, anonVerifyRequest)
	if anonVerifyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on anonymous passkey verify conversion, got: %d (%s)", anonVerifyResponseRecorder.Code, anonVerifyResponseRecorder.Body.String())
	}

	var isStillAnonymous bool
	_ = db.QueryRow(ctx, "SELECT is_anonymous FROM auth.users WHERE id = $1", anonUserID).Scan(&isStillAnonymous)
	if isStillAnonymous {
		t.Fatalf("expected anonymous user to be converted to non-anonymous after passkey registration")
	}

	// 7. Error branch: Sign-up verify with canceled context -> 500
	canceledChallenge, _ := handler.passkeyManager.GenerateChallenge(newUserID)
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
	handler.handlePasskeySignUpVerify(canceledVerifyResponseRecorder, canceledVerifyRequest)
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

	ghostSignInChallenge, _ := handler.passkeyManager.GenerateChallenge("")
	ghostSignInPayload, _ := json.Marshal(PasskeySignInVerifyRequest{
		Challenge:    ghostSignInChallenge,
		CredentialID: ghostUserCredentialID,
	})
	ghostSignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-in/verify", bytes.NewReader(ghostSignInPayload))
	ghostSignInResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignInVerify(ghostSignInResponseRecorder, ghostSignInRequest)
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
	failChallenge, _ := handler.passkeyManager.GenerateChallenge(newUserID)
	failVerifyPayload, _ := json.Marshal(PasskeySignUpVerifyRequest{
		UserID:       newUserID,
		Challenge:    failChallenge,
		CredentialID: "cred-fail-insert",
		PublicKey:    "pub-key-fail",
		FriendlyName: "fail_insert_trigger",
	})
	failVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up/verify", bytes.NewReader(failVerifyPayload))
	failVerifyResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignUpVerify(failVerifyResponseRecorder, failVerifyRequest)
	if failVerifyResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on passkey insert trigger failure, got: %d", failVerifyResponseRecorder.Code)
	}
	_, _ = db.Exec(ctx, `
		DROP TRIGGER IF EXISTS trg_fail_passkey_insert ON auth.passkeys;
		DROP FUNCTION IF EXISTS auth.trg_fail_passkey_insert_fn();
	`)
}
