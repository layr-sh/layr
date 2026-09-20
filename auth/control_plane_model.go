package auth

import (
	"time"
)

// CreateUserInput defines parameters for creating an application user via control plane.
type CreateUserInput struct {
	Email         string         `json:"email"`
	Phone         string         `json:"phone"`
	Password      string         `json:"password"`
	Role          string         `json:"role"`
	EmailVerified bool           `json:"email_verified"`
	PhoneVerified bool           `json:"phone_verified"`
	Properties    map[string]any `json:"properties"`
}

// LockUserInput defines parameters for locking an application user account.
type LockUserInput struct {
	LockedUntil *time.Time `json:"locked_until"`
}

// ListUsersResponse represents the paginated list of application users.
type ListUsersResponse struct {
	Users  []User `json:"users"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Count  int    `json:"count"`
}

// ListUserSessionsResponse represents the list of active sessions for an application user in the control plane.
type ListUserSessionsResponse struct {
	Sessions []Session `json:"sessions"`
	Count    int       `json:"count"`
}
