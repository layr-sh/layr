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

func (handler *BaseHandler) handlePasskeySignUp(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling passkey sign up initiation request")
	config := handler.configManager.Get()
	if !config.Passkeys.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "passkey sign up rejected: passkey authentication is disabled in configuration")
		return
	}

	var passkeySignUpRequest PasskeySignUpRequest
	if request.Body != nil && request.ContentLength != 0 {
		if err := json.NewDecoder(request.Body).Decode(&passkeySignUpRequest); err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
			return
		}
	}

	if passkeySignUpRequest.UserID == "" {
		if authContext := core.GetAuthContext(request.Context()); authContext.UserID != "" {
			passkeySignUpRequest.UserID = authContext.UserID
		}
	}

	if passkeySignUpRequest.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "User ID required")
		return
	}

	if passkeySignUpRequest.UserName == "" {
		passkeySignUpRequest.UserName = "User"
	}

	signUpOptions, err := handler.passkeyManager.BeginSignUp(passkeySignUpRequest.UserID, passkeySignUpRequest.UserName)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("passkey sign up challenge generation failed: %v", err))
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
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "passkey sign up verify rejected: passkey authentication is disabled in configuration")
		return
	}

	var passkeySignUpVerifyRequest PasskeySignUpVerifyRequest
	if err := json.NewDecoder(request.Body).Decode(&passkeySignUpVerifyRequest); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if passkeySignUpVerifyRequest.UserID == "" {
		if authContext := core.GetAuthContext(request.Context()); authContext.UserID != "" {
			passkeySignUpVerifyRequest.UserID = authContext.UserID
		}
	}

	expectedUserID, err := handler.passkeyManager.ConsumeChallenge(passkeySignUpVerifyRequest.Challenge)
	if handler.kvStore != nil {
		_ = handler.kvStore.Delete(request.Context(), "auth:challenge:"+passkeySignUpVerifyRequest.Challenge)
	}
	if err != nil || (expectedUserID != "" && expectedUserID != passkeySignUpVerifyRequest.UserID) {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired challenge")
		return
	}

	credentialIDBytes := []byte(passkeySignUpVerifyRequest.CredentialID)
	publicKeyBytes := []byte(passkeySignUpVerifyRequest.PublicKey)
	if passkeySignUpVerifyRequest.FriendlyName == "" {
		passkeySignUpVerifyRequest.FriendlyName = "Passkey"
	}

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "passkey sign up verify rejected: database pool unavailable")
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
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to query or upsert user for passkey: %v", err))
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
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to insert passkey credential for user %s: %v", targetUserID, execErr))
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
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "passkey sign-in rejected: passkey authentication is disabled in configuration")
		return
	}

	signInOptions, err := handler.passkeyManager.BeginSignIn()
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("passkey sign-in challenge generation failed: %v", err))
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
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "passkey sign-in verify rejected: passkey authentication is disabled in configuration")
		return
	}

	var passkeySignInVerifyRequest PasskeySignInVerifyRequest
	if err := json.NewDecoder(request.Body).Decode(&passkeySignInVerifyRequest); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if _, err := handler.passkeyManager.ConsumeChallenge(passkeySignInVerifyRequest.Challenge); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired challenge")
		return
	}
	if handler.kvStore != nil {
		_ = handler.kvStore.Delete(request.Context(), "auth:challenge:"+passkeySignInVerifyRequest.Challenge)
	}

	credentialIDBytes := []byte(passkeySignInVerifyRequest.CredentialID)
	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "passkey sign-in verify rejected: database pool unavailable")
		return
	}

	ctx := request.Context()
	var userID string
	var publicKey []byte
	err := handler.db.QueryRow(ctx, "SELECT user_id, public_key FROM auth.passkeys WHERE credential_id = $1", credentialIDBytes).Scan(&userID, &publicKey)
	if err != nil {
		if handler.eventBus != nil {
			clientIP := core.ExtractRequestClientIP(request)
			handler.eventBus.Publish(ctx, NewUserSignInFailedEvent(string(credentialIDBytes), UserSignInFailedEventData{
				Identifier: string(credentialIDBytes),
				AuthMethod: "passkey",
				Reason:     "credential_not_found",
				IPAddress:  clientIP,
				UserAgent:  request.UserAgent(),
			}))
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Passkey credential not found")
		return
	}

	if passkeySignInVerifyRequest.Signature != "" {
		clientDataBytes := []byte(passkeySignInVerifyRequest.ClientDataJSON)
		authenticatorDataBytes := []byte(passkeySignInVerifyRequest.AuthenticatorData)
		signatureBytes := []byte(passkeySignInVerifyRequest.Signature)
		if !passkey.VerifySignature(publicKey, clientDataBytes, authenticatorDataBytes, signatureBytes) {
			if handler.eventBus != nil {
				clientIP := core.ExtractRequestClientIP(request)
				handler.eventBus.Publish(ctx, NewUserSignInFailedEvent(userID, UserSignInFailedEventData{
					Identifier: userID,
					AuthMethod: "passkey",
					Reason:     "invalid_signature",
					IPAddress:  clientIP,
					UserAgent:  request.UserAgent(),
				}))
			}
			core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid passkey signature")
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
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("passkey sign-in verify user lookup failed (userID: %s): %v", userID, err))
		return
	}

	if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
		if handler.eventBus != nil {
			clientIP := core.ExtractRequestClientIP(request)
			handler.eventBus.Publish(ctx, NewUserSignInFailedEvent(userRecord.ID, UserSignInFailedEventData{
				Identifier: userRecord.ID,
				AuthMethod: "passkey",
				Reason:     "account_locked",
				IPAddress:  clientIP,
				UserAgent:  request.UserAgent(),
				User:       &userRecord,
			}))
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
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
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := authContext.UserID

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "list user passkeys rejected: database pool unavailable")
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
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("list user passkeys failed for user %s: %v", userID, err))
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
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := authContext.UserID

	passkeyID := strings.TrimSpace(request.PathValue("id"))
	if passkeyID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Passkey ID required")
		return
	}

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "delete user passkey rejected: database pool unavailable")
		return
	}

	ctx := request.Context()
	result, err := handler.db.Exec(ctx, `
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
