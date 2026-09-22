package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"uuid"

	"layr.sh/core"
)

func (handler *BaseHandler) handleListSessions(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list active user sessions request")
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID := authContext.UserID
	currentRefreshTokenHash := authContext.RefreshTokenHash
	currentSessionID := authContext.JWT.SessionID

	ctx := request.Context()
	rows, err := handler.kernel.DB().Query(ctx, `
		SELECT id, ip_address::text, user_agent, refresh_token_hash, expires_at, created_at
		FROM auth.sessions
		WHERE user_id = $1 AND expires_at > clock_timestamp()
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to query active sessions for user %s: %v", userID, err))
		return
	}
	defer rows.Close()

	sessions := make([]UserSession, 0)
	for rows.Next() {
		var sessionID string
		var ipAddress *string
		var userAgent *string
		var refreshTokenHash string
		var expiresAt time.Time
		var createdAt time.Time

		_ = rows.Scan(&sessionID, &ipAddress, &userAgent, &refreshTokenHash, &expiresAt, &createdAt)

		isCurrent := false
		if currentSessionID != "" && sessionID == currentSessionID {
			isCurrent = true
		} else if currentRefreshTokenHash != "" && refreshTokenHash == currentRefreshTokenHash {
			isCurrent = true
		}

		sessions = append(sessions, UserSession{
			ID:        sessionID,
			IPAddress: ipAddress,
			UserAgent: userAgent,
			IsCurrent: isCurrent,
			ExpiresAt: expiresAt,
			CreatedAt: createdAt,
		})
	}

	if len(sessions) == 1 && currentSessionID == "" && currentRefreshTokenHash == "" {
		sessions[0].IsCurrent = true
	}

	log.Debugf("retrieved %d active session(s) for user %s", len(sessions), userID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, ListSessionsResponse{
		Sessions: sessions,
		Count:    len(sessions),
	})
}

func (handler *BaseHandler) handleRevokeSession(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling revoke session request")
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID := authContext.UserID

	targetSessionID := request.PathValue("session_id")
	if targetSessionID == "" {
		path := strings.TrimSuffix(request.URL.Path, "/")
		targetSessionID = strings.TrimPrefix(path, "/v1/auth/user/sessions/")
		if targetSessionID == path {
			targetSessionID = strings.TrimPrefix(path, "/v1/auth/sessions/")
		}
	}
	if targetSessionID == "" || strings.Contains(targetSessionID, "/") {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid session ID")
		return
	}

	if _, parseErr := uuid.Parse(targetSessionID); parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid session ID")
		return
	}

	ctx := request.Context()
	var deletedRefreshTokenHash string
	var deletedClientID *string
	err := handler.kernel.DB().QueryRow(ctx, `
		DELETE FROM auth.sessions
		WHERE id = $1 AND user_id = $2
		RETURNING refresh_token_hash, client_id
	`, targetSessionID, userID).Scan(&deletedRefreshTokenHash, &deletedClientID)

	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Session not found")
		return
	}

	if deletedRefreshTokenHash != "" {
		_ = handler.kernel.KVStore().Delete(ctx, "auth:session:"+deletedRefreshTokenHash)
	}

	if deletedClientID != nil && *deletedClientID != "" {
		config := handler.configManager.Get()
		dispatchBackChannelSignOut(ctx, handler.httpClient, handler.kernel.JWTSigner(), config.OIDC.Clients, []ClientSessionInfo{
			{ClientID: *deletedClientID, SessionID: targetSessionID, UserID: userID},
		})
	}

	user, _ := fetchUserByID(ctx, handler.kernel.DB(), userID)
	handler.kernel.EventBus().Publish(ctx, NewSessionDeletedEvent(targetSessionID, SessionDeletedEventData{
		User:      user,
		SessionID: &targetSessionID,
	}))

	log.Debugf("session %s successfully revoked for user %s", targetSessionID, userID)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *BaseHandler) handleRevokeOtherSessions(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling revoke other sessions request")
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID := authContext.UserID
	currentRefreshTokenHash := authContext.RefreshTokenHash
	currentSessionID := authContext.JWT.SessionID

	ctx := request.Context()

	var resolvedCurrentSessionID string
	if currentSessionID != "" {
		if _, parseErr := uuid.Parse(currentSessionID); parseErr == nil {
			resolvedCurrentSessionID = currentSessionID
		}
	}
	if resolvedCurrentSessionID == "" && currentRefreshTokenHash != "" {
		_ = handler.kernel.DB().QueryRow(ctx, `
			SELECT id FROM auth.sessions WHERE user_id = $1 AND refresh_token_hash = $2
		`, userID, currentRefreshTokenHash).Scan(&resolvedCurrentSessionID)
	}

	if resolvedCurrentSessionID == "" {
		var activeSessionCount int
		err := handler.kernel.DB().QueryRow(ctx, `
			SELECT count(*) FROM auth.sessions WHERE user_id = $1 AND expires_at > clock_timestamp()
		`, userID).Scan(&activeSessionCount)
		if err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to count active sessions for user %s: %v", userID, err))
			return
		}

		if activeSessionCount <= 1 {
			log.Debugf("no other active sessions to revoke for user %s", userID)
			core.WriteJSONResponse(responseWriter, http.StatusOK, RevokeOtherSessionsResponse{
				RevokedCount: 0,
			})
			return
		}

		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Current session identifier required to revoke other devices")
		return
	}

	rows, err := handler.kernel.DB().Query(ctx, `
		DELETE FROM auth.sessions
		WHERE user_id = $1 AND id != $2
		RETURNING id, client_id, refresh_token_hash
	`, userID, resolvedCurrentSessionID)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to revoke other sessions for user %s: %v", userID, err))
		return
	}
	defer rows.Close()

	deletedHashes := make([]string, 0)
	targetSessions := make([]ClientSessionInfo, 0)
	for rows.Next() {
		var deletedSessionID string
		var deletedClientID *string
		var hash string
		if scanErr := rows.Scan(&deletedSessionID, &deletedClientID, &hash); scanErr == nil {
			deletedHashes = append(deletedHashes, hash)
			if deletedClientID != nil && *deletedClientID != "" {
				targetSessions = append(targetSessions, ClientSessionInfo{
					ClientID:  *deletedClientID,
					SessionID: deletedSessionID,
					UserID:    userID,
				})
			}
		}
	}

	for _, hash := range deletedHashes {
		if hash != "" {
			_ = handler.kernel.KVStore().Delete(ctx, "auth:session:"+hash)
		}
	}

	if len(targetSessions) > 0 {
		config := handler.configManager.Get()
		dispatchBackChannelSignOut(ctx, handler.httpClient, handler.kernel.JWTSigner(), config.OIDC.Clients, targetSessions)
	}

	user, _ := fetchUserByID(ctx, handler.kernel.DB(), userID)
	revokedCount := len(deletedHashes)
	handler.kernel.EventBus().Publish(ctx, NewSessionDeletedEvent(userID, SessionDeletedEventData{
		User:         user,
		RevokedCount: &revokedCount,
	}))

	log.Debugf("revoked %d other session(s) for user %s", len(deletedHashes), userID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, RevokeOtherSessionsResponse{
		RevokedCount: int64(len(deletedHashes)),
	})
}

func (handler *BaseHandler) handleRefreshToken(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling token refresh request")
	var refreshTokenInput RefreshTokenInput
	if request.Body != nil && request.ContentLength != 0 {
		if err := json.NewDecoder(request.Body).Decode(&refreshTokenInput); err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
			return
		}
	}
	if refreshTokenInput.RefreshToken == "" {
		refreshTokenInput.RefreshToken = core.ExtractRequestSessionToken(request)
	}
	if refreshTokenInput.RefreshToken == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Refresh token required")
		return
	}

	tokenHash := handler.kernel.JWTSigner().HashRefreshToken(refreshTokenInput.RefreshToken)
	ctx := request.Context()

	config := handler.configManager.Get()
	if config.Cache.FastPathSessionsEnabled {
		if cachedData, err := handler.kernel.KVStore().Get(ctx, "auth:session:"+tokenHash); err == nil && cachedData != "" {
			var cachedSession CachedSession
			if err := json.Unmarshal([]byte(cachedData), &cachedSession); err == nil && cachedSession.User.ID != "" {
				log.Tracef("fast-path session cache hit for user %s", cachedSession.User.ID)
				_ = handler.kernel.KVStore().Delete(ctx, "auth:session:"+tokenHash)
				_, _ = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.sessions WHERE refresh_token_hash = $1", tokenHash)
				if cachedSession.User.LockedUntil != nil && time.Now().UTC().Before(*cachedSession.User.LockedUntil) {
					core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
					return
				}
				handler.issueSessionResponse(responseWriter, request, cachedSession.User, "session_refresh")
				return
			}
		}
	}

	var sessionID, userID string
	var expiresAt time.Time

	err := handler.kernel.DB().QueryRow(ctx, `
		SELECT id, user_id, expires_at 
		FROM auth.sessions 
		WHERE refresh_token_hash = $1
	`, tokenHash).Scan(&sessionID, &userID, &expiresAt)

	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Refresh token revoked or invalid")
		return
	}

	if time.Now().UTC().After(expiresAt) {
		_, _ = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.sessions WHERE id = $1", sessionID)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Refresh token expired")
		return
	}

	var user User
	var rawProperties []byte
	err = handler.kernel.DB().QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Refresh token revoked or invalid")
		return
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	if user.LockedUntil != nil && time.Now().UTC().Before(*user.LockedUntil) {
		_, _ = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.sessions WHERE id = $1", sessionID)
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	// Rotate refresh token
	_, _ = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.sessions WHERE id = $1", sessionID)
	handler.issueSessionResponse(responseWriter, request, user, "session_refresh")
}

func (handler *BaseHandler) handleSignOut(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling sign-out request")
	var refreshTokenInput RefreshTokenInput
	if request.Body != nil && request.ContentLength != 0 {
		_ = json.NewDecoder(request.Body).Decode(&refreshTokenInput)
	}
	if refreshTokenInput.RefreshToken == "" {
		refreshTokenInput.RefreshToken = core.ExtractRequestSessionToken(request)
	}

	if refreshTokenInput.RefreshToken != "" {
		tokenHash := handler.kernel.JWTSigner().HashRefreshToken(refreshTokenInput.RefreshToken)
		_ = handler.kernel.KVStore().Delete(request.Context(), "auth:session:"+tokenHash)
		var sessionID, userID string
		var clientID *string
		err := handler.kernel.DB().QueryRow(request.Context(), `
			DELETE FROM auth.sessions 
			WHERE refresh_token_hash = $1
			RETURNING id, user_id, client_id
		`, tokenHash).Scan(&sessionID, &userID, &clientID)
		if err == nil {
			if clientID != nil && *clientID != "" {
				config := handler.configManager.Get()
				dispatchBackChannelSignOut(request.Context(), handler.httpClient, handler.kernel.JWTSigner(), config.OIDC.Clients, []ClientSessionInfo{
					{ClientID: *clientID, SessionID: sessionID, UserID: userID},
				})
			}
			user, _ := fetchUserByID(request.Context(), handler.kernel.DB(), userID)
			handler.kernel.EventBus().Publish(request.Context(), NewSessionDeletedEvent(sessionID, SessionDeletedEventData{
				User:      user,
				SessionID: &sessionID,
			}))
		}
	}

	core.ClearSessionCookie(responseWriter, request)
	responseWriter.WriteHeader(http.StatusNoContent)
}
