package auth

import (
	"encoding/json"
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
		log.Debug("list user sessions rejected: unauthenticated caller")
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	userID := authContext.UserID
	currentRefreshTokenHash := authContext.RefreshTokenHash
	currentSessionID := authContext.JWT.SessionID

	if handler.db == nil {
		log.Debug("list user sessions rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	rows, err := handler.db.Query(ctx, `
		SELECT id, ip_address::text, user_agent, refresh_token_hash, expires_at, created_at
		FROM auth.sessions
		WHERE user_id = $1 AND expires_at > clock_timestamp()
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		log.Debugf("failed to query active sessions for user %s: %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to query sessions", "LAYR_AUTH_001")
		return
	}
	defer rows.Close()

	sessions := make([]UserSessionRecord, 0)
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

		sessions = append(sessions, UserSessionRecord{
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
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(ListUserSessionsResponse{
		Sessions: sessions,
		Count:    len(sessions),
	})
}

func (handler *BaseHandler) handleRevokeSession(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling revoke session request")
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		log.Debug("revoke session rejected: unauthenticated caller")
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	userID := authContext.UserID

	targetSessionID := request.PathValue("session_id")
	if targetSessionID == "" {
		path := strings.TrimSuffix(request.URL.Path, "/")
		targetSessionID = strings.TrimPrefix(path, "/api/v1/auth/user/sessions/")
	}
	if targetSessionID == "" || strings.Contains(targetSessionID, "/") {
		log.Debugf("revoke session rejected: invalid session ID format: %q", targetSessionID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid session ID", "LAYR_AUTH_001")
		return
	}

	if _, parseErr := uuid.Parse(targetSessionID); parseErr != nil {
		log.Debugf("revoke session rejected: invalid UUID %q: %v", targetSessionID, parseErr)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid session UUID", "LAYR_AUTH_001")
		return
	}

	if handler.db == nil {
		log.Debug("revoke session rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var deletedRefreshTokenHash string
	var deletedClientID *string
	err := handler.db.QueryRow(ctx, `
		DELETE FROM auth.sessions
		WHERE id = $1 AND user_id = $2
		RETURNING refresh_token_hash, client_id
	`, targetSessionID, userID).Scan(&deletedRefreshTokenHash, &deletedClientID)

	if err != nil {
		log.Debugf("revoke session failed: session %s not found for user %s: %v", targetSessionID, userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Session not found", "LAYR_AUTH_001")
		return
	}

	if handler.kvStore != nil && deletedRefreshTokenHash != "" {
		_ = handler.kvStore.Delete(ctx, "auth:session:"+deletedRefreshTokenHash)
	}

	if deletedClientID != nil && *deletedClientID != "" {
		config := handler.configManager.Get()
		dispatchBackChannelSignOut(ctx, handler.httpClient, handler.jwtSigner, config.OIDC.Clients, []ClientSessionInfo{
			{ClientID: *deletedClientID, SessionID: targetSessionID, UserID: userID},
		})
	}

	if handler.eventBus != nil {
		userRecord, _ := fetchUserRecordByID(ctx, handler.db, userID)
		handler.eventBus.Publish(ctx, NewSessionDeletedEvent(targetSessionID, SessionDeletedEventData{
			User:      userRecord,
			SessionID: &targetSessionID,
		}))
	}

	log.Debugf("session %s successfully revoked for user %s", targetSessionID, userID)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *BaseHandler) handleRevokeOtherSessions(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling revoke other sessions request")
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		log.Debug("revoke other sessions rejected: unauthenticated caller")
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	if handler.db == nil {
		log.Debug("revoke other sessions rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
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
		_ = handler.db.QueryRow(ctx, `
			SELECT id FROM auth.sessions WHERE user_id = $1 AND refresh_token_hash = $2
		`, userID, currentRefreshTokenHash).Scan(&resolvedCurrentSessionID)
	}

	if resolvedCurrentSessionID == "" {
		var activeSessionCount int
		err := handler.db.QueryRow(ctx, `
			SELECT count(*) FROM auth.sessions WHERE user_id = $1 AND expires_at > clock_timestamp()
		`, userID).Scan(&activeSessionCount)
		if err != nil {
			log.Debugf("failed to count active sessions for user %s: %v", userID, err)
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to count active sessions", "LAYR_AUTH_001")
			return
		}

		if activeSessionCount <= 1 {
			log.Debugf("no other active sessions to revoke for user %s", userID)
			responseWriter.Header().Set("Content-Type", "application/json")
			responseWriter.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(responseWriter).Encode(RevokeOtherSessionsResponse{
				RevokedCount: 0,
			})
			return
		}

		log.Debugf("revoke other sessions rejected: ambiguous current session for user %s with %d sessions", userID, activeSessionCount)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Current session identifier required to revoke other devices", "LAYR_AUTH_001")
		return
	}

	rows, err := handler.db.Query(ctx, `
		DELETE FROM auth.sessions
		WHERE user_id = $1 AND id != $2
		RETURNING id, client_id, refresh_token_hash
	`, userID, resolvedCurrentSessionID)
	if err != nil {
		log.Debugf("failed to revoke other sessions for user %s: %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to revoke other sessions", "LAYR_AUTH_001")
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

	if handler.kvStore != nil {
		for _, hash := range deletedHashes {
			if hash != "" {
				_ = handler.kvStore.Delete(ctx, "auth:session:"+hash)
			}
		}
	}

	if len(targetSessions) > 0 {
		config := handler.configManager.Get()
		dispatchBackChannelSignOut(ctx, handler.httpClient, handler.jwtSigner, config.OIDC.Clients, targetSessions)
	}

	if handler.eventBus != nil {
		userRecord, _ := fetchUserRecordByID(ctx, handler.db, userID)
		revokedCount := len(deletedHashes)
		handler.eventBus.Publish(ctx, NewSessionDeletedEvent(userID, SessionDeletedEventData{
			User:         userRecord,
			RevokedCount: &revokedCount,
		}))
	}

	log.Debugf("revoked %d other session(s) for user %s", len(deletedHashes), userID)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(RevokeOtherSessionsResponse{
		RevokedCount: int64(len(deletedHashes)),
	})
}

func (handler *BaseHandler) handleTokenRefresh(responseWriter http.ResponseWriter, request *http.Request) {
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
		refreshTokenRequest.RefreshToken = core.ExtractRequestSessionToken(request)
	}
	if refreshTokenRequest.RefreshToken == "" {
		log.Debug("token refresh rejected: missing refresh token")
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Refresh token required", "LAYR_AUTH_002")
		return
	}

	tokenHash := handler.jwtSigner.HashRefreshToken(refreshTokenRequest.RefreshToken)
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
				if cachedSession.User.LockedUntil != nil && time.Now().UTC().Before(*cachedSession.User.LockedUntil) {
					log.Warnf("failed token refresh for locked user %s via fast-path cache", cachedSession.User.ID)
					core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked", "LAYR_AUTH_005")
					return
				}
				handler.issueSessionResponse(responseWriter, request, cachedSession.User, "session_refresh")
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
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users WHERE id = $1
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
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

	if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
		log.Warnf("failed token refresh for locked user %s", userRecord.ID)
		_, _ = handler.db.Exec(ctx, "DELETE FROM auth.sessions WHERE id = $1", sessionID)
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked", "LAYR_AUTH_005")
		return
	}

	// Rotate refresh token
	_, _ = handler.db.Exec(ctx, "DELETE FROM auth.sessions WHERE id = $1", sessionID)
	handler.issueSessionResponse(responseWriter, request, userRecord, "session_refresh")
}

func (handler *BaseHandler) handleSignOut(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling sign-out request")
	var refreshTokenRequest RefreshTokenRequest
	if request.Body != nil && request.ContentLength != 0 {
		_ = json.NewDecoder(request.Body).Decode(&refreshTokenRequest)
	}
	if refreshTokenRequest.RefreshToken == "" {
		refreshTokenRequest.RefreshToken = core.ExtractRequestSessionToken(request)
	}

	if refreshTokenRequest.RefreshToken != "" {
		tokenHash := handler.jwtSigner.HashRefreshToken(refreshTokenRequest.RefreshToken)
		if handler.kvStore != nil {
			_ = handler.kvStore.Delete(request.Context(), "auth:session:"+tokenHash)
		}
		if handler.db != nil {
			var sessionID, userID string
			var clientID *string
			err := handler.db.QueryRow(request.Context(), `
				DELETE FROM auth.sessions 
				WHERE refresh_token_hash = $1
				RETURNING id, user_id, client_id
			`, tokenHash).Scan(&sessionID, &userID, &clientID)
			if err == nil {
				if clientID != nil && *clientID != "" {
					config := handler.configManager.Get()
					dispatchBackChannelSignOut(request.Context(), handler.httpClient, handler.jwtSigner, config.OIDC.Clients, []ClientSessionInfo{
						{ClientID: *clientID, SessionID: sessionID, UserID: userID},
					})
				}
				if handler.eventBus != nil {
					userRecord, _ := fetchUserRecordByID(request.Context(), handler.db, userID)
					handler.eventBus.Publish(request.Context(), NewSessionDeletedEvent(sessionID, SessionDeletedEventData{
						User:      userRecord,
						SessionID: &sessionID,
					}))
				}
			}
		}
	}

	core.ClearSessionCookie(responseWriter, request)

	handler.writeJSON(responseWriter, map[string]bool{"ok": true})
}
