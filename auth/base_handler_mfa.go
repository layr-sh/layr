package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"layr.sh/core"
)

func (handler *BaseHandler) handleSetupMFA(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling setup MFA request")
	config := handler.configManager.Get()
	if !config.MFA.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "mfa setup rejected: MFA is disabled in configuration")
		return
	}

	var setupMFAInput SetupMFAInput
	if request.Body != nil && request.ContentLength != 0 {
		_ = json.NewDecoder(request.Body).Decode(&setupMFAInput)
	}

	userID := strings.TrimSpace(setupMFAInput.UserID)
	if userID == "" {
		authContext := core.GetAuthContext(request.Context())
		if authContext.UserID == "" {
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Bearer token required")
			return
		}
		userID = authContext.UserID
	}

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

	if user.LockedUntil != nil && time.Now().UTC().Before(*user.LockedUntil) {
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	secretBase32, _ := handler.totpManager.GenerateSecret()
	encryptedSecret, _ := handler.kernel.CryptoKeyManager().EncryptField([]byte(secretBase32))

	user.EncryptedMFASecret = &encryptedSecret
	user.MFAEnabled = false

	_, _ = handler.kernel.DB().Exec(ctx, `
		UPDATE auth.users
		SET encrypted_mfa_secret = $1, mfa_enabled = false, last_updated_at = clock_timestamp()
		WHERE id = $2
	`, encryptedSecret, user.ID)

	accountName := user.ID
	if user.Email != nil && *user.Email != "" {
		accountName = *user.Email
	} else if user.Phone != nil && *user.Phone != "" {
		accountName = *user.Phone
	}

	authURL := handler.totpManager.BuildAuthURL(accountName, secretBase32)

	core.WriteJSONResponse(responseWriter, http.StatusOK, SetupMFAResponse{
		Secret:        secretBase32,
		AuthURL:       authURL,
		Issuer:        config.MFA.Issuer,
		Digits:        6,
		PeriodSeconds: 30,
	})
}

func (handler *BaseHandler) handleVerifyMFA(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling verify MFA request")
	config := handler.configManager.Get()
	if !config.MFA.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "mfa verify rejected: MFA is disabled in configuration")
		return
	}

	var verifyMFAInput VerifyMFAInput
	if request.Body != nil && request.ContentLength != 0 {
		_ = json.NewDecoder(request.Body).Decode(&verifyMFAInput)
	}

	userID := strings.TrimSpace(verifyMFAInput.UserID)
	if userID == "" {
		authContext := core.GetAuthContext(request.Context())
		if authContext.UserID == "" {
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Bearer token required")
			return
		}
		userID = authContext.UserID
	}

	if verifyMFAInput.Code == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Code required")
		return
	}

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

	if user.LockedUntil != nil && time.Now().UTC().Before(*user.LockedUntil) {
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	if user.EncryptedMFASecret == nil || *user.EncryptedMFASecret == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "MFA is not set up on this account")
		return
	}

	secretBytes, err := handler.kernel.CryptoKeyManager().DecryptField(*user.EncryptedMFASecret)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("mfa verify rejected: failed to decrypt MFA secret for user %s: %v", user.ID, err))
		return
	}

	if !handler.totpManager.ValidateCode(string(secretBytes), verifyMFAInput.Code, time.Now().UTC(), 1) {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid MFA code")
		return
	}

	user.MFAEnabled = true

	_, _ = handler.kernel.DB().Exec(ctx, `
		UPDATE auth.users
		SET mfa_enabled = true, last_updated_at = clock_timestamp()
		WHERE id = $1
	`, user.ID)

	handler.kernel.EventBus().Publish(ctx, NewMFAEnabledEvent(user.ID, MFAEnabledEventData(user)))
	handler.kernel.EventBus().Publish(ctx, NewUserUpdatedEvent(user.ID, UserUpdatedEventData(user)))

	handler.issueSessionResponse(responseWriter, request, user, "mfa")
}

func (handler *BaseHandler) handleChallengeMFA(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling MFA challenge verification request")
	config := handler.configManager.Get()
	if !config.MFA.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "mfa challenge rejected: MFA is disabled in configuration")
		return
	}

	var challengeMFAInput ChallengeMFAInput
	if request.Body != nil && request.ContentLength != 0 {
		if err := json.NewDecoder(request.Body).Decode(&challengeMFAInput); err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
			return
		}
	}

	ticket := strings.TrimSpace(challengeMFAInput.MFATicket)
	code := strings.TrimSpace(challengeMFAInput.Code)
	if ticket == "" || code == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "MFA ticket and code are required")
		return
	}

	ctx := request.Context()
	ticketKey := "auth:mfa_ticket:" + ticket
	userID, err := handler.kernel.KVStore().Get(ctx, ticketKey)
	if err != nil || userID == "" {
		clientIP := core.ExtractRequestClientIP(request)
		handler.kernel.EventBus().Publish(ctx, NewMFAChallengeFailedEvent(ticket, MFAChallengeFailedEventData{
			Reason:    "invalid_ticket",
			IPAddress: clientIP,
			UserAgent: request.UserAgent(),
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid or expired MFA ticket")
		return
	}

	_ = handler.kernel.KVStore().Delete(ctx, ticketKey)

	var user User
	var rawProperties []byte
	err = handler.kernel.DB().QueryRow(ctx, `
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
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid or expired MFA ticket")
		return
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	if user.LockedUntil != nil && time.Now().UTC().Before(*user.LockedUntil) {
		clientIP := core.ExtractRequestClientIP(request)
		handler.kernel.EventBus().Publish(ctx, NewMFAChallengeFailedEvent(user.ID, MFAChallengeFailedEventData{
			UserID:    user.ID,
			Reason:    "account_locked",
			IPAddress: clientIP,
			UserAgent: request.UserAgent(),
			User:      &user,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	if user.EncryptedMFASecret == nil || *user.EncryptedMFASecret == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "MFA is not set up on this account")
		return
	}

	secretBytes, err := handler.kernel.CryptoKeyManager().DecryptField(*user.EncryptedMFASecret)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("MFA challenge rejected: failed to decrypt MFA secret for user %s: %v", user.ID, err))
		return
	}

	if !handler.totpManager.ValidateCode(string(secretBytes), code, time.Now().UTC(), 1) {
		clientIP := core.ExtractRequestClientIP(request)
		handler.kernel.EventBus().Publish(ctx, NewMFAChallengeFailedEvent(user.ID, MFAChallengeFailedEventData{
			UserID:    user.ID,
			Reason:    "invalid_code",
			IPAddress: clientIP,
			UserAgent: request.UserAgent(),
			User:      &user,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid MFA code")
		return
	}

	log.Debugf("MFA challenge succeeded for user %s, issuing session", user.ID)
	handler.issueSessionResponse(responseWriter, request, user, "mfa")
}

func (handler *BaseHandler) handleDisableMFA(responseWriter http.ResponseWriter, request *http.Request) {
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

	if user.LockedUntil != nil && time.Now().UTC().Before(*user.LockedUntil) {
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	user.MFAEnabled = false
	user.EncryptedMFASecret = nil

	_, _ = handler.kernel.DB().Exec(ctx, `
		UPDATE auth.users
		SET mfa_enabled = false, encrypted_mfa_secret = NULL, last_updated_at = clock_timestamp()
		WHERE id = $1
	`, user.ID)

	handler.kernel.EventBus().Publish(ctx, NewMFADisabledEvent(user.ID, MFADisabledEventData(user)))
	handler.kernel.EventBus().Publish(ctx, NewUserUpdatedEvent(user.ID, UserUpdatedEventData(user)))

	log.Debugf("MFA successfully disabled for user %s", user.ID)
	responseWriter.WriteHeader(http.StatusNoContent)
}
