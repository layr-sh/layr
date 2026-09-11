package auth

import (
	"time"

	"layr.sh/core"
)

// ConfigUpdatedEventData represents the payload for auth.config.updated.
type ConfigUpdatedEventData Config

// NewConfigUpdatedEvent creates a typed event for auth configuration updates.
func NewConfigUpdatedEvent(key string, configUpdatedEventData ConfigUpdatedEventData) core.Event {
	return core.NewEvent("auth.config.updated", configUpdatedEventData).WithResourceID(key)
}

// SessionCreatedEventData represents the payload for auth.session.created.
type SessionCreatedEventData struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	IPAddress *string   `json:"ip_address,omitempty"`
	UserAgent *string   `json:"user_agent,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// NewSessionCreatedEvent creates a typed event for session creation.
func NewSessionCreatedEvent(resourceID string, sessionCreatedEventData SessionCreatedEventData) core.Event {
	return core.NewEvent("auth.session.created", sessionCreatedEventData).WithResourceID(resourceID)
}

// SessionDeletedEventData represents the payload for auth.session.deleted.
type SessionDeletedEventData struct {
	SessionID    *string `json:"session_id,omitempty"`
	UserID       string  `json:"user_id"`
	RevokedCount *int    `json:"revoked_count,omitempty"`
}

// NewSessionDeletedEvent creates a typed event for session deletion or revocation.
func NewSessionDeletedEvent(resourceID string, sessionDeletedEventData SessionDeletedEventData) core.Event {
	return core.NewEvent("auth.session.deleted", sessionDeletedEventData).WithResourceID(resourceID)
}

// PasskeyCreatedEventData represents the payload for auth.passkey.created.
type PasskeyCreatedEventData struct {
	ID           string   `json:"id"`
	UserID       string   `json:"user_id"`
	FriendlyName string   `json:"friendly_name,omitempty"`
	Transports   []string `json:"transports,omitempty"`
}

// NewPasskeyCreatedEvent creates a typed event for passkey credential creation.
func NewPasskeyCreatedEvent(resourceID string, passkeyCreatedEventData PasskeyCreatedEventData) core.Event {
	return core.NewEvent("auth.passkey.created", passkeyCreatedEventData).WithResourceID(resourceID)
}

// UserSignedUpEventData represents the payload for auth.user.signed_up.
type UserSignedUpEventData UserRecord

// NewUserSignedUpEvent creates a typed event for user sign up.
func NewUserSignedUpEvent(resourceID string, userSignedUpEventData UserSignedUpEventData) core.Event {
	return core.NewEvent("auth.user.signed_up", userSignedUpEventData).WithResourceID(resourceID)
}

// UserConvertedEventData represents the payload for auth.user.converted.
type UserConvertedEventData UserRecord

// NewUserConvertedEvent creates a typed event for anonymous-to-authenticated user conversion.
func NewUserConvertedEvent(resourceID string, userConvertedEventData UserConvertedEventData) core.Event {
	return core.NewEvent("auth.user.converted", userConvertedEventData).WithResourceID(resourceID)
}

// UserEmailVerifiedEventData represents the payload for auth.user.email_verified.
type UserEmailVerifiedEventData UserRecord

// NewUserEmailVerifiedEvent creates a typed event for user email verification.
func NewUserEmailVerifiedEvent(resourceID string, userEmailVerifiedEventData UserEmailVerifiedEventData) core.Event {
	return core.NewEvent("auth.user.email_verified", userEmailVerifiedEventData).WithResourceID(resourceID)
}

// UserPhoneVerifiedEventData represents the payload for auth.user.phone_verified.
type UserPhoneVerifiedEventData UserRecord

// NewUserPhoneVerifiedEvent creates a typed event for user phone verification.
func NewUserPhoneVerifiedEvent(resourceID string, userPhoneVerifiedEventData UserPhoneVerifiedEventData) core.Event {
	return core.NewEvent("auth.user.phone_verified", userPhoneVerifiedEventData).WithResourceID(resourceID)
}

// UserUpdatedEventData represents the payload for auth.user.updated.
type UserUpdatedEventData UserRecord

// NewUserUpdatedEvent creates a typed event for user updates.
func NewUserUpdatedEvent(resourceID string, userUpdatedEventData UserUpdatedEventData) core.Event {
	return core.NewEvent("auth.user.updated", userUpdatedEventData).WithResourceID(resourceID)
}

// UserDeletedEventData represents the payload for auth.user.deleted.
type UserDeletedEventData UserRecord

// NewUserDeletedEvent creates a typed event for user account deletion.
func NewUserDeletedEvent(resourceID string, userDeletedEventData UserDeletedEventData) core.Event {
	return core.NewEvent("auth.user.deleted", userDeletedEventData).WithResourceID(resourceID)
}

// PasswordResetRequestedEventData represents the payload for auth.password.reset_requested.
type PasswordResetRequestedEventData struct {
	UserID    string `json:"user_id"`
	Recipient string `json:"recipient"`
}

// NewPasswordResetRequestedEvent creates a typed event for password reset request.
func NewPasswordResetRequestedEvent(resourceID string, passwordResetRequestedEventData PasswordResetRequestedEventData) core.Event {
	return core.NewEvent("auth.password.reset_requested", passwordResetRequestedEventData).WithResourceID(resourceID)
}

// PasswordResetEventData represents the payload for auth.password.reset.
type PasswordResetEventData struct {
	Recipient string     `json:"recipient"`
	User      UserRecord `json:"user"`
}

// NewPasswordResetEvent creates a typed event for password reset completion.
func NewPasswordResetEvent(resourceID string, passwordResetEventData PasswordResetEventData) core.Event {
	return core.NewEvent("auth.password.reset", passwordResetEventData).WithResourceID(resourceID)
}

// PasswordChangedEventData represents the payload for auth.password.changed.
type PasswordChangedEventData struct {
	User UserRecord `json:"user"`
}

// NewPasswordChangedEvent creates a typed event for password change.
func NewPasswordChangedEvent(resourceID string, passwordChangedEventData PasswordChangedEventData) core.Event {
	return core.NewEvent("auth.password.changed", passwordChangedEventData).WithResourceID(resourceID)
}

// OTPSentEventData represents the payload for auth.otp.sent.
type OTPSentEventData struct {
	UserID    string `json:"user_id"`
	Recipient string `json:"recipient"`
	Purpose   string `json:"purpose"`
}

// NewOTPSentEvent creates a typed event for dispatched one-time passwords.
func NewOTPSentEvent(resourceID string, otpSentEventData OTPSentEventData) core.Event {
	return core.NewEvent("auth.otp.sent", otpSentEventData).WithResourceID(resourceID)
}
