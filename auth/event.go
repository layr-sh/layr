package auth

import (
	"time"

	"layr.sh/core"
)

// ConfigUpdatedEventData represents the payload for auth.config.updated.
type ConfigUpdatedEventData Config

// NewConfigUpdatedEvent creates a typed event for auth configuration updates.
func NewConfigUpdatedEvent(resourceID string, configUpdatedEventData ConfigUpdatedEventData) core.Event {
	return core.NewEvent("auth.config.updated", configUpdatedEventData).WithResourceID(resourceID)
}

// SessionCreatedEventData represents the payload for auth.session.created.
type SessionCreatedEventData struct {
	ID         string    `json:"id"`
	User       User      `json:"user"`
	AuthMethod string    `json:"auth_method,omitempty"`
	Provider   string    `json:"provider,omitempty"`
	IPAddress  *string   `json:"ip_address,omitempty"`
	UserAgent  *string   `json:"user_agent,omitempty"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// NewSessionCreatedEvent creates a typed event for session creation.
func NewSessionCreatedEvent(resourceID string, sessionCreatedEventData SessionCreatedEventData) core.Event {
	return core.NewEvent("auth.session.created", sessionCreatedEventData).WithResourceID(resourceID)
}

// SessionDeletedEventData represents the payload for auth.session.deleted.
type SessionDeletedEventData struct {
	User         User    `json:"user"`
	SessionID    *string `json:"session_id,omitempty"`
	RevokedCount *int    `json:"revoked_count,omitempty"`
}

// NewSessionDeletedEvent creates a typed event for session deletion or revocation.
func NewSessionDeletedEvent(resourceID string, sessionDeletedEventData SessionDeletedEventData) core.Event {
	return core.NewEvent("auth.session.deleted", sessionDeletedEventData).WithResourceID(resourceID)
}

// PasskeyCreatedEventData represents the payload for auth.passkey.created.
type PasskeyCreatedEventData struct {
	ID           string   `json:"id"`
	User         User     `json:"user"`
	FriendlyName string   `json:"friendly_name,omitempty"`
	Transports   []string `json:"transports,omitempty"`
}

// NewPasskeyCreatedEvent creates a typed event for passkey credential creation.
func NewPasskeyCreatedEvent(resourceID string, passkeyCreatedEventData PasskeyCreatedEventData) core.Event {
	return core.NewEvent("auth.passkey.created", passkeyCreatedEventData).WithResourceID(resourceID)
}

// PasskeyDeletedEventData represents the payload for auth.passkey.deleted.
type PasskeyDeletedEventData struct {
	ID   string `json:"id"`
	User User   `json:"user"`
}

// NewPasskeyDeletedEvent creates a typed event for passkey credential revocation.
func NewPasskeyDeletedEvent(resourceID string, passkeyDeletedEventData PasskeyDeletedEventData) core.Event {
	return core.NewEvent("auth.passkey.deleted", passkeyDeletedEventData).WithResourceID(resourceID)
}

// UserSignedUpEventData represents the payload for auth.user.signed_up.
type UserSignedUpEventData User

// NewUserSignedUpEvent creates a typed event for user sign up.
func NewUserSignedUpEvent(resourceID string, userSignedUpEventData UserSignedUpEventData) core.Event {
	return core.NewEvent("auth.user.signed_up", userSignedUpEventData).WithResourceID(resourceID)
}

// UserConvertedEventData represents the payload for auth.user.converted.
type UserConvertedEventData User

// NewUserConvertedEvent creates a typed event for anonymous-to-authenticated user conversion.
func NewUserConvertedEvent(resourceID string, userConvertedEventData UserConvertedEventData) core.Event {
	return core.NewEvent("auth.user.converted", userConvertedEventData).WithResourceID(resourceID)
}

// UserEmailVerifiedEventData represents the payload for auth.user.email_verified.
type UserEmailVerifiedEventData User

// NewUserEmailVerifiedEvent creates a typed event for user email verification.
func NewUserEmailVerifiedEvent(resourceID string, userEmailVerifiedEventData UserEmailVerifiedEventData) core.Event {
	return core.NewEvent("auth.user.email_verified", userEmailVerifiedEventData).WithResourceID(resourceID)
}

// UserPhoneVerifiedEventData represents the payload for auth.user.phone_verified.
type UserPhoneVerifiedEventData User

// NewUserPhoneVerifiedEvent creates a typed event for user phone verification.
func NewUserPhoneVerifiedEvent(resourceID string, userPhoneVerifiedEventData UserPhoneVerifiedEventData) core.Event {
	return core.NewEvent("auth.user.phone_verified", userPhoneVerifiedEventData).WithResourceID(resourceID)
}

// UserUpdatedEventData represents the payload for auth.user.updated.
type UserUpdatedEventData User

// NewUserUpdatedEvent creates a typed event for user updates.
func NewUserUpdatedEvent(resourceID string, userUpdatedEventData UserUpdatedEventData) core.Event {
	return core.NewEvent("auth.user.updated", userUpdatedEventData).WithResourceID(resourceID)
}

// UserDeletedEventData represents the payload for auth.user.deleted.
type UserDeletedEventData User

// NewUserDeletedEvent creates a typed event for user account deletion.
func NewUserDeletedEvent(resourceID string, userDeletedEventData UserDeletedEventData) core.Event {
	return core.NewEvent("auth.user.deleted", userDeletedEventData).WithResourceID(resourceID)
}

// PasswordResetRequestedEventData represents the payload for auth.password.reset_requested.
type PasswordResetRequestedEventData struct {
	Recipient string `json:"recipient"`
	User      User   `json:"user"`
}

// NewPasswordResetRequestedEvent creates a typed event for password reset request.
func NewPasswordResetRequestedEvent(resourceID string, passwordResetRequestedEventData PasswordResetRequestedEventData) core.Event {
	return core.NewEvent("auth.password.reset_requested", passwordResetRequestedEventData).WithResourceID(resourceID)
}

// PasswordResetEventData represents the payload for auth.password.reset.
type PasswordResetEventData struct {
	Recipient string `json:"recipient"`
	User      User   `json:"user"`
}

// NewPasswordResetEvent creates a typed event for password reset completion.
func NewPasswordResetEvent(resourceID string, passwordResetEventData PasswordResetEventData) core.Event {
	return core.NewEvent("auth.password.reset", passwordResetEventData).WithResourceID(resourceID)
}

// PasswordChangedEventData represents the payload for auth.password.changed.
type PasswordChangedEventData User

// NewPasswordChangedEvent creates a typed event for password change.
func NewPasswordChangedEvent(resourceID string, passwordChangedEventData PasswordChangedEventData) core.Event {
	return core.NewEvent("auth.password.changed", passwordChangedEventData).WithResourceID(resourceID)
}

// MFAEnabledEventData represents the payload for auth.mfa.enabled.
type MFAEnabledEventData User

// NewMFAEnabledEvent creates a typed event for multi-factor authentication enablement.
func NewMFAEnabledEvent(resourceID string, mfaEnabledEventData MFAEnabledEventData) core.Event {
	return core.NewEvent("auth.mfa.enabled", mfaEnabledEventData).WithResourceID(resourceID)
}

// MFADisabledEventData represents the payload for auth.mfa.disabled.
type MFADisabledEventData User

// NewMFADisabledEvent creates a typed event for multi-factor authentication disablement.
func NewMFADisabledEvent(resourceID string, mfaDisabledEventData MFADisabledEventData) core.Event {
	return core.NewEvent("auth.mfa.disabled", mfaDisabledEventData).WithResourceID(resourceID)
}

// OTPSentEventData represents the payload for auth.otp.sent.
type OTPSentEventData struct {
	Recipient string `json:"recipient"`
	Purpose   string `json:"purpose"`
	Channel   string `json:"channel"`
	User      *User  `json:"user,omitempty"`
}

// NewOTPSentEvent creates a typed event for dispatched one-time passwords.
func NewOTPSentEvent(resourceID string, otpSentEventData OTPSentEventData) core.Event {
	return core.NewEvent("auth.otp.sent", otpSentEventData).WithResourceID(resourceID)
}

// OTPVerifiedEventData represents the payload for auth.otp.verified.
type OTPVerifiedEventData struct {
	Recipient string `json:"recipient"`
	Purpose   string `json:"purpose"`
	Channel   string `json:"channel"`
	User      *User  `json:"user,omitempty"`
}

// NewOTPVerifiedEvent creates a typed event for verified one-time passwords.
func NewOTPVerifiedEvent(resourceID string, otpVerifiedEventData OTPVerifiedEventData) core.Event {
	return core.NewEvent("auth.otp.verified", otpVerifiedEventData).WithResourceID(resourceID)
}

// UserCreatedEventData represents the payload for auth.user.created.
type UserCreatedEventData User

// NewUserCreatedEvent creates a typed event for user creation.
func NewUserCreatedEvent(resourceID string, userCreatedEventData UserCreatedEventData) core.Event {
	return core.NewEvent("auth.user.created", userCreatedEventData).WithResourceID(resourceID)
}

// UserLockedEventData represents the payload for auth.user.locked.
type UserLockedEventData struct {
	User        User       `json:"user"`
	LockedUntil *time.Time `json:"locked_until,omitempty"`
}

// NewUserLockedEvent creates a typed event for user account lockout.
func NewUserLockedEvent(resourceID string, userLockedEventData UserLockedEventData) core.Event {
	return core.NewEvent("auth.user.locked", userLockedEventData).WithResourceID(resourceID)
}

// UserUnlockedEventData represents the payload for auth.user.unlocked.
type UserUnlockedEventData User

// NewUserUnlockedEvent creates a typed event for user account unlocking.
func NewUserUnlockedEvent(resourceID string, userUnlockedEventData UserUnlockedEventData) core.Event {
	return core.NewEvent("auth.user.unlocked", userUnlockedEventData).WithResourceID(resourceID)
}

// PasswordBreachBlockedEventData represents the payload for auth.threat.password_breach_blocked.
type PasswordBreachBlockedEventData struct {
	Email string `json:"email,omitempty"`
	Count int64  `json:"count"`
}

// NewPasswordBreachBlockedEvent creates a typed event for blocked breached password attempts.
func NewPasswordBreachBlockedEvent(resourceID string, passwordBreachBlockedEventData PasswordBreachBlockedEventData) core.Event {
	return core.NewEvent("auth.threat.password_breach_blocked", passwordBreachBlockedEventData).WithResourceID(resourceID)
}

// BotChallengeFailedEventData represents the payload for auth.threat.bot_challenge_failed.
type BotChallengeFailedEventData struct {
	IPAddress string `json:"ip_address"`
	Provider  string `json:"provider"`
	Endpoint  string `json:"endpoint"`
}

// NewBotChallengeFailedEvent creates a typed event for failed bot/CAPTCHA verification challenges.
func NewBotChallengeFailedEvent(resourceID string, botChallengeFailedEventData BotChallengeFailedEventData) core.Event {
	return core.NewEvent("auth.threat.bot_challenge_failed", botChallengeFailedEventData).WithResourceID(resourceID)
}

// SuspiciousSignInEventData represents the payload for auth.user.suspicious_sign_in.
type SuspiciousSignInEventData struct {
	User      User     `json:"user"`
	IPAddress string   `json:"ip_address"`
	UserAgent string   `json:"user_agent"`
	RiskScore int      `json:"risk_score"`
	RiskLevel string   `json:"risk_level"`
	Reasons   []string `json:"reasons"`
}

// NewSuspiciousSignInEvent creates a typed event for anomalous or new-device sign-in detections.
func NewSuspiciousSignInEvent(resourceID string, suspiciousSignInEventData SuspiciousSignInEventData) core.Event {
	return core.NewEvent("auth.user.suspicious_sign_in", suspiciousSignInEventData).WithResourceID(resourceID)
}

// UserSignInFailedEventData represents the payload for auth.user.sign_in_failed.
type UserSignInFailedEventData struct {
	Identifier string `json:"identifier"`
	AuthMethod string `json:"auth_method"`
	Reason     string `json:"reason"`
	IPAddress  string `json:"ip_address"`
	UserAgent  string `json:"user_agent"`
	User       *User  `json:"user,omitempty"`
}

// NewUserSignInFailedEvent creates a typed event for failed authentication attempts.
func NewUserSignInFailedEvent(resourceID string, userSignInFailedEventData UserSignInFailedEventData) core.Event {
	return core.NewEvent("auth.user.sign_in_failed", userSignInFailedEventData).WithResourceID(resourceID)
}

// MFAChallengeFailedEventData represents the payload for auth.mfa.challenge_failed.
type MFAChallengeFailedEventData struct {
	UserID    string `json:"user_id,omitempty"`
	Reason    string `json:"reason"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
	User      *User  `json:"user,omitempty"`
}

// NewMFAChallengeFailedEvent creates a typed event for failed multi-factor authentication challenges.
func NewMFAChallengeFailedEvent(resourceID string, mfaChallengeFailedEventData MFAChallengeFailedEventData) core.Event {
	return core.NewEvent("auth.mfa.challenge_failed", mfaChallengeFailedEventData).WithResourceID(resourceID)
}

// OTPVerificationFailedEventData represents the payload for auth.otp.verification_failed.
type OTPVerificationFailedEventData struct {
	Recipient string `json:"recipient"`
	Purpose   string `json:"purpose"`
	Channel   string `json:"channel"`
	Reason    string `json:"reason"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
}

// NewOTPVerificationFailedEvent creates a typed event for failed one-time password verifications.
func NewOTPVerificationFailedEvent(resourceID string, otpVerificationFailedEventData OTPVerificationFailedEventData) core.Event {
	return core.NewEvent("auth.otp.verification_failed", otpVerificationFailedEventData).WithResourceID(resourceID)
}

// RateLimitExceededEventData represents the payload for auth.threat.rate_limit_exceeded.
type RateLimitExceededEventData struct {
	Identifier   string `json:"identifier"`
	Endpoint     string `json:"endpoint"`
	AttemptCount int64  `json:"attempt_count"`
	IPAddress    string `json:"ip_address"`
	UserAgent    string `json:"user_agent"`
}

// NewRateLimitExceededEvent creates a typed event for rate limit threshold exhaustion.
func NewRateLimitExceededEvent(resourceID string, rateLimitExceededEventData RateLimitExceededEventData) core.Event {
	return core.NewEvent("auth.threat.rate_limit_exceeded", rateLimitExceededEventData).WithResourceID(resourceID)
}

// UserExportedEventData represents the payload for auth.user.exported.
type UserExportedEventData User

// NewUserExportedEvent creates a typed event for user account data export under GDPR.
func NewUserExportedEvent(resourceID string, userExportedEventData UserExportedEventData) core.Event {
	return core.NewEvent("auth.user.exported", userExportedEventData).WithResourceID(resourceID)
}

// IdentityLinkedEventData represents the payload for auth.identity.linked.
type IdentityLinkedEventData struct {
	UserID         string         `json:"user_id"`
	Provider       string         `json:"provider"`
	ProviderUserID string         `json:"provider_user_id"`
	Properties     map[string]any `json:"properties,omitempty"`
	User           User           `json:"user"`
}

// NewIdentityLinkedEvent creates a typed event for federated identity linking.
func NewIdentityLinkedEvent(resourceID string, identityLinkedEventData IdentityLinkedEventData) core.Event {
	return core.NewEvent("auth.identity.linked", identityLinkedEventData).WithResourceID(resourceID)
}

// TokenRefreshedEventData represents the payload for auth.token.refreshed.
type TokenRefreshedEventData struct {
	SessionID string    `json:"session_id"`
	User      User      `json:"user"`
	IPAddress *string   `json:"ip_address,omitempty"`
	UserAgent *string   `json:"user_agent,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewTokenRefreshedEvent creates a typed event for access/refresh token rotation.
func NewTokenRefreshedEvent(resourceID string, tokenRefreshedEventData TokenRefreshedEventData) core.Event {
	return core.NewEvent("auth.token.refreshed", tokenRefreshedEventData).WithResourceID(resourceID)
}
