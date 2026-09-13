package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"layr.sh/auth/otp"
	"layr.sh/core"
)

func (handler *BaseHandler) handleUserEmailVerificationRequest(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling user email verification request")
	var userEmailVerificationRequest UserEmailVerificationRequest
	_ = json.NewDecoder(request.Body).Decode(&userEmailVerificationRequest)

	recipientEmail := strings.TrimSpace(strings.ToLower(userEmailVerificationRequest.Email))
	authUserID, authErr := handler.authenticateUser(request)

	if recipientEmail == "" {
		claims := handler.extractClaimsOptional(request)
		if claims != nil && claims.Email != "" {
			recipientEmail = strings.TrimSpace(strings.ToLower(claims.Email))
		}
	}

	if recipientEmail == "" || !strings.Contains(recipientEmail, "@") {
		log.Debugf("email verification request rejected: invalid recipient email %q", recipientEmail)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Email address is required", "LAYR_AUTH_001")
		return
	}

	if !handler.assertEmailDeliveryReady(responseWriter, request) {
		return
	}

	if handler.db == nil {
		log.Debug("email verification request rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var existingUserID string
	var emailVerifiedAt *time.Time
	err := handler.db.QueryRow(ctx, "SELECT id, email_verified_at FROM auth.users WHERE email = $1", recipientEmail).Scan(&existingUserID, &emailVerifiedAt)

	targetUserID := existingUserID
	if authErr == nil && authUserID != "" {
		if err == nil && existingUserID != authUserID {
			log.Debugf("email verification conflict: email %s already in use by user %s (caller: %s)", recipientEmail, existingUserID, authUserID)
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Email is already in use", "LAYR_AUTH_001")
			return
		}
		targetUserID = authUserID
	} else {
		if err != nil {
			log.Debugf("email verification request rejected: user not found for email %s: %v", recipientEmail, err)
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_002")
			return
		}
	}

	if emailVerifiedAt != nil && (authErr != nil || authUserID == existingUserID) {
		log.Debugf("email %s is already verified for user %s", recipientEmail, targetUserID)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"ok":        true,
			"status":    "already_verified",
			"message":   "Email is already verified",
			"recipient": recipientEmail,
		})
		return
	}

	code, _ := otp.GenerateCode(nil)
	codeHash := otp.HashCode(code)
	expiresAt := time.Now().UTC().Add(otp.CodeTTL)

	query := `
		INSERT INTO auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'email_verification', 0, $3, clock_timestamp())
	`
	_, _ = handler.db.Exec(ctx, query, recipientEmail, codeHash, expiresAt)
	if handler.kvStore != nil {
		_ = handler.kvStore.Set(ctx, fmt.Sprintf("auth:otp:email_verification:%s", recipientEmail), code, otp.CodeTTL)
	}

	log.Tracef("dispatching email verification code to %s", recipientEmail)
	_ = handler.emailDispatcher.SendEmailVerification(ctx, recipientEmail, code, targetUserID)

	if handler.eventBus != nil {
		userRecord, _ := fetchUserRecordByID(ctx, handler.db, targetUserID)
		handler.eventBus.Publish(ctx, NewOTPSentEvent(recipientEmail, OTPSentEventData{
			Recipient: recipientEmail,
			Purpose:   "email_verification",
			Channel:   "email",
			User:      &userRecord,
		}))
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *BaseHandler) handleUserEmailVerificationConfirm(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling user email verification confirmation")
	var userEmailVerificationConfirmRequest UserEmailVerificationConfirmRequest
	if err := json.NewDecoder(request.Body).Decode(&userEmailVerificationConfirmRequest); err != nil {
		log.Debugf("email verification confirmation rejected: invalid JSON body: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", "LAYR_AUTH_001")
		return
	}

	recipientEmail := strings.TrimSpace(strings.ToLower(userEmailVerificationConfirmRequest.Email))
	if recipientEmail == "" {
		claims := handler.extractClaimsOptional(request)
		if claims != nil && claims.Email != "" {
			recipientEmail = strings.TrimSpace(strings.ToLower(claims.Email))
		}
	}

	code := strings.TrimSpace(userEmailVerificationConfirmRequest.Code)
	if recipientEmail == "" || code == "" {
		log.Debug("email verification confirmation rejected: missing email or code")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Email and verification code are required", "LAYR_AUTH_001")
		return
	}

	if handler.db == nil {
		log.Debug("email verification confirmation rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var otpID string
	var storedHash string
	var attempts int
	var expiresAt time.Time

	query := `
		SELECT id, code_hash, attempts, expires_at 
		FROM auth.otps 
		WHERE recipient = $1 AND purpose = 'email_verification'
		ORDER BY created_at DESC 
		LIMIT 1
	`
	err := handler.db.QueryRow(ctx, query, recipientEmail).Scan(&otpID, &storedHash, &attempts, &expiresAt)
	if err != nil {
		log.Debugf("email verification confirmation failed: OTP not found for %s: %v", recipientEmail, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired verification code", "LAYR_AUTH_001")
		return
	}

	if time.Now().UTC().After(expiresAt) {
		log.Debugf("email verification confirmation failed: OTP expired for %s", recipientEmail)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Verification code has expired", "LAYR_AUTH_001")
		return
	}

	const maxOtpAttempts = 5
	if attempts >= maxOtpAttempts {
		log.Debugf("email verification confirmation failed: max attempts exceeded for %s", recipientEmail)
		_, _ = handler.db.Exec(ctx, "DELETE FROM auth.otps WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Maximum attempts exceeded", "LAYR_AUTH_001")
		return
	}

	if !otp.VerifyCode(code, storedHash) {
		log.Debugf("email verification confirmation failed: invalid code for %s", recipientEmail)
		_, _ = handler.db.Exec(ctx, "UPDATE auth.otps SET attempts = attempts + 1 WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid verification code", "LAYR_AUTH_001")
		return
	}

	_, _ = handler.db.Exec(ctx, "DELETE FROM auth.otps WHERE recipient = $1 AND purpose = 'email_verification'", recipientEmail)
	if handler.kvStore != nil {
		_ = handler.kvStore.Delete(ctx, fmt.Sprintf("auth:otp:email_verification:%s", recipientEmail))
	}

	authUserID, authErr := handler.authenticateUser(request)
	if authErr == nil && authUserID != "" {
		isCallerAnonymous := false
		_ = handler.db.QueryRow(ctx, "SELECT (email IS NULL AND phone IS NULL AND is_anonymous) FROM auth.users WHERE id = $1", authUserID).Scan(&isCallerAnonymous)

		var userRecord UserRecord
		var rawProperties []byte
		err = handler.db.QueryRow(ctx, `
			UPDATE auth.users 
			SET email = $1, email_verified_at = clock_timestamp(), is_anonymous = false, last_updated_at = clock_timestamp() 
			WHERE id = $2
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		`, recipientEmail, authUserID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
			&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
			&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
		)
		if err != nil {
			log.Debugf("failed to update user email verification for %s: %v", authUserID, err)
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to update email verification status", "LAYR_AUTH_001")
			return
		}

		userRecord.Properties = make(map[string]any)
		if len(rawProperties) > 0 {
			_ = json.Unmarshal(rawProperties, &userRecord.Properties)
		}

		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewUserEmailVerifiedEvent(userRecord.ID, UserEmailVerifiedEventData(userRecord)))
			handler.eventBus.Publish(ctx, NewUserUpdatedEvent(userRecord.ID, UserUpdatedEventData(userRecord)))
			if isCallerAnonymous {
				handler.eventBus.Publish(ctx, NewUserConvertedEvent(userRecord.ID, UserConvertedEventData(userRecord)))
			}
		}

		handler.issueSessionResponse(responseWriter, request, userRecord, "otp")
		return
	}

	var userRecord UserRecord
	var rawProperties []byte
	err = handler.db.QueryRow(ctx, `
		UPDATE auth.users 
		SET email_verified_at = clock_timestamp(), last_updated_at = clock_timestamp() 
		WHERE email = $1 
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`, recipientEmail).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("failed to update unauthenticated user email verification for %s: %v", recipientEmail, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to update email verification status", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewUserEmailVerifiedEvent(userRecord.ID, UserEmailVerifiedEventData(userRecord)))
		handler.eventBus.Publish(ctx, NewUserUpdatedEvent(userRecord.ID, UserUpdatedEventData(userRecord)))
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"ok":       true,
		"verified": true,
		"message":  "Email successfully verified",
	})
}

func (handler *BaseHandler) handleUserPhoneVerificationRequest(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling user phone verification request")
	var userPhoneVerificationRequest UserPhoneVerificationRequest
	_ = json.NewDecoder(request.Body).Decode(&userPhoneVerificationRequest)

	recipientPhone := strings.TrimSpace(userPhoneVerificationRequest.Phone)
	authUserID, authErr := handler.authenticateUser(request)

	if recipientPhone == "" {
		claims := handler.extractClaimsOptional(request)
		if claims != nil && claims.Phone != "" {
			recipientPhone = strings.TrimSpace(claims.Phone)
		}
	}

	if recipientPhone == "" {
		log.Debug("phone verification request rejected: missing phone number")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Phone number is required", "LAYR_AUTH_001")
		return
	}

	normalizedPhone, err := NormalizePhone(recipientPhone)
	if err != nil {
		log.Debugf("phone verification request rejected: invalid phone %q: %v", recipientPhone, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code", "LAYR_AUTH_INVALID_PHONE")
		return
	}
	recipientPhone = normalizedPhone

	if !handler.assertSMSDeliveryReady(responseWriter, request) {
		return
	}

	if handler.db == nil {
		log.Debug("phone verification request rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var existingUserID string
	var phoneVerifiedAt *time.Time
	err = handler.db.QueryRow(ctx, "SELECT id, phone_verified_at FROM auth.users WHERE phone = $1", recipientPhone).Scan(&existingUserID, &phoneVerifiedAt)

	targetUserID := existingUserID
	if authErr == nil && authUserID != "" {
		if err == nil && existingUserID != authUserID {
			log.Debugf("phone verification conflict: phone %s already in use by user %s (caller: %s)", recipientPhone, existingUserID, authUserID)
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Phone number is already in use", "LAYR_AUTH_001")
			return
		}
		targetUserID = authUserID
	} else {
		if err != nil {
			log.Debugf("phone verification request rejected: user not found for phone %s: %v", recipientPhone, err)
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_002")
			return
		}
	}

	if phoneVerifiedAt != nil && (authErr != nil || authUserID == existingUserID) {
		log.Debugf("phone %s is already verified for user %s", recipientPhone, targetUserID)
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"ok":        true,
			"status":    "already_verified",
			"message":   "Phone number is already verified",
			"recipient": recipientPhone,
		})
		return
	}

	code, _ := otp.GenerateCode(nil)
	codeHash := otp.HashCode(code)
	expiresAt := time.Now().UTC().Add(otp.CodeTTL)

	query := `
		INSERT INTO auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'phone_verification', 0, $3, clock_timestamp())
	`
	_, _ = handler.db.Exec(ctx, query, recipientPhone, codeHash, expiresAt)
	if handler.kvStore != nil {
		_ = handler.kvStore.Set(ctx, fmt.Sprintf("auth:otp:phone_verification:%s", recipientPhone), code, otp.CodeTTL)
	}

	log.Tracef("dispatching phone verification code to %s", recipientPhone)
	_ = handler.smsDispatcher.SendPhoneVerification(ctx, recipientPhone, code, targetUserID)

	if handler.eventBus != nil {
		userRecord, _ := fetchUserRecordByID(ctx, handler.db, targetUserID)
		handler.eventBus.Publish(ctx, NewOTPSentEvent(recipientPhone, OTPSentEventData{
			Recipient: recipientPhone,
			Purpose:   "phone_verification",
			Channel:   "sms",
			User:      &userRecord,
		}))
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *BaseHandler) handleUserPhoneVerificationConfirm(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling user phone verification confirmation")
	var userPhoneVerificationConfirmRequest UserPhoneVerificationConfirmRequest
	if err := json.NewDecoder(request.Body).Decode(&userPhoneVerificationConfirmRequest); err != nil {
		log.Debugf("phone verification confirmation rejected: invalid JSON body: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", "LAYR_AUTH_001")
		return
	}

	recipientPhone := strings.TrimSpace(userPhoneVerificationConfirmRequest.Phone)
	if recipientPhone == "" {
		claims := handler.extractClaimsOptional(request)
		if claims != nil && claims.Phone != "" {
			recipientPhone = strings.TrimSpace(claims.Phone)
		}
	}

	code := strings.TrimSpace(userPhoneVerificationConfirmRequest.Code)
	if recipientPhone == "" || code == "" {
		log.Debug("phone verification confirmation rejected: missing phone or code")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Phone number and verification code are required", "LAYR_AUTH_001")
		return
	}

	normalizedPhone, err := NormalizePhone(recipientPhone)
	if err != nil {
		log.Debugf("phone verification confirmation rejected: invalid phone %q: %v", recipientPhone, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code", "LAYR_AUTH_INVALID_PHONE")
		return
	}
	recipientPhone = normalizedPhone

	if handler.db == nil {
		log.Debug("phone verification confirmation rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var otpID string
	var storedHash string
	var attempts int
	var expiresAt time.Time

	query := `
		SELECT id, code_hash, attempts, expires_at 
		FROM auth.otps 
		WHERE recipient = $1 AND purpose = 'phone_verification'
		ORDER BY created_at DESC 
		LIMIT 1
	`
	err = handler.db.QueryRow(ctx, query, recipientPhone).Scan(&otpID, &storedHash, &attempts, &expiresAt)
	if err != nil {
		log.Debugf("phone verification confirmation failed: OTP not found for %s: %v", recipientPhone, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired verification code", "LAYR_AUTH_001")
		return
	}

	if time.Now().UTC().After(expiresAt) {
		log.Debugf("phone verification confirmation failed: OTP expired for %s", recipientPhone)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Verification code has expired", "LAYR_AUTH_001")
		return
	}

	const maxOtpAttempts = 5
	if attempts >= maxOtpAttempts {
		log.Debugf("phone verification confirmation failed: max attempts exceeded for %s", recipientPhone)
		_, _ = handler.db.Exec(ctx, "DELETE FROM auth.otps WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Maximum attempts exceeded", "LAYR_AUTH_001")
		return
	}

	if !otp.VerifyCode(code, storedHash) {
		log.Debugf("phone verification confirmation failed: invalid code for %s", recipientPhone)
		_, _ = handler.db.Exec(ctx, "UPDATE auth.otps SET attempts = attempts + 1 WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid verification code", "LAYR_AUTH_001")
		return
	}

	_, _ = handler.db.Exec(ctx, "DELETE FROM auth.otps WHERE recipient = $1 AND purpose = 'phone_verification'", recipientPhone)
	if handler.kvStore != nil {
		_ = handler.kvStore.Delete(ctx, fmt.Sprintf("auth:otp:phone_verification:%s", recipientPhone))
	}

	authUserID, authErr := handler.authenticateUser(request)
	if authErr == nil && authUserID != "" {
		isCallerAnonymous := false
		_ = handler.db.QueryRow(ctx, "SELECT (email IS NULL AND phone IS NULL AND is_anonymous) FROM auth.users WHERE id = $1", authUserID).Scan(&isCallerAnonymous)

		var userRecord UserRecord
		var rawProperties []byte
		err = handler.db.QueryRow(ctx, `
			UPDATE auth.users 
			SET phone = $1, phone_verified_at = clock_timestamp(), is_anonymous = false, last_updated_at = clock_timestamp() 
			WHERE id = $2
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		`, recipientPhone, authUserID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
			&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
			&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
		)
		if err != nil {
			log.Debugf("failed to update user phone verification for %s: %v", authUserID, err)
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to update phone verification status", "LAYR_AUTH_001")
			return
		}

		userRecord.Properties = make(map[string]any)
		if len(rawProperties) > 0 {
			_ = json.Unmarshal(rawProperties, &userRecord.Properties)
		}

		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewUserPhoneVerifiedEvent(userRecord.ID, UserPhoneVerifiedEventData(userRecord)))
			handler.eventBus.Publish(ctx, NewUserUpdatedEvent(userRecord.ID, UserUpdatedEventData(userRecord)))
			if isCallerAnonymous {
				handler.eventBus.Publish(ctx, NewUserConvertedEvent(userRecord.ID, UserConvertedEventData(userRecord)))
			}
		}

		handler.issueSessionResponse(responseWriter, request, userRecord, "otp")
		return
	}

	var userRecord UserRecord
	var rawProperties []byte
	err = handler.db.QueryRow(ctx, `
		UPDATE auth.users 
		SET phone_verified_at = clock_timestamp(), last_updated_at = clock_timestamp() 
		WHERE phone = $1 
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`, recipientPhone).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("failed to update unauthenticated user phone verification for %s: %v", recipientPhone, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to update phone verification status", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewUserPhoneVerifiedEvent(userRecord.ID, UserPhoneVerifiedEventData(userRecord)))
		handler.eventBus.Publish(ctx, NewUserUpdatedEvent(userRecord.ID, UserUpdatedEventData(userRecord)))
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"ok":       true,
		"verified": true,
		"message":  "Phone number successfully verified",
	})
}

func (handler *BaseHandler) handleUpdateUserEmail(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling update user email request")
	authUserID, authErr := handler.authenticateUser(request)
	if authErr != nil || authUserID == "" {
		log.Debugf("update user email rejected: unauthenticated caller: %v", authErr)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	var updateUpdateUserEmailRequest UpdateUserEmailRequest
	if err := json.NewDecoder(request.Body).Decode(&updateUpdateUserEmailRequest); err != nil {
		log.Debugf("update user email rejected: invalid JSON payload: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload", "LAYR_AUTH_001")
		return
	}

	recipientEmail := strings.TrimSpace(strings.ToLower(updateUpdateUserEmailRequest.Email))
	if recipientEmail == "" || !strings.Contains(recipientEmail, "@") {
		log.Debugf("update user email rejected: invalid email address %q", recipientEmail)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Valid email address is required", "LAYR_AUTH_001")
		return
	}

	if !handler.assertEmailDeliveryReady(responseWriter, request) {
		return
	}

	if handler.db == nil {
		log.Debug("update user email rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var existingUserID string
	err := handler.db.QueryRow(ctx, "SELECT id FROM auth.users WHERE email = $1", recipientEmail).Scan(&existingUserID)
	if err == nil && existingUserID != authUserID {
		log.Debugf("update user email conflict: email %s already in use by user %s", recipientEmail, existingUserID)
		core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Email is already in use by another account", "LAYR_AUTH_001")
		return
	}

	anonymousUserRecord, resolveErr := handler.resolveAnonymousCaller(request)
	if resolveErr != nil && !errors.Is(resolveErr, ErrAnonymousSessionNotFound) {
		log.Debugf("failed to resolve anonymous caller: %v", resolveErr)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	if anonymousUserRecord != nil {
		log.Debugf("converting anonymous user %s with email %s", anonymousUserRecord.ID, recipientEmail)
		_, updateErr := handler.db.Exec(ctx, `
			UPDATE auth.users
			SET email = $1, is_anonymous = false, last_updated_at = clock_timestamp()
			WHERE id = $2
		`, recipientEmail, anonymousUserRecord.ID)
		if updateErr != nil {
			log.Debugf("failed to update anonymous user email: %v", updateErr)
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to update user email", "LAYR_AUTH_001")
			return
		}

		anonymousUserRecord.Email = &recipientEmail
		anonymousUserRecord.IsAnonymous = false
		anonymousUserRecord.LastUpdatedAt = time.Now().UTC()
		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewUserConvertedEvent(anonymousUserRecord.ID, UserConvertedEventData(*anonymousUserRecord)))
		}
	}

	code, _ := otp.GenerateCode(nil)
	codeHash := otp.HashCode(code)
	expiresAt := time.Now().UTC().Add(otp.CodeTTL)

	query := `
		INSERT INTO auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'email_verification', 0, $3, clock_timestamp())
	`
	_, _ = handler.db.Exec(ctx, query, recipientEmail, codeHash, expiresAt)
	if handler.kvStore != nil {
		_ = handler.kvStore.Set(ctx, fmt.Sprintf("auth:otp:email_verification:%s", recipientEmail), code, otp.CodeTTL)
	}

	log.Tracef("dispatching update email verification code to %s", recipientEmail)
	_ = handler.emailDispatcher.SendEmailVerification(ctx, recipientEmail, code, authUserID)

	if handler.eventBus != nil {
		var targetUserRecord *UserRecord
		if anonymousUserRecord != nil {
			targetUserRecord = anonymousUserRecord
		} else if userRecord, err := fetchUserRecordByID(ctx, handler.db, authUserID); err == nil {
			targetUserRecord = &userRecord
		}
		handler.eventBus.Publish(ctx, NewOTPSentEvent(recipientEmail, OTPSentEventData{
			Recipient: recipientEmail,
			Purpose:   "email_verification",
			Channel:   "email",
			User:      targetUserRecord,
		}))
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *BaseHandler) handleUpdateUserPhone(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling update user phone request")
	authUserID, authErr := handler.authenticateUser(request)
	if authErr != nil || authUserID == "" {
		log.Debugf("update user phone rejected: unauthenticated caller: %v", authErr)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	var updateUpdateUserPhoneRequest UpdateUserPhoneRequest
	if err := json.NewDecoder(request.Body).Decode(&updateUpdateUserPhoneRequest); err != nil {
		log.Debugf("update user phone rejected: invalid JSON payload: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload", "LAYR_AUTH_001")
		return
	}

	recipientPhone := strings.TrimSpace(updateUpdateUserPhoneRequest.Phone)
	if recipientPhone == "" {
		log.Debug("update user phone rejected: empty phone number")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Phone number is required", "LAYR_AUTH_001")
		return
	}

	normalizedPhone, err := NormalizePhone(recipientPhone)
	if err != nil {
		log.Debugf("update user phone rejected: invalid phone %q: %v", recipientPhone, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code", "LAYR_AUTH_INVALID_PHONE")
		return
	}
	recipientPhone = normalizedPhone

	if !handler.assertSMSDeliveryReady(responseWriter, request) {
		return
	}

	if handler.db == nil {
		log.Debug("update user phone rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var existingUserID string
	err = handler.db.QueryRow(ctx, "SELECT id FROM auth.users WHERE phone = $1", recipientPhone).Scan(&existingUserID)
	if err == nil && existingUserID != authUserID {
		log.Debugf("update user phone conflict: phone %s already in use by user %s", recipientPhone, existingUserID)
		core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Phone number is already in use by another account", "LAYR_AUTH_001")
		return
	}

	anonymousUserRecord, resolveErr := handler.resolveAnonymousCaller(request)
	if resolveErr != nil && !errors.Is(resolveErr, ErrAnonymousSessionNotFound) {
		log.Debugf("failed to resolve anonymous caller: %v", resolveErr)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	if anonymousUserRecord != nil {
		log.Debugf("converting anonymous user %s with phone %s", anonymousUserRecord.ID, recipientPhone)
		_, updateErr := handler.db.Exec(ctx, `
			UPDATE auth.users
			SET phone = $1, is_anonymous = false, last_updated_at = clock_timestamp()
			WHERE id = $2
		`, recipientPhone, anonymousUserRecord.ID)
		if updateErr != nil {
			log.Debugf("failed to update anonymous user phone: %v", updateErr)
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to update user phone", "LAYR_AUTH_001")
			return
		}

		anonymousUserRecord.Phone = &recipientPhone
		anonymousUserRecord.IsAnonymous = false
		anonymousUserRecord.LastUpdatedAt = time.Now().UTC()
		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewUserConvertedEvent(anonymousUserRecord.ID, UserConvertedEventData(*anonymousUserRecord)))
		}
	}

	code, _ := otp.GenerateCode(nil)
	codeHash := otp.HashCode(code)
	expiresAt := time.Now().UTC().Add(otp.CodeTTL)

	query := `
		INSERT INTO auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'phone_verification', 0, $3, clock_timestamp())
	`
	_, _ = handler.db.Exec(ctx, query, recipientPhone, codeHash, expiresAt)
	if handler.kvStore != nil {
		_ = handler.kvStore.Set(ctx, fmt.Sprintf("auth:otp:phone_verification:%s", recipientPhone), code, otp.CodeTTL)
	}

	log.Tracef("dispatching update phone verification code to %s", recipientPhone)
	_ = handler.smsDispatcher.SendPhoneVerification(ctx, recipientPhone, code, authUserID)

	if handler.eventBus != nil {
		var targetUserRecord *UserRecord
		if anonymousUserRecord != nil {
			targetUserRecord = anonymousUserRecord
		} else if userRecord, err := fetchUserRecordByID(ctx, handler.db, authUserID); err == nil {
			targetUserRecord = &userRecord
		}
		handler.eventBus.Publish(ctx, NewOTPSentEvent(recipientPhone, OTPSentEventData{
			Recipient: recipientPhone,
			Purpose:   "phone_verification",
			Channel:   "sms",
			User:      targetUserRecord,
		}))
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}
