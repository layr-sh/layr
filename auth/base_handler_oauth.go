package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/auth/jwt"
	"layr.sh/auth/oauth"
	"layr.sh/core"
)

// HandleOAuthAuthorize initiates the authorization redirection for an OAuth provider.
func (handler *BaseHandler) HandleOAuthAuthorize(responseWriter http.ResponseWriter, request *http.Request) {
	provider := request.PathValue("provider")
	log.Debugf("handling OAuth authorize request for provider: %s", provider)

	config := handler.configManager.Get()
	oAuthProviderConfig, ok := config.OAuthProviders[provider]
	if !ok || !oAuthProviderConfig.Enabled {
		log.Debugf("OAuth provider %q not found or disabled", provider)
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fmt.Sprintf("OAuth provider '%s' is not enabled", provider), "LAYR_AUTH_001")
		return
	}

	resolvedProviderConfig, err := oauth.ResolveProviderConfig(provider, oauth.ProviderConfig{
		Name:            provider,
		Preset:          oAuthProviderConfig.Preset,
		AuthURL:         oAuthProviderConfig.AuthURL,
		TokenURL:        oAuthProviderConfig.TokenURL,
		UserInfoURL:     oAuthProviderConfig.UserInfoURL,
		ClientID:        oAuthProviderConfig.ClientID,
		Scope:           oAuthProviderConfig.Scope,
		ResponseMode:    oAuthProviderConfig.ResponseMode,
		ResponseType:    oAuthProviderConfig.ResponseType,
		IDAttribute:     oAuthProviderConfig.IDAttribute,
		EmailAttribute:  oAuthProviderConfig.EmailAttribute,
		NameAttribute:   oAuthProviderConfig.NameAttribute,
		AvatarAttribute: oAuthProviderConfig.AvatarAttribute,
	})
	if err != nil {
		log.Debugf("failed to resolve OAuth provider config for %q: %v", provider, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error(), "LAYR_AUTH_001")
		return
	}

	redirectURI := request.URL.Query().Get("redirect_uri")
	if redirectURI == "" {
		redirectURI = fmt.Sprintf("%s/api/v1/auth/oauth/%s/callback", core.GetConfig().ServerBaseURL(), provider)
	}

	state := request.URL.Query().Get("state")
	if state == "" {
		state = uuid.NewV7().String()
	}

	oidcStateID := request.URL.Query().Get("oidc_state")
	var anonymousID string
	if anonymousUserRecord, resolveAnonymousCallerErr := handler.resolveAnonymousCaller(request); resolveAnonymousCallerErr == nil && anonymousUserRecord != nil {
		anonymousID = anonymousUserRecord.ID
		log.Tracef("attaching anonymous user %s to OAuth state %s", anonymousUserRecord.ID, state)
	}

	if handler.kvStore != nil {
		oAuthStatePayload := OAuthStatePayload{
			StateID:       state,
			Provider:      provider,
			RedirectURI:   redirectURI,
			OIDCStateID:   oidcStateID,
			AnonymousID:   anonymousID,
			CreatedAtUnix: time.Now().Unix(),
		}
		payloadJSON, _ := json.Marshal(oAuthStatePayload)
		log.Tracef("persisting OAuth state payload to KV store: %s", state)
		_ = handler.kvStore.Set(request.Context(), "auth:pkce:"+state, string(payloadJSON), 10*time.Minute)
	}

	if scopeQuery := request.URL.Query().Get("scope"); scopeQuery != "" {
		resolvedProviderConfig.Scope = scopeQuery
	}

	authURL, _ := oauth.BuildAuthorizeURLWithConfig(resolvedProviderConfig, redirectURI, state)
	log.Debugf("redirecting to OAuth authorization URL for %s", provider)
	http.Redirect(responseWriter, request, authURL, http.StatusFound)
}

// HandleOAuthToken handles token exchange requests.
func (handler *BaseHandler) HandleOAuthToken(responseWriter http.ResponseWriter, request *http.Request) {
	if request.FormValue("grant_type") != "" {
		handler.handleOIDCToken(responseWriter, request)
		return
	}
	handler.HandleOAuthCallback(responseWriter, request)
}

// HandleOAuthCallback processes the incoming OAuth redirect callback.
func (handler *BaseHandler) HandleOAuthCallback(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling OAuth callback request")
	var provider, code, redirectURI, state string

	if request.Method == http.MethodGet {
		pathSegments := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
		if len(pathSegments) >= 5 && pathSegments[4] != "token" && pathSegments[4] != "callback" {
			provider = pathSegments[4]
		}
		code = request.URL.Query().Get("code")
		redirectURI = request.URL.Query().Get("redirect_uri")
		state = request.URL.Query().Get("state")
	} else if strings.Contains(request.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		_ = request.ParseForm()
		pathSegments := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
		if len(pathSegments) >= 5 && pathSegments[4] != "token" && pathSegments[4] != "callback" {
			provider = pathSegments[4]
		}
		if provider == "" {
			provider = request.FormValue("provider")
		}
		code = request.FormValue("code")
		redirectURI = request.FormValue("redirect_uri")
		state = request.FormValue("state")
		if state == "" {
			state = request.URL.Query().Get("state")
		}
	} else {
		var oauthTokenExchangeRequest OAuthTokenExchangeRequest
		_ = json.NewDecoder(request.Body).Decode(&oauthTokenExchangeRequest)
		provider = oauthTokenExchangeRequest.Provider
		if provider == "" {
			pathSegments := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
			if len(pathSegments) >= 5 && pathSegments[4] != "token" && pathSegments[4] != "callback" {
				provider = pathSegments[4]
			}
		}
		code = oauthTokenExchangeRequest.Code
		redirectURI = oauthTokenExchangeRequest.RedirectURI
		state = oauthTokenExchangeRequest.State
		if state == "" {
			state = request.URL.Query().Get("state")
		}
	}

	if state == "" {
		log.Debug("OAuth callback rejected: missing state parameter")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "OAuth state parameter is required", "LAYR_AUTH_INVALID_STATE")
		return
	}

	if handler.kvStore == nil {
		log.Debug("OAuth callback rejected: KV store unavailable to verify state")
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "OAuth state has expired or is invalid", "LAYR_AUTH_INVALID_STATE")
		return
	}

	storedState, err := handler.kvStore.Get(request.Context(), "auth:pkce:"+state)
	if err != nil || storedState == "" {
		log.Debugf("OAuth callback rejected: state %s not found in KV store or expired", state)
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "OAuth state has expired or is invalid", "LAYR_AUTH_INVALID_STATE")
		return
	}

	var parsedOAuthStatePayload OAuthStatePayload
	var isPayload bool
	if unmarshalErr := json.Unmarshal([]byte(storedState), &parsedOAuthStatePayload); unmarshalErr == nil && parsedOAuthStatePayload.StateID != "" {
		isPayload = true
		if parsedOAuthStatePayload.StateID != state {
			log.Debugf("OAuth callback rejected: state mismatch (%s != %s)", parsedOAuthStatePayload.StateID, state)
			core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "OAuth state has expired or is invalid", "LAYR_AUTH_INVALID_STATE")
			return
		}
		if redirectURI == "" && parsedOAuthStatePayload.RedirectURI != "" {
			redirectURI = parsedOAuthStatePayload.RedirectURI
		}
	} else if storedState != state {
		log.Debugf("OAuth callback rejected: plain state mismatch (%s != %s)", storedState, state)
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "OAuth state has expired or is invalid", "LAYR_AUTH_INVALID_STATE")
		return
	}

	_ = handler.kvStore.Delete(request.Context(), "auth:pkce:"+state)

	if provider == "" || code == "" {
		log.Debug("OAuth callback rejected: provider and authorization code are required")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Provider and authorization code required", "LAYR_AUTH_001")
		return
	}

	config := handler.configManager.Get()
	oAuthProviderConfig, ok := config.OAuthProviders[provider]
	if !ok || !oAuthProviderConfig.Enabled {
		log.Debugf("OAuth callback rejected: provider %q is disabled", provider)
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fmt.Sprintf("OAuth provider '%s' is disabled", provider), "LAYR_AUTH_001")
		return
	}

	clientSecret, _ := handler.configManager.DecryptSecret(oAuthProviderConfig.ClientSecret)
	if redirectURI == "" {
		redirectURI = fmt.Sprintf("%s/api/v1/auth/oauth/%s/callback", core.GetConfig().ServerBaseURL(), provider)
	}

	resolvedProviderConfig, err := oauth.ResolveProviderConfig(provider, oauth.ProviderConfig{
		Name:            provider,
		Preset:          oAuthProviderConfig.Preset,
		AuthURL:         oAuthProviderConfig.AuthURL,
		TokenURL:        oAuthProviderConfig.TokenURL,
		UserInfoURL:     oAuthProviderConfig.UserInfoURL,
		ClientID:        oAuthProviderConfig.ClientID,
		ClientSecret:    clientSecret,
		Scope:           oAuthProviderConfig.Scope,
		ResponseMode:    oAuthProviderConfig.ResponseMode,
		ResponseType:    oAuthProviderConfig.ResponseType,
		IDAttribute:     oAuthProviderConfig.IDAttribute,
		EmailAttribute:  oAuthProviderConfig.EmailAttribute,
		NameAttribute:   oAuthProviderConfig.NameAttribute,
		AvatarAttribute: oAuthProviderConfig.AvatarAttribute,
	})
	if err != nil {
		log.Debugf("failed to resolve OAuth provider config for callback %q: %v", provider, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error(), "LAYR_AUTH_001")
		return
	}

	log.Tracef("exchanging authorization code with provider %s", provider)
	userInfo, err := oauth.ExchangeCodeWithConfig(request.Context(), resolvedProviderConfig, code, redirectURI)
	if err != nil {
		log.Debugf("OAuth exchange error for provider %s: %v", provider, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("OAuth exchange error: %s", err.Error()), "LAYR_AUTH_001")
		return
	}

	if handler.db == nil {
		log.Debug("OAuth callback failed: database pool is not available")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()

	// 1. Check existing identity
	var existingUserID string
	err = handler.db.QueryRow(ctx, "SELECT user_id FROM auth.identities WHERE provider = $1 AND provider_user_id = $2", provider, userInfo.ProviderUserID).Scan(&existingUserID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Debugf("OAuth callback database query error: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database error", "LAYR_AUTH_001")
		return
	}

	var userRecord UserRecord
	var rawProperties []byte

	anonymousUserRecord, _ := handler.resolveAnonymousCaller(request)
	if anonymousUserRecord != nil {
		log.Debugf("linking OAuth identity %s:%s to anonymous user %s", provider, userInfo.ProviderUserID, anonymousUserRecord.ID)
		if err == nil && existingUserID != anonymousUserRecord.ID {
			log.Debugf("conflict: OAuth identity %s:%s is already linked to user %s", provider, userInfo.ProviderUserID, existingUserID)
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "OAuth identity is already linked to another account", "LAYR_AUTH_001")
			return
		}

		var emailPtr *string
		if userInfo.Email != "" {
			emailPtr = &userInfo.Email
			var conflictingUserID string
			conflictErr := handler.db.QueryRow(ctx, "SELECT id FROM auth.users WHERE email = $1", userInfo.Email).Scan(&conflictingUserID)
			if conflictErr == nil && conflictingUserID != anonymousUserRecord.ID {
				log.Debugf("conflict: email %s is already in use by user %s", userInfo.Email, conflictingUserID)
				core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Email is already in use by another account", "LAYR_AUTH_001")
				return
			}
		}

		propertiesJSON, _ := json.Marshal(userInfo.Properties)

		// Link identity
		_, _ = handler.db.Exec(ctx, `
			INSERT INTO auth.identities (user_id, provider, provider_user_id, properties, last_sign_in_at, created_at, last_updated_at)
			VALUES ($1, $2, $3, $4, clock_timestamp(), clock_timestamp(), clock_timestamp())
			ON CONFLICT (provider, provider_user_id) DO UPDATE SET user_id = $1, last_sign_in_at = clock_timestamp(), properties = $4
		`, anonymousUserRecord.ID, provider, userInfo.ProviderUserID, propertiesJSON)

		// Convert anonymous user to authenticated
		updateQuery := `
			UPDATE auth.users
			SET email = COALESCE(email, $1),
			    email_verified_at = CASE WHEN $1 IS NOT NULL THEN COALESCE(email_verified_at, clock_timestamp()) ELSE email_verified_at END,
			    is_anonymous = false,
			    properties = COALESCE(properties, '{}'::jsonb) || $2::jsonb,
			    last_updated_at = clock_timestamp()
			WHERE id = $3
			RETURNING id, email, phone, role, is_anonymous, properties, created_at, last_updated_at
		`
		updateErr := handler.db.QueryRow(ctx, updateQuery, emailPtr, propertiesJSON, anonymousUserRecord.ID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
		)
		if updateErr != nil {
			log.Debugf("failed to convert anonymous user %s: %v", anonymousUserRecord.ID, updateErr)
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

		handler.CompleteOAuthFlow(responseWriter, request, userRecord, parsedOAuthStatePayload, isPayload)
		return
	}

	if err == nil {
		log.Debugf("logging in existing federated user %s via %s", existingUserID, provider)
		_ = handler.db.QueryRow(ctx, `
			SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at 
			FROM auth.users WHERE id = $1
		`, existingUserID).Scan(&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous, &userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil, &userRecord.EncryptedMFASecret, &userRecord.MFAEnabled, &rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt)
		userRecord.Properties = make(map[string]any)
		if len(rawProperties) > 0 {
			_ = json.Unmarshal(rawProperties, &userRecord.Properties)
		}
		if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
			log.Warnf("failed OAuth sign in for locked user %s", userRecord.ID)
			core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked", "LAYR_AUTH_005")
			return
		}
		_, _ = handler.db.Exec(ctx, "UPDATE auth.identities SET last_sign_in_at = clock_timestamp() WHERE provider = $1 AND provider_user_id = $2", provider, userInfo.ProviderUserID)
	} else {
		log.Debugf("registering new federated user via %s", provider)
		var emailPtr *string
		if userInfo.Email != "" {
			emailPtr = &userInfo.Email
		}
		propertiesJSON, _ := json.Marshal(userInfo.Properties)

		_ = handler.db.QueryRow(ctx, `
			INSERT INTO auth.users (email, role, email_verified_at, properties, created_at, last_updated_at)
			VALUES ($1, 'authenticated', clock_timestamp(), $2, clock_timestamp(), clock_timestamp())
			ON CONFLICT (email) DO UPDATE SET last_updated_at = clock_timestamp()
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		`, emailPtr, propertiesJSON).Scan(&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous, &userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil, &userRecord.EncryptedMFASecret, &userRecord.MFAEnabled, &rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt)
		userRecord.Properties = make(map[string]any)
		if len(rawProperties) > 0 {
			_ = json.Unmarshal(rawProperties, &userRecord.Properties)
		}

		_, _ = handler.db.Exec(ctx, `
			INSERT INTO auth.identities (user_id, provider, provider_user_id, properties, last_sign_in_at, created_at, last_updated_at)
			VALUES ($1, $2, $3, $4, clock_timestamp(), clock_timestamp(), clock_timestamp())
			ON CONFLICT (provider, provider_user_id) DO UPDATE SET last_sign_in_at = clock_timestamp()
		`, userRecord.ID, provider, userInfo.ProviderUserID, propertiesJSON)

		log.Tracef("created new federated user %s for %s:%s", userRecord.ID, provider, userInfo.ProviderUserID)
		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewUserSignedUpEvent(userRecord.ID, UserSignedUpEventData(userRecord)))
		}
	}

	handler.CompleteOAuthFlow(responseWriter, request, userRecord, parsedOAuthStatePayload, isPayload)
}

// CompleteOAuthFlow completes the OAuth session flow, redirecting to OIDC client if linked, or issuing session tokens.
func (handler *BaseHandler) CompleteOAuthFlow(responseWriter http.ResponseWriter, request *http.Request, userRecord UserRecord, parsedOAuthStatePayload OAuthStatePayload, isPayload bool) {
	log.Debugf("completing OAuth flow for user %s (isPayload: %t, OIDCStateID: %s)", userRecord.ID, isPayload, parsedOAuthStatePayload.OIDCStateID)
	if isPayload && parsedOAuthStatePayload.OIDCStateID != "" && handler.kvStore != nil {
		oidcStateJSON, err := handler.kvStore.Get(request.Context(), "auth:oidc:state:"+parsedOAuthStatePayload.OIDCStateID)
		if err == nil && oidcStateJSON != "" {
			var oidcAuthorizationStatePayload OIDCAuthorizationStatePayload
			if err := json.Unmarshal([]byte(oidcStateJSON), &oidcAuthorizationStatePayload); err == nil {
				log.Tracef("resolving linked OIDC state %s for client %s", parsedOAuthStatePayload.OIDCStateID, oidcAuthorizationStatePayload.ClientID)
				_ = handler.kvStore.Delete(request.Context(), "auth:oidc:state:"+parsedOAuthStatePayload.OIDCStateID)
				code := handler.issueOIDCAuthorizationCode(request.Context(), oidcAuthorizationStatePayload.ClientID, oidcAuthorizationStatePayload.RedirectURI, userRecord.ID, oidcAuthorizationStatePayload.Scope, oidcAuthorizationStatePayload.CodeChallenge, oidcAuthorizationStatePayload.CodeChallengeMethod, oidcAuthorizationStatePayload.Nonce)
				refreshToken := jwt.GenerateRefreshToken()
				refreshTokenHash := jwt.HashRefreshToken(refreshToken)
				config := handler.configManager.Get()
				refreshTokenExpirySeconds := config.Sessions.RefreshTokenExpirySeconds
				if refreshTokenExpirySeconds <= 0 {
					refreshTokenExpirySeconds = defaultRefreshTokenExpirySeconds
				}
				expiresAt := time.Now().Add(time.Duration(refreshTokenExpirySeconds) * time.Second)
				if handler.db != nil {
					log.Tracef("saving session for OIDC flow for user %s", userRecord.ID)
					_, _ = handler.db.Exec(request.Context(), `
						INSERT INTO auth.sessions (user_id, refresh_token_hash, expires_at, created_at)
						VALUES ($1, $2, $3, clock_timestamp())
					`, userRecord.ID, refreshTokenHash, expiresAt)
				}
				isSecure := core.IsSecureRequest(request)
				core.SetSessionCookie(responseWriter, AuthSessionCookieName, AuthSessionInsecureCookieName, refreshToken, expiresAt, isSecure)

				targetURL, parseErr := url.Parse(oidcAuthorizationStatePayload.RedirectURI)
				if parseErr == nil {
					queryValues := targetURL.Query()
					queryValues.Set("code", code)
					if oidcAuthorizationStatePayload.ClientState != "" {
						queryValues.Set("state", oidcAuthorizationStatePayload.ClientState)
					}
					targetURL.RawQuery = queryValues.Encode()
					log.Debugf("redirecting to OIDC client callback: %s", targetURL.String())
					http.Redirect(responseWriter, request, targetURL.String(), http.StatusFound)
					return
				}
			}
		}
	}

	provider := request.PathValue("provider")
	handler.issueSessionResponse(responseWriter, request, userRecord, "oauth", provider)
}

// HandleOAuthUserInfo returns user details for the authenticated OAuth bearer token caller.
func (handler *BaseHandler) HandleOAuthUserInfo(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling OAuth user info request")
	authHeader := request.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		log.Debug("OAuth userinfo rejected: missing Bearer prefix in Authorization header")
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Bearer token required", "LAYR_AUTH_002")
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := handler.signer.VerifyAccessToken(token)
	if err != nil {
		log.Debugf("OAuth userinfo rejected: invalid access token: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid or expired access token", "LAYR_AUTH_002")
		return
	}

	if handler.db == nil {
		log.Debug("OAuth userinfo failed: database pool is not available")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	var userRecord UserRecord
	var rawProperties []byte
	err = handler.db.QueryRow(request.Context(), `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users WHERE id = $1
	`, claims.Subject).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Debugf("OAuth userinfo user not found for ID %s: %v", claims.Subject, err)
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_001")
			return
		}
		log.Debugf("OAuth userinfo database query error for ID %s: %v", claims.Subject, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database error", "LAYR_AUTH_001")
		return
	}
	_ = json.Unmarshal(rawProperties, &userRecord.Properties)

	log.Debugf("successfully retrieved OAuth user info for %s", userRecord.ID)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(userRecord)
}
