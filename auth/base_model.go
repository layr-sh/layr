package auth

import (
	"time"
)

// UserRecord represents a user in auth.users.
type UserRecord struct {
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

// SessionRecord represents an active refresh session in auth.sessions.
type SessionRecord struct {
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
	User   UserRecord     `json:"user"`
	Claims map[string]any `json:"claims,omitempty"`
}

// SessionResponse represents the successful authentication token payload.
type SessionResponse struct {
	User         UserRecord     `json:"user"`
	AccessToken  string         `json:"access_token"`
	RefreshToken string         `json:"refresh_token"`
	ExpiresIn    int            `json:"expires_in"`
	TokenType    string         `json:"token_type"`
	Claims       map[string]any `json:"claims,omitempty"`
}

// SignInResponse represents the response to a sign-in attempt, either returning session tokens or an MFA challenge.
type SignInResponse struct {
	SessionResponse
	MFARequired bool   `json:"mfa_required,omitempty"`
	MFATicket   string `json:"mfa_ticket,omitempty"`
	Factor      string `json:"factor,omitempty"`
}

// UserResponse represents the sanitized authenticated user details.
type UserResponse struct {
	ID            string         `json:"id"`
	Email         *string        `json:"email"`
	Phone         *string        `json:"phone"`
	Role          string         `json:"role"`
	IsAnonymous   bool           `json:"is_anonymous"`
	EmailVerified bool           `json:"email_verified"`
	PhoneVerified bool           `json:"phone_verified"`
	MFAEnabled    bool           `json:"mfa_enabled"`
	Properties    map[string]any `json:"properties"`
	CreatedAt     time.Time      `json:"created_at"`
	LastUpdatedAt time.Time      `json:"last_updated_at"`
}

// AnonymousSignInRequest represents optional parameters when initializing an anonymous session.
type AnonymousSignInRequest struct {
	Properties map[string]any `json:"properties,omitempty"`
}

// SignUpRequest defines registration parameters with password.
type SignUpRequest struct {
	Email      string         `json:"email"`
	Phone      string         `json:"phone"`
	Password   string         `json:"password"`
	Properties map[string]any `json:"properties"`
}

// SignInRequest defines login credentials.
type SignInRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

// RefreshTokenRequest defines token refresh input.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// PasswordResetRequest defines password reset request input.
type PasswordResetRequest struct {
	Recipient string `json:"recipient"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

// PasswordResetConfirmRequest defines password reset confirmation input.
type PasswordResetConfirmRequest struct {
	Recipient string `json:"recipient"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Code      string `json:"code"`
	Password  string `json:"password"`
}

// UpdateUserPasswordRequest represents the payload to set or change an account password.
type UpdateUserPasswordRequest struct {
	CurrentPassword string `json:"current_password,omitempty"`
	NewPassword     string `json:"new_password"`
}

// UpdateUserPropertiesRequest represents the payload to merge personal user properties.
type UpdateUserPropertiesRequest struct {
	Properties map[string]any `json:"properties"`
}

// UpdateUserPropertiesResponse represents the updated custom properties of the authenticated user.
type UpdateUserPropertiesResponse struct {
	Properties map[string]any `json:"properties"`
}

// UpdateUserEmailRequest represents the payload to request updating user email.
type UpdateUserEmailRequest struct {
	Email string `json:"email"`
}

// UpdateUserPhoneRequest represents the payload to request updating user phone number.
type UpdateUserPhoneRequest struct {
	Phone string `json:"phone"`
}

// UserEmailVerificationRequest represents the payload to request an email verification code.
type UserEmailVerificationRequest struct {
	Email string `json:"email"`
}

// UserEmailVerificationConfirmRequest represents the payload to confirm email verification with a code.
type UserEmailVerificationConfirmRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// UserPhoneVerificationRequest represents the payload to request a phone verification code.
type UserPhoneVerificationRequest struct {
	Phone string `json:"phone"`
}

// UserPhoneVerificationConfirmRequest represents the payload to confirm phone verification with a code.
type UserPhoneVerificationConfirmRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

// UserSessionRecord represents a safe, public view of an active user session.
type UserSessionRecord struct {
	ID        string    `json:"id"`
	IPAddress *string   `json:"ip_address,omitempty"`
	UserAgent *string   `json:"user_agent,omitempty"`
	IsCurrent bool      `json:"is_current"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// ListUserSessionsResponse represents the list of active sessions for the current user.
type ListUserSessionsResponse struct {
	Sessions []UserSessionRecord `json:"sessions"`
	Count    int                 `json:"count"`
}

// RevokeOtherSessionsResponse represents the result of revoking other devices.
type RevokeOtherSessionsResponse struct {
	RevokedCount int64 `json:"revoked_count"`
}

// OTPSendRequest defines parameters for dispatching a one-time passcode.
type OTPSendRequest struct {
	Recipient string `json:"recipient"`
	Purpose   string `json:"purpose"` // 'sign_in' | 'sign_up' | 'mfa'
}

// OTPVerifyRequest defines parameters for verifying a one-time passcode.
type OTPVerifyRequest struct {
	Recipient string `json:"recipient"`
	Code      string `json:"code"`
	Purpose   string `json:"purpose"`
}

// MFASetupRequest defines input for TOTP MFA setup.
type MFASetupRequest struct {
	UserID string `json:"user_id,omitempty"`
}

// MFASetupResponse defines response payload for TOTP MFA setup.
type MFASetupResponse struct {
	Secret        string `json:"secret"`
	AuthURL       string `json:"auth_url"`
	Issuer        string `json:"issuer"`
	Digits        int    `json:"digits"`
	PeriodSeconds int    `json:"period_seconds"`
}

// MFAVerifyRequest defines input for TOTP MFA verification.
type MFAVerifyRequest struct {
	UserID string `json:"user_id,omitempty"`
	Code   string `json:"code"`
}

// MFAChallengeRequest defines input to satisfy an MFA challenge during sign-in.
type MFAChallengeRequest struct {
	MFATicket string `json:"mfa_ticket"`
	Code      string `json:"code"`
}

// PasskeySignUpRequest defines input for passkey sign up ceremony.
type PasskeySignUpRequest struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
}

// PasskeySignUpVerifyRequest defines input to verify and store passkey credentials.
type PasskeySignUpVerifyRequest struct {
	UserID       string   `json:"user_id"`
	Challenge    string   `json:"challenge"`
	CredentialID string   `json:"credential_id"`
	PublicKey    string   `json:"public_key"`
	FriendlyName string   `json:"friendly_name"`
	Transports   []string `json:"transports"`
}

// PasskeySignInVerifyRequest defines input to complete passkey assertion ceremony.
type PasskeySignInVerifyRequest struct {
	Challenge         string `json:"challenge"`
	CredentialID      string `json:"credential_id"`
	ClientDataJSON    string `json:"client_data_json,omitempty"`
	AuthenticatorData string `json:"authenticator_data,omitempty"`
	Signature         string `json:"signature,omitempty"`
}

// UserPasskeyResponse represents a user's registered passkey credential.
type UserPasskeyResponse struct {
	ID           string    `json:"id"`
	FriendlyName string    `json:"friendly_name"`
	Transports   []string  `json:"transports"`
	CreatedAt    time.Time `json:"created_at"`
	LastUsedAt   time.Time `json:"last_used_at"`
}

// OAuthTokenExchangeRequest defines the body for OAuth token exchange.
type OAuthTokenExchangeRequest struct {
	Provider    string `json:"provider"`
	Code        string `json:"code"`
	RedirectURI string `json:"redirect_uri"`
	State       string `json:"state"`
}

// OAuthStatePayload enhances federated social sign-in state with return context and OIDC flow linkage.
type OAuthStatePayload struct {
	StateID       string `json:"state_id"`
	Provider      string `json:"provider"`
	RedirectURI   string `json:"redirect_uri"`
	OIDCStateID   string `json:"oidc_state_id,omitempty"`
	AnonymousID   string `json:"anonymous_id,omitempty"`
	CreatedAtUnix int64  `json:"created_at_unix"`
}

// OIDCAuthorizationStatePayload tracks and cryptographically verifies the authorization session.
type OIDCAuthorizationStatePayload struct {
	StateID             string    `json:"state_id"`
	ClientID            string    `json:"client_id"`
	RedirectURI         string    `json:"redirect_uri"`
	Scope               string    `json:"scope"`
	ClientState         string    `json:"client_state"`
	CodeChallenge       string    `json:"code_challenge"`
	CodeChallengeMethod string    `json:"code_challenge_method"`
	Nonce               string    `json:"nonce,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

// OIDCAuthorizationCodePayload stores the issued authorization code data in kvStore (5-min TTL).
type OIDCAuthorizationCodePayload struct {
	Code                string `json:"code"`
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	UserID              string `json:"user_id"`
	Scope               string `json:"scope"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	Nonce               string `json:"nonce,omitempty"`
}

// OIDCTokenResponse represents the standard OAuth 2.0 / OIDC token response.
type OIDCTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// OAuthTokenRequest represents incoming parameters for OAuth 2.0 token requests.
type OAuthTokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	CodeVerifier string `json:"code_verifier"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	Audience     string `json:"audience,omitempty"`
	Provider     string `json:"provider"`
}

// OIDCUserInfoResponse represents OpenID Connect Core 1.0 standard claims.
type OIDCUserInfoResponse struct {
	Subject             string         `json:"sub"`
	Name                string         `json:"name,omitempty"`
	Email               *string        `json:"email,omitempty"`
	EmailVerified       bool           `json:"email_verified"`
	PhoneNumber         *string        `json:"phone_number,omitempty"`
	PhoneNumberVerified bool           `json:"phone_number_verified"`
	Role                string         `json:"role"`
	IsAnonymous         bool           `json:"is_anonymous"`
	UpdatedAt           int64          `json:"updated_at"`
	Properties          map[string]any `json:"properties,omitempty"`
}

// ExportIdentityRecord represents a linked third-party identity in user data export.
type ExportIdentityRecord struct {
	Provider       string         `json:"provider"`
	ProviderUserID string         `json:"provider_user_id"`
	Properties     map[string]any `json:"properties"`
	CreatedAt      time.Time      `json:"created_at"`
	LastSignInAt   time.Time      `json:"last_sign_in_at"`
}

// ExportUserDataResponse represents comprehensive user account data graph for GDPR compliance.
type ExportUserDataResponse struct {
	User       UserRecord             `json:"user"`
	Identities []ExportIdentityRecord `json:"identities"`
	ExportDate string                 `json:"export_date"`
}
