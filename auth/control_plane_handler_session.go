package auth

import (
	"net/http"
	"uuid"
)

// HandleListUserSessions lists active sessions for a user (auth:user.read).
func (controlPlaneHandler *ControlPlaneHandler) HandleListUserSessions(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("HandleListUserSessions invoked")

	if !controlPlaneHandler.checkScope(request, "auth:user.read") {
		log.Debug("HandleListUserSessions rejected: missing auth:user.read scope")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:user.read required", "LAYR_AUTH_001")
		return
	}

	userID := controlPlaneHandler.extractUserID(request)
	if _, err := uuid.Parse(userID); err != nil {
		log.Debugf("HandleListUserSessions rejected: invalid UUID %q: %v", userID, err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid user UUID", "LAYR_AUTH_001")
		return
	}

	if controlPlaneHandler.db == nil {
		log.Debug("HandleListUserSessions rejected: database unavailable")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	rows, err := controlPlaneHandler.db.Query(ctx, `
		SELECT id, user_id, refresh_token_hash, ip_address::text, user_agent, expires_at, created_at
		FROM auth.sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		log.Debugf("HandleListUserSessions query failed: %v", err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Failed to query sessions", "LAYR_AUTH_001")
		return
	}
	defer rows.Close()

	sessionRecords := make([]SessionRecord, 0)
	for rows.Next() {
		var sessionRecord SessionRecord
		_ = rows.Scan(
			&sessionRecord.ID, &sessionRecord.UserID, &sessionRecord.RefreshTokenHash,
			&sessionRecord.IPAddress, &sessionRecord.UserAgent, &sessionRecord.ExpiresAt, &sessionRecord.CreatedAt,
		)
		sessionRecords = append(sessionRecords, sessionRecord)
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, map[string]any{
		"sessions": sessionRecords,
		"count":    len(sessionRecords),
	})
}

// HandleRevokeUserSessions terminates all sessions for a user (auth:user.write).
func (controlPlaneHandler *ControlPlaneHandler) HandleRevokeUserSessions(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("HandleRevokeUserSessions invoked")

	if !controlPlaneHandler.checkScope(request, "auth:user.write") {
		log.Debug("HandleRevokeUserSessions rejected: missing auth:user.write scope")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:user.write required", "LAYR_AUTH_001")
		return
	}

	userID := controlPlaneHandler.extractUserID(request)
	if _, err := uuid.Parse(userID); err != nil {
		log.Debugf("HandleRevokeUserSessions rejected: invalid UUID %q: %v", userID, err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid user UUID", "LAYR_AUTH_001")
		return
	}

	if controlPlaneHandler.db == nil {
		log.Debug("HandleRevokeUserSessions rejected: database unavailable")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	deletedSessionRows, deleteErr := controlPlaneHandler.db.Query(ctx, "DELETE FROM auth.sessions WHERE user_id = $1 RETURNING refresh_token_hash", userID)
	if deleteErr != nil {
		log.Debugf("HandleRevokeUserSessions delete failed: %v", deleteErr)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Failed to revoke sessions", "LAYR_AUTH_001")
		return
	}
	revokedCount := 0
	for deletedSessionRows.Next() {
		revokedCount++
		var refreshTokenHash string
		if scanErr := deletedSessionRows.Scan(&refreshTokenHash); scanErr == nil && controlPlaneHandler.kvStore != nil && refreshTokenHash != "" {
			_ = controlPlaneHandler.kvStore.Delete(ctx, "auth:session:"+refreshTokenHash)
		}
	}
	deletedSessionRows.Close()

	if controlPlaneHandler.eventBus != nil {
		controlPlaneHandler.eventBus.Publish(ctx, NewSessionDeletedEvent(userID, SessionDeletedEventData{
			UserID:       userID,
			RevokedCount: &revokedCount,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, map[string]bool{"ok": true})
}
