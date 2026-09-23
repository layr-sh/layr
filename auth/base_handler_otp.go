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
	defaultOTPCooldown = 60 * time.Second
)

func (handler *BaseHandler) handleSendOTP(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.EmailOTP.Enabled && !config.SMSOTP.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "otp send rejected: OTP authentication is disabled in configuration")
		return
	}

	var sendOTPInput SendOTPInput
	if err := json.NewDecoder(request.Body).Decode(&sendOTPInput); err != nil || sendOTPInput.Recipient == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Recipient email or phone number required")
		return
	}

	clientIP := core.ExtractRequestClientIP(request)
	if !handler.checkCaptcha(responseWriter, request, clientIP, sendOTPInput.CaptchaToken, "/v1/auth/otp") {
		return
	}

	recipient := strings.TrimSpace(strings.ToLower(sendOTPInput.Recipient))
	purpose := strings.TrimSpace(sendOTPInput.Purpose)
	if purpose == "" {
		purpose = "sign_in"
	}

	isEmail := strings.Contains(recipient, "@")
	var tokenExpiryMinutes int
	if isEmail {
		if !config.EmailOTP.Enabled {
			core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "otp send rejected: email OTP is disabled in configuration")
			return
		}
		if !handler.assertEmailDeliveryReady(responseWriter, request) {
			return
		}
		tokenExpiryMinutes = config.EmailOTP.TokenExpiryMinutes
	} else {
		if !config.SMSOTP.Enabled {
			core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "otp send rejected: SMS OTP is disabled in configuration")
			return
		}
		if !handler.assertSMSDeliveryReady(responseWriter, request) {
			return
		}
		normalizedPhone, err := NormalizePhone(recipient)
		if err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid phone number format: must be in E.164 format with country code")
			return
		}
		recipient = normalizedPhone
		tokenExpiryMinutes = config.SMSOTP.TokenExpiryMinutes
	}

	codeTTL := time.Duration(tokenExpiryMinutes) * time.Minute

	code, _ := otp.GenerateCode(nil)
	codeHash := otp.HashCode(code)

	ctx := request.Context()

	ipRateKey := fmt.Sprintf("auth:ratelimit:otp:ip:%s", clientIP)
	if count, err := handler.kernel.KVStore().Increment(ctx, ipRateKey, time.Hour); err == nil && count > 10 {
		handler.kernel.EventBus().Publish(ctx, NewRateLimitExceededEvent(clientIP, RateLimitExceededEventData{
			Identifier:   clientIP,
			Endpoint:     "/v1/auth/otp",
			AttemptCount: count,
			IPAddress:    clientIP,
			UserAgent:    request.UserAgent(),
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Rate limit exceeded. Too many requests from this IP address.")
		return
	}

	cooldownKey := fmt.Sprintf("auth:cooldown:%s:%s", purpose, recipient)
	if _, err := handler.kernel.KVStore().Get(ctx, cooldownKey); err == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Please wait 60 seconds before requesting another code")
		return
	}

	expiresAt := time.Now().UTC().Add(codeTTL)

	query := `
		INSERT INTO auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, $3, 0, $4, clock_timestamp())
	`
	_, _ = handler.kernel.DB().Exec(ctx, query, recipient, codeHash, purpose, expiresAt)
	_ = handler.kernel.KVStore().Set(ctx, fmt.Sprintf("auth:otp:%s:%s", purpose, recipient), code, codeTTL)
	_ = handler.kernel.KVStore().Set(ctx, fmt.Sprintf("auth:cooldown:%s:%s", purpose, recipient), "1", defaultOTPCooldown)

	channel := "sms"
	if isEmail {
		channel = "email"
		_ = handler.emailDispatcher.SendSignInOTP(ctx, recipient, code, "")
	} else {
		_ = handler.smsDispatcher.SendSignInOTP(ctx, recipient, code, "")
	}

	var targetUser *User
	if user, err := fetchUserByRecipient(ctx, handler.kernel.DB(), recipient); err == nil {
		targetUser = &user
	}
	handler.kernel.EventBus().Publish(ctx, NewOTPSentEvent(recipient, OTPSentEventData{
		Recipient: recipient,
		Purpose:   purpose,
		Channel:   channel,
		User:      targetUser,
	}))

	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *BaseHandler) handleVerifyOTP(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.EmailOTP.Enabled && !config.SMSOTP.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "otp verify rejected: OTP authentication is disabled in configuration")
		return
	}

	var verifyOTPInput VerifyOTPInput
	if err := json.NewDecoder(request.Body).Decode(&verifyOTPInput); err != nil || verifyOTPInput.Recipient == "" || verifyOTPInput.Code == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Recipient and code required")
		return
	}

	clientIP := core.ExtractRequestClientIP(request)
	if !handler.checkCaptcha(responseWriter, request, clientIP, verifyOTPInput.CaptchaToken, "/v1/auth/otp/verify") {
		return
	}

	recipient := strings.TrimSpace(strings.ToLower(verifyOTPInput.Recipient))
	purpose := strings.TrimSpace(verifyOTPInput.Purpose)
	if purpose == "" {
		purpose = "sign_in"
	}

	isEmail := strings.Contains(recipient, "@")
	channel := "sms"
	if isEmail {
		channel = "email"
	}
	if isEmail {
		if !config.EmailOTP.Enabled {
			core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "otp verify rejected: email OTP is disabled in configuration")
			return
		}
	} else {
		if !config.SMSOTP.Enabled {
			core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Access denied", "otp verify rejected: SMS OTP is disabled in configuration")
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
	var otpID, storedHash string
	var attempts int
	var expiresAt time.Time

	err := handler.kernel.DB().QueryRow(ctx, `
		SELECT id, code_hash, attempts, expires_at 
		FROM auth.otps 
		WHERE recipient = $1 AND purpose = $2 AND expires_at > clock_timestamp()
		ORDER BY created_at DESC 
		LIMIT 1
	`, recipient, purpose).Scan(&otpID, &storedHash, &attempts, &expiresAt)
	if err != nil {
		handler.kernel.EventBus().Publish(ctx, NewOTPVerificationFailedEvent(recipient, OTPVerificationFailedEventData{
			Recipient: recipient,
			Purpose:   purpose,
			Channel:   channel,
			Reason:    "invalid_or_expired_code",
			IPAddress: clientIP,
			UserAgent: request.UserAgent(),
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or expired OTP code")
		return
	}

	if !otp.VerifyCode(verifyOTPInput.Code, storedHash) {
		_, _ = handler.kernel.DB().Exec(ctx, "UPDATE auth.otps SET attempts = attempts + 1 WHERE id = $1", otpID)
		_, _ = threat.RecordFailedAttempt(ctx, handler.kernel.KVStore(), clientIP, 0)
		_, _ = threat.RecordFailedAttempt(ctx, handler.kernel.KVStore(), recipient, 0)
		handler.kernel.EventBus().Publish(ctx, NewOTPVerificationFailedEvent(recipient, OTPVerificationFailedEventData{
			Recipient: recipient,
			Purpose:   purpose,
			Channel:   channel,
			Reason:    "invalid_code",
			IPAddress: clientIP,
			UserAgent: request.UserAgent(),
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid OTP code")
		return
	}

	// Delete used OTP
	_, _ = handler.kernel.DB().Exec(ctx, "DELETE FROM auth.otps WHERE id = $1", otpID)
	_ = handler.kernel.KVStore().Delete(ctx, fmt.Sprintf("auth:otp:%s:%s", purpose, recipient))

	anonymousUser, _ := handler.resolveAnonymousCaller(request)
	if anonymousUser != nil {
		var conflictingUserID string
		var checkQuery string
		if isEmail {
			checkQuery = "SELECT id FROM auth.users WHERE email = $1"
		} else {
			checkQuery = "SELECT id FROM auth.users WHERE phone = $1"
		}
		conflictErr := handler.kernel.DB().QueryRow(ctx, checkQuery, recipient).Scan(&conflictingUserID)
		if conflictErr == nil && conflictingUserID != anonymousUser.ID {
			if isEmail {
				core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Email is already in use by another account")
			} else {
				core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Phone number is already in use by another account")
			}
			return
		}

		var user User
		var rawProperties []byte
		if isEmail {
			_ = handler.kernel.DB().QueryRow(ctx, `
				UPDATE auth.users 
				SET email = $1, email_verified_at = clock_timestamp(), is_anonymous = false, last_updated_at = clock_timestamp() 
				WHERE id = $2
				RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
			`, recipient, anonymousUser.ID).Scan(
				&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
				&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
				&user.EncryptedMFASecret, &user.MFAEnabled,
				&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
			)
		} else {
			_ = handler.kernel.DB().QueryRow(ctx, `
				UPDATE auth.users 
				SET phone = $1, phone_verified_at = clock_timestamp(), is_anonymous = false, last_updated_at = clock_timestamp() 
				WHERE id = $2
				RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at
			`, recipient, anonymousUser.ID).Scan(
				&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
				&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
				&user.EncryptedMFASecret, &user.MFAEnabled,
				&rawProperties, &user.CreatedAt, &user.LastUpdatedAt,
			)
		}

		user.Properties = make(map[string]any)
		if len(rawProperties) > 0 {
			_ = json.Unmarshal(rawProperties, &user.Properties)
		}

		handler.kernel.EventBus().Publish(ctx, NewUserConvertedEvent(user.ID, UserConvertedEventData(user)))
		handler.kernel.EventBus().Publish(ctx, NewOTPVerifiedEvent(recipient, OTPVerifiedEventData{
			Recipient: recipient,
			Purpose:   purpose,
			Channel:   channel,
			User:      &user,
		}))

		handler.issueSessionResponse(responseWriter, request, user, "otp")
		return
	}

	// Ensure User exists
	var user User
	var rawProperties []byte
	var isNewUser bool
	if isEmail {
		_ = handler.kernel.DB().QueryRow(ctx, `
			INSERT INTO auth.users (email, role, email_verified_at, created_at, last_updated_at)
			VALUES ($1, 'authenticated', clock_timestamp(), clock_timestamp(), clock_timestamp())
			ON CONFLICT (email) DO UPDATE SET email_verified_at = COALESCE(auth.users.email_verified_at, clock_timestamp()), last_updated_at = clock_timestamp()
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at, (xmax = 0) AS is_new
		`, recipient).Scan(
			&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
			&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
			&user.EncryptedMFASecret, &user.MFAEnabled,
			&rawProperties, &user.CreatedAt, &user.LastUpdatedAt, &isNewUser,
		)
	} else {
		_ = handler.kernel.DB().QueryRow(ctx, `
			INSERT INTO auth.users (phone, role, phone_verified_at, created_at, last_updated_at)
			VALUES ($1, 'authenticated', clock_timestamp(), clock_timestamp(), clock_timestamp())
			ON CONFLICT (phone) DO UPDATE SET phone_verified_at = COALESCE(auth.users.phone_verified_at, clock_timestamp()), last_updated_at = clock_timestamp()
			RETURNING id, email, phone, role, is_anonymous, email_verified_at, phone_verified_at, locked_until, encrypted_mfa_secret, mfa_enabled, properties, created_at, last_updated_at, (xmax = 0) AS is_new
		`, recipient).Scan(
			&user.ID, &user.Email, &user.Phone, &user.Role, &user.IsAnonymous,
			&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.LockedUntil,
			&user.EncryptedMFASecret, &user.MFAEnabled,
			&rawProperties, &user.CreatedAt, &user.LastUpdatedAt, &isNewUser,
		)
	}

	user.Properties = make(map[string]any)
	if len(rawProperties) > 0 {
		_ = json.Unmarshal(rawProperties, &user.Properties)
	}

	if !isNewUser && user.LockedUntil != nil && time.Now().UTC().Before(*user.LockedUntil) {
		handler.kernel.EventBus().Publish(ctx, NewOTPVerificationFailedEvent(recipient, OTPVerificationFailedEventData{
			Recipient: recipient,
			Purpose:   purpose,
			Channel:   channel,
			Reason:    "account_locked",
			IPAddress: clientIP,
			UserAgent: request.UserAgent(),
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusLocked, "Account temporarily locked")
		return
	}

	if isNewUser {
		handler.kernel.EventBus().Publish(ctx, NewUserSignedUpEvent(user.ID, UserSignedUpEventData(user)))
	}
	handler.kernel.EventBus().Publish(ctx, NewOTPVerifiedEvent(recipient, OTPVerifiedEventData{
		Recipient: recipient,
		Purpose:   purpose,
		Channel:   channel,
		User:      &user,
	}))

	handler.completeSignInFlow(responseWriter, request, user, "otp")
}
