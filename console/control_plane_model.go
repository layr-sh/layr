package console

// SignInConsoleInput holds login credentials for console users.
type SignInConsoleInput struct {
	Email    string  `json:"email"`
	Password string  `json:"password"`
	MFACode  *string `json:"mfa_code,omitempty"`
}

// SignInConsoleResponse returns authentication credentials and session details upon sign-in.
type SignInConsoleResponse struct {
	User        User   `json:"user,omitempty"`
	AccessToken string `json:"access_token,omitempty"`
	ExpiresIn   int    `json:"expires_in,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	MFARequired bool   `json:"mfa_required,omitempty"`
	MFATicket   string `json:"mfa_ticket,omitempty"`
	Factor      string `json:"factor,omitempty"`
}

// GetConsoleSessionResponse returns profile and status of the currently authenticated console session.
type GetConsoleSessionResponse struct {
	User    User     `json:"user"`
	Session Session  `json:"session"`
	Scopes  []string `json:"scopes"`
}

// CreateConsoleUserInput holds input parameters for creating a new console user.
type CreateConsoleUserInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UpdateConsoleUserInput holds input parameters for updating a console user.
type UpdateConsoleUserInput struct {
	Email     *string `json:"email,omitempty"`
	IsEnabled *bool   `json:"is_enabled,omitempty"`
}

// ResetConsoleUserPasswordInput holds the new password when resetting a console user account.
type ResetConsoleUserPasswordInput struct {
	NewPassword string `json:"new_password"`
}

// SetupConsoleMFAResponse returns the TOTP secret and provisioning URI.
type SetupConsoleMFAResponse struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

// VerifyConsoleMFAInput holds the TOTP code to verify and activate MFA.
type VerifyConsoleMFAInput struct {
	Code string `json:"code"`
}

// ChallengeConsoleMFAInput holds the MFA ticket and TOTP code to verify.
type ChallengeConsoleMFAInput struct {
	MFATicket string `json:"mfa_ticket"`
	Code      string `json:"code"`
}

// ListConsoleUsersResponse represents a list of console user accounts.
type ListConsoleUsersResponse []User

// ListConsoleUserSessionsResponse represents active sessions for a console user.
type ListConsoleUserSessionsResponse []Session
