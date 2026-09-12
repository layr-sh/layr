package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"layr.sh/auth/jwt"
	"layr.sh/core"
)

func TestAuthOIDCStandaloneIdentityProviderIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	databaseKVStore := core.NewDatabaseKVStore(ctx, db, 60*time.Second)
	defer func() { _ = databaseKVStore.Close() }()
	eventBus := core.NewEventBus(db, cryptoKeyManager)
	serviceAccountManager := core.NewServiceAccountManager(db)

	baseURL := core.GetConfig().ServerBaseURL()
	handler := NewHandler(db, configManager, cryptoKeyManager)
	handler.SetKVStore(databaseKVStore)
	handler.SetEventBus(eventBus)
	handler.SetServiceAccountManager(serviceAccountManager)

	// 1. Create a registered test user directly in database
	testUserEmail := "user.oidc@example.com"
	testUserPassword := "SecurePassword123!"
	passwordHash, err := handler.hasher.Hash(testUserPassword)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	var testUserID string
	err = db.QueryRow(ctx, `
		INSERT INTO auth.users (email, password_hash, role, is_anonymous, created_at, last_updated_at)
		VALUES ($1, $2, 'authenticated', false, clock_timestamp(), clock_timestamp())
		RETURNING id
	`, testUserEmail, passwordHash).Scan(&testUserID)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	// 2. Configure OIDC Clients and UI Customization
	confidentialSecret := "confidential-client-secret-999"
	encryptedConfidentialSecret, encryptErr := cryptoKeyManager.EncryptField([]byte(confidentialSecret))
	if encryptErr != nil {
		t.Fatalf("failed to encrypt client secret: %v", encryptErr)
	}

	authConfig := configManager.Get()
	authConfig.OIDC.Enabled = true
	authConfig.OIDC.SignInUI = OIDCSignInUIConfig{
		CustomCSS: ".signin-card { border: 2px solid cyan; }",
		LogoURL:   "https://example.com/assets/logo.svg",
	}
	authConfig.OIDC.Clients = []OIDCClientConfig{
		{
			Name:                    "Dashboard Portal",
			ClientID:                "client-dashboard",
			ClientSecret:            encryptedConfidentialSecret,
			RedirectURIs:            []string{"https://dashboard.example.com/callback"},
			PostSignOutRedirectURIs: []string{"https://dashboard.example.com/signed-out"},
			Public:                  false,
			Scopes:                  []string{"openid", "email", "profile"},
		},
		{
			Name:                    "Mobile Native Client",
			ClientID:                "client-mobile",
			RedirectURIs:            []string{"https://mobile.example.com/callback"},
			PostSignOutRedirectURIs: []string{"https://mobile.example.com/signed-out"},
			Public:                  true,
			Scopes:                  []string{"openid", "email"},
		},
	}
	if saveConfigErr := configManager.Save(ctx, authConfig); saveConfigErr != nil {
		t.Fatalf("failed to save oidc config: %v", saveConfigErr)
	}

	// 3. OIDC Discovery & JWKS
	discoveryRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/.well-known/openid-configuration", nil)
	discoveryResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCDiscovery(discoveryResponseRecorder, discoveryRequest)
	if discoveryResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from discovery, got: %d", discoveryResponseRecorder.Code)
	}
	var discoveryResponse map[string]any
	if decodeErr := json.NewDecoder(discoveryResponseRecorder.Body).Decode(&discoveryResponse); decodeErr != nil {
		t.Fatalf("failed to parse discovery JSON: %v", decodeErr)
	}
	if discoveryResponse["issuer"] != baseURL {
		t.Fatalf("expected issuer %s, got: %v", baseURL, discoveryResponse["issuer"])
	}
	if discoveryResponse["authorization_endpoint"] != baseURL+"/api/v1/auth/oauth/authorize" {
		t.Fatalf("unexpected authorization endpoint: %v", discoveryResponse["authorization_endpoint"])
	}

	// 4. Authorization Flow with PKCE (GET /api/v1/auth/oauth/authorize)
	codeVerifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk-long-verifier-for-testing"
	sha256Digest := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(sha256Digest[:])

	authorizeURL := fmt.Sprintf(
		"/api/v1/auth/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code&state=%s&code_challenge=%s&code_challenge_method=S256&scope=openid+email",
		url.QueryEscape("client-dashboard"),
		url.QueryEscape("https://dashboard.example.com/callback"),
		url.QueryEscape("client-csrf-state-123"),
		url.QueryEscape(codeChallenge),
	)

	authorizeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, authorizeURL, nil)
	authorizeResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorize(authorizeResponseRecorder, authorizeRequest)

	if authorizeResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK rendering sign-in page, got: %d (%s)", authorizeResponseRecorder.Code, authorizeResponseRecorder.Body.String())
	}

	htmlPage := authorizeResponseRecorder.Body.String()
	if !strings.Contains(htmlPage, ".signin-card { border: 2px solid cyan; }") {
		t.Fatalf("expected injected custom CSS in HTML, got: %s", htmlPage)
	}
	if !strings.Contains(htmlPage, "Dashboard Portal") {
		t.Fatalf("expected client name 'Dashboard Portal' in HTML, got: %s", htmlPage)
	}
	if !strings.Contains(htmlPage, "https://example.com/assets/logo.svg") {
		t.Fatalf("expected logo url in HTML, got: %s", htmlPage)
	}

	// Extract state ID from hidden form input
	stateInputPrefix := `name="state" value="`
	stateInputIndex := strings.Index(htmlPage, stateInputPrefix)
	if stateInputIndex == -1 {
		t.Fatalf("state input not found in rendered sign-in page: %s", htmlPage)
	}
	stateValueStart := stateInputIndex + len(stateInputPrefix)
	stateValueEnd := strings.Index(htmlPage[stateValueStart:], `"`)
	if stateValueEnd == -1 {
		t.Fatalf("closing quote for state input not found")
	}
	serverStateID := htmlPage[stateValueStart : stateValueStart+stateValueEnd]
	if serverStateID == "" {
		t.Fatal("extracted empty state ID from sign-in page")
	}

	// 5. State Verification & Submission (POST /api/v1/auth/oauth/authorize)
	tamperedStateValues := url.Values{
		"state":    {"tampered-state-id"},
		"email":    {testUserEmail},
		"password": {testUserPassword},
	}
	tamperedRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(tamperedStateValues.Encode()))
	tamperedRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tamperedResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(tamperedResponseRecorder, tamperedRequest)
	if tamperedResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on tampered state, got: %d", tamperedResponseRecorder.Code)
	}

	_, _ = db.Exec(ctx, "UPDATE auth.users SET phone = '+15551234567', properties = '{\"name\":\"Test User Name\"}'::jsonb WHERE email = $1", testUserEmail)

	lockedPasswordHash, _ := handler.hasher.Hash("LockedPass123!")
	_, _ = db.Exec(ctx, `
		INSERT INTO auth.users (email, password_hash, role, locked_until)
		VALUES ('locked.user@example.com', $1, 'authenticated', clock_timestamp() + interval '1 hour')
	`, lockedPasswordHash)

	// Unknown email
	unknownUserValues := url.Values{"state": {serverStateID}, "email": {"nonexistent@example.com"}, "password": {"SomePass123!"}}
	unknownUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(unknownUserValues.Encode()))
	unknownUserRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unknownUserResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(unknownUserResponseRecorder, unknownUserRequest)
	if !strings.Contains(unknownUserResponseRecorder.Body.String(), "Invalid email or password") {
		t.Fatalf("expected Invalid email or password on unknown user, got: %s", unknownUserResponseRecorder.Body.String())
	}

	// Locked user
	lockedUserValues := url.Values{"state": {serverStateID}, "email": {"locked.user@example.com"}, "password": {"LockedPass123!"}}
	lockedUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(lockedUserValues.Encode()))
	lockedUserRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	lockedUserResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(lockedUserResponseRecorder, lockedUserRequest)
	if !strings.Contains(lockedUserResponseRecorder.Body.String(), "Account temporarily locked") {
		t.Fatalf("expected Account temporarily locked on locked user, got: %s", lockedUserResponseRecorder.Body.String())
	}

	// Wrong password
	wrongPassValues := url.Values{"state": {serverStateID}, "email": {testUserEmail}, "password": {"WrongPassword!"}}
	wrongPassRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(wrongPassValues.Encode()))
	wrongPassRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wrongPassResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(wrongPassResponseRecorder, wrongPassRequest)
	if !strings.Contains(wrongPassResponseRecorder.Body.String(), "Invalid email or password") {
		t.Fatalf("expected Invalid email or password on wrong password, got: %s", wrongPassResponseRecorder.Body.String())
	}

	// Bad redirect URI in state payload
	badRedirectStateID := "bad-redirect-state-id"
	badRedirectPayload, _ := json.Marshal(OIDCAuthorizationStatePayload{ClientID: "client-dashboard", RedirectURI: "://invalid-url", Scope: "openid"})
	_ = databaseKVStore.Set(ctx, "auth:oidc:state:"+badRedirectStateID, string(badRedirectPayload), 5*time.Minute)
	badRedirectValues := url.Values{"state": {badRedirectStateID}, "email": {testUserEmail}, "password": {testUserPassword}}
	badRedirectRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(badRedirectValues.Encode()))
	badRedirectRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badRedirectResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(badRedirectResponseRecorder, badRedirectRequest)
	if badRedirectResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad redirect URI in state payload, got: %d", badRedirectResponseRecorder.Code)
	}

	// Zero expiry in sessions config to cover refreshExpiry <= 0 and accessExpiry <= 0 fallbacks
	zeroExpiryConfig := configManager.Get()
	zeroExpiryConfig.Sessions.AccessTokenExpirySeconds = 0
	zeroExpiryConfig.Sessions.RefreshTokenExpirySeconds = 0
	configManager.Set(zeroExpiryConfig)

	// Submit with valid credentials
	validSubmitValues := url.Values{
		"state":    {serverStateID},
		"email":    {testUserEmail},
		"password": {testUserPassword},
	}
	submitRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(validSubmitValues.Encode()))
	submitRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	submitResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(submitResponseRecorder, submitRequest)

	if submitResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 Found redirect on valid sign-in, got: %d (%s)", submitResponseRecorder.Code, submitResponseRecorder.Body.String())
	}

	var sessionCookie *http.Cookie
	for _, cookie := range submitResponseRecorder.Result().Cookies() {
		if cookie.Name == AuthSessionInsecureCookieName || cookie.Name == AuthSessionCookieName {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatal("expected session cookie to be set in response")
	}

	redirectLocation := submitResponseRecorder.Header().Get("Location")
	parsedRedirectURL, parseErr := url.Parse(redirectLocation)
	if parseErr != nil {
		t.Fatalf("failed to parse redirect location '%s': %v", redirectLocation, parseErr)
	}
	if parsedRedirectURL.Query().Get("state") != "client-csrf-state-123" {
		t.Fatalf("expected client state 'client-csrf-state-123', got: %s", parsedRedirectURL.Query().Get("state"))
	}
	authorizationCode := parsedRedirectURL.Query().Get("code")
	if authorizationCode == "" {
		t.Fatal("expected authorization code in redirect query params")
	}

	// Verify state payload single-use deletion: submitting same state again must fail with 400
	replayedSubmitResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorizeSubmit(replayedSubmitResponseRecorder, submitRequest)
	if replayedSubmitResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on replaying deleted authorization state, got: %d", replayedSubmitResponseRecorder.Code)
	}

	// 6. Token Exchange (POST /api/v1/auth/oauth/token)
	_ = db.QueryRow(ctx, "SELECT id FROM auth.users WHERE email = $1", testUserEmail).Scan(&testUserID)

	// Test redirect_uri mismatch -> 400
	mismatchCode := handler.issueOIDCAuthorizationCode(ctx, "client-dashboard", "https://dashboard.example.com/callback", testUserID, "openid profile email", codeChallenge, "S256", "nonce-mismatch")
	mismatchValues := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {"client-dashboard"},
		"client_secret": {confidentialSecret},
		"code":          {mismatchCode},
		"redirect_uri":  {"https://dashboard.example.com/other"},
		"code_verifier": {codeVerifier},
	}
	mismatchRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(mismatchValues.Encode()))
	mismatchRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mismatchResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(mismatchResponseRecorder, mismatchRequest)
	if mismatchResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on redirect_uri mismatch, got: %d", mismatchResponseRecorder.Code)
	}

	// Test PKCE failure -> 400
	badPKCECode := handler.issueOIDCAuthorizationCode(ctx, "client-dashboard", "https://dashboard.example.com/callback", testUserID, "openid profile email", codeChallenge, "S256", "nonce-badpkce")
	badPKCEValues := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {"client-dashboard"},
		"client_secret": {confidentialSecret},
		"code":          {badPKCECode},
		"redirect_uri":  {"https://dashboard.example.com/callback"},
		"code_verifier": {"wrong-code-verifier-12345"},
	}
	badPKCERequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(badPKCEValues.Encode()))
	badPKCERequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badPKCEResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(badPKCEResponseRecorder, badPKCERequest)
	if badPKCEResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on PKCE verification failure, got: %d", badPKCEResponseRecorder.Code)
	}

	tokenValues := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {"client-dashboard"},
		"client_secret": {confidentialSecret},
		"code":          {authorizationCode},
		"redirect_uri":  {"https://dashboard.example.com/callback"},
		"code_verifier": {codeVerifier},
	}
	tokenRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(tokenValues.Encode()))
	tokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(tokenResponseRecorder, tokenRequest)

	if tokenResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from token endpoint, got: %d (%s)", tokenResponseRecorder.Code, tokenResponseRecorder.Body.String())
	}

	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
	}
	if decodeErr := json.NewDecoder(tokenResponseRecorder.Body).Decode(&tokenResponse); decodeErr != nil {
		t.Fatalf("failed to decode token response: %v", decodeErr)
	}
	if tokenResponse.AccessToken == "" || tokenResponse.RefreshToken == "" || tokenResponse.IDToken == "" {
		t.Fatalf("incomplete token response: %+v", tokenResponse)
	}
	if tokenResponse.TokenType != "Bearer" {
		t.Fatalf("expected token_type Bearer, got: %s", tokenResponse.TokenType)
	}

	// 7. Authorization Code Single-Use Replay Prevention
	replayTokenResponseRecorder := httptest.NewRecorder()
	replayTokenRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(tokenValues.Encode()))
	replayTokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.handleOIDCToken(replayTokenResponseRecorder, replayTokenRequest)
	if replayTokenResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on authorization code replay, got: %d", replayTokenResponseRecorder.Code)
	}

	// 8. UserInfo Endpoint (GET /api/v1/auth/oauth/userinfo)
	userinfoRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/auth/oauth/userinfo", nil)
	userinfoRequest.Header.Set("Authorization", "Bearer "+tokenResponse.AccessToken)
	userinfoResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCUserInfo(userinfoResponseRecorder, userinfoRequest)

	if userinfoResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from userinfo endpoint, got: %d (%s)", userinfoResponseRecorder.Code, userinfoResponseRecorder.Body.String())
	}

	var userinfoResponse struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	userinfoBody := userinfoResponseRecorder.Body.String()
	if unmarshalErr := json.Unmarshal([]byte(userinfoBody), &userinfoResponse); unmarshalErr != nil {
		t.Fatalf("failed to decode userinfo response: %v", unmarshalErr)
	}
	if userinfoResponse.Sub == "" || userinfoResponse.Email != testUserEmail {
		t.Fatalf("unexpected userinfo response: %+v", userinfoResponse)
	}
	if !strings.Contains(userinfoBody, "Test User Name") {
		t.Fatalf("expected user name in userinfo response: %s", userinfoBody)
	}

	// 9. Single Sign-On (SSO) Active Session Bypass
	mobileVerifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk-mobile-verifier"
	mobileDigest := sha256.Sum256([]byte(mobileVerifier))
	mobileChallenge := base64.RawURLEncoding.EncodeToString(mobileDigest[:])

	ssoAuthorizeURL := fmt.Sprintf(
		"/api/v1/auth/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code&state=%s&code_challenge=%s&code_challenge_method=S256",
		url.QueryEscape("client-mobile"),
		url.QueryEscape("https://mobile.example.com/callback"),
		url.QueryEscape("mobile-client-state-789"),
		url.QueryEscape(mobileChallenge),
	)
	ssoAuthorizeRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, ssoAuthorizeURL, nil)
	ssoAuthorizeRequest.AddCookie(sessionCookie)
	ssoAuthorizeResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCAuthorize(ssoAuthorizeResponseRecorder, ssoAuthorizeRequest)

	if ssoAuthorizeResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect for active SSO session, got: %d (%s)", ssoAuthorizeResponseRecorder.Code, ssoAuthorizeResponseRecorder.Body.String())
	}

	ssoRedirectLocation := ssoAuthorizeResponseRecorder.Header().Get("Location")
	parsedSSORedirectURL, ssoParseErr := url.Parse(ssoRedirectLocation)
	if ssoParseErr != nil {
		t.Fatalf("failed to parse SSO redirect: %v", ssoParseErr)
	}
	if parsedSSORedirectURL.Query().Get("state") != "mobile-client-state-789" {
		t.Fatalf("expected mobile state 'mobile-client-state-789', got: %s", parsedSSORedirectURL.Query().Get("state"))
	}
	mobileCode := parsedSSORedirectURL.Query().Get("code")
	if mobileCode == "" {
		t.Fatal("expected authorization code in SSO bypass redirect")
	}

	// 10. Public Client Token Exchange (PKCE Only, No Secret)
	mobileTokenValues := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {"client-mobile"},
		"code":          {mobileCode},
		"redirect_uri":  {"https://mobile.example.com/callback"},
		"code_verifier": {mobileVerifier},
	}
	mobileTokenRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(mobileTokenValues.Encode()))
	mobileTokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mobileTokenResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(mobileTokenResponseRecorder, mobileTokenRequest)

	if mobileTokenResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from public client token exchange, got: %d (%s)", mobileTokenResponseRecorder.Code, mobileTokenResponseRecorder.Body.String())
	}

	// 11. Refresh Token Exchange
	refreshValues := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {"client-dashboard"},
		"client_secret": {confidentialSecret},
		"refresh_token": {tokenResponse.RefreshToken},
	}
	refreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(refreshValues.Encode()))
	refreshRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	refreshResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(refreshResponseRecorder, refreshRequest)

	if refreshResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from refresh token grant, got: %d (%s)", refreshResponseRecorder.Code, refreshResponseRecorder.Body.String())
	}

	// 12. Sign-Out Endpoint (GET /api/v1/auth/oauth/sign-out)
	signOutRequestURL := "/api/v1/auth/oauth/sign-out?post_sign_out_redirect_uri=" + url.QueryEscape("https://dashboard.example.com/signed-out")
	signOutRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, signOutRequestURL, nil)
	signOutRequest.AddCookie(sessionCookie)
	signOutResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCSignOut(signOutResponseRecorder, signOutRequest)

	if signOutResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 Found on sign out redirect, got: %d", signOutResponseRecorder.Code)
	}
	if signOutResponseRecorder.Header().Get("Location") != "https://dashboard.example.com/signed-out" {
		t.Fatalf("expected redirect to post_sign_out_redirect_uri, got: %s", signOutResponseRecorder.Header().Get("Location"))
	}

	// Assert session cookie is cleared
	var clearedSessionCookie *http.Cookie
	for _, cookie := range signOutResponseRecorder.Result().Cookies() {
		if cookie.Name == AuthSessionInsecureCookieName || cookie.Name == AuthSessionCookieName {
			clearedSessionCookie = cookie
			break
		}
	}
	if clearedSessionCookie == nil || (clearedSessionCookie.MaxAge > 0 && clearedSessionCookie.Value != "") {
		t.Fatalf("expected cleared session cookie on sign out, got: %+v", clearedSessionCookie)
	}

	// 13. Edge Cases & Error Paths:
	// a. Invalid refresh token -> 400
	invalidRefreshValues := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {"client-dashboard"},
		"client_secret": {confidentialSecret},
		"refresh_token": {"nonexistent-refresh-token-hash"},
	}
	invalidRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(invalidRefreshValues.Encode()))
	invalidRefreshRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalidRefreshResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(invalidRefreshResponseRecorder, invalidRefreshRequest)
	if invalidRefreshResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid refresh token, got: %d", invalidRefreshResponseRecorder.Code)
	}

	// b. Deleted/nonexistent user in authorization code exchange -> 500
	orphanCode := handler.issueOIDCAuthorizationCode(ctx, "client-dashboard", "https://dashboard.example.com/callback", "01918a24-9999-7000-8000-000000000099", "openid profile email", codeChallenge, "S256", "nonce-orphan")
	orphanValues := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {"client-dashboard"},
		"client_secret": {confidentialSecret},
		"code":          {orphanCode},
		"redirect_uri":  {"https://dashboard.example.com/callback"},
		"code_verifier": {codeVerifier},
	}
	orphanRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(orphanValues.Encode()))
	orphanRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	orphanResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(orphanResponseRecorder, orphanRequest)
	if orphanResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on token exchange for nonexistent user, got: %d", orphanResponseRecorder.Code)
	}

	// c. Deleted/nonexistent user in userinfo -> 404
	ghostToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject: "01918a24-8888-7000-8000-000000000088",
		Email:   "ghost@example.com",
		Role:    "authenticated",
	}, 900)
	ghostRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/auth/oauth/userinfo", nil)
	ghostRequest.Header.Set("Authorization", "Bearer "+ghostToken)
	ghostResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCUserInfo(ghostResponseRecorder, ghostRequest)
	if ghostResponseRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on userinfo for nonexistent user, got: %d", ghostResponseRecorder.Code)
	}

	// d. CompleteOAuthFlow with OIDCStateID against real pool (covers sessions insert)
	oidcStateIDInteg := "oauth-integration-oidc-state"
	oidcPayloadJSONInteg, _ := json.Marshal(OIDCAuthorizationStatePayload{
		ClientID:            "client-dashboard",
		RedirectURI:         "https://dashboard.example.com/callback",
		Scope:               "openid profile",
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: "S256",
		ClientState:         "client_state_integration",
	})
	_ = databaseKVStore.Set(ctx, "auth:oidc:state:"+oidcStateIDInteg, string(oidcPayloadJSONInteg), 10*time.Minute)

	oauthStatePayload := OAuthStatePayload{
		StateID:     "oauth_state_integ",
		Provider:    "google",
		OIDCStateID: oidcStateIDInteg,
	}
	userRecord := UserRecord{
		ID:   testUserID,
		Role: "authenticated",
	}
	completeOIDCResponseRecorder := httptest.NewRecorder()
	completeOIDCRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/auth/oauth/google/callback", nil)
	completeOIDCRequest.Header.Set("X-Forwarded-Proto", "https")
	handler.CompleteOAuthFlow(completeOIDCResponseRecorder, completeOIDCRequest, userRecord, oauthStatePayload, true)
	if completeOIDCResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect from CompleteOAuthFlow with OIDCStateID, got: %d", completeOIDCResponseRecorder.Code)
	}
}

func TestAuthOIDCClientCredentialsIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	serviceAccountManager := core.NewServiceAccountManager(db)
	handler := NewHandler(db, configManager, cryptoKeyManager)
	handler.SetServiceAccountManager(serviceAccountManager)

	activeServiceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "M2M Worker Service",
		Scopes: []string{"data:*", "auth:read"},
	})
	if err != nil {
		t.Fatalf("failed to create active service account: %v", err)
	}

	// 1. Basic Auth credentials flow (inherits all scopes)
	basicAuthValues := url.Values{
		"grant_type": {"client_credentials"},
	}
	basicAuthRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(basicAuthValues.Encode()))
	basicAuthRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	basicAuthRequest.SetBasicAuth(activeServiceAccount.ID, activeServiceAccount.SecretKey)
	basicAuthResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(basicAuthResponseRecorder, basicAuthRequest)

	if basicAuthResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on Basic Auth client_credentials, got %d (%s)", basicAuthResponseRecorder.Code, basicAuthResponseRecorder.Body.String())
	}
	if cacheControl := basicAuthResponseRecorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Errorf("expected Cache-Control: no-store, got %s", cacheControl)
	}

	var basicOIDCTokenResponse OIDCTokenResponse
	if unmarshalErr := json.Unmarshal(basicAuthResponseRecorder.Body.Bytes(), &basicOIDCTokenResponse); unmarshalErr != nil {
		t.Fatalf("failed to unmarshal token response: %v", unmarshalErr)
	}
	if basicOIDCTokenResponse.TokenType != "Bearer" || basicOIDCTokenResponse.ExpiresIn != 3600 {
		t.Errorf("unexpected token type or expiry: %+v", basicOIDCTokenResponse)
	}
	if basicOIDCTokenResponse.RefreshToken != "" || basicOIDCTokenResponse.IDToken != "" {
		t.Errorf("expected no refresh token or id token in M2M response, got %+v", basicOIDCTokenResponse)
	}

	m2MClaims, verifyErr := handler.signer.VerifyM2MToken(basicOIDCTokenResponse.AccessToken)
	if verifyErr != nil {
		t.Fatalf("failed to verify M2M token: %v", verifyErr)
	}
	if m2MClaims.Subject != activeServiceAccount.ID {
		t.Errorf("expected subject %s, got %s", activeServiceAccount.ID, m2MClaims.Subject)
	}
	slugifier := core.NewSlugifier()
	expectedHandle := slugifier.Slugify(core.GetConfig().Project.Name)
	if expectedHandle == "" {
		expectedHandle = "layr"
	}
	if m2MClaims.Audience != expectedHandle+":service_account" {
		t.Errorf("expected audience %s:service_account, got %s", expectedHandle, m2MClaims.Audience)
	}
	if len(m2MClaims.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %v", m2MClaims.Scopes)
	}

	// 2. Form body credentials with scope down-scoping
	formValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {activeServiceAccount.KeyPrefix},
		"client_secret": {activeServiceAccount.SecretKey},
		"scope":         {"data:schema.read"},
	}
	formRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(formValues.Encode()))
	formRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	formResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(formResponseRecorder, formRequest)

	if formResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on form down-scoping, got %d (%s)", formResponseRecorder.Code, formResponseRecorder.Body.String())
	}
	var formOIDCTokenResponse OIDCTokenResponse
	_ = json.Unmarshal(formResponseRecorder.Body.Bytes(), &formOIDCTokenResponse)
	if formOIDCTokenResponse.Scope != "data:schema.read" {
		t.Errorf("expected scope data:schema.read, got %s", formOIDCTokenResponse.Scope)
	}

	// 3. JSON body credentials
	jsonBodyMap := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     activeServiceAccount.ID,
		"client_secret": activeServiceAccount.SecretKey,
		"scope":         "auth:read",
	}
	jsonBytes, _ := json.Marshal(jsonBodyMap)
	jsonRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", bytes.NewReader(jsonBytes))
	jsonRequest.Header.Set("Content-Type", "application/json")
	jsonResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(jsonResponseRecorder, jsonRequest)

	if jsonResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on JSON client_credentials, got %d (%s)", jsonResponseRecorder.Code, jsonResponseRecorder.Body.String())
	}

	// 4. Invalid scope exceeding permissions -> 400 Bad Request (invalid_scope)
	invalidScopeValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {activeServiceAccount.ID},
		"client_secret": {activeServiceAccount.SecretKey},
		"scope":         {"admin:super"},
	}
	invalidScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(invalidScopeValues.Encode()))
	invalidScopeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalidScopeResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(invalidScopeResponseRecorder, invalidScopeRequest)

	if invalidScopeResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(invalidScopeResponseRecorder.Body.String(), "invalid_scope") {
		t.Fatalf("expected 400 invalid_scope, got %d (%s)", invalidScopeResponseRecorder.Code, invalidScopeResponseRecorder.Body.String())
	}

	// 5. Client ID mismatch -> 401 Unauthorized
	mismatchValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"sa_wrong_id_123"},
		"client_secret": {activeServiceAccount.SecretKey},
	}
	mismatchRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(mismatchValues.Encode()))
	mismatchRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mismatchResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(mismatchResponseRecorder, mismatchRequest)

	if mismatchResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on client_id mismatch, got %d", mismatchResponseRecorder.Code)
	}

	// 6. Disabled Service Account -> 401 Unauthorized
	disabledServiceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "Disabled M2M SA",
		Scopes: []string{"data:read"},
	})
	if err != nil {
		t.Fatalf("failed to create disabled service account: %v", err)
	}
	disabledFalse := false
	_, err = serviceAccountManager.Update(ctx, disabledServiceAccount.ID, core.UpdateServiceAccountInput{
		IsEnabled: &disabledFalse,
	})
	if err != nil {
		t.Fatalf("failed to disable service account: %v", err)
	}
	disabledValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_secret": {disabledServiceAccount.SecretKey},
	}
	disabledRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(disabledValues.Encode()))
	disabledRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	disabledResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(disabledResponseRecorder, disabledRequest)

	if disabledResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on disabled service account, got %d", disabledResponseRecorder.Code)
	}

	// 7. Expired Service Account -> 401 Unauthorized
	pastTime := time.Now().UTC().Add(-24 * time.Hour)
	expiredServiceAccount, _ := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:      "Expired M2M SA",
		ExpiresAt: &pastTime,
	})
	expiredValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_secret": {expiredServiceAccount.SecretKey},
	}
	expiredRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(expiredValues.Encode()))
	expiredRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	expiredResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(expiredResponseRecorder, expiredRequest)

	if expiredResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on expired service account, got %d", expiredResponseRecorder.Code)
	}

	// 8. IP restricted Service Account -> 401 Unauthorized
	ipRestrictedServiceAccount, _ := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:       "IP Restricted M2M SA",
		AllowedIPs: []string{"10.0.0.1"},
	})
	ipValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_secret": {ipRestrictedServiceAccount.SecretKey},
	}
	ipRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(ipValues.Encode()))
	ipRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ipRequest.RemoteAddr = "192.168.1.5:1234"
	ipResponseRecorder := httptest.NewRecorder()
	handler.handleOIDCToken(ipResponseRecorder, ipRequest)

	if ipResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on blocked IP, got %d", ipResponseRecorder.Code)
	}

	// 9. Handler with nil signer -> 500
	nilSignerHandler := &Handler{
		serviceAccountManager: serviceAccountManager,
		signer:                nil,
	}
	nilSignerValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_secret": {activeServiceAccount.SecretKey},
	}
	nilSignerRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(nilSignerValues.Encode()))
	nilSignerRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	nilSignerResponseRecorder := httptest.NewRecorder()
	nilSignerHandler.handleOIDCToken(nilSignerResponseRecorder, nilSignerRequest)

	if nilSignerResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil signer, got %d", nilSignerResponseRecorder.Code)
	}

	// 10. Handler with uninitialized signer -> 500
	uninitSignerHandler := &Handler{
		serviceAccountManager: serviceAccountManager,
		signer:                &jwt.Signer{},
	}
	uninitSignerValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_secret": {activeServiceAccount.SecretKey},
	}
	uninitSignerRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(uninitSignerValues.Encode()))
	uninitSignerRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	uninitSignerResponseRecorder := httptest.NewRecorder()
	uninitSignerHandler.handleOIDCToken(uninitSignerResponseRecorder, uninitSignerRequest)

	if uninitSignerResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on uninitialized signer token error, got %d", uninitSignerResponseRecorder.Code)
	}
}
