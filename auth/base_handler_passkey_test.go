package auth

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"layr.sh/auth/passkey"
	"layr.sh/core"
)

func TestAuthPasskeyHandlerUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := service.baseHandler
	configManager := service.configManager
	testKVStore := kernel.KVStore()

	// 1. Passkeys disabled -> 403
	disabledConfig := configManager.Get()
	disabledConfig.Passkeys.Enabled = false
	configManager.SetMemoryConfig(disabledConfig)

	passkeySignUpDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-up", strings.NewReader(`{"user_id":"u1","user_name":"Alice"}`))
	passkeySignUpDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignUp(passkeySignUpDisabledResponseRecorder, passkeySignUpDisabledRequest)
	if passkeySignUpDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on passkey sign-up when passkeys disabled, got: %d", passkeySignUpDisabledResponseRecorder.Code)
	}

	passkeyVerifyDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-up/verify", strings.NewReader(`{}`))
	passkeyVerifyDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignUp(passkeyVerifyDisabledResponseRecorder, passkeyVerifyDisabledRequest)
	if passkeyVerifyDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on passkey verify when passkeys disabled, got: %d", passkeyVerifyDisabledResponseRecorder.Code)
	}

	passkeySignInDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-in", strings.NewReader(`{}`))
	passkeySignInDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignIn(passkeySignInDisabledResponseRecorder, passkeySignInDisabledRequest)
	if passkeySignInDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on passkey sign-in when passkeys disabled, got: %d", passkeySignInDisabledResponseRecorder.Code)
	}

	passkeySignInVerifyDisabledRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-in/verify", strings.NewReader(`{}`))
	passkeySignInVerifyDisabledResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignIn(passkeySignInVerifyDisabledResponseRecorder, passkeySignInVerifyDisabledRequest)
	if passkeySignInVerifyDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on passkey sign-in verify when passkeys disabled, got: %d", passkeySignInVerifyDisabledResponseRecorder.Code)
	}

	// Enable Passkeys
	enabledConfig := configManager.Get()
	enabledConfig.Passkeys.Enabled = true
	configManager.SetMemoryConfig(enabledConfig)

	// 2. Passkey SignUp Bad JSON / Missing Fields
	badJSONPasskeySignUpRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-up", strings.NewReader(`{invalid`))
	badJSONPasskeySignUpResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignUp(badJSONPasskeySignUpResponseRecorder, badJSONPasskeySignUpRequest)
	if badJSONPasskeySignUpResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON passkey sign-up, got: %d", badJSONPasskeySignUpResponseRecorder.Code)
	}

	missingUserIDPasskeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-up", strings.NewReader(`{"user_name":"Alice"}`))
	missingUserIDPasskeyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignUp(missingUserIDPasskeyResponseRecorder, missingUserIDPasskeyRequest)
	if missingUserIDPasskeyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing passkey user_id, got: %d", missingUserIDPasskeyResponseRecorder.Code)
	}

	// 3. Passkey SignUp Default Username -> 200
	testUserUUID := "01918a24-5678-789a-bcde-f0123456789a"
	emptyNamePasskeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-up", strings.NewReader(`{"user_id":"`+testUserUUID+`"}`))
	emptyNamePasskeyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignUp(emptyNamePasskeyResponseRecorder, emptyNamePasskeyRequest)
	if emptyNamePasskeyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey sign-up with default user name, got: %d", emptyNamePasskeyResponseRecorder.Code)
	}

	// 4. Passkey SignUp Verify Bad JSON & Mismatched User ID
	badJSONPasskeyVerifyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-up/verify", strings.NewReader(`{invalid`))
	badJSONPasskeyVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignUp(badJSONPasskeyVerifyResponseRecorder, badJSONPasskeyVerifyRequest)
	if badJSONPasskeyVerifyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON in passkey verify, got: %d", badJSONPasskeyVerifyResponseRecorder.Code)
	}

	challenge1, _ := baseHandler.passkeyManager.GenerateChallenge(testUserUUID)
	_ = testKVStore.Set(context.Background(), "auth:challenge:"+challenge1, testUserUUID, 0)
	mismatchUserPasskeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-up/verify", strings.NewReader(`{"user_id":"mismatched-user-uuid","challenge":"`+challenge1+`"}`))
	mismatchUserPasskeyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignUp(mismatchUserPasskeyResponseRecorder, mismatchUserPasskeyRequest)
	if mismatchUserPasskeyResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on mismatched user ID in passkey verify, got: %d", mismatchUserPasskeyResponseRecorder.Code)
	}

	// 5. Passkey SignUp Verify Empty Friendly Name on Nil Pool -> 500
	challenge2, _ := baseHandler.passkeyManager.GenerateChallenge(testUserUUID)
	_ = testKVStore.Set(context.Background(), "auth:challenge:"+challenge2, testUserUUID, 0)
	emptyFriendlyPasskeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-up/verify", strings.NewReader(`{"user_id":"`+testUserUUID+`","challenge":"`+challenge2+`","credential_id":"cred_123","public_key":"pub_key_123"}`))
	emptyFriendlyPasskeyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignUp(emptyFriendlyPasskeyResponseRecorder, emptyFriendlyPasskeyRequest)
	if emptyFriendlyPasskeyResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on passkey verify with nil pool, got: %d", emptyFriendlyPasskeyResponseRecorder.Code)
	}

	// 6. Passkey SignIn -> 200
	passkeySignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-in", strings.NewReader(`{}`))
	passkeySignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignIn(passkeySignInResponseRecorder, passkeySignInRequest)
	if passkeySignInResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey sign-in, got: %d", passkeySignInResponseRecorder.Code)
	}

	// 7. Passkey SignIn Verify with Invalid/Consumed Challenge -> 400
	invalidPasskeySignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-in/verify", strings.NewReader(`{"challenge":"non-existent-challenge"}`))
	invalidPasskeySignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignIn(invalidPasskeySignInResponseRecorder, invalidPasskeySignInRequest)
	if invalidPasskeySignInResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid challenge in passkey sign-in verify, got: %d", invalidPasskeySignInResponseRecorder.Code)
	}

	// 8. Passkey SignIn Verify Bad JSON -> 400
	badJSONPasskeySignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-in/verify", bytes.NewReader([]byte(`{invalid`)))
	badJSONPasskeySignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignIn(badJSONPasskeySignInResponseRecorder, badJSONPasskeySignInRequest)
	if badJSONPasskeySignInResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON in passkey sign-in verify, got: %d", badJSONPasskeySignInResponseRecorder.Code)
	}

	// 9. Passkey SignIn Verify Valid Challenge on non-existent credential -> 401
	validSignInChallenge, _ := baseHandler.passkeyManager.GenerateChallenge("")
	_ = testKVStore.Set(context.Background(), "auth:challenge:"+validSignInChallenge, "", 0)
	nilDBPasskeySignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-in/verify", strings.NewReader(`{"challenge":"`+validSignInChallenge+`","credential_id":"cred_123"}`))
	nilDBPasskeySignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignIn(nilDBPasskeySignInResponseRecorder, nilDBPasskeySignInRequest)
	if nilDBPasskeySignInResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on passkey sign-in verify with non-existent credential, got: %d", nilDBPasskeySignInResponseRecorder.Code)
	}

	// 10. Entropy failure branches -> 500
	failingPasskeyManager := passkey.NewManager("localhost", "Layr")
	failingPasskeyManager.SetRandomReader(errEntropyReader{})
	baseHandler.passkeyManager = failingPasskeyManager

	failingSignUpRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-up", strings.NewReader(`{"user_id":"u-entropy"}`))
	failingSignUpResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignUp(failingSignUpResponseRecorder, failingSignUpRequest)
	if failingSignUpResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on entropy failure sign-up, got: %d", failingSignUpResponseRecorder.Code)
	}

	failingSignInRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/passkeys/sign-in", strings.NewReader(`{}`))
	failingSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignIn(failingSignInResponseRecorder, failingSignInRequest)
	if failingSignInResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on entropy failure sign-in, got: %d", failingSignInResponseRecorder.Code)
	}
}

type errEntropyReader struct{}

func (errEntropyReader) Read(_ []byte) (int, error) {
	return 0, errors.New("entropy failure")
}

func TestAuthPasskeyManagementHandlerUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)
	baseHandler := service.baseHandler
	testKVStore := kernel.KVStore()
	jwtSigner := kernel.JWTSigner()

	testUserUUID := "01918a24-5678-789a-bcde-f0123456789a"
	validToken := jwtSigner.GenerateAccessToken(core.JWTClaims{
		Subject: testUserUUID,
		Role:    "authenticated",
	}, 3600)
	bearerHeader := "Bearer " + validToken
	authContext := core.AuthContext{UserID: testUserUUID, JWT: core.JWTClaims{Subject: testUserUUID, Role: "authenticated"}}

	// 1. List user passkeys - unauthenticated -> 401
	unauthListRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/user/passkeys", nil)
	unauthListResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListPasskeys(unauthListResponseRecorder, unauthListRequest)
	if unauthListResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthenticated list user passkeys, got: %d", unauthListResponseRecorder.Code)
	}

	// 2. List user passkeys - authenticated on nil DB -> 500
	authListRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), authContext), http.MethodGet, "/v1/auth/user/passkeys", nil)
	authListRequest.Header.Set("Authorization", bearerHeader)
	authListResponseRecorder := httptest.NewRecorder()
	baseHandler.handleListPasskeys(authListResponseRecorder, authListRequest)
	if authListResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on list user passkeys with nil DB, got: %d", authListResponseRecorder.Code)
	}

	// 3. Delete user passkey - unauthenticated -> 401
	unauthDeleteRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/v1/auth/user/passkeys/pk-1", nil)
	unauthDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeletePasskey(unauthDeleteResponseRecorder, unauthDeleteRequest)
	if unauthDeleteResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unauthenticated delete user passkey, got: %d", unauthDeleteResponseRecorder.Code)
	}

	// 4. Delete user passkey - authenticated with empty ID -> 400
	emptyIDDeleteRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), authContext), http.MethodDelete, "/v1/auth/user/passkeys/", nil)
	emptyIDDeleteRequest.Header.Set("Authorization", bearerHeader)
	emptyIDDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeletePasskey(emptyIDDeleteResponseRecorder, emptyIDDeleteRequest)
	if emptyIDDeleteResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on delete passkey with empty ID, got: %d", emptyIDDeleteResponseRecorder.Code)
	}

	// 5. Delete user passkey - authenticated with valid ID on nil DB -> 500
	validIDDeleteRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), authContext), http.MethodDelete, "/v1/auth/user/passkeys/pk-1", nil)
	validIDDeleteRequest.SetPathValue("passkey_id", "pk-1")
	validIDDeleteRequest.Header.Set("Authorization", bearerHeader)
	validIDDeleteResponseRecorder := httptest.NewRecorder()
	baseHandler.handleDeletePasskey(validIDDeleteResponseRecorder, validIDDeleteRequest)
	if validIDDeleteResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on delete passkey with nil DB, got: %d", validIDDeleteResponseRecorder.Code)
	}

	// 6. Passkey SignUp with Bearer Token and empty user_id -> 200
	authSignUpRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), authContext), http.MethodPost, "/v1/auth/passkeys/sign-up", strings.NewReader(`{"user_name":"Alice"}`))
	authSignUpRequest.Header.Set("Authorization", bearerHeader)
	authSignUpResponseRecorder := httptest.NewRecorder()
	baseHandler.handleBeginPasskeySignUp(authSignUpResponseRecorder, authSignUpRequest)
	if authSignUpResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on passkey sign-up with bearer token, got: %d", authSignUpResponseRecorder.Code)
	}

	// 7. Passkey SignUpVerify with Bearer Token and empty user_id on nil DB -> 500
	challenge, _ := baseHandler.passkeyManager.GenerateChallenge(testUserUUID)
	_ = testKVStore.Set(context.Background(), "auth:challenge:"+challenge, testUserUUID, 0)
	authVerifyRequest := httptest.NewRequestWithContext(core.WithAuthContext(context.Background(), authContext), http.MethodPost, "/v1/auth/passkeys/sign-up/verify", strings.NewReader(`{"challenge":"`+challenge+`","credential_id":"cred_123","public_key":"pub_123"}`))
	authVerifyRequest.Header.Set("Authorization", bearerHeader)
	authVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleVerifyPasskeySignUp(authVerifyResponseRecorder, authVerifyRequest)
	if authVerifyResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on passkey verify with bearer token on nil DB, got: %d", authVerifyResponseRecorder.Code)
	}
}
