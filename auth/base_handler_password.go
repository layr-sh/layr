package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"layr.sh/auth/otp"
	"layr.sh/core"
)

const (
	maxPasswordResetAttempts = 5
	ipPasswordResetLimit     = 10
	passwordResetCooldownTTL = 60 * time.Second
)

func (handler *BaseHandler) handleSignUp(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling sign-up request")
	config := handler.configManager.Get()
	if !config.Password.Enabled {
		log.Debug("sign-up rejected: password registration disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Password registration is disabled", "LAYR_AUTH_001")
		return
	}

	var signUpRequest SignUpRequest
	if err := json.NewDecoder(request.Body).Decode(&signUpRequest); err != nil {
		log.Debugf("sign-up rejected: invalid JSON body: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", "LAYR_AUTH_001")
		return
	}

	signUpRequest.Email = strings.TrimSpace(strings.ToLower(signUpRequest.Email))
	signUpRequest.Phone = strings.TrimSpace(signUpRequest.Phone)
	if signUpRequest.Phone != "" {
		normalizedPhone, err := NormalizePhone(signUpRequest.Phone)
		if err != nil {
			log.Debugf("sign-up rejected: invalid phone format: %v", err)
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code", "LAYR_AUTH_INVALID_PHONE")
			return
		}
		signUpRequest.Phone = normalizedPhone
	}
	if signUpRequest.Email == "" && signUpRequest.Phone == "" {
		log.Debug("sign-up rejected: email or phone number required")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Email or phone number is required", "LAYR_AUTH_001")
		return
	}
	if len(signUpRequest.Password) < config.Password.MinLength {
		log.Debugf("sign-up rejected: password length %d below minimum %d", len(signUpRequest.Password), config.Password.MinLength)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Password must be at least %d characters", config.Password.MinLength), "LAYR_AUTH_001")
		return
	}

	passHash, _ := handler.hasher.Hash(signUpRequest.Password)

	inputProperties := signUpRequest.Properties
	if inputProperties == nil {
		inputProperties = make(map[string]any)
	}
	cleanedProperties := sanitizeUserProperties(inputProperties)
	propertiesJSON, _ := json.Marshal(cleanedProperties)

	if handler.db == nil {
		log.Debug("sign-up rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var emailPtr, phonePtr *string
	if signUpRequest.Email != "" {
		emailPtr = &signUpRequest.Email
	}
	if signUpRequest.Phone != "" {
		phonePtr = &signUpRequest.Phone
	}

	anonymousUserRecord, _ := handler.resolveAnonymousCaller(request)
	if anonymousUserRecord != nil {
		var existingUserID string
		err := handler.db.QueryRow(ctx, `
			SELECT id FROM auth.users 
			WHERE (email IS NOT NULL AND email = $1) 
			   OR (phone IS NOT NULL AND phone = $2)
			LIMIT 1
		`, emailPtr, phonePtr).Scan(&existingUserID)
		if err == nil && existingUserID != anonymousUserRecord.ID {
			log.Debugf("sign-up conversion conflict: email or phone already registered by user %s", existingUserID)
			if emailPtr != nil {
				core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Email is already in use by another account", "LAYR_AUTH_001")
			} else {
				core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Phone number is already in use by another account", "LAYR_AUTH_001")
			}
			return
		}

		var userRecord UserRecord
		var rawProperties []byte
		updateQuery := `
			UPDATE auth.users
			SET email = $1, phone = $2, password_hash = $3, is_anonymous = false,
			    properties = COALESCE(properties, '{}'::jsonb) || $4::jsonb,
			    last_updated_at = clock_timestamp()
			WHERE id = $5
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		`
		err = handler.db.QueryRow(ctx, updateQuery, emailPtr, phonePtr, passHash, propertiesJSON, anonymousUserRecord.ID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
			&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
		)
		if err != nil {
			log.Debugf("failed to convert anonymous user %s: %v", anonymousUserRecord.ID, err)
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to convert user", "LAYR_AUTH_001")
			return
		}

		userRecord.Properties = make(map[string]any)
		if len(rawProperties) > 0 {
			_ = json.Unmarshal(rawProperties, &userRecord.Properties)
		}

		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewUserConvertedEvent(userRecord.ID, UserConvertedEventData(userRecord)))
		}

		handler.issueSessionResponse(responseWriter, request, userRecord)
		return
	}

	var userRecord UserRecord
	var rawProperties []byte
	query := `
		INSERT INTO auth.users (email, phone, password_hash, role, properties, created_at, last_updated_at)
		VALUES ($1, $2, $3, 'authenticated', $4, clock_timestamp(), clock_timestamp())
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
	`
	err := handler.db.QueryRow(ctx, query, emailPtr, phonePtr, passHash, propertiesJSON).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("failed to create user: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to create user", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewUserSignedUpEvent(userRecord.ID, UserSignedUpEventData(userRecord)))
	}

	handler.issueSessionResponse(responseWriter, request, userRecord)
}

func (handler *BaseHandler) handleSignIn(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling sign-in request")
	var signInRequest SignInRequest
	if err := json.NewDecoder(request.Body).Decode(&signInRequest); err != nil {
		log.Debugf("sign-in rejected: invalid JSON body: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", "LAYR_AUTH_001")
		return
	}

	identifier := strings.TrimSpace(strings.ToLower(signInRequest.Email))
	if identifier == "" && signInRequest.Phone != "" {
		normalizedPhone, err := NormalizePhone(signInRequest.Phone)
		if err != nil {
			log.Debugf("sign-in rejected: invalid phone format: %v", err)
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code", "LAYR_AUTH_INVALID_PHONE")
			return
		}
		identifier = normalizedPhone
	}
	if identifier == "" || signInRequest.Password == "" {
		log.Debug("sign-in rejected: missing credentials")
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	config := handler.configManager.Get()
	if config.RateLimiting.Enabled && handler.kvStore != nil && identifier != "" {
		rateKey := fmt.Sprintf("auth:ratelimit:signin:%s", identifier)
		windowDuration := time.Duration(config.RateLimiting.WindowDurationSeconds) * time.Second
		if count, err := handler.kvStore.Increment(ctx, rateKey, windowDuration); err == nil && count > int64(config.RateLimiting.MaxSigninAttempts) {
			log.Debugf("sign-in rejected: rate limit exceeded for identifier %s (count: %d)", identifier, count)
			core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Too many login attempts. Please try again later.", "LAYR_AUTH_005")
			return
		}
	}

	if handler.db == nil {
		log.Debug("sign-in rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	var userRecord UserRecord
	var passHash string
	var rawProperties []byte

	query := `
		SELECT id, email, phone, password_hash, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
		FROM auth.users
		WHERE email = $1 OR phone = $1
	`
	err := handler.db.QueryRow(ctx, query, identifier).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &passHash, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("sign-in rejected: user not found for identifier %s: %v", identifier, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials", "LAYR_AUTH_001")
		return
	}

	if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
		log.Debugf("sign-in rejected: account locked until %v for user %s", *userRecord.LockedUntil, userRecord.ID)
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked", "LAYR_AUTH_005")
		return
	}

	ok, err := handler.hasher.Verify(signInRequest.Password, passHash)
	if err != nil || !ok {
		log.Debugf("sign-in rejected: password mismatch for user %s: %v", userRecord.ID, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials", "LAYR_AUTH_001")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	// Reset rate limit on successful authentication
	if handler.kvStore != nil && identifier != "" {
		rateKey := fmt.Sprintf("auth:ratelimit:signin:%s", identifier)
		_ = handler.kvStore.Delete(ctx, rateKey)
	}

	handler.issueSessionResponse(responseWriter, request, userRecord)
}

func (handler *BaseHandler) handlePasswordResetRequest(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling password reset request")
	config := handler.configManager.Get()
	if !config.Password.Enabled {
		log.Debug("password reset rejected: password authentication is disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Password authentication is disabled", "LAYR_AUTH_001")
		return
	}

	var passwordResetRequest PasswordResetRequest
	if err := json.NewDecoder(request.Body).Decode(&passwordResetRequest); err != nil {
		log.Debugf("password reset request rejected: invalid JSON body: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", "LAYR_AUTH_001")
		return
	}

	recipient := strings.TrimSpace(strings.ToLower(passwordResetRequest.Recipient))
	if recipient == "" {
		recipient = strings.TrimSpace(strings.ToLower(passwordResetRequest.Email))
	}
	if recipient == "" {
		recipient = strings.TrimSpace(passwordResetRequest.Phone)
	}
	if recipient == "" {
		log.Debug("password reset request rejected: missing recipient")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Email or phone number is required", "LAYR_AUTH_001")
		return
	}

	isEmail := strings.Contains(recipient, "@")
	if isEmail {
		if !handler.assertEmailDeliveryReady(responseWriter, request) {
			return
		}
	} else {
		if !handler.assertSMSDeliveryReady(responseWriter, request) {
			return
		}
		normalizedPhone, err := NormalizePhone(recipient)
		if err != nil {
			log.Debugf("password reset request rejected: invalid phone number %q: %v", recipient, err)
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code", "LAYR_AUTH_INVALID_PHONE")
			return
		}
		recipient = normalizedPhone
	}

	ctx := request.Context()

	if handler.kvStore != nil {
		clientIP := core.ExtractRequestClientIP(request)
		ipRateKey := fmt.Sprintf("auth:ratelimit:otp:ip:%s", clientIP)
		if count, err := handler.kvStore.Increment(ctx, ipRateKey, time.Hour); err == nil && count > ipPasswordResetLimit {
			log.Debugf("password reset request rejected: IP rate limit exceeded for %s", clientIP)
			core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Rate limit exceeded. Too many requests from this IP address.", "LAYR_AUTH_RATE_LIMIT_EXCEEDED")
			return
		}

		cooldownKey := fmt.Sprintf("auth:cooldown:password_reset:%s", recipient)
		if _, err := handler.kvStore.Get(ctx, cooldownKey); err == nil {
			log.Debugf("password reset request rejected: cooldown active for recipient %s", recipient)
			core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Please wait 60 seconds before requesting another code", "LAYR_AUTH_COOLDOWN")
			return
		}
	}

	if handler.db == nil {
		log.Debug("password reset request rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	var userID string
	err := handler.db.QueryRow(ctx, "SELECT id FROM auth.users WHERE email = $1 OR phone = $1", recipient).Scan(&userID)
	if err != nil {
		log.Debugf("password reset request rejected: user not found for recipient %s: %v", recipient, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_002")
		return
	}

	code, _ := otp.GenerateCode(nil)
	codeHash := otp.HashCode(code)
	expiresAt := time.Now().UTC().Add(otp.CodeTTL)

	query := `
		INSERT INTO auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'password_reset', 0, $3, clock_timestamp())
	`
	_, _ = handler.db.Exec(ctx, query, recipient, codeHash, expiresAt)
	if handler.kvStore != nil {
		_ = handler.kvStore.Set(ctx, fmt.Sprintf("auth:otp:password_reset:%s", recipient), code, otp.CodeTTL)
		_ = handler.kvStore.Set(ctx, fmt.Sprintf("auth:cooldown:password_reset:%s", recipient), "1", passwordResetCooldownTTL)
	}

	if isEmail {
		_ = handler.emailDispatcher.SendPasswordReset(ctx, recipient, code, userID)
	} else {
		_ = handler.smsDispatcher.SendPasswordReset(ctx, recipient, code, userID)
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewPasswordResetRequestedEvent(userID, PasswordResetRequestedEventData{
			UserID:    userID,
			Recipient: recipient,
		}))
	}

	log.Debugf("password reset code dispatched to %s", recipient)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *BaseHandler) handlePasswordResetConfirm(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling password reset confirmation request")
	config := handler.configManager.Get()
	if !config.Password.Enabled {
		log.Debug("password reset confirmation rejected: password authentication is disabled")
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Password authentication is disabled", "LAYR_AUTH_001")
		return
	}

	var passwordResetConfirmRequest PasswordResetConfirmRequest
	if err := json.NewDecoder(request.Body).Decode(&passwordResetConfirmRequest); err != nil {
		log.Debugf("password reset confirmation rejected: invalid JSON body: %v", err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body", "LAYR_AUTH_001")
		return
	}

	recipient := strings.TrimSpace(strings.ToLower(passwordResetConfirmRequest.Recipient))
	if recipient == "" {
		recipient = strings.TrimSpace(strings.ToLower(passwordResetConfirmRequest.Email))
	}
	if recipient == "" {
		recipient = strings.TrimSpace(passwordResetConfirmRequest.Phone)
	}
	if recipient == "" || passwordResetConfirmRequest.Code == "" || passwordResetConfirmRequest.Password == "" {
		log.Debug("password reset confirmation rejected: missing required fields")
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Recipient, code, and new password are required", "LAYR_AUTH_001")
		return
	}

	if !strings.Contains(recipient, "@") {
		normalizedPhone, err := NormalizePhone(recipient)
		if err != nil {
			log.Debugf("password reset confirmation rejected: invalid phone %q: %v", recipient, err)
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code", "LAYR_AUTH_INVALID_PHONE")
			return
		}
		recipient = normalizedPhone
	}

	if len(passwordResetConfirmRequest.Password) < config.Password.MinLength {
		log.Debugf("password reset confirmation rejected: password shorter than min length (%d)", config.Password.MinLength)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Password must be at least %d characters", config.Password.MinLength), "LAYR_AUTH_001")
		return
	}

	if handler.db == nil {
		log.Debug("password reset confirmation rejected: database pool unavailable")
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Database unavailable", "LAYR_AUTH_001")
		return
	}

	ctx := request.Context()
	var otpID, storedHash string
	var attempts int
	var expiresAt time.Time

	err := handler.db.QueryRow(ctx, `
		SELECT id, code_hash, attempts, expires_at 
		FROM auth.otps 
		WHERE recipient = $1 AND purpose = 'password_reset' AND expires_at > clock_timestamp()
		ORDER BY created_at DESC 
		LIMIT 1
	`, recipient).Scan(&otpID, &storedHash, &attempts, &expiresAt)
	if err != nil {
		log.Debugf("password reset confirmation failed: OTP not found or expired for %s: %v", recipient, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired reset code", "LAYR_AUTH_001")
		return
	}

	if attempts >= maxPasswordResetAttempts {
		log.Debugf("password reset confirmation failed: max attempts exceeded for %s", recipient)
		_, _ = handler.db.Exec(ctx, "DELETE FROM auth.otps WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Maximum attempts exceeded", "LAYR_AUTH_001")
		return
	}

	if !otp.VerifyCode(passwordResetConfirmRequest.Code, storedHash) {
		log.Debugf("password reset confirmation failed: invalid code for %s", recipient)
		_, _ = handler.db.Exec(ctx, "UPDATE auth.otps SET attempts = attempts + 1 WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid reset code", "LAYR_AUTH_001")
		return
	}

	// Delete used OTP
	_, _ = handler.db.Exec(ctx, "DELETE FROM auth.otps WHERE id = $1", otpID)
	if handler.kvStore != nil {
		_ = handler.kvStore.Delete(ctx, fmt.Sprintf("auth:otp:password_reset:%s", recipient))
	}

	passHash, _ := handler.hasher.Hash(passwordResetConfirmRequest.Password)

	var userRecord UserRecord
	var rawProps []byte
	err = handler.db.QueryRow(ctx, `
		UPDATE auth.users 
		SET password_hash = $1, last_updated_at = clock_timestamp() 
		WHERE email = $2 OR phone = $2
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, properties, created_at, last_updated_at
	`, passHash, recipient).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&rawProps, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		log.Debugf("password reset user update failed for recipient %s: %v", recipient, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_002")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProps) > 0 {
		_ = json.Unmarshal(rawProps, &userRecord.Properties)
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewPasswordResetEvent(userRecord.ID, PasswordResetEventData{
			Recipient: recipient,
			User:      userRecord,
		}))
	}

	log.Debugf("password reset successful for user %s", userRecord.ID)
	handler.issueSessionResponse(responseWriter, request, userRecord)
}

func (handler *BaseHandler) handleUpdateUserPassword(responseWriter http.ResponseWriter, request *http.Request) {
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
		FROM auth.users
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
		UPDATE auth.users
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
