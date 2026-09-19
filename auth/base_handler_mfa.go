package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"layr.sh/core"
)

func (handler *BaseHandler) handleMFASetup(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.MFA.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "mfa setup rejected: MFA is disabled in configuration")
		return
	}

	var mfaSetupRequest MFASetupRequest
	if request.Body != nil && request.ContentLength != 0 {
		_ = json.NewDecoder(request.Body).Decode(&mfaSetupRequest)
	}

	userID := strings.TrimSpace(mfaSetupRequest.UserID)
	if userID == "" {
		authContext := core.GetAuthContext(request.Context())
		if authContext.UserID == "" {
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Bearer token required")
			return
		}
		userID = authContext.UserID
	}

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "mfa setup rejected: database pool unavailable")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	err := handler.db.QueryRow(ctx, `
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
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	secretBase32, _ := handler.totpManager.GenerateSecret()
	encryptedSecret, _ := handler.cryptoKeyManager.EncryptField([]byte(secretBase32))

	userRecord.EncryptedMFASecret = &encryptedSecret
	userRecord.MFAEnabled = false

	_, _ = handler.db.Exec(ctx, `
		UPDATE auth.users
		SET encrypted_mfa_secret = $1, mfa_enabled = false, last_updated_at = clock_timestamp()
		WHERE id = $2
	`, encryptedSecret, userRecord.ID)

	accountName := userRecord.ID
	if userRecord.Email != nil && *userRecord.Email != "" {
		accountName = *userRecord.Email
	} else if userRecord.Phone != nil && *userRecord.Phone != "" {
		accountName = *userRecord.Phone
	}

	authURL := handler.totpManager.BuildAuthURL(accountName, secretBase32)

	handler.writeJSON(responseWriter, MFASetupResponse{
		Secret:        secretBase32,
		AuthURL:       authURL,
		Issuer:        config.MFA.Issuer,
		Digits:        6,
		PeriodSeconds: 30,
	})
}

func (handler *BaseHandler) handleMFAVerify(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.MFA.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "mfa verify rejected: MFA is disabled in configuration")
		return
	}

	var mfaVerifyRequest MFAVerifyRequest
	if request.Body != nil && request.ContentLength != 0 {
		_ = json.NewDecoder(request.Body).Decode(&mfaVerifyRequest)
	}

	userID := strings.TrimSpace(mfaVerifyRequest.UserID)
	if userID == "" {
		authContext := core.GetAuthContext(request.Context())
		if authContext.UserID == "" {
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Bearer token required")
			return
		}
		userID = authContext.UserID
	}

	if mfaVerifyRequest.Code == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Code required")
		return
	}

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "mfa verify rejected: database pool unavailable")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	err := handler.db.QueryRow(ctx, `
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
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	if userRecord.EncryptedMFASecret == nil || *userRecord.EncryptedMFASecret == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "MFA is not set up on this account")
		return
	}

	secretBytes, err := handler.cryptoKeyManager.DecryptField(*userRecord.EncryptedMFASecret)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("mfa verify rejected: failed to decrypt MFA secret for user %s: %v", userRecord.ID, err))
		return
	}

	if !handler.totpManager.ValidateCode(string(secretBytes), mfaVerifyRequest.Code, time.Now().UTC(), 1) {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid MFA code")
		return
	}

	userRecord.MFAEnabled = true

	_, _ = handler.db.Exec(ctx, `
		UPDATE auth.users
		SET mfa_enabled = true, last_updated_at = clock_timestamp()
		WHERE id = $1
	`, userRecord.ID)

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewMFAEnabledEvent(userRecord.ID, MFAEnabledEventData(userRecord)))
		handler.eventBus.Publish(ctx, NewUserUpdatedEvent(userRecord.ID, UserUpdatedEventData(userRecord)))
	}

	handler.issueSessionResponse(responseWriter, request, userRecord, "mfa")
}

func (handler *BaseHandler) handleMFAChallenge(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling MFA challenge verification request")
	config := handler.configManager.Get()
	if !config.MFA.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "mfa challenge rejected: MFA is disabled in configuration")
		return
	}

	var mfaChallengeRequest MFAChallengeRequest
	if request.Body != nil && request.ContentLength != 0 {
		if err := json.NewDecoder(request.Body).Decode(&mfaChallengeRequest); err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
			return
		}
	}

	ticket := strings.TrimSpace(mfaChallengeRequest.MFATicket)
	code := strings.TrimSpace(mfaChallengeRequest.Code)
	if ticket == "" || code == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "MFA ticket and code are required")
		return
	}

	if handler.kvStore == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "mfa challenge rejected: KV store unavailable")
		return
	}

	ctx := request.Context()
	ticketKey := "auth:mfa_ticket:" + ticket
	userID, err := handler.kvStore.Get(ctx, ticketKey)
	if err != nil || userID == "" {
		if handler.eventBus != nil {
			clientIP := core.ExtractRequestClientIP(request)
			handler.eventBus.Publish(ctx, NewMFAChallengeFailedEvent(ticket, MFAChallengeFailedEventData{
				Reason:    "invalid_ticket",
				IPAddress: clientIP,
				UserAgent: request.UserAgent(),
			}))
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid or expired MFA ticket")
		return
	}

	_ = handler.kvStore.Delete(ctx, ticketKey)

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "mfa challenge rejected: database pool unavailable")
		return
	}

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
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid or expired MFA ticket")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
		if handler.eventBus != nil {
			clientIP := core.ExtractRequestClientIP(request)
			handler.eventBus.Publish(ctx, NewMFAChallengeFailedEvent(userRecord.ID, MFAChallengeFailedEventData{
				UserID:    userRecord.ID,
				Reason:    "account_locked",
				IPAddress: clientIP,
				UserAgent: request.UserAgent(),
				User:      &userRecord,
			}))
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	if userRecord.EncryptedMFASecret == nil || *userRecord.EncryptedMFASecret == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "MFA is not set up on this account")
		return
	}

	secretBytes, err := handler.cryptoKeyManager.DecryptField(*userRecord.EncryptedMFASecret)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("MFA challenge rejected: failed to decrypt MFA secret for user %s: %v", userRecord.ID, err))
		return
	}

	if !handler.totpManager.ValidateCode(string(secretBytes), code, time.Now().UTC(), 1) {
		if handler.eventBus != nil {
			clientIP := core.ExtractRequestClientIP(request)
			handler.eventBus.Publish(ctx, NewMFAChallengeFailedEvent(userRecord.ID, MFAChallengeFailedEventData{
				UserID:    userRecord.ID,
				Reason:    "invalid_code",
				IPAddress: clientIP,
				UserAgent: request.UserAgent(),
				User:      &userRecord,
			}))
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid MFA code")
		return
	}

	log.Debugf("MFA challenge succeeded for user %s, issuing session", userRecord.ID)
	handler.issueSessionResponse(responseWriter, request, userRecord, "mfa")
}

func (handler *BaseHandler) handleMFADisable(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling MFA disable request")
	config := handler.configManager.Get()
	if !config.MFA.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "MFA disable rejected: MFA is disabled in configuration")
		return
	}

	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := authContext.UserID

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "mfa disable rejected: database pool unavailable")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	err := handler.db.QueryRow(ctx, `
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
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	userRecord.MFAEnabled = false
	userRecord.EncryptedMFASecret = nil

	_, _ = handler.db.Exec(ctx, `
		UPDATE auth.users
		SET mfa_enabled = false, encrypted_mfa_secret = NULL, last_updated_at = clock_timestamp()
		WHERE id = $1
	`, userRecord.ID)

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewMFADisabledEvent(userRecord.ID, MFADisabledEventData(userRecord)))
		handler.eventBus.Publish(ctx, NewUserUpdatedEvent(userRecord.ID, UserUpdatedEventData(userRecord)))
	}

	log.Debugf("MFA successfully disabled for user %s", userRecord.ID)
	responseWriter.WriteHeader(http.StatusNoContent)
}
