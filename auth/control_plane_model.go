package auth

import (
	"time"
)

// UserCreateRequest defines parameters for creating an application user via control plane.
type UserCreateRequest struct {
	Email         string         `json:"email"`
	Phone         string         `json:"phone"`
	Password      string         `json:"password"`
	Role          string         `json:"role"`
	EmailVerified bool           `json:"email_verified"`
	PhoneVerified bool           `json:"phone_verified"`
	Properties    map[string]any `json:"properties"`
}

// UserLockRequest defines parameters for locking an application user account.
type UserLockRequest struct {
	LockedUntil *time.Time `json:"locked_until"`
}
