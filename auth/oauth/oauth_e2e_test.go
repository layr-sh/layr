package oauth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"time"

	"encoding/base64"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

type mockOAuthServer struct {
	issuer           string
	privateKey       ed25519.PrivateKey
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

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{"iss": server.issuer, "aud": server.expectedClientID, "exp": time.Now().Add(time.Hour).Unix(), "sub": "user_oauth_7777", "email": "user777@example.com"})
	token.Header["kid"] = "test-privateKey"
	idToken, err := token.SignedString(server.privateKey)
	if err != nil {
		http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
		return
	}

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

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	oauthServer.privateKey = privateKey
	serveMux.HandleFunc("/.well-known/openid-configuration", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(responseWriter).Encode(map[string]string{"issuer": oauthServer.issuer, "authorization_endpoint": oauthServer.issuer + "/oauth/authorize", "token_endpoint": oauthServer.issuer + "/oauth/token", "jwks_uri": oauthServer.issuer + "/jwks"})
	})
	serveMux.HandleFunc("/jwks", func(responseWriter http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{"keys": []map[string]string{{"kid": "test-privateKey", "kty": "OKP", "crv": tls.Ed25519.String(), "alg": "EdDSA", "x": base64.RawURLEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey))}}})
	})
	testServer := httptest.NewTLSServer(serveMux)
	oauthServer.issuer = testServer.URL
	SetHTTPClient(testServer.Client())
	defer SetHTTPClient(nil)
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
	clientCallbackURL := "http://localhost:5297/v1/oauth/callback"
	sessionState := "random_security_state_98765"
	authorizeURL, err := BuildAuthorizeURLWithConfig(providerConfig, clientCallbackURL, sessionState)
	if err != nil {
		t.Fatalf("failed to build authorize URL: %v", err)
	}

	// 2. Simulate browser visiting authorization URL and receiving redirect with code
	httpClient := &http.Client{
		Transport: testServer.Client().Transport,
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
