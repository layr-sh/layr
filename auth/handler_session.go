package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"uuid"

	"layr.sh/core"
)

// UserSessionRecord represents a safe, public view of an active user session.
type UserSessionRecord struct {
	ID        string    `json:"id"`
	IPAddress *string   `json:"ip_address,omitempty"`
	UserAgent *string   `json:"user_agent,omitempty"`
	IsCurrent bool      `json:"is_current"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// ListUserSessionsResponse represents the list of active sessions for the current user.
type ListUserSessionsResponse struct {
	Sessions []UserSessionRecord `json:"sessions"`
	Count    int                 `json:"count"`
}

// RevokeOtherSessionsResponse represents the result of revoking other devices.
type RevokeOtherSessionsResponse struct {
	RevokedCount int64 `json:"revoked_count"`
}

func (handler *Handler) handleListSessions(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list active user sessions request")
	userID, currentRefreshTokenHash, currentSessionID, err := handler.authenticateSessionRequest(request)
	if err != nil {
		log.Debugf("list user sessions rejected: unauthenticated caller: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

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

func (handler *Handler) handleRevokeSession(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling revoke session request")
	userID, _, _, err := handler.authenticateSessionRequest(request)
	if err != nil {
		log.Debugf("revoke session rejected: unauthenticated caller: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

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
	err = handler.db.QueryRow(ctx, `
		DELETE FROM auth.sessions
		WHERE id = $1 AND user_id = $2
		RETURNING refresh_token_hash
	`, targetSessionID, userID).Scan(&deletedRefreshTokenHash)

	if err != nil {
		log.Debugf("revoke session failed: session %s not found for user %s: %v", targetSessionID, userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Session not found", "LAYR_AUTH_001")
		return
	}

	if handler.kvStore != nil && deletedRefreshTokenHash != "" {
		_ = handler.kvStore.Delete(ctx, "auth:session:"+deletedRefreshTokenHash)
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewSessionDeletedEvent(targetSessionID, SessionDeletedEventData{
			SessionID: &targetSessionID,
			UserID:    userID,
		}))
	}

	log.Debugf("session %s successfully revoked for user %s", targetSessionID, userID)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) handleRevokeOtherSessions(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling revoke other sessions request")
	userID, currentRefreshTokenHash, currentSessionID, err := handler.authenticateSessionRequest(request)
	if err != nil {
		log.Debugf("revoke other sessions rejected: unauthenticated caller: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	if handler.db == nil {
		log.Debug("revoke other sessions rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

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
		err = handler.db.QueryRow(ctx, `
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
		RETURNING refresh_token_hash
	`, userID, resolvedCurrentSessionID)
	if err != nil {
		log.Debugf("failed to revoke other sessions for user %s: %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to revoke other sessions", "LAYR_AUTH_001")
		return
	}
	defer rows.Close()

	deletedHashes := make([]string, 0)
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err == nil {
			deletedHashes = append(deletedHashes, hash)
		}
	}

	if handler.kvStore != nil {
		for _, hash := range deletedHashes {
			if hash != "" {
				_ = handler.kvStore.Delete(ctx, "auth:session:"+hash)
			}
		}
	}

	if handler.eventBus != nil {
		revokedCount := len(deletedHashes)
		handler.eventBus.Publish(ctx, NewSessionDeletedEvent(userID, SessionDeletedEventData{
			UserID:       userID,
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
