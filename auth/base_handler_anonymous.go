package auth

import (
	"encoding/json"
	"fmt"
	"net/http"

	"layr.sh/core"
)

func (handler *BaseHandler) handleAnonymousSignIn(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling anonymous sign-in request")
	config := handler.configManager.Get()
	if !config.Anonymous.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "anonymous sign-in rejected: anonymous authentication disabled in configuration")
		return
	}

	var anonymousSignInRequest AnonymousSignInRequest
	if request.Body != nil {
		_ = json.NewDecoder(request.Body).Decode(&anonymousSignInRequest)
	}

	inputProperties := anonymousSignInRequest.Properties
	if inputProperties == nil {
		inputProperties = make(map[string]any)
	}
	propertiesJSON, _ := json.Marshal(inputProperties)

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "anonymous sign-in rejected: database pool unavailable")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	query := `
		INSERT INTO auth.users (role, is_anonymous, properties, created_at, last_updated_at)
		VALUES ('authenticated', true, $1, clock_timestamp(), clock_timestamp())
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`
	err := handler.db.QueryRow(ctx, query, propertiesJSON).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to create anonymous user: %v", err))
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewUserSignedUpEvent(userRecord.ID, UserSignedUpEventData(userRecord)))
	}

	handler.issueSessionResponse(responseWriter, request, userRecord, "anonymous")
}
