package auth

import (
	"layr.sh/auth/passkey"
	"layr.sh/core"
)

// RegisterRoutes registers all Auth service REST and OpenAPI 3.1 endpoints on public and control plane routers.
func (service *Service) RegisterRoutes(baseRouter *core.Router, controlPlaneRouter *core.Router) {
	if baseRouter != nil {
		service.registerBaseRoutes(baseRouter)
	}
	if controlPlaneRouter != nil {
		service.registerControlPlaneRoutes(controlPlaneRouter)
	}
}

func (service *Service) registerBaseRoutes(router *core.Router) {
	if service.baseHandler == nil {
		return
	}

	// 1. OIDC Discovery & JWKS
	core.GetRoute[OIDCConfiguration](router, "/.well-known/openid-configuration", service.baseHandler.handleOIDCDiscovery,
		core.RouteTag("OpenID Connect"),
		core.RouteSummary("OpenID Connect discovery document"),
		core.RouteDescription("Public OpenID Connect discovery metadata document defining issuer, authorization, token, and JWKS endpoints."),
		core.RouteOperationID("auth__openid_configuration"),
		core.RouteSDKGroupName("auth"),
		core.RouteSDKMethodName("openidConfiguration"),
	)
	core.GetRoute[core.JWKS](router, "/.well-known/jwks.json", service.baseHandler.handleJWKS,
		core.RouteTag("OpenID Connect"),
		core.RouteSummary("JSON Web Key Set (JWKS) public verification keys"),
		core.RouteDescription("Public JSON Web Key Set (JWKS) containing active Ed25519 public verification keys."),
		core.RouteOperationID("auth__jwks"),
		core.RouteSDKGroupName("auth"),
		core.RouteSDKMethodName("jwks"),
	)

	// 2. Anonymous Auth
	core.PostRoute[SessionResponse, AnonymousSignInRequest](router, "/api/v1/auth/anonymous", service.baseHandler.handleAnonymousSignIn,
		core.RouteTag("Authentication"),
		core.RouteSummary("Sign in or initialize an anonymous guest user account"),
		core.RouteDescription("Issues an Ed25519 JWT access token and refresh token for an anonymous guest user with null email/phone and is_anonymous set to true."),
		core.RouteOperationID("auth__anonymous__sign_in"),
		core.RouteSDKGroupName("auth", "anonymous"),
		core.RouteSDKMethodName("signIn"),
	)

	// 3. Password Auth
	core.PostRoute[SessionResponse, SignUpRequest](router, "/api/v1/auth/sign-up", service.baseHandler.handleSignUp,
		core.RouteTag("Authentication"),
		core.RouteSummary("Register a new user with email/phone and password"),
		core.RouteDescription("Registers a new application user account with argon2id password hashing, optional auto-sign-in, and welcome email verification."),
		core.RouteOperationID("auth__sign_up"),
		core.RouteSDKGroupName("auth"),
		core.RouteSDKMethodName("signUp"),
	)
	core.PostRoute[SessionResponse, SignInRequest](router, "/api/v1/auth/sign-in", service.baseHandler.handleSignIn,
		core.RouteTag("Authentication"),
		core.RouteSummary("Authenticate with email/phone and password"),
		core.RouteDescription("Authenticates an application user with email/phone and password, returning an Ed25519 JWT access token, rotated refresh token, and custom claims resolved via public.auth_claims."),
		core.RouteOperationID("auth__sign_in"),
		core.RouteSDKGroupName("auth"),
		core.RouteSDKMethodName("signIn"),
	)
	core.PostRoute[core.Empty, core.Empty](router, "/api/v1/auth/sign-out", service.baseHandler.handleSignOut,
		core.RouteTag("Authentication"),
		core.RouteSummary("Revoke active session and refresh token"),
		core.RouteDescription("Revokes the active refresh token and invalidates the session record in auth.sessions."),
		core.RouteNoRequestBody(),
		core.RouteNoContentResponse("Signed out successfully"),
		core.RouteOperationID("auth__sign_out"),
		core.RouteSDKGroupName("auth"),
		core.RouteSDKMethodName("signOut"),
	)
	core.PostRoute[SessionResponse, RefreshTokenRequest](router, "/api/v1/auth/token/refresh", service.baseHandler.handleTokenRefresh,
		core.RouteTag("Authentication"),
		core.RouteSummary("Exchange refresh token for fresh JWT access token"),
		core.RouteDescription("Exchanges a valid refresh token for a freshly signed Ed25519 JWT access token with updated claims resolved via public.auth_claims."),
		core.RouteOperationID("auth__token__refresh"),
		core.RouteSDKGroupName("auth", "token"),
		core.RouteSDKMethodName("refresh"),
	)
	core.PostRoute[core.Empty, PasswordResetRequest](router, "/api/v1/auth/password-reset/request", service.baseHandler.handlePasswordResetRequest,
		core.RouteTag("Authentication"),
		core.RouteSummary("Request a password reset verification code via email or SMS"),
		core.RouteDescription("Initiates password recovery by dispatching a single-use verification code to the registered email or SMS number."),
		core.RouteNoContentResponse("Password reset code sent"),
		core.RouteOperationID("auth__password_reset__request"),
		core.RouteSDKGroupName("auth", "passwordReset"),
		core.RouteSDKMethodName("request"),
	)
	core.PostRoute[core.Empty, PasswordResetConfirmRequest](router, "/api/v1/auth/password-reset/confirm", service.baseHandler.handlePasswordResetConfirm,
		core.RouteTag("Authentication"),
		core.RouteSummary("Confirm password reset with verification code and new password"),
		core.RouteDescription("Verifies the password recovery code and updates the user's password using argon2id key derivation."),
		core.RouteNoContentResponse("Password reset confirmed"),
		core.RouteOperationID("auth__password_reset__confirm"),
		core.RouteSDKGroupName("auth", "passwordReset"),
		core.RouteSDKMethodName("confirm"),
	)

	// 4. Passkeys (WebAuthn / FIDO2)
	core.PostRoute[passkey.SignUpOptions, PasskeySignUpRequest](router, "/api/v1/auth/passkeys/sign-up", service.baseHandler.handlePasskeySignUp,
		core.RouteTag("Passkeys"),
		core.RouteSummary("Begin WebAuthn passkey registration ceremony"),
		core.RouteDescription("Begins the WebAuthn ceremony for registering a hardware or biometric passkey, returning a cryptographic challenge."),
		core.RouteOperationID("auth__passkeys__sign_up__begin"),
		core.RouteSDKGroupName("auth", "passkeys", "signUp"),
		core.RouteSDKMethodName("begin"),
	)
	core.PostRoute[core.Empty, PasskeySignUpVerifyRequest](router, "/api/v1/auth/passkeys/sign-up/verify", service.baseHandler.handlePasskeySignUpVerify,
		core.RouteTag("Passkeys"),
		core.RouteSummary("Verify WebAuthn passkey registration attestation"),
		core.RouteDescription("Validates the WebAuthn attestation response and stores the public key credential in auth.identities."),
		core.RouteNoContentResponse("Passkey verified and registered"),
		core.RouteOperationID("auth__passkeys__sign_up__verify"),
		core.RouteSDKGroupName("auth", "passkeys", "signUp"),
		core.RouteSDKMethodName("verify"),
	)
	core.PostRoute[passkey.SignInOptions, core.Empty](router, "/api/v1/auth/passkeys/sign-in", service.baseHandler.handlePasskeySignIn,
		core.RouteTag("Passkeys"),
		core.RouteSummary("Begin WebAuthn passkey authentication ceremony"),
		core.RouteDescription("Begins WebAuthn passkey authentication, generating an assertion challenge for the user's registered credential."),
		core.RouteNoRequestBody(),
		core.RouteOperationID("auth__passkeys__sign_in__begin"),
		core.RouteSDKGroupName("auth", "passkeys", "signIn"),
		core.RouteSDKMethodName("begin"),
	)
	core.PostRoute[SessionResponse, PasskeySignInVerifyRequest](router, "/api/v1/auth/passkeys/sign-in/verify", service.baseHandler.handlePasskeySignInVerify,
		core.RouteTag("Passkeys"),
		core.RouteSummary("Verify WebAuthn passkey assertion signature"),
		core.RouteDescription("Verifies WebAuthn assertion signature against stored public key and issues an authenticated session."),
		core.RouteOperationID("auth__passkeys__sign_in__verify"),
		core.RouteSDKGroupName("auth", "passkeys", "signIn"),
		core.RouteSDKMethodName("verify"),
	)
	core.GetRoute[[]UserPasskeyResponse](router, "/api/v1/auth/user/passkeys", service.baseHandler.handleListUserPasskeys,
		core.RouteTag("Passkeys"),
		core.RouteSummary("List registered passkeys for current user"),
		core.RouteDescription("Retrieves all registered WebAuthn passkey credentials belonging to the authenticated user."),
		core.RouteOperationID("auth__user__passkeys__list"),
		core.RouteSDKGroupName("auth", "user", "passkeys"),
		core.RouteSDKMethodName("list"),
	)
	core.DeleteRoute[core.Empty](router, "/api/v1/auth/user/passkeys/{id}", service.baseHandler.handleDeleteUserPasskey,
		core.RouteTag("Passkeys"),
		core.RouteSummary("Revoke a registered passkey"),
		core.RouteDescription("Revokes and deletes a registered WebAuthn passkey credential belonging to the authenticated user."),
		core.RouteNoContentResponse("Passkey revoked"),
		core.RouteOperationID("auth__user__passkeys__delete"),
		core.RouteSDKGroupName("auth", "user", "passkeys"),
		core.RouteSDKMethodName("delete"),
	)

	// 5. Passwordless OTP
	core.PostRoute[core.Empty, OTPSendRequest](router, "/api/v1/auth/otp", service.baseHandler.handleOTPSend,
		core.RouteTag("Passwordless"),
		core.RouteSummary("Request a 6-digit one-time password code via email or SMS"),
		core.RouteDescription("Sends a 6-digit one-time password (OTP) verification code via configured SMS or email provider."),
		core.RouteNoContentResponse("OTP verification code dispatched"),
		core.RouteOperationID("auth__otp__send"),
		core.RouteSDKGroupName("auth", "otp"),
		core.RouteSDKMethodName("send"),
	)
	core.PostRoute[SessionResponse, OTPVerifyRequest](router, "/api/v1/auth/otp/verify", service.baseHandler.handleOTPVerify,
		core.RouteTag("Passwordless"),
		core.RouteSummary("Verify 6-digit OTP code and issue authenticated session"),
		core.RouteDescription("Validates the 6-digit OTP code against argon2id hash and issues an authenticated session."),
		core.RouteOperationID("auth__otp__verify"),
		core.RouteSDKGroupName("auth", "otp"),
		core.RouteSDKMethodName("verify"),
	)

	// 6. Multi-Factor Authentication
	core.PostRoute[MFASetupResponse, MFASetupRequest](router, "/api/v1/auth/mfa", service.baseHandler.handleMFASetup,
		core.RouteTag("Multi-Factor Authentication"),
		core.RouteSummary("Generate TOTP secret and setup URI for authenticator apps"),
		core.RouteDescription("Generates a cryptographic TOTP secret and QR-code URI for Google Authenticator or 1Password enrollment."),
		core.RouteOperationID("auth__mfa__setup"),
		core.RouteSDKGroupName("auth", "mfa"),
		core.RouteSDKMethodName("setup"),
	)
	core.PostRoute[SessionResponse, MFAVerifyRequest](router, "/api/v1/auth/mfa/verify", service.baseHandler.handleMFAVerify,
		core.RouteTag("Multi-Factor Authentication"),
		core.RouteSummary("Verify 2FA TOTP code and enable MFA on account"),
		core.RouteDescription("Verifies the 6-digit TOTP code and marks MFA as enrolled on the user account."),
		core.RouteOperationID("auth__mfa__verify"),
		core.RouteSDKGroupName("auth", "mfa"),
		core.RouteSDKMethodName("verify"),
	)
	core.PostRoute[SessionResponse, MFAChallengeRequest](router, "/api/v1/auth/mfa/challenge", service.baseHandler.handleMFAChallenge,
		core.RouteTag("Multi-Factor Authentication"),
		core.RouteSummary("Verify MFA challenge ticket with TOTP code"),
		core.RouteDescription("Verifies the short-lived MFA challenge ticket and TOTP code after password sign-in and issues an authenticated session."),
		core.RouteOperationID("auth__mfa__challenge"),
		core.RouteSDKGroupName("auth", "mfa"),
		core.RouteSDKMethodName("challenge"),
	)
	core.DeleteRoute[core.Empty](router, "/api/v1/auth/mfa", service.baseHandler.handleMFADisable,
		core.RouteTag("Multi-Factor Authentication"),
		core.RouteSummary("Disable multi-factor authentication on user account"),
		core.RouteDescription("Disables TOTP multi-factor authentication, clears the user's encrypted MFA secret, and emits auth.mfa.disabled."),
		core.RouteNoContentResponse("MFA disabled on account"),
		core.RouteOperationID("auth__mfa__disable"),
		core.RouteSDKGroupName("auth", "mfa"),
		core.RouteSDKMethodName("disable"),
	)

	// 7. OAuth & OpenID Connect
	core.GetRoute[core.Empty](router, "/api/v1/auth/oauth/authorize", service.baseHandler.handleOIDCAuthorize,
		core.RouteTag("OpenID Connect"),
		core.RouteSummary("OpenID Connect authorization endpoint and Universal Sign-In page"),
		core.RouteDescription("Renders the hosted Universal Sign-In page or performs SSO active session bypass redirect with authorization code."),
		core.RouteNoContentResponse("Redirect to client callback or render HTML sign-in"),
		core.RouteOperationID("auth__oidc__authorize"),
		core.RouteSDKGroupName("auth", "oidc"),
		core.RouteSDKMethodName("authorize"),
	)
	core.GetRoute[core.Empty](router, "/api/v1/auth/oauth/{provider}/authorize", service.baseHandler.HandleOAuthAuthorize,
		core.RouteTag("OAuth"),
		core.RouteSummary("Redirect to third-party OAuth provider authorization URL"),
		core.RouteDescription("Redirects the client browser to the third-party OAuth2 / OIDC provider authorization URL with PKCE challenge."),
		core.RouteNoContentResponse("Redirect to third-party OAuth provider"),
		core.RouteOperationID("auth__oauth__authorize"),
		core.RouteSDKGroupName("auth", "oauth"),
		core.RouteSDKMethodName("authorize"),
	)
	core.PostRoute[OIDCTokenResponse, core.Empty](router, "/api/v1/auth/oauth/token", service.baseHandler.handleOIDCToken,
		core.RouteTag("OpenID Connect"),
		core.RouteSummary("Exchange authorization code or refresh token for OpenID Connect tokens"),
		core.RouteDescription("Issues access token, refresh token, and ID token in exchange for an authorization code with PKCE verification."),
		core.RouteOperationID("auth__oidc__token"),
		core.RouteSDKGroupName("auth", "oidc"),
		core.RouteSDKMethodName("token"),
	)
	core.GetRoute[OIDCUserInfoResponse](router, "/api/v1/auth/oauth/userinfo", service.baseHandler.handleOIDCUserInfo,
		core.RouteTag("OpenID Connect"),
		core.RouteSummary("Retrieve OpenID Connect Core 1.0 user claims"),
		core.RouteDescription("Retrieves standard OpenID Connect profile claims (sub, email, email_verified, name) for the authenticated caller."),
		core.RouteOperationID("auth__oidc__userinfo"),
		core.RouteSDKGroupName("auth", "oidc"),
		core.RouteSDKMethodName("userinfo"),
	)

	// Runtime non-OpenAPI browser callback and form submission routes
	router.Mux().HandleFunc("POST /api/v1/auth/oauth/authorize", service.baseHandler.handleOIDCAuthorizeSubmit)
	router.Mux().HandleFunc("GET /api/v1/auth/oauth/sign-out", service.baseHandler.handleOIDCSignOut)
	router.Mux().HandleFunc("POST /api/v1/auth/oauth/sign-out", service.baseHandler.handleOIDCSignOut)
	router.Mux().HandleFunc("GET /api/v1/auth/oauth/{provider}/callback", service.baseHandler.HandleOAuthCallback)
	router.Mux().HandleFunc("POST /api/v1/auth/oauth/{provider}/callback", service.baseHandler.HandleOAuthCallback)

	// Runtime password reset and action aliases
	router.Mux().HandleFunc("POST /api/v1/auth/password/reset", service.baseHandler.handlePasswordResetRequest)
	router.Mux().HandleFunc("POST /api/v1/auth/password/reset/confirm", service.baseHandler.handlePasswordResetConfirm)
	router.Mux().HandleFunc("POST /api/v1/auth/otp/send", service.baseHandler.handleOTPSend)
	router.Mux().HandleFunc("POST /api/v1/auth/mfa/setup", service.baseHandler.handleMFASetup)
	router.Mux().HandleFunc("GET /api/v1/auth/passkeys", service.baseHandler.handleListUserPasskeys)
	router.Mux().HandleFunc("DELETE /api/v1/auth/passkeys/{id}", service.baseHandler.handleDeleteUserPasskey)

	// 8. GDPR Export
	core.PostRoute[ExportUserDataResponse, core.Empty](router, "/api/v1/auth/users/{user_id}/export", service.baseHandler.handleUserExport,
		core.RouteTag("GDPR Compliance"),
		core.RouteSummary("Export comprehensive user account data graph"),
		core.RouteDescription("Exports all personal data, sessions, and identities associated with the user in compliance with GDPR Article 20."),
		core.RouteNoRequestBody(),
		core.RouteOperationID("auth__users__export"),
		core.RouteSDKGroupName("auth", "users"),
		core.RouteSDKMethodName("export"),
	)

	// 9. Device & Session Management
	core.GetRoute[ListUserSessionsResponse](router, "/api/v1/auth/sessions", service.baseHandler.handleListSessions,
		core.RouteTag("Authentication"),
		core.RouteSummary("List active sessions for current user"),
		core.RouteDescription("Lists active devices and sessions for the authenticated user with IP, user-agent, creation timestamp, and current session marker."),
		core.RouteOperationID("auth__sessions__list"),
		core.RouteSDKGroupName("auth", "sessions"),
		core.RouteSDKMethodName("list"),
	)
	core.DeleteRoute[core.Empty](router, "/api/v1/auth/sessions/{session_id}", service.baseHandler.handleRevokeSession,
		core.RouteTag("Authentication"),
		core.RouteSummary("Revoke specific user session"),
		core.RouteDescription("Revokes an active session by session ID belonging to the authenticated user."),
		core.RouteNoContentResponse("Session revoked"),
		core.RouteOperationID("auth__sessions__revoke"),
		core.RouteSDKGroupName("auth", "sessions"),
		core.RouteSDKMethodName("revoke"),
	)
	core.PostRoute[RevokeOtherSessionsResponse, core.Empty](router, "/api/v1/auth/sessions/revoke-others", service.baseHandler.handleRevokeOtherSessions,
		core.RouteTag("Authentication"),
		core.RouteSummary("Sign out of all other devices"),
		core.RouteDescription("Revokes all active sessions for the current user except the currently active device session."),
		core.RouteNoRequestBody(),
		core.RouteOperationID("auth__sessions__revoke_others"),
		core.RouteSDKGroupName("auth", "sessions"),
		core.RouteSDKMethodName("revokeOthers"),
	)

	// Session management user-prefixed aliases
	router.Mux().HandleFunc("GET /api/v1/auth/user/sessions", service.baseHandler.handleListSessions)
	router.Mux().HandleFunc("DELETE /api/v1/auth/user/sessions/{session_id}", service.baseHandler.handleRevokeSession)
	router.Mux().HandleFunc("POST /api/v1/auth/user/sessions/revoke-others", service.baseHandler.handleRevokeOtherSessions)

	// 10. User Self-Service Account Management
	core.GetRoute[UserResponse](router, "/api/v1/auth/user", service.baseHandler.handleGetUser,
		core.RouteTag("User Self-Service"),
		core.RouteSummary("Get authenticated user"),
		core.RouteDescription("Retrieves the authenticated user, verification status, and MFA enrollment status."),
		core.RouteOperationID("auth__user__get"),
		core.RouteSDKGroupName("auth", "user"),
		core.RouteSDKMethodName("get"),
	)
	core.PatchRoute[UpdateUserPropertiesResponse, UpdateUserPropertiesRequest](router, "/api/v1/auth/user/properties", service.baseHandler.handleUpdateUserProperties,
		core.RouteTag("User Self-Service"),
		core.RouteSummary("Update user personal properties"),
		core.RouteDescription("Merges personal custom properties into the authenticated user."),
		core.RouteOperationID("auth__user__properties__update"),
		core.RouteSDKGroupName("auth", "user", "properties"),
		core.RouteSDKMethodName("update"),
	)
	core.PatchRoute[core.Empty, UpdateUserEmailRequest](router, "/api/v1/auth/user/email", service.baseHandler.handleUpdateUserEmail,
		core.RouteTag("User Self-Service"),
		core.RouteSummary("Request to update authenticated user email"),
		core.RouteDescription("Asserts email uniqueness, generates a single-use verification code, and dispatches it to the new email address."),
		core.RouteNoContentResponse("Email verification code sent"),
		core.RouteOperationID("auth__user__email__update"),
		core.RouteSDKGroupName("auth", "user", "email"),
		core.RouteSDKMethodName("update"),
	)
	core.PostRoute[core.Empty, UserEmailVerificationRequest](router, "/api/v1/auth/user/email/verification/request", service.baseHandler.handleUserEmailVerificationRequest,
		core.RouteTag("User Self-Service"),
		core.RouteSummary("Request an email verification code"),
		core.RouteDescription("Dispatches a single-use verification code to the recipient's email address."),
		core.RouteNoContentResponse("Email verification code sent"),
		core.RouteOperationID("auth__user__email__verification__request"),
		core.RouteSDKGroupName("auth", "user", "email", "verification"),
		core.RouteSDKMethodName("request"),
	)
	core.PostRoute[core.Empty, UserEmailVerificationConfirmRequest](router, "/api/v1/auth/user/email/verification/confirm", service.baseHandler.handleUserEmailVerificationConfirm,
		core.RouteTag("User Self-Service"),
		core.RouteSummary("Confirm email verification with code"),
		core.RouteDescription("Verifies the email code and marks the user's email as verified."),
		core.RouteNoContentResponse("Email successfully verified"),
		core.RouteOperationID("auth__user__email__verification__confirm"),
		core.RouteSDKGroupName("auth", "user", "email", "verification"),
		core.RouteSDKMethodName("confirm"),
	)
	core.PatchRoute[core.Empty, UpdateUserPhoneRequest](router, "/api/v1/auth/user/phone", service.baseHandler.handleUpdateUserPhone,
		core.RouteTag("User Self-Service"),
		core.RouteSummary("Request to update authenticated user phone number"),
		core.RouteDescription("Asserts phone uniqueness, generates a single-use verification code, and dispatches it via SMS to the new phone number."),
		core.RouteNoContentResponse("Phone verification code sent"),
		core.RouteOperationID("auth__user__phone__update"),
		core.RouteSDKGroupName("auth", "user", "phone"),
		core.RouteSDKMethodName("update"),
	)
	core.PostRoute[core.Empty, UserPhoneVerificationRequest](router, "/api/v1/auth/user/phone/verification/request", service.baseHandler.handleUserPhoneVerificationRequest,
		core.RouteTag("User Self-Service"),
		core.RouteSummary("Request a phone verification code"),
		core.RouteDescription("Dispatches a single-use verification code via SMS to the recipient's phone number."),
		core.RouteNoContentResponse("Phone verification code sent"),
		core.RouteOperationID("auth__user__phone__verification__request"),
		core.RouteSDKGroupName("auth", "user", "phone", "verification"),
		core.RouteSDKMethodName("request"),
	)
	core.PostRoute[core.Empty, UserPhoneVerificationConfirmRequest](router, "/api/v1/auth/user/phone/verification/confirm", service.baseHandler.handleUserPhoneVerificationConfirm,
		core.RouteTag("User Self-Service"),
		core.RouteSummary("Confirm phone verification with code"),
		core.RouteDescription("Verifies the phone code and marks the user's phone number as verified."),
		core.RouteNoContentResponse("Phone successfully verified"),
		core.RouteOperationID("auth__user__phone__verification__confirm"),
		core.RouteSDKGroupName("auth", "user", "phone", "verification"),
		core.RouteSDKMethodName("confirm"),
	)
	core.PatchRoute[core.Empty, UpdateUserPasswordRequest](router, "/api/v1/auth/user/password", service.baseHandler.handleUpdateUserPassword,
		core.RouteTag("User Self-Service"),
		core.RouteSummary("Change or set account password"),
		core.RouteDescription("Updates the user's password, verifying current password if one is already set. Prohibited on anonymous accounts."),
		core.RouteNoContentResponse("Password updated successfully"),
		core.RouteOperationID("auth__user__password__update"),
		core.RouteSDKGroupName("auth", "user", "password"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[core.Empty](router, "/api/v1/auth/user", service.baseHandler.handleDeleteUser,
		core.RouteTag("User Self-Service"),
		core.RouteSummary("Permanently delete authenticated user account"),
		core.RouteDescription("Deletes the user account, cascading active sessions, passkeys, and identities, and revoking authentication cookies."),
		core.RouteNoContentResponse("Account deleted successfully"),
		core.RouteOperationID("auth__user__delete"),
		core.RouteSDKGroupName("auth", "user"),
		core.RouteSDKMethodName("delete"),
	)
}

func (service *Service) registerControlPlaneRoutes(router *core.Router) {
	if service.controlPlaneHandler == nil || service.configManager == nil {
		return
	}

	// 1. Dynamic Runtime Configuration
	core.GetRoute[Config](router, "/api/v1/_/auth/config", service.configManager.HandleGetConfig,
		core.RouteTag("Auth Control Plane"),
		core.RouteSummary("Retrieve dynamic runtime auth configuration (zero-decryption projection)"),
		core.RouteDescription("Retrieves dynamic runtime authentication configuration with secrets projected as boolean indicators."),
		core.RouteOperationID("auth__config__get"),
		core.RouteSDKGroupName("auth", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/api/v1/_/auth/config", service.configManager.HandlePutConfig,
		core.RouteTag("Auth Control Plane"),
		core.RouteSummary("Update dynamic runtime auth configuration with write-only secrets"),
		core.RouteDescription("Updates dynamic runtime auth configuration, envelope-encrypting new secrets with master encryption key."),
		core.RouteOperationID("auth__config__update"),
		core.RouteSDKGroupName("auth", "config"),
		core.RouteSDKMethodName("update"),
	)

	// 2. User Management
	core.GetRoute[[]UserRecord](router, "/api/v1/_/auth/users", service.controlPlaneHandler.HandleListUsers,
		core.RouteTag("Auth User Management"),
		core.RouteSummary("List registered application users with filtering and pagination"),
		core.RouteDescription("Lists registered application users with pagination, role filters, and search capabilities."),
		core.RouteOperationID("auth__users__list"),
		core.RouteSDKGroupName("auth", "users"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[UserRecord, UserCreateRequest](router, "/api/v1/_/auth/users", service.controlPlaneHandler.HandleCreateUser,
		core.RouteTag("Auth User Management"),
		core.RouteSummary("Create a new application user account"),
		core.RouteDescription("Creates a new application user account directly through the control plane."),
		core.RouteOperationID("auth__users__create"),
		core.RouteSDKGroupName("auth", "users"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[UserRecord](router, "/api/v1/_/auth/users/{user_id}", service.controlPlaneHandler.HandleGetUser,
		core.RouteTag("Auth User Management"),
		core.RouteSummary("Get detailed user record by UUID"),
		core.RouteDescription("Retrieves full user account details including identities, verification status, and lockout metadata."),
		core.RouteOperationID("auth__users__get"),
		core.RouteSDKGroupName("auth", "users"),
		core.RouteSDKMethodName("get"),
	)
	core.DeleteRoute[core.Empty](router, "/api/v1/_/auth/users/{user_id}", service.controlPlaneHandler.HandleDeleteUser,
		core.RouteTag("Auth User Management"),
		core.RouteSummary("Delete application user and cascade related sessions/identities"),
		core.RouteDescription("Permanently deletes user account and cascades deletion to sessions and identities."),
		core.RouteNoContentResponse("User deleted"),
		core.RouteOperationID("auth__users__delete"),
		core.RouteSDKGroupName("auth", "users"),
		core.RouteSDKMethodName("delete"),
	)

	// 3. User Lock Management
	core.PostRoute[core.Empty, core.Empty](router, "/api/v1/_/auth/users/{user_id}/lock", service.controlPlaneHandler.HandleLockUser,
		core.RouteTag("Auth User Management"),
		core.RouteSummary("Lock application user and revoke active sessions"),
		core.RouteDescription("Locks user account to prevent authentication and revokes all active sessions immediately."),
		core.RouteNoRequestBody(),
		core.RouteNoContentResponse("User locked"),
		core.RouteOperationID("auth__users__lock"),
		core.RouteSDKGroupName("auth", "users"),
		core.RouteSDKMethodName("lock"),
	)
	core.DeleteRoute[core.Empty](router, "/api/v1/_/auth/users/{user_id}/lock", service.controlPlaneHandler.HandleUnlockUser,
		core.RouteTag("Auth User Management"),
		core.RouteSummary("Unlock application user and restore access"),
		core.RouteDescription("Unlocks user account and restores login capabilities."),
		core.RouteNoContentResponse("User unlocked"),
		core.RouteOperationID("auth__users__unlock"),
		core.RouteSDKGroupName("auth", "users"),
		core.RouteSDKMethodName("unlock"),
	)

	// 4. Session Management
	core.GetRoute[[]SessionRecord](router, "/api/v1/_/auth/users/{user_id}/sessions", service.controlPlaneHandler.HandleListUserSessions,
		core.RouteTag("Auth Session Management"),
		core.RouteSummary("List active sessions for an application user"),
		core.RouteDescription("Lists all active sessions for a specific user, including IP address, user agent, and expiration time."),
		core.RouteOperationID("auth__users__sessions__list"),
		core.RouteSDKGroupName("auth", "users", "sessions"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[core.Empty, core.Empty](router, "/api/v1/_/auth/users/{user_id}/sessions/revoke", service.controlPlaneHandler.HandleRevokeUserSessions,
		core.RouteTag("Auth Session Management"),
		core.RouteSummary("Revoke all active sessions for an application user"),
		core.RouteDescription("Revokes all active sessions and refresh tokens for the specified user."),
		core.RouteNoRequestBody(),
		core.RouteNoContentResponse("Sessions revoked"),
		core.RouteOperationID("auth__users__sessions__revoke"),
		core.RouteSDKGroupName("auth", "users", "sessions"),
		core.RouteSDKMethodName("revoke"),
	)
}
