package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/auth/passkey"
	"layr.sh/auth/password"
	"layr.sh/core"
)

const (
	defaultOIDCAuthCodeCacheTTL = 5 * time.Minute
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
	jwtSigner             *core.JWTSigner
	hasher                *password.Hasher
	passkeyManager        *passkey.Manager
	totpManager           *core.TOTPManager
	emailDispatcher       *EmailDispatcher
	smsDispatcher         *SMSDispatcher
	kvStore               *core.KVStore
	serviceAccountManager *core.ServiceAccountManager
	eventBus              *core.EventBus
	httpClient            HTTPClient
	dummyPasswordHash     string
}

// NewBaseHandler creates a new Auth HTTP BaseHandler.
func NewBaseHandler(db *core.DatabasePool, configManager *ConfigManager, cryptoKeyManager *core.CryptoKeyManager) *BaseHandler {
	log.Debug("initializing auth base handler")
	jwtSigner, _ := core.NewJWTSigner(cryptoKeyManager)
	config := configManager.Get()

	emailDispatcher := NewEmailDispatcher(db, func() *EmailDispatcherConfig {
		emailDispatcherConfig := configManager.Get().EmailDispatcher
		return &emailDispatcherConfig
	}, cryptoKeyManager)

	smsDispatcher := NewSMSDispatcher(db, func() *SMSDispatcherConfig {
		smsDispatcherConfig := configManager.Get().SMSDispatcher
		return &smsDispatcherConfig
	}, cryptoKeyManager)

	hasher := password.NewHasher()
	dummyHash, _ := hasher.Hash("antigravity_timing_dummy_password")

	return &BaseHandler{
		db:                db,
		configManager:     configManager,
		cryptoKeyManager:  cryptoKeyManager,
		jwtSigner:         jwtSigner,
		hasher:            hasher,
		passkeyManager:    passkey.NewManager(config.Passkeys.RelyingPartyID, config.Passkeys.RelyingPartyName),
		totpManager:       core.NewTOTPManager(config.MFA.Issuer),
		emailDispatcher:   emailDispatcher,
		smsDispatcher:     smsDispatcher,
		httpClient:        &http.Client{Timeout: 5 * time.Second},
		dummyPasswordHash: dummyHash,
	}
}

func (handler *BaseHandler) verifyDummyPassword(plainPassword string) {
	_, _ = handler.hasher.Verify(plainPassword, handler.dummyPasswordHash)
}

// Handler is an alias for BaseHandler.
type Handler = BaseHandler

// NewHandler creates a new Auth HTTP BaseHandler.
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
func (handler *BaseHandler) SetKVStore(kvStore *core.KVStore) {
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

// SetHTTPClient configures the outbound HTTP client for federated sign-out notifications.
func (handler *BaseHandler) SetHTTPClient(httpClient HTTPClient) {
	log.Debug("configuring HTTP client on auth handler")
	handler.httpClient = httpClient
}

// GetTOTPManager returns the active TOTP manager.
func (handler *BaseHandler) GetTOTPManager() *core.TOTPManager {
	return handler.totpManager
}

func (handler *BaseHandler) assertEmailDeliveryReady(responseWriter http.ResponseWriter, request *http.Request) bool {
	if handler.emailDispatcher == nil || !handler.emailDispatcher.IsConfigured() {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "email delivery check failed: email dispatcher is not configured")
		return false
	}
	log.Trace("email delivery readiness asserted")
	return true
}

func (handler *BaseHandler) assertSMSDeliveryReady(responseWriter http.ResponseWriter, request *http.Request) bool {
	if handler.smsDispatcher == nil || !handler.smsDispatcher.IsConfigured() {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "sms delivery check failed: sms dispatcher is not configured")
		return false
	}
	log.Trace("sms delivery readiness asserted")
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

	refreshToken := handler.jwtSigner.GenerateRefreshToken()
	refreshHash := handler.jwtSigner.HashRefreshToken(refreshToken)

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

	customClaims := handler.resolveCustomClaims(request.Context(), userRecord.ID)
	userJWTClaims := core.JWTClaims{
		Subject:     userRecord.ID,
		SessionID:   sessionID,
		Email:       email,
		Phone:       phone,
		Role:        userRecord.Role,
		IsAnonymous: userRecord.IsAnonymous,
		Claims:      customClaims,
	}
	accessToken, _ := handler.jwtSigner.GenerateAccessToken(userJWTClaims, config.Sessions.AccessTokenExpirySeconds)

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

	core.SetSessionCookie(responseWriter, request, refreshToken, refreshTokenExpiredAt)

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

// resolveAnonymousCaller checks if the caller provided an active anonymous session
// strictly defined as email IS NULL AND phone IS NULL AND is_anonymous = true.
// Returns ErrAnonymousSessionNotFound if the caller is unauthenticated or not an anonymous user.
func (handler *BaseHandler) resolveAnonymousCaller(request *http.Request) (*UserRecord, error) {
	log.Trace("resolving anonymous caller from request")
	if handler == nil || handler.db == nil {
		log.Debug("anonymous caller resolution rejected: database pool unavailable")
		return nil, ErrAnonymousSessionNotFound
	}

	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		log.Debug("anonymous caller resolution rejected: unauthenticated caller")
		return nil, ErrAnonymousSessionNotFound
	}
	userID := authContext.UserID

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	err := handler.db.QueryRow(ctx, `
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
