// Package console provides the administrative control plane, operator identity, and dynamic configuration engine.
package console

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"layr.sh/core"
)

const minimumPasswordLength = 8

// handleListConsoleUsers handles GET /v1/_/console/users.
func (controlPlaneHandler *ControlPlaneHandler) handleListConsoleUsers(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list console users request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeConsoleUserRead) {
		return
	}

	const selectUsersSQL = `
		SELECT id, email, encrypted_mfa_secret, is_enabled, last_login_at, created_at, last_updated_at
		FROM console.users
		ORDER BY created_at ASC
	`
	rows, err := controlPlaneHandler.kernel.DB().Query(request.Context(), selectUsersSQL)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	listConsoleUsersResponse := make(ListConsoleUsersResponse, 0)
	for rows.Next() {
		var user User
		var encryptedMFASecret []byte
		_ = rows.Scan(
			&user.ID,
			&user.Email,
			&encryptedMFASecret,
			&user.IsEnabled,
			&user.LastLoginAt,
			&user.CreatedAt,
			&user.LastUpdatedAt,
		)
		user.MFAEnabled = len(encryptedMFASecret) > 0
		listConsoleUsersResponse = append(listConsoleUsersResponse, user)
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, listConsoleUsersResponse)
}

// handleCreateConsoleUser handles POST /v1/_/console/users.
func (controlPlaneHandler *ControlPlaneHandler) handleCreateConsoleUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling create console user request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeConsoleUserWrite) {
		return
	}

	var createConsoleUserInput CreateConsoleUserInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createConsoleUserInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	email := strings.ToLower(strings.TrimSpace(createConsoleUserInput.Email))
	if email == "" || !strings.Contains(email, "@") {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Valid email is required")
		return
	}
	if len(createConsoleUserInput.Password) < minimumPasswordLength {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}

	passwordHash, _ := controlPlaneHandler.hasher.Hash(createConsoleUserInput.Password)

	const insertUserSQL = `
		INSERT INTO console.users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, is_enabled, created_at, last_updated_at
	`
	var user User
	err := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), insertUserSQL, email, passwordHash).Scan(
		&user.ID,
		&user.Email,
		&user.IsEnabled,
		&user.CreatedAt,
		&user.LastUpdatedAt,
	)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" {
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Console user already exists")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	if controlPlaneHandler.kernel != nil && controlPlaneHandler.kernel.EventBus() != nil {
		controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewUserCreatedEvent(user.ID, UserCreatedEventData(user)))
	}

	core.WriteJSONResponse(responseWriter, http.StatusCreated, user)
}

// handleGetConsoleUser handles GET /v1/_/console/users/{user_id}.
func (controlPlaneHandler *ControlPlaneHandler) handleGetConsoleUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get console user request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeConsoleUserRead) {
		return
	}

	userID := request.PathValue("user_id")
	if _, err := uuid.Parse(userID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid user UUID")
		return
	}

	const selectUserSQL = `
		SELECT id, email, encrypted_mfa_secret, is_enabled, last_login_at, created_at, last_updated_at
		FROM console.users
		WHERE id = $1
	`
	var user User
	var encryptedMFASecret []byte
	err := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), selectUserSQL, userID).Scan(
		&user.ID,
		&user.Email,
		&encryptedMFASecret,
		&user.IsEnabled,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Console user not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	user.MFAEnabled = len(encryptedMFASecret) > 0

	core.WriteJSONResponse(responseWriter, http.StatusOK, user)
}

// handleUpdateConsoleUser handles PUT /v1/_/console/users/{user_id}.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateConsoleUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling update console user request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeConsoleUserWrite) {
		return
	}

	userID := request.PathValue("user_id")
	if _, err := uuid.Parse(userID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid user UUID")
		return
	}

	var updateConsoleUserInput UpdateConsoleUserInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&updateConsoleUserInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if updateConsoleUserInput.Email != nil {
		trimmed := strings.ToLower(strings.TrimSpace(*updateConsoleUserInput.Email))
		if trimmed == "" || !strings.Contains(trimmed, "@") {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Valid email is required")
			return
		}
	}

	var newEmail *string
	if updateConsoleUserInput.Email != nil {
		trimmed := strings.ToLower(strings.TrimSpace(*updateConsoleUserInput.Email))
		newEmail = &trimmed
	}

	const updateSQL = `
		UPDATE console.users
		SET
			email = COALESCE($1, email),
			is_enabled = COALESCE($2, is_enabled),
			last_updated_at = clock_timestamp()
		WHERE id = $3
		RETURNING id, email, encrypted_mfa_secret, is_enabled, last_login_at, created_at, last_updated_at
	`
	var updatedUser User
	var encryptedMFASecret []byte
	err := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), updateSQL, newEmail, updateConsoleUserInput.IsEnabled, userID).Scan(
		&updatedUser.ID,
		&updatedUser.Email,
		&encryptedMFASecret,
		&updatedUser.IsEnabled,
		&updatedUser.LastLoginAt,
		&updatedUser.CreatedAt,
		&updatedUser.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Console user not found")
			return
		}
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" {
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Email already in use")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	updatedUser.MFAEnabled = len(encryptedMFASecret) > 0

	if !updatedUser.IsEnabled {
		const deleteSessionsSQL = `DELETE FROM console.sessions WHERE user_id = $1`
		_, _ = controlPlaneHandler.kernel.DB().Exec(request.Context(), deleteSessionsSQL, userID)
	}

	if controlPlaneHandler.kernel != nil && controlPlaneHandler.kernel.EventBus() != nil {
		controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewUserUpdatedEvent(updatedUser.ID, UserUpdatedEventData(updatedUser)))
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, updatedUser)
}

// handleDeleteConsoleUser handles DELETE /v1/_/console/users/{user_id}.
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteConsoleUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling delete console user request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeConsoleUserWrite) {
		return
	}

	userID := request.PathValue("user_id")
	if _, err := uuid.Parse(userID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid user UUID")
		return
	}

	var sessionCount int
	const countSessionsSQL = `SELECT COUNT(*) FROM console.sessions WHERE user_id = $1`
	_ = controlPlaneHandler.kernel.DB().QueryRow(request.Context(), countSessionsSQL, userID).Scan(&sessionCount)

	const deleteSQL = `
		DELETE FROM console.users
		WHERE id = $1
		RETURNING id, email, is_enabled
	`
	var existingUser User
	err := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), deleteSQL, userID).Scan(
		&existingUser.ID,
		&existingUser.Email,
		&existingUser.IsEnabled,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Console user not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	if controlPlaneHandler.kernel != nil && controlPlaneHandler.kernel.EventBus() != nil {
		controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewUserDeletedEvent(userID, UserDeletedEventData{
			User:                 existingUser,
			RevokedSessionsCount: sessionCount,
		}))
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

// handleResetConsoleUserPassword handles POST /v1/_/console/users/{user_id}/password.
func (controlPlaneHandler *ControlPlaneHandler) handleResetConsoleUserPassword(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling reset console user password request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeConsoleUserWrite) {
		return
	}

	userID := request.PathValue("user_id")
	if _, err := uuid.Parse(userID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid user UUID")
		return
	}

	var resetConsoleUserPasswordInput ResetConsoleUserPasswordInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&resetConsoleUserPasswordInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if len(resetConsoleUserPasswordInput.NewPassword) < minimumPasswordLength {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}

	newHash, _ := controlPlaneHandler.hasher.Hash(resetConsoleUserPasswordInput.NewPassword)

	const updatePasswordSQL = `
		UPDATE console.users
		SET password_hash = $1, last_updated_at = clock_timestamp()
		WHERE id = $2
	`
	result, err := controlPlaneHandler.kernel.DB().Exec(request.Context(), updatePasswordSQL, newHash, userID)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	if result.RowsAffected() == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Console user not found")
		return
	}

	const deleteSessionsSQL = `DELETE FROM console.sessions WHERE user_id = $1`
	_, _ = controlPlaneHandler.kernel.DB().Exec(request.Context(), deleteSessionsSQL, userID)

	if controlPlaneHandler.kernel != nil && controlPlaneHandler.kernel.EventBus() != nil {
		controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewUserPasswordResetEvent(userID, UserPasswordResetEventData{
			User: User{ID: userID},
		}))
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, core.Empty{})
}

// handleListConsoleUserSessions handles GET /v1/_/console/users/{user_id}/sessions.
func (controlPlaneHandler *ControlPlaneHandler) handleListConsoleUserSessions(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list console user sessions request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeConsoleUserRead) {
		return
	}

	userID := request.PathValue("user_id")
	if _, err := uuid.Parse(userID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid user UUID")
		return
	}

	const selectSessionsSQL = `
		SELECT id, user_id, host(ip_address), COALESCE(user_agent, ''), expires_at, created_at
		FROM console.sessions
		WHERE user_id = $1 AND expires_at > clock_timestamp()
		ORDER BY created_at DESC
	`
	rows, err := controlPlaneHandler.kernel.DB().Query(request.Context(), selectSessionsSQL, userID)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	listConsoleUserSessionsResponse := make(ListConsoleUserSessionsResponse, 0)
	for rows.Next() {
		var session Session
		_ = rows.Scan(
			&session.ID,
			&session.UserID,
			&session.IPAddress,
			&session.UserAgent,
			&session.ExpiresAt,
			&session.CreatedAt,
		)
		listConsoleUserSessionsResponse = append(listConsoleUserSessionsResponse, session)
	}

	core.WriteJSONResponse(responseWriter, http.StatusOK, listConsoleUserSessionsResponse)
}

// handleDeleteConsoleUserSession handles DELETE /v1/_/console/users/{user_id}/sessions/{session_id}.
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteConsoleUserSession(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling delete console user session request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeConsoleUserWrite) {
		return
	}

	userID := request.PathValue("user_id")
	sessionID := request.PathValue("session_id")
	if _, err := uuid.Parse(userID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid user UUID")
		return
	}
	if _, err := uuid.Parse(sessionID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid session UUID")
		return
	}

	const deleteSQL = `DELETE FROM console.sessions WHERE id = $1 AND user_id = $2`
	result, err := controlPlaneHandler.kernel.DB().Exec(request.Context(), deleteSQL, sessionID, userID)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	if result.RowsAffected() == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Session not found")
		return
	}

	if controlPlaneHandler.kernel != nil && controlPlaneHandler.kernel.EventBus() != nil {
		controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewSessionRevokedEvent(sessionID, SessionRevokedEventData{
			SessionID: sessionID,
			UserID:    userID,
			Reason:    "manual_revocation",
		}))
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

// handleRevokeConsoleUserSessions handles POST /v1/_/console/users/{user_id}/sessions/revoke.
func (controlPlaneHandler *ControlPlaneHandler) handleRevokeConsoleUserSessions(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling revoke all console user sessions request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeConsoleUserWrite) {
		return
	}

	userID := request.PathValue("user_id")
	if _, err := uuid.Parse(userID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid user UUID")
		return
	}

	const deleteSQL = `DELETE FROM console.sessions WHERE user_id = $1`
	_, err := controlPlaneHandler.kernel.DB().Exec(request.Context(), deleteSQL, userID)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	if controlPlaneHandler.kernel != nil && controlPlaneHandler.kernel.EventBus() != nil {
		controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewSessionRevokedEvent(userID, SessionRevokedEventData{
			SessionID: "*",
			UserID:    userID,
			Reason:    "bulk_revocation",
		}))
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}
