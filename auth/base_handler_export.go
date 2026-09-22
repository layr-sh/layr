package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"layr.sh/core"
)

func (handler *BaseHandler) handleExportUser(responseWriter http.ResponseWriter, request *http.Request) {
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

	ctx := request.Context()
	var user User
	var rawProperties []byte
	err := handler.kernel.DB().QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, properties, created_at, last_updated_at
		FROM auth.users WHERE id = $1
	`, targetUserID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found")
		return
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	// Fetch identities
	identities := make([]ExportIdentityRecord, 0)
	identityRows, queryErr := handler.kernel.DB().Query(ctx, `
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

	exportUserResponse := ExportUserResponse{
		User:       user,
		Identities: identities,
		ExportDate: time.Now().UTC().Format(time.RFC3339),
	}

	handler.kernel.EventBus().Publish(ctx, NewUserExportedEvent(user.ID, UserExportedEventData(user)))

	core.WriteJSONResponse(responseWriter, http.StatusOK, exportUserResponse)
}
