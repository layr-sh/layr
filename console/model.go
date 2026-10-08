package console

import (
	"time"
)

// User represents an administrative user stored in console.users.
type User struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	PasswordHash  string     `json:"-"`
	MFAEnabled    bool       `json:"mfa_enabled"`
	IsEnabled     bool       `json:"is_enabled"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LastUpdatedAt time.Time  `json:"last_updated_at"`
}

// Session represents an active session record stored in console.sessions.
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Passkey represents a WebAuthn passkey credential for a console user.
type Passkey struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	CredentialID []byte    `json:"credential_id"`
	PublicKey    []byte    `json:"public_key"`
	Counter      int64     `json:"counter"`
	Transports   []string  `json:"transports"`
	FriendlyName string    `json:"friendly_name"`
	CreatedAt    time.Time `json:"created_at"`
	LastUsedAt   time.Time `json:"last_used_at"`
}
