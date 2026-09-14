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
	"layr.sh/auth/otp"
	"layr.sh/core"
)

const defaultM2MTokenExpirySeconds = 3600
const defaultOIDCMFATTL = 5 * time.Minute

// 1. OIDC Discovery & JWKS

func (handler *BaseHandler) handleOIDCDiscovery(responseWriter http.ResponseWriter, request *http.Request) {
	baseURL := core.GetConfig().ServerBaseURL()
	oidcConfiguration := jwt.BuildOIDCDiscovery(baseURL)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(oidcConfiguration)
}

func (handler *BaseHandler) handleJWKS(responseWriter http.ResponseWriter, request *http.Request) {
	jwks := handler.signer.BuildJWKS()
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(jwks)
}

// 2. Authorization Endpoint (GET /api/v1/auth/oauth/authorize)

func (handler *BaseHandler) handleOIDCAuthorize(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.OIDC.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "OIDC Identity Provider is disabled by the console user", "oidc_disabled")
		return
	}

	stateIDParam := request.URL.Query().Get("state")
	modeParam := request.URL.Query().Get("mode")
	if stateIDParam != "" && modeParam != "" && handler.kvStore != nil {
		stateJSON, err := handler.kvStore.Get(request.Context(), "auth:oidc:state:"+stateIDParam)
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

func (handler *BaseHandler) completeOIDCAuthorization(
	responseWriter http.ResponseWriter,
	request *http.Request,
	stateID string,
	oidcAuthorizationStatePayload OIDCAuthorizationStatePayload,
	userRecord UserRecord,
) {
	ctx := request.Context()
	config := handler.configManager.Get()

	// Delete state payload to prevent replay / CSRF fixation
	if handler.kvStore != nil {
		_ = handler.kvStore.Delete(ctx, "auth:oidc:state:"+stateID)
	}

	// Issue authorization code
	code := handler.issueOIDCAuthorizationCode(
		ctx,
		oidcAuthorizationStatePayload.ClientID,
		oidcAuthorizationStatePayload.RedirectURI,
		userRecord.ID,
		oidcAuthorizationStatePayload.Scope,
		oidcAuthorizationStatePayload.CodeChallenge,
		oidcAuthorizationStatePayload.CodeChallengeMethod,
		oidcAuthorizationStatePayload.Nonce,
	)

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
			User:      userRecord,
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

// 3. Authorization Form Submission (POST /api/v1/auth/oauth/authorize)

func (handler *BaseHandler) handleOIDCAuthorizeSubmit(responseWriter http.ResponseWriter, request *http.Request) {
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

	ctx := request.Context()
	stateJSON, err := handler.kvStore.Get(ctx, "auth:oidc:state:"+stateID)
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
		var mfaUserID string
		if handler.kvStore != nil {
			mfaUserID, _ = handler.kvStore.Get(ctx, "auth:oidc:mfa:"+mfaToken)
		}
		if mfaUserID == "" {
			handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "MFA session expired. Please sign in again.")
			return
		}
		if mfaCode == "" {
			handler.renderOIDCMFAPage(responseWriter, stateID, oidcClientConfig, "Two-factor authentication code is required", "", mfaToken)
			return
		}

		var userRecord UserRecord
		var rawProperties []byte
		query := `
			SELECT id, email, phone, password_hash, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
			FROM auth.users
			WHERE id = $1
		`
		scanErr := handler.db.QueryRow(ctx, query, mfaUserID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.PasswordHash,
			&userRecord.Role, &userRecord.IsAnonymous, &userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt,
			&userRecord.LockedUntil, &userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
			&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
		)
		if scanErr != nil {
			handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "User account not found")
			return
		}

		if userRecord.EncryptedMFASecret == nil || *userRecord.EncryptedMFASecret == "" {
			handler.renderOIDCMFAPage(responseWriter, stateID, oidcClientConfig, "Multi-factor authentication configuration error", "", mfaToken)
			return
		}
		secretBytes, decryptErr := handler.cryptoKeyManager.DecryptField(*userRecord.EncryptedMFASecret)
		if decryptErr != nil {
			log.Errorf("failed to decrypt MFA secret for user %s: %v", userRecord.ID, decryptErr)
			handler.renderOIDCMFAPage(responseWriter, stateID, oidcClientConfig, "Failed to verify multi-factor authentication", "", mfaToken)
			return
		}
		if !handler.totpManager.ValidateCode(string(secretBytes), mfaCode, time.Now().UTC(), 1) {
			log.Debugf("invalid MFA code supplied during OIDC MFA challenge for user %s", userRecord.ID)
			handler.renderOIDCMFAPage(responseWriter, stateID, oidcClientConfig, "Invalid two-factor authentication code", "", mfaToken)
			return
		}

		if handler.kvStore != nil {
			_ = handler.kvStore.Delete(ctx, "auth:oidc:mfa:"+mfaToken)
		}
		handler.completeOIDCAuthorization(responseWriter, request, stateID, oidcAuthorizationStatePayload, userRecord)
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
			if !config.EmailOTP.Enabled || !config.OIDC.UI.ShowEmailOTP || handler.emailDispatcher == nil || !handler.emailDispatcher.IsConfigured() {
				handler.renderOIDCOTPRequestPage(responseWriter, stateID, oidcClientConfig, "Email OTP sign-in is not available", "")
				return
			}
			recipient = strings.ToLower(recipient)
			expiryMinutes = config.EmailOTP.TokenExpiryMinutes
		} else {
			if !config.SMSOTP.Enabled || !config.OIDC.UI.ShowSMSOTP || handler.smsDispatcher == nil || !handler.smsDispatcher.IsConfigured() {
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

		if handler.kvStore != nil {
			clientIP := core.ExtractRequestClientIP(request)
			ipRateKey := fmt.Sprintf("auth:ratelimit:otp:ip:%s", clientIP)
			if count, err := handler.kvStore.Increment(ctx, ipRateKey, time.Hour); err == nil && count > 10 {
				handler.renderOIDCOTPRequestPage(responseWriter, stateID, oidcClientConfig, "Rate limit exceeded. Too many requests from this IP address.", recipient)
				return
			}

			cooldownKey := fmt.Sprintf("auth:cooldown:sign_in:%s", recipient)
			if _, err := handler.kvStore.Get(ctx, cooldownKey); err == nil {
				handler.renderOIDCOTPRequestPage(responseWriter, stateID, oidcClientConfig, "Please wait 60 seconds before requesting another code", recipient)
				return
			}
		}

		codeTTL := time.Duration(expiryMinutes) * time.Minute
		code, _ := otp.GenerateCode(nil)
		codeHash := otp.HashCode(code)

		query := `
			INSERT INTO auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
			VALUES ($1, $2, 'sign_in', 0, $3, clock_timestamp())
		`
		expiresAt := time.Now().UTC().Add(codeTTL)
		_, _ = handler.db.Exec(ctx, query, recipient, codeHash, expiresAt)
		if handler.kvStore != nil {
			_ = handler.kvStore.Set(ctx, fmt.Sprintf("auth:otp:sign_in:%s", recipient), code, codeTTL)
			_ = handler.kvStore.Set(ctx, fmt.Sprintf("auth:cooldown:sign_in:%s", recipient), "1", defaultOTPCooldown)
		}

		channel := "sms"
		if isEmail {
			channel = "email"
			_ = handler.emailDispatcher.SendSignInOTP(ctx, recipient, code, "")
		} else {
			_ = handler.smsDispatcher.SendSignInOTP(ctx, recipient, code, "")
		}

		if handler.eventBus != nil {
			var targetUserRecord *UserRecord
			if fetchedUserRecord, fetchErr := fetchUserRecordByRecipient(ctx, handler.db, recipient); fetchErr == nil {
				targetUserRecord = &fetchedUserRecord
			}
			handler.eventBus.Publish(ctx, NewOTPSentEvent(recipient, OTPSentEventData{
				Recipient: recipient,
				Purpose:   "sign_in",
				Channel:   channel,
				User:      targetUserRecord,
			}))
		}

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
		err := handler.db.QueryRow(ctx, `
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
			_, _ = handler.db.Exec(ctx, "UPDATE auth.otps SET attempts = attempts + 1 WHERE id = $1", otpID)
			handler.renderOIDCOTPVerifyPage(responseWriter, stateID, oidcClientConfig, "Invalid or expired verification code", "", recipient)
			return
		}

		_, _ = handler.db.Exec(ctx, "DELETE FROM auth.otps WHERE id = $1", otpID)
		if handler.kvStore != nil {
			_ = handler.kvStore.Delete(ctx, fmt.Sprintf("auth:otp:sign_in:%s", recipient))
		}

		var userRecord UserRecord
		var rawProperties []byte
		var isNewUser bool
		if isEmail {
			_ = handler.db.QueryRow(ctx, `
				INSERT INTO auth.users (email, role, email_verified_at, created_at, last_updated_at)
				VALUES ($1, 'authenticated', clock_timestamp(), clock_timestamp(), clock_timestamp())
				ON CONFLICT (email) DO UPDATE SET email_verified_at = COALESCE(auth.users.email_verified_at, clock_timestamp()), last_updated_at = clock_timestamp()
				RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at, (xmax = 0) AS is_new
			`, recipient).Scan(
				&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
				&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
				&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
				&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt, &isNewUser,
			)
		} else {
			_ = handler.db.QueryRow(ctx, `
				INSERT INTO auth.users (phone, role, phone_verified_at, created_at, last_updated_at)
				VALUES ($1, 'authenticated', clock_timestamp(), clock_timestamp(), clock_timestamp())
				ON CONFLICT (phone) DO UPDATE SET phone_verified_at = COALESCE(auth.users.phone_verified_at, clock_timestamp()), last_updated_at = clock_timestamp()
				RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at, (xmax = 0) AS is_new
			`, recipient).Scan(
				&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
				&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
				&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
				&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt, &isNewUser,
			)
		}

		if !isNewUser && userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
			handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Account temporarily locked. Please try again later.")
			return
		}

		channel := "sms"
		if isEmail {
			channel = "email"
		}
		if handler.eventBus != nil {
			if isNewUser {
				handler.eventBus.Publish(ctx, NewUserSignedUpEvent(userRecord.ID, UserSignedUpEventData(userRecord)))
			}
			handler.eventBus.Publish(ctx, NewOTPVerifiedEvent(recipient, OTPVerifiedEventData{
				Recipient: recipient,
				Purpose:   "sign_in",
				Channel:   channel,
				User:      &userRecord,
			}))
		}

		if userRecord.MFAEnabled {
			mfaToken := "mfa_oidc_" + uuid.NewV7().String()
			if handler.kvStore != nil {
				_ = handler.kvStore.Set(ctx, "auth:oidc:mfa:"+mfaToken, userRecord.ID, defaultOIDCMFATTL)
			}
			handler.renderOIDCMFAPage(responseWriter, stateID, oidcClientConfig, "", "Two-factor authentication required", mfaToken)
			return
		}

		handler.completeOIDCAuthorization(responseWriter, request, stateID, oidcAuthorizationStatePayload, userRecord)
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
		err := handler.db.QueryRow(ctx, "SELECT id FROM auth.users WHERE email = $1", email).Scan(&existingID)
		if err == nil {
			handler.renderOIDCSignUpPage(responseWriter, stateID, oidcClientConfig, "An account with this email already exists")
			return
		}

		passHash, _ := handler.hasher.Hash(password)
		var userRecord UserRecord
		var rawProperties []byte
		userID := uuid.NewV7().String()
		query := `
			INSERT INTO auth.users (id, email, password_hash, role, is_anonymous, created_at, last_updated_at)
			VALUES ($1, $2, $3, 'authenticated', false, clock_timestamp(), clock_timestamp())
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		`
		err = handler.db.QueryRow(ctx, query, userID, email, passHash).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
			&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
			&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
		)
		if err != nil {
			log.Debugf("failed to create user in OIDC sign up: %v", err)
			handler.renderOIDCSignUpPage(responseWriter, stateID, oidcClientConfig, "Failed to create account")
			return
		}

		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewUserSignedUpEvent(userRecord.ID, UserSignedUpEventData(userRecord)))
		}

		handler.completeOIDCAuthorization(responseWriter, request, stateID, oidcAuthorizationStatePayload, userRecord)
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
	var userRecord UserRecord
	var rawProperties []byte
	query := `
		SELECT id, email, phone, password_hash, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE email = $1
	`
	scanErr := handler.db.QueryRow(ctx, query, email).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.PasswordHash,
		&userRecord.Role, &userRecord.IsAnonymous, &userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt,
		&userRecord.LockedUntil, &userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
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

	if userRecord.MFAEnabled {
		mfaCode := strings.TrimSpace(request.FormValue("mfa_code"))
		if mfaCode != "" {
			if userRecord.EncryptedMFASecret == nil || *userRecord.EncryptedMFASecret == "" {
				handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Multi-factor authentication configuration error")
				return
			}
			secretBytes, err := handler.cryptoKeyManager.DecryptField(*userRecord.EncryptedMFASecret)
			if err != nil {
				log.Errorf("failed to decrypt MFA secret for user %s: %v", userRecord.ID, err)
				handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Failed to verify multi-factor authentication")
				return
			}
			if !handler.totpManager.ValidateCode(string(secretBytes), mfaCode, time.Now().UTC(), 1) {
				log.Debugf("invalid MFA code supplied during OIDC authorize submit for user %s", userRecord.ID)
				handler.renderOIDCSignInPage(responseWriter, stateID, oidcClientConfig, "Invalid two-factor authentication code")
				return
			}
		} else {
			mfaToken := "mfa_oidc_" + uuid.NewV7().String()
			if handler.kvStore != nil {
				_ = handler.kvStore.Set(ctx, "auth:oidc:mfa:"+mfaToken, userRecord.ID, defaultOIDCMFATTL)
			}
			handler.renderOIDCMFAPage(responseWriter, stateID, oidcClientConfig, "", "Two-factor authentication required", mfaToken)
			return
		}
	}

	handler.completeOIDCAuthorization(responseWriter, request, stateID, oidcAuthorizationStatePayload, userRecord)
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

func (handler *BaseHandler) handleOIDCToken(responseWriter http.ResponseWriter, request *http.Request) {
	oauthTokenRequest := parseOAuthTokenRequest(request)

	// If no grant_type or if provider parameter is present, delegate to handleOAuthCallback
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

func (handler *BaseHandler) handleOAuthClientCredentials(responseWriter http.ResponseWriter, request *http.Request, oauthTokenRequest OAuthTokenRequest) {
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
func (handler *BaseHandler) handleOAuthToken(responseWriter http.ResponseWriter, request *http.Request) {
	handler.handleOIDCToken(responseWriter, request)
}

func (handler *BaseHandler) handleOIDCTokenAuthorizationCode(responseWriter http.ResponseWriter, request *http.Request) {
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
			User:      userRecord,
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

func (handler *BaseHandler) handleOIDCTokenRefreshToken(responseWriter http.ResponseWriter, request *http.Request) {
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

	var userRecord UserRecord
	err = handler.db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until
		FROM auth.users WHERE id = $1
	`, userID).Scan(&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous, &userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "User not found", "invalid_grant")
		return
	}

	if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
		log.Warnf("failed OIDC token refresh for locked user %s", userRecord.ID)
		_, _ = handler.db.Exec(ctx, "DELETE FROM auth.sessions WHERE id = $1", sessionID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Account is temporarily locked", "invalid_grant")
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

func (handler *BaseHandler) handleOIDCUserInfo(responseWriter http.ResponseWriter, request *http.Request) {
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

func (handler *BaseHandler) handleOIDCSignOut(responseWriter http.ResponseWriter, request *http.Request) {
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
	showEmailOTP := config.EmailOTP.Enabled && config.OIDC.UI.ShowEmailOTP && (handler.emailDispatcher != nil && handler.emailDispatcher.IsConfigured())
	showSMSOTP := config.SMSOTP.Enabled && config.OIDC.UI.ShowSMSOTP && (handler.smsDispatcher != nil && handler.smsDispatcher.IsConfigured())

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
        <form action="/api/v1/auth/oauth/authorize" method="POST">
          <input type="hidden" name="state" value="{{.StateID}}">
          <input type="hidden" name="action" value="verify_mfa">
          <input type="hidden" name="mfa_token" value="{{.MFAToken}}">

          <div role="group" class="field">
            <label for="mfa-code">Authenticator Code</label>
            <input id="mfa-code" name="mfa_code" type="text" autocomplete="one-time-code" required autofocus placeholder="6-digit code" class="input input-text">
          </div>

          <button type="submit" class="btn submit-btn w-full">Verify Code</button>
          <a href="/api/v1/auth/oauth/authorize?state={{.StateID}}" class="btn back-link" data-variant="link">Back to sign in</a>
        </form>
      {{else if eq .AuthMode "otp_verify"}}
        <!-- OTP Code Verification Form -->
        <form action="/api/v1/auth/oauth/authorize" method="POST">
          <input type="hidden" name="state" value="{{.StateID}}">
          <input type="hidden" name="action" value="verify_otp">
          <input type="hidden" name="recipient" value="{{.OTPRecipient}}">

          <div role="group" class="field">
            <label for="otp-code">Verification Code</label>
            <input id="otp-code" name="otp_code" type="text" autocomplete="one-time-code" required autofocus placeholder="6-digit code" class="input input-text">
          </div>

          <button type="submit" class="btn submit-btn w-full">Verify & Sign in</button>
          <a href="/api/v1/auth/oauth/authorize?state={{.StateID}}" class="btn back-link" data-variant="link">Back to sign in</a>
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
            <form action="/api/v1/auth/oauth/authorize" method="POST">
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
                <a href="/api/v1/auth/oauth/{{.ID}}/authorize?oidc_state={{$.StateID}}" class="btn provider-btn w-full" data-variant="outline">
                  Sign in with {{.Name}}
                </a>
              {{end}}
            </div>
          {{end}}
        </div>

        <!-- Sign-up form -->
        {{if and .ShowSignUp .ShowPassword}}
          <div id="sign-up-container" class="sign-up-container" style="{{if ne .AuthMode "sign_up"}}display:none;{{end}}">
            <form action="/api/v1/auth/oauth/authorize" method="POST">
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
            <form action="/api/v1/auth/oauth/authorize" method="POST">
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
        var beginRes = await fetch('/api/v1/auth/passkeys/sign-in', {
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

        var verifyRes = await fetch('/api/v1/auth/passkeys/sign-in/verify', {
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
