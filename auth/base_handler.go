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

// BaseHandler handles all authentication HTTP routes and state transitions on the base router.
type BaseHandler struct {
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
	eventBus              *core.EventBus
}

// NewBaseHandler creates a new Auth HTTP BaseHandler.
func NewBaseHandler(db *core.DatabasePool, configManager *ConfigManager, cryptoKeyManager *core.CryptoKeyManager) *BaseHandler {
	log.Debug("initializing auth base handler")
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

	return &BaseHandler{
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

// Handler is an alias for BaseHandler for backwards compatibility.
type Handler = BaseHandler

// NewHandler creates a new Auth HTTP BaseHandler (alias for NewBaseHandler).
func NewHandler(db *core.DatabasePool, configManager *ConfigManager, cryptoKeyManager *core.CryptoKeyManager) *BaseHandler {
	return NewBaseHandler(db, configManager, cryptoKeyManager)
}

// SetEmailDispatcher sets the email dispatcher for the handler.
func (handler *BaseHandler) SetEmailDispatcher(emailDispatcher *EmailDispatcher) {
	log.Debug("configuring custom email dispatcher on auth handler")
	handler.emailDispatcher = emailDispatcher
}

// SetSMSDispatcher sets the SMS dispatcher for the handler.
func (handler *BaseHandler) SetSMSDispatcher(smsDispatcher *SMSDispatcher) {
	log.Debug("configuring custom SMS dispatcher on auth handler")
	handler.smsDispatcher = smsDispatcher
}

// SetPasskeyManager sets the passkey manager on the handler.
func (handler *BaseHandler) SetPasskeyManager(passkeyManager *passkey.Manager) {
	log.Debug("configuring custom passkey manager on auth handler")
	handler.passkeyManager = passkeyManager
}

// SetKVStore configures the pluggable KVStore for distributed rate-limiting and caching.
func (handler *BaseHandler) SetKVStore(kvStore core.KVStore) {
	log.Debug("configuring KV store on auth handler")
	handler.kvStore = kvStore
}

// SetServiceAccountManager sets the service account manager for the handler.
func (handler *BaseHandler) SetServiceAccountManager(serviceAccountManager *core.ServiceAccountManager) {
	log.Debug("configuring service account manager on auth handler")
	handler.serviceAccountManager = serviceAccountManager
}

// SetEventBus sets the platform event bus for broadcasting events.
func (handler *BaseHandler) SetEventBus(eventBus *core.EventBus) {
	log.Debug("configuring event bus on auth handler")
	handler.eventBus = eventBus
}

// GetTOTPManager returns the active TOTP manager.
func (handler *BaseHandler) GetTOTPManager() *core.TOTPManager {
	return handler.totpManager
}

func (handler *BaseHandler) assertEmailDeliveryReady(responseWriter http.ResponseWriter, request *http.Request) bool {
	if handler.emailDispatcher == nil || !handler.emailDispatcher.IsConfigured() {
		log.Debugf("email delivery check failed: email dispatcher is not configured (remote: %s)", request.RemoteAddr)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnprocessableEntity, "Email delivery is currently unavailable", "LAYR_AUTH_EMAIL_UNAVAILABLE")
		return false
	}
	log.Trace("email delivery readiness asserted")
	return true
}

func (handler *BaseHandler) assertSMSDeliveryReady(responseWriter http.ResponseWriter, request *http.Request) bool {
	if handler.smsDispatcher == nil || !handler.smsDispatcher.IsConfigured() {
		log.Debugf("SMS delivery check failed: SMS dispatcher is not configured (remote: %s)", request.RemoteAddr)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnprocessableEntity, "SMS delivery is currently unavailable", "LAYR_AUTH_SMS_UNAVAILABLE")
		return false
	}
	log.Trace("SMS delivery readiness asserted")
	return true
}

func (handler *BaseHandler) issueSessionResponse(responseWriter http.ResponseWriter, request *http.Request, userRecord UserRecord, sessionMeta ...string) {
	log.Debugf("issuing session response for user %s (role: %s, is_anonymous: %t)", userRecord.ID, userRecord.Role, userRecord.IsAnonymous)
	config := handler.configManager.Get()

	var authMethod string
	var provider string
	if len(sessionMeta) > 0 {
		authMethod = sessionMeta[0]
	}
	if len(sessionMeta) > 1 {
		provider = sessionMeta[1]
	}

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

	var sessionID string
	var sessionCreatedAt time.Time
	if handler.db != nil {
		log.Tracef("persisting session record in database for user %s", userRecord.ID)
		queryErr := handler.db.QueryRow(request.Context(), `
			INSERT INTO auth.sessions (user_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at)
			VALUES ($1, $2, $3, $4, $5, clock_timestamp())
			RETURNING id, created_at
		`, userRecord.ID, refreshHash, clientIP, userAgent, refreshTokenExpiredAt).Scan(&sessionID, &sessionCreatedAt)
		if queryErr != nil {
			log.Debugf("failed to persist database session for user %s: %v", userRecord.ID, queryErr)
		}
	}
	if sessionID == "" {
		sessionID = uuid.NewV7().String()
		sessionCreatedAt = time.Now().UTC()
	}

	if config.Cache.FastPathSessionsEnabled && handler.kvStore != nil {
		sessionKey := "auth:session:" + refreshHash
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

	if handler.eventBus != nil {
		var ipAddressPtr *string
		if clientIP != "" {
			ipAddressPtr = &clientIP
		}
		publishCtx := request.Context()
		if parsedUserUUID, parseErr := uuid.Parse(userRecord.ID); parseErr == nil {
			role := userRecord.Role
			publishCtx = core.WithEventActor(publishCtx, core.EventActor{
				Type: "user",
				ID:   &parsedUserUUID,
				Role: &role,
			})
		}
		handler.eventBus.Publish(publishCtx, NewSessionCreatedEvent(sessionID, SessionCreatedEventData{
			ID:         sessionID,
			User:       userRecord,
			AuthMethod: authMethod,
			Provider:   provider,
			IPAddress:  ipAddressPtr,
			UserAgent:  userAgent,
			ExpiresAt:  refreshTokenExpiredAt,
			CreatedAt:  sessionCreatedAt,
		}))
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

	handler.writeJSON(responseWriter, sessionResponse)
}

func (handler *BaseHandler) issueOIDCAuthorizationCode(ctx context.Context, clientID, redirectURI, userID, scope, codeChallenge, codeChallengeMethod, nonce string) string {
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
		_ = handler.kvStore.Set(ctx, "auth:code:"+code, string(payloadJSON), defaultOIDCAuthCodeCacheTTL)
	}
	return code
}

func (handler *BaseHandler) writeJSON(responseWriter http.ResponseWriter, payload any) {
	log.Trace("writing JSON response with status 200")
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(payload)
}

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (handler *BaseHandler) resolveCustomClaims(ctx context.Context, userID string) map[string]any {
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

func (handler *BaseHandler) authenticateSessionRequest(request *http.Request) (string, string, string, error) {
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
	if parsedUserUUID, parseErr := uuid.Parse(claims.Subject); parseErr == nil {
		role := claims.Role
		*request = *request.WithContext(core.WithEventActor(request.Context(), core.EventActor{
			Type: "user",
			ID:   &parsedUserUUID,
			Role: &role,
		}))
	}
	return claims.Subject, currentRefreshTokenHash, currentSessionID, nil
}

func (handler *BaseHandler) extractClaimsOptional(request *http.Request) *jwt.Claims {
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

func (handler *BaseHandler) authenticateUser(request *http.Request) (string, error) {
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
			if parsedUserUUID, parseErr := uuid.Parse(claims.Subject); parseErr == nil {
				role := claims.Role
				*request = *request.WithContext(core.WithEventActor(request.Context(), core.EventActor{
					Type: "user",
					ID:   &parsedUserUUID,
					Role: &role,
				}))
			}
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
			SELECT user_id FROM auth.sessions
			WHERE refresh_token_hash = $1 AND expires_at > clock_timestamp()
		`, refreshTokenHash).Scan(&userID)
		if err == nil && userID != "" {
			log.Tracef("user authenticated via session refresh token: %s", userID)
			if parsedUserUUID, parseErr := uuid.Parse(userID); parseErr == nil {
				role := "authenticated"
				*request = *request.WithContext(core.WithEventActor(request.Context(), core.EventActor{
					Type: "user",
					ID:   &parsedUserUUID,
					Role: &role,
				}))
			}
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
func (handler *BaseHandler) resolveAnonymousCaller(request *http.Request) (*UserRecord, error) {
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
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE id = $1
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
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
