package console

// SignInConsoleInput holds login credentials for console users.
type SignInConsoleInput struct {
	Email    string  `json:"email"`
	Password string  `json:"password"`
	MFACode  *string `json:"mfa_code,omitempty"`
}

// SignInConsoleResponse returns authentication credentials and session details upon sign-in.
type SignInConsoleResponse struct {
	User        User   `json:"user"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
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

// ListConsoleUsersResponse represents a list of console user accounts.
type ListConsoleUsersResponse []User

// ListConsoleUserSessionsResponse represents active sessions for a console user.
type ListConsoleUserSessionsResponse []Session
