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
	*Service
}

// NewBaseHandler creates a new Auth HTTP BaseHandler extending Service.
func NewBaseHandler(service *Service) *BaseHandler {
	return &BaseHandler{
		Service: service,
	}
}

func (handler *BaseHandler) verifyDummyPassword(plainPassword string) {
	_, _ = handler.hasher.Verify(plainPassword, handler.dummyPasswordHash)
}

func (handler *BaseHandler) assertEmailDeliveryReady(responseWriter http.ResponseWriter, request *http.Request) bool {
	if !handler.emailDispatcher.IsConfigured() {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "email delivery check failed: email dispatcher is not configured")
		return false
	}
	log.Trace("email delivery readiness asserted")
	return true
}

func (handler *BaseHandler) assertSMSDeliveryReady(responseWriter http.ResponseWriter, request *http.Request) bool {
	if !handler.smsDispatcher.IsConfigured() {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "sms delivery check failed: sms dispatcher is not configured")
		return false
	}
	log.Trace("sms delivery readiness asserted")
	return true
}

func (handler *BaseHandler) issueSessionResponse(responseWriter http.ResponseWriter, request *http.Request, user User, sessionMeta ...string) {
	log.Debugf("issuing session response for user %s (role: %s, is_anonymous: %t)", user.ID, user.Role, user.IsAnonymous)
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
	if user.Email != nil {
		email = *user.Email
	}
	phone := ""
	if user.Phone != nil {
		phone = *user.Phone
	}

	jwtSigner := handler.kernel.JWTSigner()
	refreshToken := jwtSigner.GenerateRefreshToken()
	refreshHash := jwtSigner.HashRefreshToken(refreshToken)

	refreshTokenExpirySeconds := config.Sessions.RefreshTokenExpirySeconds
	if refreshTokenExpirySeconds <= 0 {
		refreshTokenExpirySeconds = defaultRefreshTokenExpirySeconds
	}
	refreshTokenExpiredAt := time.Now().UTC().Add(time.Duration(refreshTokenExpirySeconds) * time.Second)

	clientIP := core.ExtractRequestClientIP(request)
	userAgent := nilIfEmpty(request.UserAgent())

	var sessionID string
	var sessionCreatedAt time.Time
	log.Tracef("persisting session record in database for user %s", user.ID)
	queryErr := handler.kernel.DB().QueryRow(request.Context(), `
		INSERT INTO auth.sessions (user_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, clock_timestamp())
		RETURNING id, created_at
	`, user.ID, refreshHash, clientIP, userAgent, refreshTokenExpiredAt).Scan(&sessionID, &sessionCreatedAt)
	if queryErr != nil {
		log.Debugf("failed to persist database session for user %s: %v", user.ID, queryErr)
		sessionID = uuid.NewV7().String()
		sessionCreatedAt = time.Now().UTC()
	}

	customClaims := handler.resolveCustomClaims(request.Context(), user.ID)
	userJWTClaims := core.JWTClaims{
		Subject:     user.ID,
		SessionID:   sessionID,
		Email:       email,
		Phone:       phone,
		Role:        user.Role,
		IsAnonymous: user.IsAnonymous,
		Claims:      customClaims,
	}
	accessToken := jwtSigner.GenerateAccessToken(userJWTClaims, config.Sessions.AccessTokenExpirySeconds)

	if config.Cache.FastPathSessionsEnabled {
		sessionKey := "auth:session:" + refreshHash
		cachedSession := CachedSession{
			User:   user,
			Claims: customClaims,
		}
		sessionData, _ := json.Marshal(cachedSession)
		sessionTTLSeconds := config.Cache.SessionTTLSeconds
		if sessionTTLSeconds <= 0 {
			sessionTTLSeconds = defaultSessionTTLSeconds
		}
		log.Tracef("caching fast-path session in KV store: %s (ttl: %ds)", sessionKey, sessionTTLSeconds)
		if kvErr := handler.kernel.KVStore().Set(request.Context(), sessionKey, string(sessionData), time.Duration(sessionTTLSeconds)*time.Second); kvErr != nil {
			log.Debugf("failed to cache fast-path session in KV store for user %s: %v", user.ID, kvErr)
		}
	}

	var ipAddressPtr *string
	if clientIP != "" {
		ipAddressPtr = &clientIP
	}
	publishCtx := request.Context()
	if parsedUserUUID, parseErr := uuid.Parse(user.ID); parseErr == nil {
		role := user.Role
		publishCtx = core.WithEventActor(publishCtx, core.EventActor{
			Type: "user",
			ID:   &parsedUserUUID,
			Role: &role,
		})
	}
	handler.kernel.EventBus().Publish(publishCtx, NewSessionCreatedEvent(sessionID, SessionCreatedEventData{
		ID:         sessionID,
		User:       user,
		AuthMethod: authMethod,
		Provider:   provider,
		IPAddress:  ipAddressPtr,
		UserAgent:  userAgent,
		ExpiresAt:  refreshTokenExpiredAt,
		CreatedAt:  sessionCreatedAt,
	}))

	if authMethod == "session_refresh" {
		handler.kernel.EventBus().Publish(publishCtx, NewTokenRefreshedEvent(sessionID, TokenRefreshedEventData{
			SessionID: sessionID,
			User:      user,
			IPAddress: ipAddressPtr,
			UserAgent: userAgent,
			ExpiresAt: refreshTokenExpiredAt,
		}))
	}

	core.SetSessionCookie(responseWriter, request, refreshToken, refreshTokenExpiredAt)

	authTokenResponse := AuthTokenResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    config.Sessions.AccessTokenExpirySeconds,
		TokenType:    "Bearer",
		Claims:       customClaims,
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, authTokenResponse)
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
	log.Tracef("caching OIDC authorization code in KV store: %s", code)
	_ = handler.kernel.KVStore().Set(ctx, "auth:code:"+code, string(payloadJSON), defaultOIDCAuthCodeCacheTTL)
	return code
}

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (handler *BaseHandler) resolveCustomClaims(ctx context.Context, userID string) map[string]any {
	if userID == "" {
		return nil
	}

	log.Tracef("resolving custom claims for user %s", userID)
	var registeredProcedure *string
	err := handler.kernel.DB().QueryRow(ctx, "SELECT to_regprocedure('public.auth_claims(uuid)')::text").Scan(&registeredProcedure)
	if err != nil || registeredProcedure == nil || *registeredProcedure == "" {
		log.Tracef("custom claims procedure public.auth_claims not found for user %s: %v", userID, err)
		return nil
	}

	var rawClaimsJSON []byte
	err = handler.kernel.DB().QueryRow(ctx, "SELECT public.auth_claims($1::uuid)", userID).Scan(&rawClaimsJSON)
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
func (handler *BaseHandler) resolveAnonymousCaller(request *http.Request) (*User, error) {
	log.Trace("resolving anonymous caller from request")

	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		log.Debug("anonymous caller resolution rejected: unauthenticated caller")
		return nil, ErrAnonymousSessionNotFound
	}
	userID := authContext.UserID

	ctx := request.Context()
	var user User
	var rawProperties []byte
	err := handler.kernel.DB().QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Debugf("caller %s not found in users table", userID)
			return nil, ErrAnonymousSessionNotFound
		}
		log.Debugf("failed to query user record for anonymous check (userID: %s): %v", userID, err)
		return nil, err
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	if user.Email == nil && user.Phone == nil && user.IsAnonymous {
		log.Debugf("resolved active anonymous caller: %s", user.ID)
		return &user, nil
	}

	log.Debugf("caller %s is not an anonymous user", user.ID)
	return nil, ErrAnonymousSessionNotFound
}
