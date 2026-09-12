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

// UserEmailVerificationRequest represents the payload to request an email verification code.
type UserEmailVerificationRequest struct {
	Email string `json:"email"`
}

// UserEmailVerificationConfirmRequest represents the payload to confirm email verification with a code.
type UserEmailVerificationConfirmRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// UserPhoneVerificationRequest represents the payload to request a phone verification code.
type UserPhoneVerificationRequest struct {
	Phone string `json:"phone"`
}

// UserPhoneVerificationConfirmRequest represents the payload to confirm phone verification with a code.
type UserPhoneVerificationConfirmRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

// UserResponse represents the sanitized authenticated user details.
type UserResponse struct {
	ID            string         `json:"id"`
	Email         *string        `json:"email"`
	Phone         *string        `json:"phone"`
	Role          string         `json:"role"`
	IsAnonymous   bool           `json:"is_anonymous"`
	EmailVerified bool           `json:"email_verified"`
	PhoneVerified bool           `json:"phone_verified"`
	MFAEnabled    bool           `json:"mfa_enabled"`
	Properties    map[string]any `json:"properties"`
	CreatedAt     time.Time      `json:"created_at"`
	LastUpdatedAt time.Time      `json:"last_updated_at"`
}

// UpdateUserEmailRequest represents the payload to request updating user email.
type UpdateUserEmailRequest struct {
	Email string `json:"email"`
}

// UpdateUserPhoneRequest represents the payload to request updating user phone number.
type UpdateUserPhoneRequest struct {
	Phone string `json:"phone"`
}

// UpdateUserPropertiesRequest represents the payload to merge personal user properties.
type UpdateUserPropertiesRequest struct {
	Properties map[string]any `json:"properties"`
}

// UpdateUserPropertiesResponse represents the updated custom properties of the authenticated user.
type UpdateUserPropertiesResponse struct {
	Properties map[string]any `json:"properties"`
}

// UpdateUserPasswordRequest represents the payload to set or change an account password.
type UpdateUserPasswordRequest struct {
	CurrentPassword string `json:"current_password,omitempty"`
	NewPassword     string `json:"new_password"`
}

func sanitizeUserProperties(properties map[string]any) map[string]any {
	cleanedProperties := make(map[string]any)
	for propertyKey, propertyValue := range properties {
		if propertyKey == "mfa_secret_enc" || propertyKey == "mfa_pending" || propertyKey == "mfa_enabled" {
			continue
		}
		cleanedProperties[propertyKey] = propertyValue
	}
	return cleanedProperties
}

func (handler *Handler) handleUserEmailVerificationRequest(responseWriter http.ResponseWriter, request *http.Request) {
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
	err := handler.db.QueryRow(ctx, "SELECT id, email_verified_at FROM layr_auth.users WHERE email = $1", recipientEmail).Scan(&existingUserID, &emailVerifiedAt)

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
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'email_verification', 0, $3, clock_timestamp())
	`
	_, _ = handler.db.Exec(ctx, query, recipientEmail, codeHash, expiresAt)
	if handler.kvStore != nil {
		_ = handler.kvStore.Set(ctx, fmt.Sprintf("auth:otp:email_verification:%s", recipientEmail), code, otp.CodeTTL)
	}

	log.Tracef("dispatching email verification code to %s", recipientEmail)
	_ = handler.emailDispatcher.SendEmailVerification(ctx, recipientEmail, code, targetUserID)

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewOTPSentEvent(targetUserID, OTPSentEventData{
			UserID:    targetUserID,
			Recipient: recipientEmail,
			Purpose:   "email_verification",
		}))
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) handleUserEmailVerificationConfirm(responseWriter http.ResponseWriter, request *http.Request) {
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
		FROM layr_auth.otps 
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
		_, _ = handler.db.Exec(ctx, "DELETE FROM layr_auth.otps WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Maximum attempts exceeded", "LAYR_AUTH_001")
		return
	}

	if !otp.VerifyCode(code, storedHash) {
		log.Debugf("email verification confirmation failed: invalid code for %s", recipientEmail)
		_, _ = handler.db.Exec(ctx, "UPDATE layr_auth.otps SET attempts = attempts + 1 WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid verification code", "LAYR_AUTH_001")
		return
	}

	_, _ = handler.db.Exec(ctx, "DELETE FROM layr_auth.otps WHERE recipient = $1 AND purpose = 'email_verification'", recipientEmail)
	if handler.kvStore != nil {
		_ = handler.kvStore.Delete(ctx, fmt.Sprintf("auth:otp:email_verification:%s", recipientEmail))
	}

	authUserID, authErr := handler.authenticateUser(request)
	if authErr == nil && authUserID != "" {
		isCallerAnonymous := false
		_ = handler.db.QueryRow(ctx, "SELECT (email IS NULL AND phone IS NULL AND is_anonymous) FROM layr_auth.users WHERE id = $1", authUserID).Scan(&isCallerAnonymous)

		var userRecord UserRecord
		var rawProperties []byte
		err = handler.db.QueryRow(ctx, `
			UPDATE layr_auth.users 
			SET email = $1, email_verified_at = clock_timestamp(), is_anonymous = false, last_updated_at = clock_timestamp() 
			WHERE id = $2
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		`, recipientEmail, authUserID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
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
			if isCallerAnonymous {
				handler.eventBus.Publish(ctx, NewUserConvertedEvent(userRecord.ID, UserConvertedEventData(userRecord)))
			}
		}

		handler.issueSessionResponse(responseWriter, request, userRecord)
		return
	}

	var userRecord UserRecord
	var rawProperties []byte
	err = handler.db.QueryRow(ctx, `
		UPDATE layr_auth.users 
		SET email_verified_at = clock_timestamp(), last_updated_at = clock_timestamp() 
		WHERE email = $1 
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
	`, recipientEmail).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
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
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"ok":       true,
		"verified": true,
		"message":  "Email successfully verified",
	})
}

func (handler *Handler) handleUserPhoneVerificationRequest(responseWriter http.ResponseWriter, request *http.Request) {
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
	err = handler.db.QueryRow(ctx, "SELECT id, phone_verified_at FROM layr_auth.users WHERE phone = $1", recipientPhone).Scan(&existingUserID, &phoneVerifiedAt)

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
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'phone_verification', 0, $3, clock_timestamp())
	`
	_, _ = handler.db.Exec(ctx, query, recipientPhone, codeHash, expiresAt)
	if handler.kvStore != nil {
		_ = handler.kvStore.Set(ctx, fmt.Sprintf("auth:otp:phone_verification:%s", recipientPhone), code, otp.CodeTTL)
	}

	log.Tracef("dispatching phone verification code to %s", recipientPhone)
	_ = handler.smsDispatcher.SendPhoneVerification(ctx, recipientPhone, code, targetUserID)

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewOTPSentEvent(targetUserID, OTPSentEventData{
			UserID:    targetUserID,
			Recipient: recipientPhone,
			Purpose:   "phone_verification",
		}))
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) handleUserPhoneVerificationConfirm(responseWriter http.ResponseWriter, request *http.Request) {
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
		FROM layr_auth.otps 
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
		_, _ = handler.db.Exec(ctx, "DELETE FROM layr_auth.otps WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Maximum attempts exceeded", "LAYR_AUTH_001")
		return
	}

	if !otp.VerifyCode(code, storedHash) {
		log.Debugf("phone verification confirmation failed: invalid code for %s", recipientPhone)
		_, _ = handler.db.Exec(ctx, "UPDATE layr_auth.otps SET attempts = attempts + 1 WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid verification code", "LAYR_AUTH_001")
		return
	}

	_, _ = handler.db.Exec(ctx, "DELETE FROM layr_auth.otps WHERE recipient = $1 AND purpose = 'phone_verification'", recipientPhone)
	if handler.kvStore != nil {
		_ = handler.kvStore.Delete(ctx, fmt.Sprintf("auth:otp:phone_verification:%s", recipientPhone))
	}

	authUserID, authErr := handler.authenticateUser(request)
	if authErr == nil && authUserID != "" {
		isCallerAnonymous := false
		_ = handler.db.QueryRow(ctx, "SELECT (email IS NULL AND phone IS NULL AND is_anonymous) FROM layr_auth.users WHERE id = $1", authUserID).Scan(&isCallerAnonymous)

		var userRecord UserRecord
		var rawProperties []byte
		err = handler.db.QueryRow(ctx, `
			UPDATE layr_auth.users 
			SET phone = $1, phone_verified_at = clock_timestamp(), is_anonymous = false, last_updated_at = clock_timestamp() 
			WHERE id = $2
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		`, recipientPhone, authUserID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
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
			if isCallerAnonymous {
				handler.eventBus.Publish(ctx, NewUserConvertedEvent(userRecord.ID, UserConvertedEventData(userRecord)))
			}
		}

		handler.issueSessionResponse(responseWriter, request, userRecord)
		return
	}

	var userRecord UserRecord
	var rawProperties []byte
	err = handler.db.QueryRow(ctx, `
		UPDATE layr_auth.users 
		SET phone_verified_at = clock_timestamp(), last_updated_at = clock_timestamp() 
		WHERE phone = $1 
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
	`, recipientPhone).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
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
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(map[string]any{
		"ok":       true,
		"verified": true,
		"message":  "Phone number successfully verified",
	})
}

func (handler *Handler) handleGetUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling get user profile request")
	userID, err := handler.authenticateUser(request)
	if err != nil {
		log.Debugf("get user profile rejected: unauthenticated caller: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	if handler.db == nil {
		log.Debug("get user profile rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	err = handler.db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		FROM layr_auth.users
		WHERE id = $1
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("get user profile failed: user %s not found: %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	isMFAEnabled := false
	if enabledValue, ok := userRecord.Properties["mfa_enabled"]; ok {
		if boolValue, boolOk := enabledValue.(bool); boolOk && boolValue {
			isMFAEnabled = true
		} else if stringValue, isString := enabledValue.(string); isString && stringValue == "true" {
			isMFAEnabled = true
		}
	} else if secretValue, hasSecret := userRecord.Properties["mfa_secret_enc"]; hasSecret && secretValue != nil && secretValue != "" {
		if pendingValue, hasPending := userRecord.Properties["mfa_pending"]; !hasPending || pendingValue == false || pendingValue == "false" {
			isMFAEnabled = true
		}
	}

	if !isMFAEnabled {
		var hasPasskey bool
		err = handler.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM layr_auth.passkeys WHERE user_id = $1)", userID).Scan(&hasPasskey)
		if err == nil && hasPasskey {
			isMFAEnabled = true
		}
	}

	cleanedProperties := sanitizeUserProperties(userRecord.Properties)

	userResponse := UserResponse{
		ID:            userRecord.ID,
		Email:         userRecord.Email,
		Phone:         userRecord.Phone,
		Role:          userRecord.Role,
		IsAnonymous:   userRecord.IsAnonymous,
		EmailVerified: userRecord.EmailVerifiedAt != nil,
		PhoneVerified: userRecord.PhoneVerifiedAt != nil,
		MFAEnabled:    isMFAEnabled,
		Properties:    cleanedProperties,
		CreatedAt:     userRecord.CreatedAt,
		LastUpdatedAt: userRecord.LastUpdatedAt,
	}

	log.Debugf("user profile successfully retrieved for %s", userID)
	handler.writeJSON(responseWriter, userResponse)
}

func (handler *Handler) handleUpdateUserProperties(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling update user properties request")
	userID, err := handler.authenticateUser(request)
	if err != nil {
		log.Debugf("update user properties rejected: unauthenticated caller: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	var updateUpdateUserPropertiesRequest UpdateUserPropertiesRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&updateUpdateUserPropertiesRequest); decodeErr != nil {
		log.Debugf("update user properties rejected: invalid JSON payload: %v", decodeErr)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload", "LAYR_AUTH_001")
		return
	}

	if handler.db == nil {
		log.Debug("update user properties rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	cleanedInputProperties := sanitizeUserProperties(updateUpdateUserPropertiesRequest.Properties)
	propertiesJSON, _ := json.Marshal(cleanedInputProperties)

	ctx := request.Context()
	var userRecord UserRecord
	var rawProperties []byte
	err = handler.db.QueryRow(ctx, `
		UPDATE layr_auth.users
		SET properties = COALESCE(properties, '{}'::jsonb) || $1::jsonb,
		    last_updated_at = clock_timestamp()
		WHERE id = $2
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
	`, propertiesJSON, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("failed to update user properties for %s: %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to update user properties", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewUserUpdatedEvent(userRecord.ID, UserUpdatedEventData(userRecord)))
	}

	cleanedProperties := sanitizeUserProperties(userRecord.Properties)

	log.Debugf("user properties successfully updated for %s", userID)
	handler.writeJSON(responseWriter, UpdateUserPropertiesResponse{
		Properties: cleanedProperties,
	})
}

func (handler *Handler) handleUpdateUserEmail(responseWriter http.ResponseWriter, request *http.Request) {
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
	err := handler.db.QueryRow(ctx, "SELECT id FROM layr_auth.users WHERE email = $1", recipientEmail).Scan(&existingUserID)
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
			UPDATE layr_auth.users
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
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'email_verification', 0, $3, clock_timestamp())
	`
	_, _ = handler.db.Exec(ctx, query, recipientEmail, codeHash, expiresAt)
	if handler.kvStore != nil {
		_ = handler.kvStore.Set(ctx, fmt.Sprintf("auth:otp:email_verification:%s", recipientEmail), code, otp.CodeTTL)
	}

	log.Tracef("dispatching update email verification code to %s", recipientEmail)
	_ = handler.emailDispatcher.SendEmailVerification(ctx, recipientEmail, code, authUserID)

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewOTPSentEvent(authUserID, OTPSentEventData{
			UserID:    authUserID,
			Recipient: recipientEmail,
			Purpose:   "email_verification",
		}))
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) handleUpdateUserPhone(responseWriter http.ResponseWriter, request *http.Request) {
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
	err = handler.db.QueryRow(ctx, "SELECT id FROM layr_auth.users WHERE phone = $1", recipientPhone).Scan(&existingUserID)
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
			UPDATE layr_auth.users
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
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'phone_verification', 0, $3, clock_timestamp())
	`
	_, _ = handler.db.Exec(ctx, query, recipientPhone, codeHash, expiresAt)
	if handler.kvStore != nil {
		_ = handler.kvStore.Set(ctx, fmt.Sprintf("auth:otp:phone_verification:%s", recipientPhone), code, otp.CodeTTL)
	}

	log.Tracef("dispatching update phone verification code to %s", recipientPhone)
	_ = handler.smsDispatcher.SendPhoneVerification(ctx, recipientPhone, code, authUserID)

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewOTPSentEvent(authUserID, OTPSentEventData{
			UserID:    authUserID,
			Recipient: recipientPhone,
			Purpose:   "phone_verification",
		}))
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) handleUpdateUserPassword(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling update user password request")
	userID, err := handler.authenticateUser(request)
	if err != nil {
		log.Debugf("update user password rejected: unauthenticated caller: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	var updateUserPasswordRequest UpdateUserPasswordRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&updateUserPasswordRequest); decodeErr != nil {
		log.Debugf("update user password rejected: invalid JSON payload: %v", decodeErr)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload", "LAYR_AUTH_001")
		return
	}

	if handler.db == nil {
		log.Debug("update user password rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var isCallerAnonymous bool
	var email *string
	var phone *string
	var existingPasswordHash *string

	err = handler.db.QueryRow(ctx, `
		SELECT is_anonymous, email, phone, password_hash
		FROM layr_auth.users
		WHERE id = $1
	`, userID).Scan(&isCallerAnonymous, &email, &phone, &existingPasswordHash)
	if err != nil {
		log.Debugf("update user password failed: user %s not found: %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_001")
		return
	}

	if isCallerAnonymous || (email == nil && phone == nil) {
		log.Debugf("update user password rejected: anonymous or identifier-less account %s", userID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot set password on an account without a registered email or phone number", "LAYR_AUTH_001")
		return
	}

	if existingPasswordHash != nil && *existingPasswordHash != "" {
		if updateUserPasswordRequest.CurrentPassword == "" {
			log.Debug("update user password rejected: missing current password")
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Current password is required", "LAYR_AUTH_001")
			return
		}
		isCurrentPasswordValid, verifyErr := handler.hasher.Verify(updateUserPasswordRequest.CurrentPassword, *existingPasswordHash)
		if verifyErr != nil || !isCurrentPasswordValid {
			log.Debugf("update user password rejected: incorrect current password for %s: %v", userID, verifyErr)
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Current password is incorrect", "LAYR_AUTH_001")
			return
		}
	}

	config := handler.configManager.Get()
	if len(updateUserPasswordRequest.NewPassword) < config.Password.MinLength {
		log.Debugf("update user password rejected: password shorter than min length (%d)", config.Password.MinLength)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Password must be at least %d characters", config.Password.MinLength), "LAYR_AUTH_001")
		return
	}

	hashedPassword, _ := handler.hasher.Hash(updateUserPasswordRequest.NewPassword)

	var userRecord UserRecord
	var rawProperties []byte
	queryErr := handler.db.QueryRow(ctx, `
		UPDATE layr_auth.users
		SET password_hash = $1, last_updated_at = clock_timestamp()
		WHERE id = $2
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
	`, hashedPassword, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if queryErr == nil {
		userRecord.Properties = make(map[string]any)
		if len(rawProperties) > 0 {
			_ = json.Unmarshal(rawProperties, &userRecord.Properties)
		}
		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewPasswordChangedEvent(userRecord.ID, PasswordChangedEventData{
				User: userRecord,
			}))
		}
	}

	log.Debugf("user password successfully updated for %s", userID)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) handleDeleteUser(responseWriter http.ResponseWriter, request *http.Request) {
	log.Debug("handling delete user account request")
	userID, err := handler.authenticateUser(request)
	if err != nil {
		log.Debugf("delete user account rejected: unauthenticated caller: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required", "LAYR_AUTH_002")
		return
	}

	if handler.db == nil {
		log.Debug("delete user account rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()

	var userRecord UserRecord
	var rawProperties []byte
	_ = handler.db.QueryRow(ctx, `
		SELECT id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at 
		FROM layr_auth.users WHERE id = $1
	`, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	sessionRows, err := handler.db.Query(ctx, "SELECT refresh_token_hash FROM layr_auth.sessions WHERE user_id = $1", userID)
	if err == nil {
		for sessionRows.Next() {
			var refreshTokenHash string
			if scanErr := sessionRows.Scan(&refreshTokenHash); scanErr == nil && handler.kvStore != nil {
				_ = handler.kvStore.Delete(ctx, "auth:session:"+refreshTokenHash)
			}
		}
		sessionRows.Close()
	}

	_, err = handler.db.Exec(ctx, "DELETE FROM layr_auth.users WHERE id = $1", userID)
	if err != nil {
		log.Debugf("failed to delete user account %s: %v", userID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to delete user account", "LAYR_AUTH_001")
		return
	}

	if userRecord.Email != nil && *userRecord.Email != "" {
		_, _ = handler.db.Exec(ctx, "DELETE FROM layr_auth.otps WHERE recipient = $1", *userRecord.Email)
	}
	if userRecord.Phone != nil && *userRecord.Phone != "" {
		_, _ = handler.db.Exec(ctx, "DELETE FROM layr_auth.otps WHERE recipient = $1", *userRecord.Phone)
	}

	isSecure := core.IsSecureRequest(request)
	core.ClearSessionCookie(responseWriter, AuthSessionCookieName, AuthSessionInsecureCookieName, isSecure)

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewUserDeletedEvent(userRecord.ID, UserDeletedEventData(userRecord)))
	}

	log.Debugf("user account %s deleted successfully", userID)
	responseWriter.WriteHeader(http.StatusNoContent)
}

// RegisterUserRoutes registers end-user self-service endpoints on the provided router.
func (handler *Handler) RegisterUserRoutes(router *core.Router) {
	log.Debug("registering user self-service routes on router")
	router.Mux().HandleFunc("GET /api/v1/auth/user", handler.handleGetUser)
	router.Mux().HandleFunc("PATCH /api/v1/auth/user/properties", handler.handleUpdateUserProperties)
	router.Mux().HandleFunc("PATCH /api/v1/auth/user/email", handler.handleUpdateUserEmail)
	router.Mux().HandleFunc("POST /api/v1/auth/user/email/verification/request", handler.handleUserEmailVerificationRequest)
	router.Mux().HandleFunc("POST /api/v1/auth/user/email/verification/confirm", handler.handleUserEmailVerificationConfirm)
	router.Mux().HandleFunc("PATCH /api/v1/auth/user/phone", handler.handleUpdateUserPhone)
	router.Mux().HandleFunc("POST /api/v1/auth/user/phone/verification/request", handler.handleUserPhoneVerificationRequest)
	router.Mux().HandleFunc("POST /api/v1/auth/user/phone/verification/confirm", handler.handleUserPhoneVerificationConfirm)
	router.Mux().HandleFunc("PATCH /api/v1/auth/user/password", handler.handleUpdateUserPassword)
	router.Mux().HandleFunc("DELETE /api/v1/auth/user", handler.handleDeleteUser)
}
