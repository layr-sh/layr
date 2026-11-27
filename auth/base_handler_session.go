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
	"layr.sh/auth/threat"
	"layr.sh/core"
)

func (handler *BaseHandler) persistSession(ctx context.Context, session *Session, scope string) error {
	tx, err := handler.kernel.DB().Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var userID string
	if lookupErr := tx.QueryRow(ctx, "SELECT id FROM auth.users WHERE id = $1 AND (locked_until IS NULL OR locked_until <= clock_timestamp()) FOR UPDATE", session.UserID).Scan(&userID); lookupErr != nil {
		return lookupErr
	}
	var revoked []ClientSessionInfo
	var hashes []string
	config := handler.configManager.Get()
	if config.Sessions.SingleSessionPerUser {
		rows, queryErr := tx.Query(ctx, "DELETE FROM auth.sessions WHERE user_id = $1 RETURNING id, client_id, refresh_token_hash", session.UserID)
		if queryErr != nil {
			return queryErr
		}
		deletedSessions, collectErr := pgx.CollectRows(rows, pgx.RowToStructByPos[struct {
			ID               string
			ClientID         *string
			RefreshTokenHash string
		}])
		if collectErr != nil {
			return collectErr
		}
		for _, deletedSession := range deletedSessions {
			hashes = append(hashes, deletedSession.RefreshTokenHash)
			if deletedSession.ClientID != nil {
				revoked = append(revoked, ClientSessionInfo{SessionID: deletedSession.ID, ClientID: *deletedSession.ClientID, UserID: session.UserID})
			}
		}
	}
	if session.ID == "" {
		session.ID = uuid.NewV7().String()
	}
	err = tx.QueryRow(ctx, `INSERT INTO auth.sessions (id, user_id, client_id, refresh_token_hash, ip_address, user_agent, expires_at, scope)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING created_at`, session.ID, session.UserID, session.ClientID, session.RefreshTokenHash, session.IPAddress, session.UserAgent, session.ExpiresAt, scope).Scan(&session.CreatedAt)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	for _, hash := range hashes {
		_ = handler.kernel.KVStore().Delete(ctx, "auth:session:"+hash)
	}
	if len(revoked) > 0 {
		dispatchBackChannelSignOut(ctx, handler.httpClient, handler.kernel.JWTSigner(), config.OIDC.Clients, revoked)
	}
	user, userErr := fetchUserByID(ctx, handler.kernel.DB(), session.UserID)
	if userErr == nil {
		for _, identifier := range []*string{user.Email, user.Phone} {
			if identifier != nil {
				_ = handler.kernel.KVStore().Delete(ctx, "auth:ratelimit:sign_in:"+*identifier)
			}
		}
	}
	_ = threat.ResetFailedAttempts(ctx, handler.kernel.KVStore(), session.UserID)
	if session.IPAddress != nil {
		_ = threat.ResetFailedAttempts(ctx, handler.kernel.KVStore(), *session.IPAddress)
	}
	return nil
}

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
		Data:  sessions,
		Count: len(sessions),
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
		targetSessionID = strings.TrimPrefix(strings.TrimSuffix(request.URL.Path, "/"), "/v1/auth/user/sessions/")
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

		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Current session identifier is required to revoke other devices")
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
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", err.Error())
			return
		}
	}
	if refreshTokenInput.RefreshToken == "" {
		refreshTokenInput.RefreshToken = core.ExtractRequestSessionToken(request)
	}
	if refreshTokenInput.RefreshToken == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Refresh token is required")
		return
	}

	tokenHash := handler.kernel.JWTSigner().HashRefreshToken(refreshTokenInput.RefreshToken)
	ctx := request.Context()

	config := handler.configManager.Get()

	var sessionID, userID string
	var expiresAt, userUpdatedAt time.Time
	var lockedUntil *time.Time

	err := handler.kernel.DB().QueryRow(ctx, `
		DELETE FROM auth.sessions s USING auth.users u
		WHERE s.user_id = u.id AND s.refresh_token_hash = $1 AND s.client_id IS NULL
		AND s.expires_at > clock_timestamp()
		AND ($2 <= 0 OR s.last_used_at > clock_timestamp() - make_interval(secs => $2))
		RETURNING s.id, s.user_id, s.expires_at, u.last_updated_at, u.locked_until
	`, tokenHash, config.Sessions.IdleTimeoutSeconds).Scan(&sessionID, &userID, &expiresAt, &userUpdatedAt, &lockedUntil)

	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Refresh token revoked or invalid")
		return
	}

	if lockedUntil != nil && lockedUntil.After(time.Now().UTC()) {
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	var user User
	if config.Cache.FastPathSessionsEnabled {
		if data, cacheErr := handler.kernel.KVStore().Get(ctx, "auth:session:"+tokenHash); cacheErr == nil {
			var cachedSession CachedSession
			if json.Unmarshal([]byte(data), &cachedSession) == nil && cachedSession.User.ID == userID && cachedSession.User.LastUpdatedAt.Equal(userUpdatedAt) {
				user = cachedSession.User
			}
		}
	}
	if user.ID == "" {
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
	}

	if user.LockedUntil != nil && time.Now().UTC().Before(*user.LockedUntil) {
		_, _ = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.sessions WHERE id = $1", sessionID)
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	// The old token was atomically consumed above.
	_ = handler.kernel.KVStore().Delete(ctx, "auth:session:"+tokenHash)
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

	var tokenSessionID, tokenUserID string
	if strings.Count(refreshTokenInput.RefreshToken, ".") == 2 {
		jwtClaims, err := handler.kernel.JWTSigner().VerifyAccessToken(refreshTokenInput.RefreshToken)
		if err != nil || jwtClaims == nil || jwtClaims.SessionID == "" || jwtClaims.Subject == "" || jwtClaims.Audience == "layr:console:user" || (core.AuthContext{JWT: *jwtClaims}).IsServiceAccount() {
			refreshTokenInput.RefreshToken = ""
		} else {
			tokenSessionID, tokenUserID = jwtClaims.SessionID, jwtClaims.Subject
		}
	}

	if refreshTokenInput.RefreshToken != "" {
		tokenHash := handler.kernel.JWTSigner().HashRefreshToken(refreshTokenInput.RefreshToken)
		var sessionID, userID, refreshTokenHash string
		var clientID *string
		err := handler.kernel.DB().QueryRow(request.Context(), `
			DELETE FROM auth.sessions
			WHERE ($2 = '' AND refresh_token_hash = $1)
			   OR (id::text = $2 AND user_id::text = $3)
			RETURNING id, user_id, client_id, refresh_token_hash
		`, tokenHash, tokenSessionID, tokenUserID).Scan(&sessionID, &userID, &clientID, &refreshTokenHash)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to revoke session", err.Error())
			return
		}
		if refreshTokenHash != "" {
			tokenHash = refreshTokenHash
		}
		if cacheErr := handler.kernel.KVStore().Delete(request.Context(), "auth:session:"+tokenHash); cacheErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to revoke cachedSession session", cacheErr.Error())
			return
		}
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
