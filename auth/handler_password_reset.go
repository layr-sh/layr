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

// PasswordResetRequest defines password reset request input.
type PasswordResetRequest struct {
	Recipient string `json:"recipient"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

func (handler *Handler) handlePasswordResetRequest(responseWriter http.ResponseWriter, request *http.Request) {
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
		ipRateKey := fmt.Sprintf("layr:auth:ratelimit:otp:ip:%s", clientIP)
		if count, err := handler.kvStore.Increment(ctx, ipRateKey, time.Hour); err == nil && count > ipPasswordResetLimit {
			log.Debugf("password reset request rejected: IP rate limit exceeded for %s", clientIP)
			core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Rate limit exceeded. Too many requests from this IP address.", "LAYR_AUTH_RATE_LIMIT_EXCEEDED")
			return
		}

		cooldownKey := fmt.Sprintf("layr:auth:cooldown:password_reset:%s", recipient)
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
	err := handler.db.QueryRow(ctx, "SELECT id FROM layr_auth.users WHERE email = $1 OR phone = $1", recipient).Scan(&userID)
	if err != nil {
		log.Debugf("password reset request rejected: user not found for recipient %s: %v", recipient, err)
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "User not found", "LAYR_AUTH_002")
		return
	}

	code, _ := otp.GenerateCode(nil)
	codeHash := otp.HashCode(code)
	expiresAt := time.Now().UTC().Add(otp.CodeTTL)

	query := `
		INSERT INTO layr_auth.otps (recipient, code_hash, purpose, attempts, expires_at, created_at)
		VALUES ($1, $2, 'password_reset', 0, $3, clock_timestamp())
	`
	_, _ = handler.db.Exec(ctx, query, recipient, codeHash, expiresAt)
	if handler.kvStore != nil {
		_ = handler.kvStore.Set(ctx, fmt.Sprintf("layr:auth:otp:password_reset:%s", recipient), code, otp.CodeTTL)
		_ = handler.kvStore.Set(ctx, fmt.Sprintf("layr:auth:cooldown:password_reset:%s", recipient), "1", passwordResetCooldownTTL)
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

// PasswordResetConfirmRequest defines password reset confirmation input.
type PasswordResetConfirmRequest struct {
	Recipient string `json:"recipient"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Code      string `json:"code"`
	Password  string `json:"password"`
}

func (handler *Handler) handlePasswordResetConfirm(responseWriter http.ResponseWriter, request *http.Request) {
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
		FROM layr_auth.otps 
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
		_, _ = handler.db.Exec(ctx, "DELETE FROM layr_auth.otps WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Maximum attempts exceeded", "LAYR_AUTH_001")
		return
	}

	if !otp.VerifyCode(passwordResetConfirmRequest.Code, storedHash) {
		log.Debugf("password reset confirmation failed: invalid code for %s", recipient)
		_, _ = handler.db.Exec(ctx, "UPDATE layr_auth.otps SET attempts = attempts + 1 WHERE id = $1", otpID)
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid reset code", "LAYR_AUTH_001")
		return
	}

	// Delete used OTP
	_, _ = handler.db.Exec(ctx, "DELETE FROM layr_auth.otps WHERE id = $1", otpID)
	if handler.kvStore != nil {
		_ = handler.kvStore.Delete(ctx, fmt.Sprintf("layr:auth:otp:password_reset:%s", recipient))
	}

	passHash, _ := handler.hasher.Hash(passwordResetConfirmRequest.Password)

	var userRecord UserRecord
	var rawProps []byte
	err = handler.db.QueryRow(ctx, `
		UPDATE layr_auth.users 
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

// RegisterPasswordResetRoutes registers password reset endpoints on the provided router.
func (handler *Handler) RegisterPasswordResetRoutes(router *core.Router) {
	log.Debug("registering password reset routes on router")
	router.Mux().HandleFunc("POST /api/v1/auth/password-reset/request", handler.handlePasswordResetRequest)
	router.Mux().HandleFunc("POST /api/v1/auth/password-reset/confirm", handler.handlePasswordResetConfirm)
}
