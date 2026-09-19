package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"layr.sh/auth/oauth"
	"layr.sh/core"
)

func TestAuthHandlerOAuthUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	baseHandler := NewBaseHandler(nil, configManager, cryptoKeyManager)
	testKVStore := newInMemoryKVStore()
	baseHandler.SetKVStore(testKVStore)

	// 1. Unknown provider -> 404
	unknownProviderOAuthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/unknown_provider/authorize", nil)
	unknownProviderOAuthRequest.SetPathValue("provider", "unknown_provider")
	unknownProviderOAuthResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthAuthorize(unknownProviderOAuthResponseRecorder, unknownProviderOAuthRequest)
	if unknownProviderOAuthResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on unknown OAuth provider, got: %d", unknownProviderOAuthResponseRecorder.Code)
	}

	// 1b. Disabled provider authorize -> 403
	oauthDisabledConfig := DefaultConfig()
	oauthDisabledConfig.OAuthProviders["disabled_provider"] = OAuthProviderConfig{
		Enabled:  false,
		ClientID: "disabled-id",
	}
	configManager.Set(oauthDisabledConfig)
	disabledOAuthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/disabled_provider/authorize", nil)
	disabledOAuthRequest.SetPathValue("provider", "disabled_provider")
	disabledOAuthResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthAuthorize(disabledOAuthResponseRecorder, disabledOAuthRequest)
	if disabledOAuthResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on disabled OAuth provider authorize, got: %d", disabledOAuthResponseRecorder.Code)
	}

	// 2. Authorize with scope, redirect URI, and state -> 302
	oauthConfig := DefaultConfig()
	oauthConfig.OAuthProviders["google"] = OAuthProviderConfig{
		Enabled:  true,
		ClientID: "google-test-id",
		Scope:    "openid profile email custom_scope",
	}
	configManager.Set(oauthConfig)

	oauthAuthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/authorize?redirect_uri=http://localhost:3000/callback&state=teststate123&scope=custom_scope", nil)
	oauthAuthRequest.SetPathValue("provider", "google")
	oauthAuthResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthAuthorize(oauthAuthResponseRecorder, oauthAuthRequest)
	if oauthAuthResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on OAuth authorize, got: %d", oauthAuthResponseRecorder.Code)
	}
	redirectLocation := oauthAuthResponseRecorder.Header().Get("Location")
	if !strings.Contains(redirectLocation, "custom_scope") || !strings.Contains(redirectLocation, "teststate123") {
		t.Fatalf("unexpected redirect location: %s", redirectLocation)
	}

	// 3. Authorize with default redirect URI
	defaultRedirectRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/authorize", nil)
	defaultRedirectRequest.SetPathValue("provider", "google")
	defaultRedirectResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthAuthorize(defaultRedirectResponseRecorder, defaultRedirectRequest)
	if defaultRedirectResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 on default redirect URI authorize, got: %d", defaultRedirectResponseRecorder.Code)
	}

	// 4. Broken provider without endpoints -> 400
	brokenOAuthConfig := DefaultConfig()
	brokenOAuthConfig.OAuthProviders["broken"] = OAuthProviderConfig{
		Enabled: true,
	}
	brokenOAuthConfigManager := NewConfigManager(nil, cryptoKeyManager)
	brokenOAuthConfigManager.Set(brokenOAuthConfig)
	brokenOAuthBaseHandler := NewBaseHandler(nil, brokenOAuthConfigManager, cryptoKeyManager)
	brokenOAuthBaseHandler.SetKVStore(testKVStore)

	brokenOAuthAuthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/broken/authorize", nil)
	brokenOAuthAuthRequest.SetPathValue("provider", "broken")
	brokenOAuthAuthResponseRecorder := httptest.NewRecorder()
	brokenOAuthBaseHandler.HandleOAuthAuthorize(brokenOAuthAuthResponseRecorder, brokenOAuthAuthRequest)
	if brokenOAuthAuthResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on broken oauth authorize, got: %d", brokenOAuthAuthResponseRecorder.Code)
	}

	_ = testKVStore.Set(context.Background(), "auth:pkce:broken-state", "broken-state", 10*time.Minute)
	brokenOAuthCallbackRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/broken/callback?code=mock_code&state=broken-state", nil)
	brokenOAuthCallbackRequest.SetPathValue("provider", "broken")
	brokenOAuthCallbackResponseRecorder := httptest.NewRecorder()
	brokenOAuthBaseHandler.HandleOAuthCallback(brokenOAuthCallbackResponseRecorder, brokenOAuthCallbackRequest)
	if brokenOAuthCallbackResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on broken oauth callback, got: %d", brokenOAuthCallbackResponseRecorder.Code)
	}

	// 5. Token form & JSON endpoints
	_ = testKVStore.Set(context.Background(), "auth:pkce:form-state", "form-state", 10*time.Minute)
	oauthTokenFormRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/google/token", strings.NewReader("code=authcode123&redirect_uri=http://localhost:3000/callback&state=form-state"))
	oauthTokenFormRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	oauthTokenFormResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthToken(oauthTokenFormResponseRecorder, oauthTokenFormRequest)
	if oauthTokenFormResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on fake code exchange, got: %d", oauthTokenFormResponseRecorder.Code)
	}

	_ = testKVStore.Set(context.Background(), "auth:pkce:json-state", "json-state", 10*time.Minute)
	oauthTokenJSONRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(`{"provider":"google","code":"authcode123","state":"json-state"}`))
	oauthTokenJSONResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthToken(oauthTokenJSONResponseRecorder, oauthTokenJSONRequest)
	if oauthTokenJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on fake code JSON exchange, got: %d", oauthTokenJSONResponseRecorder.Code)
	}

	oauthTokenGrantRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=client_credentials"))
	oauthTokenGrantRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	oauthTokenGrantResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthToken(oauthTokenGrantResponseRecorder, oauthTokenGrantRequest)
	if oauthTokenGrantResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on grant_type token exchange without credentials, got: %d", oauthTokenGrantResponseRecorder.Code)
	}

	// 6. Callback provider error parameter -> 400
	_ = testKVStore.Set(context.Background(), "auth:pkce:err-state", "err-state", 10*time.Minute)
	oauthErrorCallbackRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?error=access_denied&error_description=user_cancelled&state=err-state", nil)
	oauthErrorCallbackResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(oauthErrorCallbackResponseRecorder, oauthErrorCallbackRequest)
	if oauthErrorCallbackResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on oauth error callback, got: %d", oauthErrorCallbackResponseRecorder.Code)
	}

	// 7. Callback with code and state -> 400 (mock code fails live exchange)
	_ = testKVStore.Set(context.Background(), "auth:pkce:mock-state", "mock-state", 10*time.Minute)
	oauthGetCallbackRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=mock-code&state=mock-state", nil)
	oauthGetCallbackResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(oauthGetCallbackResponseRecorder, oauthGetCallbackRequest)
	if oauthGetCallbackResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on mock get callback, got: %d", oauthGetCallbackResponseRecorder.Code)
	}

	// 8. Callback with disabled provider -> 404
	_ = testKVStore.Set(context.Background(), "auth:pkce:disabled-provider-state", "disabled-provider-state", 10*time.Minute)
	oauthDisabledCallbackRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/unknown_disabled_provider/callback?code=123&state=disabled-provider-state", nil)
	oauthDisabledCallbackResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(oauthDisabledCallbackResponseRecorder, oauthDisabledCallbackRequest)
	if oauthDisabledCallbackResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on disabled oauth provider callback, got: %d", oauthDisabledCallbackResponseRecorder.Code)
	}

	// 8b. Callback with configured disabled provider -> 403
	oauthConfig.OAuthProviders["disabled_provider"] = OAuthProviderConfig{
		Enabled:  false,
		ClientID: "disabled-id",
	}
	configManager.Set(oauthConfig)
	_ = testKVStore.Set(context.Background(), "auth:pkce:disabled-provider-state-2", "disabled-provider-state-2", 10*time.Minute)
	oauthConfiguredDisabledCallbackRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/disabled_provider/callback?code=123&state=disabled-provider-state-2", nil)
	oauthConfiguredDisabledCallbackResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(oauthConfiguredDisabledCallbackResponseRecorder, oauthConfiguredDisabledCallbackRequest)
	if oauthConfiguredDisabledCallbackResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on configured disabled oauth provider callback, got: %d", oauthConfiguredDisabledCallbackResponseRecorder.Code)
	}

	// 9. State validation branches
	missingStateGetRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=any-code", nil)
	missingStateGetResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(missingStateGetResponseRecorder, missingStateGetRequest)
	if missingStateGetResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(missingStateGetResponseRecorder.Body.String(), "invalid_request") {
		t.Fatalf("expected 400 invalid_request on missing state GET, got: %d (%s)", missingStateGetResponseRecorder.Code, missingStateGetResponseRecorder.Body.String())
	}

	missingStateFormRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/google/callback", strings.NewReader("code=any-code"))
	missingStateFormRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missingStateFormResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(missingStateFormResponseRecorder, missingStateFormRequest)
	if missingStateFormResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing state form POST, got: %d", missingStateFormResponseRecorder.Code)
	}

	missingStateJSONRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/google/callback", strings.NewReader(`{"code":"any-code"}`))
	missingStateJSONResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(missingStateJSONResponseRecorder, missingStateJSONRequest)
	if missingStateJSONResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing state JSON POST, got: %d", missingStateJSONResponseRecorder.Code)
	}

	fakeStateGetRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=any-code&state=fake-nonexistent-state", nil)
	fakeStateGetResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(fakeStateGetResponseRecorder, fakeStateGetRequest)
	if fakeStateGetResponseRecorder.Code != http.StatusForbidden || !strings.Contains(fakeStateGetResponseRecorder.Body.String(), "access_denied") {
		t.Fatalf("expected 403 access_denied on fake state GET, got: %d", fakeStateGetResponseRecorder.Code)
	}

	_ = testKVStore.Set(context.Background(), "auth:pkce:mismatched-state", "different-stored-value", 10*time.Minute)
	mismatchedStateRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=any-code&state=mismatched-state", nil)
	mismatchedStateResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(mismatchedStateResponseRecorder, mismatchedStateRequest)
	if mismatchedStateResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on mismatched state value, got: %d", mismatchedStateResponseRecorder.Code)
	}

	baseHandler.SetKVStore(nil)
	nilKVStoreStateRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=any-code&state=valid-looking-state", nil)
	nilKVStoreStateResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(nilKVStoreStateResponseRecorder, nilKVStoreStateRequest)
	if nilKVStoreStateResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 with nil kvstore, got: %d", nilKVStoreStateResponseRecorder.Code)
	}
	baseHandler.SetKVStore(testKVStore)

	_ = testKVStore.Set(context.Background(), "auth:pkce:missing-code-state", "missing-code-state", 10*time.Minute)
	missingCodeStateRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?state=missing-code-state", nil)
	missingCodeStateResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(missingCodeStateResponseRecorder, missingCodeStateRequest)
	if missingCodeStateResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(missingCodeStateResponseRecorder.Body.String(), "invalid_request") {
		t.Fatalf("expected 400 invalid_request on missing code with valid state, got: %d (%s)", missingCodeStateResponseRecorder.Code, missingCodeStateResponseRecorder.Body.String())
	}

	// 10. UserInfo
	invalidTokenUserInfoRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/userinfo", nil)
	invalidTokenUserInfoRequest.Header.Set("Authorization", "Bearer invalid-token")
	invalidTokenUserInfoResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthUserInfo(invalidTokenUserInfoResponseRecorder, invalidTokenUserInfoRequest)
	if invalidTokenUserInfoResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid token userinfo, got: %d", invalidTokenUserInfoResponseRecorder.Code)
	}

	noBearerUserInfoRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/userinfo", nil)
	noBearerUserInfoRequest.Header.Set("Authorization", "Basic credentials")
	noBearerUserInfoResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthUserInfo(noBearerUserInfoResponseRecorder, noBearerUserInfoRequest)
	if noBearerUserInfoResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on non-bearer userinfo, got: %d", noBearerUserInfoResponseRecorder.Code)
	}

	testUserUUID := "018f2234-5678-789a-bcde-f0123456789a"
	validToken, err := baseHandler.jwtSigner.GenerateAccessToken(core.JWTClaims{
		Subject: testUserUUID,
		Email:   "test@example.com",
		Role:    "authenticated",
	}, 900)
	if err != nil {
		t.Fatalf("failed to sign access token: %v", err)
	}
	bearerHeader := "Bearer " + validToken

	validTokenUserInfoCtx := core.WithAuthContext(context.Background(), core.AuthContext{
		UserID: testUserUUID,
		JWT: core.JWTClaims{
			Subject: testUserUUID,
			Email:   "test@example.com",
			Role:    "authenticated",
		},
	})
	validTokenUserInfoRequest := httptest.NewRequestWithContext(validTokenUserInfoCtx, http.MethodGet, "/api/v1/auth/oauth/userinfo", nil)
	validTokenUserInfoRequest.Header.Set("Authorization", bearerHeader)
	validTokenUserInfoResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthUserInfo(validTokenUserInfoResponseRecorder, validTokenUserInfoRequest)
	if validTokenUserInfoResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on valid token userinfo with nil pool, got: %d", validTokenUserInfoResponseRecorder.Code)
	}

	// 11. OAuth exchange mock success on nil pool -> 500
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
				Body:       io.NopCloser(strings.NewReader(`{"sub":"oauth-user-999","email":"oauth999@gmail.com","name":"OAuth User"}`)),
			}, nil
		},
	})
	defer oauth.SetHTTPClient(nil)

	_ = testKVStore.Set(context.Background(), "auth:pkce:mock_state", "mock_state", 10*time.Minute)
	nilDBOAuthCallbackRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=mock_code&state=mock_state", nil)
	nilDBOAuthCallbackResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(nilDBOAuthCallbackResponseRecorder, nilDBOAuthCallbackRequest)
	if nilDBOAuthCallbackResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on oauth callback with nil pool, got: %d", nilDBOAuthCallbackResponseRecorder.Code)
	}

	// 12. Authorize with anonymous user linking
	anonToken, _ := baseHandler.jwtSigner.GenerateAccessToken(core.JWTClaims{
		Subject:     "018f2234-5678-789a-bcde-f0123456789b",
		Role:        "authenticated",
		IsAnonymous: true,
	}, 900)
	anonAuthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/authorize", nil)
	anonAuthRequest.SetPathValue("provider", "google")
	anonAuthRequest.Header.Set("Authorization", "Bearer "+anonToken)
	anonAuthResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthAuthorize(anonAuthResponseRecorder, anonAuthRequest)
	if anonAuthResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 on authorize with anonymous user, got: %d", anonAuthResponseRecorder.Code)
	}

	// 13. OAuth callback with mismatched state ID in payload
	mismatchedPayloadJSON, _ := json.Marshal(OAuthStatePayload{
		StateID:  "different_state_id",
		Provider: "google",
	})
	_ = testKVStore.Set(context.Background(), "auth:pkce:mismatch_state", string(mismatchedPayloadJSON), 10*time.Minute)
	mismatchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback?code=mock_code&state=mismatch_state", nil)
	mismatchRequest.SetPathValue("provider", "google")
	mismatchResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(mismatchResponseRecorder, mismatchRequest)
	if mismatchResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on mismatched state ID in payload, got: %d", mismatchResponseRecorder.Code)
	}

	// 14. CompleteOAuthFlow with OIDCStateID
	oidcStateID := "oidc_state_test_123"
	oidcPayloadJSON, _ := json.Marshal(OIDCAuthorizationStatePayload{
		ClientID:      "client-spa-1",
		RedirectURI:   "https://app.client.com/callback",
		Scope:         "openid profile",
		CodeChallenge: "challenge123",
		ClientState:   "client_state_abc",
	})
	_ = testKVStore.Set(context.Background(), "auth:oidc:state:"+oidcStateID, string(oidcPayloadJSON), 10*time.Minute)

	oAuthStatePayload := OAuthStatePayload{
		StateID:     "oauth_state_123",
		Provider:    "google",
		OIDCStateID: oidcStateID,
	}
	userRecord := UserRecord{
		ID:   "018f2234-5678-789a-bcde-f0123456789c",
		Role: "authenticated",
	}

	// Zero refresh expiry fallback
	baseHandler.configManager.rwMutex.Lock()
	baseHandler.configManager.config.Sessions.RefreshTokenExpirySeconds = 0
	baseHandler.configManager.rwMutex.Unlock()

	completeOIDCRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/callback", nil)
	completeOIDCRequest.Header.Set("X-Forwarded-Proto", "https")
	completeOIDCResponseRecorder := httptest.NewRecorder()
	baseHandler.CompleteOAuthFlow(completeOIDCResponseRecorder, completeOIDCRequest, userRecord, oAuthStatePayload, true)
	if completeOIDCResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect from CompleteOAuthFlow with OIDCStateID, got: %d", completeOIDCResponseRecorder.Code)
	}
	loc := completeOIDCResponseRecorder.Header().Get("Location")
	if !strings.Contains(loc, "code=") || !strings.Contains(loc, "state=client_state_abc") {
		t.Fatalf("expected code and client state in redirect location, got: %s", loc)
	}

	// Provider in form body fallback
	formBodyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/callback", strings.NewReader("provider=google&code=mock_code&state=fake_state"))
	formBodyRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	formBodyResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthCallback(formBodyResponseRecorder, formBodyRequest)
	if formBodyResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on fake state with provider in form body, got: %d", formBodyResponseRecorder.Code)
	}
}

func TestAuthHandlerOAuthAnonymousAuthorizeUnit(t *testing.T) {
	cryptoKeyManager, _ := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	configManager := NewConfigManager(nil, cryptoKeyManager)
	baseHandler := NewBaseHandler(nil, configManager, cryptoKeyManager)
	testKVStore := newInMemoryKVStore()
	baseHandler.SetKVStore(testKVStore)

	authConfig := configManager.Get()
	authConfig.OAuthProviders = map[string]OAuthProviderConfig{
		"google": {
			Enabled:      true,
			ClientID:     "google-client-id",
			ClientSecret: "google-secret",
		},
	}
	configManager.Set(authConfig)

	anonToken, _ := baseHandler.jwtSigner.GenerateAccessToken(core.JWTClaims{
		Subject:     "018f2234-5678-789a-bcde-f0123456789a",
		Role:        "authenticated",
		IsAnonymous: true,
	}, 900)
	anonOAuthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/google/authorize?state=custom-state-1", nil)
	anonOAuthRequest.SetPathValue("provider", "google")
	anonOAuthRequest.Header.Set("Authorization", "Bearer "+anonToken)
	anonOAuthResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleOAuthAuthorize(anonOAuthResponseRecorder, anonOAuthRequest)
	if anonOAuthResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on anon oauth authorize, got: %d", anonOAuthResponseRecorder.Code)
	}
}
