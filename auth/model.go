package auth

import (
	"time"
)

// User represents a user in auth.users.
type User struct {
	ID                 string         `json:"id"`
	Email              *string        `json:"email"`
	Phone              *string        `json:"phone"`
	PasswordHash       *string        `json:"-"`
	EncryptedMFASecret *string        `json:"-"`
	MFAEnabled         bool           `json:"mfa_enabled"`
	Role               string         `json:"role"`
	IsAnonymous        bool           `json:"is_anonymous"`
	EmailVerifiedAt    *time.Time     `json:"email_verified_at,omitempty"`
	PhoneVerifiedAt    *time.Time     `json:"phone_verified_at,omitempty"`
	LockedUntil        *time.Time     `json:"locked_until,omitempty"`
	Properties         map[string]any `json:"properties"`
	CreatedAt          time.Time      `json:"created_at"`
	LastUpdatedAt      time.Time      `json:"last_updated_at"`
}

// Session represents an active refresh session in auth.sessions.
type Session struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	ClientID         *string   `json:"client_id,omitempty"`
	RefreshTokenHash string    `json:"refresh_token_hash"`
	IPAddress        *string   `json:"ip_address,omitempty"`
	UserAgent        *string   `json:"user_agent,omitempty"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
}

// CachedSession represents the cached user session data in kvstore for fast-path validation.
type CachedSession struct {
	User   User           `json:"user"`
	Claims map[string]any `json:"claims,omitempty"`
}

// Passkey represents a user's registered passkey credential.
type Passkey struct {
	ID           string    `json:"id"`
	FriendlyName string    `json:"friendly_name"`
	Transports   []string  `json:"transports"`
	CreatedAt    time.Time `json:"created_at"`
	LastUsedAt   time.Time `json:"last_used_at"`
}
