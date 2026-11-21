package auth

import (
	"time"

	"layr.sh/core"
)

// StatsResponse reports directory-wide application user, credential, and session counts.
// Active sessions exclude expired, idle, and locked-user sessions; they are not sign-in event counts.
type StatsResponse struct {
	TotalUsers              int64               `json:"total_users"`
	RegisteredUsers         int64               `json:"registered_users"`
	AnonymousUsers          int64               `json:"anonymous_users"`
	LockedUsers             int64               `json:"locked_users"`
	MFAUsers                int64               `json:"mfa_users"`
	EmailVerifiedUsers      int64               `json:"email_verified_users"`
	PhoneVerifiedUsers      int64               `json:"phone_verified_users"`
	PasswordUsers           int64               `json:"password_users"`
	PasskeyUsers            int64               `json:"passkey_users"`
	FederatedUsers          int64               `json:"federated_users"`
	NewUsersLast24Hours     int64               `json:"new_users_last_24_hours"`
	TotalSessions           int64               `json:"total_sessions"`
	ActiveSessions          int64               `json:"active_sessions"`
	ActiveSessionUsers      int64               `json:"active_session_users"`
	TotalPasskeys           int64               `json:"total_passkeys"`
	TotalIdentities         int64               `json:"total_identities"`
	GeneratedAt             time.Time           `json:"generated_at"`
	NewUsersPrevious24Hours int64               `json:"new_users_previous_24_hours"`
	NewUsersLast7Days       int64               `json:"new_users_last_7_days"`
	EmailUnverifiedUsers    int64               `json:"email_unverified_users"`
	PhoneUnverifiedUsers    int64               `json:"phone_unverified_users"`
	UsersWithoutCredentials int64               `json:"users_without_credentials"`
	ExpiredSessions         int64               `json:"expired_sessions"`
	LockedSessions          int64               `json:"locked_sessions"`
	IdleSessions            int64               `json:"idle_sessions"`
	Providers               []ProviderStats     `json:"providers"`
	Roles                   []RoleStats         `json:"roles"`
	Registrations           []RegistrationStats `json:"registrations"`
}

// ProviderStats summarizes currently linked identities, not historical sign-in volume.
type ProviderStats struct {
	Provider       string     `json:"provider"`
	IdentityCount  int64      `json:"identity_count"`
	UserCount      int64      `json:"user_count"`
	LastSignedInAt *time.Time `json:"last_signed_in_at"`
}

// RoleStats summarizes the current application user directory by JWT role.
type RoleStats struct {
	Role       string `json:"role"`
	TotalUsers int64  `json:"total_users"`
}

// RegistrationStats counts retained accounts created on a UTC calendar day.
// The response includes zero-filled days and does not reconstruct deleted accounts.
type RegistrationStats struct {
	Day             time.Time `json:"day"`
	TotalUsers      int64     `json:"total_users"`
	RegisteredUsers int64     `json:"registered_users"`
	AnonymousUsers  int64     `json:"anonymous_users"`
}

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
	Data []User `json:"data"`
	core.PaginationOutput
}

// ListUserSessionsResponse represents the list of active sessions for an application user in the control plane.
type ListUserSessionsResponse struct {
	Data  []Session `json:"data"`
	Count int       `json:"count"`
}
