package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"layr.sh/auth/otp"
	"layr.sh/auth/threat"
	"layr.sh/core"
)

const (
	maxPasswordResetAttempts = 5
	ipPasswordResetLimit     = 10
	passwordResetCooldownTTL = 60 * time.Second
	mfaTicketTTL             = 5 * time.Minute
)

func (handler *BaseHandler) handleSignUp(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling sign-up request")
	config := handler.configManager.Get()
	if !config.Password.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "sign-up rejected: password registration is disabled in configuration")
		return
	}

	var signUpRequest SignUpRequest
	if err := json.NewDecoder(request.Body).Decode(&signUpRequest); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	signUpRequest.Email = strings.TrimSpace(strings.ToLower(signUpRequest.Email))
	signUpRequest.Phone = strings.TrimSpace(signUpRequest.Phone)
	if signUpRequest.Phone != "" {
		normalizedPhone, err := NormalizePhone(signUpRequest.Phone)
		if err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code")
			return
		}
		signUpRequest.Phone = normalizedPhone
	}
	if signUpRequest.Email == "" && signUpRequest.Phone == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Email or phone number is required")
		return
	}
	if len(signUpRequest.Password) < config.Password.MinLength {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Password must be at least %d characters", config.Password.MinLength))
		return
	}

	clientIP := core.ExtractRequestClientIP(request)
	if !handler.checkCaptcha(responseWriter, request, clientIP, signUpRequest.CaptchaToken, "/api/v1/auth/sign-up") {
		return
	}
	if !handler.checkPasswordBreach(responseWriter, request, signUpRequest.Password, signUpRequest.Email) {
		return
	}

	passHash, _ := handler.hasher.Hash(signUpRequest.Password)

	inputProperties := signUpRequest.Properties
	if inputProperties == nil {
		inputProperties = make(map[string]any)
	}
	propertiesJSON, _ := json.Marshal(inputProperties)

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "sign-up rejected: database pool unavailable")
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
			if emailPtr != nil {
				core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Email is already in use by another account")
			} else {
				core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Phone number is already in use by another account")
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
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		`
		err = handler.db.QueryRow(ctx, updateQuery, emailPtr, phonePtr, passHash, propertiesJSON, anonymousUserRecord.ID).Scan(
			&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
			&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
			&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
			&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
		)
		if err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to convert anonymous user %s: %v", anonymousUserRecord.ID, err))
			return
		}

		userRecord.Properties = make(map[string]any)
		if len(rawProperties) > 0 {
			_ = json.Unmarshal(rawProperties, &userRecord.Properties)
		}

		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewUserConvertedEvent(userRecord.ID, UserConvertedEventData(userRecord)))
		}

		handler.issueSessionResponse(responseWriter, request, userRecord, "password")
		return
	}

	var userRecord UserRecord
	var rawProperties []byte
	query := `
		INSERT INTO auth.users (email, phone, password_hash, role, properties, created_at, last_updated_at)
		VALUES ($1, $2, $3, 'authenticated', $4, clock_timestamp(), clock_timestamp())
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`
	err := handler.db.QueryRow(ctx, query, emailPtr, phonePtr, passHash, propertiesJSON).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to create user: %v", err))
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewUserSignedUpEvent(userRecord.ID, UserSignedUpEventData(userRecord)))
	}

	handler.issueSessionResponse(responseWriter, request, userRecord, "password")
}

func (handler *BaseHandler) handleSignIn(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling sign-in request")
	var signInRequest SignInRequest
	if err := json.NewDecoder(request.Body).Decode(&signInRequest); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	identifier := strings.TrimSpace(strings.ToLower(signInRequest.Email))
	if identifier == "" && signInRequest.Phone != "" {
		normalizedPhone, err := NormalizePhone(signInRequest.Phone)
		if err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code")
			return
		}
		identifier = normalizedPhone
	}
	if identifier == "" || signInRequest.Password == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	ctx := request.Context()
	config := handler.configManager.Get()
	clientIP := core.ExtractRequestClientIP(request)

	if !handler.checkCaptcha(responseWriter, request, clientIP, signInRequest.CaptchaToken, "/api/v1/auth/sign-in") {
		return
	}

	if config.RateLimiting.Enabled && handler.kvStore != nil && identifier != "" {
		rateKey := fmt.Sprintf("auth:ratelimit:sign_in:%s", identifier)
		windowDuration := time.Duration(config.RateLimiting.WindowDurationSeconds) * time.Second
		if count, err := handler.kvStore.Increment(ctx, rateKey, windowDuration); err == nil && count > int64(config.RateLimiting.MaxSignInAttempts) {
			if handler.eventBus != nil {
				handler.eventBus.Publish(ctx, NewRateLimitExceededEvent(identifier, RateLimitExceededEventData{
					Identifier:   identifier,
					Endpoint:     "/api/v1/auth/sign-in",
					AttemptCount: count,
					IPAddress:    clientIP,
					UserAgent:    request.UserAgent(),
				}))
			}
			core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Too many login attempts. Please try again later.")
			return
		}
	}

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "sign-in rejected: database pool unavailable")
		return
	}

	var userRecord UserRecord
	var passHash string
	var rawProperties []byte

	query := `
		SELECT id, email, phone, password_hash, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE email = $1 OR phone = $1
	`
	err := handler.db.QueryRow(ctx, query, identifier).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &passHash, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		handler.verifyDummyPassword(signInRequest.Password)
		if handler.kvStore != nil {
			_, _ = threat.RecordFailedAttempt(ctx, handler.kvStore, clientIP, 0)
		}
		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewUserSignInFailedEvent(identifier, UserSignInFailedEventData{
				Identifier: identifier,
				AuthMethod: "password",
				Reason:     "user_not_found",
				IPAddress:  clientIP,
				UserAgent:  request.UserAgent(),
			}))
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
		if handler.kvStore != nil {
			_, _ = threat.RecordFailedAttempt(ctx, handler.kvStore, clientIP, 0)
			_, _ = threat.RecordFailedAttempt(ctx, handler.kvStore, userRecord.ID, 0)
		}
		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewUserSignInFailedEvent(userRecord.ID, UserSignInFailedEventData{
				Identifier: identifier,
				AuthMethod: "password",
				Reason:     "account_locked",
				IPAddress:  clientIP,
				UserAgent:  request.UserAgent(),
				User:       &userRecord,
			}))
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	ok, err := handler.hasher.Verify(signInRequest.Password, passHash)
	if err != nil || !ok {
		if handler.kvStore != nil {
			_, _ = threat.RecordFailedAttempt(ctx, handler.kvStore, clientIP, 0)
			_, _ = threat.RecordFailedAttempt(ctx, handler.kvStore, userRecord.ID, 0)
		}
		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewUserSignInFailedEvent(userRecord.ID, UserSignInFailedEventData{
				Identifier: identifier,
				AuthMethod: "password",
				Reason:     "invalid_credentials",
				IPAddress:  clientIP,
				UserAgent:  request.UserAgent(),
				User:       &userRecord,
			}))
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &userRecord.Properties)
	}

	// Reset rate limit on successful authentication
	if handler.kvStore != nil && identifier != "" {
		rateKey := fmt.Sprintf("auth:ratelimit:sign_in:%s", identifier)
		_ = handler.kvStore.Delete(ctx, rateKey)
	}

	handler.completeSignInFlow(responseWriter, request, userRecord, "password")
}

func (handler *BaseHandler) handlePasswordResetRequest(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling password reset request")
	config := handler.configManager.Get()
	if !config.Password.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "password reset rejected: password authentication is disabled in configuration")
		return
	}

	var passwordResetRequest PasswordResetRequest
	if err := json.NewDecoder(request.Body).Decode(&passwordResetRequest); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
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
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Email or phone number is required")
		return
	}

	clientIP := core.ExtractRequestClientIP(request)
	if !handler.checkCaptcha(responseWriter, request, clientIP, passwordResetRequest.CaptchaToken, "/api/v1/auth/password/reset") {
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
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code")
			return
		}
		recipient = normalizedPhone
	}

	ctx := request.Context()

	if handler.kvStore != nil {
		clientIP := core.ExtractRequestClientIP(request)
		ipRateKey := fmt.Sprintf("auth:ratelimit:otp:ip:%s", clientIP)
		if count, err := handler.kvStore.Increment(ctx, ipRateKey, time.Hour); err == nil && count > ipPasswordResetLimit {
			core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Rate limit exceeded. Too many requests from this IP address.")
			return
		}

		cooldownKey := fmt.Sprintf("auth:cooldown:password_reset:%s", recipient)
		if _, err := handler.kvStore.Get(ctx, cooldownKey); err == nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Please wait 60 seconds before requesting another code")
			return
		}
	}

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "password reset request rejected: database pool unavailable")
		return
	}

	userRecord, err := fetchUserRecordByRecipient(ctx, handler.db, recipient)
	if err != nil {
		log.Debugf("password reset requested for unregistered recipient %s", recipient)
		responseWriter.WriteHeader(http.StatusNoContent)
		return
	}
	userID := userRecord.ID

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
		handler.eventBus.Publish(ctx, NewPasswordResetRequestedEvent(userRecord.ID, PasswordResetRequestedEventData{
			Recipient: recipient,
			User:      userRecord,
		}))
	}

	log.Debugf("password reset code dispatched to %s", recipient)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *BaseHandler) handlePasswordResetConfirm(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling password reset confirmation request")
	config := handler.configManager.Get()
	if !config.Password.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "password reset confirmation rejected: password authentication is disabled in configuration")
		return
	}

	var passwordResetConfirmRequest PasswordResetConfirmRequest
	if err := json.NewDecoder(request.Body).Decode(&passwordResetConfirmRequest); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
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
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Recipient, code, and new password are required")
		return
	}

	if !strings.Contains(recipient, "@") {
		normalizedPhone, err := NormalizePhone(recipient)
		if err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code")
			return
		}
		recipient = normalizedPhone
	}

	if len(passwordResetConfirmRequest.Password) < config.Password.MinLength {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Password must be at least %d characters", config.Password.MinLength))
		return
	}

	if !handler.checkPasswordBreach(responseWriter, request, passwordResetConfirmRequest.Password, recipient) {
		return
	}

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "password reset confirmation rejected: database pool unavailable")
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
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired reset code")
		return
	}

	if attempts >= maxPasswordResetAttempts {
		_, _ = handler.db.Exec(ctx, "DELETE FROM auth.otps WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Maximum attempts exceeded")
		return
	}

	if !otp.VerifyCode(passwordResetConfirmRequest.Code, storedHash) {
		_, _ = handler.db.Exec(ctx, "UPDATE auth.otps SET attempts = attempts + 1 WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid reset code")
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
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`, passHash, recipient).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProps, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired reset code")
		return
	}

	userRecord.Properties = make(map[string]any)
	if len(rawProps) > 0 {
		_ = json.Unmarshal(rawProps, &userRecord.Properties)
	}

	if userRecord.LockedUntil != nil && time.Now().UTC().Before(*userRecord.LockedUntil) {
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	if handler.eventBus != nil {
		handler.eventBus.Publish(ctx, NewPasswordResetEvent(userRecord.ID, PasswordResetEventData{
			Recipient: recipient,
			User:      userRecord,
		}))
	}

	log.Debugf("password reset successful for user %s", userRecord.ID)
	handler.issueSessionResponse(responseWriter, request, userRecord, "password_reset")
}

func (handler *BaseHandler) handleUpdateUserPassword(responseWriter http.ResponseWriter, request *http.Request) {
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := authContext.UserID

	var updateUserPasswordRequest UpdateUserPasswordRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&updateUserPasswordRequest); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if handler.db == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", "update user password rejected: database pool unavailable")
		return
	}

	ctx := request.Context()
	var isCallerAnonymous bool
	var email *string
	var phone *string
	var existingPasswordHash *string

	var lockedUntil *time.Time
	err := handler.db.QueryRow(ctx, `
		SELECT is_anonymous, email, phone, password_hash, locked_until
		FROM auth.users
		WHERE id = $1
	`, userID).Scan(&isCallerAnonymous, &email, &phone, &existingPasswordHash, &lockedUntil)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found")
		return
	}

	if lockedUntil != nil && time.Now().UTC().Before(*lockedUntil) {
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	if isCallerAnonymous || (email == nil && phone == nil) {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot set password on an account without a registered email or phone number")
		return
	}

	if existingPasswordHash != nil && *existingPasswordHash != "" {
		if updateUserPasswordRequest.CurrentPassword == "" {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Current password is required")
			return
		}
		isCurrentPasswordValid, verifyErr := handler.hasher.Verify(updateUserPasswordRequest.CurrentPassword, *existingPasswordHash)
		if verifyErr != nil || !isCurrentPasswordValid {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Current password is incorrect")
			return
		}
	}

	config := handler.configManager.Get()
	if len(updateUserPasswordRequest.NewPassword) < config.Password.MinLength {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Password must be at least %d characters", config.Password.MinLength))
		return
	}

	var userEmail string
	if email != nil {
		userEmail = *email
	}
	if !handler.checkPasswordBreach(responseWriter, request, updateUserPasswordRequest.NewPassword, userEmail) {
		return
	}

	hashedPassword, _ := handler.hasher.Hash(updateUserPasswordRequest.NewPassword)

	var userRecord UserRecord
	var rawProperties []byte
	queryErr := handler.db.QueryRow(ctx, `
		UPDATE auth.users
		SET password_hash = $1, last_updated_at = clock_timestamp()
		WHERE id = $2
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`, hashedPassword, userID).Scan(
		&userRecord.ID, &userRecord.Email, &userRecord.Phone, &userRecord.Role, &userRecord.IsAnonymous,
		&userRecord.EmailVerifiedAt, &userRecord.PhoneVerifiedAt, &userRecord.LockedUntil,
		&userRecord.EncryptedMFASecret, &userRecord.MFAEnabled,
		&rawProperties, &userRecord.CreatedAt, &userRecord.LastUpdatedAt,
	)
	if queryErr == nil {
		userRecord.Properties = make(map[string]any)
		if len(rawProperties) > 0 {
			_ = json.Unmarshal(rawProperties, &userRecord.Properties)
		}
		if handler.eventBus != nil {
			handler.eventBus.Publish(ctx, NewPasswordChangedEvent(userRecord.ID, PasswordChangedEventData(userRecord)))
		}
	}

	log.Debugf("user password successfully updated for %s", userID)
	responseWriter.WriteHeader(http.StatusNoContent)
}
