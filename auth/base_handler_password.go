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

	var signUpInput SignUpInput
	if err := json.NewDecoder(request.Body).Decode(&signUpInput); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	signUpInput.Email = strings.TrimSpace(strings.ToLower(signUpInput.Email))
	signUpInput.Phone = strings.TrimSpace(signUpInput.Phone)
	if signUpInput.Phone != "" {
		normalizedPhone, err := NormalizePhone(signUpInput.Phone)
		if err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code")
			return
		}
		signUpInput.Phone = normalizedPhone
	}
	if signUpInput.Email == "" && signUpInput.Phone == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Email or phone number is required")
		return
	}
	if len(signUpInput.Password) < config.Password.MinLength {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Password must be at least %d characters", config.Password.MinLength))
		return
	}

	clientIP := core.ExtractRequestClientIP(request)
	if !handler.checkCaptcha(responseWriter, request, clientIP, signUpInput.CaptchaToken, "/v1/auth/sign-up") {
		return
	}
	if !handler.checkPasswordBreach(responseWriter, request, signUpInput.Password, signUpInput.Email) {
		return
	}

	passHash, _ := handler.hasher.Hash(signUpInput.Password)

	inputProperties := signUpInput.Properties
	if inputProperties == nil {
		inputProperties = make(map[string]any)
	}
	propertiesJSON, _ := json.Marshal(inputProperties)

	ctx := request.Context()
	var emailPtr, phonePtr *string
	if signUpInput.Email != "" {
		emailPtr = &signUpInput.Email
	}
	if signUpInput.Phone != "" {
		phonePtr = &signUpInput.Phone
	}

	anonymousUser, _ := handler.resolveAnonymousCaller(request)
	if anonymousUser != nil {
		var existingUserID string
		err := handler.kernel.DB().QueryRow(ctx, `
			SELECT id FROM auth.users 
			WHERE (email IS NOT NULL AND email = $1) 
			   OR (phone IS NOT NULL AND phone = $2)
			LIMIT 1
		`, emailPtr, phonePtr).Scan(&existingUserID)
		if err == nil && existingUserID != anonymousUser.ID {
			if emailPtr != nil {
				core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Email is already in use by another account")
			} else {
				core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Phone number is already in use by another account")
			}
			return
		}

		var user User
		var rawProperties []byte
		updateQuery := `
			UPDATE auth.users
			SET email = $1, phone = $2, password_hash = $3, is_anonymous = false,
			    properties = COALESCE(properties, '{}'::jsonb) || $4::jsonb,
			    last_updated_at = clock_timestamp()
			WHERE id = $5
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		`
		err = handler.kernel.DB().QueryRow(ctx, updateQuery, emailPtr, phonePtr, passHash, propertiesJSON, anonymousUser.ID).Scan(
			&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
			&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
			&user.EncryptedMFASecret, &user.MFAEnabled,
			&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
		)
		if err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to convert anonymous user %s: %v", anonymousUser.ID, err))
			return
		}

		user.Properties = make(map[string]any)
		if len(rawProperties) > 0 {
			_ = json.Unmarshal(rawProperties, &user.Properties)
		}

		handler.kernel.EventBus().Publish(ctx, NewUserConvertedEvent(user.ID, UserConvertedEventData(user)))

		handler.issueSessionResponse(responseWriter, request, user, "password")
		return
	}

	var user User
	var rawProperties []byte
	query := `
		INSERT INTO auth.users (email, phone, password_hash, role, properties, created_at, last_updated_at)
		VALUES ($1, $2, $3, 'authenticated', $4, clock_timestamp(), clock_timestamp())
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`
	err := handler.kernel.DB().QueryRow(ctx, query, emailPtr, phonePtr, passHash, propertiesJSON).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to create user: %v", err))
		return
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	handler.kernel.EventBus().Publish(ctx, NewUserSignedUpEvent(user.ID, UserSignedUpEventData(user)))

	handler.issueSessionResponse(responseWriter, request, user, "password")
}

func (handler *BaseHandler) handleSignIn(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling sign-in request")
	var signInInput SignInInput
	if err := json.NewDecoder(request.Body).Decode(&signInInput); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	identifier := strings.TrimSpace(strings.ToLower(signInInput.Email))
	if identifier == "" && signInInput.Phone != "" {
		normalizedPhone, err := NormalizePhone(signInInput.Phone)
		if err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code")
			return
		}
		identifier = normalizedPhone
	}
	if identifier == "" || signInInput.Password == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	ctx := request.Context()
	config := handler.configManager.Get()
	clientIP := core.ExtractRequestClientIP(request)

	if !handler.checkCaptcha(responseWriter, request, clientIP, signInInput.CaptchaToken, "/v1/auth/sign-in") {
		return
	}

	if config.RateLimiting.Enabled && identifier != "" {
		rateKey := fmt.Sprintf("auth:ratelimit:sign_in:%s", identifier)
		windowDuration := time.Duration(config.RateLimiting.WindowDurationSeconds) * time.Second
		if count, err := handler.kernel.KVStore().Increment(ctx, rateKey, windowDuration); err == nil && count > int64(config.RateLimiting.MaxSignInAttempts) {
			handler.kernel.EventBus().Publish(ctx, NewRateLimitExceededEvent(identifier, RateLimitExceededEventData{
				Identifier:   identifier,
				Endpoint:     "/v1/auth/sign-in",
				AttemptCount: count,
				IPAddress:    clientIP,
				UserAgent:    request.UserAgent(),
			}))
			core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Too many login attempts. Please try again later.")
			return
		}
	}

	var user User
	var passHash string
	var rawProperties []byte

	query := `
		SELECT id, email, phone, password_hash, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
		FROM auth.users
		WHERE email = $1 OR phone = $1
	`
	err := handler.kernel.DB().QueryRow(ctx, query, identifier).Scan(
		&user.ID, &user.Email, &user.Phone, &passHash, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		handler.verifyDummyPassword(signInInput.Password)
		_, _ = threat.RecordFailedAttempt(ctx, handler.kernel.KVStore(), clientIP, 0)
		handler.kernel.EventBus().Publish(ctx, NewUserSignInFailedEvent(identifier, UserSignInFailedEventData{
			Identifier: identifier,
			AuthMethod: "password",
			Reason:     "user_not_found",
			IPAddress:  clientIP,
			UserAgent:  request.UserAgent(),
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if user.LockedUntil != nil && time.Now().UTC().Before(*user.LockedUntil) {
		_, _ = threat.RecordFailedAttempt(ctx, handler.kernel.KVStore(), clientIP, 0)
		_, _ = threat.RecordFailedAttempt(ctx, handler.kernel.KVStore(), user.ID, 0)
		handler.kernel.EventBus().Publish(ctx, NewUserSignInFailedEvent(user.ID, UserSignInFailedEventData{
			Identifier: identifier,
			AuthMethod: "password",
			Reason:     "account_locked",
			IPAddress:  clientIP,
			UserAgent:  request.UserAgent(),
			User:       &user,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	ok, err := handler.hasher.Verify(signInInput.Password, passHash)
	if err != nil || !ok {
		_, _ = threat.RecordFailedAttempt(ctx, handler.kernel.KVStore(), clientIP, 0)
		_, _ = threat.RecordFailedAttempt(ctx, handler.kernel.KVStore(), user.ID, 0)
		handler.kernel.EventBus().Publish(ctx, NewUserSignInFailedEvent(user.ID, UserSignInFailedEventData{
			Identifier: identifier,
			AuthMethod: "password",
			Reason:     "invalid_credentials",
			IPAddress:  clientIP,
			UserAgent:  request.UserAgent(),
			User:       &user,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	// Reset rate limit on successful authentication
	if identifier != "" {
		rateKey := fmt.Sprintf("auth:ratelimit:sign_in:%s", identifier)
		_ = handler.kernel.KVStore().Delete(ctx, rateKey)
	}

	handler.completeSignInFlow(responseWriter, request, user, "password")
}

func (handler *BaseHandler) handleRequestPasswordReset(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling password reset request")
	config := handler.configManager.Get()
	if !config.Password.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "password reset rejected: password authentication is disabled in configuration")
		return
	}

	var requestPasswordResetInput RequestPasswordResetInput
	if err := json.NewDecoder(request.Body).Decode(&requestPasswordResetInput); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	recipient := strings.TrimSpace(strings.ToLower(requestPasswordResetInput.Recipient))
	if recipient == "" {
		recipient = strings.TrimSpace(strings.ToLower(requestPasswordResetInput.Email))
	}
	if recipient == "" {
		recipient = strings.TrimSpace(requestPasswordResetInput.Phone)
	}
	if recipient == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Email or phone number is required")
		return
	}

	clientIP := core.ExtractRequestClientIP(request)
	if !handler.checkCaptcha(responseWriter, request, clientIP, requestPasswordResetInput.CaptchaToken, "/v1/auth/password-reset/request") {
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

	ipRateKey := fmt.Sprintf("auth:ratelimit:otp:ip:%s", clientIP)
	if count, err := handler.kernel.KVStore().Increment(ctx, ipRateKey, time.Hour); err == nil && count > ipPasswordResetLimit {
		core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Rate limit exceeded. Too many requests from this IP address.")
		return
	}

	cooldownKey := fmt.Sprintf("auth:cooldown:password_reset:%s", recipient)
	if _, err := handler.kernel.KVStore().Get(ctx, cooldownKey); err == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Please wait 60 seconds before requesting another code")
		return
	}

	user, err := fetchUserByRecipient(ctx, handler.kernel.DB(), recipient)
	if err != nil {
		log.Debugf("password reset requested for unregistered recipient %s", recipient)
		responseWriter.WriteHeader(http.StatusNoContent)
		return
	}
	userID := user.ID

	code, _ := otp.GenerateCode(nil)
	codeHash := otp.HashCode(code)
	expiresAt := time.Now().UTC().Add(otp.CodeTTL)

	query := `
		INSERT INTO auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'password_reset', 0, $3, clock_timestamp())
	`
	_, _ = handler.kernel.DB().Exec(ctx, query, recipient, codeHash, expiresAt)
	_ = handler.kernel.KVStore().Set(ctx, fmt.Sprintf("auth:otp:password_reset:%s", recipient), code, otp.CodeTTL)
	_ = handler.kernel.KVStore().Set(ctx, fmt.Sprintf("auth:cooldown:password_reset:%s", recipient), "1", passwordResetCooldownTTL)

	if isEmail {
		_ = handler.emailDispatcher.SendPasswordReset(ctx, recipient, code, userID)
	} else {
		_ = handler.smsDispatcher.SendPasswordReset(ctx, recipient, code, userID)
	}

	handler.kernel.EventBus().Publish(ctx, NewPasswordResetRequestedEvent(user.ID, PasswordResetRequestedEventData{
		Recipient: recipient,
		User:      user,
	}))

	log.Debugf("password reset code dispatched to %s", recipient)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *BaseHandler) handleConfirmPasswordReset(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling password reset confirmation request")
	config := handler.configManager.Get()
	if !config.Password.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "password reset confirmation rejected: password authentication is disabled in configuration")
		return
	}

	var confirmPasswordResetInput ConfirmPasswordResetInput
	if err := json.NewDecoder(request.Body).Decode(&confirmPasswordResetInput); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	recipient := strings.TrimSpace(strings.ToLower(confirmPasswordResetInput.Recipient))
	if recipient == "" {
		recipient = strings.TrimSpace(strings.ToLower(confirmPasswordResetInput.Email))
	}
	if recipient == "" {
		recipient = strings.TrimSpace(confirmPasswordResetInput.Phone)
	}
	if recipient == "" || confirmPasswordResetInput.Code == "" || confirmPasswordResetInput.Password == "" {
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

	if len(confirmPasswordResetInput.Password) < config.Password.MinLength {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Password must be at least %d characters", config.Password.MinLength))
		return
	}

	if !handler.checkPasswordBreach(responseWriter, request, confirmPasswordResetInput.Password, recipient) {
		return
	}

	ctx := request.Context()
	var otpID, storedHash string
	var attempts int
	var expiresAt time.Time

	err := handler.kernel.DB().QueryRow(ctx, `
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
		_, _ = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.otps WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Maximum attempts exceeded")
		return
	}

	if !otp.VerifyCode(confirmPasswordResetInput.Code, storedHash) {
		_, _ = handler.kernel.DB().Exec(ctx, "UPDATE auth.otps SET attempts = attempts + 1 WHERE id = $1", otpID)
		channel := "email"
		if !strings.Contains(recipient, "@") {
			channel = "sms"
		}
		clientIP := core.ExtractRequestClientIP(request)
		handler.kernel.EventBus().Publish(ctx, NewOTPVerificationFailedEvent(recipient, OTPVerificationFailedEventData{
			Recipient: recipient,
			Purpose:   "password_reset",
			Channel:   channel,
			Reason:    "invalid_code",
			IPAddress: clientIP,
			UserAgent: request.UserAgent(),
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid reset code")
		return
	}

	// Delete used OTP
	_, _ = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.otps WHERE id = $1", otpID)
	_ = handler.kernel.KVStore().Delete(ctx, fmt.Sprintf("auth:otp:password_reset:%s", recipient))

	passHash, _ := handler.hasher.Hash(confirmPasswordResetInput.Password)

	var user User
	var rawProps []byte
	err = handler.kernel.DB().QueryRow(ctx, `
		UPDATE auth.users 
		SET password_hash = $1, last_updated_at = clock_timestamp() 
		WHERE email = $2 OR phone = $2
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`, passHash, recipient).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProps, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired reset code")
		return
	}

	user.Properties = make(map[string]any)
	if len(rawProps) > 0 {
		_ = json.Unmarshal(rawProps, &user.Properties)
	}

	if user.LockedUntil != nil && time.Now().UTC().Before(*user.LockedUntil) {
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	handler.kernel.EventBus().Publish(ctx, NewPasswordResetEvent(user.ID, PasswordResetEventData{
		Recipient: recipient,
		User:      user,
	}))

	log.Debugf("password reset successful for user %s", user.ID)
	handler.issueSessionResponse(responseWriter, request, user, "password_reset")
}

func (handler *BaseHandler) handleUpdateUserPassword(responseWriter http.ResponseWriter, request *http.Request) {
	authContext := core.GetAuthContext(request.Context())
	if authContext.UserID == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := authContext.UserID

	var updateUserPasswordInput UpdateUserPasswordInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&updateUserPasswordInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	ctx := request.Context()
	var isCallerAnonymous bool
	var email *string
	var phone *string
	var existingPasswordHash *string

	var lockedUntil *time.Time
	err := handler.kernel.DB().QueryRow(ctx, `
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
		if updateUserPasswordInput.CurrentPassword == "" {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Current password is required")
			return
		}
		isCurrentPasswordValid, verifyErr := handler.hasher.Verify(updateUserPasswordInput.CurrentPassword, *existingPasswordHash)
		if verifyErr != nil || !isCurrentPasswordValid {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Current password is incorrect")
			return
		}
	}

	config := handler.configManager.Get()
	if len(updateUserPasswordInput.NewPassword) < config.Password.MinLength {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Password must be at least %d characters", config.Password.MinLength))
		return
	}

	var userEmail string
	if email != nil {
		userEmail = *email
	}
	if !handler.checkPasswordBreach(responseWriter, request, updateUserPasswordInput.NewPassword, userEmail) {
		return
	}

	hashedPassword, _ := handler.hasher.Hash(updateUserPasswordInput.NewPassword)

	var user User
	var rawProperties []byte
	queryErr := handler.kernel.DB().QueryRow(ctx, `
		UPDATE auth.users
		SET password_hash = $1, last_updated_at = clock_timestamp()
		WHERE id = $2
		RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
	`, hashedPassword, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
		&user.EncryptedMFASecret, &user.MFAEnabled,
		&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
	)
	if queryErr == nil {
		user.Properties = make(map[string]any)
		if len(rawProperties) > 0 {
			_ = json.Unmarshal(rawProperties, &user.Properties)
		}
		handler.kernel.EventBus().Publish(ctx, NewPasswordChangedEvent(user.ID, PasswordChangedEventData(user)))
	}

	log.Debugf("user password successfully updated for %s", userID)
	responseWriter.WriteHeader(http.StatusNoContent)
}
