package auth

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-fuego/fuego"
	"layr.sh/core"
)

func TestAuthHandlerAuthUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	handler := NewHandler(nil, configManager, cryptoKeyManager)
	ctx := context.Background()

	// 1. Password disabled -> 403 on signup
	disabledConfig := DefaultConfig()
	disabledConfig.Password.Enabled = false
	configManager.Set(disabledConfig)

	signupDisabledRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-up", strings.NewReader(`{"email":"test@example.com","password":"Password123!"}`))
	signupDisabledResponseRecorder := httptest.NewRecorder()
	handler.handleSignUp(signupDisabledResponseRecorder, signupDisabledRequest)
	if signupDisabledResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when password disabled, got: %d", signupDisabledResponseRecorder.Code)
	}

	// 2. Validation errors on SignUp
	configManager.Set(DefaultConfig())

	badJSONSignupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-up", bytes.NewReader([]byte(`bad-json`)))
	badJSONSignupResponseRecorder := httptest.NewRecorder()
	handler.handleSignUp(badJSONSignupResponseRecorder, badJSONSignupRequest)
	if badJSONSignupResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON signup, got: %d", badJSONSignupResponseRecorder.Code)
	}

	missingEmailSignupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-up", strings.NewReader(`{"password":"Password123!"}`))
	missingEmailSignupResponseRecorder := httptest.NewRecorder()
	handler.handleSignUp(missingEmailSignupResponseRecorder, missingEmailSignupRequest)
	if missingEmailSignupResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing email/phone signup, got: %d", missingEmailSignupResponseRecorder.Code)
	}

	shortPasswordSignupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-up", strings.NewReader(`{"email":"test@example.com","password":"short"}`))
	shortPasswordSignupResponseRecorder := httptest.NewRecorder()
	handler.handleSignUp(shortPasswordSignupResponseRecorder, shortPasswordSignupRequest)
	if shortPasswordSignupResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on short password, got: %d", shortPasswordSignupResponseRecorder.Code)
	}

	invalidPhoneSignupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-up", strings.NewReader(`{"phone":"invalid","password":"Password123!"}`))
	invalidPhoneSignupResponseRecorder := httptest.NewRecorder()
	handler.handleSignUp(invalidPhoneSignupResponseRecorder, invalidPhoneSignupRequest)
	if invalidPhoneSignupResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone signup, got: %d", invalidPhoneSignupResponseRecorder.Code)
	}

	// 3. Validation errors on SignIn
	invalidPhoneSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", strings.NewReader(`{"phone":"invalid","password":"Password123!"}`))
	invalidPhoneSignInResponseRecorder := httptest.NewRecorder()
	handler.handleSignIn(invalidPhoneSignInResponseRecorder, invalidPhoneSignInRequest)
	if invalidPhoneSignInResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid phone signin, got: %d", invalidPhoneSignInResponseRecorder.Code)
	}

	badSignInJSONRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", bytes.NewReader([]byte(`bad-json`)))
	badSignInJSONResponseRecorder := httptest.NewRecorder()
	handler.handleSignIn(badSignInJSONResponseRecorder, badSignInJSONRequest)
	if badSignInJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON signin, got: %d", badSignInJSONResponseRecorder.Code)
	}

	missingCredentialsSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", strings.NewReader(`{"email":"test@example.com"}`))
	missingCredentialsSignInResponseRecorder := httptest.NewRecorder()
	handler.handleSignIn(missingCredentialsSignInResponseRecorder, missingCredentialsSignInRequest)
	if missingCredentialsSignInResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing password signin, got: %d", missingCredentialsSignInResponseRecorder.Code)
	}

	// 4. Validation errors on Refresh
	badRefreshJSONRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/token/refresh", bytes.NewReader([]byte(`bad-json`)))
	badRefreshJSONResponseRecorder := httptest.NewRecorder()
	handler.handleTokenRefresh(badRefreshJSONResponseRecorder, badRefreshJSONRequest)
	if badRefreshJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad refresh JSON, got: %d", badRefreshJSONResponseRecorder.Code)
	}

	missingTokenRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/token/refresh", strings.NewReader(`{}`))
	missingTokenRefreshResponseRecorder := httptest.NewRecorder()
	handler.handleTokenRefresh(missingTokenRefreshResponseRecorder, missingTokenRefreshRequest)
	if missingTokenRefreshResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing refresh token, got: %d", missingTokenRefreshResponseRecorder.Code)
	}

	// 5. SignOut
	signOutRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-out", nil)
	signOutResponseRecorder := httptest.NewRecorder()
	handler.handleSignOut(signOutResponseRecorder, signOutRequest)
	if signOutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on signout, got: %d", signOutResponseRecorder.Code)
	}

	signOutWithBodyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-out", strings.NewReader(`{"refresh_token":"dummy-refresh-token"}`))
	signOutWithBodyResponseRecorder := httptest.NewRecorder()
	handler.handleSignOut(signOutWithBodyResponseRecorder, signOutWithBodyRequest)
	if signOutWithBodyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on signout with body, got: %d", signOutWithBodyResponseRecorder.Code)
	}

	// 6. Nil database pool -> 500 on valid inputs
	phoneSignupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-up", strings.NewReader(`{"phone":"+1234567890","password":"Password123!"}`))
	phoneSignupResponseRecorder := httptest.NewRecorder()
	handler.handleSignUp(phoneSignupResponseRecorder, phoneSignupRequest)
	if phoneSignupResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on phone signup nil db pool, got: %d", phoneSignupResponseRecorder.Code)
	}

	phoneSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", strings.NewReader(`{"phone":"+1234567890","password":"Password123!"}`))
	phoneSignInResponseRecorder := httptest.NewRecorder()
	handler.handleSignIn(phoneSignInResponseRecorder, phoneSignInRequest)
	if phoneSignInResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on phone signin nil db pool, got: %d", phoneSignInResponseRecorder.Code)
	}

	emailSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/sign-in", strings.NewReader(`{"email":"alice@example.com","password":"Password123!"}`))
	emailSignInResponseRecorder := httptest.NewRecorder()
	handler.handleSignIn(emailSignInResponseRecorder, emailSignInRequest)
	if emailSignInResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on email signin nil db pool, got: %d", emailSignInResponseRecorder.Code)
	}

	validRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/token/refresh", strings.NewReader(`{"refresh_token":"valid-refresh-token"}`))
	validRefreshResponseRecorder := httptest.NewRecorder()
	handler.handleTokenRefresh(validRefreshResponseRecorder, validRefreshRequest)
	if validRefreshResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on refresh nil db pool, got: %d", validRefreshResponseRecorder.Code)
	}

	// 7. Anonymous Sign In Unit Tests
	disabledAnonymousConfig := DefaultConfig()
	disabledAnonymousConfig.Anonymous.Enabled = false
	configManager.Set(disabledAnonymousConfig)

	anonymousDisabledRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/anonymous", nil)
	anonymousDisabledResponseRecorder := httptest.NewRecorder()
	handler.handleAnonymousSignIn(anonymousDisabledResponseRecorder, anonymousDisabledRequest)
	if anonymousDisabledResponseRecorder.Code != http.StatusForbidden || !strings.Contains(anonymousDisabledResponseRecorder.Body.String(), "LAYR_AUTH_001") {
		t.Fatalf("expected 403 LAYR_AUTH_001 on disabled anonymous auth, got: %d (%s)", anonymousDisabledResponseRecorder.Code, anonymousDisabledResponseRecorder.Body.String())
	}

	configManager.Set(DefaultConfig())
	anonymousNilDBRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/anonymous", strings.NewReader(`{"properties":{"source":"mobile"}}`))
	anonymousNilDBResponseRecorder := httptest.NewRecorder()
	handler.handleAnonymousSignIn(anonymousNilDBResponseRecorder, anonymousNilDBRequest)
	if anonymousNilDBResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on anonymous signin with nil db, got: %d", anonymousNilDBResponseRecorder.Code)
	}

	// 8. RegisterAuthRoutes
	fuegoEngine := fuego.NewServer()
	router := core.NewRouter(fuegoEngine)
	handler.RegisterAuthRoutes(router)
}
