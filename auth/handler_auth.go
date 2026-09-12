package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"layr.sh/auth/jwt"
	"layr.sh/core"
)

// SignUpRequest defines registration parameters with password.
type SignUpRequest struct {
	Email      string         `json:"email"`
	Phone      string         `json:"phone"`
	Password   string         `json:"password"`
	Properties map[string]any `json:"properties"`
}

// SignInRequest defines login credentials.
type SignInRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

// RefreshTokenRequest defines token refresh input.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (handler *Handler) handleAnonymousSignIn(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling anonymous sign-in request")
	config := handler.configManager.Get()
	if !config.Anonymous.Enabled {
		log.Debug("anonymous sign-in rejected: anonymous authentication disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Anonymous authentication is disabled", "LAYR_AUTH_001")
		return
	}

	var anonymousSignInRequest AnonymousSignInRequest
	if request.Body != nil {
		_ = json.NewDecoder(request.Body).Decode(&anonymousSignInRequest)
	}

	inputProperties := anonymousSignInRequest.Properties
	if inputProperties == nil {
		inputProperties = make(map[string]any)
	}
	cleanedProperties := sanitizeUserProperties(inputProperties)
	propertiesJSON, _ := json.Marshal(cleanedProperties)

	if handler.db == nil {
		log.Debug("anonymous sign-in rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	query := `
		INSERT INTO auth.users (role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ('authenticated', true, $1, clock_timestamp(), clock_timestamp())
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
	`
	err := handler.db.QueryRow(ctx, query, propertiesJSON).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("failed to create anonymous user: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to create anonymous user", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewUserSignedUpEvent(userRecord.ID, UserSignedUpEventData(userRecord)))
	}

	handler.issueSessionResponse(responseWriter, request, userRecord)
}

func (handler *Handler) handleSignUp(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling sign-up request")
	config := handler.configManager.Get()
	if !config.Password.Enabled {
		log.Debug("sign-up rejected: password registration disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Password registration is disabled", "LAYR_AUTH_001")
		return
	}

	var signUpRequest SignUpRequest
	if err := json.NewDecoder(request.Body).Decode(&signUpRequest); err != nil {
		log.Debugf("sign-up rejected: invalid JSON body: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", "LAYR_AUTH_001")
		return
	}

	signUpRequest.Email = strings.TrimSpace(strings.ToLower(signUpRequest.Email))
	signUpRequest.Phone = strings.TrimSpace(signUpRequest.Phone)
	if signUpRequest.Phone != "" {
		normalizedPhone, err := NormalizePhone(signUpRequest.Phone)
		if err != nil {
			log.Debugf("sign-up rejected: invalid phone format: %v", err)
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code", "LAYR_AUTH_INVALID_PHONE")
			return
		}
		signUpRequest.Phone = normalizedPhone
	}
	if signUpRequest.Email == "" && signUpRequest.Phone == "" {
		log.Debug("sign-up rejected: email or phone number required")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Email or phone number is required", "LAYR_AUTH_001")
		return
	}
	if len(signUpRequest.Password) < config.Password.MinLength {
		log.Debugf("sign-up rejected: password length %d below minimum %d", len(signUpRequest.Password), config.Password.MinLength)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Password must be at least %d characters", config.Password.MinLength), "LAYR_AUTH_001")
		return
	}

	passHash, _ := handler.hasher.Hash(signUpRequest.Password)

	inputProperties := signUpRequest.Properties
	if inputProperties == nil {
		inputProperties = make(map[string]any)
	}
	cleanedProperties := sanitizeUserProperties(inputProperties)
	propertiesJSON, _ := json.Marshal(cleanedProperties)

	if handler.db == nil {
		log.Debug("sign-up rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var emailPtr, phonePtr *string
	if signUpRequest.Email != "" {
		emailPtr = &signUpRequest.Email
	}
	if signUpRequest.Phone != "" {
		phonePtr = &signUpRequest.Phone
	}

	anonymousUserRecord, _ := handler.resolveAnonymousCaller(request)
	if anonymousUserRecord != nil {
		var existingUserID string
		err := handler.db.QueryRow(ctx, `
			SELECT id FROM auth.users 
			WHERE (email IS NOT NULL AND email = $1) 
			   OR (phone IS NOT NULL AND phone = $2)
			LIMIT 1
		`, emailPtr, phonePtr).Scan(&existingUserID)
		if err == nil && existingUserID != anonymousUserRecord.ID {
			log.Debugf("sign-up conversion conflict: email or phone already registered by user %s", existingUserID)
			if emailPtr != nil {
				core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Email is already in use by another account", "LAYR_AUTH_001")
			} else {
				core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Phone number is already in use by another account", "LAYR_AUTH_001")
			}
			return
		}

		var userRecord UserRecord
		var rawProperties []byte
		updateQuery := `
			UPDATE auth.users
			SET email = $1, phone = $2, password_hash = $3, is_anonymous = false,
			    properties = COALESCE(properties, '{}'::jsonb) || $4::jsonb,
			    last_updated_at = clock_timestamp()
			WHERE id = $5
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		`
		err = handler.db.QueryRow(ctx, updateQuery, emailPtr, phonePtr, passHash, propertiesJSON, anonymousUserRecord.ID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
			&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
		)
		if err != nil {
			log.Debugf("failed to convert anonymous user %s: %v", anonymousUserRecord.ID, err)
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to convert user", "LAYR_AUTH_001")
			return
		}

		userRecord.Properties = make(map[string]any)
		if len(rawProperties) > 0 {
			_ = json.Unmarshal(rawProperties, &userRecord.Properties)
		}

		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewUserConvertedEvent(userRecord.ID, UserConvertedEventData(userRecord)))
		}

		handler.issueSessionResponse(responseWriter, request, userRecord)
		return
	}

	var userRecord UserRecord
	var rawProperties []byte
	query := `
		INSERT INTO auth.users (email, phone, password_hash, role, properties, created_at, last_updated_at)
		VALUES ($1, $2, $3, 'authenticated', $4, clock_timestamp(), clock_timestamp())
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
	`
	err := handler.db.QueryRow(ctx, query, emailPtr, phonePtr, passHash, propertiesJSON).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("failed to create user: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to create user", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewUserSignedUpEvent(userRecord.ID, UserSignedUpEventData(userRecord)))
	}

	handler.issueSessionResponse(responseWriter, request, userRecord)
}

func (handler *Handler) handleSignIn(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling sign-in request")
	var signInRequest SignInRequest
	if err := json.NewDecoder(request.Body).Decode(&signInRequest); err != nil {
		log.Debugf("sign-in rejected: invalid JSON body: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", "LAYR_AUTH_001")
		return
	}

	identifier := strings.TrimSpace(strings.ToLower(signInRequest.Email))
	if identifier == "" && signInRequest.Phone != "" {
		normalizedPhone, err := NormalizePhone(signInRequest.Phone)
		if err != nil {
			log.Debugf("sign-in rejected: invalid phone format: %v", err)
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code", "LAYR_AUTH_INVALID_PHONE")
			return
		}
		identifier = normalizedPhone
	}
	if identifier == "" || signInRequest.Password == "" {
		log.Debug("sign-in rejected: missing credentials")
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	config := handler.configManager.Get()
	if config.RateLimiting.Enabled && handler.kvStore != nil && identifier != "" {
		rateKey := fmt.Sprintf("auth:ratelimit:signin:%s", identifier)
		windowDuration := time.Duration(config.RateLimiting.WindowDurationSeconds) * time.Second
		if count, err := handler.kvStore.Increment(ctx, rateKey, windowDuration); err == nil && count > int64(config.RateLimiting.MaxSigninAttempts) {
			log.Debugf("sign-in rejected: rate limit exceeded for identifier %s (count: %d)", identifier, count)
			core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Too many login attempts. Please try again later.", "LAYR_AUTH_005")
			return
		}
	}

	if handler.db == nil {
		log.Debug("sign-in rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	var userRecord UserRecord
	var passHash string
	var rawProperties []byte

	query := `
		SELECT id, email, phone, password_hash, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		FROM auth.users
		WHERE email = $1 OR phone = $1
	`
	err := handler.db.QueryRow(ctx, query, identifier).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &passHash, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("sign-in rejected: user not found for identifier %s: %v", identifier, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials", "LAYR_AUTH_001")
		return
	}

	if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
		log.Debugf("sign-in rejected: account locked until %v for user %s", *userRecord.LockedUntil, userRecord.ID)
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked", "LAYR_AUTH_005")
		return
	}

	ok, err := handler.hasher.Verify(signInRequest.Password, passHash)
	if err != nil || !ok {
		log.Debugf("sign-in rejected: password mismatch for user %s: %v", userRecord.ID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	// Reset rate limit on successful authentication
	if handler.kvStore != nil && identifier != "" {
		rateKey := fmt.Sprintf("auth:ratelimit:signin:%s", identifier)
		_ = handler.kvStore.Delete(ctx, rateKey)
	}

	handler.issueSessionResponse(responseWriter, request, userRecord)
}

func (handler *Handler) handleTokenRefresh(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling token refresh request")
	var refreshTokenRequest RefreshTokenRequest
	if request.Body != nil && request.ContentLength != 0 {
		if err := json.NewDecoder(request.Body).Decode(&refreshTokenRequest); err != nil {
			log.Debugf("token refresh rejected: invalid JSON body: %v", err)
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", "LAYR_AUTH_001")
			return
		}
	}
	if refreshTokenRequest.RefreshToken == "" {
		refreshTokenRequest.RefreshToken = core.ExtractRequestSessionToken(request, AuthSessionCookieName, AuthSessionInsecureCookieName)
	}
	if refreshTokenRequest.RefreshToken == "" {
		log.Debug("token refresh rejected: missing refresh token")
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Refresh token required", "LAYR_AUTH_002")
		return
	}

	tokenHash := jwt.HashRefreshToken(refreshTokenRequest.RefreshToken)
	ctx := request.Context()

	config := handler.configManager.Get()
	if config.Cache.FastPathSessionsEnabled && handler.kvStore != nil {
		if cachedData, err := handler.kvStore.Get(ctx, "auth:session:"+tokenHash); err == nil && cachedData != "" {
			var cachedSession CachedSession
			if err := json.Unmarshal([]byte(cachedData), &cachedSession); err == nil && cachedSession.User.ID != "" {
				log.Tracef("fast-path session cache hit for user %s", cachedSession.User.ID)
				_ = handler.kvStore.Delete(ctx, "auth:session:"+tokenHash)
				if handler.db != nil {
					_, _ = handler.db.Exec(ctx, "DELETE FROM auth.sessions WHERE refresh_token_hash = $1", tokenHash)
				}
				handler.issueSessionResponse(responseWriter, request, cachedSession.User)
				return
			}
		}
	}

	if handler.db == nil {
		log.Debug("token refresh rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	var sessionID, userID string
	var expiresAt time.Time

	err := handler.db.QueryRow(ctx, `
		SELECT id, user_id, expires_at 
		FROM auth.sessions 
		WHERE refresh_token_hash = $1
	`, tokenHash).Scan(&sessionID, &userID, &expiresAt)

	if err != nil {
		log.Debugf("token refresh rejected: session not found for token hash %s: %v", tokenHash, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Refresh token revoked or invalid", "LAYR_AUTH_003")
		return
	}

	if time.Now().UTC().After(expiresAt) {
		log.Debugf("token refresh rejected: session %s expired at %v", sessionID, expiresAt)
		_, _ = handler.db.Exec(ctx, "DELETE FROM auth.sessions WHERE id = $1", sessionID)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Refresh token expired", "LAYR_AUTH_003")
		return
	}

	var userRecord UserRecord
	var rawProperties []byte
	err = handler.db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		FROM auth.users WHERE id = $1
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("token refresh rejected: user %s not found: %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "User not found", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	// Rotate refresh token
	_, _ = handler.db.Exec(ctx, "DELETE FROM auth.sessions WHERE id = $1", sessionID)
	handler.issueSessionResponse(responseWriter, request, userRecord)
}

func (handler *Handler) handleSignOut(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling sign-out request")
	var refreshTokenRequest RefreshTokenRequest
	if request.Body != nil && request.ContentLength != 0 {
		_ = json.NewDecoder(request.Body).Decode(&refreshTokenRequest)
	}
	if refreshTokenRequest.RefreshToken == "" {
		refreshTokenRequest.RefreshToken = core.ExtractRequestSessionToken(request, AuthSessionCookieName, AuthSessionInsecureCookieName)
	}

	if refreshTokenRequest.RefreshToken != "" {
		tokenHash := jwt.HashRefreshToken(refreshTokenRequest.RefreshToken)
		if handler.kvStore != nil {
			_ = handler.kvStore.Delete(request.Context(), "auth:session:"+tokenHash)
		}
		if handler.db != nil {
			var sessionID, userID string
			err := handler.db.QueryRow(request.Context(), `
				DELETE FROM auth.sessions 
				WHERE refresh_token_hash = $1
				RETURNING id, user_id
			`, tokenHash).Scan(&sessionID, &userID)
			if err == nil && handler.eventBus != nil {
				handler.eventBus.Publish(request.Context(), NewSessionDeletedEvent(sessionID, SessionDeletedEventData{
					SessionID: &sessionID,
					UserID:    userID,
				}))
			}
		}
	}

	isSecure := core.IsSecureRequest(request)
	core.ClearSessionCookie(responseWriter, AuthSessionCookieName, AuthSessionInsecureCookieName, isSecure)

	handler.writeJSON(responseWriter, map[string]bool{"ok": true})
}
