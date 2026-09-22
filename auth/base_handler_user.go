package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"layr.sh/core"
)

func (handler *BaseHandler) handleGetUser(responseWriter http.ResponseWriter, request *http.Request) {
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := authContext.UserID

	ctx := request.Context()
	var user User
	var rawProperties []byte
	err := handler.kernel.DB().QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found")
		return
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	getUserResponse := GetUserResponse{
		ID:            user.ID,
		Email:         user.Email,
		Phone:         user.Phone,
		Role:          user.Role,
		IsAnonymous:   user.IsAnonymous,
		EmailVerified: user.EmailVerifiedAt != nil,
		PhoneVerified: user.PhoneVerifiedAt != nil,
		MFAEnabled:    user.MFAEnabled,
		Properties:    user.Properties,
		CreatedAt:     user.CreatedAt,
		LastUpdatedAt: user.LastUpdatedAt,
	}

	log.Debugf("user profile successfully retrieved for %s", userID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, getUserResponse)
}

func (handler *BaseHandler) handleUpdateUserProperties(responseWriter http.ResponseWriter, request *http.Request) {
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := authContext.UserID

	var updateUserPropertiesInput UpdateUserPropertiesInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&updateUserPropertiesInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	propertiesJSON, _ := json.Marshal(updateUserPropertiesInput.Properties)

	ctx := request.Context()
	var user User
	var rawProperties []byte
	err := handler.kernel.DB().QueryRow(ctx, `
		UPDATE auth.users
		SET properties = COALESCE(properties, '{}'::jsonb) || $1::jsonb,
		    last_updated_at = clock_timestamp()
		WHERE id = $2
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`, propertiesJSON, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to update user properties for %s: %v", userID, err))
		return
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	handler.kernel.EventBus().Publish(ctx, NewUserUpdatedEvent(user.ID, UserUpdatedEventData(user)))

	log.Debugf("user properties successfully updated for %s", userID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, UpdateUserPropertiesResponse{
		Properties: user.Properties,
	})
}

func (handler *BaseHandler) handleDeleteUser(responseWriter http.ResponseWriter, request *http.Request) {
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := authContext.UserID

	ctx := request.Context()

	var user User
	var rawProperties []byte
	_ = handler.kernel.DB().QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at 
		FROM auth.users WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	sessionRows, err := handler.kernel.DB().Query(ctx, "SELECT refresh_token_hash FROM auth.sessions WHERE user_id = $1", userID)
	if err == nil {
		for sessionRows.Next() {
			var refreshTokenHash string
			if scanErr := sessionRows.Scan(&refreshTokenHash); scanErr == nil {
				_ = handler.kernel.KVStore().Delete(ctx, "auth:session:"+refreshTokenHash)
			}
		}
		sessionRows.Close()
	}

	_, err = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.users WHERE id = $1", userID)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to delete user account %s: %v", userID, err))
		return
	}

	if user.Email != nil && *user.Email != "" {
		_, _ = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.otps WHERE recipient = $1", *user.Email)
	}
	if user.Phone != nil && *user.Phone != "" {
		_, _ = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.otps WHERE recipient = $1", *user.Phone)
	}

	core.ClearSessionCookie(responseWriter, request)

	handler.kernel.EventBus().Publish(ctx, NewUserDeletedEvent(user.ID, UserDeletedEventData(user)))

	log.Debugf("user account %s deleted successfully", userID)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func fetchUserByID(ctx context.Context, db *core.DatabasePool, userID string) (User, error) {
	var user User
	var rawProperties []byte
	err := db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		return User{ID: userID}, err
	}
	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}
	return user, nil
}

func fetchUserByRecipient(ctx context.Context, db *core.DatabasePool, recipient string) (User, error) {
	var user User
	var rawProperties []byte
	err := db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE email = $1 OR phone = $1
	`, recipient).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		return User{}, err
	}
	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}
	return user, nil
}
