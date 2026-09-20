package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

type mockOAuthServer struct {
	mutex            sync.Mutex
	validCode        string
	expectedClientID string
	expectedSecret   string
	issuedTokens     map[string]string // accessToken -> userID
}

func newMockOAuthServer(expectedClientID, expectedSecret, validCode string) *mockOAuthServer {
	return &mockOAuthServer{
		validCode:        validCode,
		expectedClientID: expectedClientID,
		expectedSecret:   expectedSecret,
		issuedTokens:     make(map[string]string),
	}
}

func (server *mockOAuthServer) handleAuthorize(responseWriter http.ResponseWriter, request *http.Request) {
	queryValues := request.URL.Query()
	redirectURI := queryValues.Get("redirect_uri")
	state := queryValues.Get("state")
	clientID := queryValues.Get("client_id")

	if clientID != server.expectedClientID {
		http.Error(responseWriter, "unauthorized client", http.StatusBadRequest)
		return
	}

	redirectURL, err := url.Parse(redirectURI)
	if err != nil {
		http.Error(responseWriter, "invalid redirect_uri", http.StatusBadRequest)
		return
	}

	redirectQueryValues := redirectURL.Query()
	redirectQueryValues.Set("code", server.validCode)
	redirectQueryValues.Set("state", state)
	redirectURL.RawQuery = redirectQueryValues.Encode()

	http.Redirect(responseWriter, request, redirectURL.String(), http.StatusFound)
}

func (server *mockOAuthServer) handleToken(responseWriter http.ResponseWriter, request *http.Request) {
	if err := request.ParseForm(); err != nil {
		http.Error(responseWriter, "invalid form data", http.StatusBadRequest)
		return
	}

	clientID := request.FormValue("client_id")
	clientSecret := request.FormValue("client_secret")
	code := request.FormValue("code")
	grantType := request.FormValue("grant_type")

	if grantType != "authorization_code" {
		responseWriter.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(responseWriter).Encode(map[string]string{"error": "unsupported_grant_type"})
		return
	}

	if clientID != server.expectedClientID || clientSecret != server.expectedSecret {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(responseWriter).Encode(map[string]string{"error": "invalid_client"})
		return
	}

	if code != server.validCode {
		responseWriter.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(responseWriter).Encode(map[string]string{"error": "invalid_grant"})
		return
	}

	accessToken := "mock_access_token_live_9999"
	server.mutex.Lock()
	server.issuedTokens[accessToken] = "user_oauth_7777"
	server.mutex.Unlock()

	// Construct mock ID Token JWT
	headerJSON := `{"alg":"none","typ":"JWT"}`
	claimsJSON := `{"sub":"user_oauth_7777","email":"user777@example.com"}`
	idToken := fmt.Sprintf("%s.%s.",
		base64.RawURLEncoding.EncodeToString([]byte(headerJSON)),
		base64.RawURLEncoding.EncodeToString([]byte(claimsJSON)),
	)

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"id_token":     idToken,
		"expires_in":   3600,
	})
}

func (server *mockOAuthServer) handleUserInfo(responseWriter http.ResponseWriter, request *http.Request) {
	authHeader := request.Header.Get("Authorization")
	expectedToken := "Bearer mock_access_token_live_9999"
	if authHeader != expectedToken {
		responseWriter.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(responseWriter).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"sub":         "user_oauth_7777",
		"email":       "user777@example.com",
		"name":        "Test OAuth User",
		"picture":     "https://cdn.example.com/avatars/user777.png",
		"locale":      "en-US",
		"custom_tier": "enterprise",
	})
}

func TestOauthAuthenticationFlowE2E(t *testing.T) {
	expectedClientID := "layr-oauth-client-id"
	expectedSecret := "layr-oauth-client-secret"
	validAuthorizationCode := "valid_auth_code_123456"

	oauthServer := newMockOAuthServer(expectedClientID, expectedSecret, validAuthorizationCode)

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/oauth/authorize", oauthServer.handleAuthorize)
	serveMux.HandleFunc("/oauth/token", oauthServer.handleToken)
	serveMux.HandleFunc("/oauth/userinfo", oauthServer.handleUserInfo)

	testServer := httptest.NewServer(serveMux)
	defer testServer.Close()

	providerConfig := ProviderConfig{
		Name:         "mock-idp",
		AuthURL:      testServer.URL + "/oauth/authorize",
		TokenURL:     testServer.URL + "/oauth/token",
		UserInfoURL:  testServer.URL + "/oauth/userinfo",
		ClientID:     expectedClientID,
		ClientSecret: expectedSecret,
		Scope:        "openid email profile",
		ResponseType: "code",
	}

	// 1. Initiate authorization request
	clientCallbackURL := "http://localhost:8080/v1/auth/oauth/callback"
	sessionState := "random_security_state_98765"
	authorizeURL, err := BuildAuthorizeURLWithConfig(providerConfig, clientCallbackURL, sessionState)
	if err != nil {
		t.Fatalf("failed to build authorize URL: %v", err)
	}

	// 2. Simulate browser visiting authorization URL and receiving redirect with code
	httpClient := &http.Client{
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			// Do not follow redirect; inspect redirect target directly
			return http.ErrUseLastResponse
		},
	}

	getRequest, err := http.NewRequestWithContext(context.Background(), http.MethodGet, authorizeURL, nil)
	if err != nil {
		t.Fatalf("failed to create authorize request: %v", err)
	}
	authorizationResponse, err := httpClient.Do(getRequest)
	if err != nil {
		t.Fatalf("failed to GET authorize URL: %v", err)
	}
	defer func() { _ = authorizationResponse.Body.Close() }()

	if authorizationResponse.StatusCode != http.StatusFound {
		t.Fatalf("expected 302 Found redirect, got: %d", authorizationResponse.StatusCode)
	}

	redirectLocation := authorizationResponse.Header.Get("Location")
	parsedRedirectURL, err := url.Parse(redirectLocation)
	if err != nil {
		t.Fatalf("failed to parse redirect URL: %v", err)
	}

	receivedState := parsedRedirectURL.Query().Get("state")
	if receivedState != sessionState {
		t.Fatalf("state mismatch: expected %s, got %s", sessionState, receivedState)
	}

	receivedCode := parsedRedirectURL.Query().Get("code")
	if receivedCode != validAuthorizationCode {
		t.Fatalf("code mismatch: expected %s, got %s", validAuthorizationCode, receivedCode)
	}

	// 3. Application callback exchanges authorization code for user profile
	ctx := context.Background()
	userInfo, err := ExchangeCodeWithConfig(ctx, providerConfig, receivedCode, clientCallbackURL)
	if err != nil {
		t.Fatalf("ExchangeCodeWithConfig failed: %v", err)
	}

	if userInfo.Provider != "mock-idp" {
		t.Fatalf("unexpected provider: %s", userInfo.Provider)
	}
	if userInfo.ProviderUserID != "user_oauth_7777" {
		t.Fatalf("unexpected ProviderUserID: %s", userInfo.ProviderUserID)
	}
	if userInfo.Email != "user777@example.com" {
		t.Fatalf("unexpected Email: %s", userInfo.Email)
	}
	if userInfo.Name != "Test OAuth User" {
		t.Fatalf("unexpected Name: %s", userInfo.Name)
	}
	if userInfo.AvatarURL != "https://cdn.example.com/avatars/user777.png" {
		t.Fatalf("unexpected AvatarURL: %s", userInfo.AvatarURL)
	}
	if userInfo.Properties["custom_tier"] != "enterprise" {
		t.Fatalf("unexpected custom property: %v", userInfo.Properties["custom_tier"])
	}

	// 4. Failure scenario: Invalid authorization code
	_, err = ExchangeCodeWithConfig(ctx, providerConfig, "invalid_or_expired_code", clientCallbackURL)
	if err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("expected invalid_grant error on invalid code, got: %v", err)
	}

	// 5. Failure scenario: Invalid client secret
	badSecretProviderConfig := providerConfig
	badSecretProviderConfig.ClientSecret = "wrong-secret"
	_, err = ExchangeCodeWithConfig(ctx, badSecretProviderConfig, receivedCode, clientCallbackURL)
	if err == nil || !strings.Contains(err.Error(), "invalid_client") {
		t.Fatalf("expected invalid_client error on bad secret, got: %v", err)
	}
}
