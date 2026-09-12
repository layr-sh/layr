package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"layr.sh/core"
)

// MFASetupRequest defines input for TOTP MFA setup.
type MFASetupRequest struct {
	UserID string `json:"user_id,omitempty"`
}

// MFASetupResponse defines response payload for TOTP MFA setup.
type MFASetupResponse struct {
	Secret        string `json:"secret"`
	AuthURL       string `json:"auth_url"`
	Issuer        string `json:"issuer"`
	Digits        int    `json:"digits"`
	PeriodSeconds int    `json:"period_seconds"`
}

// MFAVerifyRequest defines input for TOTP MFA verification.
type MFAVerifyRequest struct {
	UserID string `json:"user_id,omitempty"`
	Code   string `json:"code"`
}

func (handler *Handler) handleMFASetup(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.MFA.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "MFA is disabled", "LAYR_AUTH_001")
		return
	}

	var mfaSetupRequest MFASetupRequest
	if request.Body != nil && request.ContentLength != 0 {
		_ = json.NewDecoder(request.Body).Decode(&mfaSetupRequest)
	}

	userID := strings.TrimSpace(mfaSetupRequest.UserID)
	if userID == "" {
		token := core.ExtractRequestSessionToken(request, AuthSessionCookieName, AuthSessionInsecureCookieName)
		if token == "" {
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Bearer token required", "LAYR_AUTH_002")
			return
		}
		claims, err := handler.signer.VerifyAccessToken(token)
		if err != nil || claims == nil || claims.Subject == "" {
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid token", "LAYR_AUTH_002")
			return
		}
		userID = claims.Subject
	}

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	err := handler.db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		FROM auth.users
		WHERE id = $1
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	secretBase32, _ := handler.totpManager.GenerateSecret()
	encryptedSecret, _ := handler.cryptoKeyManager.EncryptField([]byte(secretBase32))

	userRecord.Properties["mfa_secret_enc"] = encryptedSecret
	userRecord.Properties["mfa_pending"] = true
	propertiesJSON, _ := json.Marshal(userRecord.Properties)

	_, _ = handler.db.Exec(ctx, `
		UPDATE auth.users
		SET properties = $1, last_updated_at = clock_timestamp()
		WHERE id = $2
	`, propertiesJSON, userRecord.ID)

	accountName := userRecord.ID
	if userRecord.Email != nil && *userRecord.Email != "" {
		accountName = *userRecord.Email
	} else if userRecord.Phone != nil && *userRecord.Phone != "" {
		accountName = *userRecord.Phone
	}

	authURL := handler.totpManager.BuildAuthURL(accountName, secretBase32)
	issuer := config.MFA.Issuer
	if issuer == "" {
		issuer = "Layr"
	}

	handler.writeJSON(responseWriter, MFASetupResponse{
		Secret:        secretBase32,
		AuthURL:       authURL,
		Issuer:        issuer,
		Digits:        6,
		PeriodSeconds: 30,
	})
}

func (handler *Handler) handleMFAVerify(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.MFA.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "MFA is disabled", "LAYR_AUTH_001")
		return
	}

	var mfaVerifyRequest MFAVerifyRequest
	if request.Body != nil && request.ContentLength != 0 {
		_ = json.NewDecoder(request.Body).Decode(&mfaVerifyRequest)
	}

	userID := strings.TrimSpace(mfaVerifyRequest.UserID)
	if userID == "" {
		token := core.ExtractRequestSessionToken(request, AuthSessionCookieName, AuthSessionInsecureCookieName)
		if token == "" {
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Bearer token required", "LAYR_AUTH_002")
			return
		}
		claims, err := handler.signer.VerifyAccessToken(token)
		if err != nil || claims == nil || claims.Subject == "" {
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid token", "LAYR_AUTH_002")
			return
		}
		userID = claims.Subject
	}

	if mfaVerifyRequest.Code == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Code required", "LAYR_AUTH_001")
		return
	}

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	err := handler.db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		FROM auth.users
		WHERE id = $1
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked", "LAYR_AUTH_005")
		return
	}

	encryptedSecret, ok := userRecord.Properties["mfa_secret_enc"].(string)
	if !ok || encryptedSecret == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "MFA is not set up on this account", "LAYR_AUTH_001")
		return
	}

	secretBytes, err := handler.cryptoKeyManager.DecryptField(encryptedSecret)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to decrypt MFA secret", "LAYR_AUTH_001")
		return
	}

	if !handler.totpManager.ValidateCode(string(secretBytes), mfaVerifyRequest.Code, time.Now().UTC(), 1) {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid MFA code", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties["mfa_enabled"] = true
	delete(userRecord.Properties, "mfa_pending")
	propertiesJSON, _ := json.Marshal(userRecord.Properties)

	_, _ = handler.db.Exec(ctx, `
		UPDATE auth.users
		SET properties = $1, last_updated_at = clock_timestamp()
		WHERE id = $2
	`, propertiesJSON, userRecord.ID)

	if handler.eventBus != nil {
		recipient := userRecord.ID
		if userRecord.Email != nil && *userRecord.Email != "" {
			recipient = *userRecord.Email
		} else if userRecord.Phone != nil && *userRecord.Phone != "" {
			recipient = *userRecord.Phone
		}
		handler.eventBus.Publish(ctx, NewOTPVerifiedEvent(userRecord.ID, OTPVerifiedEventData{
			UserID:    userRecord.ID,
			Recipient: recipient,
			Purpose:   "mfa",
		}))
		handler.eventBus.Publish(ctx, NewUserUpdatedEvent(userRecord.ID, UserUpdatedEventData(userRecord)))
	}

	handler.issueSessionResponse(responseWriter, request, userRecord)
}
