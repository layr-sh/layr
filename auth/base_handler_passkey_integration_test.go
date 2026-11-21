package auth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func passkeyTestKey(t *testing.T) (ed25519.PrivateKey, string) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	return privateKey, base64.StdEncoding.EncodeToString(der)
}

func passkeyTestAssertion(t *testing.T, privateKey ed25519.PrivateKey, challenge, credentialID string) VerifyPasskeySignInInput {
	t.Helper()
	client, err := json.Marshal(map[string]string{"type": "webauthn.get", "challenge": challenge, "origin": "http://localhost"})
	if err != nil {
		t.Fatal(err)
	}
	rpHash := sha256.Sum256([]byte("localhost"))
	authData := append(rpHash[:], 1, 0, 0, 0, 0)
	clientHash := sha256.Sum256(client)
	signed := append(append([]byte{}, authData...), clientHash[:]...)
	return VerifyPasskeySignInInput{Challenge: challenge, CredentialID: credentialID, ClientDataJSON: base64.StdEncoding.EncodeToString(client), AuthenticatorData: base64.StdEncoding.EncodeToString(authData), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, signed))}
}

func TestAuthBaseHandlerPasskeyCeremoniesIntegration(t *testing.T) {
	privateKey, publicKey := passkeyTestKey(t)
	kernel, cleanupDatabase := core.SetupTestKernel(t, Migrations)
	defer cleanupDatabase()
	db := kernel.DB()

	ctx := context.Background()
	service := NewService(kernel)
	configManager := service.configManager
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load initial auth config: %v", err)
	}

	activeConfig := configManager.Get()
	activeConfig.Passkeys.Enabled = true
	configManager.SetMemoryConfig(activeConfig)

	eventBus := kernel.EventBus()
	jwtSigner := kernel.JWTSigner()
	baseHandler := service.baseHandler

	emittedEvents := make([]core.Event, 0)
	var eventsMutex sync.Mutex
	eventBus.Subscribe("auth.passkey.created", func(eventCtx context.Context, event core.Event) error {
		eventsMutex.Lock()
		emittedEvents = append(emittedEvents, event)
		eventsMutex.Unlock()
		return nil
	})
	eventBus.Subscribe("auth.user.converted", func(eventCtx context.Context, event core.Event) error {
		eventsMutex.Lock()
		emittedEvents = append(emittedEvents, event)
		eventsMutex.Unlock()
		return nil
	})

	// 1. Begin Sign-up Flow
	newUserID := "01918a24-1111-7000-8000-000000000001"
	signUpPayload, _ := json.Marshal(BeginPasskeySignUpInput{
		UserID:   newUserID,
		UserName: "Alice",
	})
	signUpRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-up", bytes.NewReader(signUpPayload))
	signUpResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignUp(signUpResponseRecorder, signUpRequest)
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
	verifySignUpPayload, _ := json.Marshal(VerifyPasskeySignUpInput{
		Challenge:    challenge,
		CredentialID: credentialID,
		PublicKey:    publicKey,
		FriendlyName: "MacBook TouchID",
		Transports:   []string{"internal"},
	})
	verifySignUpRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-up/verify", bytes.NewReader(verifySignUpPayload))
	verifySignUpResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignUp(verifySignUpResponseRecorder, verifySignUpRequest)
	if verifySignUpResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey sign-up verify, got: %d (%s)", verifySignUpResponseRecorder.Code, verifySignUpResponseRecorder.Body.String())
	}

	var signUpAuthTokenResponse AuthTokenResponse
	if err := json.NewDecoder(verifySignUpResponseRecorder.Body).Decode(&signUpAuthTokenResponse); err != nil || signUpAuthTokenResponse.AccessToken == "" {
		t.Fatalf("expected valid AuthTokenResponse, got: %+v (err: %v)", signUpAuthTokenResponse, err)
	}
	if signUpAuthTokenResponse.User.ID != newUserID {
		t.Fatalf("expected user ID %s, got: %s", newUserID, signUpAuthTokenResponse.User.ID)
	}

	// Verify DB record
	var passkeyCount int
	_ = db.QueryRow(ctx, "SELECT count(*) FROM auth.passkeys WHERE user_id = $1", newUserID).Scan(&passkeyCount)
	if passkeyCount != 1 {
		t.Fatalf("expected 1 passkey record in DB, got: %d", passkeyCount)
	}

	// 3. Begin Sign-in Flow
	signInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-in", strings.NewReader(`{}`))
	signInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignIn(signInResponseRecorder, signInRequest)
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
	verifySignInPayload, _ := json.Marshal(passkeyTestAssertion(t, privateKey, signInChallenge, credentialID))
	verifySignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-in/verify", bytes.NewReader(verifySignInPayload))
	verifySignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignIn(verifySignInResponseRecorder, verifySignInRequest)
	if verifySignInResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey sign-in verify, got: %d (%s)", verifySignInResponseRecorder.Code, verifySignInResponseRecorder.Body.String())
	}

	var signInAuthTokenResponse AuthTokenResponse
	if err := json.NewDecoder(verifySignInResponseRecorder.Body).Decode(&signInAuthTokenResponse); err != nil || signInAuthTokenResponse.AccessToken == "" {
		t.Fatalf("expected valid session response on sign-in verify: %+v", signInAuthTokenResponse)
	}

	// 5. Sign-in Verify with Non-existent Credential -> 401
	ghostChallenge, _ := baseHandler.passkeyManager.GenerateChallenge("")
	ghostVerifyPayload, _ := json.Marshal(VerifyPasskeySignInInput{
		Challenge:    ghostChallenge,
		CredentialID: "unknown-credential-id",
	})
	ghostVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-in/verify", bytes.NewReader(ghostVerifyPayload))
	ghostVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignIn(ghostVerifyResponseRecorder, ghostVerifyRequest)
	if ghostVerifyResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unknown credential verify, got: %d", ghostVerifyResponseRecorder.Code)
	}

	// 6. Sign-up Verify for Anonymous User (Converts anonymous caller)
	anonUserID := "01918a24-7777-7000-8000-000000000007"
	_, _ = db.Exec(ctx, `
		INSERT INTO auth.users (id, role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ($1, 'authenticated', true, '{"tier":"free"}'::jsonb, clock_timestamp(), clock_timestamp())
	`, anonUserID)
	anonToken := jwtSigner.GenerateAccessToken(core.JWTClaims{
		Subject:     anonUserID,
		Role:        "authenticated",
		IsAnonymous: true,
	}, 3600)
	anonAuthContext := core.AuthContext{UserID: anonUserID, JWT: core.JWTClaims{Subject: anonUserID, Role: "authenticated", IsAnonymous: true}}

	anonChallenge, _ := baseHandler.passkeyManager.GenerateChallenge(anonUserID)
	anonVerifyPayload, _ := json.Marshal(VerifyPasskeySignUpInput{
		Challenge:    anonChallenge,
		CredentialID: "credential-anon-999",
		PublicKey:    publicKey,
		FriendlyName: "YubiKey 5C",
		Transports:   []string{"usb", "nfc"},
	})
	anonVerifyRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), anonAuthContext), http.MethodPost, "/v1/auth/passkeys/sign-up/verify", bytes.NewReader(anonVerifyPayload))
	anonVerifyRequest.Header.Set("Authorization", "Bearer "+anonToken)
	anonVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignUp(anonVerifyResponseRecorder, anonVerifyRequest)
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
	canceledPayload, _ := json.Marshal(VerifyPasskeySignUpInput{
		Challenge:    canceledChallenge,
		CredentialID: "cred-canceled",
		PublicKey:    publicKey,
	})
	canceledVerifyRequest := httptest.NewRequestWithContext(canceledCtx, http.MethodPost, "/v1/auth/passkeys/sign-up/verify", bytes.NewReader(canceledPayload))
	canceledVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignUp(canceledVerifyResponseRecorder, canceledVerifyRequest)
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
		VALUES ('01918a24-8888-7000-8000-000000000099', $1, $2, $3, 0, clock_timestamp(), clock_timestamp())
	`, ghostUserID, []byte(ghostUserCredentialID), mustPasskeyDER(t, publicKey))
	if insertErr != nil {
		t.Fatalf("failed to insert orphaned passkey: %v", insertErr)
	}

	ghostSignInChallenge, _ := baseHandler.passkeyManager.GenerateChallenge("")
	ghostSignInPayload, _ := json.Marshal(passkeyTestAssertion(t, privateKey, ghostSignInChallenge, ghostUserCredentialID))
	ghostSignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-in/verify", bytes.NewReader(ghostSignInPayload))
	ghostSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignIn(ghostSignInResponseRecorder, ghostSignInRequest)
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
	failVerifyPayload, _ := json.Marshal(VerifyPasskeySignUpInput{
		Challenge:    failChallenge,
		CredentialID: "cred-fail-insert",
		PublicKey:    publicKey,
		FriendlyName: "fail_insert_trigger",
	})
	failVerifyRequest := httptest.NewRequestWithContext(core.WithAuthContext(ctx, core.AuthContext{UserID: newUserID}), http.MethodPost, "/v1/auth/passkeys/sign-up/verify", bytes.NewReader(failVerifyPayload))
	failVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignUp(failVerifyResponseRecorder, failVerifyRequest)
	if failVerifyResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on passkey insert trigger failure, got: %d", failVerifyResponseRecorder.Code)
	}
	_, _ = db.Exec(ctx, `
		DROP TRIGGER IF EXISTS trg_fail_passkey_insert ON auth.passkeys;
		DROP FUNCTION IF EXISTS auth.trg_fail_passkey_insert_fn();
	`)
}

func TestAuthBaseHandlerPasskeyManagementAndHardeningIntegration(t *testing.T) {
	privateKey, publicKey := passkeyTestKey(t)
	kernel, cleanupDatabase := core.SetupTestKernel(t, Migrations)
	defer cleanupDatabase()
	db := kernel.DB()

	ctx := context.Background()
	service := NewService(kernel)
	configManager := service.configManager
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load initial auth config: %v", err)
	}

	activeConfig := configManager.Get()
	activeConfig.Passkeys.Enabled = true
	configManager.SetMemoryConfig(activeConfig)

	eventBus := kernel.EventBus()
	jwtSigner := kernel.JWTSigner()
	baseHandler := service.baseHandler

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

	userToken := jwtSigner.GenerateAccessToken(core.JWTClaims{
		Subject: userID,
		Role:    "authenticated",
	}, 3600)
	bearerHeader := "Bearer " + userToken
	userAuthContext := core.AuthContext{UserID: userID, JWT: core.JWTClaims{Subject: userID, Role: "authenticated"}}

	// 1. Authenticated caller registers passkey using session token (empty user_id in payload)
	signUpPayload, _ := json.Marshal(BeginPasskeySignUpInput{
		UserName: "Bob Session",
	})
	signUpRequest := httptest.NewRequestWithContext(core.WithAuthContext(ctx, userAuthContext), http.MethodPost, "/v1/auth/passkeys/sign-up", bytes.NewReader(signUpPayload))
	signUpRequest.Header.Set("Authorization", bearerHeader)
	signUpResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignUp(signUpResponseRecorder, signUpRequest)
	if signUpResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on authenticated passkey sign-up, got: %d (%s)", signUpResponseRecorder.Code, signUpResponseRecorder.Body.String())
	}

	var signUpResponse map[string]any
	_ = json.NewDecoder(signUpResponseRecorder.Body).Decode(&signUpResponse)
	signUpChallenge := signUpResponse["challenge"].(string)

	credentialID1 := "cred-bob-session-1"
	verifySignUpPayload, _ := json.Marshal(VerifyPasskeySignUpInput{
		Challenge:    signUpChallenge,
		CredentialID: credentialID1,
		PublicKey:    publicKey,
		FriendlyName: "Work Laptop",
		Transports:   []string{"internal"},
	})
	verifySignUpRequest := httptest.NewRequestWithContext(core.WithAuthContext(ctx, userAuthContext), http.MethodPost, "/v1/auth/passkeys/sign-up/verify", bytes.NewReader(verifySignUpPayload))
	verifySignUpRequest.Header.Set("Authorization", bearerHeader)
	verifySignUpResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignUp(verifySignUpResponseRecorder, verifySignUpRequest)
	if verifySignUpResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on authenticated passkey sign-up verify, got: %d (%s)", verifySignUpResponseRecorder.Code, verifySignUpResponseRecorder.Body.String())
	}

	// 2. Register a second passkey for the authenticated owner
	challenge2, _ := baseHandler.passkeyManager.GenerateChallenge(userID)
	credentialID2 := "cred-bob-session-2"
	verify2Payload, _ := json.Marshal(VerifyPasskeySignUpInput{
		Challenge:    challenge2,
		CredentialID: credentialID2,
		PublicKey:    publicKey,
		FriendlyName: "Backup Security Key",
		Transports:   []string{"usb"},
	})
	verify2Request := httptest.NewRequestWithContext(core.WithAuthContext(ctx, userAuthContext), http.MethodPost, "/v1/auth/passkeys/sign-up/verify", bytes.NewReader(verify2Payload))
	verify2ResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignUp(verify2ResponseRecorder, verify2Request)
	if verify2ResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on second passkey verify, got: %d", verify2ResponseRecorder.Code)
	}

	// 3. List passkeys -> 200 with 2 items
	listRequest := httptest.NewRequestWithContext(core.WithAuthContext(ctx, userAuthContext), http.MethodGet, "/v1/auth/user/passkeys", nil)
	listRequest.Header.Set("Authorization", bearerHeader)
	listResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListPasskeys(listResponseRecorder, listRequest)
	if listResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on list user passkeys, got: %d (%s)", listResponseRecorder.Code, listResponseRecorder.Body.String())
	}

	var listPasskeysResponse ListPasskeysResponse
	if decodeErr := json.NewDecoder(listResponseRecorder.Body).Decode(&listPasskeysResponse); decodeErr != nil {
		t.Fatalf("failed to decode list passkeys response: %v", decodeErr)
	}
	if len(listPasskeysResponse) != 2 {
		t.Fatalf("expected 2 passkeys, got: %d", len(listPasskeysResponse))
	}

	// 4. Sign-in verify with assertion signature:
	// 4a. Valid signature over non-empty public privateKey -> 200
	signInChallenge1, _ := baseHandler.passkeyManager.GenerateChallenge("")
	verifySigPayload, _ := json.Marshal(passkeyTestAssertion(t, privateKey, signInChallenge1, credentialID1))
	verifySigRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/passkeys/sign-in/verify", bytes.NewReader(verifySigPayload))
	verifySigResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignIn(verifySigResponseRecorder, verifySigRequest)
	if verifySigResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey verify with valid signature, got: %d (%s)", verifySigResponseRecorder.Code, verifySigResponseRecorder.Body.String())
	}

	// 4b. Signature over credential with empty public privateKey -> 401
	signInChallenge2, _ := baseHandler.passkeyManager.GenerateChallenge("")
	invalidSigPayload, _ := json.Marshal(VerifyPasskeySignInInput{
		Challenge:         signInChallenge2,
		CredentialID:      credentialID2,
		ClientDataJSON:    `{"type":"webauthn.get"}`,
		AuthenticatorData: "authenticator-data",
		Signature:         "test-signature",
	})
	invalidSigRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/passkeys/sign-in/verify", bytes.NewReader(invalidSigPayload))
	invalidSigResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignIn(invalidSigResponseRecorder, invalidSigRequest)
	if invalidSigResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid passkey assertion signature, got: %d", invalidSigResponseRecorder.Code)
	}

	// 5. Sign-in verify with locked account -> 423
	lockedUntil := time.Now().UTC().Add(time.Hour)
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = $1 WHERE id = $2", lockedUntil, userID)

	lockedSignInChallenge, _ := baseHandler.passkeyManager.GenerateChallenge("")
	lockedPayload, _ := json.Marshal(passkeyTestAssertion(t, privateKey, lockedSignInChallenge, credentialID1))
	lockedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/passkeys/sign-in/verify", bytes.NewReader(lockedPayload))
	lockedResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignIn(lockedResponseRecorder, lockedRequest)
	if lockedResponseRecorder.Code != http.StatusLocked {
		t.Fatalf("expected 423 on locked user passkey sign-in, got: %d", lockedResponseRecorder.Code)
	}
	_, _ = db.Exec(ctx, "UPDATE auth.users SET locked_until = NULL WHERE id = $1", userID)

	// 6. Delete passkey -> 204 No Content
	passkeyToDeleteID := listPasskeysResponse[0].ID
	deleteRequest := httptest.NewRequestWithContext(core.WithAuthContext(ctx, userAuthContext), http.MethodDelete, "/v1/auth/user/passkeys/"+passkeyToDeleteID, nil)
	deleteRequest.SetPathValue("passkey_id", passkeyToDeleteID)
	deleteRequest.Header.Set("Authorization", bearerHeader)
	deleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeletePasskey(deleteResponseRecorder, deleteRequest)
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
	delete404Request := core.WithTestAuthContext(httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/auth/user/passkeys/"+passkeyToDeleteID, nil), userID, "authenticated", false)
	delete404Request.SetPathValue("passkey_id", passkeyToDeleteID)
	delete404Request.Header.Set("Authorization", bearerHeader)
	delete404ResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeletePasskey(delete404ResponseRecorder, delete404Request)
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

	passkeyToFailDeleteID := listPasskeysResponse[1].ID
	failDeleteRequest := core.WithTestAuthContext(httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/auth/user/passkeys/"+passkeyToFailDeleteID, nil), userID, "authenticated", false)
	failDeleteRequest.SetPathValue("passkey_id", passkeyToFailDeleteID)
	failDeleteRequest.Header.Set("Authorization", bearerHeader)
	failDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeletePasskey(failDeleteResponseRecorder, failDeleteRequest)
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
	canceledListRequest := core.WithTestAuthContext(httptest.NewRequestWithContext(canceledCtx, http.MethodGet, "/v1/auth/user/passkeys", nil), userID, "authenticated", false)
	canceledListRequest.Header.Set("Authorization", bearerHeader)
	canceledListResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListPasskeys(canceledListResponseRecorder, canceledListRequest)
	if canceledListResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on canceled context list passkeys, got: %d", canceledListResponseRecorder.Code)
	}
}

func mustPasskeyDER(t *testing.T, value string) []byte {
	t.Helper()
	der, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

func TestAuthPasskeyAssertionValidationIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	ctx := context.Background()
	service := NewService(kernel)
	config := DefaultConfig()
	config.Passkeys.Enabled = true
	service.configManager.SetMemoryConfig(config)
	privateKey, publicKey := passkeyTestKey(t)
	var userID string
	if err := kernel.DB().QueryRow(ctx, "INSERT INTO auth.users (email) VALUES ('passkey-validation@example.com') RETURNING id").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.DB().Exec(ctx, "INSERT INTO auth.passkeys (user_id,credential_id,public_key) VALUES ($1,$2,$3)", userID, []byte("credential-validation"), mustPasskeyDER(t, publicKey)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"empty signature", "wrong signature", "wrong origin", "wrong challenge", "wrong type", "wrong RP", "missing presence", "registration challenge"} {
		t.Run(name, func(t *testing.T) {
			challengeUserID := ""
			if name == "registration challenge" {
				challengeUserID = userID
			}
			challenge, err := service.passkeyManager.GenerateChallenge(challengeUserID)
			if err != nil {
				t.Fatal(err)
			}
			verifyPasskeySignInInput := passkeyTestAssertion(t, privateKey, challenge, "credential-validation")
			clientData, err := base64.StdEncoding.DecodeString(verifyPasskeySignInInput.ClientDataJSON)
			if err != nil {
				t.Fatal(err)
			}
			authData, err := base64.StdEncoding.DecodeString(verifyPasskeySignInInput.AuthenticatorData)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "wrong origin":
				clientData = bytes.ReplaceAll(clientData, []byte("http://localhost"), []byte("https://attacker.example"))
			case "wrong challenge":
				clientData = bytes.ReplaceAll(clientData, []byte(challenge), []byte("different-challenge"))
			case "wrong type":
				clientData = bytes.ReplaceAll(clientData, []byte("webauthn.get"), []byte("webauthn.create"))
			case "wrong RP":
				authData[0] ^= 1
			case "missing presence":
				authData[32] = 0
			}
			clientHash := sha256.Sum256(clientData)
			verifyPasskeySignInInput.ClientDataJSON = base64.StdEncoding.EncodeToString(clientData)
			verifyPasskeySignInInput.AuthenticatorData = base64.StdEncoding.EncodeToString(authData)
			verifyPasskeySignInInput.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, append(authData, clientHash[:]...)))
			if name == "empty signature" {
				verifyPasskeySignInInput.Signature = ""
			}
			if name == "wrong signature" {
				verifyPasskeySignInInput.Signature = base64.StdEncoding.EncodeToString([]byte("forged"))
			}
			payload, err := json.Marshal(verifyPasskeySignInInput)
			if err != nil {
				t.Fatal(err)
			}
			responseRecorder := httptest.NewRecorder()
			service.baseHandler.handleVerifyPasskeySignIn(responseRecorder, httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/passkeys/sign-in/verify", bytes.NewReader(payload)))
			if responseRecorder.Code != http.StatusUnauthorized && responseRecorder.Code != http.StatusBadRequest {
				t.Fatalf("accepted invalid assertion: %d", responseRecorder.Code)
			}
			if len(responseRecorder.Result().Cookies()) != 0 {
				t.Fatal("invalid assertion set a session cookie")
			}
		})
	}
	challenge, err := service.passkeyManager.GenerateChallenge(userID)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(VerifyPasskeySignUpInput{Challenge: challenge, CredentialID: "attacker-added", PublicKey: publicKey})
	if err != nil {
		t.Fatal(err)
	}
	responseRecorder := httptest.NewRecorder()
	service.baseHandler.handleVerifyPasskeySignUp(responseRecorder, httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/passkeys/sign-up/verify", bytes.NewReader(payload)))
	if responseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated registration targeted an existing user: %d", responseRecorder.Code)
	}
}

func TestAuthPasskeyKeyFormatsAndStorageFailuresIntegration(t *testing.T) {
	ctx := context.Background()
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	service := NewService(kernel)
	config := DefaultConfig()
	config.Passkeys.Enabled = true
	service.configManager.SetMemoryConfig(config)
	var userID string
	require.NoError(t, kernel.DB().QueryRow(ctx, "INSERT INTO auth.users DEFAULT VALUES RETURNING id").Scan(&userID))
	ecPrivateKey, ecErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, ecErr)
	rsaPrivateKey, rsaErr := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, rsaErr)
	for _, testCase := range []struct {
		name      string
		publicKey any
		pem       bool
	}{{"EC PEM", &ecPrivateKey.PublicKey, true}, {"RSA DER", &rsaPrivateKey.PublicKey, false}} {
		t.Run(testCase.name, func(t *testing.T) {
			der, err := x509.MarshalPKIXPublicKey(testCase.publicKey)
			require.NoError(t, err)
			publicKey := base64.StdEncoding.EncodeToString(der)
			if testCase.pem {
				publicKey = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
			}
			challenge, err := service.passkeyManager.GenerateChallenge(userID)
			require.NoError(t, err)
			payload, err := json.Marshal(VerifyPasskeySignUpInput{Challenge: challenge, CredentialID: testCase.name, PublicKey: publicKey})
			require.NoError(t, err)
			responseRecorder := httptest.NewRecorder()
			service.baseHandler.handleVerifyPasskeySignUp(responseRecorder, httptest.NewRequestWithContext(core.WithAuthContext(ctx, core.AuthContext{UserID: userID}), http.MethodPost, "/", bytes.NewReader(payload)))
			require.Equal(t, http.StatusOK, responseRecorder.Code, responseRecorder.Body.String())
		})
	}
	t.Run("invalid public key", func(t *testing.T) {
		challenge, err := service.passkeyManager.GenerateChallenge(userID)
		require.NoError(t, err)
		payload, err := json.Marshal(VerifyPasskeySignUpInput{Challenge: challenge, CredentialID: "invalid", PublicKey: "invalid"})
		require.NoError(t, err)
		responseRecorder := httptest.NewRecorder()
		service.baseHandler.handleVerifyPasskeySignUp(responseRecorder, httptest.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(payload)))
		require.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})
	privateKey, publicKey := passkeyTestKey(t)
	t.Run("user persistence fails", func(t *testing.T) {
		_, err := kernel.DB().Exec(ctx, `CREATE FUNCTION auth.fail_passkey_user() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'user persistence failed'; END $$; CREATE TRIGGER fail_passkey_user BEFORE INSERT ON auth.users FOR EACH ROW EXECUTE FUNCTION auth.fail_passkey_user()`)
		require.NoError(t, err)
		challenge, err := service.passkeyManager.GenerateChallenge(userID)
		require.NoError(t, err)
		payload, err := json.Marshal(VerifyPasskeySignUpInput{Challenge: challenge, CredentialID: "failed-user", PublicKey: publicKey})
		require.NoError(t, err)
		responseRecorder := httptest.NewRecorder()
		service.baseHandler.handleVerifyPasskeySignUp(responseRecorder, httptest.NewRequestWithContext(core.WithAuthContext(ctx, core.AuthContext{UserID: userID}), http.MethodPost, "/", bytes.NewReader(payload)))
		require.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		_, err = kernel.DB().Exec(ctx, "DROP TRIGGER fail_passkey_user ON auth.users; DROP FUNCTION auth.fail_passkey_user()")
		require.NoError(t, err)
	})
	_, err := kernel.DB().Exec(ctx, "INSERT INTO auth.passkeys (user_id,credential_id,public_key,counter) VALUES ($1,$2,$3,1)", userID, []byte("counter-credential"), mustPasskeyDER(t, publicKey))
	require.NoError(t, err)
	for _, name := range []string{"counter replay", "counter update fails", "default RP"} {
		t.Run(name, func(t *testing.T) {
			if name != "counter replay" {
				_, err := kernel.DB().Exec(ctx, "UPDATE auth.passkeys SET counter=0 WHERE credential_id=$1", []byte("counter-credential"))
				require.NoError(t, err)
			}
			if name == "counter update fails" {
				_, err := kernel.DB().Exec(ctx, `CREATE FUNCTION auth.skip_passkey_counter() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$; CREATE TRIGGER skip_passkey_counter BEFORE UPDATE ON auth.passkeys FOR EACH ROW EXECUTE FUNCTION auth.skip_passkey_counter()`)
				require.NoError(t, err)
			}
			if name == "default RP" {
				config.Passkeys.RelyingPartyID = ""
				service.configManager.SetMemoryConfig(config)
			}
			challenge, err := service.passkeyManager.GenerateChallenge("")
			require.NoError(t, err)
			payload, err := json.Marshal(passkeyTestAssertion(t, privateKey, challenge, "counter-credential"))
			require.NoError(t, err)
			responseRecorder := httptest.NewRecorder()
			service.baseHandler.handleVerifyPasskeySignIn(responseRecorder, httptest.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(payload)))
			expected := http.StatusUnauthorized
			if name == "default RP" {
				expected = http.StatusOK
			}
			require.Equal(t, expected, responseRecorder.Code, responseRecorder.Body.String())
			if name == "counter update fails" {
				_, err := kernel.DB().Exec(ctx, "DROP TRIGGER skip_passkey_counter ON auth.passkeys; DROP FUNCTION auth.skip_passkey_counter()")
				require.NoError(t, err)
			}
		})
	}
}
