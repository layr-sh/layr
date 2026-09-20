package auth

import (
	"net/http"
	"uuid"

	"layr.sh/core"
)

// handleListUserSessions lists active sessions for a user (auth:user.read).
func (controlPlaneHandler *ControlPlaneHandler) handleListUserSessions(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleListUserSessions invoked")

	if !controlPlaneHandler.checkScope(request, "auth:user.read") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:user.read required")
		return
	}

	userID := controlPlaneHandler.extractUserID(request)
	if _, err := uuid.Parse(userID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid user UUID")
		return
	}

	if controlPlaneHandler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable")
		return
	}

	ctx := request.Context()
	rows, err := controlPlaneHandler.db.Query(ctx, `
		SELECT id, user_id, client_id, refresh_token_hash, ip_address::text, user_agent, expires_at, created_at
		FROM auth.sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	sessions := make([]Session, 0)
	for rows.Next() {
		var session Session
		_ = rows.Scan(
			&session.ID, &session.UserID, &session.ClientID, &session.RefreshTokenHash,
			&session.IPAddress, &session.UserAgent, &session.ExpiresAt, &session.CreatedAt,
		)
		sessions = append(sessions, session)
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, ListUserSessionsResponse{
		Sessions: sessions,
		Count:    len(sessions),
	})
}

// handleRevokeUserSessions terminates all sessions for a user (auth:user.write).
func (controlPlaneHandler *ControlPlaneHandler) handleRevokeUserSessions(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleRevokeUserSessions invoked")

	if !controlPlaneHandler.checkScope(request, "auth:user.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:user.write required")
		return
	}

	userID := controlPlaneHandler.extractUserID(request)
	if _, err := uuid.Parse(userID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid user UUID")
		return
	}

	if controlPlaneHandler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable")
		return
	}

	ctx := request.Context()
	deletedSessionRows, deleteErr := controlPlaneHandler.db.Query(ctx, "DELETE FROM auth.sessions WHERE user_id = $1 RETURNING id, client_id, refresh_token_hash", userID)
	if deleteErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, deleteErr.Error())
		return
	}
	targetSessions := make([]ClientSessionInfo, 0)
	revokedCount := 0
	for deletedSessionRows.Next() {
		revokedCount++
		var targetSessionID string
		var targetClientID *string
		var refreshTokenHash string
		if scanErr := deletedSessionRows.Scan(&targetSessionID, &targetClientID, &refreshTokenHash); scanErr == nil {
			if controlPlaneHandler.kvStore != nil && refreshTokenHash != "" {
				_ = controlPlaneHandler.kvStore.Delete(ctx, "auth:session:"+refreshTokenHash)
			}
			if targetClientID != nil && *targetClientID != "" {
				targetSessions = append(targetSessions, ClientSessionInfo{
					ClientID:  *targetClientID,
					SessionID: targetSessionID,
					UserID:    userID,
				})
			}
		}
	}
	deletedSessionRows.Close()

	if len(targetSessions) > 0 && controlPlaneHandler.jwtSigner != nil {
		config := controlPlaneHandler.configManager.Get()
		dispatchBackChannelSignOut(ctx, controlPlaneHandler.httpClient, controlPlaneHandler.jwtSigner, config.OIDC.Clients, targetSessions)
	}

	if controlPlaneHandler.eventBus != nil {
		user, _ := fetchUserByID(ctx, controlPlaneHandler.db, userID)
		controlPlaneHandler.eventBus.Publish(ctx, NewSessionDeletedEvent(userID, SessionDeletedEventData{
			User:         user,
			RevokedCount: &revokedCount,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, map[string]bool{"ok": true})
}
