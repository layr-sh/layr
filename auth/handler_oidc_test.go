package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/go-fuego/fuego"
	"layr.sh/core"
)

func TestAuthOIDCHandlerUnit(t *testing.T) {
	layrConfig := core.DefaultConfig()
	layrConfig.Project.Name = "TestLayrApp"
	core.SetLoadedConfig(layrConfig)
	t.Cleanup(core.UnloadConfig)

	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	handler := NewHandler(nil, configManager, cryptoKeyManager)
	testKVStore := newInMemoryKVStore()
	handler.SetKVStore(testKVStore)

	// 1. OIDC Discovery
	discoveryRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/openid-configuration", nil)
	discoveryResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCDiscovery(discoveryResponseRecorder, discoveryRequest)
	if discoveryResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from OIDC discovery, got: %d", discoveryResponseRecorder.Code)
	}

	// 2. Custom host discovery via config
	customConfig := core.DefaultConfig()
	customConfig.Server.BaseURL = "http://layr.local:8080"
	core.SetLoadedConfig(customConfig)
	customResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCDiscovery(customResponseRecorder, discoveryRequest)
	if customResponseRecorder.Code != http.StatusOK || !strings.Contains(customResponseRecorder.Body.String(), "http://layr.local:8080") {
		t.Fatalf("expected custom host in OIDC discovery, got: %s", customResponseRecorder.Body.String())
	}
	core.SetLoadedConfig(layrConfig)

	// 3. JWKS
	jwksRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/jwks.json", nil)
	jwksResponseRecorder := httptest.NewRecorder()
	handler.handleJWKS(jwksResponseRecorder, jwksRequest)
	if jwksResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from JWKS, got: %d", jwksResponseRecorder.Code)
	}

	// 4. Authorization Endpoint Validation (GET /api/v1/auth/oauth/authorize)
	activeConfig := configManager.Get()
	activeConfig.OIDC.Enabled = true
	activeConfig.OIDC.SignInUI.CustomCSS = ".custom-class { color: red; }"
	activeConfig.OIDC.Clients = []OIDCClientConfig{
		{
			Name:         "Demo App",
			ClientID:     "client-app-1",
			ClientSecret: "secret-123",
			RedirectURIs: []string{"https://demo.app/callback"},
			Public:       false,
		},
		{
			Name:         "SPA App",
			ClientID:     "client-spa-1",
			RedirectURIs: []string{"https://spa.app/callback"},
			Public:       true,
		},
	}
	configManager.Set(activeConfig)

	// Missing client_id
	missingClientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/authorize", nil)
	missingClientResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorize(missingClientResponseRecorder, missingClientRequest)
	if missingClientResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on missing client_id, got: %d", missingClientResponseRecorder.Code)
	}

	// Unknown client_id
	unknownClientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/authorize?client_id=nonexistent", nil)
	unknownClientResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorize(unknownClientResponseRecorder, unknownClientRequest)
	if unknownClientResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on unknown client_id, got: %d", unknownClientResponseRecorder.Code)
	}

	// Missing redirect_uri
	missingRedirectRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/authorize?client_id=client-app-1", nil)
	missingRedirectResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorize(missingRedirectResponseRecorder, missingRedirectRequest)
	if missingRedirectResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on missing redirect_uri, got: %d", missingRedirectResponseRecorder.Code)
	}

	// Unauthorized redirect_uri
	unauthorizedRedirectRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/authorize?client_id=client-app-1&redirect_uri=https://evil.com/callback", nil)
	unauthorizedRedirectResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorize(unauthorizedRedirectResponseRecorder, unauthorizedRedirectRequest)
	if unauthorizedRedirectResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on unauthorized redirect_uri, got: %d", unauthorizedRedirectResponseRecorder.Code)
	}

	// Unsupported response_type (must redirect to redirect_uri with error)
	unsupportedResponseTypeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/authorize?client_id=client-app-1&redirect_uri=https://demo.app/callback&response_type=token&state=teststate", nil)
	unsupportedResponseTypeResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorize(unsupportedResponseTypeResponseRecorder, unsupportedResponseTypeRequest)
	if unsupportedResponseTypeResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on unsupported response_type, got: %d", unsupportedResponseTypeResponseRecorder.Code)
	}
	location := unsupportedResponseTypeResponseRecorder.Header().Get("Location")
	if !strings.Contains(location, "error=unsupported_response_type") || !strings.Contains(location, "state=teststate") {
		t.Fatalf("expected error=unsupported_response_type in Location header, got: %s", location)
	}

	// Missing PKCE / invalid method
	missingPKCERequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/authorize?client_id=client-app-1&redirect_uri=https://demo.app/callback&response_type=code&state=teststate", nil)
	missingPKCEResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorize(missingPKCEResponseRecorder, missingPKCERequest)
	if missingPKCEResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on missing PKCE, got: %d", missingPKCEResponseRecorder.Code)
	}
	location = missingPKCEResponseRecorder.Header().Get("Location")
	if !strings.Contains(location, "error=invalid_request") {
		t.Fatalf("expected error=invalid_request in Location header, got: %s", location)
	}

	// Valid unauthenticated request -> Renders Hosted Universal Sign-In Page
	validAuthorizeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/authorize?client_id=client-app-1&redirect_uri=https://demo.app/callback&response_type=code&state=clientstate123&code_challenge=abc123challenge&code_challenge_method=S256", nil)
	validAuthorizeResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorize(validAuthorizeResponseRecorder, validAuthorizeRequest)
	if validAuthorizeResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on valid authorize GET, got: %d (%s)", validAuthorizeResponseRecorder.Code, validAuthorizeResponseRecorder.Body.String())
	}
	htmlBody := validAuthorizeResponseRecorder.Body.String()
	if !strings.Contains(htmlBody, "Sign in to Demo App · TestLayrApp") {
		t.Fatalf("expected title with client name and project name in HTML, got: %s", htmlBody)
	}
	if !strings.Contains(htmlBody, "Demo App") {
		t.Fatalf("expected client name Demo App in HTML, got: %s", htmlBody)
	}
	if !strings.Contains(htmlBody, "Secured by TestLayrApp") {
		t.Fatalf("expected brand footer Secured by TestLayrApp in HTML, got: %s", htmlBody)
	}
	if !strings.Contains(htmlBody, ".custom-class { color: red; }") {
		t.Fatalf("expected custom CSS injected into HTML, got: %s", htmlBody)
	}

	// Test Disabled OIDC.Enabled -> 403 Forbidden on all OIDC endpoints
	disabledOIDCConfig := configManager.Get()
	disabledOIDCConfig.OIDC.Enabled = false
	configManager.Set(disabledOIDCConfig)

	disabledAuthorizeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/authorize?client_id=client-app-1&redirect_uri=https://demo.app/callback&response_type=code&state=clientstate123&code_challenge=abc123challenge&code_challenge_method=S256", nil)
	disabledAuthorizeResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorize(disabledAuthorizeResponseRecorder, disabledAuthorizeRequest)
	if disabledAuthorizeResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on disabled OIDC authorize, got: %d", disabledAuthorizeResponseRecorder.Code)
	}

	disabledTokenRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&code=anycode"))
	disabledTokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	disabledTokenResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(disabledTokenResponseRecorder, disabledTokenRequest)
	if disabledTokenResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on disabled OIDC token, got: %d", disabledTokenResponseRecorder.Code)
	}

	// Re-enable OIDC for subsequent tests
	enabledConfig := configManager.Get()
	enabledConfig.OIDC.Enabled = true
	configManager.Set(enabledConfig)

	// 5. Authorization Submit (POST /api/v1/auth/oauth/authorize)
	invalidStateRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader("state=nonexistent-state-id&email=test@example.com&password=pass"))
	invalidStateRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalidStateResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(invalidStateResponseRecorder, invalidStateRequest)
	if invalidStateResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on invalid state submission, got: %d", invalidStateResponseRecorder.Code)
	}

	validStateID := uuid.NewV7().String()
	oidcAuthorizationStatePayload := OIDCAuthorizationStatePayload{
		StateID:             validStateID,
		ClientID:            "client-app-1",
		RedirectURI:         "https://demo.app/callback",
		ClientState:         "clientstate123",
		CodeChallenge:       "abc123challenge",
		CodeChallengeMethod: "S256",
		CreatedAt:           time.Now().UTC(),
	}
	stateBytes, _ := json.Marshal(oidcAuthorizationStatePayload)
	_ = testKVStore.Set(context.Background(), "layr:auth:oidc:state:"+validStateID, string(stateBytes), 10*time.Minute)

	emptyCredsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader("state="+validStateID+"&email=&password="))
	emptyCredsRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyCredsResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(emptyCredsResponseRecorder, emptyCredsRequest)
	if emptyCredsResponseRecorder.Code != http.StatusOK || !strings.Contains(emptyCredsResponseRecorder.Body.String(), "Email and password are required") {
		t.Fatalf("expected 200 re-render with validation message, got: %d (%s)", emptyCredsResponseRecorder.Code, emptyCredsResponseRecorder.Body.String())
	}

	// 6. Token Endpoint Validation (POST /api/v1/auth/oauth/token)
	badGrantRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=implicit"))
	badGrantRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badGrantResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(badGrantResponseRecorder, badGrantRequest)
	if badGrantResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on unsupported grant_type, got: %d", badGrantResponseRecorder.Code)
	}

	missingCredsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&code=testcode"))
	missingCredsRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missingCredsResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(missingCredsResponseRecorder, missingCredsRequest)
	if missingCredsResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized on missing client credentials, got: %d", missingCredsResponseRecorder.Code)
	}

	badSecretRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-app-1&client_secret=wrongsecret&code=testcode"))
	badSecretRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badSecretResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(badSecretResponseRecorder, badSecretRequest)
	if badSecretResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized on invalid client_secret, got: %d", badSecretResponseRecorder.Code)
	}

	missingCodeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-spa-1&code=nonexistent-code&code_verifier=xyz"))
	missingCodeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missingCodeResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(missingCodeResponseRecorder, missingCodeRequest)
	if missingCodeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on missing authorization code, got: %d", missingCodeResponseRecorder.Code)
	}

	// 7. Userinfo Endpoint Validation (GET /api/v1/auth/oauth/userinfo)
	missingBearerRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/userinfo", nil)
	missingBearerResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCUserInfo(missingBearerResponseRecorder, missingBearerRequest)
	if missingBearerResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing Bearer token, got: %d", missingBearerResponseRecorder.Code)
	}

	invalidBearerRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/userinfo", nil)
	invalidBearerRequest.Header.Set("Authorization", "Bearer invalid.token.structure")
	invalidBearerResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCUserInfo(invalidBearerResponseRecorder, invalidBearerRequest)
	if invalidBearerResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid Bearer token, got: %d", invalidBearerResponseRecorder.Code)
	}

	// 8. Sign-Out Endpoint Validation (GET & POST /api/v1/auth/oauth/sign-out)
	signOutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/sign-out", nil)
	signOutResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCSignOut(signOutResponseRecorder, signOutRequest)
	if signOutResponseRecorder.Code != http.StatusOK || !strings.Contains(signOutResponseRecorder.Body.String(), "Signed Out") {
		t.Fatalf("expected 200 OK HTML on sign-out without redirect, got: %d", signOutResponseRecorder.Code)
	}

	signOutRedirectRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/oauth/sign-out?post_sign_out_redirect_uri="+url.QueryEscape("https://demo.app/goodbye"), nil)
	signOutRedirectResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCSignOut(signOutRedirectResponseRecorder, signOutRedirectRequest)
	if signOutRedirectResponseRecorder.Code != http.StatusFound || signOutRedirectResponseRecorder.Header().Get("Location") != "https://demo.app/goodbye" {
		t.Fatalf("expected 302 Found redirect to post_sign_out_redirect_uri, got: %d, Location: %s", signOutRedirectResponseRecorder.Code, signOutRedirectResponseRecorder.Header().Get("Location"))
	}
}

func TestAuthOIDCEdgeCasesUnit(t *testing.T) {
	layrConfig := core.DefaultConfig()
	layrConfig.Project.Name = "TestLayrApp"
	core.SetLoadedConfig(layrConfig)
	t.Cleanup(core.UnloadConfig)

	cryptoKeyManager, err := core.NewCryptoKeyManager(testMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	baseConfig := configManager.Get()
	baseConfig.OIDC.Enabled = true
	encryptedSecret, _ := cryptoKeyManager.EncryptField([]byte("secret-confidential-123"))
	baseConfig.OIDC.Clients = []OIDCClientConfig{
		{
			Name:         "SPA Client",
			ClientID:     "client-spa-1",
			RedirectURIs: []string{"https://demo.app/callback"},
			Public:       true,
		},
		{
			Name:         "Confidential Client",
			ClientID:     "client-confidential-1",
			ClientSecret: encryptedSecret,
			RedirectURIs: []string{"https://demo.app/callback"},
			Public:       false,
		},
	}
	configManager.Set(baseConfig)

	handler := NewHandler(nil, configManager, cryptoKeyManager)
	testKVStore := newInMemoryKVStore()
	handler.SetKVStore(testKVStore)

	// 1. handleOIDCAuthorizeSubmit when OIDC disabled -> 403
	disabledConfig := configManager.Get()
	disabledConfig.OIDC.Enabled = false
	configManager.Set(disabledConfig)

	submitRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/authorize", nil)
	submitResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(submitResponseRecorder, submitRequest)
	if submitResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when OIDC disabled on submit, got: %d", submitResponseRecorder.Code)
	}

	// 2. handleOIDCUserInfo when OIDC disabled -> 403
	userInfoResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCUserInfo(userInfoResponseRecorder, submitRequest)
	if userInfoResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when OIDC disabled on userinfo, got: %d", userInfoResponseRecorder.Code)
	}

	// 3. handleOIDCSignOut when OIDC disabled -> 403
	signOutResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCSignOut(signOutResponseRecorder, submitRequest)
	if signOutResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when OIDC disabled on signout, got: %d", signOutResponseRecorder.Code)
	}

	// Re-enable OIDC
	disabledConfig.OIDC.Enabled = true
	configManager.Set(disabledConfig)

	// 4. handleOIDCAuthorizeSubmit invalid form data -> 400
	badFormRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader("key=%zz"))
	badFormRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badFormResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(badFormResponseRecorder, badFormRequest)
	if badFormResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad form data, got: %d", badFormResponseRecorder.Code)
	}

	// 5. handleOIDCAuthorizeSubmit missing state -> 400
	emptyFormRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader("state="))
	emptyFormRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyFormResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(emptyFormResponseRecorder, emptyFormRequest)
	if emptyFormResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty state, got: %d", emptyFormResponseRecorder.Code)
	}

	// 6. handleOIDCAuthorizeSubmit nonexistent state in kvStore -> 400
	missingStateRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader("state=nonexistent_state"))
	missingStateRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missingStateResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(missingStateResponseRecorder, missingStateRequest)
	if missingStateResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing state in kvStore, got: %d", missingStateResponseRecorder.Code)
	}

	// 7. handleOIDCAuthorizeSubmit corrupt state JSON in kvStore -> 400
	_ = testKVStore.Set(context.Background(), "layr:auth:oidc:state:corrupt_state", "{invalid_json", 5*time.Minute)
	corruptStateRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader("state=corrupt_state"))
	corruptStateRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	corruptStateResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(corruptStateResponseRecorder, corruptStateRequest)
	if corruptStateResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on corrupt state JSON, got: %d", corruptStateResponseRecorder.Code)
	}

	// 8. handleOIDCAuthorizeSubmit empty email/password -> render sign in page error
	validStateID := "valid_state_empty_creds"
	validStatePayload, _ := json.Marshal(OIDCAuthorizationStatePayload{
		ClientID:    "client-spa-1",
		RedirectURI: "https://demo.app/callback",
	})
	_ = testKVStore.Set(context.Background(), "layr:auth:oidc:state:"+validStateID, string(validStatePayload), 5*time.Minute)
	emptyCredsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader("state="+validStateID+"&email=&password="))
	emptyCredsRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyCredsResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(emptyCredsResponseRecorder, emptyCredsRequest)
	if !strings.Contains(emptyCredsResponseRecorder.Body.String(), "Email and password are required") {
		t.Fatalf("expected Email and password required in render, got: %s", emptyCredsResponseRecorder.Body.String())
	}

	// 9. handleOIDCToken delegation branches (empty grant_type and provider param)
	firstDelegatedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(""))
	firstDelegatedRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	firstDelegatedResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(firstDelegatedResponseRecorder, firstDelegatedRequest)
	if firstDelegatedResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on delegated callback without provider/code, got: %d", firstDelegatedResponseRecorder.Code)
	}

	secondDelegatedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&provider=unknown"))
	secondDelegatedRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	secondDelegatedResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(secondDelegatedResponseRecorder, secondDelegatedRequest)
	if secondDelegatedResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on delegated callback with provider, got: %d", secondDelegatedResponseRecorder.Code)
	}

	// 10. handleOIDCTokenAuthorizationCode client not found -> 401
	unknownClientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=unknown_client"))
	unknownClientRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unknownClientResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(unknownClientResponseRecorder, unknownClientRequest)
	if unknownClientResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unknown client ID, got: %d", unknownClientResponseRecorder.Code)
	}

	// 11. handleOIDCTokenAuthorizationCode corrupt code JSON in kvStore -> 400
	_ = testKVStore.Set(context.Background(), "layr:auth:code:corrupt_code_123", "{invalid_json", 5*time.Minute)
	corruptCodeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-spa-1&code=corrupt_code_123"))
	corruptCodeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	corruptCodeResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(corruptCodeResponseRecorder, corruptCodeRequest)
	if corruptCodeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on corrupt code JSON, got: %d", corruptCodeResponseRecorder.Code)
	}

	// 12. handleOIDCTokenAuthorizationCode client mismatch -> 400
	mismatchedClientPayload, _ := json.Marshal(OIDCAuthorizationCodePayload{
		ClientID: "other-client-id",
	})
	_ = testKVStore.Set(context.Background(), "layr:auth:code:mismatch_client_code", string(mismatchedClientPayload), 5*time.Minute)
	mismatchClientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-spa-1&code=mismatch_client_code"))
	mismatchClientRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mismatchClientResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(mismatchClientResponseRecorder, mismatchClientRequest)
	if mismatchClientResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on client mismatch, got: %d", mismatchClientResponseRecorder.Code)
	}

	// 13. handleOIDCTokenAuthorizationCode redirect_uri mismatch -> 400
	mismatchedRedirectPayload, _ := json.Marshal(OIDCAuthorizationCodePayload{
		ClientID:    "client-spa-1",
		RedirectURI: "https://demo.app/expected_callback",
	})
	_ = testKVStore.Set(context.Background(), "layr:auth:code:mismatch_redirect_code", string(mismatchedRedirectPayload), 5*time.Minute)
	mismatchRedirectRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-spa-1&code=mismatch_redirect_code&redirect_uri=https://demo.app/other_callback"))
	mismatchRedirectRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mismatchRedirectResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(mismatchRedirectResponseRecorder, mismatchRedirectRequest)
	if mismatchRedirectResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on redirect_uri mismatch, got: %d", mismatchRedirectResponseRecorder.Code)
	}

	// 14. handleOIDCTokenAuthorizationCode PKCE verification failure -> 400
	badPKCEPayload, _ := json.Marshal(OIDCAuthorizationCodePayload{
		ClientID:      "client-spa-1",
		CodeChallenge: "expected_pkce_challenge_hash",
	})
	_ = testKVStore.Set(context.Background(), "layr:auth:code:bad_pkce_code", string(badPKCEPayload), 5*time.Minute)
	badPKCERequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-spa-1&code=bad_pkce_code&code_verifier=invalid_verifier"))
	badPKCERequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badPKCEResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(badPKCEResponseRecorder, badPKCERequest)
	if badPKCEResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on PKCE failure, got: %d", badPKCEResponseRecorder.Code)
	}

	// 15. handleOIDCTokenRefreshToken confidential client invalid secret -> 401
	badConfidentialSecretRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=refresh_token&client_id=client-confidential-1&client_secret=wrong_secret"))
	badConfidentialSecretRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badConfidentialSecretResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(badConfidentialSecretResponseRecorder, badConfidentialSecretRequest)
	if badConfidentialSecretResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on bad confidential client secret, got: %d", badConfidentialSecretResponseRecorder.Code)
	}

	// 16. handleOIDCTokenRefreshToken missing refresh_token -> 400
	missingRefreshRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=refresh_token&client_id=client-spa-1&refresh_token="))
	missingRefreshRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missingRefreshResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(missingRefreshResponseRecorder, missingRefreshRequest)
	if missingRefreshResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing refresh token, got: %d", missingRefreshResponseRecorder.Code)
	}

	// 17. redirectError with invalid URI -> 400
	redirectResponseRecorder := httptest.NewRecorder()
	redirectRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	redirectError(redirectResponseRecorder, redirectRequest, "://invalid-uri", "test_slug", "test_description", "state_123")
	if redirectResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on redirectError with invalid URI, got: %d", redirectResponseRecorder.Code)
	}

	// 18. renderOIDCSignInPage with empty ProjectName fallback and enabled OAuth provider buttons
	emptyProjectConfig := core.DefaultConfig()
	emptyProjectConfig.Project.Name = ""
	core.SetLoadedConfig(emptyProjectConfig)

	oauthButtonsConfig := configManager.Get()
	oauthButtonsConfig.OAuthProviders["google"] = OAuthProviderConfig{Enabled: true}
	oauthButtonsConfig.OAuthProviders["github"] = OAuthProviderConfig{Enabled: true}
	configManager.Set(oauthButtonsConfig)

	renderResponseRecorder := httptest.NewRecorder()
	handler.renderOIDCSignInPage(renderResponseRecorder, "state_render", &OIDCClientConfig{Name: "Client App"}, "Some error")
	renderHTML := renderResponseRecorder.Body.String()
	if !strings.Contains(renderHTML, "Layr") || !strings.Contains(renderHTML, "Google") || !strings.Contains(renderHTML, "Github") {
		t.Fatalf("expected Layr and provider buttons in render, got: %s", renderHTML)
	}

	// 19. handleOIDCToken missing code -> 400 invalid_grant
	emptyCodeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-spa-1&code="))
	emptyCodeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyCodeResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(emptyCodeResponseRecorder, emptyCodeRequest)
	if emptyCodeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing code in token exchange, got: %d", emptyCodeResponseRecorder.Code)
	}

	// 20. RegisterOIDCRoutes
	fuegoEngine := fuego.NewServer()
	router := core.NewRouter(fuegoEngine)
	handler.RegisterOIDCRoutes(router)
}

func TestAuthOIDCClientCredentialsUnit(t *testing.T) {
	testCtx := context.Background()

	// 1. Missing secret
	handler := &Handler{}
	missingSecretRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=client_credentials&client_id=sa-1"))
	missingSecretRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missingSecretResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(missingSecretResponseRecorder, missingSecretRequest)
	if missingSecretResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing secret, got: %d", missingSecretResponseRecorder.Code)
	}

	// 2. Nil ServiceAccountManager
	nilManagerRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=client_credentials&client_id=sa-1&client_secret=secret123"))
	nilManagerRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	nilManagerResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(nilManagerResponseRecorder, nilManagerRequest)
	if nilManagerResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on nil serviceAccountManager, got: %d", nilManagerResponseRecorder.Code)
	}

	// 3. ServiceAccountManager Authenticate error (e.g. nil database pool)
	serviceAccountManager := core.NewServiceAccountManager(nil)
	managerHandler := &Handler{serviceAccountManager: serviceAccountManager}
	authErrorRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=client_credentials&client_id=sa-1&client_secret=secret123"))
	authErrorRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	authErrorResponseRecorder := httptest.NewRecorder()
	managerHandler.handleOIDCToken(authErrorResponseRecorder, authErrorRequest)
	if authErrorResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on auth error, got: %d", authErrorResponseRecorder.Code)
	}

	// 4. JSON body parsing with Content-Type application/json
	jsonRequestBody := `{"grant_type":"client_credentials","client_id":"sa-1","client_secret":"sec_123","scope":"data:read"}`
	jsonRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(jsonRequestBody))
	jsonRequest.Header.Set("Content-Type", "application/json")
	jsonResponseRecorder := httptest.NewRecorder()
	managerHandler.handleOIDCToken(jsonResponseRecorder, jsonRequest)
	if jsonResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from json request through auth error, got: %d", jsonResponseRecorder.Code)
	}

	// 5. JSON body parsing without Content-Type header
	untypedJSONRequestBody := `{"grant_type":"client_credentials","client_id":"sa-1","client_secret":"sec_123"}`
	untypedJSONRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(untypedJSONRequestBody))
	untypedJSONResponseRecorder := httptest.NewRecorder()
	managerHandler.handleOIDCToken(untypedJSONResponseRecorder, untypedJSONRequest)
	if untypedJSONResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from untyped json request, got: %d", untypedJSONResponseRecorder.Code)
	}

	// 6. Basic Auth header extraction
	basicAuthRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=client_credentials&scope=data:read"))
	basicAuthRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	basicAuthRequest.SetBasicAuth("sa-basic", "secret-basic")
	basicAuthResponseRecorder := httptest.NewRecorder()
	managerHandler.handleOIDCToken(basicAuthResponseRecorder, basicAuthRequest)
	if basicAuthResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from basic auth request through auth error, got: %d", basicAuthResponseRecorder.Code)
	}

	// 7. handleOAuthToken delegation
	delegatedRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader("grant_type=client_credentials&client_id=sa-1"))
	delegatedRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	delegatedResponseRecorder := httptest.NewRecorder()
	handler.handleOAuthToken(delegatedResponseRecorder, delegatedRequest)
	if delegatedResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from handleOAuthToken, got: %d", delegatedResponseRecorder.Code)
	}
}
