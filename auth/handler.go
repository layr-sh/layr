package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/auth/jwt"
	"layr.sh/auth/passkey"
	"layr.sh/auth/password"
	"layr.sh/core"
)

const (
	// AuthSessionCookieName is the standard isolated __Host cookie for application users over HTTPS.
	AuthSessionCookieName = "__Host-session"
	// AuthSessionInsecureCookieName is the fallback cookie used over non-HTTPS/plain HTTP connections.
	AuthSessionInsecureCookieName = "session"
	defaultOIDCAuthCodeCacheTTL   = 5 * time.Minute
)

var (
	// ErrAnonymousSessionNotFound is returned when the caller does not have an active anonymous session.
	ErrAnonymousSessionNotFound = errors.New("anonymous session not found")
)

// AnonymousSignInRequest represents optional parameters when initializing an anonymous session.
type AnonymousSignInRequest struct {
	Properties map[string]any `json:"properties,omitempty"`
}

// UserRecord represents a user in layr_auth.users.
type UserRecord struct {
	ID              string         `json:"id"`
	Email           *string        `json:"email"`
	Phone           *string        `json:"phone"`
	PasswordHash    *string        `json:"-"`
	Role            string         `json:"role"`
	IsAnonymous     bool           `json:"is_anonymous"`
	EmailVerifiedAt *time.Time     `json:"email_verified_at,omitempty"`
	PhoneVerifiedAt *time.Time     `json:"phone_verified_at,omitempty"`
	LockedUntil     *time.Time     `json:"locked_until,omitempty"`
	Properties      map[string]any `json:"properties"`
	CreatedAt       time.Time      `json:"created_at"`
	LastUpdatedAt   time.Time      `json:"last_updated_at"`
}

// SessionRecord represents an active refresh session in layr_auth.sessions.
type SessionRecord struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	RefreshTokenHash string    `json:"refresh_token_hash"`
	IPAddress        *string   `json:"ip_address,omitempty"`
	UserAgent        *string   `json:"user_agent,omitempty"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
}

// CachedSession represents the cached user session data in kvstore for fast-path validation.
type CachedSession struct {
	User   UserRecord     `json:"user"`
	Claims map[string]any `json:"claims,omitempty"`
}

// SessionResponse represents the successful authentication token payload.
type SessionResponse struct {
	User         UserRecord     `json:"user"`
	AccessToken  string         `json:"access_token"`
	RefreshToken string         `json:"refresh_token"`
	ExpiresIn    int            `json:"expires_in"`
	TokenType    string         `json:"token_type"`
	Claims       map[string]any `json:"claims,omitempty"`
}

// OAuthStatePayload enhances federated social sign-in state with return context and OIDC flow linkage.
type OAuthStatePayload struct {
	StateID       string `json:"state_id"`
	Provider      string `json:"provider"`
	RedirectURI   string `json:"redirect_uri"`
	OIDCStateID   string `json:"oidc_state_id,omitempty"`
	AnonymousID   string `json:"anonymous_id,omitempty"`
	CreatedAtUnix int64  `json:"created_at_unix"`
}

// OIDCAuthorizationStatePayload tracks and cryptographically verifies the authorization session.
type OIDCAuthorizationStatePayload struct {
	StateID             string    `json:"state_id"`
	ClientID            string    `json:"client_id"`
	RedirectURI         string    `json:"redirect_uri"`
	Scope               string    `json:"scope"`
	ClientState         string    `json:"client_state"`
	CodeChallenge       string    `json:"code_challenge"`
	CodeChallengeMethod string    `json:"code_challenge_method"`
	Nonce               string    `json:"nonce,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

// OIDCAuthorizationCodePayload stores the issued authorization code data in kvStore (5-min TTL).
type OIDCAuthorizationCodePayload struct {
	Code                string `json:"code"`
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	UserID              string `json:"user_id"`
	Scope               string `json:"scope"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	Nonce               string `json:"nonce,omitempty"`
}

// Handler handles all authentication HTTP routes and state transitions.
type Handler struct {
	db                    *core.DatabasePool
	configManager         *ConfigManager
	cryptoKeyManager      *core.CryptoKeyManager
	signer                *jwt.Signer
	hasher                *password.Hasher
	passkeyManager        *passkey.Manager
	totpManager           *core.TOTPManager
	emailDispatcher       *EmailDispatcher
	smsDispatcher         *SMSDispatcher
	kvStore               core.KVStore
	serviceAccountManager *core.ServiceAccountManager
	webhookEventBus       *core.WebhookEventBus
}

// NewHandler creates a new Auth HTTP Handler.
func NewHandler(db *core.DatabasePool, configManager *ConfigManager, cryptoKeyManager *core.CryptoKeyManager) *Handler {
	log.Debug("initializing auth handler")
	signer, _ := jwt.NewSigner(cryptoKeyManager)
	config := configManager.Get()
	issuer := config.MFA.Issuer
	if issuer == "" {
		issuer = "Layr"
	}

	emailDispatcher := NewEmailDispatcher(db, func() *EmailDispatcherConfig {
		emailDispatcherConfig := configManager.Get().EmailDispatcher
		return &emailDispatcherConfig
	}, cryptoKeyManager)

	smsDispatcher := NewSMSDispatcher(db, func() *SMSDispatcherConfig {
		smsDispatcherConfig := configManager.Get().SMSDispatcher
		return &smsDispatcherConfig
	}, cryptoKeyManager)

	return &Handler{
		db:               db,
		configManager:    configManager,
		cryptoKeyManager: cryptoKeyManager,
		signer:           signer,
		hasher:           password.NewHasher(),
		passkeyManager:   passkey.NewManager(config.Passkeys.RelyingPartyID, config.Passkeys.RelyingPartyName),
		totpManager:      core.NewTOTPManager(issuer),
		emailDispatcher:  emailDispatcher,
		smsDispatcher:    smsDispatcher,
	}
}

// SetEmailDispatcher sets the email dispatcher for the handler.
func (handler *Handler) SetEmailDispatcher(emailDispatcher *EmailDispatcher) {
	log.Debug("configuring custom email dispatcher on auth handler")
	handler.emailDispatcher = emailDispatcher
}

// SetSMSDispatcher sets the SMS dispatcher for the handler.
func (handler *Handler) SetSMSDispatcher(smsDispatcher *SMSDispatcher) {
	log.Debug("configuring custom SMS dispatcher on auth handler")
	handler.smsDispatcher = smsDispatcher
}

// SetPasskeyManager sets the passkey manager on the handler.
func (handler *Handler) SetPasskeyManager(passkeyManager *passkey.Manager) {
	log.Debug("configuring custom passkey manager on auth handler")
	handler.passkeyManager = passkeyManager
}

// SetKVStore configures the pluggable KVStore for distributed rate-limiting and caching.
func (handler *Handler) SetKVStore(kvStore core.KVStore) {
	log.Debug("configuring KV store on auth handler")
	handler.kvStore = kvStore
}

// SetServiceAccountManager sets the service account manager for the handler.
func (handler *Handler) SetServiceAccountManager(serviceAccountManager *core.ServiceAccountManager) {
	log.Debug("configuring service account manager on auth handler")
	handler.serviceAccountManager = serviceAccountManager
}

// SetWebhookEventBus sets the platform event bus for broadcasting events.
func (handler *Handler) SetWebhookEventBus(webhookEventBus *core.WebhookEventBus) {
	log.Debug("configuring webhook event bus on auth handler")
	handler.webhookEventBus = webhookEventBus
}

// GetTOTPManager returns the active TOTP manager.
func (handler *Handler) GetTOTPManager() *core.TOTPManager {
	return handler.totpManager
}

func (handler *Handler) assertEmailDeliveryReady(responseWriter http.ResponseWriter, request *http.Request) bool {
	if handler.emailDispatcher == nil || !handler.emailDispatcher.IsConfigured() {
		log.Debugf("email delivery check failed: email dispatcher is not configured (remote: %s)", request.RemoteAddr)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnprocessableEntity, "Email delivery is currently unavailable", "LAYR_AUTH_EMAIL_UNAVAILABLE")
		return false
	}
	log.Trace("email delivery readiness asserted")
	return true
}

func (handler *Handler) assertSMSDeliveryReady(responseWriter http.ResponseWriter, request *http.Request) bool {
	if handler.smsDispatcher == nil || !handler.smsDispatcher.IsConfigured() {
		log.Debugf("SMS delivery check failed: SMS dispatcher is not configured (remote: %s)", request.RemoteAddr)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnprocessableEntity, "SMS delivery is currently unavailable", "LAYR_AUTH_SMS_UNAVAILABLE")
		return false
	}
	log.Trace("SMS delivery readiness asserted")
	return true
}

func (handler *Handler) issueSessionResponse(responseWriter http.ResponseWriter, request *http.Request, userRecord UserRecord) {
	log.Debugf("issuing session response for user %s (role: %s, is_anonymous: %t)", userRecord.ID, userRecord.Role, userRecord.IsAnonymous)
	config := handler.configManager.Get()

	email := ""
	if userRecord.Email != nil {
		email = *userRecord.Email
	}
	phone := ""
	if userRecord.Phone != nil {
		phone = *userRecord.Phone
	}

	customClaims := handler.resolveCustomClaims(request.Context(), userRecord.ID)
	userClaims := jwt.Claims{
		Subject:     userRecord.ID,
		Email:       email,
		Phone:       phone,
		Role:        userRecord.Role,
		IsAnonymous: userRecord.IsAnonymous,
		Claims:      customClaims,
	}
	accessToken, _ := handler.signer.GenerateAccessToken(userClaims, config.Sessions.AccessTokenExpirySeconds)
	refreshToken := jwt.GenerateRefreshToken()
	refreshHash := jwt.HashRefreshToken(refreshToken)

	refreshTokenExpirySeconds := config.Sessions.RefreshTokenExpirySeconds
	if refreshTokenExpirySeconds <= 0 {
		refreshTokenExpirySeconds = defaultRefreshTokenExpirySeconds
	}
	refreshTokenExpiredAt := time.Now().UTC().Add(time.Duration(refreshTokenExpirySeconds) * time.Second)

	clientIP := core.ExtractRequestClientIP(request)
	userAgent := nilIfEmpty(request.UserAgent())

	if handler.db != nil {
		log.Tracef("persisting session record in database for user %s", userRecord.ID)
		_, execErr := handler.db.Exec(request.Context(), `
			INSERT INTO layr_auth.sessions (user_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at)
			VALUES ($1, $2, $3, $4, $5, clock_timestamp())
		`, userRecord.ID, refreshHash, clientIP, userAgent, refreshTokenExpiredAt)
		if execErr != nil {
			log.Debugf("failed to persist database session for user %s: %v", userRecord.ID, execErr)
		}
	}

	if config.Cache.FastPathSessionsEnabled && handler.kvStore != nil {
		sessionKey := "layr:auth:session:" + refreshHash
		cachedSession := CachedSession{
			User:   userRecord,
			Claims: customClaims,
		}
		sessionData, _ := json.Marshal(cachedSession)
		sessionTTLSeconds := config.Cache.SessionTTLSeconds
		if sessionTTLSeconds <= 0 {
			sessionTTLSeconds = defaultSessionTTLSeconds
		}
		log.Tracef("caching fast-path session in KV store: %s (ttl: %ds)", sessionKey, sessionTTLSeconds)
		if kvErr := handler.kvStore.Set(request.Context(), sessionKey, string(sessionData), time.Duration(sessionTTLSeconds)*time.Second); kvErr != nil {
			log.Debugf("failed to cache fast-path session in KV store for user %s: %v", userRecord.ID, kvErr)
		}
	}

	if handler.webhookEventBus != nil {
		handler.webhookEventBus.Publish(request.Context(), core.WebhookEventEnvelope{
			ID:        uuid.NewV7().String(),
			Event:     "auth.session.created",
			Timestamp: time.Now().UTC(),
			Service:   "auth",
			Resource:  "session",
			Action:    "created",
			Data: map[string]string{
				"user_id": userRecord.ID,
			},
		})
	}

	isSecure := core.IsSecureRequest(request)
	log.Tracef("setting session cookie for user %s (secure: %t, clientIP: %s)", userRecord.ID, isSecure, clientIP)
	core.SetSessionCookie(responseWriter, AuthSessionCookieName, AuthSessionInsecureCookieName, refreshToken, refreshTokenExpiredAt, isSecure)

	sessionResponse := SessionResponse{
		User:         userRecord,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    config.Sessions.AccessTokenExpirySeconds,
		TokenType:    "Bearer",
		Claims:       customClaims,
	}

	handler.writeJSON(responseWriter, http.StatusOK, sessionResponse)
}

func (handler *Handler) issueOIDCAuthorizationCode(ctx context.Context, clientID, redirectURI, userID, scope, codeChallenge, codeChallengeMethod, nonce string) string {
	log.Debugf("issuing OIDC authorization code for client %s, user %s", clientID, userID)
	code := strings.ReplaceAll(uuid.NewV7().String()+uuid.NewV7().String(), "-", "")
	oidcAuthorizationCodePayload := OIDCAuthorizationCodePayload{
		Code:                code,
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		UserID:              userID,
		Scope:               scope,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		Nonce:               nonce,
	}
	payloadJSON, _ := json.Marshal(oidcAuthorizationCodePayload)
	if handler.kvStore != nil {
		log.Tracef("caching OIDC authorization code in KV store: %s", code)
		_ = handler.kvStore.Set(ctx, "layr:auth:code:"+code, string(payloadJSON), defaultOIDCAuthCodeCacheTTL)
	}
	return code
}

func (handler *Handler) writeJSON(responseWriter http.ResponseWriter, statusCode int, payload any) {
	log.Tracef("writing JSON response with status %d", statusCode)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(payload)
}

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (handler *Handler) resolveCustomClaims(ctx context.Context, userID string) map[string]any {
	if handler.db == nil || userID == "" {
		return nil
	}

	log.Tracef("resolving custom claims for user %s", userID)
	var registeredProcedure *string
	err := handler.db.QueryRow(ctx, "SELECT to_regprocedure('public.auth_claims(uuid)')::text").Scan(&registeredProcedure)
	if err != nil || registeredProcedure == nil || *registeredProcedure == "" {
		log.Tracef("custom claims procedure public.auth_claims not found for user %s: %v", userID, err)
		return nil
	}

	var rawClaimsJSON []byte
	err = handler.db.QueryRow(ctx, "SELECT public.auth_claims($1::uuid)", userID).Scan(&rawClaimsJSON)
	if err != nil || len(rawClaimsJSON) == 0 || string(rawClaimsJSON) == "null" {
		log.Tracef("custom claims procedure returned empty or error for user %s: %v", userID, err)
		return nil
	}

	var customClaims map[string]any
	if err := json.Unmarshal(rawClaimsJSON, &customClaims); err != nil {
		log.Debugf("failed to parse custom claims JSON for user %s: %v", userID, err)
		return nil
	}
	log.Tracef("resolved %d custom claims for user %s", len(customClaims), userID)
	return customClaims
}

func (handler *Handler) authenticateSessionRequest(request *http.Request) (string, string, string, error) {
	log.Trace("authenticating session request")
	if handler == nil || handler.signer == nil {
		log.Debug("session authentication rejected: auth signer unavailable")
		return "", "", "", fmt.Errorf("auth signer unavailable")
	}

	token := core.ExtractRequestSessionToken(request, AuthSessionCookieName, AuthSessionInsecureCookieName)
	if token == "" {
		log.Debug("session authentication rejected: missing authentication token in headers and cookies")
		return "", "", "", fmt.Errorf("missing authentication token")
	}

	claims, err := handler.signer.VerifyAccessToken(token)
	if err != nil || claims == nil || claims.Subject == "" {
		log.Debugf("session authentication rejected: access token validation failed: %v", err)
		return "", "", "", fmt.Errorf("invalid access token: %w", err)
	}

	var currentRefreshTokenHash string
	var tokenSource string
	if cookie, err := request.Cookie(AuthSessionCookieName); err == nil && cookie.Value != "" {
		currentRefreshTokenHash = jwt.HashRefreshToken(cookie.Value)
		tokenSource = "cookie:" + AuthSessionCookieName
	} else if cookie, err := request.Cookie(AuthSessionInsecureCookieName); err == nil && cookie.Value != "" {
		currentRefreshTokenHash = jwt.HashRefreshToken(cookie.Value)
		tokenSource = "cookie:" + AuthSessionInsecureCookieName
	}
	if currentRefreshTokenHash == "" {
		if refreshTokenHeader := request.Header.Get("X-Refresh-Token"); refreshTokenHeader != "" {
			currentRefreshTokenHash = jwt.HashRefreshToken(refreshTokenHeader)
			tokenSource = "header:X-Refresh-Token"
		} else if sessionTokenHeader := request.Header.Get("X-Session-Token"); sessionTokenHeader != "" {
			currentRefreshTokenHash = jwt.HashRefreshToken(sessionTokenHeader)
			tokenSource = "header:X-Session-Token"
		}
	}

	currentSessionID := request.Header.Get("X-Session-ID")
	log.Tracef("session request authenticated for subject %s (source: %s, sessionID: %s)", claims.Subject, tokenSource, currentSessionID)
	return claims.Subject, currentRefreshTokenHash, currentSessionID, nil
}

func (handler *Handler) extractClaimsOptional(request *http.Request) *jwt.Claims {
	log.Trace("extracting optional claims from request")
	authHeader := request.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if handler.signer != nil {
			claims, err := handler.signer.VerifyAccessToken(token)
			if err == nil {
				log.Tracef("successfully extracted optional claims for subject %s", claims.Subject)
				return claims
			}
			log.Debugf("optional bearer token verification failed: %v", err)
		}
	}
	return nil
}

func (handler *Handler) authenticateUser(request *http.Request) (string, error) {
	log.Trace("authenticating user from request")
	if handler == nil || handler.signer == nil {
		log.Debug("user authentication rejected: auth signer unavailable")
		return "", fmt.Errorf("auth signer unavailable")
	}

	token := core.ExtractRequestSessionToken(request, AuthSessionCookieName, AuthSessionInsecureCookieName)
	if token != "" {
		claims, err := handler.signer.VerifyAccessToken(token)
		if err == nil && claims != nil && claims.Subject != "" {
			log.Tracef("user authenticated via access token: %s", claims.Subject)
			return claims.Subject, nil
		}
		log.Debugf("access token verification failed during user authentication: %v", err)
	}

	var refreshToken string
	var tokenSource string
	if cookie, err := request.Cookie(AuthSessionCookieName); err == nil && cookie.Value != "" {
		refreshToken = cookie.Value
		tokenSource = "cookie:" + AuthSessionCookieName
	} else if cookie, err := request.Cookie(AuthSessionInsecureCookieName); err == nil && cookie.Value != "" {
		refreshToken = cookie.Value
		tokenSource = "cookie:" + AuthSessionInsecureCookieName
	}

	if refreshToken == "" {
		if refreshTokenHeader := request.Header.Get("X-Refresh-Token"); refreshTokenHeader != "" {
			refreshToken = refreshTokenHeader
			tokenSource = "header:X-Refresh-Token"
		} else if sessionTokenHeader := request.Header.Get("X-Session-Token"); sessionTokenHeader != "" {
			refreshToken = sessionTokenHeader
			tokenSource = "header:X-Session-Token"
		}
	}

	if refreshToken != "" && handler.db != nil {
		refreshTokenHash := jwt.HashRefreshToken(refreshToken)
		log.Tracef("looking up active session in database via %s", tokenSource)
		var userID string
		err := handler.db.QueryRow(request.Context(), `
			SELECT user_id FROM layr_auth.sessions
			WHERE refresh_token_hash = $1 AND expires_at > clock_timestamp()
		`, refreshTokenHash).Scan(&userID)
		if err == nil && userID != "" {
			log.Tracef("user authenticated via session refresh token: %s", userID)
			return userID, nil
		}
		log.Debugf("session lookup via refresh token failed (source: %s): %v", tokenSource, err)
	}

	log.Debug("user authentication failed: unauthorized (no valid token or active session found)")
	return "", fmt.Errorf("unauthorized")
}

// resolveAnonymousCaller checks if the caller provided an active anonymous session
// strictly defined as email IS NULL AND phone IS NULL AND is_anonymous = true.
// Returns ErrAnonymousSessionNotFound if the caller is unauthenticated or not an anonymous user.
func (handler *Handler) resolveAnonymousCaller(request *http.Request) (*UserRecord, error) {
	log.Trace("resolving anonymous caller from request")
	if handler == nil || handler.db == nil {
		log.Debug("anonymous caller resolution rejected: database pool unavailable")
		return nil, ErrAnonymousSessionNotFound
	}

	userID, err := handler.authenticateUser(request)
	if err != nil || userID == "" {
		log.Debugf("anonymous caller resolution rejected: unauthenticated caller: %v", err)
		return nil, ErrAnonymousSessionNotFound
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	err = handler.db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		FROM layr_auth.users
		WHERE id = $1
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Debugf("caller %s not found in users table", userID)
			return nil, ErrAnonymousSessionNotFound
		}
		log.Debugf("failed to query user record for anonymous check (userID: %s): %v", userID, err)
		return nil, err
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if userRecord.Email == nil && userRecord.Phone == nil && userRecord.IsAnonymous {
		log.Debugf("resolved active anonymous caller: %s", userRecord.ID)
		return &userRecord, nil
	}

	log.Debugf("caller %s is not an anonymous user", userRecord.ID)
	return nil, ErrAnonymousSessionNotFound
}
