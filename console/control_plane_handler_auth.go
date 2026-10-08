// Package console provides the administrative control plane, operator identity, and dynamic configuration engine.
package console

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"layr.sh/core"
)

func (controlPlaneHandler *ControlPlaneHandler) extractConsoleToken(request *http.Request) string {
	if request == nil {
		return ""
	}
	if cookie, err := request.Cookie(core.ConsoleCookieNameSecure); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	if cookie, err := request.Cookie(core.ConsoleCookieNameInsecure); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return core.ExtractRequestSessionToken(request)
}

func (controlPlaneHandler *ControlPlaneHandler) authenticateSession(request *http.Request) (*User, *Session, error) {
	token := controlPlaneHandler.extractConsoleToken(request)
	if token == "" {
		return nil, nil, errors.New("authentication required")
	}

	tokenHash := controlPlaneHandler.kernel.JWTSigner().HashRefreshToken(token)

	ctx := request.Context()
	const selectSQLStatement = `
		SELECT
			s.id, s.user_id, host(s.ip_address), COALESCE(s.user_agent, ''), s.expires_at, s.created_at,
			u.id, u.email, u.encrypted_mfa_secret, u.is_enabled, u.last_login_at, u.created_at, u.last_updated_at
		FROM console.sessions s
		JOIN console.users u ON s.user_id = u.id
		WHERE s.session_token_hash = $1
	`
	var session Session
	var user User
	var encryptedMFASecret []byte

	err := controlPlaneHandler.kernel.DB().QueryRow(ctx, selectSQLStatement, tokenHash).Scan(
		&session.ID,
		&session.UserID,
		&session.IPAddress,
		&session.UserAgent,
		&session.ExpiresAt,
		&session.CreatedAt,
		&user.ID,
		&user.Email,
		&encryptedMFASecret,
		&user.IsEnabled,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.LastUpdatedAt,
	)
	if err == nil {
		user.MFAEnabled = len(encryptedMFASecret) > 0
		if session.ExpiresAt.Before(time.Now().UTC()) {
			const deleteExpiredSQL = `DELETE FROM console.sessions WHERE id = $1`
			_, _ = controlPlaneHandler.kernel.DB().Exec(ctx, deleteExpiredSQL, session.ID)
			return nil, nil, errors.New("session expired")
		}
		if !user.IsEnabled {
			return nil, nil, errors.New("account disabled")
		}
		return &user, &session, nil
	}

	return nil, nil, errors.New("invalid console token")
}

// handleSignInConsole handles POST /v1/_/console/sign-in.
func (controlPlaneHandler *ControlPlaneHandler) handleSignInConsole(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling console sign in request")

	var signInConsoleInput SignInConsoleInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&signInConsoleInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	email := strings.ToLower(strings.TrimSpace(signInConsoleInput.Email))
	if email == "" || signInConsoleInput.Password == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Email and password are required")
		return
	}

	ctx := request.Context()
	const selectUserSQL = `
		SELECT id, email, password_hash, encrypted_mfa_secret, is_enabled, last_login_at, created_at, last_updated_at
		FROM console.users
		WHERE LOWER(email) = LOWER($1)
	`
	var user User
	var passwordHash string
	var encryptedMFASecret []byte

	err := controlPlaneHandler.kernel.DB().QueryRow(ctx, selectUserSQL, email).Scan(
		&user.ID,
		&user.Email,
		&passwordHash,
		&encryptedMFASecret,
		&user.IsEnabled,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	user.MFAEnabled = len(encryptedMFASecret) > 0

	valid, _ := controlPlaneHandler.hasher.Verify(signInConsoleInput.Password, passwordHash)
	if !valid {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if !user.IsEnabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Account is disabled")
		return
	}

	if user.MFAEnabled {
		if signInConsoleInput.MFACode == nil || strings.TrimSpace(*signInConsoleInput.MFACode) == "" {
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "MFA code required")
			return
		}
		decryptedSecret, decErr := controlPlaneHandler.kernel.CryptoKeyManager().DecryptField(string(encryptedMFASecret))
		if decErr != nil || !controlPlaneHandler.totpManager.ValidateCode(string(decryptedSecret), *signInConsoleInput.MFACode, time.Now().UTC(), 1) {
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid MFA code")
			return
		}
	}

	rawToken := controlPlaneHandler.kernel.JWTSigner().GenerateRefreshToken()
	tokenHash := controlPlaneHandler.kernel.JWTSigner().HashRefreshToken(rawToken)

	maxLifetime := time.Duration(controlPlaneHandler.configManager.Get().SessionMaxLifetimeSeconds) * time.Second
	expiresAt := time.Now().UTC().Add(maxLifetime)

	clientIP := core.ExtractRequestClientIP(request)

	const insertSessionSQL = `
		INSERT INTO console.sessions (user_id, session_token_hash, ip_address, user_agent, expires_at)
		VALUES ($1, $2, $3::inet, $4, $5)
		RETURNING id, created_at
	`
	var sessionID string
	var sessionCreatedAt time.Time
	_ = controlPlaneHandler.kernel.DB().QueryRow(ctx, insertSessionSQL, user.ID, tokenHash, clientIP, request.UserAgent(), expiresAt).Scan(
		&sessionID,
		&sessionCreatedAt,
	)

	isSecure := core.IsSecureRequest(request)
	cookieName := core.ConsoleCookieNameInsecure
	if isSecure {
		cookieName = core.ConsoleCookieNameSecure
	}
	http.SetCookie(responseWriter, &http.Cookie{
		Name:     cookieName,
		Value:    rawToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecure,
	})

	jwtClaims := core.JWTClaims{
		Subject:   user.ID,
		SessionID: sessionID,
		Email:     user.Email,
		Role:      "console_operator",
		Audience:  "layr:console:user",
		Scope:     fmt.Sprintf("%s %s %s %s", core.ScopeConsoleUserRead, core.ScopeConsoleUserWrite, core.ScopeConsoleConfigRead, core.ScopeConsoleConfigWrite),
	}
	accessToken := controlPlaneHandler.kernel.JWTSigner().GenerateAccessToken(jwtClaims, controlPlaneHandler.configManager.Get().SessionIdleTimeoutSeconds)

	now := time.Now().UTC()
	user.LastLoginAt = &now
	const updateLastLoginSQL = `UPDATE console.users SET last_login_at = clock_timestamp(), last_updated_at = clock_timestamp() WHERE id = $1`
	_, _ = controlPlaneHandler.kernel.DB().Exec(ctx, updateLastLoginSQL, user.ID)

	controlPlaneHandler.kernel.EventBus().Publish(ctx, NewSessionCreatedEvent(sessionID, SessionCreatedEventData{
		ID:        sessionID,
		User:      user,
		IPAddress: clientIP,
		UserAgent: request.UserAgent(),
		ExpiresAt: expiresAt,
		CreatedAt: sessionCreatedAt,
	}))

	signInConsoleResponse := SignInConsoleResponse{
		User:        user,
		AccessToken: accessToken,
		ExpiresIn:   controlPlaneHandler.configManager.Get().SessionIdleTimeoutSeconds,
		TokenType:   "Bearer",
	}
	core.WriteJSONResponse(responseWriter, http.StatusOK, signInConsoleResponse)
}

// handleSignOutConsole handles POST /v1/_/console/sign-out.
func (controlPlaneHandler *ControlPlaneHandler) handleSignOutConsole(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling console sign out request")

	token := controlPlaneHandler.extractConsoleToken(request)
	if token != "" {
		tokenHash := controlPlaneHandler.kernel.JWTSigner().HashRefreshToken(token)

		var sessionID string
		var userID string
		const selectSessionSQL = `SELECT id, user_id FROM console.sessions WHERE session_token_hash = $1`
		_ = controlPlaneHandler.kernel.DB().QueryRow(request.Context(), selectSessionSQL, tokenHash).Scan(&sessionID, &userID)

		const deleteSessionSQL = `DELETE FROM console.sessions WHERE session_token_hash = $1`
		_, _ = controlPlaneHandler.kernel.DB().Exec(request.Context(), deleteSessionSQL, tokenHash)

		if sessionID != "" {
			controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewSessionRevokedEvent(sessionID, SessionRevokedEventData{
				SessionID: sessionID,
				UserID:    userID,
				Reason:    "sign_out",
			}))
		}
	}

	isSecure := core.IsSecureRequest(request)
	for _, name := range []string{core.ConsoleCookieNameInsecure, core.ConsoleCookieNameSecure} {
		http.SetCookie(responseWriter, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   name == core.ConsoleCookieNameSecure || isSecure,
		})
	}
	core.ClearSessionCookie(responseWriter, request)
	responseWriter.WriteHeader(http.StatusNoContent)
}

// handleGetConsoleSession handles GET /v1/_/console/session.
func (controlPlaneHandler *ControlPlaneHandler) handleGetConsoleSession(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get console session request")

	user, session, err := controlPlaneHandler.authenticateSession(request)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}

	scopes := []string{
		core.ScopeConsoleUserRead,
		core.ScopeConsoleUserWrite,
		core.ScopeConsoleConfigRead,
		core.ScopeConsoleConfigWrite,
	}

	getConsoleSessionResponse := GetConsoleSessionResponse{
		User:    *user,
		Session: *session,
		Scopes:  scopes,
	}
	core.WriteJSONResponse(responseWriter, http.StatusOK, getConsoleSessionResponse)
}

// handleSetupConsoleMFA handles POST /v1/_/console/mfa/setup.
func (controlPlaneHandler *ControlPlaneHandler) handleSetupConsoleMFA(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling setup console MFA request")

	user, _, err := controlPlaneHandler.authenticateSession(request)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}

	secret, _ := controlPlaneHandler.totpManager.GenerateSecret()
	_ = controlPlaneHandler.kernel.KVStore().Set(request.Context(), "console:mfa_setup:"+user.ID, secret, 10*time.Minute)

	uri := controlPlaneHandler.totpManager.BuildAuthURL(user.Email, secret)
	core.WriteJSONResponse(responseWriter, http.StatusOK, SetupConsoleMFAResponse{
		Secret: secret,
		URI:    uri,
	})
}

// handleVerifyConsoleMFA handles POST /v1/_/console/mfa/verify.
func (controlPlaneHandler *ControlPlaneHandler) handleVerifyConsoleMFA(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling verify console MFA request")

	user, _, err := controlPlaneHandler.authenticateSession(request)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}

	var verifyConsoleMFAInput VerifyConsoleMFAInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&verifyConsoleMFAInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	secret, _ := controlPlaneHandler.kernel.KVStore().Get(request.Context(), "console:mfa_setup:"+user.ID)
	if secret == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "MFA setup not initiated")
		return
	}

	if !controlPlaneHandler.totpManager.ValidateCode(secret, verifyConsoleMFAInput.Code, time.Now().UTC(), 1) {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid MFA code")
		return
	}

	encryptedSecret, _ := controlPlaneHandler.kernel.CryptoKeyManager().EncryptField([]byte(secret))
	const updateSecretSQL = `UPDATE console.users SET encrypted_mfa_secret = $1, last_updated_at = clock_timestamp() WHERE id = $2`
	_, _ = controlPlaneHandler.kernel.DB().Exec(request.Context(), updateSecretSQL, []byte(encryptedSecret), user.ID)

	_ = controlPlaneHandler.kernel.KVStore().Delete(request.Context(), "console:mfa_setup:"+user.ID)

	core.WriteJSONResponse(responseWriter, http.StatusOK, core.Empty{})
}

// handleDisableConsoleMFA handles DELETE /v1/_/console/mfa.
func (controlPlaneHandler *ControlPlaneHandler) handleDisableConsoleMFA(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling disable console MFA request")

	user, _, err := controlPlaneHandler.authenticateSession(request)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}

	const disableSQL = `UPDATE console.users SET encrypted_mfa_secret = NULL, last_updated_at = clock_timestamp() WHERE id = $1`
	_, _ = controlPlaneHandler.kernel.DB().Exec(request.Context(), disableSQL, user.ID)

	responseWriter.WriteHeader(http.StatusNoContent)
}
