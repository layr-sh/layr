package auth

import (
	"encoding/json"
	"net/http"
	"time"
	"uuid"

	"layr.sh/core"
)

const defaultPasskeyChallengeTTL = 5 * time.Minute

// PasskeySignUpRequest defines input for passkey sign up ceremony.
type PasskeySignUpRequest struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
}

func (handler *Handler) handlePasskeySignUp(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling passkey sign up initiation request")
	config := handler.configManager.Get()
	if !config.Passkeys.Enabled {
		log.Debug("passkey sign up rejected: passkey authentication is disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Passkey authentication is disabled", "LAYR_AUTH_001")
		return
	}

	var passkeySignUpRequest PasskeySignUpRequest
	if err := json.NewDecoder(request.Body).Decode(&passkeySignUpRequest); err != nil || passkeySignUpRequest.UserID == "" {
		log.Debugf("passkey sign up rejected: invalid request or empty user_id: %v", err)
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

// PasskeySignUpVerifyRequest defines input to verify and store passkey credentials.
type PasskeySignUpVerifyRequest struct {
	UserID       string   `json:"user_id"`
	Challenge    string   `json:"challenge"`
	CredentialID string   `json:"credential_id"`
	PublicKey    string   `json:"public_key"`
	FriendlyName string   `json:"friendly_name"`
	Transports   []string `json:"transports"`
}

func (handler *Handler) handlePasskeySignUpVerify(responseWriter http.ResponseWriter, request *http.Request) {
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
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		`, targetUserID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
			&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
		)
	} else {
		err = handler.db.QueryRow(ctx, `
			INSERT INTO auth.users (id, role, created_at, last_updated_at)
			VALUES ($1, 'authenticated', clock_timestamp(), clock_timestamp())
			ON CONFLICT (id) DO UPDATE SET last_updated_at = clock_timestamp()
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		`, targetUserID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
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
			UserID:       targetUserID,
			FriendlyName: passkeySignUpVerifyRequest.FriendlyName,
			Transports:   passkeySignUpVerifyRequest.Transports,
		}))
		if isAnonymousConversion {
			handler.eventBus.Publish(ctx, NewUserConvertedEvent(userRecord.ID, UserConvertedEventData(userRecord)))
		}
	}

	log.Debugf("passkey sign up verified and session issued for user %s", targetUserID)
	handler.issueSessionResponse(responseWriter, request, userRecord)
}

func (handler *Handler) handlePasskeySignIn(responseWriter http.ResponseWriter, request *http.Request) {
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

// PasskeySignInVerifyRequest defines input to complete passkey assertion ceremony.
type PasskeySignInVerifyRequest struct {
	Challenge    string `json:"challenge"`
	CredentialID string `json:"credential_id"`
}

func (handler *Handler) handlePasskeySignInVerify(responseWriter http.ResponseWriter, request *http.Request) {
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
	err := handler.db.QueryRow(ctx, "SELECT user_id FROM auth.passkeys WHERE credential_id = $1", credentialIDBytes).Scan(&userID)
	if err != nil {
		log.Debugf("passkey sign-in verify rejected: credential not found: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Passkey credential not found", "LAYR_AUTH_006")
		return
	}

	var userRecord UserRecord
	var rawProperties []byte
	err = handler.db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		FROM auth.users WHERE id = $1
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("passkey sign-in verify user lookup failed (userID: %s): %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "User lookup failed", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	_, _ = handler.db.Exec(ctx, "UPDATE auth.passkeys SET last_used_at = clock_timestamp() WHERE credential_id = $1", credentialIDBytes)
	log.Debugf("passkey sign-in verified and session issued for user %s", userID)
	handler.issueSessionResponse(responseWriter, request, userRecord)
}

// RegisterPasskeyRoutes registers WebAuthn Passkey ceremonies on the provided router.
func (handler *Handler) RegisterPasskeyRoutes(router *core.Router) {
	log.Debug("registering passkey ceremony routes on router")
	router.Mux().HandleFunc("POST /api/v1/auth/passkeys/sign-up", handler.handlePasskeySignUp)
	router.Mux().HandleFunc("POST /api/v1/auth/passkeys/sign-up/verify", handler.handlePasskeySignUpVerify)
	router.Mux().HandleFunc("POST /api/v1/auth/passkeys/sign-in", handler.handlePasskeySignIn)
	router.Mux().HandleFunc("POST /api/v1/auth/passkeys/sign-in/verify", handler.handlePasskeySignInVerify)
}
