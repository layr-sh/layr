package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"layr.sh/core"
	"uuid"
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
	baseHandler := NewHandler(db, configManager, cryptoKeyManager)
	baseHandler.SetKVStore(databaseKVStore)
	baseHandler.SetEventBus(eventBus)
	baseHandler.SetServiceAccountManager(serviceAccountManager)

	// 1. Create a registered test user directly in database
	testUserEmail := "user.oidc@example.com"
	testUserPassword := "SecurePassword123!"
	passwordHash, err := baseHandler.hasher.Hash(testUserPassword)
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
	authConfig.OIDC.UI = OIDCUIConfig{
		CustomCSS:    ".sign-in-card { border: 2px solid cyan; }",
		LogoURL:      "https://example.com/assets/logo.svg",
		ShowPassword: true,
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
	baseHandler.handleOIDCDiscovery(discoveryResponseRecorder, discoveryRequest)
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
	baseHandler.handleOIDCAuthorize(authorizeResponseRecorder, authorizeRequest)

	if authorizeResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK rendering sign-in page, got: %d (%s)", authorizeResponseRecorder.Code, authorizeResponseRecorder.Body.String())
	}

	htmlPage := authorizeResponseRecorder.Body.String()
	if !strings.Contains(htmlPage, ".sign-in-card { border: 2px solid cyan; }") {
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
	baseHandler.handleOIDCAuthorizeSubmit(tamperedResponseRecorder, tamperedRequest)
	if tamperedResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on tampered state, got: %d", tamperedResponseRecorder.Code)
	}

	_, _ = db.Exec(ctx, "UPDATE auth.users SET phone = '+15551234567', properties = '{\"name\":\"Test User Name\"}'::jsonb WHERE email = $1", testUserEmail)

	lockedPasswordHash, _ := baseHandler.hasher.Hash("LockedPass123!")
	_, _ = db.Exec(ctx, `
		INSERT INTO auth.users (email, password_hash, role, locked_until)
		VALUES ('locked.user@example.com', $1, 'authenticated', clock_timestamp() + interval '1 hour')
	`, lockedPasswordHash)

	// Unknown email
	unknownUserValues := url.Values{"state": {serverStateID}, "email": {"nonexistent@example.com"}, "password": {"SomePass123!"}}
	unknownUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(unknownUserValues.Encode()))
	unknownUserRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unknownUserResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(unknownUserResponseRecorder, unknownUserRequest)
	if !strings.Contains(unknownUserResponseRecorder.Body.String(), "Invalid email or password") {
		t.Fatalf("expected Invalid email or password on unknown user, got: %s", unknownUserResponseRecorder.Body.String())
	}

	// Locked user
	lockedUserValues := url.Values{"state": {serverStateID}, "email": {"locked.user@example.com"}, "password": {"LockedPass123!"}}
	lockedUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(lockedUserValues.Encode()))
	lockedUserRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	lockedUserResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(lockedUserResponseRecorder, lockedUserRequest)
	if !strings.Contains(lockedUserResponseRecorder.Body.String(), "Account temporarily locked") {
		t.Fatalf("expected Account temporarily locked on locked user, got: %s", lockedUserResponseRecorder.Body.String())
	}

	// Wrong password
	wrongPassValues := url.Values{"state": {serverStateID}, "email": {testUserEmail}, "password": {"WrongPassword!"}}
	wrongPassRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(wrongPassValues.Encode()))
	wrongPassRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wrongPassResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(wrongPassResponseRecorder, wrongPassRequest)
	if !strings.Contains(wrongPassResponseRecorder.Body.String(), "Invalid email or password") {
		t.Fatalf("expected Invalid email or password on wrong password, got: %s", wrongPassResponseRecorder.Body.String())
	}

	// MFA-enabled user tests
	mfaUserStateID := "mfa-user-state-id"
	mfaUserPayload, _ := json.Marshal(OIDCAuthorizationStatePayload{ClientID: "client-dashboard", RedirectURI: "https://dashboard.example.com/callback", Scope: "openid"})
	_ = databaseKVStore.Set(ctx, "auth:oidc:state:"+mfaUserStateID, string(mfaUserPayload), 5*time.Minute)

	mfaUserEmail := "mfa.user@example.com"
	mfaSecret := "JBSWY3DPEHPK3PXP"
	encryptedMFASecret, _ := cryptoKeyManager.EncryptField([]byte(mfaSecret))
	mfaPassHash, _ := baseHandler.hasher.Hash("MfaPass123!")
	var mfaUserID string
	_ = db.QueryRow(ctx, `
		INSERT INTO auth.users (email, password_hash, role, is_anonymous, mfa_enabled, encrypted_mfa_secret, created_at, last_updated_at)
		VALUES ($1, $2, 'authenticated', false, true, $3, clock_timestamp(), clock_timestamp())
		RETURNING id
	`, mfaUserEmail, mfaPassHash, encryptedMFASecret).Scan(&mfaUserID)

	// 1. Password submit with MFA enrolled -> Renders challenge page ("Two-factor authentication required")
	challengePageValues := url.Values{"state": {mfaUserStateID}, "email": {mfaUserEmail}, "password": {"MfaPass123!"}}
	challengePageRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(challengePageValues.Encode()))
	challengePageRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	challengePageResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(challengePageResponseRecorder, challengePageRequest)
	challengeHTML := challengePageResponseRecorder.Body.String()
	if !strings.Contains(challengeHTML, "Two-factor authentication required") {
		t.Fatalf("expected Two-factor authentication required, got: %s", challengeHTML)
	}

	// Extract mfa_token from challenge page
	mfaTokenPrefix := `name="mfa_token" value="`
	mfaTokenIdx := strings.Index(challengeHTML, mfaTokenPrefix)
	if mfaTokenIdx == -1 {
		t.Fatalf("mfa_token input not found in challenge page: %s", challengeHTML)
	}
	mfaTokenStart := mfaTokenIdx + len(mfaTokenPrefix)
	mfaTokenEnd := strings.Index(challengeHTML[mfaTokenStart:], `"`)
	extractedMFAToken := challengeHTML[mfaTokenStart : mfaTokenStart+mfaTokenEnd]

	// 2. Submit challenge form with empty code -> "Two-factor authentication code is required"
	emptyMFACodeValues := url.Values{
		"state":     {mfaUserStateID},
		"action":    {"verify_mfa"},
		"mfa_token": {extractedMFAToken},
		"mfa_code":  {""},
	}
	emptyMFACodeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(emptyMFACodeValues.Encode()))
	emptyMFACodeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyMFACodeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(emptyMFACodeResponseRecorder, emptyMFACodeRequest)
	if !strings.Contains(emptyMFACodeResponseRecorder.Body.String(), "Two-factor authentication code is required") {
		t.Fatalf("expected Two-factor authentication code is required, got: %s", emptyMFACodeResponseRecorder.Body.String())
	}

	// 3. Missing encrypted secret in DB -> "Multi-factor authentication configuration error"
	_, _ = db.Exec(ctx, "UPDATE auth.users SET encrypted_mfa_secret = NULL WHERE id = $1", mfaUserID)
	noSecretMFAValues := url.Values{"state": {mfaUserStateID}, "action": {"verify_mfa"}, "mfa_token": {extractedMFAToken}, "mfa_code": {"123456"}}
	noSecretMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(noSecretMFAValues.Encode()))
	noSecretMFARequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	noSecretMFAResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(noSecretMFAResponseRecorder, noSecretMFARequest)
	if !strings.Contains(noSecretMFAResponseRecorder.Body.String(), "Multi-factor authentication configuration error") {
		t.Fatalf("expected Multi-factor authentication configuration error, got: %s", noSecretMFAResponseRecorder.Body.String())
	}

	// 4. Corrupted encrypted secret -> "Failed to verify multi-factor authentication"
	_, _ = db.Exec(ctx, "UPDATE auth.users SET encrypted_mfa_secret = 'corrupted-secret' WHERE id = $1", mfaUserID)
	corruptedMFAValues := url.Values{"state": {mfaUserStateID}, "action": {"verify_mfa"}, "mfa_token": {extractedMFAToken}, "mfa_code": {"123456"}}
	corruptedMFARequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(corruptedMFAValues.Encode()))
	corruptedMFARequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	corruptedMFAResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(corruptedMFAResponseRecorder, corruptedMFARequest)
	if !strings.Contains(corruptedMFAResponseRecorder.Body.String(), "Failed to verify multi-factor authentication") {
		t.Fatalf("expected Failed to verify multi-factor authentication, got: %s", corruptedMFAResponseRecorder.Body.String())
	}

	// Restore encrypted secret
	_, _ = db.Exec(ctx, "UPDATE auth.users SET encrypted_mfa_secret = $1 WHERE id = $2", encryptedMFASecret, mfaUserID)

	// 5. Invalid MFA code -> "Invalid two-factor authentication code"
	invalidMFACodeValues := url.Values{"state": {mfaUserStateID}, "action": {"verify_mfa"}, "mfa_token": {extractedMFAToken}, "mfa_code": {"000000"}}
	invalidMFACodeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(invalidMFACodeValues.Encode()))
	invalidMFACodeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalidMFACodeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(invalidMFACodeResponseRecorder, invalidMFACodeRequest)
	if !strings.Contains(invalidMFACodeResponseRecorder.Body.String(), "Invalid two-factor authentication code") {
		t.Fatalf("expected Invalid two-factor authentication code, got: %s", invalidMFACodeResponseRecorder.Body.String())
	}

	// 6. Valid MFA code -> 302 Found redirect
	validTOTPCode, _ := baseHandler.GetTOTPManager().GenerateCode(mfaSecret, time.Now())
	validMFACodeValues := url.Values{"state": {mfaUserStateID}, "action": {"verify_mfa"}, "mfa_token": {extractedMFAToken}, "mfa_code": {validTOTPCode}}
	validMFACodeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(validMFACodeValues.Encode()))
	validMFACodeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	validMFACodeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(validMFACodeResponseRecorder, validMFACodeRequest)
	if validMFACodeResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 on valid MFA OIDC submit, got: %d", validMFACodeResponseRecorder.Code)
	}

	// Bad redirect URI in state payload
	badRedirectStateID := "bad-redirect-state-id"
	badRedirectPayload, _ := json.Marshal(OIDCAuthorizationStatePayload{ClientID: "client-dashboard", RedirectURI: "://invalid-url", Scope: "openid"})
	_ = databaseKVStore.Set(ctx, "auth:oidc:state:"+badRedirectStateID, string(badRedirectPayload), 5*time.Minute)
	badRedirectValues := url.Values{"state": {badRedirectStateID}, "email": {testUserEmail}, "password": {testUserPassword}}
	badRedirectRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(badRedirectValues.Encode()))
	badRedirectRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badRedirectResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(badRedirectResponseRecorder, badRedirectRequest)
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
	baseHandler.handleOIDCAuthorizeSubmit(submitResponseRecorder, submitRequest)

	if submitResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 Found redirect on valid sign-in, got: %d (%s)", submitResponseRecorder.Code, submitResponseRecorder.Body.String())
	}

	var sessionCookie *http.Cookie
	for _, cookie := range submitResponseRecorder.Result().Cookies() {
		if cookie.Name == core.SessionCookieNameInsecure || cookie.Name == core.SessionCookieNameSecure {
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
	baseHandler.handleOIDCAuthorizeSubmit(replayedSubmitResponseRecorder, submitRequest)
	if replayedSubmitResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on replaying deleted authorization state, got: %d", replayedSubmitResponseRecorder.Code)
	}

	// 6. Token Exchange (POST /api/v1/auth/oauth/token)
	_ = db.QueryRow(ctx, "SELECT id FROM auth.users WHERE email = $1", testUserEmail).Scan(&testUserID)

	// Test redirect_uri mismatch -> 400
	mismatchCode := baseHandler.issueOIDCAuthorizationCode(ctx, "client-dashboard", "https://dashboard.example.com/callback", testUserID, "openid profile email", codeChallenge, "S256", "nonce-mismatch")
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
	baseHandler.handleOIDCToken(mismatchResponseRecorder, mismatchRequest)
	if mismatchResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on redirect_uri mismatch, got: %d", mismatchResponseRecorder.Code)
	}

	// Test PKCE failure -> 400
	badPKCECode := baseHandler.issueOIDCAuthorizationCode(ctx, "client-dashboard", "https://dashboard.example.com/callback", testUserID, "openid profile email", codeChallenge, "S256", "nonce-badpkce")
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
	baseHandler.handleOIDCToken(badPKCEResponseRecorder, badPKCERequest)
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
	baseHandler.handleOIDCToken(tokenResponseRecorder, tokenRequest)

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
	baseHandler.handleOIDCToken(replayTokenResponseRecorder, replayTokenRequest)
	if replayTokenResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on authorization code replay, got: %d", replayTokenResponseRecorder.Code)
	}

	// 8. UserInfo Endpoint (GET /api/v1/auth/oauth/userinfo)
	tokenJWTClaims, _ := baseHandler.jwtSigner.VerifyAccessToken(tokenResponse.AccessToken)
	userinfoRequest := withUserAuthClaims(httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/auth/oauth/userinfo", nil), *tokenJWTClaims)
	userinfoRequest.Header.Set("Authorization", "Bearer "+tokenResponse.AccessToken)
	userinfoResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCUserInfo(userinfoResponseRecorder, userinfoRequest)

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
	ssoAuthorizeRequest := withUserAuth(httptest.NewRequestWithContext(ctx, http.MethodGet, ssoAuthorizeURL, nil), testUserID, "authenticated", false)
	ssoAuthorizeRequest.AddCookie(sessionCookie)
	ssoAuthorizeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorize(ssoAuthorizeResponseRecorder, ssoAuthorizeRequest)

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
	baseHandler.handleOIDCToken(mobileTokenResponseRecorder, mobileTokenRequest)

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
	baseHandler.handleOIDCToken(refreshResponseRecorder, refreshRequest)

	if refreshResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from refresh token grant, got: %d (%s)", refreshResponseRecorder.Code, refreshResponseRecorder.Body.String())
	}

	// 12. Sign-Out Endpoint (GET /api/v1/auth/oauth/sign-out)
	signOutRequestURL := "/api/v1/auth/oauth/sign-out?post_sign_out_redirect_uri=" + url.QueryEscape("https://dashboard.example.com/signed-out")
	signOutRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, signOutRequestURL, nil)
	signOutRequest.AddCookie(sessionCookie)
	signOutResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCSignOut(signOutResponseRecorder, signOutRequest)

	if signOutResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 Found on sign out redirect, got: %d", signOutResponseRecorder.Code)
	}
	if signOutResponseRecorder.Header().Get("Location") != "https://dashboard.example.com/signed-out" {
		t.Fatalf("expected redirect to post_sign_out_redirect_uri, got: %s", signOutResponseRecorder.Header().Get("Location"))
	}

	// Assert session cookie is cleared
	var clearedSessionCookie *http.Cookie
	for _, cookie := range signOutResponseRecorder.Result().Cookies() {
		if cookie.Name == core.SessionCookieNameInsecure || cookie.Name == core.SessionCookieNameSecure {
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
	baseHandler.handleOIDCToken(invalidRefreshResponseRecorder, invalidRefreshRequest)
	if invalidRefreshResponseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid refresh token, got: %d", invalidRefreshResponseRecorder.Code)
	}

	// a1. Locked user during refresh token exchange -> 400 invalid_grant
	lockedUserRefreshToken := baseHandler.jwtSigner.GenerateRefreshToken()
	lockedUserRefreshHash := baseHandler.jwtSigner.HashRefreshToken(lockedUserRefreshToken)
	var lockedUserID string
	_ = db.QueryRow(ctx, "SELECT id FROM auth.users WHERE email = 'locked.user@example.com'").Scan(&lockedUserID)
	_, _ = db.Exec(ctx, `
		INSERT INTO auth.sessions (id, user_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, clock_timestamp() + interval '1 hour', clock_timestamp())
	`, uuid.NewV7().String(), lockedUserID, lockedUserRefreshHash)

	lockedRefreshValues := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {"client-dashboard"},
		"client_secret": {confidentialSecret},
		"refresh_token": {lockedUserRefreshToken},
	}
	lockedRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(lockedRefreshValues.Encode()))
	lockedRefreshRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	lockedRefreshResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCToken(lockedRefreshResponseRecorder, lockedRefreshRequest)
	if lockedRefreshResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(lockedRefreshResponseRecorder.Body.String(), "Account is temporarily locked") {
		t.Fatalf("expected 400 Account is temporarily locked on locked user token refresh, got: %d (%s)", lockedRefreshResponseRecorder.Code, lockedRefreshResponseRecorder.Body.String())
	}

	// a2. User not found during refresh token exchange -> 400 invalid_grant
	_, err = db.Exec(ctx, `ALTER TABLE auth.sessions DROP CONSTRAINT IF EXISTS sessions_user_id_fkey`)
	if err != nil {
		t.Fatalf("failed to drop foreign key constraint: %v", err)
	}

	ghostRefreshToken := baseHandler.jwtSigner.GenerateRefreshToken()
	ghostRefreshHash := baseHandler.jwtSigner.HashRefreshToken(ghostRefreshToken)
	ghostUserID := uuid.NewV7().String()
	_, err = db.Exec(ctx, `
		INSERT INTO auth.sessions (id, user_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, clock_timestamp() + interval '1 hour', clock_timestamp())
	`, uuid.NewV7().String(), ghostUserID, ghostRefreshHash)
	if err != nil {
		t.Fatalf("failed to insert ghost session: %v", err)
	}

	ghostRefreshValues := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {"client-dashboard"},
		"client_secret": {confidentialSecret},
		"refresh_token": {ghostRefreshToken},
	}
	ghostRefreshRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(ghostRefreshValues.Encode()))
	ghostRefreshRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ghostRefreshResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCToken(ghostRefreshResponseRecorder, ghostRefreshRequest)
	if ghostRefreshResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(ghostRefreshResponseRecorder.Body.String(), "User not found") {
		t.Fatalf("expected 400 User not found on ghost user token refresh, got: %d (%s)", ghostRefreshResponseRecorder.Code, ghostRefreshResponseRecorder.Body.String())
	}
	_, _ = db.Exec(ctx, "DELETE FROM auth.sessions WHERE refresh_token_hash = $1", ghostRefreshHash)
	_, _ = db.Exec(ctx, `
		ALTER TABLE auth.sessions 
		ADD CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE
	`)

	// b. Deleted/nonexistent user in authorization code exchange -> 500
	orphanCode := baseHandler.issueOIDCAuthorizationCode(ctx, "client-dashboard", "https://dashboard.example.com/callback", "01918a24-9999-7000-8000-000000000099", "openid profile email", codeChallenge, "S256", "nonce-orphan")
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
	baseHandler.handleOIDCToken(orphanResponseRecorder, orphanRequest)
	if orphanResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on token exchange for nonexistent user, got: %d", orphanResponseRecorder.Code)
	}

	// c. Deleted/nonexistent user in userinfo -> 401 invalid_token
	ghostToken, _ := baseHandler.jwtSigner.GenerateAccessToken(core.JWTClaims{
		Subject: "01918a24-8888-7000-8000-000000000088",
		Email:   "ghost@example.com",
		Role:    "authenticated",
	}, 900)
	ghostCtx := core.WithAuthContext(ctx, core.AuthContext{
		UserID: "01918a24-8888-7000-8000-000000000088",
		JWT: core.JWTClaims{
			Subject: "01918a24-8888-7000-8000-000000000088",
			Email:   "ghost@example.com",
			Role:    "authenticated",
		},
	})
	ghostRequest := httptest.NewRequestWithContext(ghostCtx, http.MethodGet, "/api/v1/auth/oauth/userinfo", nil)
	ghostRequest.Header.Set("Authorization", "Bearer "+ghostToken)
	ghostResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCUserInfo(ghostResponseRecorder, ghostRequest)
	if ghostResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on userinfo for nonexistent user, got: %d", ghostResponseRecorder.Code)
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
	baseHandler.CompleteOAuthFlow(completeOIDCResponseRecorder, completeOIDCRequest, userRecord, oauthStatePayload, true)
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
	baseHandler := NewHandler(db, configManager, cryptoKeyManager)
	baseHandler.SetServiceAccountManager(serviceAccountManager)

	activeServiceAccount, err := serviceAccountManager.Create(ctx, core.CreateServiceAccountInput{
		Name:   "M2M Worker Service",
		Scopes: []string{"data:*", "auth:read"},
	})
	if err != nil {
		t.Fatalf("failed to create active service account: %v", err)
	}

	slugifier := core.NewSlugifier()
	expectedHandle := slugifier.Slugify(core.GetConfig().Project.Name)
	if expectedHandle == "" {
		expectedHandle = "layr"
	}
	layrAudience := expectedHandle + ":service_account"

	// 1. Basic Auth credentials flow (inherits all scopes)
	basicAuthValues := url.Values{
		"grant_type": {"client_credentials"},
		"audience":   {layrAudience},
	}
	basicAuthRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(basicAuthValues.Encode()))
	basicAuthRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	basicAuthRequest.SetBasicAuth(activeServiceAccount.ID, activeServiceAccount.SecretKey)
	basicAuthResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCToken(basicAuthResponseRecorder, basicAuthRequest)

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

	m2mJWTClaims, verifyErr := baseHandler.jwtSigner.VerifyM2MToken(basicOIDCTokenResponse.AccessToken)
	if verifyErr != nil {
		t.Fatalf("failed to verify M2M token: %v", verifyErr)
	}
	if m2mJWTClaims.Subject != activeServiceAccount.ID {
		t.Errorf("expected subject %s, got %s", activeServiceAccount.ID, m2mJWTClaims.Subject)
	}
	if m2mJWTClaims.Audience != layrAudience {
		t.Errorf("expected audience %s, got %s", layrAudience, m2mJWTClaims.Audience)
	}
	if len(m2mJWTClaims.Scopes()) != 2 {
		t.Errorf("expected 2 scopes, got %v", m2mJWTClaims.Scopes())
	}

	// 2. Form body credentials with scope down-scoping
	formValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {activeServiceAccount.KeyPrefix},
		"client_secret": {activeServiceAccount.SecretKey},
		"scope":         {"data:schema.read"},
		"audience":      {layrAudience},
	}
	formRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(formValues.Encode()))
	formRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	formResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCToken(formResponseRecorder, formRequest)

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
		"audience":      layrAudience,
	}
	jsonBytes, _ := json.Marshal(jsonBodyMap)
	jsonRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", bytes.NewReader(jsonBytes))
	jsonRequest.Header.Set("Content-Type", "application/json")
	jsonResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCToken(jsonResponseRecorder, jsonRequest)

	if jsonResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on JSON client_credentials, got %d (%s)", jsonResponseRecorder.Code, jsonResponseRecorder.Body.String())
	}

	// 4. Invalid scope exceeding permissions -> 400 Bad Request (invalid_scope)
	invalidScopeValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {activeServiceAccount.ID},
		"client_secret": {activeServiceAccount.SecretKey},
		"scope":         {"admin:super"},
		"audience":      {layrAudience},
	}
	invalidScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(invalidScopeValues.Encode()))
	invalidScopeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalidScopeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCToken(invalidScopeResponseRecorder, invalidScopeRequest)

	if invalidScopeResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(invalidScopeResponseRecorder.Body.String(), "invalid_scope") {
		t.Fatalf("expected 400 invalid_scope, got %d (%s)", invalidScopeResponseRecorder.Code, invalidScopeResponseRecorder.Body.String())
	}

	// 4b. Missing audience -> 400 Bad Request (invalid_request)
	missingAudienceValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {activeServiceAccount.ID},
		"client_secret": {activeServiceAccount.SecretKey},
	}
	missingAudienceRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(missingAudienceValues.Encode()))
	missingAudienceRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missingAudienceResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCToken(missingAudienceResponseRecorder, missingAudienceRequest)
	if missingAudienceResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(missingAudienceResponseRecorder.Body.String(), "audience parameter is required") {
		t.Fatalf("expected 400 invalid_request on missing audience, got %d (%s)", missingAudienceResponseRecorder.Code, missingAudienceResponseRecorder.Body.String())
	}

	// 4c. Unregistered audience -> 400 Bad Request (invalid_target)
	unregisteredAudienceValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {activeServiceAccount.ID},
		"client_secret": {activeServiceAccount.SecretKey},
		"audience":      {"https://unregistered.api.com"},
	}
	unregisteredAudienceRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(unregisteredAudienceValues.Encode()))
	unregisteredAudienceRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unregisteredAudienceResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCToken(unregisteredAudienceResponseRecorder, unregisteredAudienceRequest)
	if unregisteredAudienceResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(unregisteredAudienceResponseRecorder.Body.String(), "invalid_target") {
		t.Fatalf("expected 400 invalid_target on unregistered audience, got %d (%s)", unregisteredAudienceResponseRecorder.Code, unregisteredAudienceResponseRecorder.Body.String())
	}

	// 4d. Registered external Resource Server with valid external scope
	rsConfig := configManager.Get()
	rsConfig.OIDC.ResourceServers = []ResourceServerConfig{
		{
			Identifier: "https://billing.example.com",
			Scopes:     []string{"invoices:read", "invoices:write", "payments:read"},
		},
	}
	configManager.Set(rsConfig)

	rsValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {activeServiceAccount.ID},
		"client_secret": {activeServiceAccount.SecretKey},
		"audience":      {"https://billing.example.com"},
		"scope":         {"invoices:read"},
	}
	rsRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(rsValues.Encode()))
	rsRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rsResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCToken(rsResponseRecorder, rsRequest)
	if rsResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid RS client_credentials, got %d (%s)", rsResponseRecorder.Code, rsResponseRecorder.Body.String())
	}
	var rsOIDCTokenResponse OIDCTokenResponse
	_ = json.Unmarshal(rsResponseRecorder.Body.Bytes(), &rsOIDCTokenResponse)
	if rsOIDCTokenResponse.Scope != "invoices:read" {
		t.Fatalf("expected scope invoices:read, got %s", rsOIDCTokenResponse.Scope)
	}
	rsJWTClaims, rsVerifyErr := baseHandler.jwtSigner.VerifyM2MToken(rsOIDCTokenResponse.AccessToken)
	if rsVerifyErr != nil || rsJWTClaims.Audience != "https://billing.example.com" || !rsJWTClaims.HasScope("invoices:read") {
		t.Fatalf("unexpected RS token claims: %+v, err: %v", rsJWTClaims, rsVerifyErr)
	}

	// 4e. Registered external Resource Server with all scopes inherited when scope omitted
	rsAllValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {activeServiceAccount.ID},
		"client_secret": {activeServiceAccount.SecretKey},
		"audience":      {"https://billing.example.com"},
	}
	rsAllRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(rsAllValues.Encode()))
	rsAllRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rsAllResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCToken(rsAllResponseRecorder, rsAllRequest)
	if rsAllResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on RS all scopes, got %d (%s)", rsAllResponseRecorder.Code, rsAllResponseRecorder.Body.String())
	}
	var rsAllOIDCTokenResponse OIDCTokenResponse
	_ = json.Unmarshal(rsAllResponseRecorder.Body.Bytes(), &rsAllOIDCTokenResponse)
	rsAllJWTClaims, _ := baseHandler.jwtSigner.VerifyM2MToken(rsAllOIDCTokenResponse.AccessToken)
	if len(rsAllJWTClaims.Scopes()) != 3 {
		t.Fatalf("expected 3 RS scopes inherited, got: %v", rsAllJWTClaims.Scopes())
	}

	// 4f. Registered external Resource Server with undefined external scope -> 400 Bad Request (invalid_scope)
	rsInvalidScopeValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {activeServiceAccount.ID},
		"client_secret": {activeServiceAccount.SecretKey},
		"audience":      {"https://billing.example.com"},
		"scope":         {"invoices:delete"},
	}
	rsInvalidScopeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(rsInvalidScopeValues.Encode()))
	rsInvalidScopeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rsInvalidScopeResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCToken(rsInvalidScopeResponseRecorder, rsInvalidScopeRequest)
	if rsInvalidScopeResponseRecorder.Code != http.StatusBadRequest || !strings.Contains(rsInvalidScopeResponseRecorder.Body.String(), "invalid_scope") {
		t.Fatalf("expected 400 invalid_scope on undefined RS scope, got %d (%s)", rsInvalidScopeResponseRecorder.Code, rsInvalidScopeResponseRecorder.Body.String())
	}

	// 4g. Empty project name fallback audience "layr:service_account"
	serverConfig := core.DefaultConfig()
	serverConfig.Project.Name = ""
	core.SetLoadedConfig(serverConfig)
	emptyProjectAudienceValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {activeServiceAccount.ID},
		"client_secret": {activeServiceAccount.SecretKey},
		"audience":      {"layr:service_account"},
	}
	emptyProjectAudienceRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(emptyProjectAudienceValues.Encode()))
	emptyProjectAudienceRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emptyProjectAudienceResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCToken(emptyProjectAudienceResponseRecorder, emptyProjectAudienceRequest)
	core.UnloadConfig()
	if emptyProjectAudienceResponseRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on empty project name fallback audience, got %d (%s)", emptyProjectAudienceResponseRecorder.Code, emptyProjectAudienceResponseRecorder.Body.String())
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
	baseHandler.handleOIDCToken(mismatchResponseRecorder, mismatchRequest)

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
	baseHandler.handleOIDCToken(disabledResponseRecorder, disabledRequest)

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
	baseHandler.handleOIDCToken(expiredResponseRecorder, expiredRequest)

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
	baseHandler.handleOIDCToken(ipResponseRecorder, ipRequest)

	if ipResponseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on blocked IP, got %d", ipResponseRecorder.Code)
	}

	// 9. Handler with nil signer -> 500
	nilSignerBaseHandler := &Handler{
		serviceAccountManager: serviceAccountManager,
		configManager:         configManager,
		jwtSigner:             nil,
	}
	nilSignerValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_secret": {activeServiceAccount.SecretKey},
		"audience":      {layrAudience},
	}
	nilSignerRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(nilSignerValues.Encode()))
	nilSignerRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	nilSignerResponseRecorder := httptest.NewRecorder()
	nilSignerBaseHandler.handleOIDCToken(nilSignerResponseRecorder, nilSignerRequest)

	if nilSignerResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on nil signer, got %d", nilSignerResponseRecorder.Code)
	}

	// 10. Handler with uninitialized signer -> 500
	uninitSignerBaseHandler := &Handler{
		serviceAccountManager: serviceAccountManager,
		configManager:         configManager,
		jwtSigner:             &core.JWTSigner{},
	}
	uninitSignerValues := url.Values{
		"grant_type":    {"client_credentials"},
		"client_secret": {activeServiceAccount.SecretKey},
		"audience":      {layrAudience},
	}
	uninitSignerRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/token", strings.NewReader(uninitSignerValues.Encode()))
	uninitSignerRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	uninitSignerResponseRecorder := httptest.NewRecorder()
	uninitSignerBaseHandler.handleOIDCToken(uninitSignerResponseRecorder, uninitSignerRequest)

	if uninitSignerResponseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on uninitialized signer token error, got %d", uninitSignerResponseRecorder.Code)
	}
}

func TestAuthOIDCSignUpAndOTPIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(db, cryptoKeyManager)
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	driverWebhook := "webhook"
	authConfig := configManager.Get()
	authConfig.OIDC.Enabled = true
	authConfig.EmailOTP.Enabled = true
	authConfig.SMSOTP.Enabled = true
	authConfig.OIDC.UI.ShowSignUp = true
	authConfig.OIDC.UI.ShowPassword = true
	authConfig.OIDC.UI.ShowEmailOTP = true
	authConfig.OIDC.UI.ShowSMSOTP = true
	authConfig.EmailDispatcher = EmailDispatcherConfig{
		Driver:      &driverWebhook,
		SenderEmail: "auth@layr.sh",
		SenderName:  "Layr",
		Webhook: EmailDispatcherWebhookConfig{
			URL: "http://localhost:9999/webhook",
		},
	}
	authConfig.SMSDispatcher = SMSDispatcherConfig{
		Driver: &driverWebhook,
		Webhook: SMSDispatcherWebhookConfig{
			URL: "http://localhost:9999/webhook",
		},
	}
	authConfig.OIDC.Clients = []OIDCClientConfig{
		{
			Name:         "App Portal",
			ClientID:     "client-app-portal",
			RedirectURIs: []string{"https://portal.example.com/callback"},
			Public:       true,
			Scopes:       []string{"openid", "email"},
		},
	}
	if err := configManager.Save(ctx, authConfig); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	databaseKVStore := core.NewDatabaseKVStore(ctx, db, 60*time.Second)
	defer func() { _ = databaseKVStore.Close() }()
	baseHandler := NewHandler(db, configManager, cryptoKeyManager)
	baseHandler.SetKVStore(databaseKVStore)

	emailDispatcher := NewEmailDispatcher(db, func() *EmailDispatcherConfig {
		emailDispatcherConfig := configManager.Get().EmailDispatcher
		return &emailDispatcherConfig
	}, cryptoKeyManager)
	baseHandler.SetEmailDispatcher(emailDispatcher)

	smsDispatcher := NewSMSDispatcher(db, func() *SMSDispatcherConfig {
		smsDispatcherConfig := configManager.Get().SMSDispatcher
		return &smsDispatcherConfig
	}, cryptoKeyManager)
	baseHandler.SetSMSDispatcher(smsDispatcher)

	var capturedEvents []core.Event
	var eventsMutex sync.Mutex
	eventBus := core.NewEventBus(db, cryptoKeyManager)
	defer eventBus.Close()
	eventBus.Subscribe("*", func(_ context.Context, event core.Event) error {
		eventsMutex.Lock()
		capturedEvents = append(capturedEvents, event)
		eventsMutex.Unlock()
		return nil
	})
	baseHandler.SetEventBus(eventBus)

	// Helper to create valid OIDC state
	createState := func(stateID string) {
		payload, _ := json.Marshal(OIDCAuthorizationStatePayload{
			ClientID:    "client-app-portal",
			RedirectURI: "https://portal.example.com/callback",
			Scope:       "openid email",
		})
		_ = databaseKVStore.Set(ctx, "auth:oidc:state:"+stateID, string(payload), 5*time.Minute)
	}

	// 1. OIDC Sign-Up duplicate email error
	existingEmail := "existing.user@example.com"
	passHash, _ := baseHandler.hasher.Hash("Pass12345!")
	_, _ = db.Exec(ctx, `
		INSERT INTO auth.users (email, password_hash, role, created_at, last_updated_at)
		VALUES ($1, $2, 'authenticated', clock_timestamp(), clock_timestamp())
	`, existingEmail, passHash)

	st1 := "st-sign-up-dup"
	createState(st1)
	dupValues := url.Values{
		"state":            {st1},
		"action":           {"sign_up"},
		"email":            {existingEmail},
		"password":         {"Pass12345!"},
		"confirm_password": {"Pass12345!"},
	}
	dupRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(dupValues.Encode()))
	dupRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	dupResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(dupResponseRecorder, dupRequest)
	if !strings.Contains(dupResponseRecorder.Body.String(), "An account with this email already exists") {
		t.Fatalf("expected duplicate email error, got: %s", dupResponseRecorder.Body.String())
	}

	// 2. OIDC Sign-Up successful creation & redirect
	st2 := "st-sign-up-ok"
	createState(st2)
	newEmail := "new.user.oidc@example.com"
	okValues := url.Values{
		"state":            {st2},
		"action":           {"sign_up"},
		"email":            {newEmail},
		"password":         {"Pass12345!"},
		"confirm_password": {"Pass12345!"},
	}
	okRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(okValues.Encode()))
	okRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	okResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(okResponseRecorder, okRequest)
	if okResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect from sign up, got: %d (%s)", okResponseRecorder.Code, okResponseRecorder.Body.String())
	}
	if !strings.Contains(okResponseRecorder.Header().Get("Location"), "code=") {
		t.Fatalf("expected code parameter in location: %s", okResponseRecorder.Header().Get("Location"))
	}

	// 3. Password sign-in with direct MFA code error branches
	mfaUserEmail := "direct.mfa@example.com"
	mfaSecret := "JBSWY3DPEHPK3PXP"
	encSecret, _ := cryptoKeyManager.EncryptField([]byte(mfaSecret))
	var mfaUID string
	_ = db.QueryRow(ctx, `
		INSERT INTO auth.users (email, password_hash, role, mfa_enabled, encrypted_mfa_secret, created_at, last_updated_at)
		VALUES ($1, $2, 'authenticated', true, $3, clock_timestamp(), clock_timestamp())
		RETURNING id
	`, mfaUserEmail, passHash, encSecret).Scan(&mfaUID)

	// a. missing secret in DB
	st3a := "st-mfa-nosecret"
	createState(st3a)
	_, _ = db.Exec(ctx, "UPDATE auth.users SET encrypted_mfa_secret = NULL WHERE id = $1", mfaUID)
	noSecValues := url.Values{"state": {st3a}, "action": {"sign_in"}, "email": {mfaUserEmail}, "password": {"Pass12345!"}, "mfa_code": {"123456"}}
	noSecRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(noSecValues.Encode()))
	noSecRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	noSecResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(noSecResponseRecorder, noSecRequest)
	if !strings.Contains(noSecResponseRecorder.Body.String(), "Multi-factor authentication configuration error") {
		t.Fatalf("expected configuration error, got: %s", noSecResponseRecorder.Body.String())
	}

	// b. corrupted secret in DB
	st3b := "st-mfa-corrupt"
	createState(st3b)
	_, _ = db.Exec(ctx, "UPDATE auth.users SET encrypted_mfa_secret = 'corrupted' WHERE id = $1", mfaUID)
	corruptValues := url.Values{"state": {st3b}, "action": {"sign_in"}, "email": {mfaUserEmail}, "password": {"Pass12345!"}, "mfa_code": {"123456"}}
	corruptRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(corruptValues.Encode()))
	corruptRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	corruptResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(corruptResponseRecorder, corruptRequest)
	if !strings.Contains(corruptResponseRecorder.Body.String(), "Failed to verify multi-factor authentication") {
		t.Fatalf("expected failed verify error, got: %s", corruptResponseRecorder.Body.String())
	}

	// c. invalid mfa_code
	st3c := "st-mfa-invalid"
	createState(st3c)
	_, _ = db.Exec(ctx, "UPDATE auth.users SET encrypted_mfa_secret = $1 WHERE id = $2", encSecret, mfaUID)
	invValues := url.Values{"state": {st3c}, "action": {"sign_in"}, "email": {mfaUserEmail}, "password": {"Pass12345!"}, "mfa_code": {"000000"}}
	invRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(invValues.Encode()))
	invRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(invResponseRecorder, invRequest)
	if !strings.Contains(invResponseRecorder.Body.String(), "Invalid two-factor authentication code") {
		t.Fatalf("expected invalid code error, got: %s", invResponseRecorder.Body.String())
	}

	// 4. verify_mfa with user not found in DB
	st4 := "st-mfa-nouser"
	createState(st4)
	mfaTkNoUser := "mfa_oidc_nouser"
	_ = databaseKVStore.Set(ctx, "auth:oidc:mfa:"+mfaTkNoUser, "01918a24-9999-7000-8000-000000000000", 5*time.Minute)
	noUserValues := url.Values{"state": {st4}, "action": {"verify_mfa"}, "mfa_token": {mfaTkNoUser}, "mfa_code": {"123456"}}
	noUserRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(noUserValues.Encode()))
	noUserRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	noUserResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(noUserResponseRecorder, noUserRequest)
	if !strings.Contains(noUserResponseRecorder.Body.String(), "User account not found") {
		t.Fatalf("expected user account not found, got: %s", noUserResponseRecorder.Body.String())
	}

	// 5. Passwordless OTP in OIDC
	// a. invalid phone number
	st5a := "st-otp-phone-inv"
	createState(st5a)
	invPhoneValues := url.Values{"state": {st5a}, "action": {"send_otp"}, "recipient": {"not-a-phone"}}
	invPhoneRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(invPhoneValues.Encode()))
	invPhoneRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invPhoneResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(invPhoneResponseRecorder, invPhoneRequest)
	if !strings.Contains(invPhoneResponseRecorder.Body.String(), "Invalid phone number format") {
		t.Fatalf("expected invalid phone format error, got: %s", invPhoneResponseRecorder.Body.String())
	}

	// b. valid email send_otp -> renders otp_verify
	st5b := "st-otp-email-ok"
	createState(st5b)
	otpEmail := "otp.oidc.user@example.com"
	emailSendValues := url.Values{"state": {st5b}, "action": {"send_otp"}, "recipient": {otpEmail}}
	emailSendRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(emailSendValues.Encode()))
	emailSendRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	emailSendResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(emailSendResponseRecorder, emailSendRequest)
	if !strings.Contains(emailSendResponseRecorder.Body.String(), "Verification Code") {
		t.Fatalf("expected Verification Code on otp_verify page, got: %s", emailSendResponseRecorder.Body.String())
	}

	// c. IP rate limit on send_otp
	invIPRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(emailSendValues.Encode()))
	invIPRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invIPRequest.RemoteAddr = "10.0.0.99:1234"
	_ = databaseKVStore.Set(ctx, "auth:ratelimit:otp:ip:10.0.0.99", "11", time.Hour)
	invIPResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(invIPResponseRecorder, invIPRequest)
	if !strings.Contains(invIPResponseRecorder.Body.String(), "Rate limit exceeded") {
		t.Fatalf("expected rate limit error, got: %s", invIPResponseRecorder.Body.String())
	}

	// d. cooldown on send_otp
	_ = databaseKVStore.Delete(ctx, "auth:ratelimit:otp:ip:10.0.0.99")
	_ = databaseKVStore.Set(ctx, "auth:cooldown:sign_in:"+otpEmail, "1", time.Minute)
	coolRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(emailSendValues.Encode()))
	coolRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	coolResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(coolResponseRecorder, coolRequest)
	if !strings.Contains(coolResponseRecorder.Body.String(), "Please wait 60 seconds") {
		t.Fatalf("expected cooldown error, got: %s", coolResponseRecorder.Body.String())
	}
	_ = databaseKVStore.Delete(ctx, "auth:cooldown:sign_in:"+otpEmail)

	// e. verify_otp with invalid code
	invVerifyValues := url.Values{"state": {st5b}, "action": {"verify_otp"}, "recipient": {otpEmail}, "otp_code": {"999999"}}
	invVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(invVerifyValues.Encode()))
	invVerifyRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(invVerifyResponseRecorder, invVerifyRequest)
	if !strings.Contains(invVerifyResponseRecorder.Body.String(), "Invalid or expired verification code") {
		t.Fatalf("expected invalid code error, got: %s", invVerifyResponseRecorder.Body.String())
	}

	// f. verify_otp with valid code -> 302 Found redirect
	otpCode, _ := databaseKVStore.Get(ctx, "auth:otp:sign_in:"+otpEmail)
	okVerifyValues := url.Values{"state": {st5b}, "action": {"verify_otp"}, "recipient": {otpEmail}, "otp_code": {otpCode}}
	okVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(okVerifyValues.Encode()))
	okVerifyRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	okResponseRecorder = httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(okResponseRecorder, okVerifyRequest)
	if okResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 on valid OTP verify, got: %d (%s)", okResponseRecorder.Code, okResponseRecorder.Body.String())
	}

	// g. send_otp & verify_otp phone
	st5g := "st-otp-phone-ok"
	createState(st5g)
	otpPhone := "+14155553333"
	phoneSendValues := url.Values{"state": {st5g}, "action": {"send_otp"}, "recipient": {otpPhone}}
	phoneSendRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(phoneSendValues.Encode()))
	phoneSendRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	phoneSendResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(phoneSendResponseRecorder, phoneSendRequest)
	if !strings.Contains(phoneSendResponseRecorder.Body.String(), "Verification Code") {
		t.Fatalf("expected Verification Code for phone, got: %s", phoneSendResponseRecorder.Body.String())
	}

	phoneCode, _ := databaseKVStore.Get(ctx, "auth:otp:sign_in:"+otpPhone)
	phoneVerifyValues := url.Values{"state": {st5g}, "action": {"verify_otp"}, "recipient": {otpPhone}, "otp_code": {phoneCode}}
	phoneVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(phoneVerifyValues.Encode()))
	phoneVerifyRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	phoneVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(phoneVerifyResponseRecorder, phoneVerifyRequest)
	if phoneVerifyResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 on valid phone OTP verify, got: %d", phoneVerifyResponseRecorder.Code)
	}

	// h. verify_otp when user has MFA enabled -> renders MFA challenge page
	mfaOTPUser := "mfa.otp.user@example.com"
	_, _ = db.Exec(ctx, `
		INSERT INTO auth.users (email, role, mfa_enabled, encrypted_mfa_secret, created_at, last_updated_at)
		VALUES ($1, 'authenticated', true, $2, clock_timestamp(), clock_timestamp())
	`, mfaOTPUser, encSecret)
	st5h := "st-otp-mfa"
	createState(st5h)
	mfaOTPSendValues := url.Values{"state": {st5h}, "action": {"send_otp"}, "recipient": {mfaOTPUser}}
	mfaOTPSendRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(mfaOTPSendValues.Encode()))
	mfaOTPSendRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mfaOTPSendResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(mfaOTPSendResponseRecorder, mfaOTPSendRequest)

	mfaOTPCode, _ := databaseKVStore.Get(ctx, "auth:otp:sign_in:"+mfaOTPUser)
	mfaOTPVerifyValues := url.Values{"state": {st5h}, "action": {"verify_otp"}, "recipient": {mfaOTPUser}, "otp_code": {mfaOTPCode}}
	mfaOTPVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(mfaOTPVerifyValues.Encode()))
	mfaOTPVerifyRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mfaOTPVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(mfaOTPVerifyResponseRecorder, mfaOTPVerifyRequest)
	if !strings.Contains(mfaOTPVerifyResponseRecorder.Body.String(), "Two-factor authentication required") {
		t.Fatalf("expected Two-factor authentication required for OTP user with MFA, got: %s", mfaOTPVerifyResponseRecorder.Body.String())
	}

	// i. verify_otp when no OTP record exists in DB -> Invalid or expired verification code
	st5i := "st-otp-no-record"
	createState(st5i)
	noOTPVerifyValues := url.Values{"state": {st5i}, "action": {"verify_otp"}, "recipient": {"no-otp@example.com"}, "otp_code": {"123456"}}
	noOTPVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(noOTPVerifyValues.Encode()))
	noOTPVerifyRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	noOTPVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(noOTPVerifyResponseRecorder, noOTPVerifyRequest)
	if !strings.Contains(noOTPVerifyResponseRecorder.Body.String(), "Invalid or expired verification code") {
		t.Fatalf("expected invalid code error for non-existent OTP, got: %s", noOTPVerifyResponseRecorder.Body.String())
	}

	// j. verify_otp for existing locked user -> Account temporarily locked
	lockedUser := "locked.otp.user@example.com"
	_, _ = db.Exec(ctx, `
		INSERT INTO auth.users (email, role, locked_until, created_at, last_updated_at)
		VALUES ($1, 'authenticated', clock_timestamp() + interval '1 hour', clock_timestamp(), clock_timestamp())
	`, lockedUser)
	stLocked := "st-otp-locked"
	createState(stLocked)
	lockedSendValues := url.Values{"state": {stLocked}, "action": {"send_otp"}, "recipient": {lockedUser}}
	lockedSendRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(lockedSendValues.Encode()))
	lockedSendRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	lockedSendResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(lockedSendResponseRecorder, lockedSendRequest)

	lockedCode, _ := databaseKVStore.Get(ctx, "auth:otp:sign_in:"+lockedUser)
	lockedVerifyValues := url.Values{"state": {stLocked}, "action": {"verify_otp"}, "recipient": {lockedUser}, "otp_code": {lockedCode}}
	lockedVerifyRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(lockedVerifyValues.Encode()))
	lockedVerifyRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	lockedVerifyResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(lockedVerifyResponseRecorder, lockedVerifyRequest)
	if !strings.Contains(lockedVerifyResponseRecorder.Body.String(), "Account temporarily locked. Please try again later.") {
		t.Fatalf("expected locked error for locked OTP user, got: %s", lockedVerifyResponseRecorder.Body.String())
	}

	// k. sign_up DB insert failure (constraint violation) -> Failed to create account
	_, constraintErr := db.Exec(ctx, "ALTER TABLE auth.users ADD CONSTRAINT test_oidc_sign_up_fail CHECK (email != 'fail.sign_up@example.com')")
	if constraintErr != nil {
		t.Fatalf("failed to add constraint: %v", constraintErr)
	}
	defer func() { _, _ = db.Exec(ctx, "ALTER TABLE auth.users DROP CONSTRAINT IF EXISTS test_oidc_sign_up_fail") }()

	stFail := "st-sign-up-fail"
	createState(stFail)
	failSignUpValues := url.Values{
		"state":            {stFail},
		"action":           {"sign_up"},
		"email":            {"fail.sign_up@example.com"},
		"password":         {"SecurePass123!"},
		"confirm_password": {"SecurePass123!"},
	}
	failSignUpRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/oauth/authorize", strings.NewReader(failSignUpValues.Encode()))
	failSignUpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	failSignUpResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCAuthorizeSubmit(failSignUpResponseRecorder, failSignUpRequest)
	if !strings.Contains(failSignUpResponseRecorder.Body.String(), "Failed to create account") {
		t.Fatalf("expected Failed to create account error, got: %s", failSignUpResponseRecorder.Body.String())
	}
}

func TestAuthOIDCFederatedSignOutBackChannelIntegration(t *testing.T) {
	db, cryptoKeyManager, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	databaseKVStore := core.NewDatabaseKVStore(ctx, db, 60*time.Second)
	defer func() { _ = databaseKVStore.Close() }()
	eventBus := core.NewEventBus(db, cryptoKeyManager)
	serviceAccountManager := core.NewServiceAccountManager(db)

	var receivedTokenMutex sync.Mutex
	var receivedSignOutToken string
	mockRPServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		bodyBytes, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			responseWriter.WriteHeader(http.StatusBadRequest)
			return
		}
		parsedValues, parseErr := url.ParseQuery(string(bodyBytes))
		if parseErr != nil {
			responseWriter.WriteHeader(http.StatusBadRequest)
			return
		}
		receivedTokenMutex.Lock()
		receivedSignOutToken = parsedValues.Get("logout_token")
		receivedTokenMutex.Unlock()
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer mockRPServer.Close()

	configManager := NewConfigManager(db, cryptoKeyManager)
	authConfig := configManager.Get()
	authConfig.OIDC.Enabled = true
	authConfig.OIDC.Clients = []OIDCClientConfig{
		{
			ClientID:                "client-other-rp",
			Name:                    "Other RP",
			PostSignOutRedirectURIs: []string{"https://other.example.com/signed-out"},
		},
		{
			ClientID:                          "client-federated-rp",
			Name:                              "Federated RP",
			BackChannelSignOutURI:             mockRPServer.URL,
			BackChannelSignOutSessionRequired: true,
			PostSignOutRedirectURIs:           []string{"https://rp.example.com/signed-out"},
		},
	}
	if saveErr := configManager.Save(ctx, authConfig); saveErr != nil {
		t.Fatalf("failed to save config: %v", saveErr)
	}

	baseHandler := NewHandler(db, configManager, cryptoKeyManager)
	baseHandler.SetKVStore(databaseKVStore)
	baseHandler.SetEventBus(eventBus)
	baseHandler.SetServiceAccountManager(serviceAccountManager)
	baseHandler.SetHTTPClient(mockRPServer.Client())

	var testUserID string
	err := db.QueryRow(ctx, `
		INSERT INTO auth.users (email, role, is_anonymous, created_at, last_updated_at)
		VALUES ('federated.user@example.com', 'authenticated', false, clock_timestamp(), clock_timestamp())
		RETURNING id
	`).Scan(&testUserID)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	rawSessionToken := "federated-raw-session-token-abc"
	hashedToken := baseHandler.jwtSigner.HashRefreshToken(rawSessionToken)
	var sessionID string
	err = db.QueryRow(ctx, `
		INSERT INTO auth.sessions (user_id, client_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at)
		VALUES ($1, 'client-federated-rp', $2, '127.0.0.1', 'IntegrationTest', clock_timestamp() + interval '7 days', clock_timestamp())
		RETURNING id
	`, testUserID, hashedToken).Scan(&sessionID)
	if err != nil {
		t.Fatalf("failed to insert session: %v", err)
	}

	signOutRequestURL := "/api/v1/auth/oauth/sign-out?client_id=client-federated-rp&post_sign_out_redirect_uri=" + url.QueryEscape("https://rp.example.com/signed-out")
	signOutRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, signOutRequestURL, nil)
	signOutRequest.AddCookie(&http.Cookie{
		Name:  core.SessionCookieNameInsecure,
		Value: rawSessionToken,
	})
	signOutResponseRecorder := httptest.NewRecorder()
	baseHandler.handleOIDCSignOut(signOutResponseRecorder, signOutRequest)

	if signOutResponseRecorder.Code != http.StatusFound {
		t.Fatalf("expected 302 Found, got: %d (%s)", signOutResponseRecorder.Code, signOutResponseRecorder.Body.String())
	}
	if signOutResponseRecorder.Header().Get("Location") != "https://rp.example.com/signed-out" {
		t.Fatalf("expected redirect to post sign out URI, got: %s", signOutResponseRecorder.Header().Get("Location"))
	}

	var activeSessionCount int
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM auth.sessions WHERE user_id = $1", testUserID).Scan(&activeSessionCount)
	if activeSessionCount != 0 {
		t.Fatalf("expected 0 active sessions after sign out, got: %d", activeSessionCount)
	}

	receivedTokenMutex.Lock()
	capturedToken := receivedSignOutToken
	receivedTokenMutex.Unlock()

	if capturedToken == "" {
		t.Fatal("expected mock RP to receive back-channel sign-out request with logout_token")
	}

	verifiedSignOutJWTClaims, verifyErr := baseHandler.jwtSigner.VerifySignOutToken(capturedToken)
	if verifyErr != nil {
		t.Fatalf("failed to verify received sign-out token: %v", verifyErr)
	}
	if verifiedSignOutJWTClaims.Subject != testUserID {
		t.Fatalf("expected subject %s, got: %s", testUserID, verifiedSignOutJWTClaims.Subject)
	}
	if verifiedSignOutJWTClaims.Audience != "client-federated-rp" {
		t.Fatalf("expected audience client-federated-rp, got: %s", verifiedSignOutJWTClaims.Audience)
	}
	if verifiedSignOutJWTClaims.SessionID != sessionID {
		t.Fatalf("expected session ID %s, got: %s", sessionID, verifiedSignOutJWTClaims.SessionID)
	}
	if verifiedSignOutJWTClaims.Events[core.SignOutTokenEventURI] == nil {
		t.Fatalf("missing backchannel logout event claim in verified token")
	}
}
