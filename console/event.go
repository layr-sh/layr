// Package console provides the administrative control plane, operator identity, and dynamic configuration engine.
package console

import (
	"time"

	"layr.sh/core"
)

// --- 1. Runtime Configuration Events ---

// ConfigUpdatedEventData represents the payload for console.config.updated.
type ConfigUpdatedEventData Config

// NewConfigUpdatedEvent creates a typed event for console configuration updates.
func NewConfigUpdatedEvent(resourceID string, configUpdatedEventData ConfigUpdatedEventData) core.Event {
	return core.NewEvent("console.config.updated", configUpdatedEventData).WithResourceID(resourceID)
}

// --- 2. User Lifecycle Events ---

// UserCreatedEventData represents the payload for console.user.created.
type UserCreatedEventData User

// NewUserCreatedEvent creates a typed event when a console user is created.
func NewUserCreatedEvent(resourceID string, userCreatedEventData UserCreatedEventData) core.Event {
	return core.NewEvent("console.user.created", userCreatedEventData).WithResourceID(resourceID)
}

// UserUpdatedEventData represents the payload for console.user.updated.
type UserUpdatedEventData User

// NewUserUpdatedEvent creates a typed event when a console user is updated.
func NewUserUpdatedEvent(resourceID string, userUpdatedEventData UserUpdatedEventData) core.Event {
	return core.NewEvent("console.user.updated", userUpdatedEventData).WithResourceID(resourceID)
}

// UserDeletedEventData represents the payload for console.user.deleted.
type UserDeletedEventData struct {
	User                 User `json:"user"`
	RevokedSessionsCount int  `json:"revoked_sessions_count"`
}

// NewUserDeletedEvent creates a typed event when a console user is deleted.
func NewUserDeletedEvent(resourceID string, userDeletedEventData UserDeletedEventData) core.Event {
	return core.NewEvent("console.user.deleted", userDeletedEventData).WithResourceID(resourceID)
}

// UserPasswordResetEventData represents the payload for console.user.password_reset.
type UserPasswordResetEventData struct {
	User User `json:"user"`
}

// NewUserPasswordResetEvent creates a typed event when a console user's password is reset.
func NewUserPasswordResetEvent(resourceID string, userPasswordResetEventData UserPasswordResetEventData) core.Event {
	return core.NewEvent("console.user.password_reset", userPasswordResetEventData).WithResourceID(resourceID)
}

// --- 3. Session Lifecycle Events ---

// SessionCreatedEventData represents the payload for console.session.created.
type SessionCreatedEventData struct {
	ID        string    `json:"id"`
	User      User      `json:"user"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// NewSessionCreatedEvent creates a typed event when a console session is created.
func NewSessionCreatedEvent(resourceID string, sessionCreatedEventData SessionCreatedEventData) core.Event {
	return core.NewEvent("console.session.created", sessionCreatedEventData).WithResourceID(resourceID)
}

// SessionRevokedEventData represents the payload for console.session.revoked.
type SessionRevokedEventData struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	Reason    string `json:"reason"`
}

// NewSessionRevokedEvent creates a typed event when a console session is revoked.
func NewSessionRevokedEvent(resourceID string, sessionRevokedEventData SessionRevokedEventData) core.Event {
	return core.NewEvent("console.session.revoked", sessionRevokedEventData).WithResourceID(resourceID)
}
