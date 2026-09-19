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
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "User ID required")
		return
	}

	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Bearer token required")
		return
	}
	if authContext.UserID != targetUserID {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Unauthorized user export")
		return
	}

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "user export rejected: database pool unavailable")
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
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found")
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
