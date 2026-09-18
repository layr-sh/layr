package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"layr.sh/core"
)

func (handler *BaseHandler) handleUserExport(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling user export request")
	targetUserID := request.PathValue("user_id")
	if targetUserID == "" {
		log.Debug("user export rejected: missing user_id path parameter")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "User ID required", "LAYR_AUTH_001")
		return
	}

	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		log.Debug("user export rejected: missing bearer token")
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Bearer token required", "LAYR_AUTH_002")
		return
	}
	if authContext.UserID != targetUserID {
		log.Debugf("user export rejected: unauthorized caller (authUserID: %q, targetUserID: %q)", authContext.UserID, targetUserID)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Unauthorized user export", "LAYR_AUTH_002")
		return
	}

	if handler.db == nil {
		log.Debug("user export rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	err := handler.db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, properties, created_at, last_updated_at
		FROM auth.users WHERE id = $1
	`, targetUserID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("user export failed: user %s not found: %v", targetUserID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	// Fetch identities
	identities := make([]ExportIdentityRecord, 0)
	identityRows, queryErr := handler.db.Query(ctx, `
		SELECT provider, provider_user_id, properties, created_at, last_sign_in_at 
		FROM auth.identities 
		WHERE user_id = $1
	`, targetUserID)
	if queryErr == nil {
		defer identityRows.Close()
		for identityRows.Next() {
			var exportIdentityRecord ExportIdentityRecord
			var propertiesJSON []byte
			if scanErr := identityRows.Scan(&exportIdentityRecord.Provider, &exportIdentityRecord.ProviderUserID, &propertiesJSON, &exportIdentityRecord.CreatedAt, &exportIdentityRecord.LastSignInAt); scanErr == nil {
				exportIdentityRecord.Properties = make(map[string]any)
				if len(propertiesJSON) > 0 {
					_ = json.Unmarshal(propertiesJSON, &exportIdentityRecord.Properties)
				}
				identities = append(identities, exportIdentityRecord)
			}
		}
	}

	exportUserDataResponse := ExportUserDataResponse{
		User:       userRecord,
		Identities: identities,
		ExportDate: time.Now().UTC().Format(time.RFC3339),
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewUserExportedEvent(userRecord.ID, UserExportedEventData(userRecord)))
	}

	handler.writeJSON(responseWriter, exportUserDataResponse)
}
