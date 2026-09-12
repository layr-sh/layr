package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"uuid"

	"layr.sh/auth/jwt"
	"layr.sh/core"
)

const defaultM2MTokenExpirySeconds = 3600

// OIDCTokenResponse represents the standard OAuth 2.0 / OIDC token response.
type OIDCTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// OAuthTokenRequest represents incoming parameters for OAuth 2.0 token requests.
type OAuthTokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	CodeVerifier string `json:"code_verifier"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	Provider     string `json:"provider"`
}

// OIDCUserInfoResponse represents OpenID Connect Core 1.0 standard claims.
type OIDCUserInfoResponse struct {
	Subject             string         `json:"sub"`
	Name                string         `json:"name,omitempty"`
	Email               *string        `json:"email,omitempty"`
	EmailVerified       bool           `json:"email_verified"`
	PhoneNumber         *string        `json:"phone_number,omitempty"`
	PhoneNumberVerified bool           `json:"phone_number_verified"`
	Role                string         `json:"role"`
	IsAnonymous         bool           `json:"is_anonymous"`
	UpdatedAt           int64          `json:"updated_at"`
	Properties          map[string]any `json:"properties,omitempty"`
}

// 1. OIDC Discovery & JWKS

func (handler *Handler) handleOIDCDiscovery(responseWriter http.ResponseWriter, request *http.Request) {
	baseURL := core.GetConfig().ServerBaseURL()
	oidcConfiguration := jwt.BuildOIDCDiscovery(baseURL)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(oidcConfiguration)
}

func (handler *Handler) handleJWKS(responseWriter http.ResponseWriter, request *http.Request) {
	jwks := handler.signer.BuildJWKS()
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(jwks)
}

// 2. Authorization Endpoint (GET /api/v1/auth/oauth/authorize)

func (handler *Handler) handleOIDCAuthorize(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.OIDC.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "OIDC Identity Provider is disabled by the console user", "oidc_disabled")
		return
	}

	clientID := request.URL.Query().Get("client_id")
	redirectURI := request.URL.Query().Get("redirect_uri")
	responseType := request.URL.Query().Get("response_type")
	clientState := request.URL.Query().Get("state")
	codeChallenge := request.URL.Query().Get("code_challenge")
	codeChallengeMethod := request.URL.Query().Get("code_challenge_method")
	scope := request.URL.Query().Get("scope")
	nonce := request.URL.Query().Get("nonce")

	if clientID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Missing client_id parameter", "invalid_client")
		return
	}

	oidcClientConfig, ok := handler.configManager.GetOIDCClient(clientID)
	if !ok {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Unknown client_id '%s'", clientID), "invalid_client")
		return
	}

	if redirectURI == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Missing redirect_uri parameter", "invalid_request")
		return
	}

	validRedirect := false
	for _, registeredURI := range oidcClientConfig.RedirectURIs {
		if registeredURI == redirectURI {
			validRedirect = true
			break
		}
	}
	if !validRedirect {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Unauthorized redirect_uri", "invalid_request")
		return
	}

	if responseType != "code" {
		redirectError(responseWriter, request, redirectURI, "unsupported_response_type", "Response type must be 'code'", clientState)
		return
	}

	if codeChallenge == "" || codeChallengeMethod != "S256" {
		redirectError(responseWriter, request, redirectURI, "invalid_request", "PKCE code_challenge with S256 method is required", clientState)
		return
	}

	// Active session bypass (Single Sign-On)
	activeUserID, err := handler.authenticateUser(request)
	if err == nil && activeUserID != "" {
		code := handler.issueOIDCAuthorizationCode(request.Context(), clientID, redirectURI, activeUserID, scope, codeChallenge, codeChallengeMethod, nonce)
		targetURL, parseErr := url.Parse(redirectURI)
		if parseErr == nil {
			queryValues := targetURL.Query()
			queryValues.Set("code", code)
			if clientState != "" {
				queryValues.Set("state", clientState)
			}
			targetURL.RawQuery = queryValues.Encode()
			http.Redirect(responseWriter, request, targetURL.String(), http.StatusFound)
			return
		}
	}

	// Generate and store secure authorization state payload (10-minute TTL)
	stateID := uuid.NewV7().String()
	oidcAuthorizationStatePayload := OIDCAuthorizationStatePayload{
		StateID:             stateID,
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scope:               scope,
		ClientState:         clientState,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		Nonce:               nonce,
		CreatedAt:           time.Now().UTC(),
	}

	if handler.kvStore != nil {
		payloadJSON, _ := json.Marshal(oidcAuthorizationStatePayload)
		_ = handler.kvStore.Set(request.Context(), "auth:oidc:state:"+stateID, string(payloadJSON), 10*time.Minute)
	}

	handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "")
}

// 3. Authorization Form Submission (POST /api/v1/auth/oauth/authorize)

func (handler *Handler) handleOIDCAuthorizeSubmit(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.OIDC.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "OIDC Identity Provider is disabled by the console user", "oidc_disabled")
		return
	}

	if err := request.ParseForm(); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid form data", "invalid_request")
		return
	}

	stateID := request.FormValue("state")
	if stateID == "" || handler.kvStore == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Authorization session expired or invalid", "invalid_request")
		return
	}

	stateJSON, err := handler.kvStore.Get(request.Context(), "auth:oidc:state:"+stateID)
	if err != nil || stateJSON == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Authorization session expired or invalid", "invalid_request")
		return
	}

	var oidcAuthorizationStatePayload OIDCAuthorizationStatePayload
	if unmarshalErr := json.Unmarshal([]byte(stateJSON), &oidcAuthorizationStatePayload); unmarshalErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Failed to read authorization state", "invalid_request")
		return
	}

	oidcClientConfig, _ := handler.configManager.GetOIDCClient(oidcAuthorizationStatePayload.ClientID)

	email := strings.ToLower(strings.TrimSpace(request.FormValue("email")))
	userPassword := request.FormValue("password")
	if email == "" || userPassword == "" {
		handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Email and password are required")
		return
	}

	// Verify user credentials
	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	query := `
		SELECT id, email, phone, password_hash, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		FROM auth.users
		WHERE email = $1
	`
	scanErr := handler.db.QueryRow(ctx, query, email).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.PasswordHash,
		&userRecord.Role, &userRecord.IsAnonymous, &userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt,
		&userRecord.LockedUntil, &rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if scanErr != nil || userRecord.PasswordHash == nil {
		handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Invalid email or password")
		return
	}

	if userRecord.LockedUntil != nil && userRecord.LockedUntil.After(time.Now().UTC()) {
		handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Account temporarily locked. Please try again later.")
		return
	}

	match, verifyErr := handler.hasher.Verify(userPassword, *userRecord.PasswordHash)
	if verifyErr != nil || !match {
		handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Invalid email or password")
		return
	}

	// Delete state payload to prevent replay / CSRF fixation
	_ = handler.kvStore.Delete(ctx, "auth:oidc:state:"+stateID)

	// Issue authorization code
	code := handler.issueOIDCAuthorizationCode(ctx, oidcAuthorizationStatePayload.ClientID, oidcAuthorizationStatePayload.RedirectURI, userRecord.ID, oidcAuthorizationStatePayload.Scope, oidcAuthorizationStatePayload.CodeChallenge, oidcAuthorizationStatePayload.CodeChallengeMethod, oidcAuthorizationStatePayload.Nonce)

	// Set browser session cookie for SSO
	refreshToken := jwt.GenerateRefreshToken()
	refreshTokenHash := jwt.HashRefreshToken(refreshToken)
	expiresAt := time.Now().UTC().Add(time.Duration(config.Sessions.RefreshTokenExpirySeconds) * time.Second)
	sessionID := uuid.NewV7().String()
	_, _ = handler.db.Exec(ctx, `
		INSERT INTO auth.sessions (id, user_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, clock_timestamp())
	`, sessionID, userRecord.ID, refreshTokenHash, expiresAt)

	isSecure := core.IsSecureRequest(request)
	core.SetSessionCookie(responseWriter, AuthSessionCookieName, AuthSessionInsecureCookieName, refreshToken, expiresAt, isSecure)

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewSessionCreatedEvent(sessionID, SessionCreatedEventData{
			ID:        sessionID,
			UserID:    userRecord.ID,
			ExpiresAt: expiresAt,
			CreatedAt: time.Now().UTC(),
		}))
	}

	// 302 Found redirect back to client redirect_uri
	targetURL, parseErr := url.Parse(oidcAuthorizationStatePayload.RedirectURI)
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid redirect_uri", "invalid_request")
		return
	}
	queryValues := targetURL.Query()
	queryValues.Set("code", code)
	if oidcAuthorizationStatePayload.ClientState != "" {
		queryValues.Set("state", oidcAuthorizationStatePayload.ClientState)
	}
	targetURL.RawQuery = queryValues.Encode()
	http.Redirect(responseWriter, request, targetURL.String(), http.StatusFound)
}

func parseOAuthTokenRequest(request *http.Request) OAuthTokenRequest {
	var oauthTokenRequest OAuthTokenRequest

	if strings.Contains(request.Header.Get("Content-Type"), "application/json") && request.Body != nil {
		bodyBytes, readErr := io.ReadAll(request.Body)
		if readErr == nil {
			_ = json.Unmarshal(bodyBytes, &oauthTokenRequest)
			request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	} else {
		_ = request.ParseForm()
		if request.FormValue("grant_type") == "" && request.Body != nil {
			bodyBytes, readErr := io.ReadAll(request.Body)
			if readErr == nil {
				_ = json.Unmarshal(bodyBytes, &oauthTokenRequest)
				request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}
	}

	if oauthTokenRequest.GrantType == "" {
		oauthTokenRequest.GrantType = request.FormValue("grant_type")
	}
	if oauthTokenRequest.ClientID == "" {
		oauthTokenRequest.ClientID = request.FormValue("client_id")
	}
	if oauthTokenRequest.ClientSecret == "" {
		oauthTokenRequest.ClientSecret = request.FormValue("client_secret")
	}
	if oauthTokenRequest.Code == "" {
		oauthTokenRequest.Code = request.FormValue("code")
	}
	if oauthTokenRequest.RedirectURI == "" {
		oauthTokenRequest.RedirectURI = request.FormValue("redirect_uri")
	}
	if oauthTokenRequest.CodeVerifier == "" {
		oauthTokenRequest.CodeVerifier = request.FormValue("code_verifier")
	}
	if oauthTokenRequest.RefreshToken == "" {
		oauthTokenRequest.RefreshToken = request.FormValue("refresh_token")
	}
	if oauthTokenRequest.Scope == "" {
		oauthTokenRequest.Scope = request.FormValue("scope")
	}
	if oauthTokenRequest.Provider == "" {
		oauthTokenRequest.Provider = request.FormValue("provider")
	}

	basicClientID, basicClientSecret, hasBasicAuth := request.BasicAuth()
	if hasBasicAuth && (basicClientID != "" || basicClientSecret != "") {
		oauthTokenRequest.ClientID = basicClientID
		oauthTokenRequest.ClientSecret = basicClientSecret
	}

	return oauthTokenRequest
}

// 4. Token Endpoint (POST /api/v1/auth/oauth/token)

func (handler *Handler) handleOIDCToken(responseWriter http.ResponseWriter, request *http.Request) {
	oauthTokenRequest := parseOAuthTokenRequest(request)

	// If no grant_type or if provider parameter is present, delegate to handleOAuthCallback for compatibility
	if oauthTokenRequest.GrantType == "" || oauthTokenRequest.Provider != "" {
		handler.HandleOAuthCallback(responseWriter, request)
		return
	}

	if oauthTokenRequest.GrantType == "client_credentials" {
		handler.handleOAuthClientCredentials(responseWriter, request, oauthTokenRequest)
		return
	}

	config := handler.configManager.Get()
	if !config.OIDC.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "OIDC Identity Provider is disabled by the console user", "oidc_disabled")
		return
	}

	if oauthTokenRequest.GrantType == "authorization_code" {
		handler.handleOIDCTokenAuthorizationCode(responseWriter, request)
		return
	}

	if oauthTokenRequest.GrantType == "refresh_token" {
		handler.handleOIDCTokenRefreshToken(responseWriter, request)
		return
	}

	core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Unsupported grant_type '%s'", oauthTokenRequest.GrantType), "unsupported_grant_type")
}

func (handler *Handler) handleOAuthClientCredentials(responseWriter http.ResponseWriter, request *http.Request, oauthTokenRequest OAuthTokenRequest) {
	clientID := oauthTokenRequest.ClientID
	clientSecret := oauthTokenRequest.ClientSecret

	if clientSecret == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid client credentials", "invalid_client")
		return
	}

	if handler.serviceAccountManager == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid client credentials", "invalid_client")
		return
	}

	clientIP := core.ExtractRequestClientIP(request)
	serviceAccount, authErr := handler.serviceAccountManager.Authenticate(request.Context(), clientSecret, clientIP)
	if authErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid client credentials", "invalid_client")
		return
	}

	if clientID != "" && serviceAccount.ID != clientID && serviceAccount.KeyPrefix != clientID {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid client credentials", "invalid_client")
		return
	}

	var grantedScopes []string
	trimmedScope := strings.TrimSpace(oauthTokenRequest.Scope)
	if trimmedScope != "" {
		requestedScopes := strings.Fields(trimmedScope)
		for _, requestedScope := range requestedScopes {
			if !core.HasScope(serviceAccount.Scopes, requestedScope) {
				core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "The requested scope exceeds permissions granted to the client", "invalid_scope")
				return
			}
		}
		grantedScopes = requestedScopes
	} else {
		grantedScopes = serviceAccount.Scopes
	}

	if handler.signer == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Token signer not available", "server_error")
		return
	}

	accessToken, tokenErr := handler.signer.GenerateM2MToken(serviceAccount.ID, grantedScopes, defaultM2MTokenExpirySeconds)
	if tokenErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to generate M2M access token", "server_error")
		return
	}

	oidcTokenResponse := OIDCTokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   defaultM2MTokenExpirySeconds,
		Scope:       strings.Join(grantedScopes, " "),
	}

	responseWriter.Header().Set("Content-Type", "application/json;charset=UTF-8")
	responseWriter.Header().Set("Cache-Control", "no-store")
	responseWriter.Header().Set("Pragma", "no-cache")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(oidcTokenResponse)
}

// handleOAuthToken delegates to handleOIDCToken for OAuth 2.0 token requests.
func (handler *Handler) handleOAuthToken(responseWriter http.ResponseWriter, request *http.Request) {
	handler.handleOIDCToken(responseWriter, request)
}

func (handler *Handler) handleOIDCTokenAuthorizationCode(responseWriter http.ResponseWriter, request *http.Request) {
	clientID, clientSecret, hasBasicAuth := request.BasicAuth()
	if !hasBasicAuth {
		clientID = request.FormValue("client_id")
		clientSecret = request.FormValue("client_secret")
	}

	if clientID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Missing client credentials", "invalid_client")
		return
	}

	oidcClientConfig, ok := handler.configManager.GetOIDCClient(clientID)
	if !ok {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid client credentials", "invalid_client")
		return
	}

	// Verify confidential client secret
	if !oidcClientConfig.Public {
		decryptedSecret, _ := handler.configManager.DecryptSecret(oidcClientConfig.ClientSecret)
		if clientSecret == "" || clientSecret != decryptedSecret {
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid client secret", "invalid_client")
			return
		}
	}

	code := request.FormValue("code")
	redirectURI := request.FormValue("redirect_uri")
	codeVerifier := request.FormValue("code_verifier")

	if code == "" || handler.kvStore == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Authorization code invalid or expired", "invalid_grant")
		return
	}

	codeJSON, err := handler.kvStore.Get(request.Context(), "auth:code:"+code)
	if err != nil || codeJSON == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Authorization code invalid or expired", "invalid_grant")
		return
	}

	// Delete code immediately (single-use)
	_ = handler.kvStore.Delete(request.Context(), "auth:code:"+code)

	var oidcAuthorizationCodePayload OIDCAuthorizationCodePayload
	if unmarshalErr := json.Unmarshal([]byte(codeJSON), &oidcAuthorizationCodePayload); unmarshalErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Failed to read authorization code", "invalid_grant")
		return
	}

	if oidcAuthorizationCodePayload.ClientID != clientID {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Client mismatch", "invalid_grant")
		return
	}

	if redirectURI != "" && oidcAuthorizationCodePayload.RedirectURI != redirectURI {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "redirect_uri mismatch", "invalid_grant")
		return
	}

	// Verify PKCE S256 code challenge
	sha256Digest := sha256.Sum256([]byte(codeVerifier))
	calculatedChallenge := base64.RawURLEncoding.EncodeToString(sha256Digest[:])
	if oidcAuthorizationCodePayload.CodeChallenge == "" || oidcAuthorizationCodePayload.CodeChallenge != calculatedChallenge {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid code_verifier for PKCE challenge", "invalid_grant")
		return
	}

	// Retrieve user record
	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	query := `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, properties, created_at, last_updated_at
		FROM auth.users
		WHERE id = $1
	`
	scanErr := handler.db.QueryRow(ctx, query, oidcAuthorizationCodePayload.UserID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &rawProperties,
		&userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if scanErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to query user", "server_error")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	config := handler.configManager.Get()
	accessExpiry := config.Sessions.AccessTokenExpirySeconds

	userEmail := ""
	if userRecord.Email != nil {
		userEmail = *userRecord.Email
	}
	userPhone := ""
	if userRecord.Phone != nil {
		userPhone = *userRecord.Phone
	}

	accessToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     userRecord.ID,
		Email:       userEmail,
		Phone:       userPhone,
		Role:        userRecord.Role,
		IsAnonymous: userRecord.IsAnonymous,
	}, accessExpiry)

	refreshToken := jwt.GenerateRefreshToken()
	refreshTokenHash := jwt.HashRefreshToken(refreshToken)
	sessionExpiry := config.Sessions.RefreshTokenExpirySeconds
	expiresAt := time.Now().UTC().Add(time.Duration(sessionExpiry) * time.Second)
	sessionID := uuid.NewV7().String()
	_, _ = handler.db.Exec(ctx, `
		INSERT INTO auth.sessions (id, user_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, clock_timestamp())
	`, sessionID, userRecord.ID, refreshTokenHash, expiresAt)

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewSessionCreatedEvent(sessionID, SessionCreatedEventData{
			ID:        sessionID,
			UserID:    userRecord.ID,
			ExpiresAt: expiresAt,
			CreatedAt: time.Now().UTC(),
		}))
	}

	baseURL := core.GetConfig().ServerBaseURL()
	idToken, _ := handler.signer.GenerateIDToken(jwt.OIDCIDTokenClaims{
		Issuer:              baseURL,
		Subject:             userRecord.ID,
		Audience:            clientID,
		Nonce:               oidcAuthorizationCodePayload.Nonce,
		Email:               userEmail,
		EmailVerified:       userRecord.EmailVerifiedAt != nil,
		PhoneNumber:         userPhone,
		PhoneNumberVerified: userRecord.PhoneVerifiedAt != nil,
		Role:                userRecord.Role,
		IsAnonymous:         userRecord.IsAnonymous,
	}, accessExpiry)

	oidcTokenResponse := OIDCTokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    accessExpiry,
		RefreshToken: refreshToken,
		IDToken:      idToken,
		Scope:        oidcAuthorizationCodePayload.Scope,
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(oidcTokenResponse)
}

func (handler *Handler) handleOIDCTokenRefreshToken(responseWriter http.ResponseWriter, request *http.Request) {
	clientID, clientSecret, hasBasicAuth := request.BasicAuth()
	if !hasBasicAuth {
		clientID = request.FormValue("client_id")
		clientSecret = request.FormValue("client_secret")
	}

	if clientID != "" {
		oidcClientConfig, ok := handler.configManager.GetOIDCClient(clientID)
		if ok && !oidcClientConfig.Public {
			decryptedSecret, _ := handler.configManager.DecryptSecret(oidcClientConfig.ClientSecret)
			if clientSecret == "" || clientSecret != decryptedSecret {
				core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid client secret", "invalid_client")
				return
			}
		}
	}

	refreshToken := request.FormValue("refresh_token")
	if refreshToken == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Missing refresh_token parameter", "invalid_request")
		return
	}

	ctx := request.Context()
	refreshTokenHash := jwt.HashRefreshToken(refreshToken)

	var sessionID, userID string
	err := handler.db.QueryRow(ctx, `
		SELECT id, user_id FROM auth.sessions
		WHERE refresh_token_hash = $1 AND expires_at > clock_timestamp()
	`, refreshTokenHash).Scan(&sessionID, &userID)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired refresh token", "invalid_grant")
		return
	}

	// Rotate refresh token
	newRefreshToken := jwt.GenerateRefreshToken()
	newRefreshTokenHash := jwt.HashRefreshToken(newRefreshToken)
	config := handler.configManager.Get()
	sessionExpiry := config.Sessions.RefreshTokenExpirySeconds
	newExpiresAt := time.Now().UTC().Add(time.Duration(sessionExpiry) * time.Second)

	_, _ = handler.db.Exec(ctx, `
		UPDATE auth.sessions
		SET refresh_token_hash = $1, expires_at = $2
		WHERE id = $3
	`, newRefreshTokenHash, newExpiresAt, sessionID)

	var userRecord UserRecord
	_ = handler.db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at
		FROM auth.users WHERE id = $1
	`, userID).Scan(&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous, &userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt)

	accessExpiry := config.Sessions.AccessTokenExpirySeconds

	userEmail := ""
	if userRecord.Email != nil {
		userEmail = *userRecord.Email
	}
	userPhone := ""
	if userRecord.Phone != nil {
		userPhone = *userRecord.Phone
	}

	accessToken, _ := handler.signer.GenerateAccessToken(jwt.Claims{
		Subject:     userRecord.ID,
		Email:       userEmail,
		Phone:       userPhone,
		Role:        userRecord.Role,
		IsAnonymous: userRecord.IsAnonymous,
	}, accessExpiry)

	baseURL := core.GetConfig().ServerBaseURL()
	idToken, _ := handler.signer.GenerateIDToken(jwt.OIDCIDTokenClaims{
		Issuer:              baseURL,
		Subject:             userRecord.ID,
		Audience:            clientID,
		Email:               userEmail,
		EmailVerified:       userRecord.EmailVerifiedAt != nil,
		PhoneNumber:         userPhone,
		PhoneNumberVerified: userRecord.PhoneVerifiedAt != nil,
		Role:                userRecord.Role,
		IsAnonymous:         userRecord.IsAnonymous,
	}, accessExpiry)

	oidcTokenResponse := OIDCTokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    accessExpiry,
		RefreshToken: newRefreshToken,
		IDToken:      idToken,
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(oidcTokenResponse)
}

// 5. Userinfo Endpoint (GET /api/v1/auth/oauth/userinfo)

func (handler *Handler) handleOIDCUserInfo(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.OIDC.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "OIDC Identity Provider is disabled by the console user", "oidc_disabled")
		return
	}

	authHeader := request.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Bearer token required", "LAYR_AUTH_002")
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := handler.signer.VerifyAccessToken(token)
	if err != nil || claims == nil || claims.Subject == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid token", "LAYR_AUTH_002")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	query := `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, properties, created_at, last_updated_at
		FROM auth.users WHERE id = $1
	`
	scanErr := handler.db.QueryRow(ctx, query, claims.Subject).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &rawProperties,
		&userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if scanErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	name := ""
	if nameProperty, ok := userRecord.Properties["name"].(string); ok {
		name = nameProperty
	}

	oidcUserInfoResponse := OIDCUserInfoResponse{
		Subject:             userRecord.ID,
		Name:                name,
		Email:               userRecord.Email,
		EmailVerified:       userRecord.EmailVerifiedAt != nil,
		PhoneNumber:         userRecord.Phone,
		PhoneNumberVerified: userRecord.PhoneVerifiedAt != nil,
		Role:                userRecord.Role,
		IsAnonymous:         userRecord.IsAnonymous,
		UpdatedAt:           userRecord.LastUpdatedAt.Unix(),
		Properties:          userRecord.Properties,
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(oidcUserInfoResponse)
}

// 6. Sign-Out / End Session Endpoint (GET & POST /api/v1/auth/oauth/sign-out)

func (handler *Handler) handleOIDCSignOut(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.OIDC.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "OIDC Identity Provider is disabled by the console user", "oidc_disabled")
		return
	}

	isSecure := core.IsSecureRequest(request)
	core.ClearSessionCookie(responseWriter, AuthSessionCookieName, AuthSessionInsecureCookieName, isSecure)

	postSignOutRedirectURI := request.URL.Query().Get("post_sign_out_redirect_uri")
	if postSignOutRedirectURI == "" {
		_ = request.ParseForm()
		postSignOutRedirectURI = request.FormValue("post_sign_out_redirect_uri")
	}

	if postSignOutRedirectURI != "" {
		http.Redirect(responseWriter, request, postSignOutRedirectURI, http.StatusFound)
		return
	}

	responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	responseWriter.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(responseWriter, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Signed Out</title></head><body style="background:#09090b;color:#f4f4f5;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;"><div style="text-align:center;"><h2>You have been signed out</h2><p>You can now close this window.</p></div></body></html>`)
}

// Helper methods

func redirectError(responseWriter http.ResponseWriter, request *http.Request, redirectURI, errSlug, errDescription, clientState string) {
	targetURL, err := url.Parse(redirectURI)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, errDescription, errSlug)
		return
	}
	queryValues := targetURL.Query()
	queryValues.Set("error", errSlug)
	queryValues.Set("error_description", errDescription)
	if clientState != "" {
		queryValues.Set("state", clientState)
	}
	targetURL.RawQuery = queryValues.Encode()
	http.Redirect(responseWriter, request, targetURL.String(), http.StatusFound)
}

type signInPageData struct {
	ProjectName     string
	ClientName      string
	LogoURL         string
	StateID         string
	ErrorMessage    string
	CustomCSS       template.CSS
	PasskeysEnabled bool
	Providers       []providerButtonData
}

type providerButtonData struct {
	ID   string
	Name string
}

func (handler *Handler) renderOIDCSignInPage(responseWriter http.ResponseWriter, stateID string, oidcClientConfig *OIDCClientConfig, errorMessage string) {
	config := handler.configManager.Get()
	projectName := core.GetConfig().Project.Name
	if projectName == "" {
		projectName = "Layr"
	}
	clientName := ""
	if oidcClientConfig != nil && oidcClientConfig.Name != "" {
		clientName = oidcClientConfig.Name
	}

	logoURL := config.OIDC.SignInUI.LogoURL
	customCSS := config.OIDC.SignInUI.CustomCSS

	var providers []providerButtonData
	for providerKey, providerConfig := range config.OAuthProviders {
		if providerConfig.Enabled {
			name := strings.ToUpper(providerKey[:1]) + providerKey[1:]
			providers = append(providers, providerButtonData{
				ID:   providerKey,
				Name: name,
			})
		}
	}

	data := signInPageData{
		ProjectName:     projectName,
		ClientName:      clientName,
		LogoURL:         logoURL,
		StateID:         stateID,
		ErrorMessage:    errorMessage,
		CustomCSS:       template.CSS(customCSS),
		PasskeysEnabled: config.Passkeys.Enabled,
		Providers:       providers,
	}

	htmlTemplate := template.Must(template.New("signInPage").Parse(signInPageTemplateHTML))
	responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	responseWriter.WriteHeader(http.StatusOK)
	_ = htmlTemplate.Execute(responseWriter, data)
}

const signInPageTemplateHTML = `<!DOCTYPE html>
<html lang="en" class="dark">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Sign in{{if .ClientName}} to {{.ClientName}}{{end}} · {{.ProjectName}}</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/basecoat-css/dist/basecoat.min.css">
  <style>
    :root {
      --bg: #09090b;
      --card-bg: #121215;
      --card-border: #27272a;
      --fg: #f4f4f5;
      --muted: #a1a1aa;
      --primary: #ffffff;
      --primary-fg: #09090b;
      --destructive: #ef4444;
      --destructive-bg: rgba(239, 68, 68, 0.12);
      --destructive-border: rgba(239, 68, 68, 0.3);
      --input-bg: #18181b;
      --input-border: #3f3f46;
    }
    * {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
    }
    body {
      background-color: var(--bg);
      color: var(--fg);
      font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 1.5rem;
      position: relative;
      overflow-x: hidden;
    }
    .bg-gradient {
      position: fixed;
      inset: 0;
      background: radial-gradient(circle at 50% 15%, rgba(120, 119, 198, 0.12), transparent 60%);
      pointer-events: none;
    }
    .card-container {
      position: relative;
      z-index: 10;
      width: 100%;
      max-width: 400px;
    }
    .signin-card {
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      border-radius: 1rem;
      padding: 2.25rem 2rem;
      box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.65);
    }
    .brand-header {
      margin-bottom: 1.75rem;
      text-align: left;
    }
    .brand-logo-icon {
      width: 2.25rem;
      height: 2.25rem;
      background: var(--primary);
      color: var(--primary-fg);
      border-radius: 0.625rem;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      margin-bottom: 0.875rem;
      box-shadow: 0 4px 12px rgba(255, 255, 255, 0.1);
    }
    .brand-logo-img {
      max-height: 2.5rem;
      margin-bottom: 0.875rem;
      display: block;
    }
    h1 {
      font-size: 1.5rem;
      font-weight: 700;
      letter-spacing: -0.025em;
      color: var(--fg);
      margin-bottom: 0.35rem;
    }
    p.subtitle {
      font-size: 0.875rem;
      color: var(--muted);
    }
    .alert-error {
      background: var(--destructive-bg);
      border: 1px solid var(--destructive-border);
      color: var(--destructive);
      padding: 0.75rem 1rem;
      border-radius: 0.5rem;
      font-size: 0.875rem;
      margin-bottom: 1.25rem;
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }
    .field {
      display: flex;
      flex-direction: column;
      gap: 0.4rem;
      margin-bottom: 1rem;
    }
    label {
      font-size: 0.8125rem;
      font-weight: 500;
      color: var(--fg);
    }
    input.input-text {
      width: 100%;
      background: var(--input-bg);
      border: 1px solid var(--input-border);
      color: var(--fg);
      padding: 0.625rem 0.875rem;
      border-radius: 0.5rem;
      font-size: 0.875rem;
      outline: none;
      transition: border-color 0.15s ease, box-shadow 0.15s ease;
    }
    input.input-text:focus {
      border-color: #a1a1aa;
      box-shadow: 0 0 0 2px rgba(161, 161, 170, 0.2);
    }
    .submit-btn {
      width: 100%;
      background: var(--primary);
      color: var(--primary-fg);
      border: none;
      padding: 0.6875rem 1rem;
      border-radius: 0.5rem;
      font-size: 0.875rem;
      font-weight: 600;
      cursor: pointer;
      margin-top: 0.5rem;
      transition: opacity 0.15s ease;
    }
    .submit-btn:hover {
      opacity: 0.92;
    }
    .passkey-btn {
      width: 100%;
      background: #27272a;
      color: var(--fg);
      border: 1px solid var(--card-border);
      padding: 0.625rem 1rem;
      border-radius: 0.5rem;
      font-size: 0.875rem;
      font-weight: 500;
      cursor: pointer;
      margin-top: 0.625rem;
      transition: background-color 0.15s ease;
    }
    .passkey-btn:hover {
      background: #3f3f46;
    }
    .divider {
      display: flex;
      align-items: center;
      text-align: center;
      margin: 1.5rem 0;
      color: var(--muted);
      font-size: 0.75rem;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }
    .divider::before, .divider::after {
      content: '';
      flex: 1;
      border-bottom: 1px solid var(--card-border);
    }
    .divider span {
      padding: 0 0.75rem;
    }
    .providers-grid {
      display: grid;
      gap: 0.625rem;
    }
    .provider-btn {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 0.5rem;
      width: 100%;
      background: #18181b;
      border: 1px solid var(--input-border);
      color: var(--fg);
      padding: 0.625rem 1rem;
      border-radius: 0.5rem;
      font-size: 0.875rem;
      font-weight: 500;
      text-decoration: none;
      transition: background-color 0.15s ease;
    }
    .provider-btn:hover {
      background: #27272a;
    }
    .brand-footer {
      margin-top: 1.5rem;
      text-align: center;
      font-size: 0.75rem;
      color: var(--muted);
      letter-spacing: 0.025em;
    }
  </style>
  {{if .CustomCSS}}
  <style id="layr-custom-css">
{{.CustomCSS}}
  </style>
  {{end}}
</head>
<body>
  <div class="bg-gradient"></div>
  <div class="card-container">
    <div class="signin-card">
      <div class="brand-header">
        {{if .LogoURL}}
          <img src="{{.LogoURL}}" alt="{{.ProjectName}} Logo" class="brand-logo-img">
        {{else}}
          <div class="brand-logo-icon">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/>
              <path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.65"/>
              <path d="m22 12.65-9.17 4.16a2 2 0 0 1-1.66 0L2 12.65"/>
            </svg>
          </div>
        {{end}}
        <h1>Sign in</h1>
        {{if .ClientName}}
        <p class="subtitle">Continue to {{.ClientName}}</p>
        {{end}}
      </div>

      {{if .ErrorMessage}}
        <div class="alert-error">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
          <span>{{.ErrorMessage}}</span>
        </div>
      {{end}}

      <form action="/api/v1/auth/oauth/authorize" method="POST">
        <input type="hidden" name="state" value="{{.StateID}}">

        <div class="field">
          <label for="signin-email">Email</label>
          <input id="signin-email" name="email" type="email" autocomplete="email" required autofocus placeholder="name@example.com" class="input-text">
        </div>

        <div class="field">
          <label for="signin-password">Password</label>
          <input id="signin-password" name="password" type="password" autocomplete="current-password" required placeholder="••••••••••••" class="input-text">
        </div>

        <button type="submit" class="submit-btn">Sign in</button>
      </form>

      {{if .PasskeysEnabled}}
        <button type="button" class="passkey-btn" onclick="alert('Passkey sign-in available via client application')">
          Sign in with Passkey
        </button>
      {{end}}

      {{if .Providers}}
        <div class="divider"><span>Or</span></div>
        <div class="providers-grid">
          {{range .Providers}}
            <a href="/api/v1/auth/oauth/{{.ID}}/authorize?oidc_state={{$.StateID}}" class="provider-btn">
              Sign in with {{.Name}}
            </a>
          {{end}}
        </div>
      {{end}}

      <div class="brand-footer">
        Secured by {{.ProjectName}}
      </div>
    </div>
  </div>
</body>
</html>`

// RegisterOIDCRoutes registers OpenID Connect Identity Provider routes on the provided router.
func (handler *Handler) RegisterOIDCRoutes(router *core.Router) {
	log.Debug("registering OIDC Identity Provider routes on router")
	router.Mux().HandleFunc("GET /.well-known/openid-configuration", handler.handleOIDCDiscovery)
	router.Mux().HandleFunc("GET /api/v1/auth/oauth/jwks.json", handler.handleJWKS)
	router.Mux().HandleFunc("GET /api/v1/auth/oauth/authorize", handler.handleOIDCAuthorize)
	router.Mux().HandleFunc("POST /api/v1/auth/oauth/authorize", handler.handleOIDCAuthorizeSubmit)
	router.Mux().HandleFunc("POST /api/v1/auth/oauth/token", handler.handleOIDCToken)
	router.Mux().HandleFunc("GET /api/v1/auth/oauth/userinfo", handler.handleOIDCUserInfo)
	router.Mux().HandleFunc("GET /api/v1/auth/oauth/sign-out", handler.handleOIDCSignOut)
	router.Mux().HandleFunc("POST /api/v1/auth/oauth/sign-out", handler.handleOIDCSignOut)
}
