package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"uuid"

	"layr.sh/auth/passkey"
	"layr.sh/core"
)

const defaultPasskeyChallengeTTL = 5 * time.Minute

func (handler *BaseHandler) handleBeginPasskeySignUp(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling passkey sign up initiation request")
	config := handler.configManager.Get()
	if !config.Passkeys.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "passkey sign up rejected: passkey authentication is disabled in configuration")
		return
	}

	var beginPasskeySignUpInput BeginPasskeySignUpInput
	if request.Body != nil && request.ContentLength != 0 {
		if err := json.NewDecoder(request.Body).Decode(&beginPasskeySignUpInput); err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
			return
		}
	}

	if beginPasskeySignUpInput.UserID == "" {
		if authContext := core.GetAuthContext(request.Context()); authContext.UserID != "" {
			beginPasskeySignUpInput.UserID = authContext.UserID
		}
	}

	if beginPasskeySignUpInput.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "User ID required")
		return
	}

	if beginPasskeySignUpInput.UserName == "" {
		beginPasskeySignUpInput.UserName = "User"
	}

	signUpOptions, err := handler.passkeyManager.BeginSignUp(beginPasskeySignUpInput.UserID, beginPasskeySignUpInput.UserName)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("passkey sign up challenge generation failed: %v", err))
		return
	}

	_ = handler.kernel.KVStore().Set(request.Context(), "auth:challenge:"+signUpOptions.Challenge, beginPasskeySignUpInput.UserID, defaultPasskeyChallengeTTL)

	log.Debugf("passkey sign up ceremony initiated for user %s", beginPasskeySignUpInput.UserID)
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(signUpOptions)
}

func (handler *BaseHandler) handleVerifyPasskeySignUp(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling passkey sign up verification request")
	config := handler.configManager.Get()
	if !config.Passkeys.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "passkey sign up verify rejected: passkey authentication is disabled in configuration")
		return
	}

	var verifyPasskeySignUpInput VerifyPasskeySignUpInput
	if err := json.NewDecoder(request.Body).Decode(&verifyPasskeySignUpInput); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if verifyPasskeySignUpInput.UserID == "" {
		if authContext := core.GetAuthContext(request.Context()); authContext.UserID != "" {
			verifyPasskeySignUpInput.UserID = authContext.UserID
		}
	}

	expectedUserID, err := handler.passkeyManager.ConsumeChallenge(verifyPasskeySignUpInput.Challenge)
	_ = handler.kernel.KVStore().Delete(request.Context(), "auth:challenge:"+verifyPasskeySignUpInput.Challenge)
	if err != nil || (expectedUserID != "" && expectedUserID != verifyPasskeySignUpInput.UserID) {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired challenge")
		return
	}

	credentialIDBytes := []byte(verifyPasskeySignUpInput.CredentialID)
	publicKeyBytes := []byte(verifyPasskeySignUpInput.PublicKey)
	if verifyPasskeySignUpInput.FriendlyName == "" {
		verifyPasskeySignUpInput.FriendlyName = "Passkey"
	}

	ctx := request.Context()

	anonymousUser, _ := handler.resolveAnonymousCaller(request)
	targetUserID := verifyPasskeySignUpInput.UserID
	isAnonymousConversion := false
	if anonymousUser != nil {
		targetUserID = anonymousUser.ID
		isAnonymousConversion = true
	}

	var user User
	var rawProperties []byte
	if isAnonymousConversion {
		err = handler.kernel.DB().QueryRow(ctx, `
			UPDATE auth.users
			SET is_anonymous = false, last_updated_at = clock_timestamp()
			WHERE id = $1
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		`, targetUserID).Scan(
			&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
			&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
			&user.EncryptedMFASecret, &user.MFAEnabled,
			&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
		)
	} else {
		err = handler.kernel.DB().QueryRow(ctx, `
			INSERT INTO auth.users (id, role, created_at, last_updated_at)
			VALUES ($1, 'authenticated', clock_timestamp(), clock_timestamp())
			ON CONFLICT (id) DO UPDATE SET last_updated_at = clock_timestamp()
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		`, targetUserID).Scan(
			&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
			&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
			&user.EncryptedMFASecret, &user.MFAEnabled,
			&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
		)
	}
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to query or upsert user for passkey: %v", err))
		return
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	// Insert passkey credential
	passkeyID := uuid.NewV7().String()
	_, execErr := handler.kernel.DB().Exec(ctx, `
		INSERT INTO auth.passkeys (id, user_id, credential_id, public_key, counter, transports, friendly_name, created_at, last_used_at)
		VALUES ($1, $2, $3, $4, 0, $5, $6, clock_timestamp(), clock_timestamp())
		ON CONFLICT (credential_id) DO UPDATE SET last_used_at = clock_timestamp()
	`, passkeyID, targetUserID, credentialIDBytes, publicKeyBytes, verifyPasskeySignUpInput.Transports, verifyPasskeySignUpInput.FriendlyName)
	if execErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to insert passkey credential for user %s: %v", targetUserID, execErr))
		return
	}

	handler.kernel.EventBus().Publish(ctx, NewPasskeyCreatedEvent(passkeyID, PasskeyCreatedEventData{
		ID:           passkeyID,
		User:         user,
		FriendlyName: verifyPasskeySignUpInput.FriendlyName,
		Transports:   verifyPasskeySignUpInput.Transports,
	}))
	if isAnonymousConversion {
		handler.kernel.EventBus().Publish(ctx, NewUserConvertedEvent(user.ID, UserConvertedEventData(user)))
	} else {
		handler.kernel.EventBus().Publish(ctx, NewUserSignedUpEvent(user.ID, UserSignedUpEventData(user)))
	}

	log.Debugf("passkey sign up verified and session issued for user %s", targetUserID)
	handler.issueSessionResponse(responseWriter, request, user, "passkey")
}

func (handler *BaseHandler) handleBeginPasskeySignIn(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling passkey sign-in initiation request")
	config := handler.configManager.Get()
	if !config.Passkeys.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "passkey sign-in rejected: passkey authentication is disabled in configuration")
		return
	}

	signInOptions, err := handler.passkeyManager.BeginSignIn()
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("passkey sign-in challenge generation failed: %v", err))
		return
	}

	_ = handler.kernel.KVStore().Set(request.Context(), "auth:challenge:"+signInOptions.Challenge, "", defaultPasskeyChallengeTTL)

	log.Debug("passkey sign-in challenge generated")
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(signInOptions)
}

func (handler *BaseHandler) handleVerifyPasskeySignIn(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling passkey sign-in assertion verification request")
	config := handler.configManager.Get()
	if !config.Passkeys.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "passkey sign-in verify rejected: passkey authentication is disabled in configuration")
		return
	}

	var verifyPasskeySignInInput VerifyPasskeySignInInput
	if err := json.NewDecoder(request.Body).Decode(&verifyPasskeySignInInput); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if _, err := handler.passkeyManager.ConsumeChallenge(verifyPasskeySignInInput.Challenge); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired challenge")
		return
	}
	_ = handler.kernel.KVStore().Delete(request.Context(), "auth:challenge:"+verifyPasskeySignInInput.Challenge)

	credentialIDBytes := []byte(verifyPasskeySignInInput.CredentialID)
	ctx := request.Context()
	var userID string
	var publicKey []byte
	err := handler.kernel.DB().QueryRow(ctx, "SELECT user_id, public_key FROM auth.passkeys WHERE credential_id = $1", credentialIDBytes).Scan(&userID, &publicKey)
	if err != nil {
		clientIP := core.ExtractRequestClientIP(request)
		handler.kernel.EventBus().Publish(ctx, NewUserSignInFailedEvent(string(credentialIDBytes), UserSignInFailedEventData{
			Identifier: string(credentialIDBytes),
			AuthMethod: "passkey",
			Reason:     "credential_not_found",
			IPAddress:  clientIP,
			UserAgent:  request.UserAgent(),
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if verifyPasskeySignInInput.Signature != "" {
		clientDataBytes := []byte(verifyPasskeySignInInput.ClientDataJSON)
		authenticatorDataBytes := []byte(verifyPasskeySignInInput.AuthenticatorData)
		signatureBytes := []byte(verifyPasskeySignInInput.Signature)
		if !passkey.VerifySignature(publicKey, clientDataBytes, authenticatorDataBytes, signatureBytes) {
			clientIP := core.ExtractRequestClientIP(request)
			handler.kernel.EventBus().Publish(ctx, NewUserSignInFailedEvent(userID, UserSignInFailedEventData{
				Identifier: userID,
				AuthMethod: "passkey",
				Reason:     "invalid_signature",
				IPAddress:  clientIP,
				UserAgent:  request.UserAgent(),
			}))
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials")
			return
		}
	}

	var user User
	var rawProperties []byte
	err = handler.kernel.DB().QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("passkey sign-in verify user lookup failed (userID: %s): %v", userID, err))
		return
	}

	if user.LockedUntil != nil && time.Now().UTC().Before(*user.LockedUntil) {
		clientIP := core.ExtractRequestClientIP(request)
		handler.kernel.EventBus().Publish(ctx, NewUserSignInFailedEvent(user.ID, UserSignInFailedEventData{
			Identifier: user.ID,
			AuthMethod: "passkey",
			Reason:     "account_locked",
			IPAddress:  clientIP,
			UserAgent:  request.UserAgent(),
			User:       &user,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	_, _ = handler.kernel.DB().Exec(ctx, "UPDATE auth.passkeys SET last_used_at = clock_timestamp() WHERE credential_id = $1", credentialIDBytes)
	log.Debugf("passkey sign-in verified and session issued for user %s", userID)
	handler.issueSessionResponse(responseWriter, request, user, "passkey")
}

func (handler *BaseHandler) handleListPasskeys(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list user passkeys request")
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := authContext.UserID
	ctx := request.Context()
	rows, err := handler.kernel.DB().Query(ctx, `
		SELECT id, friendly_name, transports, created_at, last_used_at
		FROM auth.passkeys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("list user passkeys failed for user %s: %v", userID, err))
		return
	}
	defer rows.Close()

	listPasskeysResponse := make(ListPasskeysResponse, 0)
	for rows.Next() {
		var passkey Passkey
		_ = rows.Scan(
			&passkey.ID,
			&passkey.FriendlyName,
			&passkey.Transports,
			&passkey.CreatedAt,
			&passkey.LastUsedAt,
		)
		listPasskeysResponse = append(listPasskeysResponse, passkey)
	}

	log.Debugf("retrieved %d passkey(s) for user %s", len(listPasskeysResponse), userID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, listPasskeysResponse)
}

func (handler *BaseHandler) handleDeletePasskey(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling delete user passkey request")
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := authContext.UserID

	passkeyID := strings.TrimSpace(request.PathValue("passkey_id"))
	if passkeyID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Passkey ID required")
		return
	}

	ctx := request.Context()
	result, err := handler.kernel.DB().Exec(ctx, `
		DELETE FROM auth.passkeys
		WHERE id = $1 AND user_id = $2
	`, passkeyID, userID)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("delete user passkey %s failed for user %s: %v", passkeyID, userID, err))
		return
	}

	if result.RowsAffected() == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Passkey not found")
		return
	}

	var user User
	var rawProperties []byte
	fetchErr := handler.kernel.DB().QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if fetchErr == nil {
		user.Properties = make(map[string]any)
		if len(rawProperties) > 0 {
			_ = json.Unmarshal(rawProperties, &user.Properties)
		}
		handler.kernel.EventBus().Publish(ctx, NewPasskeyDeletedEvent(passkeyID, PasskeyDeletedEventData{
			ID:   passkeyID,
			User: user,
		}))
	}

	log.Debugf("passkey %s successfully deleted for user %s", passkeyID, userID)
	responseWriter.WriteHeader(http.StatusNoContent)
}
