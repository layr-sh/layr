package auth

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-fuego/fuego"
	"layr.sh/auth/passkey"
	"layr.sh/core"
)

func TestAuthPasskeyHandlerUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	handler := NewHandler(nil, configManager, cryptoKeyManager)
	testKVStore := newInMemoryKVStore()
	handler.SetKVStore(testKVStore)

	// 1. Passkeys disabled -> 403
	disabledConfig := configManager.Get()
	disabledConfig.Passkeys.Enabled = false
	configManager.Set(disabledConfig)

	passkeySignUpDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up", strings.NewReader(`{"user_id":"u1","user_name":"Alice"}`))
	passkeySignUpDisabledResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignUp(passkeySignUpDisabledResponseRecorder, passkeySignUpDisabledRequest)
	if passkeySignUpDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on passkey sign-up when passkeys disabled, got: %d", passkeySignUpDisabledResponseRecorder.Code)
	}

	passkeyVerifyDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up/verify", strings.NewReader(`{}`))
	passkeyVerifyDisabledResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignUpVerify(passkeyVerifyDisabledResponseRecorder, passkeyVerifyDisabledRequest)
	if passkeyVerifyDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on passkey verify when passkeys disabled, got: %d", passkeyVerifyDisabledResponseRecorder.Code)
	}

	passkeySignInDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-in", strings.NewReader(`{}`))
	passkeySignInDisabledResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignIn(passkeySignInDisabledResponseRecorder, passkeySignInDisabledRequest)
	if passkeySignInDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on passkey sign-in when passkeys disabled, got: %d", passkeySignInDisabledResponseRecorder.Code)
	}

	passkeySignInVerifyDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-in/verify", strings.NewReader(`{}`))
	passkeySignInVerifyDisabledResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignInVerify(passkeySignInVerifyDisabledResponseRecorder, passkeySignInVerifyDisabledRequest)
	if passkeySignInVerifyDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on passkey sign-in verify when passkeys disabled, got: %d", passkeySignInVerifyDisabledResponseRecorder.Code)
	}

	// Enable Passkeys
	enabledConfig := configManager.Get()
	enabledConfig.Passkeys.Enabled = true
	configManager.Set(enabledConfig)

	// 2. Passkey SignUp Bad JSON / Missing Fields
	badJSONPasskeySignUpRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up", strings.NewReader(`{invalid`))
	badJSONPasskeySignUpResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignUp(badJSONPasskeySignUpResponseRecorder, badJSONPasskeySignUpRequest)
	if badJSONPasskeySignUpResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON passkey sign-up, got: %d", badJSONPasskeySignUpResponseRecorder.Code)
	}

	missingUserIDPasskeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up", strings.NewReader(`{"user_name":"Alice"}`))
	missingUserIDPasskeyResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignUp(missingUserIDPasskeyResponseRecorder, missingUserIDPasskeyRequest)
	if missingUserIDPasskeyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing passkey user_id, got: %d", missingUserIDPasskeyResponseRecorder.Code)
	}

	// 3. Passkey SignUp Default Username -> 200
	testUserUUID := "01918a24-5678-789a-bcde-f0123456789a"
	emptyNamePasskeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up", strings.NewReader(`{"user_id":"`+testUserUUID+`"}`))
	emptyNamePasskeyResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignUp(emptyNamePasskeyResponseRecorder, emptyNamePasskeyRequest)
	if emptyNamePasskeyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey sign-up with default user name, got: %d", emptyNamePasskeyResponseRecorder.Code)
	}

	// 4. Passkey SignUp Verify Bad JSON & Mismatched User ID
	badJSONPasskeyVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up/verify", strings.NewReader(`{invalid`))
	badJSONPasskeyVerifyResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignUpVerify(badJSONPasskeyVerifyResponseRecorder, badJSONPasskeyVerifyRequest)
	if badJSONPasskeyVerifyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON in passkey verify, got: %d", badJSONPasskeyVerifyResponseRecorder.Code)
	}

	challenge1, _ := handler.passkeyManager.GenerateChallenge(testUserUUID)
	_ = testKVStore.Set(context.Background(), "auth:challenge:"+challenge1, testUserUUID, 0)
	mismatchUserPasskeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up/verify", strings.NewReader(`{"user_id":"mismatched-user-uuid","challenge":"`+challenge1+`"}`))
	mismatchUserPasskeyResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignUpVerify(mismatchUserPasskeyResponseRecorder, mismatchUserPasskeyRequest)
	if mismatchUserPasskeyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on mismatched user ID in passkey verify, got: %d", mismatchUserPasskeyResponseRecorder.Code)
	}

	// 5. Passkey SignUp Verify Empty Friendly Name on Nil Pool -> 500
	challenge2, _ := handler.passkeyManager.GenerateChallenge(testUserUUID)
	_ = testKVStore.Set(context.Background(), "auth:challenge:"+challenge2, testUserUUID, 0)
	emptyFriendlyPasskeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up/verify", strings.NewReader(`{"user_id":"`+testUserUUID+`","challenge":"`+challenge2+`","credential_id":"cred_123","public_key":"pub_key_123"}`))
	emptyFriendlyPasskeyResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignUpVerify(emptyFriendlyPasskeyResponseRecorder, emptyFriendlyPasskeyRequest)
	if emptyFriendlyPasskeyResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on passkey verify with nil pool, got: %d", emptyFriendlyPasskeyResponseRecorder.Code)
	}

	// 6. Passkey SignIn -> 200
	passkeySignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-in", strings.NewReader(`{}`))
	passkeySignInResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignIn(passkeySignInResponseRecorder, passkeySignInRequest)
	if passkeySignInResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey sign-in, got: %d", passkeySignInResponseRecorder.Code)
	}

	// 7. Passkey SignIn Verify with Invalid/Consumed Challenge -> 400
	invalidPasskeySignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-in/verify", strings.NewReader(`{"challenge":"non-existent-challenge"}`))
	invalidPasskeySignInResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignInVerify(invalidPasskeySignInResponseRecorder, invalidPasskeySignInRequest)
	if invalidPasskeySignInResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid challenge in passkey signin verify, got: %d", invalidPasskeySignInResponseRecorder.Code)
	}

	// 8. Passkey SignIn Verify Bad JSON -> 400
	badJSONPasskeySignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-in/verify", bytes.NewReader([]byte(`{invalid`)))
	badJSONPasskeySignInResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignInVerify(badJSONPasskeySignInResponseRecorder, badJSONPasskeySignInRequest)
	if badJSONPasskeySignInResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON in passkey signin verify, got: %d", badJSONPasskeySignInResponseRecorder.Code)
	}

	// 9. Passkey SignIn Verify Valid Challenge on Nil Pool -> 500
	validSignInChallenge, _ := handler.passkeyManager.GenerateChallenge("")
	_ = testKVStore.Set(context.Background(), "auth:challenge:"+validSignInChallenge, "", 0)
	nilDBPasskeySignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-in/verify", strings.NewReader(`{"challenge":"`+validSignInChallenge+`","credential_id":"cred_123"}`))
	nilDBPasskeySignInResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignInVerify(nilDBPasskeySignInResponseRecorder, nilDBPasskeySignInRequest)
	if nilDBPasskeySignInResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on passkey sign-in verify with nil pool, got: %d", nilDBPasskeySignInResponseRecorder.Code)
	}

	// 10. RegisterPasskeyRoutes
	fuegoEngine := fuego.NewServer()
	router := core.NewRouter(fuegoEngine)
	handler.RegisterPasskeyRoutes(router)

	// 11. Entropy failure branches -> 500
	failingPasskeyManager := passkey.NewManager("localhost", "Layr")
	failingPasskeyManager.SetRandomReader(errEntropyReader{})
	handler.SetPasskeyManager(failingPasskeyManager)

	failingSignUpRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-up", strings.NewReader(`{"user_id":"u-entropy"}`))
	failingSignUpResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignUp(failingSignUpResponseRecorder, failingSignUpRequest)
	if failingSignUpResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on entropy failure sign-up, got: %d", failingSignUpResponseRecorder.Code)
	}

	failingSignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/passkeys/sign-in", strings.NewReader(`{}`))
	failingSignInResponseRecorder := httptest.NewRecorder()
	handler.handlePasskeySignIn(failingSignInResponseRecorder, failingSignInRequest)
	if failingSignInResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on entropy failure sign-in, got: %d", failingSignInResponseRecorder.Code)
	}
}

type errEntropyReader struct{}

func (errEntropyReader) Read(_ []byte) (int, error) {
	return 0, errors.New("entropy failure")
}
