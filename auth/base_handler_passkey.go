package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"uuid"

	"layr.sh/auth/passkey"
	"layr.sh/core"
)

const defaultPasskeyChallengeTTL = 5 * time.Minute

func (handler *BaseHandler) handlePasskeySignUp(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling passkey sign up initiation request")
	config := handler.configManager.Get()
	if !config.Passkeys.Enabled {
		log.Debug("passkey sign up rejected: passkey authentication is disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Passkey authentication is disabled", "LAYR_AUTH_001")
		return
	}

	var passkeySignUpRequest PasskeySignUpRequest
	if request.Body != nil && request.ContentLength != 0 {
		if err := json.NewDecoder(request.Body).Decode(&passkeySignUpRequest); err != nil {
			log.Debugf("passkey sign up rejected: invalid JSON body: %v", err)
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", "LAYR_AUTH_001")
			return
		}
	}

	if passkeySignUpRequest.UserID == "" {
		if authUserID, err := handler.authenticateUser(request); err == nil && authUserID != "" {
			passkeySignUpRequest.UserID = authUserID
		}
	}

	if passkeySignUpRequest.UserID == "" {
		log.Debug("passkey sign up rejected: empty user_id")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "User ID required", "LAYR_AUTH_001")
		return
	}

	if passkeySignUpRequest.UserName == "" {
		passkeySignUpRequest.UserName = "User"
	}

	signUpOptions, err := handler.passkeyManager.BeginSignUp(passkeySignUpRequest.UserID, passkeySignUpRequest.UserName)
	if err != nil {
		log.Debugf("passkey sign up challenge generation failed: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to begin passkey sign up", "LAYR_AUTH_001")
		return
	}

	if handler.kvStore != nil {
		_ = handler.kvStore.Set(request.Context(), "auth:challenge:"+signUpOptions.Challenge, passkeySignUpRequest.UserID, defaultPasskeyChallengeTTL)
	}

	log.Debugf("passkey sign up ceremony initiated for user %s", passkeySignUpRequest.UserID)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(signUpOptions)
}

func (handler *BaseHandler) handlePasskeySignUpVerify(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling passkey sign up verification request")
	config := handler.configManager.Get()
	if !config.Passkeys.Enabled {
		log.Debug("passkey sign up verify rejected: passkey authentication is disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Passkey authentication is disabled", "LAYR_AUTH_001")
		return
	}

	var passkeySignUpVerifyRequest PasskeySignUpVerifyRequest
	if err := json.NewDecoder(request.Body).Decode(&passkeySignUpVerifyRequest); err != nil {
		log.Debugf("passkey sign up verify rejected: invalid JSON body: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", "LAYR_AUTH_006")
		return
	}

	if passkeySignUpVerifyRequest.UserID == "" {
		if authUserID, authErr := handler.authenticateUser(request); authErr == nil && authUserID != "" {
			passkeySignUpVerifyRequest.UserID = authUserID
		}
	}

	expectedUserID, err := handler.passkeyManager.ConsumeChallenge(passkeySignUpVerifyRequest.Challenge)
	if handler.kvStore != nil {
		_ = handler.kvStore.Delete(request.Context(), "auth:challenge:"+passkeySignUpVerifyRequest.Challenge)
	}
	if err != nil || (expectedUserID != "" && expectedUserID != passkeySignUpVerifyRequest.UserID) {
		log.Debugf("passkey challenge consumption failed (expected: %q, got: %q): %v", expectedUserID, passkeySignUpVerifyRequest.UserID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired challenge", "LAYR_AUTH_006")
		return
	}

	credentialIDBytes := []byte(passkeySignUpVerifyRequest.CredentialID)
	publicKeyBytes := []byte(passkeySignUpVerifyRequest.PublicKey)
	if passkeySignUpVerifyRequest.FriendlyName == "" {
		passkeySignUpVerifyRequest.FriendlyName = "Passkey"
	}

	if handler.db == nil {
		log.Debug("passkey sign up verify rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()

	anonymousUserRecord, _ := handler.resolveAnonymousCaller(request)
	targetUserID := passkeySignUpVerifyRequest.UserID
	isAnonymousConversion := false
	if anonymousUserRecord != nil {
		targetUserID = anonymousUserRecord.ID
		isAnonymousConversion = true
	}

	var userRecord UserRecord
	var rawProperties []byte
	if isAnonymousConversion {
		err = handler.db.QueryRow(ctx, `
			UPDATE auth.users
			SET is_anonymous = false, last_updated_at = clock_timestamp()
			WHERE id = $1
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		`, targetUserID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
			&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
			&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
		)
	} else {
		err = handler.db.QueryRow(ctx, `
			INSERT INTO auth.users (id, role, created_at, last_updated_at)
			VALUES ($1, 'authenticated', clock_timestamp(), clock_timestamp())
			ON CONFLICT (id) DO UPDATE SET last_updated_at = clock_timestamp()
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		`, targetUserID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
			&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
			&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
		)
	}
	if err != nil {
		log.Debugf("failed to query or upsert user for passkey: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to persist user for passkey", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	// Insert passkey credential
	passkeyID := uuid.NewV7().String()
	_, execErr := handler.db.Exec(ctx, `
		INSERT INTO auth.passkeys (id, user_id, credential_id, public_key, counter, transports, friendly_name, created_at, last_used_at)
		VALUES ($1, $2, $3, $4, 0, $5, $6, clock_timestamp(), clock_timestamp())
		ON CONFLICT (credential_id) DO UPDATE SET last_used_at = clock_timestamp()
	`, passkeyID, targetUserID, credentialIDBytes, publicKeyBytes, passkeySignUpVerifyRequest.Transports, passkeySignUpVerifyRequest.FriendlyName)
	if execErr != nil {
		log.Debugf("failed to insert passkey credential for user %s: %v", targetUserID, execErr)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to store passkey credential", "LAYR_AUTH_001")
		return
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewPasskeyCreatedEvent(passkeyID, PasskeyCreatedEventData{
			ID:           passkeyID,
			User:         userRecord,
			FriendlyName: passkeySignUpVerifyRequest.FriendlyName,
			Transports:   passkeySignUpVerifyRequest.Transports,
		}))
		if isAnonymousConversion {
			handler.eventBus.Publish(ctx, NewUserConvertedEvent(userRecord.ID, UserConvertedEventData(userRecord)))
		} else {
			handler.eventBus.Publish(ctx, NewUserSignedUpEvent(userRecord.ID, UserSignedUpEventData(userRecord)))
		}
	}

	log.Debugf("passkey sign up verified and session issued for user %s", targetUserID)
	handler.issueSessionResponse(responseWriter, request, userRecord, "passkey")
}

func (handler *BaseHandler) handlePasskeySignIn(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling passkey sign-in initiation request")
	config := handler.configManager.Get()
	if !config.Passkeys.Enabled {
		log.Debug("passkey sign-in rejected: passkey authentication is disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Passkey authentication is disabled", "LAYR_AUTH_001")
		return
	}

	signInOptions, err := handler.passkeyManager.BeginSignIn()
	if err != nil {
		log.Debugf("passkey sign-in challenge generation failed: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to begin passkey sign-in", "LAYR_AUTH_001")
		return
	}

	if handler.kvStore != nil {
		_ = handler.kvStore.Set(request.Context(), "auth:challenge:"+signInOptions.Challenge, "", defaultPasskeyChallengeTTL)
	}

	log.Debug("passkey sign-in challenge generated")
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(signInOptions)
}

func (handler *BaseHandler) handlePasskeySignInVerify(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling passkey sign-in assertion verification request")
	config := handler.configManager.Get()
	if !config.Passkeys.Enabled {
		log.Debug("passkey sign-in verify rejected: passkey authentication is disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Passkey authentication is disabled", "LAYR_AUTH_001")
		return
	}

	var passkeySignInVerifyRequest PasskeySignInVerifyRequest
	if err := json.NewDecoder(request.Body).Decode(&passkeySignInVerifyRequest); err != nil {
		log.Debugf("passkey sign-in verify rejected: invalid JSON body: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", "LAYR_AUTH_006")
		return
	}

	if _, err := handler.passkeyManager.ConsumeChallenge(passkeySignInVerifyRequest.Challenge); err != nil {
		log.Debugf("passkey sign-in verify rejected: challenge consumption failed: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired challenge", "LAYR_AUTH_006")
		return
	}
	if handler.kvStore != nil {
		_ = handler.kvStore.Delete(request.Context(), "auth:challenge:"+passkeySignInVerifyRequest.Challenge)
	}

	credentialIDBytes := []byte(passkeySignInVerifyRequest.CredentialID)
	if handler.db == nil {
		log.Debug("passkey sign-in verify rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var userID string
	var publicKey []byte
	err := handler.db.QueryRow(ctx, "SELECT user_id, public_key FROM auth.passkeys WHERE credential_id = $1", credentialIDBytes).Scan(&userID, &publicKey)
	if err != nil {
		log.Debugf("passkey sign-in verify rejected: credential not found: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Passkey credential not found", "LAYR_AUTH_006")
		return
	}

	if passkeySignInVerifyRequest.Signature != "" {
		clientDataBytes := []byte(passkeySignInVerifyRequest.ClientDataJSON)
		authenticatorDataBytes := []byte(passkeySignInVerifyRequest.AuthenticatorData)
		signatureBytes := []byte(passkeySignInVerifyRequest.Signature)
		if !passkey.VerifySignature(publicKey, clientDataBytes, authenticatorDataBytes, signatureBytes) {
			log.Debug("passkey sign-in verify rejected: invalid assertion signature")
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid passkey signature", "LAYR_AUTH_006")
			return
		}
	}

	var userRecord UserRecord
	var rawProperties []byte
	err = handler.db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users WHERE id = $1
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("passkey sign-in verify user lookup failed (userID: %s): %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "User lookup failed", "LAYR_AUTH_001")
		return
	}

	if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
		log.Debugf("passkey sign-in rejected: account locked until %v for user %s", *userRecord.LockedUntil, userRecord.ID)
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked", "LAYR_AUTH_005")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	_, _ = handler.db.Exec(ctx, "UPDATE auth.passkeys SET last_used_at = clock_timestamp() WHERE credential_id = $1", credentialIDBytes)
	log.Debugf("passkey sign-in verified and session issued for user %s", userID)
	handler.issueSessionResponse(responseWriter, request, userRecord, "passkey")
}

func (handler *BaseHandler) handleListUserPasskeys(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling list user passkeys request")
	userID, err := handler.authenticateUser(request)
	if err != nil {
		log.Debugf("list user passkeys rejected: unauthenticated caller: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	if handler.db == nil {
		log.Debug("list user passkeys rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	rows, err := handler.db.Query(ctx, `
		SELECT id, friendly_name, transports, created_at, last_used_at
		FROM auth.passkeys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		log.Debugf("list user passkeys failed for user %s: %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to list passkeys", "LAYR_AUTH_001")
		return
	}
	defer rows.Close()

	userPasskeyResponses := make([]UserPasskeyResponse, 0)
	for rows.Next() {
		var userPasskeyResponse UserPasskeyResponse
		_ = rows.Scan(
			&userPasskeyResponse.ID,
			&userPasskeyResponse.FriendlyName,
			&userPasskeyResponse.Transports,
			&userPasskeyResponse.CreatedAt,
			&userPasskeyResponse.LastUsedAt,
		)
		userPasskeyResponses = append(userPasskeyResponses, userPasskeyResponse)
	}

	handler.writeJSON(responseWriter, userPasskeyResponses)
}

func (handler *BaseHandler) handleDeleteUserPasskey(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling delete user passkey request")
	userID, err := handler.authenticateUser(request)
	if err != nil {
		log.Debugf("delete user passkey rejected: unauthenticated caller: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	passkeyID := strings.TrimSpace(request.PathValue("id"))
	if passkeyID == "" {
		log.Debug("delete user passkey rejected: passkey id is empty")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Passkey ID required", "LAYR_AUTH_001")
		return
	}

	if handler.db == nil {
		log.Debug("delete user passkey rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	result, err := handler.db.Exec(ctx, `
		DELETE FROM auth.passkeys
		WHERE id = $1 AND user_id = $2
	`, passkeyID, userID)
	if err != nil {
		log.Debugf("delete user passkey %s failed for user %s: %v", passkeyID, userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to delete passkey", "LAYR_AUTH_001")
		return
	}

	if result.RowsAffected() == 0 {
		log.Debugf("passkey %s not found for user %s", passkeyID, userID)
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Passkey not found", "LAYR_AUTH_001")
		return
	}

	if handler.eventBus != nil {
		var userRecord UserRecord
		var rawProperties []byte
		fetchErr := handler.db.QueryRow(ctx, `
			SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
			FROM auth.users
			WHERE id = $1
		`, userID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
			&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
			&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
		)
		if fetchErr == nil {
			userRecord.Properties = make(map[string]any)
			if len(rawProperties) > 0 {
				_ = json.Unmarshal(rawProperties, &userRecord.Properties)
			}
			handler.eventBus.Publish(ctx, NewPasskeyDeletedEvent(passkeyID, PasskeyDeletedEventData{
				ID:   passkeyID,
				User: userRecord,
			}))
		}
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}
