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
)

// HandleListUsers lists registered application users with filtering and pagination (auth:user.read).
func (controlPlaneHandler *ControlPlaneHandler) HandleListUsers(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("HandleListUsers invoked")

	if !controlPlaneHandler.checkScope(request, "auth:user.read") {
		log.Debug("HandleListUsers rejected: missing auth:user.read scope")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:user.read required", "LAYR_AUTH_001")
		return
	}
	if controlPlaneHandler.db == nil {
		log.Debug("HandleListUsers rejected: database unavailable")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
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

	rows, err = controlPlaneHandler.db.Query(ctx, baseQuery, arguments...)
	if err != nil {
		log.Debugf("HandleListUsers query failed: %v", err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Failed to query users", "LAYR_AUTH_001")
		return
	}
	defer rows.Close()

	userRecords := make([]UserRecord, 0)
	for rows.Next() {
		var userRecord UserRecord
		var rawProperties []byte
		_ = rows.Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
			&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
			&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
		)
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
		userRecords = append(userRecords, userRecord)
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, map[string]any{
		"users":  userRecords,
		"limit":  limit,
		"offset": offset,
		"count":  len(userRecords),
	})
}

// HandleCreateUser creates an application user via control plane (auth:user.write).
func (controlPlaneHandler *ControlPlaneHandler) HandleCreateUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("HandleCreateUser invoked")

	if !controlPlaneHandler.checkScope(request, "auth:user.write") {
		log.Debug("HandleCreateUser rejected: missing auth:user.write scope")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:user.write required", "LAYR_AUTH_001")
		return
	}

	var userCreateRequest UserCreateRequest
	if err := json.NewDecoder(request.Body).Decode(&userCreateRequest); err != nil {
		log.Debugf("HandleCreateUser rejected: invalid JSON body: %v", err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", "LAYR_AUTH_001")
		return
	}

	userCreateRequest.Email = strings.TrimSpace(strings.ToLower(userCreateRequest.Email))
	userCreateRequest.Phone = strings.TrimSpace(userCreateRequest.Phone)

	if userCreateRequest.Phone != "" {
		normalizedPhone, err := NormalizePhone(userCreateRequest.Phone)
		if err != nil {
			log.Debugf("HandleCreateUser rejected: invalid phone %q: %v", userCreateRequest.Phone, err)
			controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code", "LAYR_AUTH_INVALID_PHONE")
			return
		}
		userCreateRequest.Phone = normalizedPhone
	}

	if userCreateRequest.Email == "" && userCreateRequest.Phone == "" {
		log.Debug("HandleCreateUser rejected: email or phone required")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Email or phone number is required", "LAYR_AUTH_001")
		return
	}

	if controlPlaneHandler.db == nil {
		log.Debug("HandleCreateUser rejected: database unavailable")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	var passwordHash *string
	if userCreateRequest.Password != "" && controlPlaneHandler.hasher != nil {
		hash, _ := controlPlaneHandler.hasher.Hash(userCreateRequest.Password)
		passwordHash = &hash
	}

	role := "authenticated"
	if userCreateRequest.Role != "" {
		role = userCreateRequest.Role
	}

	var emailVerifiedAt, phoneVerifiedAt *time.Time
	now := time.Now().UTC()
	if userCreateRequest.EmailVerified {
		emailVerifiedAt = &now
	}
	if userCreateRequest.PhoneVerified {
		phoneVerifiedAt = &now
	}

	properties := userCreateRequest.Properties
	if properties == nil {
		properties = make(map[string]any)
	}
	propertiesJSON, _ := json.Marshal(properties)

	var emailPointer, phonePointer *string
	if userCreateRequest.Email != "" {
		emailPointer = &userCreateRequest.Email
	}
	if userCreateRequest.Phone != "" {
		phonePointer = &userCreateRequest.Phone
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	query := `
		INSERT INTO auth.users (email, phone, password_hash, role, email_verified_at, phone_verified_at, properties, created_at, last_updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, clock_timestamp(), clock_timestamp())
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`
	err := controlPlaneHandler.db.QueryRow(ctx, query, emailPointer, phonePointer, passwordHash, role, emailVerifiedAt, phoneVerifiedAt, propertiesJSON).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("HandleCreateUser insert failed: %v", err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Failed to create user", "LAYR_AUTH_001")
		return
	}
	_ = json.Unmarshal(rawProperties, &userRecord.Properties)

	if controlPlaneHandler.eventBus != nil {
		controlPlaneHandler.eventBus.Publish(ctx, NewUserCreatedEvent(userRecord.ID, UserCreatedEventData(userRecord)))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusCreated, userRecord)
}

// HandleGetUser retrieves a specific user by UUID (auth:user.read).
func (controlPlaneHandler *ControlPlaneHandler) HandleGetUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("HandleGetUser invoked")

	if !controlPlaneHandler.checkScope(request, "auth:user.read") {
		log.Debug("HandleGetUser rejected: missing auth:user.read scope")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:user.read required", "LAYR_AUTH_001")
		return
	}

	userID := controlPlaneHandler.extractUserID(request)
	if _, err := uuid.Parse(userID); err != nil {
		log.Debugf("HandleGetUser rejected: invalid UUID %q: %v", userID, err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid user UUID", "LAYR_AUTH_001")
		return
	}

	if controlPlaneHandler.db == nil {
		log.Debug("HandleGetUser rejected: database unavailable")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	query := `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE id = $1
	`
	err := controlPlaneHandler.db.QueryRow(ctx, query, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Debugf("HandleGetUser: user %s not found", userID)
			controlPlaneHandler.writeError(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_001")
			return
		}
		log.Debugf("HandleGetUser query failed: %v", err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Failed to query user", "LAYR_AUTH_001")
		return
	}
	_ = json.Unmarshal(rawProperties, &userRecord.Properties)

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, userRecord)
}

// HandleDeleteUser deletes a user and cascades sessions, passkeys, identities (auth:user.write).
func (controlPlaneHandler *ControlPlaneHandler) HandleDeleteUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("HandleDeleteUser invoked")

	if !controlPlaneHandler.checkScope(request, "auth:user.write") {
		log.Debug("HandleDeleteUser rejected: missing auth:user.write scope")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:user.write required", "LAYR_AUTH_001")
		return
	}

	userID := controlPlaneHandler.extractUserID(request)
	if _, err := uuid.Parse(userID); err != nil {
		log.Debugf("HandleDeleteUser rejected: invalid UUID %q: %v", userID, err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid user UUID", "LAYR_AUTH_001")
		return
	}

	if controlPlaneHandler.db == nil {
		log.Debug("HandleDeleteUser rejected: database unavailable")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	err := controlPlaneHandler.db.QueryRow(ctx, `
		DELETE FROM auth.users
		WHERE id = $1
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Debugf("HandleDeleteUser: user %s not found", userID)
			controlPlaneHandler.writeError(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_001")
			return
		}
		log.Debugf("HandleDeleteUser exec failed: %v", err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Failed to delete user", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if controlPlaneHandler.eventBus != nil {
		controlPlaneHandler.eventBus.Publish(ctx, NewUserDeletedEvent(userRecord.ID, UserDeletedEventData(userRecord)))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, map[string]bool{"ok": true})
}

// HandleLockUser locks a user account and immediately invalidates all active sessions (auth:user.write).
func (controlPlaneHandler *ControlPlaneHandler) HandleLockUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("HandleLockUser invoked")

	if !controlPlaneHandler.checkScope(request, "auth:user.write") {
		log.Debug("HandleLockUser rejected: missing auth:user.write scope")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:user.write required", "LAYR_AUTH_001")
		return
	}

	userID := controlPlaneHandler.extractUserID(request)
	if _, err := uuid.Parse(userID); err != nil {
		log.Debugf("HandleLockUser rejected: invalid UUID %q: %v", userID, err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid user UUID", "LAYR_AUTH_001")
		return
	}

	if controlPlaneHandler.db == nil {
		log.Debug("HandleLockUser rejected: database unavailable")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	var userLockRequest UserLockRequest
	_ = json.NewDecoder(request.Body).Decode(&userLockRequest)

	lockedUntil := time.Now().UTC().AddDate(100, 0, 0)
	if userLockRequest.LockedUntil != nil {
		lockedUntil = userLockRequest.LockedUntil.UTC()
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	query := `
		UPDATE auth.users
		SET locked_until = $1, last_updated_at = clock_timestamp()
		WHERE id = $2
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`
	err := controlPlaneHandler.db.QueryRow(ctx, query, lockedUntil, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Debugf("HandleLockUser: user %s not found", userID)
			controlPlaneHandler.writeError(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_001")
			return
		}
		log.Debugf("HandleLockUser query failed: %v", err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Failed to lock user", "LAYR_AUTH_001")
		return
	}
	_ = json.Unmarshal(rawProperties, &userRecord.Properties)

	// Revoke active sessions and invalidate cache
	if deletedSessionRows, deleteErr := controlPlaneHandler.db.Query(ctx, "DELETE FROM auth.sessions WHERE user_id = $1 RETURNING id, client_id, refresh_token_hash", userID); deleteErr == nil {
		targetSessions := make([]ClientSessionInfo, 0)
		for deletedSessionRows.Next() {
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
	}

	if controlPlaneHandler.eventBus != nil {
		controlPlaneHandler.eventBus.Publish(ctx, NewUserLockedEvent(userID, UserLockedEventData{
			User:        userRecord,
			LockedUntil: &lockedUntil,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, userRecord)
}

// HandleUnlockUser lifts lock restrictions on a user account (auth:user.write).
func (controlPlaneHandler *ControlPlaneHandler) HandleUnlockUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("HandleUnlockUser invoked")

	if !controlPlaneHandler.checkScope(request, "auth:user.write") {
		log.Debug("HandleUnlockUser rejected: missing auth:user.write scope")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:user.write required", "LAYR_AUTH_001")
		return
	}

	userID := controlPlaneHandler.extractUserID(request)
	if _, err := uuid.Parse(userID); err != nil {
		log.Debugf("HandleUnlockUser rejected: invalid UUID %q: %v", userID, err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid user UUID", "LAYR_AUTH_001")
		return
	}

	if controlPlaneHandler.db == nil {
		log.Debug("HandleUnlockUser rejected: database unavailable")
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	query := `
		UPDATE auth.users
		SET locked_until = NULL, last_updated_at = clock_timestamp()
		WHERE id = $1
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`
	err := controlPlaneHandler.db.QueryRow(ctx, query, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Debugf("HandleUnlockUser: user %s not found", userID)
			controlPlaneHandler.writeError(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_001")
			return
		}
		log.Debugf("HandleUnlockUser query failed: %v", err)
		controlPlaneHandler.writeError(responseWriter, request, http.StatusInternalServerError, "Failed to unlock user", "LAYR_AUTH_001")
		return
	}
	_ = json.Unmarshal(rawProperties, &userRecord.Properties)

	if controlPlaneHandler.eventBus != nil {
		controlPlaneHandler.eventBus.Publish(ctx, NewUserUnlockedEvent(userID, UserUnlockedEventData(userRecord)))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, userRecord)
}
