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
	baseHandler := NewBaseHandler(nil, configManager, cryptoKeyManager)
	testKVStore := newInMemoryKVStore()
	baseHandler.SetKVStore(testKVStore)

	// 1. OIDC Discovery
	discoveryRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/openid-configuration", nil)
	discoveryResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetOIDCDiscovery(discoveryResponseRecorder, discoveryRequest)
	if discoveryResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from OIDC discovery, got: %d", discoveryResponseRecorder.Code)
	}

	// 2. Custom host discovery via config
	customConfig := core.DefaultConfig()
	customConfig.Server.BaseURL = "http://layr.local:8080"
	core.SetLoadedConfig(customConfig)
	customResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetOIDCDiscovery(customResponseRecorder, discoveryRequest)
	if customResponseRecorder.Code != http.StatusOK || !strings.Contains(customResponseRecorder.Body.String(), "http://layr.local:8080") {
		t.Fatalf("expected custom host in OIDC discovery, got: %s", customResponseRecorder.Body.String())
	}
	core.SetLoadedConfig(layrConfig)

	// 3. JWKS
	jwksRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/jwks.json", nil)
	jwksResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetJWKS(jwksResponseRecorder, jwksRequest)
	if jwksResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from JWKS, got: %d", jwksResponseRecorder.Code)
	}

	// 4. Authorization Endpoint Validation (GET /v1/auth/oauth/authorize)
	activeConfig := configManager.Get()
	activeConfig.OIDC.Enabled = true
	activeConfig.OIDC.UI.CustomCSS = ".custom-class { color: red; }"
	activeConfig.OIDC.UI.TermsOfServiceURL = "https://demo.app/terms"
	activeConfig.OIDC.UI.PrivacyPolicyURL = "https://demo.app/privacy"
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
	missingClientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/oauth/authorize", nil)
	missingClientResponseRecorder := httptest.NewRecorder()
	baseHandler.handleAuthorizeOIDC(missingClientResponseRecorder, missingClientRequest)
	if missingClientResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on missing client_id, got: %d", missingClientResponseRecorder.Code)
	}

	// Unknown client_id
	unknownClientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/oauth/authorize?client_id=nonexistent", nil)
	unknownClientResponseRecorder := httptest.NewRecorder()
	baseHandler.handleAuthorizeOIDC(unknownClientResponseRecorder, unknownClientRequest)
	if unknownClientResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on unknown client_id, got: %d", unknownClientResponseRecorder.Code)
	}

	// Missing redirect_uri
	missingRedirectRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/oauth/authorize?client_id=client-app-1", nil)
	missingRedirectResponseRecorder := httptest.NewRecorder()
	baseHandler.handleAuthorizeOIDC(missingRedirectResponseRecorder, missingRedirectRequest)
	if missingRedirectResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on missing redirect_uri, got: %d", missingRedirectResponseRecorder.Code)
	}

	// Unauthorized redirect_uri
	unauthorizedRedirectRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/oauth/authorize?client_id=client-app-1&redirect_uri=https://evil.com/callback", nil)
	unauthorizedRedirectResponseRecorder := httptest.NewRecorder()
	baseHandler.handleAuthorizeOIDC(unauthorizedRedirectResponseRecorder, unauthorizedRedirectRequest)
	if unauthorizedRedirectResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on unauthorized redirect_uri, got: %d", unauthorizedRedirectResponseRecorder.Code)
	}

	// Unsupported response_type (must redirect to redirect_uri with error)
	unsupportedResponseTypeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/oauth/authorize?client_id=client-app-1&redirect_uri=https://demo.app/callback&response_type=token&state=teststate", nil)
	unsupportedResponseTypeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleAuthorizeOIDC(unsupportedResponseTypeResponseRecorder, unsupportedResponseTypeRequest)
	if unsupportedResponseTypeResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on unsupported response_type, got: %d", unsupportedResponseTypeResponseRecorder.Code)
	}
	location := unsupportedResponseTypeResponseRecorder.Header().Get("Location")
	if !strings.Contains(location, "error=unsupported_response_type") || !strings.Contains(location, "state=teststate") {
		t.Fatalf("expected error=unsupported_response_type in Location header, got: %s", location)
	}

	// Missing PKCE / invalid method
	missingPKCERequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/oauth/authorize?client_id=client-app-1&redirect_uri=https://demo.app/callback&response_type=code&state=teststate", nil)
	missingPKCEResponseRecorder := httptest.NewRecorder()
	baseHandler.handleAuthorizeOIDC(missingPKCEResponseRecorder, missingPKCERequest)
	if missingPKCEResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on missing PKCE, got: %d", missingPKCEResponseRecorder.Code)
	}
	location = missingPKCEResponseRecorder.Header().Get("Location")
	if !strings.Contains(location, "error=invalid_request") {
		t.Fatalf("expected error=invalid_request in Location header, got: %s", location)
	}

	// Valid unauthenticated request -> Renders Hosted Universal Sign-In Page
	validAuthorizeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/oauth/authorize?client_id=client-app-1&redirect_uri=https://demo.app/callback&response_type=code&state=clientstate123&code_challenge=abc123challenge&code_challenge_method=S256", nil)
	validAuthorizeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleAuthorizeOIDC(validAuthorizeResponseRecorder, validAuthorizeRequest)
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
	if strings.Contains(htmlBody, "Secured by") {
		t.Fatalf("expected brand footer Secured by to be removed from HTML, got: %s", htmlBody)
	}
	if !strings.Contains(htmlBody, "https://demo.app/terms") || !strings.Contains(htmlBody, "https://demo.app/privacy") {
		t.Fatalf("expected legal footer links in HTML, got: %s", htmlBody)
	}
	if !strings.Contains(htmlBody, ".custom-class { color: red; }") {
		t.Fatalf("expected custom CSS injected into HTML, got: %s", htmlBody)
	}

	// Test Disabled OIDC.Enabled -> 403 Forbidden on all OIDC endpoints
	disabledOIDCConfig := configManager.Get()
	disabledOIDCConfig.OIDC.Enabled = false
	configManager.Set(disabledOIDCConfig)

	disabledAuthorizeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/oauth/authorize?client_id=client-app-1&redirect_uri=https://demo.app/callback&response_type=code&state=clientstate123&code_challenge=abc123challenge&code_challenge_method=S256", nil)
	disabledAuthorizeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleAuthorizeOIDC(disabledAuthorizeResponseRecorder, disabledAuthorizeRequest)
	if disabledAuthorizeResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on disabled OIDC authorize, got: %d", disabledAuthorizeResponseRecorder.Code)
	}

	disabledTokenRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&code=anycode"))
	disabledTokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	disabledTokenResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(disabledTokenResponseRecorder, disabledTokenRequest)
	if disabledTokenResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on disabled OIDC token, got: %d", disabledTokenResponseRecorder.Code)
	}

	// Re-enable OIDC for subsequent tests
	enabledConfig := configManager.Get()
	enabledConfig.OIDC.Enabled = true
	configManager.Set(enabledConfig)

	// 5. Authorization Submit (POST /v1/auth/oauth/authorize)
	invalidStateRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state=nonexistent-state-id&email=test@example.com&password=pass"))
	invalidStateRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalidStateResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(invalidStateResponseRecorder, invalidStateRequest)
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
	_ = testKVStore.Set(context.Background(), "auth:oidc:state:"+validStateID, string(stateBytes), 10*time.Minute)

	emptyCredsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="+validStateID+"&email=&password="))
	emptyCredsRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyCredsResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(emptyCredsResponseRecorder, emptyCredsRequest)
	if emptyCredsResponseRecorder.Code != http.StatusOK || !strings.Contains(emptyCredsResponseRecorder.Body.String(), "Email and password are required") {
		t.Fatalf("expected 200 re-render with validation message, got: %d (%s)", emptyCredsResponseRecorder.Code, emptyCredsResponseRecorder.Body.String())
	}

	// 6. Token Endpoint Validation (POST /v1/auth/oauth/token)
	badGrantRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=implicit"))
	badGrantRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badGrantResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(badGrantResponseRecorder, badGrantRequest)
	if badGrantResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on unsupported grant_type, got: %d", badGrantResponseRecorder.Code)
	}

	missingCredsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&code=testcode"))
	missingCredsRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missingCredsResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(missingCredsResponseRecorder, missingCredsRequest)
	if missingCredsResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized on missing client credentials, got: %d", missingCredsResponseRecorder.Code)
	}

	badSecretRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-app-1&client_secret=wrongsecret&code=testcode"))
	badSecretRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badSecretResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(badSecretResponseRecorder, badSecretRequest)
	if badSecretResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized on invalid client_secret, got: %d", badSecretResponseRecorder.Code)
	}

	missingCodeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-spa-1&code=nonexistent-code&code_verifier=xyz"))
	missingCodeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missingCodeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(missingCodeResponseRecorder, missingCodeRequest)
	if missingCodeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on missing authorization code, got: %d", missingCodeResponseRecorder.Code)
	}

	// 7. Userinfo Endpoint Validation (GET /v1/auth/oauth/userinfo)
	missingBearerRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/oauth/userinfo", nil)
	missingBearerResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetOIDCUserInfo(missingBearerResponseRecorder, missingBearerRequest)
	if missingBearerResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing Bearer token, got: %d", missingBearerResponseRecorder.Code)
	}

	invalidBearerRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/oauth/userinfo", nil)
	invalidBearerRequest.Header.Set("Authorization", "Bearer invalid.token.structure")
	invalidBearerResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetOIDCUserInfo(invalidBearerResponseRecorder, invalidBearerRequest)
	if invalidBearerResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid Bearer token, got: %d", invalidBearerResponseRecorder.Code)
	}

	// 8. Sign-Out Endpoint Validation (GET & POST /v1/auth/oauth/sign-out)
	signOutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/oauth/sign-out", nil)
	signOutResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignOutOIDC(signOutResponseRecorder, signOutRequest)
	if signOutResponseRecorder.Code != http.StatusOK || !strings.Contains(signOutResponseRecorder.Body.String(), "Signed Out") {
		t.Fatalf("expected 200 OK HTML on sign-out without redirect, got: %d", signOutResponseRecorder.Code)
	}

	signOutRedirectRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/oauth/sign-out?post_sign_out_redirect_uri="+url.QueryEscape("https://demo.app/goodbye"), nil)
	signOutRedirectResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignOutOIDC(signOutRedirectResponseRecorder, signOutRedirectRequest)
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

	baseHandler := NewBaseHandler(nil, configManager, cryptoKeyManager)
	testKVStore := newInMemoryKVStore()
	baseHandler.SetKVStore(testKVStore)

	// 1. handleSubmitOIDCAuthorize when OIDC disabled -> 403
	disabledConfig := configManager.Get()
	disabledConfig.OIDC.Enabled = false
	configManager.Set(disabledConfig)

	submitRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/authorize", nil)
	submitResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(submitResponseRecorder, submitRequest)
	if submitResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when OIDC disabled on submit, got: %d", submitResponseRecorder.Code)
	}

	// 2. handleGetOIDCUserInfo when OIDC disabled -> 403
	userInfoResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetOIDCUserInfo(userInfoResponseRecorder, submitRequest)
	if userInfoResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when OIDC disabled on userinfo, got: %d", userInfoResponseRecorder.Code)
	}

	// 3. handleSignOutOIDC when OIDC disabled -> 403
	signOutResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignOutOIDC(signOutResponseRecorder, submitRequest)
	if signOutResponseRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when OIDC disabled on sign-out, got: %d", signOutResponseRecorder.Code)
	}

	// Re-enable OIDC
	disabledConfig.OIDC.Enabled = true
	configManager.Set(disabledConfig)

	// 4. handleSubmitOIDCAuthorize invalid form data -> 400
	badFormRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("key=%zz"))
	badFormRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badFormResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(badFormResponseRecorder, badFormRequest)
	if badFormResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad form data, got: %d", badFormResponseRecorder.Code)
	}

	// 5. handleSubmitOIDCAuthorize missing state -> 400
	emptyFormRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="))
	emptyFormRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyFormResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(emptyFormResponseRecorder, emptyFormRequest)
	if emptyFormResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty state, got: %d", emptyFormResponseRecorder.Code)
	}

	// 6. handleSubmitOIDCAuthorize nonexistent state in kvStore -> 400
	missingStateRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state=nonexistent_state"))
	missingStateRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missingStateResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(missingStateResponseRecorder, missingStateRequest)
	if missingStateResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing state in kvStore, got: %d", missingStateResponseRecorder.Code)
	}

	// 7. handleSubmitOIDCAuthorize corrupt state JSON in kvStore -> 400
	_ = testKVStore.Set(context.Background(), "auth:oidc:state:corrupt_state", "{invalid_json", 5*time.Minute)
	corruptStateRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state=corrupt_state"))
	corruptStateRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	corruptStateResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(corruptStateResponseRecorder, corruptStateRequest)
	if corruptStateResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on corrupt state JSON, got: %d", corruptStateResponseRecorder.Code)
	}

	// 8. handleSubmitOIDCAuthorize empty email/password -> render sign in page error
	validStateID := "valid_state_empty_creds"
	validStatePayload, _ := json.Marshal(OIDCAuthorizationStatePayload{
		ClientID:    "client-spa-1",
		RedirectURI: "https://demo.app/callback",
	})
	_ = testKVStore.Set(context.Background(), "auth:oidc:state:"+validStateID, string(validStatePayload), 5*time.Minute)
	emptyCredsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="+validStateID+"&email=&password="))
	emptyCredsRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyCredsResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(emptyCredsResponseRecorder, emptyCredsRequest)
	if !strings.Contains(emptyCredsResponseRecorder.Body.String(), "Email and password are required") {
		t.Fatalf("expected Email and password required in render, got: %s", emptyCredsResponseRecorder.Body.String())
	}

	// 9. handleIssueOIDCToken delegation branches (empty grant_type and provider param)
	firstDelegatedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader(""))
	firstDelegatedRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	firstDelegatedResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(firstDelegatedResponseRecorder, firstDelegatedRequest)
	if firstDelegatedResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on delegated callback without provider/code, got: %d", firstDelegatedResponseRecorder.Code)
	}

	secondDelegatedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&provider=unknown"))
	secondDelegatedRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	secondDelegatedResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(secondDelegatedResponseRecorder, secondDelegatedRequest)
	if secondDelegatedResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on delegated callback with provider, got: %d", secondDelegatedResponseRecorder.Code)
	}

	// 10. handleIssueOIDCTokenAuthorizationCode client not found -> 401
	unknownClientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=unknown_client"))
	unknownClientRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unknownClientResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(unknownClientResponseRecorder, unknownClientRequest)
	if unknownClientResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unknown client ID, got: %d", unknownClientResponseRecorder.Code)
	}

	// 11. handleIssueOIDCTokenAuthorizationCode corrupt code JSON in kvStore -> 400
	_ = testKVStore.Set(context.Background(), "auth:code:corrupt_code_123", "{invalid_json", 5*time.Minute)
	corruptCodeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-spa-1&code=corrupt_code_123"))
	corruptCodeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	corruptCodeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(corruptCodeResponseRecorder, corruptCodeRequest)
	if corruptCodeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on corrupt code JSON, got: %d", corruptCodeResponseRecorder.Code)
	}

	// 12. handleIssueOIDCTokenAuthorizationCode client mismatch -> 400
	mismatchedClientPayload, _ := json.Marshal(OIDCAuthorizationCodePayload{
		ClientID: "other-client-id",
	})
	_ = testKVStore.Set(context.Background(), "auth:code:mismatch_client_code", string(mismatchedClientPayload), 5*time.Minute)
	mismatchClientRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-spa-1&code=mismatch_client_code"))
	mismatchClientRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mismatchClientResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(mismatchClientResponseRecorder, mismatchClientRequest)
	if mismatchClientResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on client mismatch, got: %d", mismatchClientResponseRecorder.Code)
	}

	// 13. handleIssueOIDCTokenAuthorizationCode redirect_uri mismatch -> 400
	mismatchedRedirectPayload, _ := json.Marshal(OIDCAuthorizationCodePayload{
		ClientID:    "client-spa-1",
		RedirectURI: "https://demo.app/expected_callback",
	})
	_ = testKVStore.Set(context.Background(), "auth:code:mismatch_redirect_code", string(mismatchedRedirectPayload), 5*time.Minute)
	mismatchRedirectRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-spa-1&code=mismatch_redirect_code&redirect_uri=https://demo.app/other_callback"))
	mismatchRedirectRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mismatchRedirectResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(mismatchRedirectResponseRecorder, mismatchRedirectRequest)
	if mismatchRedirectResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on redirect_uri mismatch, got: %d", mismatchRedirectResponseRecorder.Code)
	}

	// 14. handleIssueOIDCTokenAuthorizationCode PKCE verification failure -> 400
	badPKCEPayload, _ := json.Marshal(OIDCAuthorizationCodePayload{
		ClientID:      "client-spa-1",
		CodeChallenge: "expected_pkce_challenge_hash",
	})
	_ = testKVStore.Set(context.Background(), "auth:code:bad_pkce_code", string(badPKCEPayload), 5*time.Minute)
	badPKCERequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-spa-1&code=bad_pkce_code&code_verifier=invalid_verifier"))
	badPKCERequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badPKCEResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(badPKCEResponseRecorder, badPKCERequest)
	if badPKCEResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on PKCE failure, got: %d", badPKCEResponseRecorder.Code)
	}

	// 15. handleIssueOIDCTokenRefreshToken confidential client invalid secret -> 401
	badConfidentialSecretRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=refresh_token&client_id=client-confidential-1&client_secret=wrong_secret"))
	badConfidentialSecretRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badConfidentialSecretResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(badConfidentialSecretResponseRecorder, badConfidentialSecretRequest)
	if badConfidentialSecretResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on bad confidential client secret, got: %d", badConfidentialSecretResponseRecorder.Code)
	}

	// 16. handleIssueOIDCTokenRefreshToken missing refresh_token -> 400
	missingRefreshRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=refresh_token&client_id=client-spa-1&refresh_token="))
	missingRefreshRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missingRefreshResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(missingRefreshResponseRecorder, missingRefreshRequest)
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
	baseHandler.renderOIDCSignInPage(renderResponseRecorder, "state_render", &OIDCClientConfig{Name: "Client App"}, "Some error")
	renderHTML := renderResponseRecorder.Body.String()
	if !strings.Contains(renderHTML, "Layr") || !strings.Contains(renderHTML, "Google") || !strings.Contains(renderHTML, "Github") {
		t.Fatalf("expected Layr and provider buttons in render, got: %s", renderHTML)
	}

	// 19. handleIssueOIDCToken missing code -> 400 invalid_grant
	emptyCodeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=client-spa-1&code="))
	emptyCodeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyCodeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(emptyCodeResponseRecorder, emptyCodeRequest)
	if emptyCodeResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing code in token exchange, got: %d", emptyCodeResponseRecorder.Code)
	}
}

func TestAuthOIDCClientCredentialsUnit(t *testing.T) {
	testCtx := context.Background()

	// 1. Missing secret
	baseHandler := &BaseHandler{}
	missingSecretRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=client_credentials&client_id=sa-1"))
	missingSecretRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missingSecretResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(missingSecretResponseRecorder, missingSecretRequest)
	if missingSecretResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing secret, got: %d", missingSecretResponseRecorder.Code)
	}

	// 2. Nil ServiceAccountManager
	nilManagerRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=client_credentials&client_id=sa-1&client_secret=secret123"))
	nilManagerRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	nilManagerResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOIDCToken(nilManagerResponseRecorder, nilManagerRequest)
	if nilManagerResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on nil serviceAccountManager, got: %d", nilManagerResponseRecorder.Code)
	}

	// 3. ServiceAccountManager Authenticate error (e.g. nil database pool)
	serviceAccountManager := core.NewServiceAccountManager(nil)
	managerBaseHandler := &BaseHandler{serviceAccountManager: serviceAccountManager}
	authErrorRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=client_credentials&client_id=sa-1&client_secret=secret123"))
	authErrorRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	authErrorResponseRecorder := httptest.NewRecorder()
	managerBaseHandler.handleIssueOIDCToken(authErrorResponseRecorder, authErrorRequest)
	if authErrorResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on auth error, got: %d", authErrorResponseRecorder.Code)
	}

	// 4. JSON body parsing with Content-Type application/json
	jsonRequestBody := `{"grant_type":"client_credentials","client_id":"sa-1","client_secret":"invalid_client_secret","scope":"data:read"}`
	jsonRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/auth/oauth/token", strings.NewReader(jsonRequestBody))
	jsonRequest.Header.Set("Content-Type", "application/json")
	jsonResponseRecorder := httptest.NewRecorder()
	managerBaseHandler.handleIssueOIDCToken(jsonResponseRecorder, jsonRequest)
	if jsonResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from json request through auth error, got: %d", jsonResponseRecorder.Code)
	}

	// 5. JSON body parsing without Content-Type header
	untypedJSONRequestBody := `{"grant_type":"client_credentials","client_id":"sa-1","client_secret":"invalid_client_secret"}`
	untypedJSONRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/auth/oauth/token", strings.NewReader(untypedJSONRequestBody))
	untypedJSONResponseRecorder := httptest.NewRecorder()
	managerBaseHandler.handleIssueOIDCToken(untypedJSONResponseRecorder, untypedJSONRequest)
	if untypedJSONResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from untyped json request, got: %d", untypedJSONResponseRecorder.Code)
	}

	// 6. Basic Auth header extraction
	basicAuthRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=client_credentials&scope=data:read"))
	basicAuthRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	basicAuthRequest.SetBasicAuth("sa-basic", "secret-basic")
	basicAuthResponseRecorder := httptest.NewRecorder()
	managerBaseHandler.handleIssueOIDCToken(basicAuthResponseRecorder, basicAuthRequest)
	if basicAuthResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from basic auth request through auth error, got: %d", basicAuthResponseRecorder.Code)
	}

	// 7. handleIssueOAuthToken delegation
	delegatedRequest := httptest.NewRequestWithContext(testCtx, http.MethodPost, "/v1/auth/oauth/token", strings.NewReader("grant_type=client_credentials&client_id=sa-1"))
	delegatedRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	delegatedResponseRecorder := httptest.NewRecorder()
	baseHandler.handleIssueOAuthToken(delegatedResponseRecorder, delegatedRequest)
	if delegatedResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from handleIssueOAuthToken, got: %d", delegatedResponseRecorder.Code)
	}
}

func TestAuthOIDCRenderHelperFunctionsUnit(t *testing.T) {
	configManager := NewConfigManager(nil, nil)
	config := configManager.Get()
	config.OIDC.Enabled = true
	config.OIDC.UI.ShowSignUp = true
	config.OIDC.UI.ShowEmailOTP = true
	configManager.Set(config)

	baseHandler := &BaseHandler{configManager: configManager}

	signUpResponseRecorder := httptest.NewRecorder()
	baseHandler.renderOIDCSignUpPage(signUpResponseRecorder, "st-1", &OIDCClientConfig{Name: "App"}, "sign up error")
	if !strings.Contains(signUpResponseRecorder.Body.String(), "sign up error") {
		t.Fatal("expected sign up error in html")
	}

	otpRequestResponseRecorder := httptest.NewRecorder()
	baseHandler.renderOIDCOTPRequestPage(otpRequestResponseRecorder, "st-2", &OIDCClientConfig{Name: "App"}, "otp request error", "otp request notice")
	if !strings.Contains(otpRequestResponseRecorder.Body.String(), "otp request error") || !strings.Contains(otpRequestResponseRecorder.Body.String(), "otp request notice") {
		t.Fatal("expected otp error/notice in html")
	}

	otpVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.renderOIDCOTPVerifyPage(otpVerifyResponseRecorder, "st-3", &OIDCClientConfig{Name: "App"}, "otp verify error", "otp verify notice", "test@example.com")
	if !strings.Contains(otpVerifyResponseRecorder.Body.String(), "otp verify error") || !strings.Contains(otpVerifyResponseRecorder.Body.String(), "test@example.com") {
		t.Fatal("expected verify error/recipient in html")
	}
}

func TestAuthOIDCModeQueryParamUnit(t *testing.T) {
	configManager := NewConfigManager(nil, nil)
	config := configManager.Get()
	config.OIDC.Enabled = true
	config.OIDC.Clients = []OIDCClientConfig{
		{ClientID: "client-1", Name: "Client One"},
	}
	configManager.Set(config)

	kvStore := newInMemoryKVStore()
	baseHandler := &BaseHandler{
		configManager: configManager,
		kvStore:       kvStore,
	}

	statePayload, _ := json.Marshal(OIDCAuthorizationStatePayload{
		ClientID: "client-1",
	})
	_ = kvStore.Set(context.Background(), "auth:oidc:state:state-mode-1", string(statePayload), 5*time.Minute)

	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/auth/oauth/authorize?state=state-mode-1&mode=sign_up", nil)
	responseRecorder := httptest.NewRecorder()
	baseHandler.handleAuthorizeOIDC(responseRecorder, request)
	if responseRecorder.Code != http.StatusOK || !strings.Contains(responseRecorder.Body.String(), "Create account") {
		t.Fatalf("expected 200 OK and Create account in body, got: %d", responseRecorder.Code)
	}
}

func TestAuthOIDCAuthorizeSubmitUnit(t *testing.T) {
	ctx := context.Background()
	configManager := NewConfigManager(nil, nil)
	config := configManager.Get()
	config.OIDC.Enabled = true
	config.OIDC.Clients = []OIDCClientConfig{
		{ClientID: "client-sub-1", Name: "Submit Client"},
	}
	configManager.Set(config)

	kvStore := newInMemoryKVStore()
	baseHandler := &BaseHandler{
		configManager: configManager,
		kvStore:       kvStore,
	}

	stateID := "sub-state-1"
	statePayload, _ := json.Marshal(OIDCAuthorizationStatePayload{
		ClientID:    "client-sub-1",
		RedirectURI: "https://example.com/cb",
	})
	_ = kvStore.Set(ctx, "auth:oidc:state:"+stateID, string(statePayload), 5*time.Minute)

	// 1. verify_mfa with empty mfa_token
	emptyTokenRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="+stateID+"&action=verify_mfa&mfa_token="))
	emptyTokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyTokenResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(emptyTokenResponseRecorder, emptyTokenRequest)
	if !strings.Contains(emptyTokenResponseRecorder.Body.String(), "MFA session expired") {
		t.Fatalf("expected MFA session expired, got: %s", emptyTokenResponseRecorder.Body.String())
	}

	// 2. verify_mfa with unknown mfa_token in kvStore
	unknownTokenRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="+stateID+"&action=verify_mfa&mfa_token=unknown_tk"))
	unknownTokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unknownTokenResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(unknownTokenResponseRecorder, unknownTokenRequest)
	if !strings.Contains(unknownTokenResponseRecorder.Body.String(), "MFA session expired") {
		t.Fatalf("expected MFA session expired, got: %s", unknownTokenResponseRecorder.Body.String())
	}

	// 3. send_otp with empty recipient
	emptyRecipientRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="+stateID+"&action=send_otp&recipient="))
	emptyRecipientRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyRecipientResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(emptyRecipientResponseRecorder, emptyRecipientRequest)
	if !strings.Contains(emptyRecipientResponseRecorder.Body.String(), "Please enter an email address or phone number") {
		t.Fatalf("expected Please enter an email address or phone number, got: %s", emptyRecipientResponseRecorder.Body.String())
	}

	// 4. send_otp email with email OTP disabled
	disabledEmailOTPRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="+stateID+"&action=send_otp&recipient=user@example.com"))
	disabledEmailOTPRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	disabledEmailOTPResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(disabledEmailOTPResponseRecorder, disabledEmailOTPRequest)
	if !strings.Contains(disabledEmailOTPResponseRecorder.Body.String(), "Email OTP sign-in is not available") {
		t.Fatalf("expected Email OTP sign-in is not available, got: %s", disabledEmailOTPResponseRecorder.Body.String())
	}

	// 5. send_otp phone with SMS OTP disabled
	disabledSMSOTPRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="+stateID+"&action=send_otp&recipient=+14155551234"))
	disabledSMSOTPRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	disabledSMSOTPResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(disabledSMSOTPResponseRecorder, disabledSMSOTPRequest)
	if !strings.Contains(disabledSMSOTPResponseRecorder.Body.String(), "SMS OTP sign-in is not available") {
		t.Fatalf("expected SMS OTP sign-in is not available, got: %s", disabledSMSOTPResponseRecorder.Body.String())
	}

	// 6. verify_otp with empty code
	emptyCodeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="+stateID+"&action=verify_otp&recipient=user@example.com&otp_code="))
	emptyCodeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyCodeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(emptyCodeResponseRecorder, emptyCodeRequest)
	if !strings.Contains(emptyCodeResponseRecorder.Body.String(), "Verification code is required") {
		t.Fatalf("expected Verification code is required, got: %s", emptyCodeResponseRecorder.Body.String())
	}

	// 7. sign_up when sign-up disabled
	disabledSignUpRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="+stateID+"&action=sign_up&email=a@b.com&password=pass"))
	disabledSignUpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	disabledSignUpResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(disabledSignUpResponseRecorder, disabledSignUpRequest)
	if !strings.Contains(disabledSignUpResponseRecorder.Body.String(), "Sign-up is disabled") {
		t.Fatalf("expected Sign-up is disabled, got: %s", disabledSignUpResponseRecorder.Body.String())
	}

	// 8. sign_up validation: empty email, passwords mismatch, short password
	config.OIDC.UI.ShowSignUp = true
	configManager.Set(config)

	emptyEmailRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="+stateID+"&action=sign_up&email=&password=pass"))
	emptyEmailRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyEmailResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(emptyEmailResponseRecorder, emptyEmailRequest)
	if !strings.Contains(emptyEmailResponseRecorder.Body.String(), "Email and password are required") {
		t.Fatalf("expected Email and password are required, got: %s", emptyEmailResponseRecorder.Body.String())
	}

	mismatchPasswordRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="+stateID+"&action=sign_up&email=u@e.com&password=pass1&confirm_password=pass2"))
	mismatchPasswordRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mismatchPasswordResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(mismatchPasswordResponseRecorder, mismatchPasswordRequest)
	if !strings.Contains(mismatchPasswordResponseRecorder.Body.String(), "Passwords do not match") {
		t.Fatalf("expected Passwords do not match, got: %s", mismatchPasswordResponseRecorder.Body.String())
	}

	shortPasswordRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="+stateID+"&action=sign_up&email=u@e.com&password=p&confirm_password=p"))
	shortPasswordRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	shortPasswordResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(shortPasswordResponseRecorder, shortPasswordRequest)
	if !strings.Contains(shortPasswordResponseRecorder.Body.String(), "Password must be at least") {
		t.Fatalf("expected Password must be at least, got: %s", shortPasswordResponseRecorder.Body.String())
	}

	// 9. sign_in when ShowPassword disabled
	config.OIDC.UI.ShowPassword = false
	configManager.Set(config)

	disabledSignInRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/auth/oauth/authorize", strings.NewReader("state="+stateID+"&action=sign_in&email=a@b.com&password=pass"))
	disabledSignInRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	disabledSignInResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSubmitOIDCAuthorize(disabledSignInResponseRecorder, disabledSignInRequest)
	if !strings.Contains(disabledSignInResponseRecorder.Body.String(), "Password sign-in is disabled") {
		t.Fatalf("expected Password sign-in is disabled, got: %s", disabledSignInResponseRecorder.Body.String())
	}
}

func TestAuthOIDCDiscoverySignOutMetadataUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	baseHandler := NewBaseHandler(nil, configManager, cryptoKeyManager)

	discoveryRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/.well-known/openid-configuration", nil)
	discoveryResponseRecorder := httptest.NewRecorder()
	baseHandler.handleGetOIDCDiscovery(discoveryResponseRecorder, discoveryRequest)

	if discoveryResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from discovery, got: %d", discoveryResponseRecorder.Code)
	}

	var discoveryResponse map[string]any
	if unmarshalErr := json.Unmarshal(discoveryResponseRecorder.Body.Bytes(), &discoveryResponse); unmarshalErr != nil {
		t.Fatalf("failed to parse discovery JSON: %v", unmarshalErr)
	}

	if discoveryResponse["backchannel_logout_supported"] != true {
		t.Fatalf("expected backchannel_logout_supported to be true, got: %v", discoveryResponse["backchannel_logout_supported"])
	}
	if discoveryResponse["backchannel_logout_session_supported"] != true {
		t.Fatalf("expected backchannel_logout_session_supported to be true, got: %v", discoveryResponse["backchannel_logout_session_supported"])
	}
	if discoveryResponse["frontchannel_logout_supported"] != true {
		t.Fatalf("expected frontchannel_logout_supported to be true, got: %v", discoveryResponse["frontchannel_logout_supported"])
	}
	if discoveryResponse["frontchannel_logout_session_supported"] != true {
		t.Fatalf("expected frontchannel_logout_session_supported to be true, got: %v", discoveryResponse["frontchannel_logout_session_supported"])
	}
}

func TestAuthOIDCSignOutFrontChannelIframeUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}
	jwtSigner, err := core.NewJWTSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create jwt signer: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	config := DefaultConfig()
	config.OIDC.Enabled = true
	config.OIDC.Clients = []OIDCClientConfig{
		{
			ClientID:                           "client-front-channel",
			Name:                               "Front Channel Client",
			FrontChannelSignOutURI:             "https://rp.example.com/front-sign-out",
			FrontChannelSignOutSessionRequired: true,
			PostSignOutRedirectURIs:            []string{"https://rp.example.com/signed-out"},
		},
	}
	configManager.Set(config)

	baseHandler := NewBaseHandler(nil, configManager, cryptoKeyManager)

	// Create valid ID token hint for user and session
	idTokenHint, generateErr := jwtSigner.GenerateIDToken(core.JWTClaims{
		Subject:   "user-front-123",
		SessionID: "sess-front-456",
		Audience:  "client-front-channel",
	})
	if generateErr != nil {
		t.Fatalf("failed to generate id token hint: %v", generateErr)
	}

	// Sign out request with front-channel client and post_sign_out_redirect_uri
	signOutURL := "/v1/auth/oauth/sign-out?id_token_hint=" + idTokenHint + "&post_sign_out_redirect_uri=https://rp.example.com/signed-out&state=state123"
	signOutRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, signOutURL, nil)
	signOutResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignOutOIDC(signOutResponseRecorder, signOutRequest)

	if signOutResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from sign-out with front channel, got: %d", signOutResponseRecorder.Code)
	}
	responseBody := signOutResponseRecorder.Body.String()
	if !strings.Contains(responseBody, "<iframe src=\"https://rp.example.com/front-sign-out") {
		t.Fatalf("expected hidden iframe in body, got: %s", responseBody)
	}
	if !strings.Contains(responseBody, "https://rp.example.com/signed-out") {
		t.Fatalf("expected redirect URL in iframe page, got: %s", responseBody)
	}
}

func TestAuthOIDCSignOutPostRedirectValidationUnit(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}
	jwtSigner, err := core.NewJWTSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create jwt signer: %v", err)
	}

	configManager := NewConfigManager(nil, cryptoKeyManager)
	config := DefaultConfig()
	config.OIDC.Enabled = true
	config.OIDC.Clients = []OIDCClientConfig{
		{
			ClientID:                "trusted-client",
			PostSignOutRedirectURIs: []string{"https://trusted.example.com/logout-done"},
		},
	}
	configManager.Set(config)

	baseHandler := NewBaseHandler(nil, configManager, cryptoKeyManager)

	idTokenHint, _ := jwtSigner.GenerateIDToken(core.JWTClaims{
		Subject:  "user-trusted",
		Audience: "trusted-client",
	})

	// 1. Valid post_sign_out_redirect_uri -> redirects 302 to registered URI
	validURL := "/v1/auth/oauth/sign-out?id_token_hint=" + idTokenHint + "&post_sign_out_redirect_uri=https://trusted.example.com/logout-done&state=abc"
	validRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, validURL, nil)
	validResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignOutOIDC(validResponseRecorder, validRequest)

	if validResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect for valid URI, got: %d", validResponseRecorder.Code)
	}
	redirectLocation := validResponseRecorder.Header().Get("Location")
	if !strings.Contains(redirectLocation, "https://trusted.example.com/logout-done") || !strings.Contains(redirectLocation, "state=abc") {
		t.Fatalf("unexpected redirect location: %s", redirectLocation)
	}

	// 2. Untrusted post_sign_out_redirect_uri -> renders default signed out HTML page instead of redirecting
	untrustedURL := "/v1/auth/oauth/sign-out?id_token_hint=" + idTokenHint + "&post_sign_out_redirect_uri=https://attacker.example.com/evil"
	untrustedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, untrustedURL, nil)
	untrustedResponseRecorder := httptest.NewRecorder()
	baseHandler.handleSignOutOIDC(untrustedResponseRecorder, untrustedRequest)

	if untrustedResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for untrusted redirect URI, got: %d", untrustedResponseRecorder.Code)
	}
	if !strings.Contains(untrustedResponseRecorder.Body.String(), "You have been signed out") {
		t.Fatalf("expected default signed-out confirmation page, got: %s", untrustedResponseRecorder.Body.String())
	}
}
