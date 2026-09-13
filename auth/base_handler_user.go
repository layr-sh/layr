package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"layr.sh/core"
)

func sanitizeUserProperties(properties map[string]any) map[string]any {
	cleanedProperties := make(map[string]any, len(properties))
	for propertyKey, propertyValue := range properties {
		cleanedProperties[propertyKey] = propertyValue
	}
	return cleanedProperties
}

func (handler *BaseHandler) handleGetUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling get user profile request")
	userID, err := handler.authenticateUser(request)
	if err != nil {
		log.Debugf("get user profile rejected: unauthenticated caller: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	if handler.db == nil {
		log.Debug("get user profile rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	err = handler.db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE id = $1
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("get user profile failed: user %s not found: %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	isMFAEnabled := userRecord.MFAEnabled
	if !isMFAEnabled {
		var hasPasskey bool
		err = handler.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM auth.passkeys WHERE user_id = $1)", userID).Scan(&hasPasskey)
		if err == nil && hasPasskey {
			isMFAEnabled = true
		}
	}

	userResponse := UserResponse{
		ID:            userRecord.ID,
		Email:         userRecord.Email,
		Phone:         userRecord.Phone,
		Role:          userRecord.Role,
		IsAnonymous:   userRecord.IsAnonymous,
		EmailVerified: userRecord.EmailVerifiedAt != nil,
		PhoneVerified: userRecord.PhoneVerifiedAt != nil,
		MFAEnabled:    isMFAEnabled,
		Properties:    userRecord.Properties,
		CreatedAt:     userRecord.CreatedAt,
		LastUpdatedAt: userRecord.LastUpdatedAt,
	}

	log.Debugf("user profile successfully retrieved for %s", userID)
	handler.writeJSON(responseWriter, userResponse)
}

func (handler *BaseHandler) handleUpdateUserProperties(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling update user properties request")
	userID, err := handler.authenticateUser(request)
	if err != nil {
		log.Debugf("update user properties rejected: unauthenticated caller: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	var updateUpdateUserPropertiesRequest UpdateUserPropertiesRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&updateUpdateUserPropertiesRequest); decodeErr != nil {
		log.Debugf("update user properties rejected: invalid JSON payload: %v", decodeErr)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload", "LAYR_AUTH_001")
		return
	}

	if handler.db == nil {
		log.Debug("update user properties rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	cleanedInputProperties := sanitizeUserProperties(updateUpdateUserPropertiesRequest.Properties)
	propertiesJSON, _ := json.Marshal(cleanedInputProperties)

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	err = handler.db.QueryRow(ctx, `
		UPDATE auth.users
		SET properties = COALESCE(properties, '{}'::jsonb) || $1::jsonb,
		    last_updated_at = clock_timestamp()
		WHERE id = $2
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`, propertiesJSON, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("failed to update user properties for %s: %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to update user properties", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewUserUpdatedEvent(userRecord.ID, UserUpdatedEventData(userRecord)))
	}

	log.Debugf("user properties successfully updated for %s", userID)
	handler.writeJSON(responseWriter, UpdateUserPropertiesResponse{
		Properties: userRecord.Properties,
	})
}

func (handler *BaseHandler) handleDeleteUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling delete user account request")
	userID, err := handler.authenticateUser(request)
	if err != nil {
		log.Debugf("delete user account rejected: unauthenticated caller: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	if handler.db == nil {
		log.Debug("delete user account rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()

	var userRecord UserRecord
	var rawProperties []byte
	_ = handler.db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at 
		FROM auth.users WHERE id = $1
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	sessionRows, err := handler.db.Query(ctx, "SELECT refresh_token_hash FROM auth.sessions WHERE user_id = $1", userID)
	if err == nil {
		for sessionRows.Next() {
			var refreshTokenHash string
			if scanErr := sessionRows.Scan(&refreshTokenHash); scanErr == nil && handler.kvStore != nil {
				_ = handler.kvStore.Delete(ctx, "auth:session:"+refreshTokenHash)
			}
		}
		sessionRows.Close()
	}

	_, err = handler.db.Exec(ctx, "DELETE FROM auth.users WHERE id = $1", userID)
	if err != nil {
		log.Debugf("failed to delete user account %s: %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to delete user account", "LAYR_AUTH_001")
		return
	}

	if userRecord.Email != nil && *userRecord.Email != "" {
		_, _ = handler.db.Exec(ctx, "DELETE FROM auth.otps WHERE recipient = $1", *userRecord.Email)
	}
	if userRecord.Phone != nil && *userRecord.Phone != "" {
		_, _ = handler.db.Exec(ctx, "DELETE FROM auth.otps WHERE recipient = $1", *userRecord.Phone)
	}

	isSecure := core.IsSecureRequest(request)
	core.ClearSessionCookie(responseWriter, AuthSessionCookieName, AuthSessionInsecureCookieName, isSecure)

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewUserDeletedEvent(userRecord.ID, UserDeletedEventData(userRecord)))
	}

	log.Debugf("user account %s deleted successfully", userID)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func fetchUserRecordByID(ctx context.Context, db *core.DatabasePool, userID string) (UserRecord, error) {
	if db == nil {
		return UserRecord{ID: userID}, errors.New("database pool unavailable")
	}
	var userRecord UserRecord
	var rawProperties []byte
	err := db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE id = $1
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		return UserRecord{ID: userID}, err
	}
	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}
	return userRecord, nil
}

func fetchUserRecordByRecipient(ctx context.Context, db *core.DatabasePool, recipient string) (UserRecord, error) {
	if db == nil {
		return UserRecord{}, errors.New("database pool unavailable")
	}
	var userRecord UserRecord
	var rawProperties []byte
	err := db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE email = $1 OR phone = $1
	`, recipient).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		return UserRecord{}, err
	}
	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}
	return userRecord, nil
}
