package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

// handleListUsers lists registered application users with filtering and pagination (auth:user.read).
func (controlPlaneHandler *ControlPlaneHandler) handleListUsers(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list users request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeAuthUserRead) {
		return
	}

	limit := 50
	if limitParameter := request.URL.Query().Get("limit"); limitParameter != "" {
		if parsedLimit, err := strconv.Atoi(limitParameter); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetParameter := request.URL.Query().Get("offset"); offsetParameter != "" {
		if parsedOffset, err := strconv.Atoi(offsetParameter); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	roleFilter := strings.TrimSpace(request.URL.Query().Get("role"))
	searchQuery := strings.TrimSpace(request.URL.Query().Get("search"))

	ctx := request.Context()
	var rows pgx.Rows
	var err error

	baseQuery := `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE 1=1
	`
	arguments := []any{}
	argumentIndex := 1

	if roleFilter != "" {
		baseQuery += fmt.Sprintf(" AND role = $%d", argumentIndex)
		arguments = append(arguments, roleFilter)
		argumentIndex++
	}

	if searchQuery != "" {
		baseQuery += fmt.Sprintf(" AND (email ILIKE $%d OR phone ILIKE $%d)", argumentIndex, argumentIndex)
		arguments = append(arguments, "%"+searchQuery+"%")
		argumentIndex++
	}

	baseQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argumentIndex, argumentIndex+1)
	arguments = append(arguments, limit, offset)

	rows, err = controlPlaneHandler.kernel.DB().Query(ctx, baseQuery, arguments...)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var user User
		var rawProperties []byte
		_ = rows.Scan(
			&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
			&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
			&user.EncryptedMFASecret, &user.MFAEnabled,
			&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
		)
		_ = json.Unmarshal(rawProperties, &user.Properties)
		users = append(users, user)
	}

	log.Debugf("retrieved %d user(s)", len(users))
	core.WriteJSONResponse(responseWriter, http.StatusOK, ListUsersResponse{
		Users:  users,
		Limit:  limit,
		Offset: offset,
		Count:  len(users),
	})
}

// handleCreateUser creates an application user via control plane (auth:user.write).
func (controlPlaneHandler *ControlPlaneHandler) handleCreateUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling create user request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeAuthUserWrite) {
		return
	}

	var createUserInput CreateUserInput
	if err := json.NewDecoder(request.Body).Decode(&createUserInput); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	createUserInput.Email = strings.TrimSpace(strings.ToLower(createUserInput.Email))
	createUserInput.Phone = strings.TrimSpace(createUserInput.Phone)

	if createUserInput.Phone != "" {
		normalizedPhone, err := NormalizePhone(createUserInput.Phone)
		if err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code")
			return
		}
		createUserInput.Phone = normalizedPhone
	}

	if createUserInput.Email == "" && createUserInput.Phone == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Email or phone number is required")
		return
	}

	var passwordHash *string
	if createUserInput.Password != "" {
		hash, _ := controlPlaneHandler.hasher.Hash(createUserInput.Password)
		passwordHash = &hash
	}

	role := "authenticated"
	if createUserInput.Role != "" {
		role = createUserInput.Role
	}

	var emailVerifiedAt, phoneVerifiedAt *time.Time
	now := time.Now().UTC()
	if createUserInput.EmailVerified {
		emailVerifiedAt = &now
	}
	if createUserInput.PhoneVerified {
		phoneVerifiedAt = &now
	}

	properties := createUserInput.Properties
	if properties == nil {
		properties = make(map[string]any)
	}
	propertiesJSON, _ := json.Marshal(properties)

	var emailPointer, phonePointer *string
	if createUserInput.Email != "" {
		emailPointer = &createUserInput.Email
	}
	if createUserInput.Phone != "" {
		phonePointer = &createUserInput.Phone
	}

	ctx := request.Context()
	var user User
	var rawProperties []byte
	query := `
		INSERT INTO auth.users (email, phone, password_hash, role, email_verified_at, phone_verified_at, properties, created_at, last_updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, clock_timestamp(), clock_timestamp())
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`
	err := controlPlaneHandler.kernel.DB().QueryRow(ctx, query, emailPointer, phonePointer, passwordHash, role, emailVerifiedAt, phoneVerifiedAt, propertiesJSON).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	_ = json.Unmarshal(rawProperties, &user.Properties)

	controlPlaneHandler.kernel.EventBus().Publish(ctx, NewUserCreatedEvent(user.ID, UserCreatedEventData(user)))

	log.Debugf("user %s successfully created", user.ID)
	core.WriteJSONResponse(responseWriter, http.StatusCreated, user)
}

// handleGetUser retrieves a specific user by UUID (auth:user.read).
func (controlPlaneHandler *ControlPlaneHandler) handleGetUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get user request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeAuthUserRead) {
		return
	}

	userID := request.PathValue("user_id")
	if _, err := uuid.Parse(userID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid user UUID")
		return
	}

	ctx := request.Context()
	var user User
	var rawProperties []byte
	query := `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE id = $1
	`
	err := controlPlaneHandler.kernel.DB().QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	_ = json.Unmarshal(rawProperties, &user.Properties)

	log.Debugf("retrieved user %s", user.ID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, user)
}

// handleDeleteUser deletes a user and cascades sessions, passkeys, identities (auth:user.write).
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling delete user request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeAuthUserWrite) {
		return
	}

	userID := request.PathValue("user_id")
	if _, err := uuid.Parse(userID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid user UUID")
		return
	}

	ctx := request.Context()
	var user User
	var rawProperties []byte
	err := controlPlaneHandler.kernel.DB().QueryRow(ctx, `
		DELETE FROM auth.users
		WHERE id = $1
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	controlPlaneHandler.kernel.EventBus().Publish(ctx, NewUserDeletedEvent(user.ID, UserDeletedEventData(user)))

	log.Debugf("user %s successfully deleted", userID)
	responseWriter.WriteHeader(http.StatusNoContent)
}

// handleLockUser locks a user account and immediately invalidates all active sessions (auth:user.write).
func (controlPlaneHandler *ControlPlaneHandler) handleLockUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling lock user request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeAuthUserWrite) {
		return
	}

	userID := request.PathValue("user_id")
	if _, err := uuid.Parse(userID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid user UUID")
		return
	}

	var lockUserInput LockUserInput
	_ = json.NewDecoder(request.Body).Decode(&lockUserInput)

	lockedUntil := time.Now().UTC().AddDate(100, 0, 0)
	if lockUserInput.LockedUntil != nil {
		lockedUntil = lockUserInput.LockedUntil.UTC()
	}

	ctx := request.Context()
	var user User
	var rawProperties []byte
	query := `
		UPDATE auth.users
		SET locked_until = $1, last_updated_at = clock_timestamp()
		WHERE id = $2
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`
	err := controlPlaneHandler.kernel.DB().QueryRow(ctx, query, lockedUntil, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	_ = json.Unmarshal(rawProperties, &user.Properties)

	// Revoke active sessions and invalidate cache
	if deletedSessionRows, deleteErr := controlPlaneHandler.kernel.DB().Query(ctx, "DELETE FROM auth.sessions WHERE user_id = $1 RETURNING id, client_id, refresh_token_hash", userID); deleteErr == nil {
		targetSessions := make([]ClientSessionInfo, 0)
		for deletedSessionRows.Next() {
			var targetSessionID string
			var targetClientID *string
			var refreshTokenHash string
			if scanErr := deletedSessionRows.Scan(&targetSessionID, &targetClientID, &refreshTokenHash); scanErr == nil {
				if refreshTokenHash != "" {
					_ = controlPlaneHandler.kernel.KVStore().Delete(ctx, "auth:session:"+refreshTokenHash)
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

		if len(targetSessions) > 0 {
			config := controlPlaneHandler.configManager.Get()
			dispatchBackChannelSignOut(ctx, controlPlaneHandler.httpClient, controlPlaneHandler.kernel.JWTSigner(), config.OIDC.Clients, targetSessions)
		}
	}

	controlPlaneHandler.kernel.EventBus().Publish(ctx, NewUserLockedEvent(userID, UserLockedEventData{
		User:        user,
		LockedUntil: &lockedUntil,
	}))

	log.Debugf("user %s successfully locked", userID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, user)
}

// handleUnlockUser lifts lock restrictions on a user account (auth:user.write).
func (controlPlaneHandler *ControlPlaneHandler) handleUnlockUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling unlock user request")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeAuthUserWrite) {
		return
	}

	userID := request.PathValue("user_id")
	if _, err := uuid.Parse(userID); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid user UUID")
		return
	}

	ctx := request.Context()
	var user User
	var rawProperties []byte
	query := `
		UPDATE auth.users
		SET locked_until = NULL, last_updated_at = clock_timestamp()
		WHERE id = $1
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`
	err := controlPlaneHandler.kernel.DB().QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	_ = json.Unmarshal(rawProperties, &user.Properties)

	controlPlaneHandler.kernel.EventBus().Publish(ctx, NewUserUnlockedEvent(userID, UserUnlockedEventData(user)))

	log.Debugf("user %s successfully unlocked", userID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, user)
}
