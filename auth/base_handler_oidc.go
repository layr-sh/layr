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

	"layr.sh/auth/otp"
	"layr.sh/core"
)

const defaultM2MTokenExpirySeconds = 3600
const defaultOIDCMFATTL = 5 * time.Minute

// OIDCConfiguration represents OpenID Connect Core 1.0 discovery metadata (RFC 8414).
type OIDCConfiguration struct {
	Issuer                              string   `json:"issuer"`
	AuthorizationEndpoint               string   `json:"authorization_endpoint"`
	TokenEndpoint                       string   `json:"token_endpoint"`
	UserinfoEndpoint                    string   `json:"userinfo_endpoint"`
	JwksURI                             string   `json:"jwks_uri"`
	EndSessionEndpoint                  string   `json:"end_session_endpoint"`
	BackChannelSignOutSupported         bool     `json:"backchannel_logout_supported"`
	BackChannelSignOutSessionSupported  bool     `json:"backchannel_logout_session_supported"`
	FrontChannelSignOutSupported        bool     `json:"frontchannel_logout_supported"`
	FrontChannelSignOutSessionSupported bool     `json:"frontchannel_logout_session_supported"`
	ResponseTypesSupported              []string `json:"response_types_supported"`
	SubjectTypesSupported               []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported    []string `json:"id_token_signing_alg_values_supported"`
	ScopesSupported                     []string `json:"scopes_supported"`
	TokenEndpointAuthMethodsSupported   []string `json:"token_endpoint_auth_methods_supported"`
	ClaimsSupported                     []string `json:"claims_supported"`
	CodeChallengeMethodsSupported       []string `json:"code_challenge_methods_supported"`
	GrantTypesSupported                 []string `json:"grant_types_supported"`
}

// BuildOIDCDiscovery generates OpenID Connect Core 1.0 discovery metadata.
func BuildOIDCDiscovery(baseURL string) OIDCConfiguration {
	normalizedBaseURL := strings.TrimRight(baseURL, "/")
	return OIDCConfiguration{
		Issuer:                              normalizedBaseURL,
		AuthorizationEndpoint:               normalizedBaseURL + "/v1/auth/oauth/authorize",
		TokenEndpoint:                       normalizedBaseURL + "/v1/auth/oauth/token",
		UserinfoEndpoint:                    normalizedBaseURL + "/v1/auth/oauth/userinfo",
		JwksURI:                             normalizedBaseURL + "/.well-known/jwks.json",
		EndSessionEndpoint:                  normalizedBaseURL + "/v1/auth/oauth/sign-out",
		BackChannelSignOutSupported:         true,
		BackChannelSignOutSessionSupported:  true,
		FrontChannelSignOutSupported:        true,
		FrontChannelSignOutSessionSupported: true,
		ResponseTypesSupported: []string{
			"code",
			"token",
			"id_token",
			"code token",
			"code id_token",
			"token id_token",
			"code token id_token",
		},
		SubjectTypesSupported: []string{
			"public",
		},
		IDTokenSigningAlgValuesSupported: []string{
			"EdDSA",
		},
		ScopesSupported: []string{
			"openid",
			"profile",
			"email",
			"phone",
			"offline_access",
		},
		TokenEndpointAuthMethodsSupported: []string{
			"client_secret_post",
			"client_secret_basic",
			"none",
		},
		ClaimsSupported: []string{
			"sub",
			"iss",
			"aud",
			"exp",
			"iat",
			"auth_time",
			"nonce",
			"email",
			"email_verified",
			"phone_number",
			"phone_number_verified",
			"role",
		},
		CodeChallengeMethodsSupported: []string{
			"S256",
			"plain",
		},
		GrantTypesSupported: []string{
			"authorization_code",
			"refresh_token",
			"client_credentials",
		},
	}
}

// 1. OIDC Discovery & JWKS

func (handler *BaseHandler) handleGetOIDCDiscovery(responseWriter http.ResponseWriter, request *http.Request) {
	baseURL := core.GetConfig().ServerBaseURL()
	oidcConfiguration := BuildOIDCDiscovery(baseURL)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(oidcConfiguration)
}

func (handler *BaseHandler) handleGetJWKS(responseWriter http.ResponseWriter, request *http.Request) {
	jwks := handler.kernel.JWTSigner().BuildJWKS()
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(jwks)
}

// 2. Authorization Endpoint (GET /v1/auth/oauth/authorize)

func (handler *BaseHandler) handleAuthorizeOIDC(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.OIDC.Enabled {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusForbidden, "access_denied", "Access denied", "OIDC authorize request rejected: OIDC identity provider is disabled in configuration")
		return
	}

	stateIDParam := request.URL.Query().Get("state")
	modeParam := request.URL.Query().Get("mode")
	if stateIDParam != "" && modeParam != "" {
		stateJSON, err := handler.kernel.KVStore().Get(request.Context(), "auth:oidc:state:"+stateIDParam)
		if err == nil && stateJSON != "" {
			var oidcAuthorizationStatePayload OIDCAuthorizationStatePayload
			if json.Unmarshal([]byte(stateJSON), &oidcAuthorizationStatePayload) == nil {
				oidcClientConfig, _ := handler.configManager.GetOIDCClient(oidcAuthorizationStatePayload.ClientID)
				handler.renderOIDCPage(responseWriter, stateIDParam, oidcClientConfig, "", "", modeParam, "", "")
				return
			}
		}
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
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_client", "Missing client_id parameter")
		return
	}

	oidcClientConfig, ok := handler.configManager.GetOIDCClient(clientID)
	if !ok {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_client", fmt.Sprintf("Unknown client_id '%s'", clientID))
		return
	}

	if redirectURI == "" {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_request", "Missing redirect_uri parameter")
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
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_request", "Unauthorized redirect_uri")
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
	if authContext := core.GetAuthContext(request.Context()); authContext.UserID != "" {
		activeUserID := authContext.UserID
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

	payloadJSON, _ := json.Marshal(oidcAuthorizationStatePayload)
	_ = handler.kernel.KVStore().Set(request.Context(), "auth:oidc:state:"+stateID, string(payloadJSON), 10*time.Minute)

	handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "")
}

func (handler *BaseHandler) completeOIDCAuthorization(
	responseWriter http.ResponseWriter,
	request *http.Request,
	stateID string,
	oidcAuthorizationStatePayload OIDCAuthorizationStatePayload,
	user User,
) {
	ctx := request.Context()
	config := handler.configManager.Get()

	// Delete state payload to prevent replay / CSRF fixation
	_ = handler.kernel.KVStore().Delete(ctx, "auth:oidc:state:"+stateID)

	// Issue authorization code
	code := handler.issueOIDCAuthorizationCode(
		ctx,
		oidcAuthorizationStatePayload.ClientID,
		oidcAuthorizationStatePayload.RedirectURI,
		user.ID,
		oidcAuthorizationStatePayload.Scope,
		oidcAuthorizationStatePayload.CodeChallenge,
		oidcAuthorizationStatePayload.CodeChallengeMethod,
		oidcAuthorizationStatePayload.Nonce,
	)

	// Set browser session cookie for SSO
	refreshToken := handler.kernel.JWTSigner().GenerateRefreshToken()
	refreshTokenHash := handler.kernel.JWTSigner().HashRefreshToken(refreshToken)
	expiresAt := time.Now().UTC().Add(time.Duration(config.Sessions.RefreshTokenExpirySeconds) * time.Second)
	sessionID := uuid.NewV7().String()
	_, _ = handler.kernel.DB().Exec(ctx, `
		INSERT INTO auth.sessions (id, user_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, clock_timestamp())
	`, sessionID, user.ID, refreshTokenHash, expiresAt)

	core.SetSessionCookie(responseWriter, request, refreshToken, expiresAt)

	handler.kernel.EventBus().Publish(ctx, NewSessionCreatedEvent(sessionID, SessionCreatedEventData{
		ID:        sessionID,
		User:      user,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}))

	// 302 Found redirect back to client redirect_uri
	targetURL, parseErr := url.Parse(oidcAuthorizationStatePayload.RedirectURI)
	if parseErr != nil {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_request", "Invalid redirect_uri")
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

// 3. Authorization Form Submission (POST /v1/auth/oauth/authorize)

func (handler *BaseHandler) handleSubmitOIDCAuthorize(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.OIDC.Enabled {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusForbidden, "access_denied", "Access denied", "OIDC authorize submit rejected: OIDC identity provider is disabled in configuration")
		return
	}

	if err := request.ParseForm(); err != nil {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_request", "Invalid form data")
		return
	}

	stateID := request.FormValue("state")
	if stateID == "" {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_request", "Authorization session expired or invalid")
		return
	}

	ctx := request.Context()
	stateJSON, err := handler.kernel.KVStore().Get(ctx, "auth:oidc:state:"+stateID)
	if err != nil || stateJSON == "" {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_request", "Authorization session expired or invalid")
		return
	}

	var oidcAuthorizationStatePayload OIDCAuthorizationStatePayload
	if unmarshalErr := json.Unmarshal([]byte(stateJSON), &oidcAuthorizationStatePayload); unmarshalErr != nil {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_request", "Authorization session expired or invalid")
		return
	}

	oidcClientConfig, _ := handler.configManager.GetOIDCClient(oidcAuthorizationStatePayload.ClientID)
	action := strings.TrimSpace(request.FormValue("action"))
	switch action {
	case "", "sign_in":
		action = "sign_in"
	case "sign_up":
		action = "sign_up"
	}

	// 1. MFA Verification Challenge Form
	if action == "verify_mfa" {
		mfaToken := request.FormValue("mfa_token")
		mfaCode := strings.TrimSpace(request.FormValue("mfa_code"))
		if mfaToken == "" {
			handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "MFA session expired. Please sign in again.")
			return
		}
		mfaUserID, _ := handler.kernel.KVStore().Get(ctx, "auth:oidc:mfa:"+mfaToken)
		if mfaUserID == "" {
			handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "MFA session expired. Please sign in again.")
			return
		}
		if mfaCode == "" {
			handler.renderOIDCMFAPage(responseWriter, stateID, oidcClientConfig, "Two-factor authentication code is required", "", mfaToken)
			return
		}

		var user User
		var rawProperties []byte
		query := `
			SELECT id, email, phone, password_hash, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
			FROM auth.users
			WHERE id = $1
		`
		scanErr := handler.kernel.DB().QueryRow(ctx, query, mfaUserID).Scan(
			&user.ID, &user.Email, &user.Phone, &user.PasswordHash,
			&user.Role, &user.IsAnonymous, &user.EmailVerifiedAt, &user.PhoneVerifiedAt,
			&user.LockedUntil, &user.EncryptedMFASecret, &user.MFAEnabled,
			&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
		)
		if scanErr != nil {
			handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "MFA session expired. Please sign in again.")
			return
		}

		if user.EncryptedMFASecret == nil || *user.EncryptedMFASecret == "" {
			handler.renderOIDCMFAPage(responseWriter, stateID, oidcClientConfig, "Multi-factor authentication configuration error", "", mfaToken)
			return
		}
		secretBytes, decryptErr := handler.kernel.CryptoKeyManager().DecryptField(*user.EncryptedMFASecret)
		if decryptErr != nil {
			log.Errorf("failed to decrypt MFA secret for user %s: %v", user.ID, decryptErr)
			handler.renderOIDCMFAPage(responseWriter, stateID, oidcClientConfig, "Failed to verify multi-factor authentication", "", mfaToken)
			return
		}
		if !handler.totpManager.ValidateCode(string(secretBytes), mfaCode, time.Now().UTC(), 1) {
			log.Debugf("invalid MFA code supplied during OIDC MFA challenge for user %s", user.ID)
			handler.renderOIDCMFAPage(responseWriter, stateID, oidcClientConfig, "Invalid two-factor authentication code", "", mfaToken)
			return
		}

		_ = handler.kernel.KVStore().Delete(ctx, "auth:oidc:mfa:"+mfaToken)
		handler.completeOIDCAuthorization(responseWriter, request, stateID, oidcAuthorizationStatePayload, user)
		return
	}

	// 2. OTP Request Action
	if action == "send_otp" {
		recipient := strings.TrimSpace(request.FormValue("recipient"))
		if recipient == "" {
			handler.renderOIDCOTPRequestPage(responseWriter, stateID, oidcClientConfig, "Please enter an email address or phone number", "")
			return
		}

		isEmail := strings.Contains(recipient, "@")
		var expiryMinutes int
		if isEmail {
			if !config.EmailOTP.Enabled || !config.OIDC.UI.ShowEmailOTP || !handler.emailDispatcher.IsConfigured() {
				handler.renderOIDCOTPRequestPage(responseWriter, stateID, oidcClientConfig, "Email OTP sign-in is not available", "")
				return
			}
			recipient = strings.ToLower(recipient)
			expiryMinutes = config.EmailOTP.TokenExpiryMinutes
		} else {
			if !config.SMSOTP.Enabled || !config.OIDC.UI.ShowSMSOTP || !handler.smsDispatcher.IsConfigured() {
				handler.renderOIDCOTPRequestPage(responseWriter, stateID, oidcClientConfig, "SMS OTP sign-in is not available", "")
				return
			}
			normalizedPhone, err := NormalizePhone(recipient)
			if err != nil {
				handler.renderOIDCOTPRequestPage(responseWriter, stateID, oidcClientConfig, "Invalid phone number format: must be in E.164 format with country code", "")
				return
			}
			recipient = normalizedPhone
			expiryMinutes = config.SMSOTP.TokenExpiryMinutes
		}

		clientIP := core.ExtractRequestClientIP(request)
		ipRateKey := fmt.Sprintf("auth:ratelimit:otp:ip:%s", clientIP)
		if count, err := handler.kernel.KVStore().Increment(ctx, ipRateKey, time.Hour); err == nil && count > 10 {
			handler.renderOIDCOTPRequestPage(responseWriter, stateID, oidcClientConfig, "Rate limit exceeded. Too many requests from this IP address.", recipient)
			return
		}

		cooldownKey := fmt.Sprintf("auth:cooldown:sign_in:%s", recipient)
		if _, err := handler.kernel.KVStore().Get(ctx, cooldownKey); err == nil {
			handler.renderOIDCOTPRequestPage(responseWriter, stateID, oidcClientConfig, "Please wait 60 seconds before requesting another code", recipient)
			return
		}

		codeTTL := time.Duration(expiryMinutes) * time.Minute
		code, _ := otp.GenerateCode(nil)
		codeHash := otp.HashCode(code)

		query := `
			INSERT INTO auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
			VALUES ($1, $2, 'sign_in', 0, $3, clock_timestamp())
		`
		expiresAt := time.Now().UTC().Add(codeTTL)
		_, _ = handler.kernel.DB().Exec(ctx, query, recipient, codeHash, expiresAt)
		_ = handler.kernel.KVStore().Set(ctx, fmt.Sprintf("auth:otp:sign_in:%s", recipient), code, codeTTL)
		_ = handler.kernel.KVStore().Set(ctx, fmt.Sprintf("auth:cooldown:sign_in:%s", recipient), "1", defaultOTPCooldown)

		channel := "sms"
		if isEmail {
			channel = "email"
			_ = handler.emailDispatcher.SendSignInOTP(ctx, recipient, code, "")
		} else {
			_ = handler.smsDispatcher.SendSignInOTP(ctx, recipient, code, "")
		}

		var targetUser *User
		if fetchedUser, fetchErr := fetchUserByRecipient(ctx, handler.kernel.DB(), recipient); fetchErr == nil {
			targetUser = &fetchedUser
		}
		handler.kernel.EventBus().Publish(ctx, NewOTPSentEvent(recipient, OTPSentEventData{
			Recipient: recipient,
			Purpose:   "sign_in",
			Channel:   channel,
			User:      targetUser,
		}))

		handler.renderOIDCOTPVerifyPage(responseWriter, stateID, oidcClientConfig, "", "Verification code sent!", recipient)
		return
	}

	// 3. OTP Verify Action
	if action == "verify_otp" {
		recipient := strings.TrimSpace(request.FormValue("recipient"))
		otpCode := strings.TrimSpace(request.FormValue("otp_code"))
		if recipient == "" || otpCode == "" {
			handler.renderOIDCOTPVerifyPage(responseWriter, stateID, oidcClientConfig, "Verification code is required", "", recipient)
			return
		}

		isEmail := strings.Contains(recipient, "@")
		if !isEmail {
			if norm, normErr := NormalizePhone(recipient); normErr == nil {
				recipient = norm
			}
		} else {
			recipient = strings.ToLower(recipient)
		}

		var otpID, storedHash string
		var attempts int
		var expiresAt time.Time
		err := handler.kernel.DB().QueryRow(ctx, `
			SELECT id, code_hash, attempts, expires_at 
			FROM auth.otps 
			WHERE recipient = $1 AND purpose = 'sign_in' AND expires_at > clock_timestamp()
			ORDER BY created_at DESC 
			LIMIT 1
		`, recipient).Scan(&otpID, &storedHash, &attempts, &expiresAt)
		if err != nil {
			handler.renderOIDCOTPVerifyPage(responseWriter, stateID, oidcClientConfig, "Invalid or expired verification code", "", recipient)
			return
		}

		if !otp.VerifyCode(otpCode, storedHash) {
			_, _ = handler.kernel.DB().Exec(ctx, "UPDATE auth.otps SET attempts = attempts + 1 WHERE id = $1", otpID)
			handler.renderOIDCOTPVerifyPage(responseWriter, stateID, oidcClientConfig, "Invalid or expired verification code", "", recipient)
			return
		}

		_, _ = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.otps WHERE id = $1", otpID)
		_ = handler.kernel.KVStore().Delete(ctx, fmt.Sprintf("auth:otp:sign_in:%s", recipient))

		var user User
		var rawProperties []byte
		var isNewUser bool
		if isEmail {
			_ = handler.kernel.DB().QueryRow(ctx, `
				INSERT INTO auth.users (email, role, email_verified_at, created_at, last_updated_at)
				VALUES ($1, 'authenticated', clock_timestamp(), clock_timestamp(), clock_timestamp())
				ON CONFLICT (email) DO UPDATE SET email_verified_at = COALESCE(auth.users.email_verified_at, clock_timestamp()), last_updated_at = clock_timestamp()
				RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at, (xmax = 0) AS is_new
			`, recipient).Scan(
				&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
				&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
				&user.EncryptedMFASecret, &user.MFAEnabled,
				&rawProperties, &user.CreatedAt, &user.LastUpdatedAt, &isNewUser,
			)
		} else {
			_ = handler.kernel.DB().QueryRow(ctx, `
				INSERT INTO auth.users (phone, role, phone_verified_at, created_at, last_updated_at)
				VALUES ($1, 'authenticated', clock_timestamp(), clock_timestamp(), clock_timestamp())
				ON CONFLICT (phone) DO UPDATE SET phone_verified_at = COALESCE(auth.users.phone_verified_at, clock_timestamp()), last_updated_at = clock_timestamp()
				RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at, (xmax = 0) AS is_new
			`, recipient).Scan(
				&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
				&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
				&user.EncryptedMFASecret, &user.MFAEnabled,
				&rawProperties, &user.CreatedAt, &user.LastUpdatedAt, &isNewUser,
			)
		}

		if !isNewUser && user.LockedUntil != nil && time.Now().UTC().Before(*user.LockedUntil) {
			handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Account temporarily locked. Please try again later.")
			return
		}

		channel := "sms"
		if isEmail {
			channel = "email"
		}
		if isNewUser {
			handler.kernel.EventBus().Publish(ctx, NewUserSignedUpEvent(user.ID, UserSignedUpEventData(user)))
		}
		handler.kernel.EventBus().Publish(ctx, NewOTPVerifiedEvent(recipient, OTPVerifiedEventData{
			Recipient: recipient,
			Purpose:   "sign_in",
			Channel:   channel,
			User:      &user,
		}))

		if user.MFAEnabled {
			mfaToken := "mfa_oidc_" + uuid.NewV7().String()
			_ = handler.kernel.KVStore().Set(ctx, "auth:oidc:mfa:"+mfaToken, user.ID, defaultOIDCMFATTL)
			handler.renderOIDCMFAPage(responseWriter, stateID, oidcClientConfig, "", "Two-factor authentication required", mfaToken)
			return
		}

		handler.completeOIDCAuthorization(responseWriter, request, stateID, oidcAuthorizationStatePayload, user)
		return
	}

	// 4. Sign-Up Action
	if action == "sign_up" {
		if !config.Password.Enabled || !config.OIDC.UI.ShowSignUp {
			handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Sign-up is disabled")
			return
		}

		email := strings.ToLower(strings.TrimSpace(request.FormValue("email")))
		password := request.FormValue("password")
		confirmPassword := request.FormValue("confirm_password")

		if email == "" || password == "" {
			handler.renderOIDCSignUpPage(responseWriter, stateID, oidcClientConfig, "Email and password are required")
			return
		}
		if password != confirmPassword {
			handler.renderOIDCSignUpPage(responseWriter, stateID, oidcClientConfig, "Passwords do not match")
			return
		}
		if len(password) < config.Password.MinLength {
			handler.renderOIDCSignUpPage(responseWriter, stateID, oidcClientConfig, fmt.Sprintf("Password must be at least %d characters", config.Password.MinLength))
			return
		}

		var existingID string
		err := handler.kernel.DB().QueryRow(ctx, "SELECT id FROM auth.users WHERE email = $1", email).Scan(&existingID)
		if err == nil {
			handler.renderOIDCSignUpPage(responseWriter, stateID, oidcClientConfig, "An account with this email already exists")
			return
		}

		passHash, _ := handler.hasher.Hash(password)
		var user User
		var rawProperties []byte
		userID := uuid.NewV7().String()
		query := `
			INSERT INTO auth.users (id, email, password_hash, role, is_anonymous, created_at, last_updated_at)
			VALUES ($1, $2, $3, 'authenticated', false, clock_timestamp(), clock_timestamp())
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		`
		err = handler.kernel.DB().QueryRow(ctx, query, userID, email, passHash).Scan(
			&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
			&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
			&user.EncryptedMFASecret, &user.MFAEnabled,
			&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
		)
		if err != nil {
			log.Debugf("failed to create user in OIDC sign up: %v", err)
			handler.renderOIDCSignUpPage(responseWriter, stateID, oidcClientConfig, "Failed to create account")
			return
		}

		handler.kernel.EventBus().Publish(ctx, NewUserSignedUpEvent(user.ID, UserSignedUpEventData(user)))

		handler.completeOIDCAuthorization(responseWriter, request, stateID, oidcAuthorizationStatePayload, user)
		return
	}

	// 5. Sign-In Action (Default)
	if !config.Password.Enabled || !config.OIDC.UI.ShowPassword {
		handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Password sign-in is disabled")
		return
	}

	email := strings.ToLower(strings.TrimSpace(request.FormValue("email")))
	userPassword := request.FormValue("password")
	if email == "" || userPassword == "" {
		handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Email and password are required")
		return
	}

	// Verify user credentials
	var user User
	var rawProperties []byte
	query := `
		SELECT id, email, phone, password_hash, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE email = $1
	`
	scanErr := handler.kernel.DB().QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Phone, &user.PasswordHash,
		&user.Role, &user.IsAnonymous, &user.EmailVerifiedAt, &user.PhoneVerifiedAt,
		&user.LockedUntil, &user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if scanErr != nil || user.PasswordHash == nil {
		handler.verifyDummyPassword(userPassword)
		handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Invalid email or password")
		return
	}

	if user.LockedUntil != nil && user.LockedUntil.After(time.Now().UTC()) {
		handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Account temporarily locked. Please try again later.")
		return
	}

	match, verifyErr := handler.hasher.Verify(userPassword, *user.PasswordHash)
	if verifyErr != nil || !match {
		handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Invalid email or password")
		return
	}

	if user.MFAEnabled {
		mfaCode := strings.TrimSpace(request.FormValue("mfa_code"))
		if mfaCode != "" {
			if user.EncryptedMFASecret == nil || *user.EncryptedMFASecret == "" {
				handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Multi-factor authentication configuration error")
				return
			}
			secretBytes, err := handler.kernel.CryptoKeyManager().DecryptField(*user.EncryptedMFASecret)
			if err != nil {
				log.Errorf("failed to decrypt MFA secret for user %s: %v", user.ID, err)
				handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Failed to verify multi-factor authentication")
				return
			}
			if !handler.totpManager.ValidateCode(string(secretBytes), mfaCode, time.Now().UTC(), 1) {
				log.Debugf("invalid MFA code supplied during OIDC authorize submit for user %s", user.ID)
				handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Invalid two-factor authentication code")
				return
			}
		} else {
			mfaToken := "mfa_oidc_" + uuid.NewV7().String()
			_ = handler.kernel.KVStore().Set(ctx, "auth:oidc:mfa:"+mfaToken, user.ID, defaultOIDCMFATTL)
			handler.renderOIDCMFAPage(responseWriter, stateID, oidcClientConfig, "", "Two-factor authentication required", mfaToken)
			return
		}
	}

	handler.completeOIDCAuthorization(responseWriter, request, stateID, oidcAuthorizationStatePayload, user)
}

func parseOAuthTokenRequest(request *http.Request) OAuthTokenInput {
	var oauthTokenInput OAuthTokenInput

	if strings.Contains(request.Header.Get("Content-Type"), "application/json") && request.Body != nil {
		bodyBytes, readErr := io.ReadAll(request.Body)
		if readErr == nil {
			_ = json.Unmarshal(bodyBytes, &oauthTokenInput)
			request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	} else {
		_ = request.ParseForm()
		if request.FormValue("grant_type") == "" && request.Body != nil {
			bodyBytes, readErr := io.ReadAll(request.Body)
			if readErr == nil {
				_ = json.Unmarshal(bodyBytes, &oauthTokenInput)
				request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}
	}

	if oauthTokenInput.GrantType == "" {
		oauthTokenInput.GrantType = request.FormValue("grant_type")
	}
	if oauthTokenInput.ClientID == "" {
		oauthTokenInput.ClientID = request.FormValue("client_id")
	}
	if oauthTokenInput.ClientSecret == "" {
		oauthTokenInput.ClientSecret = request.FormValue("client_secret")
	}
	if oauthTokenInput.Code == "" {
		oauthTokenInput.Code = request.FormValue("code")
	}
	if oauthTokenInput.RedirectURI == "" {
		oauthTokenInput.RedirectURI = request.FormValue("redirect_uri")
	}
	if oauthTokenInput.CodeVerifier == "" {
		oauthTokenInput.CodeVerifier = request.FormValue("code_verifier")
	}
	if oauthTokenInput.RefreshToken == "" {
		oauthTokenInput.RefreshToken = request.FormValue("refresh_token")
	}
	if oauthTokenInput.Scope == "" {
		oauthTokenInput.Scope = request.FormValue("scope")
	}
	if oauthTokenInput.Audience == "" {
		oauthTokenInput.Audience = request.FormValue("audience")
	}
	if oauthTokenInput.Provider == "" {
		oauthTokenInput.Provider = request.FormValue("provider")
	}

	basicClientID, basicClientSecret, hasBasicAuth := request.BasicAuth()
	if hasBasicAuth && (basicClientID != "" || basicClientSecret != "") {
		oauthTokenInput.ClientID = basicClientID
		oauthTokenInput.ClientSecret = basicClientSecret
	}

	return oauthTokenInput
}

// 4. Token Endpoint (POST /v1/auth/oauth/token)

func (handler *BaseHandler) handleIssueOIDCToken(responseWriter http.ResponseWriter, request *http.Request) {
	oauthTokenInput := parseOAuthTokenRequest(request)

	// If no grant_type or if provider parameter is present, delegate to handleOAuthCallback
	if oauthTokenInput.GrantType == "" || oauthTokenInput.Provider != "" {
		handler.handleProcessOAuthCallback(responseWriter, request)
		return
	}

	if oauthTokenInput.GrantType == "client_credentials" {
		handler.handleOAuthClientCredentials(responseWriter, request, oauthTokenInput)
		return
	}

	config := handler.configManager.Get()
	if !config.OIDC.Enabled {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusForbidden, "access_denied", "Access denied", "OIDC token request rejected: OIDC identity provider is disabled in configuration")
		return
	}

	if oauthTokenInput.GrantType == "authorization_code" {
		handler.handleIssueOIDCTokenAuthorizationCode(responseWriter, request)
		return
	}

	if oauthTokenInput.GrantType == "refresh_token" {
		handler.handleIssueOIDCTokenRefreshToken(responseWriter, request)
		return
	}

	core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "unsupported_grant_type", fmt.Sprintf("Unsupported grant_type '%s'", oauthTokenInput.GrantType))
}

func (handler *BaseHandler) handleOAuthClientCredentials(responseWriter http.ResponseWriter, request *http.Request, oauthTokenInput OAuthTokenInput) {
	clientID := oauthTokenInput.ClientID
	clientSecret := oauthTokenInput.ClientSecret

	if clientSecret == "" {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusUnauthorized, "invalid_client", "Invalid client credentials")
		return
	}

	clientIP := core.ExtractRequestClientIP(request)
	serviceAccount, authErr := handler.kernel.ServiceAccountManager().Authenticate(request.Context(), clientSecret, clientIP)
	if authErr != nil {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusUnauthorized, "invalid_client", "Invalid client credentials")
		return
	}

	if clientID != "" && serviceAccount.ID != clientID && serviceAccount.KeyPrefix != clientID {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusUnauthorized, "invalid_client", "Invalid client credentials")
		return
	}

	targetAudience := strings.TrimSpace(oauthTokenInput.Audience)
	if targetAudience == "" {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_request", "audience parameter is required")
		return
	}

	slugifier := core.NewSlugifier()
	handle := slugifier.Slugify(core.GetConfig().Project.Name)
	if handle == "" {
		handle = "layr"
	}
	layrAudience := handle + ":service_account"

	var matchingResourceServerConfig *ResourceServerConfig
	config := handler.configManager.Get()
	for _, rs := range config.OIDC.ResourceServers {
		if rs.Identifier == targetAudience {
			resourceServerConfig := rs
			matchingResourceServerConfig = &resourceServerConfig
			break
		}
	}

	if targetAudience != layrAudience && matchingResourceServerConfig == nil {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_target", "The requested audience is not a registered resource server", fmt.Sprintf("OIDC token request rejected: audience %q is not a registered resource server", targetAudience))
		return
	}

	var grantedScopes []string
	trimmedScope := strings.TrimSpace(oauthTokenInput.Scope)
	if targetAudience == layrAudience {
		if trimmedScope != "" {
			requestedScopes := strings.Fields(trimmedScope)
			var deduplicatedRequestedScopes []string
			seenScopes := make(map[string]bool)
			for _, s := range requestedScopes {
				if !seenScopes[s] {
					seenScopes[s] = true
					deduplicatedRequestedScopes = append(deduplicatedRequestedScopes, s)
				}
			}
			for _, requestedScope := range deduplicatedRequestedScopes {
				if !core.HasScope(serviceAccount.Scopes, requestedScope) {
					core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_scope", "The requested scope exceeds permissions granted to the client", fmt.Sprintf("OIDC token request rejected: scope %q exceeds service account scopes %v", requestedScope, serviceAccount.Scopes))
					return
				}
			}
			grantedScopes = deduplicatedRequestedScopes
		} else {
			grantedScopes = serviceAccount.Scopes
		}
	} else {
		if trimmedScope != "" {
			requestedScopes := strings.Fields(trimmedScope)
			var deduplicatedRequestedScopes []string
			seenScopes := make(map[string]bool)
			for _, s := range requestedScopes {
				if !seenScopes[s] {
					seenScopes[s] = true
					deduplicatedRequestedScopes = append(deduplicatedRequestedScopes, s)
				}
			}
			for _, requestedScope := range deduplicatedRequestedScopes {
				if !core.HasScope(matchingResourceServerConfig.Scopes, requestedScope) {
					core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_scope", "The requested scope is not defined for the target resource server", fmt.Sprintf("OIDC token request rejected: scope %q not defined for resource server %q", requestedScope, targetAudience))
					return
				}
			}
			grantedScopes = deduplicatedRequestedScopes
		} else {
			grantedScopes = matchingResourceServerConfig.Scopes
		}
	}

	accessToken, tokenErr := handler.kernel.JWTSigner().GenerateM2MToken(serviceAccount.ID, grantedScopes, defaultM2MTokenExpirySeconds, targetAudience)
	if tokenErr != nil {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusInternalServerError, "server_error", "Service temporarily unavailable", fmt.Sprintf("Failed to generate M2M access token: %v", tokenErr))
		return
	}

	issueOIDCTokenResponse := IssueOIDCTokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   defaultM2MTokenExpirySeconds,
		Scope:       strings.Join(grantedScopes, " "),
	}

	responseWriter.Header().Set("Content-Type", "application/json;charset=UTF-8")
	responseWriter.Header().Set("Cache-Control", "no-store")
	responseWriter.Header().Set("Pragma", "no-cache")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(issueOIDCTokenResponse)
}

// handleIssueOAuthToken delegates to handleIssueOIDCToken for OAuth 2.0 token requests.
func (handler *BaseHandler) handleIssueOAuthToken(responseWriter http.ResponseWriter, request *http.Request) {
	handler.handleIssueOIDCToken(responseWriter, request)
}

func (handler *BaseHandler) handleIssueOIDCTokenAuthorizationCode(responseWriter http.ResponseWriter, request *http.Request) {
	clientID, clientSecret, hasBasicAuth := request.BasicAuth()
	if !hasBasicAuth {
		clientID = request.FormValue("client_id")
		clientSecret = request.FormValue("client_secret")
	}

	if clientID == "" {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusUnauthorized, "invalid_client", "Missing client credentials")
		return
	}

	oidcClientConfig, ok := handler.configManager.GetOIDCClient(clientID)
	if !ok {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusUnauthorized, "invalid_client", "Invalid client credentials")
		return
	}

	// Verify confidential client secret
	if !oidcClientConfig.Public {
		decryptedSecret, _ := handler.configManager.DecryptSecret(oidcClientConfig.ClientSecret)
		if clientSecret == "" || clientSecret != decryptedSecret {
			core.WriteOAuthErrorResponse(responseWriter, http.StatusUnauthorized, "invalid_client", "Invalid client secret")
			return
		}
	}

	code := request.FormValue("code")
	redirectURI := request.FormValue("redirect_uri")
	codeVerifier := request.FormValue("code_verifier")

	if code == "" {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_grant", "Authorization code invalid or expired")
		return
	}

	codeJSON, err := handler.kernel.KVStore().Get(request.Context(), "auth:code:"+code)
	if err != nil || codeJSON == "" {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_grant", "Authorization code invalid or expired")
		return
	}

	// Delete code immediately (single-use)
	_ = handler.kernel.KVStore().Delete(request.Context(), "auth:code:"+code)

	var oidcAuthorizationCodePayload OIDCAuthorizationCodePayload
	if unmarshalErr := json.Unmarshal([]byte(codeJSON), &oidcAuthorizationCodePayload); unmarshalErr != nil {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_grant", "Authorization code invalid or expired")
		return
	}

	if oidcAuthorizationCodePayload.ClientID != clientID {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_grant", "Client mismatch")
		return
	}

	if redirectURI != "" && oidcAuthorizationCodePayload.RedirectURI != redirectURI {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_grant", "redirect_uri mismatch")
		return
	}

	// Verify PKCE S256 code challenge
	sha256Digest := sha256.Sum256([]byte(codeVerifier))
	calculatedChallenge := base64.RawURLEncoding.EncodeToString(sha256Digest[:])
	if oidcAuthorizationCodePayload.CodeChallenge == "" || oidcAuthorizationCodePayload.CodeChallenge != calculatedChallenge {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_grant", "Invalid code_verifier for PKCE challenge")
		return
	}

	// Retrieve user record
	ctx := request.Context()
	var user User
	var rawProperties []byte
	query := `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, properties, created_at, last_updated_at
		FROM auth.users
		WHERE id = $1
	`
	scanErr := handler.kernel.DB().QueryRow(ctx, query, oidcAuthorizationCodePayload.UserID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &rawProperties,
		&user.CreatedAt, &user.LastUpdatedAt,
	)
	if scanErr != nil {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusInternalServerError, "server_error", "Service temporarily unavailable", fmt.Sprintf("Failed to query user: %v", scanErr))
		return
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	config := handler.configManager.Get()
	accessExpiry := config.Sessions.AccessTokenExpirySeconds

	userEmail := ""
	if user.Email != nil {
		userEmail = *user.Email
	}
	userPhone := ""
	if user.Phone != nil {
		userPhone = *user.Phone
	}

	sessionID := uuid.NewV7().String()
	customClaims := handler.resolveCustomClaims(ctx, user.ID)
	accessToken := handler.kernel.JWTSigner().GenerateAccessToken(core.JWTClaims{
		Subject:     user.ID,
		SessionID:   sessionID,
		Email:       userEmail,
		Phone:       userPhone,
		Role:        user.Role,
		IsAnonymous: user.IsAnonymous,
		Claims:      customClaims,
	}, accessExpiry)

	refreshToken := handler.kernel.JWTSigner().GenerateRefreshToken()
	refreshTokenHash := handler.kernel.JWTSigner().HashRefreshToken(refreshToken)
	sessionExpiry := config.Sessions.RefreshTokenExpirySeconds
	expiresAt := time.Now().UTC().Add(time.Duration(sessionExpiry) * time.Second)
	_, _ = handler.kernel.DB().Exec(ctx, `
		INSERT INTO auth.sessions (id, user_id, client_id, refresh_token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, clock_timestamp())
	`, sessionID, user.ID, clientID, refreshTokenHash, expiresAt)

	handler.kernel.EventBus().Publish(ctx, NewSessionCreatedEvent(sessionID, SessionCreatedEventData{
		ID:        sessionID,
		User:      user,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}))

	baseURL := core.GetConfig().ServerBaseURL()
	idToken := handler.kernel.JWTSigner().GenerateIDToken(core.JWTClaims{
		Issuer:        baseURL,
		Subject:       user.ID,
		SessionID:     sessionID,
		Audience:      clientID,
		Nonce:         oidcAuthorizationCodePayload.Nonce,
		Email:         userEmail,
		EmailVerified: user.EmailVerifiedAt != nil,
		Phone:         userPhone,
		PhoneVerified: user.PhoneVerifiedAt != nil,
		Role:          user.Role,
		IsAnonymous:   user.IsAnonymous,
		Claims:        customClaims,
	}, accessExpiry)

	issueOIDCTokenResponse := IssueOIDCTokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    accessExpiry,
		RefreshToken: refreshToken,
		IDToken:      idToken,
		Scope:        oidcAuthorizationCodePayload.Scope,
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(issueOIDCTokenResponse)
}

func (handler *BaseHandler) handleIssueOIDCTokenRefreshToken(responseWriter http.ResponseWriter, request *http.Request) {
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
				core.WriteOAuthErrorResponse(responseWriter, http.StatusUnauthorized, "invalid_client", "Invalid client secret")
				return
			}
		}
	}

	refreshToken := request.FormValue("refresh_token")
	if refreshToken == "" {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_request", "Missing refresh_token parameter")
		return
	}

	ctx := request.Context()
	refreshTokenHash := handler.kernel.JWTSigner().HashRefreshToken(refreshToken)

	var sessionID, userID string
	err := handler.kernel.DB().QueryRow(ctx, `
		SELECT id, user_id FROM auth.sessions
		WHERE refresh_token_hash = $1 AND expires_at > clock_timestamp()
	`, refreshTokenHash).Scan(&sessionID, &userID)
	if err != nil {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_grant", "Invalid or expired refresh token")
		return
	}

	var user User
	err = handler.kernel.DB().QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until
		FROM auth.users WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous, &user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil)
	if err != nil {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_grant", "Invalid or expired refresh token")
		return
	}

	if user.LockedUntil != nil && time.Now().UTC().Before(*user.LockedUntil) {
		_, _ = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.sessions WHERE id = $1", sessionID)
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_grant", "Account is temporarily locked")
		return
	}

	// Rotate refresh token
	newRefreshToken := handler.kernel.JWTSigner().GenerateRefreshToken()
	newRefreshTokenHash := handler.kernel.JWTSigner().HashRefreshToken(newRefreshToken)
	config := handler.configManager.Get()
	sessionExpiry := config.Sessions.RefreshTokenExpirySeconds
	newExpiresAt := time.Now().UTC().Add(time.Duration(sessionExpiry) * time.Second)

	_, _ = handler.kernel.DB().Exec(ctx, `
		UPDATE auth.sessions
		SET refresh_token_hash = $1, expires_at = $2
		WHERE id = $3
	`, newRefreshTokenHash, newExpiresAt, sessionID)

	accessExpiry := config.Sessions.AccessTokenExpirySeconds

	userEmail := ""
	if user.Email != nil {
		userEmail = *user.Email
	}
	userPhone := ""
	if user.Phone != nil {
		userPhone = *user.Phone
	}

	customClaims := handler.resolveCustomClaims(ctx, user.ID)
	accessToken := handler.kernel.JWTSigner().GenerateAccessToken(core.JWTClaims{
		Subject:     user.ID,
		SessionID:   sessionID,
		Email:       userEmail,
		Phone:       userPhone,
		Role:        user.Role,
		IsAnonymous: user.IsAnonymous,
		Claims:      customClaims,
	}, accessExpiry)

	baseURL := core.GetConfig().ServerBaseURL()
	idToken := handler.kernel.JWTSigner().GenerateIDToken(core.JWTClaims{
		Issuer:        baseURL,
		Subject:       user.ID,
		SessionID:     sessionID,
		Audience:      clientID,
		Email:         userEmail,
		EmailVerified: user.EmailVerifiedAt != nil,
		Phone:         userPhone,
		PhoneVerified: user.PhoneVerifiedAt != nil,
		Role:          user.Role,
		IsAnonymous:   user.IsAnonymous,
		Claims:        customClaims,
	}, accessExpiry)

	issueOIDCTokenResponse := IssueOIDCTokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    accessExpiry,
		RefreshToken: newRefreshToken,
		IDToken:      idToken,
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(issueOIDCTokenResponse)
}

// 5. Userinfo Endpoint (GET /v1/auth/oauth/userinfo)

func (handler *BaseHandler) handleGetOIDCUserInfo(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.OIDC.Enabled {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusForbidden, "access_denied", "Access denied", "OIDC userinfo rejected: OIDC identity provider is disabled in configuration")
		return
	}

	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusUnauthorized, "invalid_token", "Bearer token required")
		return
	}

	ctx := request.Context()
	var user User
	var rawProperties []byte
	query := `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, properties, created_at, last_updated_at
		FROM auth.users WHERE id = $1
	`
	scanErr := handler.kernel.DB().QueryRow(ctx, query, authContext.UserID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &rawProperties,
		&user.CreatedAt, &user.LastUpdatedAt,
	)
	if scanErr != nil {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusUnauthorized, "invalid_token", "The access token is invalid")
		return
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	name := ""
	if nameProperty, ok := user.Properties["name"].(string); ok {
		name = nameProperty
	}

	getOIDCUserInfoResponse := GetOIDCUserInfoResponse{
		Subject:             user.ID,
		Name:                name,
		Email:               user.Email,
		EmailVerified:       user.EmailVerifiedAt != nil,
		PhoneNumber:         user.Phone,
		PhoneNumberVerified: user.PhoneVerifiedAt != nil,
		Role:                user.Role,
		IsAnonymous:         user.IsAnonymous,
		UpdatedAt:           user.LastUpdatedAt.Unix(),
		Properties:          user.Properties,
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(getOIDCUserInfoResponse)
}

// 6. Sign-Out Endpoint (GET/POST /v1/auth/oauth/sign-out)

func (handler *BaseHandler) handleSignOutOIDC(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.OIDC.Enabled {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusForbidden, "access_denied", "Access denied", "OIDC sign-out rejected: OIDC identity provider is disabled in configuration")
		return
	}

	ctx := request.Context()
	var callerUserID string
	var callerSessionID string
	var clientID string

	// 1. Check id_token_hint if provided
	idTokenHint := request.URL.Query().Get("id_token_hint")
	if idTokenHint == "" {
		_ = request.ParseForm()
		idTokenHint = request.FormValue("id_token_hint")
	}
	if idTokenHint != "" {
		jwtClaims, err := handler.kernel.JWTSigner().VerifyAccessToken(idTokenHint)
		if err == nil && jwtClaims != nil {
			callerUserID = jwtClaims.Subject
			callerSessionID = jwtClaims.SessionID
			clientID = jwtClaims.Audience
		}
	}

	// 2. Check active session cookie if user not yet resolved
	sessionToken := core.ExtractRequestSessionToken(request)
	if callerUserID == "" && sessionToken != "" {
		tokenHash := handler.kernel.JWTSigner().HashRefreshToken(sessionToken)
		_ = handler.kernel.DB().QueryRow(ctx, `
			SELECT id, user_id FROM auth.sessions WHERE refresh_token_hash = $1
		`, tokenHash).Scan(&callerSessionID, &callerUserID)
	}

	// 3. Extract client_id from query/form if not from id_token_hint
	if clientID == "" {
		clientID = request.URL.Query().Get("client_id")
		if clientID == "" {
			clientID = request.FormValue("client_id")
		}
	}

	// 4. Query active downstream client sessions for this user
	targetSessions := make([]ClientSessionInfo, 0)
	if callerUserID != "" {
		rows, err := handler.kernel.DB().Query(ctx, `
			SELECT id, client_id, refresh_token_hash
			FROM auth.sessions
			WHERE user_id = $1 AND client_id IS NOT NULL
		`, callerUserID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var targetSessionID, targetClientID, tokenHash string
				if scanErr := rows.Scan(&targetSessionID, &targetClientID, &tokenHash); scanErr == nil {
					targetSessions = append(targetSessions, ClientSessionInfo{
						ClientID:  targetClientID,
						SessionID: targetSessionID,
						UserID:    callerUserID,
					})
					if tokenHash != "" {
						_ = handler.kernel.KVStore().Delete(ctx, "auth:session:"+tokenHash)
					}
				}
			}
		}

		// Delete all active sessions for this user
		_, _ = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.sessions WHERE user_id = $1", callerUserID)
		user, _ := fetchUserByID(ctx, handler.kernel.DB(), callerUserID)
		revokedCount := len(targetSessions)
		handler.kernel.EventBus().Publish(ctx, NewSessionDeletedEvent(callerUserID, SessionDeletedEventData{
			User:         user,
			RevokedCount: &revokedCount,
		}))
	}

	if clientID != "" && (callerUserID != "" || callerSessionID != "") {
		alreadyPresent := false
		for _, targetSession := range targetSessions {
			if targetSession.ClientID == clientID {
				alreadyPresent = true
				break
			}
		}
		if !alreadyPresent {
			targetSessions = append(targetSessions, ClientSessionInfo{
				ClientID:  clientID,
				SessionID: callerSessionID,
				UserID:    callerUserID,
			})
		}
	}

	// 5. Clear Layr SSO cookie
	core.ClearSessionCookie(responseWriter, request)

	// 6. Dispatch Back-Channel Sign-Out to active clients
	if len(targetSessions) > 0 {
		dispatchBackChannelSignOut(ctx, handler.httpClient, handler.kernel.JWTSigner(), config.OIDC.Clients, targetSessions)
	}

	// 7. Validate post_sign_out_redirect_uri
	postSignOutRedirectURI := request.URL.Query().Get("post_sign_out_redirect_uri")
	if postSignOutRedirectURI == "" {
		postSignOutRedirectURI = request.FormValue("post_sign_out_redirect_uri")
	}

	state := request.URL.Query().Get("state")
	if state == "" {
		state = request.FormValue("state")
	}

	validRedirect := false
	if postSignOutRedirectURI != "" {
		hasRegisteredClients := false
		for _, registeredClient := range config.OIDC.Clients {
			if len(registeredClient.PostSignOutRedirectURIs) > 0 {
				hasRegisteredClients = true
			}
			if clientID != "" && registeredClient.ClientID != clientID {
				continue
			}
			for _, allowedURI := range registeredClient.PostSignOutRedirectURIs {
				if allowedURI == postSignOutRedirectURI {
					validRedirect = true
					break
				}
			}
			if validRedirect {
				break
			}
		}
		if !hasRegisteredClients {
			validRedirect = true
		}
	}

	// 8. Build Front-Channel Sign-Out URLs
	baseURL := core.GetConfig().ServerBaseURL()
	frontChannelURLs := buildFrontChannelSignOutURLs(baseURL, config.OIDC.Clients, targetSessions)

	// 9. If front-channel sign-out URLs exist, render hidden iframes
	if len(frontChannelURLs) > 0 {
		finalRedirectURI := ""
		if validRedirect {
			finalRedirectURI = postSignOutRedirectURI
			if state != "" {
				if parsedRedirectURL, err := url.Parse(postSignOutRedirectURI); err == nil {
					queryValues := parsedRedirectURL.Query()
					queryValues.Set("state", state)
					parsedRedirectURL.RawQuery = queryValues.Encode()
					finalRedirectURI = parsedRedirectURL.String()
				}
			}
		}

		responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
		responseWriter.WriteHeader(http.StatusOK)
		frontChannelTemplate := template.Must(template.New("frontchannel_sign_out").Parse(`<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<title>Signing Out...</title>
	{{ if .RedirectURI }}
	<meta http-equiv="refresh" content="2;url={{ .RedirectURI }}">
	{{ end }}
</head>
<body style="background:#09090b;color:#f4f4f5;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;">
	<div style="text-align:center;">
	<p>Signing out...</p>
	{{ range .FrontChannelURLs }}
	<iframe src="{{ . }}" style="display:none;width:0;height:0;border:0;"></iframe>
	{{ end }}
	</div>
	{{ if .RedirectURI }}
	<script>
		setTimeout(function() { window.location.href = {{ .RedirectURI }}; }, 1500);
	</script>
	{{ end }}
</body>
</html>`))
		_ = frontChannelTemplate.Execute(responseWriter, map[string]any{
			"RedirectURI":      finalRedirectURI,
			"FrontChannelURLs": frontChannelURLs,
		})
		return
	}

	// 10. If no front-channel URLs and valid redirect requested, redirect directly
	if validRedirect {
		redirectURL := postSignOutRedirectURI
		if state != "" {
			parsedRedirectURL, err := url.Parse(postSignOutRedirectURI)
			if err == nil {
				queryValues := parsedRedirectURL.Query()
				queryValues.Set("state", state)
				parsedRedirectURL.RawQuery = queryValues.Encode()
				redirectURL = parsedRedirectURL.String()
			}
		}
		http.Redirect(responseWriter, request, redirectURL, http.StatusFound)
		return
	}

	// Default signed-out page
	responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	responseWriter.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(responseWriter, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Signed Out</title></head><body style="background:#09090b;color:#f4f4f5;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;"><div style="text-align:center;"><h2>You have been signed out</h2><p>You can now close this window.</p></div></body></html>`)
}

// Helper methods

func redirectError(responseWriter http.ResponseWriter, request *http.Request, redirectURI, errSlug, errDescription, clientState string) {
	targetURL, err := url.Parse(redirectURI)
	if err != nil {
		core.WriteOAuthErrorResponse(responseWriter, http.StatusBadRequest, "invalid_request", errDescription)
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
	ProjectName       string
	ClientName        string
	LogoURL           string
	PrivacyPolicyURL  string
	TermsOfServiceURL string
	StateID           string
	ErrorMessage      string
	NoticeMessage     string
	CustomCSS         template.CSS
	ShowPassword      bool
	ShowSignUp        bool
	ShowPasskeys      bool
	ShowOAuth         bool
	ShowEmailOTP      bool
	ShowSMSOTP        bool
	PasskeysEnabled   bool
	Providers         []providerButtonData
	RequiresMFA       bool
	MFAToken          string
	RequiresOTP       bool
	OTPRecipient      string
	AuthMode          string
}

type providerButtonData struct {
	ID   string
	Name string
}

func (handler *BaseHandler) renderOIDCPage(
	responseWriter http.ResponseWriter,
	stateID string,
	oidcClientConfig *OIDCClientConfig,
	errorMessage string,
	noticeMessage string,
	authMode string,
	mfaToken string,
	otpRecipient string,
) {
	config := handler.configManager.Get()
	projectName := core.GetConfig().Project.Name
	if projectName == "" {
		projectName = "Layr"
	}
	clientName := ""
	if oidcClientConfig != nil && oidcClientConfig.Name != "" {
		clientName = oidcClientConfig.Name
	}

	logoURL := config.OIDC.UI.LogoURL
	customCSS := config.OIDC.UI.CustomCSS
	privacyPolicyURL := config.OIDC.UI.PrivacyPolicyURL
	termsOfServiceURL := config.OIDC.UI.TermsOfServiceURL

	showPassword := config.Password.Enabled && config.OIDC.UI.ShowPassword
	showSignUp := config.Password.Enabled && config.OIDC.UI.ShowSignUp
	showPasskeys := config.Passkeys.Enabled && config.OIDC.UI.ShowPasskeys
	showEmailOTP := config.EmailOTP.Enabled && config.OIDC.UI.ShowEmailOTP && handler.emailDispatcher.IsConfigured()
	showSMSOTP := config.SMSOTP.Enabled && config.OIDC.UI.ShowSMSOTP && handler.smsDispatcher.IsConfigured()

	var providers []providerButtonData
	if config.OIDC.UI.ShowOAuth {
		for providerKey, providerConfig := range config.OAuthProviders {
			if providerConfig.Enabled {
				name := strings.ToUpper(providerKey[:1]) + providerKey[1:]
				providers = append(providers, providerButtonData{
					ID:   providerKey,
					Name: name,
				})
			}
		}
	}

	switch authMode {
	case "", "sign_in":
		authMode = "sign_in"
	case "sign_up":
		authMode = "sign_up"
	}

	data := signInPageData{
		ProjectName:       projectName,
		ClientName:        clientName,
		LogoURL:           logoURL,
		PrivacyPolicyURL:  privacyPolicyURL,
		TermsOfServiceURL: termsOfServiceURL,
		StateID:           stateID,
		ErrorMessage:      errorMessage,
		NoticeMessage:     noticeMessage,
		CustomCSS:         template.CSS(customCSS),
		ShowPassword:      showPassword,
		ShowSignUp:        showSignUp,
		ShowPasskeys:      showPasskeys,
		ShowOAuth:         config.OIDC.UI.ShowOAuth,
		ShowEmailOTP:      showEmailOTP,
		ShowSMSOTP:        showSMSOTP,
		PasskeysEnabled:   showPasskeys,
		Providers:         providers,
		RequiresMFA:       authMode == "mfa",
		MFAToken:          mfaToken,
		RequiresOTP:       authMode == "otp_request" || authMode == "otp_verify",
		OTPRecipient:      otpRecipient,
		AuthMode:          authMode,
	}

	htmlTemplate := template.Must(template.New("signInPage").Parse(signInPageTemplateHTML))
	responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	responseWriter.WriteHeader(http.StatusOK)
	_ = htmlTemplate.Execute(responseWriter, data)
}

func (handler *BaseHandler) renderOIDCSignInPage(responseWriter http.ResponseWriter, stateID string, oidcClientConfig *OIDCClientConfig, errorMessage string) {
	handler.renderOIDCPage(responseWriter, stateID, oidcClientConfig, errorMessage, "", "sign_in", "", "")
}

func (handler *BaseHandler) renderOIDCSignUpPage(responseWriter http.ResponseWriter, stateID string, oidcClientConfig *OIDCClientConfig, errorMessage string) {
	handler.renderOIDCPage(responseWriter, stateID, oidcClientConfig, errorMessage, "", "sign_up", "", "")
}

func (handler *BaseHandler) renderOIDCMFAPage(responseWriter http.ResponseWriter, stateID string, oidcClientConfig *OIDCClientConfig, errorMessage string, noticeMessage string, mfaToken string) {
	handler.renderOIDCPage(responseWriter, stateID, oidcClientConfig, errorMessage, noticeMessage, "mfa", mfaToken, "")
}

func (handler *BaseHandler) renderOIDCOTPRequestPage(responseWriter http.ResponseWriter, stateID string, oidcClientConfig *OIDCClientConfig, errorMessage string, noticeMessage string) {
	handler.renderOIDCPage(responseWriter, stateID, oidcClientConfig, errorMessage, noticeMessage, "otp_request", "", "")
}

func (handler *BaseHandler) renderOIDCOTPVerifyPage(responseWriter http.ResponseWriter, stateID string, oidcClientConfig *OIDCClientConfig, errorMessage string, noticeMessage string, recipient string) {
	handler.renderOIDCPage(responseWriter, stateID, oidcClientConfig, errorMessage, noticeMessage, "otp_verify", "", recipient)
}

const signInPageTemplateHTML = `<!DOCTYPE html>
<html lang="en" class="dark">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{if eq .AuthMode "sign_up"}}Sign up{{else if eq .AuthMode "mfa"}}Two-Factor Authentication{{else if or (eq .AuthMode "otp_request") (eq .AuthMode "otp_verify")}}Sign in with Code{{else}}Sign in{{end}}{{if .ClientName}} to {{.ClientName}}{{end}} · {{.ProjectName}}</title>
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
      --info: #3b82f6;
      --info-bg: rgba(59, 130, 246, 0.12);
      --info-border: rgba(59, 130, 246, 0.3);
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
    .sign-in-card {
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      border-radius: 1rem;
      padding: 2.25rem 2rem;
      box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.65);
    }
    .brand-header {
      margin-bottom: 1.5rem;
      text-align: left;
    }
    .brand-logo-icon {
      width: 2.25rem;
      height: 2.25rem;
      margin-bottom: 0.75rem;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      background: var(--brand-accent);
      color: #000;
      border-radius: 0.5rem;
    }
    .brand-logo-img {
      max-height: 48px;
      margin-bottom: 1rem;
      object-fit: contain;
    }
    .brand-header h1 {
      font-size: 1.35rem;
      font-weight: 700;
      color: var(--fg);
      margin: 0;
      letter-spacing: -0.025em;
    }
    .brand-header .subtitle {
      font-size: 0.875rem;
      color: var(--fg-muted);
      margin-top: 0.25rem;
      margin-bottom: 0;
    }
    .alert {
      padding: 0.75rem 1rem;
      border-radius: 0.5rem;
      font-size: 0.875rem;
      margin-bottom: 1.25rem;
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }
    .alert-error {
      background: rgba(239, 68, 68, 0.1);
      border: 1px solid rgba(239, 68, 68, 0.2);
      color: #f87171;
    }
    .alert-notice {
      background: rgba(59, 130, 246, 0.1);
      border: 1px solid rgba(59, 130, 246, 0.2);
      color: #60a5fa;
    }
    .auth-tabs {
      margin-bottom: 1.25rem;
    }
    .auth-tabs nav {
      display: flex;
      background: rgba(255, 255, 255, 0.05);
      border-radius: 0.5rem;
      padding: 3px;
      gap: 3px;
    }
    .auth-tab {
      flex: 1;
      background: none;
      border: none;
      border-bottom: 2px solid transparent;
      padding: 0.625rem 0;
      font-size: 0.875rem;
      font-weight: 500;
      color: var(--muted);
      cursor: pointer;
      text-align: center;
      transition: all 0.15s ease;
    }
    .auth-tab:hover {
      color: var(--fg);
    }
    .auth-tab.active {
      color: var(--fg);
      font-weight: 600;
      border-bottom-color: var(--primary);
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
    .alert-notice {
      background: var(--info-bg);
      border: 1px solid var(--info-border);
      color: var(--info);
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
    .secondary-btn {
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
      text-align: center;
      transition: background-color 0.15s ease;
      display: block;
    }
    .secondary-btn:hover {
      background: #3f3f46;
    }
    .back-link {
      display: block;
      font-size: 0.8125rem;
      color: var(--muted);
      text-decoration: none;
      margin-top: 1rem;
      text-align: center;
      cursor: pointer;
      background: none;
      border: none;
      width: 100%;
    }
    .back-link:hover {
      color: var(--fg);
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
    .legal-footer {
      margin-top: 1.5rem;
      text-align: center;
      font-size: 0.75rem;
      color: var(--muted);
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 0.5rem;
    }
    .legal-footer a {
      color: var(--muted);
      text-decoration: none;
      transition: color 0.15s ease;
    }
    .legal-footer a:hover {
      color: var(--fg);
    }
    .legal-footer .dot {
      opacity: 0.5;
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
    <div class="card sign-in-card">
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
        <h1>{{if eq .AuthMode "sign_up"}}Create account{{else if eq .AuthMode "mfa"}}Two-factor challenge{{else if or (eq .AuthMode "otp_request") (eq .AuthMode "otp_verify")}}Sign in with code{{else}}Sign in{{end}}</h1>
        {{if .ClientName}}
        <p class="subtitle">Continue to {{.ClientName}}</p>
        {{end}}
      </div>

      {{if .ErrorMessage}}
        <div class="alert alert-error" data-variant="destructive">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
          <span>{{.ErrorMessage}}</span>
        </div>
      {{end}}

      {{if .NoticeMessage}}
        <div class="alert alert-notice" data-variant="secondary">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="8"/>
          </svg>
          <span>{{.NoticeMessage}}</span>
        </div>
      {{end}}

      {{if eq .AuthMode "mfa"}}
        <!-- MFA Challenge Form -->
        <form action="/v1/auth/oauth/authorize" method="POST">
          <input type="hidden" name="state" value="{{.StateID}}">
          <input type="hidden" name="action" value="verify_mfa">
          <input type="hidden" name="mfa_token" value="{{.MFAToken}}">

          <div role="group" class="field">
            <label for="mfa-code">Authenticator Code</label>
            <input id="mfa-code" name="mfa_code" type="text" autocomplete="one-time-code" required autofocus placeholder="6-digit code" class="input input-text">
          </div>

          <button type="submit" class="btn submit-btn w-full">Verify Code</button>
          <a href="/v1/auth/oauth/authorize?state={{.StateID}}" class="btn back-link" data-variant="link">Back to sign in</a>
        </form>
      {{else if eq .AuthMode "otp_verify"}}
        <!-- OTP Code Verification Form -->
        <form action="/v1/auth/oauth/authorize" method="POST">
          <input type="hidden" name="state" value="{{.StateID}}">
          <input type="hidden" name="action" value="verify_otp">
          <input type="hidden" name="recipient" value="{{.OTPRecipient}}">

          <div role="group" class="field">
            <label for="otp-code">Verification Code</label>
            <input id="otp-code" name="otp_code" type="text" autocomplete="one-time-code" required autofocus placeholder="6-digit code" class="input input-text">
          </div>

          <button type="submit" class="btn submit-btn w-full">Verify & Sign in</button>
          <a href="/v1/auth/oauth/authorize?state={{.StateID}}" class="btn back-link" data-variant="link">Back to sign in</a>
        </form>
      {{else}}
        <!-- Standard Sign-in / Sign-up / OTP-request container -->
        {{if and .ShowSignUp .ShowPassword}}
          <div class="tabs auth-tabs w-full">
            <nav role="tablist">
              <button type="button" role="tab" id="tab-sign-in" aria-selected="{{if eq .AuthMode "sign_up"}}false{{else}}true{{end}}" class="btn auth-tab {{if ne .AuthMode "sign_up"}}active{{end}}" onclick="showTab('sign_in')">Sign in</button>
              <button type="button" role="tab" id="tab-sign-up" aria-selected="{{if eq .AuthMode "sign_up"}}true{{else}}false{{end}}" class="btn auth-tab {{if eq .AuthMode "sign_up"}}active{{end}}" onclick="showTab('sign_up')">Sign up</button>
            </nav>
          </div>
        {{end}}

        <!-- Sign-in form -->
        <div id="sign-in-container" class="sign-in-container" style="{{if or (eq .AuthMode "sign_up") (eq .AuthMode "otp_request")}}display:none;{{end}}">
          {{if .ShowPassword}}
            <form action="/v1/auth/oauth/authorize" method="POST">
              <input type="hidden" name="state" value="{{.StateID}}">
              <input type="hidden" name="action" value="sign_in">

              <div role="group" class="field">
                <label for="sign-in-email">Email</label>
                <input id="sign-in-email" name="email" type="email" autocomplete="email" required autofocus placeholder="name@example.com" class="input input-text">
              </div>

              <div role="group" class="field">
                <label for="sign-in-password">Password</label>
                <input id="sign-in-password" name="password" type="password" autocomplete="current-password" required placeholder="••••••••••••" class="input input-text">
              </div>

              <button type="submit" class="btn submit-btn w-full">Sign in</button>
            </form>
          {{end}}

          {{if .PasskeysEnabled}}
            <button type="button" class="btn secondary-btn w-full" data-variant="outline" id="passkey-btn" onclick="handlePasskeySignIn()">
              Sign in with Passkey
            </button>
          {{end}}

          {{if or .ShowEmailOTP .ShowSMSOTP}}
            <button type="button" class="btn secondary-btn w-full" data-variant="outline" onclick="showTab('otp')">
              Sign in with One-Time Code
            </button>
          {{end}}

          {{if .Providers}}
            <div class="divider"><span>Or</span></div>
            <div class="providers-grid">
              {{range .Providers}}
                <a href="/v1/auth/oauth/{{.ID}}/authorize?oidc_state={{$.StateID}}" class="btn provider-btn w-full" data-variant="outline">
                  Sign in with {{.Name}}
                </a>
              {{end}}
            </div>
          {{end}}
        </div>

        <!-- Sign-up form -->
        {{if and .ShowSignUp .ShowPassword}}
          <div id="sign-up-container" class="sign-up-container" style="{{if ne .AuthMode "sign_up"}}display:none;{{end}}">
            <form action="/v1/auth/oauth/authorize" method="POST">
              <input type="hidden" name="state" value="{{.StateID}}">
              <input type="hidden" name="action" value="sign_up">

              <div role="group" class="field">
                <label for="sign-up-email">Email</label>
                <input id="sign-up-email" name="email" type="email" autocomplete="email" required placeholder="name@example.com" class="input input-text">
              </div>

              <div role="group" class="field">
                <label for="sign-up-password">Password</label>
                <input id="sign-up-password" name="password" type="password" autocomplete="new-password" required placeholder="••••••••••••" class="input input-text">
              </div>

              <div role="group" class="field">
                <label for="sign-up-confirm-password">Confirm Password</label>
                <input id="sign-up-confirm-password" name="confirm_password" type="password" autocomplete="new-password" required placeholder="••••••••••••" class="input input-text">
              </div>

              <button type="submit" class="btn submit-btn w-full">Sign up</button>
            </form>
          </div>
        {{end}}

        <!-- OTP Request form -->
        {{if or .ShowEmailOTP .ShowSMSOTP}}
          <div id="otp-container" style="{{if ne .AuthMode "otp_request"}}display:none;{{end}}">
            <form action="/v1/auth/oauth/authorize" method="POST">
              <input type="hidden" name="state" value="{{.StateID}}">
              <input type="hidden" name="action" value="send_otp">

              <div role="group" class="field">
                <label for="otp-recipient">{{if and .ShowEmailOTP .ShowSMSOTP}}Email or Phone Number{{else if .ShowEmailOTP}}Email{{else}}Phone Number{{end}}</label>
                <input id="otp-recipient" name="recipient" type="text" required placeholder="{{if and .ShowEmailOTP .ShowSMSOTP}}name@example.com or +1234567890{{else if .ShowEmailOTP}}name@example.com{{else}}+1234567890{{end}}" class="input input-text">
              </div>

              <button type="submit" class="btn submit-btn w-full">Send verification code</button>
              <button type="button" class="btn back-link" data-variant="link" onclick="showTab('sign_in')">Back to sign in</button>
            </form>
          </div>
        {{end}}
      {{end}}

      {{if or .PrivacyPolicyURL .TermsOfServiceURL}}
        <div class="legal-footer">
          {{if .TermsOfServiceURL}}<a href="{{.TermsOfServiceURL}}" target="_blank" rel="noopener noreferrer">Terms of Service</a>{{end}}
          {{if and .PrivacyPolicyURL .TermsOfServiceURL}}<span class="dot">·</span>{{end}}
          {{if .PrivacyPolicyURL}}<a href="{{.PrivacyPolicyURL}}" target="_blank" rel="noopener noreferrer">Privacy Policy</a>{{end}}
        </div>
      {{end}}
    </div>
  </div>
  <script>
    function showTab(tab) {
      var isSignIn = (tab === 'sign_in');
      var isSignUp = (tab === 'sign_up');
      var isOTP = (tab === 'otp');

      var signIn = document.getElementById('sign-in-container');
      var signUp = document.getElementById('sign-up-container');
      var otp = document.getElementById('otp-container');
      if (signIn) signIn.style.display = isSignIn ? 'block' : 'none';
      if (signUp) signUp.style.display = isSignUp ? 'block' : 'none';
      if (otp) otp.style.display = isOTP ? 'block' : 'none';

      var tabSignIn = document.getElementById('tab-sign-in');
      var tabSignUp = document.getElementById('tab-sign-up');
      if (tabSignIn) {
        if (isSignIn) {
          tabSignIn.classList.add('active');
          tabSignIn.setAttribute('aria-selected', 'true');
        } else {
          tabSignIn.classList.remove('active');
          tabSignIn.setAttribute('aria-selected', 'false');
        }
      }
      if (tabSignUp) {
        if (isSignUp) {
          tabSignUp.classList.add('active');
          tabSignUp.setAttribute('aria-selected', 'true');
        } else {
          tabSignUp.classList.remove('active');
          tabSignUp.setAttribute('aria-selected', 'false');
        }
      }
    }

    async function handlePasskeySignIn() {
      if (!window.PublicKeyCredential) {
        alert('Passkeys are not supported on this browser or device.');
        return;
      }
      var btn = document.getElementById('passkey-btn');
      if (btn) btn.disabled = true;
      try {
        var beginRes = await fetch('/v1/auth/passkeys/sign-in', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'same-origin'
        });
        if (!beginRes.ok) {
          var errData = await beginRes.json().catch(function() { return {}; });
          throw new Error(errData.message || 'Failed to initialize passkey sign-in');
        }
        var options = await beginRes.json();
        var binaryString = atob(options.challenge.replace(/-/g, '+').replace(/_/g, '/'));
        var challengeBytes = new Uint8Array(binaryString.length);
        for (var i = 0; i < binaryString.length; i++) {
          challengeBytes[i] = binaryString.charCodeAt(i);
        }
        var assertion = await navigator.credentials.get({
          publicKey: {
            challenge: challengeBytes,
            rpId: options.rp_id,
            userVerification: 'preferred'
          }
        });
        if (!assertion) {
          if (btn) btn.disabled = false;
          return;
        }

        var rawIdBytes = new Uint8Array(assertion.rawId);
        var rawIdStr = '';
        for (var j = 0; j < rawIdBytes.length; j++) {
          rawIdStr += String.fromCharCode(rawIdBytes[j]);
        }
        var credentialId = btoa(rawIdStr).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');

        var clientDataBytes = new Uint8Array(assertion.response.clientDataJSON);
        var clientDataStr = '';
        for (var k = 0; k < clientDataBytes.length; k++) {
          clientDataStr += String.fromCharCode(clientDataBytes[k]);
        }
        var clientData = btoa(clientDataStr);

        var authDataBytes = new Uint8Array(assertion.response.authenticatorData);
        var authDataStr = '';
        for (var l = 0; l < authDataBytes.length; l++) {
          authDataStr += String.fromCharCode(authDataBytes[l]);
        }
        var authData = btoa(authDataStr);

        var sigBytes = new Uint8Array(assertion.response.signature);
        var sigStr = '';
        for (var m = 0; m < sigBytes.length; m++) {
          sigStr += String.fromCharCode(sigBytes[m]);
        }
        var signature = btoa(sigStr);

        var verifyRes = await fetch('/v1/auth/passkeys/sign-in/verify', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'same-origin',
          body: JSON.stringify({
            challenge: options.challenge,
            credential_id: credentialId,
            client_data_json: clientData,
            authenticator_data: authData,
            signature: signature
          })
        });
        if (!verifyRes.ok) {
          var verifyErrData = await verifyRes.json().catch(function() { return {}; });
          throw new Error(verifyErrData.message || 'Passkey verification failed');
        }
        window.location.reload();
      } catch (err) {
        if (err.name !== 'NotAllowedError') {
          alert(err.message || 'Passkey sign-in failed');
        }
        if (btn) btn.disabled = false;
      }
    }
  </script>
</body>
</html>`
