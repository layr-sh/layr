package auth

import (
	"layr.sh/auth/passkey"
	"layr.sh/core"
)

// RegisterRoutes registers all Auth service REST and OpenAPI 3.1 endpoints on public and control plane routers.
func (service *Service) RegisterRoutes(baseRouter *core.Router, controlPlaneRouter *core.Router) {
	service.registerBaseRoutes(baseRouter)
	service.registerControlPlaneRoutes(controlPlaneRouter)
}

func (service *Service) registerBaseRoutes(router *core.Router) {
	// 1. OIDC Discovery & JWKS
	core.GetRoute[OIDCConfiguration](router, "/.well-known/openid-configuration", service.baseHandler.handleGetOIDCDiscovery,
		core.RouteHide(),
		core.RouteTag("OpenID Connect"),
		core.RouteSummary("OpenID Connect discovery document"),
		core.RouteDescription("Public OpenID Connect discovery metadata document defining issuer, authorization, token, and JWKS endpoints."),
		core.RouteOperationID("auth__openid_configuration"),
		core.RouteSDKGroupName("auth"),
		core.RouteSDKMethodName("openid_configuration"),
	)
	core.GetRoute[core.JWKS](router, "/.well-known/jwks.json", service.baseHandler.handleGetJWKS,
		core.RouteHide(),
		core.RouteTag("OpenID Connect"),
		core.RouteSummary("JSON Web Key Set (JWKS) public verification keys"),
		core.RouteDescription("Public JSON Web Key Set (JWKS) containing active Ed25519 public verification keys."),
		core.RouteOperationID("auth__jwks"),
		core.RouteSDKGroupName("auth"),
		core.RouteSDKMethodName("jwks"),
	)

	// 2. Anonymous Auth
	core.PostRoute[AuthTokenResponse, SignInAnonymousInput](router, "/v1/auth/anonymous", service.baseHandler.handleSignInAnonymous,
		core.RouteTag("Anonymous"),
		core.RouteSummary("Initialize or sign in an anonymous guest user account"),
		core.RouteDescription("Creates an ephemeral guest user account in auth.users with role 'authenticated', is_anonymous set to true, null email/phone, and optional initial profile properties. Publishes auth.user.signed_up event and issues an Ed25519 JWT access token, rotated refresh token in auth.sessions, and authentication cookies."),
		core.RouteOperationID("auth__anonymous__sign_in"),
		core.RouteSDKGroupName("auth", "anonymous"),
		core.RouteSDKMethodName("sign_in"),
	)

	// 3. Password Auth
	core.PostRoute[AuthTokenResponse, SignUpInput](router, "/v1/auth/sign-up", service.baseHandler.handleSignUp,
		core.RouteTag("Sign Up"),
		core.RouteSummary("Register a new application user with password"),
		core.RouteDescription("Registers a new application user account with email or normalized E.164 phone, password complexity validation, breach detection, and Argon2id key derivation. Supports seamless upgrade of anonymous guest accounts. Dispatches verification code if confirmation is required; otherwise issues Ed25519 JWT tokens."),
		core.RouteOperationID("auth__sign_up"),
		core.RouteSDKGroupName("auth"),
		core.RouteSDKMethodName("sign_up"),
	)
	core.PostRoute[AuthTokenResponse, SignInInput](router, "/v1/auth/sign-in", service.baseHandler.handleSignIn,
		core.RouteTag("Sign In"),
		core.RouteSummary("Authenticate with email or phone and password"),
		core.RouteDescription("Authenticates application user with email or normalized E.164 phone and password. Verifies Argon2id hash, checks account lockout status and breach flags. If MFA is enrolled, returns an ephemeral mfa_ticket challenge; otherwise issues Ed25519 JWT access token, refresh token in auth.sessions, and session cookies."),
		core.RouteOperationID("auth__sign_in"),
		core.RouteSDKGroupName("auth"),
		core.RouteSDKMethodName("sign_in"),
	)
	core.PostRoute[core.Empty, core.Empty](router, "/v1/auth/sign-out", service.baseHandler.handleSignOut,
		core.RouteTag("Sign Out"),
		core.RouteSummary("Revoke active session and refresh token"),
		core.RouteDescription("Revokes active refresh token, deletes corresponding session record from auth.sessions, clears session cookies, and emits auth.user.signed_out event."),
		core.RouteNoRequestBody(),
		core.RouteNoContentResponse("Signed out successfully"),
		core.RouteOperationID("auth__sign_out"),
		core.RouteSDKGroupName("auth"),
		core.RouteSDKMethodName("sign_out"),
	)
	core.PostRoute[AuthTokenResponse, RefreshTokenInput](router, "/v1/auth/token/refresh", service.baseHandler.handleRefreshToken,
		core.RouteTag("Token"),
		core.RouteSummary("Exchange refresh token for fresh JWT access token"),
		core.RouteDescription("Validates submitted refresh token against SHA-256 hash in auth.sessions, checks session expiry and user lock status, rotates refresh token, and issues a freshly signed Ed25519 JWT access token with updated claims resolved from public.auth_claims."),
		core.RouteOperationID("auth__token__refresh"),
		core.RouteSDKGroupName("auth", "token"),
		core.RouteSDKMethodName("refresh"),
	)
	core.PostRoute[core.Empty, RequestPasswordResetInput](router, "/v1/auth/password-reset/request", service.baseHandler.handleRequestPasswordReset,
		core.RouteTag("Password Reset"),
		core.RouteSummary("Request a password reset verification code via email or SMS"),
		core.RouteDescription("Initiates password recovery for registered user, generating an Argon2id-hashed single-use verification code with cooldown guards and dispatching it to registered email or SMS destination."),
		core.RouteNoContentResponse("Password reset code sent"),
		core.RouteOperationID("auth__password_reset__request"),
		core.RouteSDKGroupName("auth", "password_reset"),
		core.RouteSDKMethodName("request"),
	)
	core.PostRoute[core.Empty, ConfirmPasswordResetInput](router, "/v1/auth/password-reset/confirm", service.baseHandler.handleConfirmPasswordReset,
		core.RouteTag("Password Reset"),
		core.RouteSummary("Confirm password reset with verification code and new password"),
		core.RouteDescription("Verifies single-use password recovery verification code, enforces password complexity and breach checks, updates password hash using Argon2id key derivation in auth.users, and revokes existing sessions."),
		core.RouteNoContentResponse("Password reset confirmed"),
		core.RouteOperationID("auth__password_reset__confirm"),
		core.RouteSDKGroupName("auth", "password_reset"),
		core.RouteSDKMethodName("confirm"),
	)

	// 4. Passkeys (WebAuthn / FIDO2)
	core.PostRoute[passkey.SignUpOptions, BeginPasskeySignUpInput](router, "/v1/auth/passkeys/sign-up", service.baseHandler.handleBeginPasskeySignUp,
		core.RouteTag("Passkeys Sign Up"),
		core.RouteSummary("Begin WebAuthn passkey registration ceremony"),
		core.RouteDescription("Initiates WebAuthn / FIDO2 credential creation ceremony, generating an attestation challenge, relying party metadata, and user entity options for hardware or platform passkey enrollment."),
		core.RouteOperationID("auth__passkeys__sign_up__begin"),
		core.RouteSDKGroupName("auth", "passkeys", "sign_up"),
		core.RouteSDKMethodName("begin"),
	)
	core.PostRoute[AuthTokenResponse, VerifyPasskeySignUpInput](router, "/v1/auth/passkeys/sign-up/verify", service.baseHandler.handleVerifyPasskeySignUp,
		core.RouteTag("Passkeys Sign Up"),
		core.RouteSummary("Verify WebAuthn passkey registration attestation and store credential"),
		core.RouteDescription("Validates client attestation response and public key, stores credential record in auth.passkeys, creates user identity, and issues an authenticated session with Ed25519 JWT tokens."),
		core.RouteOperationID("auth__passkeys__sign_up__verify"),
		core.RouteSDKGroupName("auth", "passkeys", "sign_up"),
		core.RouteSDKMethodName("verify"),
	)
	core.PostRoute[passkey.SignInOptions, core.Empty](router, "/v1/auth/passkeys/sign-in", service.baseHandler.handleBeginPasskeySignIn,
		core.RouteTag("Passkeys Sign In"),
		core.RouteSummary("Begin WebAuthn passkey authentication ceremony"),
		core.RouteDescription("Initiates WebAuthn / FIDO2 assertion ceremony, generating a cryptographic challenge and credential request options for the client browser or authenticator hardware."),
		core.RouteNoRequestBody(),
		core.RouteOperationID("auth__passkeys__sign_in__begin"),
		core.RouteSDKGroupName("auth", "passkeys", "sign_in"),
		core.RouteSDKMethodName("begin"),
	)
	core.PostRoute[AuthTokenResponse, VerifyPasskeySignInInput](router, "/v1/auth/passkeys/sign-in/verify", service.baseHandler.handleVerifyPasskeySignIn,
		core.RouteTag("Passkeys Sign In"),
		core.RouteSummary("Verify WebAuthn passkey assertion signature and issue session"),
		core.RouteDescription("Verifies the client's WebAuthn assertion signature against the registered public key and signature counter stored in auth.passkeys. Issues an Ed25519 JWT access token, refresh token, and session record in auth.sessions."),
		core.RouteOperationID("auth__passkeys__sign_in__verify"),
		core.RouteSDKGroupName("auth", "passkeys", "sign_in"),
		core.RouteSDKMethodName("verify"),
	)
	core.GetRoute[ListPasskeysResponse](router, "/v1/auth/user/passkeys", service.baseHandler.handleListPasskeys,
		core.RouteTag("User Passkeys"),
		core.RouteSummary("List registered passkeys for current user"),
		core.RouteDescription("Retrieves all registered WebAuthn / FIDO2 passkey credentials for the authenticated user from auth.passkeys, including credential IDs, friendly names, creation timestamps, and last used dates. Requires authenticated user session."),
		core.RouteOperationID("auth__user__passkeys__list"),
		core.RouteSDKGroupName("auth", "user", "passkeys"),
		core.RouteSDKMethodName("list"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/auth/user/passkeys/{passkey_id}", service.baseHandler.handleDeletePasskey,
		core.RouteTag("User Passkeys"),
		core.RouteSummary("Revoke a registered passkey credential"),
		core.RouteDescription("Deletes a registered WebAuthn passkey credential from auth.passkeys belonging to the authenticated user. Prohibits deleting credentials belonging to other users. Requires authenticated user session."),
		core.RouteNoContentResponse("Passkey revoked"),
		core.RouteOperationID("auth__user__passkeys__delete"),
		core.RouteSDKGroupName("auth", "user", "passkeys"),
		core.RouteSDKMethodName("delete"),
	)

	// 5. Passwordless OTP
	core.PostRoute[core.Empty, SendOTPInput](router, "/v1/auth/otp", service.baseHandler.handleSendOTP,
		core.RouteTag("OTP"),
		core.RouteSummary("Request a one-time password verification code via email or SMS"),
		core.RouteDescription("Generates a single-use 6-digit numeric OTP with configurable TTL and rate limiting, hashes it with Argon2id, stores it in auth.otps, and dispatches it via configured email or SMS provider (Twilio, Vonage, SendGrid, Resend, or SMTP)."),
		core.RouteNoContentResponse("OTP verification code dispatched"),
		core.RouteOperationID("auth__otp__send"),
		core.RouteSDKGroupName("auth", "otp"),
		core.RouteSDKMethodName("send"),
	)
	core.PostRoute[AuthTokenResponse, VerifyOTPInput](router, "/v1/auth/otp/verify", service.baseHandler.handleVerifyOTP,
		core.RouteTag("OTP"),
		core.RouteSummary("Verify one-time password code and issue authenticated session"),
		core.RouteDescription("Validates the 6-digit OTP code against stored Argon2id hash in auth.otps. On successful verification, marks contact method verified, issues an Ed25519 JWT access token and rotated refresh token, and creates an active session in auth.sessions."),
		core.RouteOperationID("auth__otp__verify"),
		core.RouteSDKGroupName("auth", "otp"),
		core.RouteSDKMethodName("verify"),
	)

	// 6. Multi-Factor Authentication
	core.PostRoute[SetupMFAResponse, core.Empty](router, "/v1/auth/mfa", service.baseHandler.handleSetupMFA,
		core.RouteTag("MFA"),
		core.RouteSummary("Generate TOTP secret and setup URI for authenticator enrollment"),
		core.RouteDescription("Generates a cryptographic RFC 6238 TOTP secret, stores it temporarily encrypted in the session cache, and returns an otpauth:// URI and QR code payload compatible with Google Authenticator, 1Password, or Authy. Requires authenticated user session."),
		core.RouteNoRequestBody(),
		core.RouteOperationID("auth__mfa__setup"),
		core.RouteSDKGroupName("auth", "mfa"),
		core.RouteSDKMethodName("setup"),
	)
	core.PostRoute[AuthTokenResponse, VerifyMFAInput](router, "/v1/auth/mfa/verify", service.baseHandler.handleVerifyMFA,
		core.RouteTag("MFA"),
		core.RouteSummary("Verify initial TOTP code to complete MFA enrollment"),
		core.RouteDescription("Validates the initial 6-digit TOTP code against the pending setup secret, sets mfa_enabled to true on auth.users, stores the AES-encrypted secret permanently, and publishes auth.mfa.enabled event. Requires authenticated user session."),
		core.RouteOperationID("auth__mfa__verify"),
		core.RouteSDKGroupName("auth", "mfa"),
		core.RouteSDKMethodName("verify"),
	)
	core.PostRoute[AuthTokenResponse, ChallengeMFAInput](router, "/v1/auth/mfa/challenge", service.baseHandler.handleChallengeMFA,
		core.RouteTag("MFA"),
		core.RouteSummary("Verify MFA challenge ticket with TOTP passcode"),
		core.RouteDescription("Verifies a 6-digit TOTP passcode against the user's enrolled MFA secret using a short-lived challenge ticket (5-minute TTL) issued during password sign-in. On verification, issues an Ed25519 JWT access token, rotated refresh token, and creates a session record in auth.sessions."),
		core.RouteOperationID("auth__mfa__challenge"),
		core.RouteSDKGroupName("auth", "mfa"),
		core.RouteSDKMethodName("challenge"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/auth/mfa", service.baseHandler.handleDisableMFA,
		core.RouteTag("MFA"),
		core.RouteSummary("Disable multi-factor authentication on user account"),
		core.RouteDescription("Disables TOTP multi-factor authentication on auth.users for the authenticated user, clears the encrypted MFA secret, and publishes auth.mfa.disabled event. Requires authenticated user session."),
		core.RouteNoContentResponse("MFA disabled on account"),
		core.RouteOperationID("auth__mfa__disable"),
		core.RouteSDKGroupName("auth", "mfa"),
		core.RouteSDKMethodName("disable"),
	)

	// 7. OAuth & OpenID Connect
	core.GetRoute[core.Empty](router, "/v1/auth/oauth/authorize", service.baseHandler.handleAuthorizeOIDC,
		core.RouteHide(),
		core.RouteTag("OIDC"),
		core.RouteSummary("OpenID Connect authorization endpoint"),
		core.RouteDescription("Renders the hosted Universal Sign-In page or performs SSO active session bypass redirect with authorization code."),
		core.RouteNoContentResponse("Redirect to client callback or render HTML sign-in"),
		core.RouteOperationID("auth__oidc__authorize"),
		core.RouteSDKGroupName("auth", "oidc"),
		core.RouteSDKMethodName("authorize"),
	)
	core.GetRoute[core.Empty](router, "/v1/auth/oauth/{provider}/authorize", service.baseHandler.handleAuthorizeOAuth,
		core.RouteHide(),
		core.RouteTag("OAuth"),
		core.RouteSummary("Redirect to third-party OAuth provider authorization URL"),
		core.RouteDescription("Redirects the client browser to the third-party OAuth2 / OIDC provider authorization URL with PKCE challenge."),
		core.RouteNoContentResponse("Redirect to third-party OAuth provider"),
		core.RouteOperationID("auth__oauth__authorize"),
		core.RouteSDKGroupName("auth", "oauth"),
		core.RouteSDKMethodName("authorize"),
	)
	core.PostRoute[IssueOIDCTokenResponse, core.Empty](router, "/v1/auth/oauth/token", service.baseHandler.handleIssueOIDCToken,
		core.RouteTag("OIDC"),
		core.RouteSummary("Exchange authorization code or refresh token for OpenID Connect tokens"),
		core.RouteDescription("Exchanges an OAuth2 / OIDC authorization code (with PKCE code_verifier challenge verification) or refresh token for an Ed25519 JWT ID token, access token, and refresh token according to OpenID Connect Core 1.0 specifications."),
		core.RouteOperationID("auth__oidc__token"),
		core.RouteSDKGroupName("auth", "oidc"),
		core.RouteSDKMethodName("token"),
	)
	core.GetRoute[GetOIDCUserInfoResponse](router, "/v1/auth/oauth/userinfo", service.baseHandler.handleGetOIDCUserInfo,
		core.RouteTag("OIDC"),
		core.RouteSummary("Retrieve OpenID Connect Core 1.0 user claims"),
		core.RouteDescription("Retrieves standard OpenID Connect profile claims (sub, email, email_verified, phone, phone_verified, name) for the authenticated caller bearing a valid Bearer access token."),
		core.RouteOperationID("auth__oidc__userinfo"),
		core.RouteSDKGroupName("auth", "oidc"),
		core.RouteSDKMethodName("userinfo"),
	)

	// Runtime non-OpenAPI browser callback and form submission routes
	router.Mux().HandleFunc("POST /v1/auth/oauth/authorize", service.baseHandler.handleSubmitOIDCAuthorize)
	router.Mux().HandleFunc("GET /v1/auth/oauth/sign-out", service.baseHandler.handleSignOutOIDC)
	router.Mux().HandleFunc("POST /v1/auth/oauth/sign-out", service.baseHandler.handleSignOutOIDC)
	router.Mux().HandleFunc("GET /v1/auth/oauth/{provider}/callback", service.baseHandler.handleProcessOAuthCallback)
	router.Mux().HandleFunc("POST /v1/auth/oauth/{provider}/callback", service.baseHandler.handleProcessOAuthCallback)

	// 8. Device & Session Management
	core.GetRoute[ListSessionsResponse](router, "/v1/auth/user/sessions", service.baseHandler.handleListSessions,
		core.RouteTag("User Sessions"),
		core.RouteSummary("List active sessions for current user"),
		core.RouteDescription("Returns active sessions from auth.sessions for the authenticated user, detailing IP address, user agent, creation date, expiry timestamp, and a flag indicating the current device session. Requires authenticated user session."),
		core.RouteOperationID("auth__user__sessions__list"),
		core.RouteSDKGroupName("auth", "user", "sessions"),
		core.RouteSDKMethodName("list"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/auth/user/sessions/{session_id}", service.baseHandler.handleRevokeSession,
		core.RouteTag("User Sessions"),
		core.RouteSummary("Revoke a specific user session by ID"),
		core.RouteDescription("Deletes a specific session record from auth.sessions belonging to the authenticated user, terminating access for that device. Requires authenticated user session."),
		core.RouteNoContentResponse("Session revoked"),
		core.RouteOperationID("auth__user__sessions__revoke"),
		core.RouteSDKGroupName("auth", "user", "sessions"),
		core.RouteSDKMethodName("revoke"),
	)
	core.PostRoute[RevokeOtherSessionsResponse, core.Empty](router, "/v1/auth/user/sessions/revoke-others", service.baseHandler.handleRevokeOtherSessions,
		core.RouteTag("User Sessions"),
		core.RouteSummary("Sign out of all other devices and sessions"),
		core.RouteDescription("Revokes all active sessions in auth.sessions belonging to the authenticated user except the current device session, invalidating other refresh tokens immediately. Requires authenticated user session."),
		core.RouteNoRequestBody(),
		core.RouteOperationID("auth__user__sessions__revoke_others"),
		core.RouteSDKGroupName("auth", "user", "sessions"),
		core.RouteSDKMethodName("revoke_others"),
	)

	// Backward-compatibility aliases for legacy /v1/auth/sessions paths
	router.Mux().HandleFunc("GET /v1/auth/sessions", service.baseHandler.handleListSessions)
	router.Mux().HandleFunc("DELETE /v1/auth/sessions/{session_id}", service.baseHandler.handleRevokeSession)
	router.Mux().HandleFunc("POST /v1/auth/sessions/revoke-others", service.baseHandler.handleRevokeOtherSessions)

	// 9. User Self-Service Account Management
	core.GetRoute[GetUserResponse](router, "/v1/auth/user", service.baseHandler.handleGetUser,
		core.RouteTag("User"),
		core.RouteSummary("Get profile and verification details of authenticated user"),
		core.RouteDescription("Retrieves authenticated user profile from auth.users, including email, phone, verification timestamps, role, custom properties JSON, MFA enrollment status, and lockout flags. Requires authenticated user session."),
		core.RouteOperationID("auth__user__get"),
		core.RouteSDKGroupName("auth", "user"),
		core.RouteSDKMethodName("get"),
	)
	core.PostRoute[ExportUserResponse, core.Empty](router, "/v1/auth/user/export", service.baseHandler.handleExportUser,
		core.RouteTag("User"),
		core.RouteSummary("Export comprehensive user account data graph (GDPR Article 20)"),
		core.RouteDescription("Exports full personal data graph associated with the authenticated user, including profile metadata, linked authentication identities, active sessions, and passkey registrations in portable JSON format. Requires authenticated user session."),
		core.RouteNoRequestBody(),
		core.RouteOperationID("auth__user__export"),
		core.RouteSDKGroupName("auth", "user"),
		core.RouteSDKMethodName("export"),
	)
	core.PatchRoute[UpdateUserPropertiesResponse, UpdateUserPropertiesInput](router, "/v1/auth/user/properties", service.baseHandler.handleUpdateUserProperties,
		core.RouteTag("User Properties"),
		core.RouteSummary("Update custom personal properties on user profile"),
		core.RouteDescription("Performs shallow merge of custom JSON properties into the properties column of auth.users for the authenticated user, validating JSON size limits. Requires authenticated user session."),
		core.RouteOperationID("auth__user__properties__update"),
		core.RouteSDKGroupName("auth", "user", "properties"),
		core.RouteSDKMethodName("update"),
	)
	core.PatchRoute[core.Empty, UpdateUserEmailInput](router, "/v1/auth/user/email", service.baseHandler.handleUpdateUserEmail,
		core.RouteTag("User Email"),
		core.RouteSummary("Request update of authenticated user email address"),
		core.RouteDescription("Asserts uniqueness of the new email address, generates an Argon2id-hashed single-use verification code, and dispatches it to the requested email. The address is updated once verified. Requires authenticated user session."),
		core.RouteNoContentResponse("Email verification code sent"),
		core.RouteOperationID("auth__user__email__update"),
		core.RouteSDKGroupName("auth", "user", "email"),
		core.RouteSDKMethodName("update"),
	)
	core.PostRoute[core.Empty, RequestEmailVerificationInput](router, "/v1/auth/user/email/verification/request", service.baseHandler.handleRequestEmailVerification,
		core.RouteTag("User Email Verification"),
		core.RouteSummary("Request an email verification code for current email"),
		core.RouteDescription("Generates and dispatches a single-use verification code to the authenticated user's registered unverified email address. Rate-limited by IP and user cooldowns. Requires authenticated user session."),
		core.RouteNoContentResponse("Email verification code sent"),
		core.RouteOperationID("auth__user__email__verification__request"),
		core.RouteSDKGroupName("auth", "user", "email", "verification"),
		core.RouteSDKMethodName("request"),
	)
	core.PostRoute[core.Empty, ConfirmEmailVerificationInput](router, "/v1/auth/user/email/verification/confirm", service.baseHandler.handleConfirmEmailVerification,
		core.RouteTag("User Email Verification"),
		core.RouteSummary("Confirm email verification code"),
		core.RouteDescription("Verifies submitted single-use code against pending email verification record, sets email_verified_at timestamp in auth.users, and updates primary email if part of an address change flow. Requires authenticated user session."),
		core.RouteNoContentResponse("Email successfully verified"),
		core.RouteOperationID("auth__user__email__verification__confirm"),
		core.RouteSDKGroupName("auth", "user", "email", "verification"),
		core.RouteSDKMethodName("confirm"),
	)
	core.PatchRoute[core.Empty, UpdateUserPhoneInput](router, "/v1/auth/user/phone", service.baseHandler.handleUpdateUserPhone,
		core.RouteTag("User Phone"),
		core.RouteSummary("Request update of authenticated user phone number"),
		core.RouteDescription("Normalizes new phone number to E.164, verifies uniqueness in auth.users, generates single-use verification code, and dispatches via SMS. Requires authenticated user session."),
		core.RouteNoContentResponse("Phone verification code sent"),
		core.RouteOperationID("auth__user__phone__update"),
		core.RouteSDKGroupName("auth", "user", "phone"),
		core.RouteSDKMethodName("update"),
	)
	core.PostRoute[core.Empty, RequestPhoneVerificationInput](router, "/v1/auth/user/phone/verification/request", service.baseHandler.handleRequestPhoneVerification,
		core.RouteTag("User Phone Verification"),
		core.RouteSummary("Request a phone verification code for current phone"),
		core.RouteDescription("Generates and dispatches a single-use verification code via SMS to the authenticated user's registered phone number. Requires authenticated user session."),
		core.RouteNoContentResponse("Phone verification code sent"),
		core.RouteOperationID("auth__user__phone__verification__request"),
		core.RouteSDKGroupName("auth", "user", "phone", "verification"),
		core.RouteSDKMethodName("request"),
	)
	core.PostRoute[core.Empty, ConfirmPhoneVerificationInput](router, "/v1/auth/user/phone/verification/confirm", service.baseHandler.handleConfirmPhoneVerification,
		core.RouteTag("User Phone Verification"),
		core.RouteSummary("Confirm phone verification code"),
		core.RouteDescription("Verifies submitted single-use SMS code against pending verification record, updates phone_verified_at in auth.users, and marks phone as verified. Requires authenticated user session."),
		core.RouteNoContentResponse("Phone successfully verified"),
		core.RouteOperationID("auth__user__phone__verification__confirm"),
		core.RouteSDKGroupName("auth", "user", "phone", "verification"),
		core.RouteSDKMethodName("confirm"),
	)
	core.PatchRoute[core.Empty, UpdateUserPasswordInput](router, "/v1/auth/user/password", service.baseHandler.handleUpdateUserPassword,
		core.RouteTag("User Password"),
		core.RouteSummary("Change or set password for current user"),
		core.RouteDescription("Updates the account password in auth.users. Verifies current password if one is already set, enforces password complexity and breach checks, and rehashes with Argon2id. Prohibited on anonymous accounts. Requires authenticated user session."),
		core.RouteNoContentResponse("Password updated successfully"),
		core.RouteOperationID("auth__user__password__update"),
		core.RouteSDKGroupName("auth", "user", "password"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/auth/user", service.baseHandler.handleDeleteUser,
		core.RouteTag("User"),
		core.RouteSummary("Permanently delete authenticated user account"),
		core.RouteDescription("Permanently deletes the authenticated user from auth.users, cascading deletion across auth.sessions, auth.identities, auth.passkeys, and user properties. Clears authentication cookies and emits auth.user.deleted event. Requires authenticated user session."),
		core.RouteNoContentResponse("Account deleted successfully"),
		core.RouteOperationID("auth__user__delete"),
		core.RouteSDKGroupName("auth", "user"),
		core.RouteSDKMethodName("delete"),
	)
}

func (service *Service) registerControlPlaneRoutes(router *core.Router) {
	// 1. Dynamic Runtime Configuration
	core.GetRoute[Config](router, "/v1/_/auth/config", service.controlPlaneHandler.handleGetConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Retrieve dynamic runtime authentication configuration"),
		core.RouteDescription("Returns active dynamic authentication settings from auth.config including password strength rules, session TTLs, rate limits, MFA policies, and enabled OAuth providers, with client secrets projected as boolean indicators. Requires auth:config.read scope."),
		core.RouteOperationID("auth__config__get"),
		core.RouteSDKGroupName("auth", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/v1/_/auth/config", service.controlPlaneHandler.handleUpdateConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Update dynamic runtime authentication configuration"),
		core.RouteDescription("Updates dynamic authentication configuration parameters in auth.config without requiring server restarts. Automatically envelope-encrypts newly supplied third-party OAuth provider secrets with AES-256-GCM using the master encryption key. Requires auth:config.write scope."),
		core.RouteOperationID("auth__config__update"),
		core.RouteSDKGroupName("auth", "config"),
		core.RouteSDKMethodName("update"),
	)

	// 2. User Management
	core.GetRoute[ListUsersResponse](router, "/v1/_/auth/users", service.controlPlaneHandler.handleListUsers,
		core.RouteTag("Users"),
		core.RouteSummary("List registered application users with filtering and pagination"),
		core.RouteDescription("Queries auth.users with pagination, text search over email/phone, role filters, and verification status. Returns user metadata and creation timestamps. Requires auth:user.read scope."),
		core.RouteOperationID("auth__users__list"),
		core.RouteSDKGroupName("auth", "users"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[User, CreateUserInput](router, "/v1/_/auth/users", service.controlPlaneHandler.handleCreateUser,
		core.RouteTag("Users"),
		core.RouteSummary("Create an application user account via control plane"),
		core.RouteDescription("Directly provisions a new user record in auth.users with specified email, normalized phone, Argon2id password hash, role, custom properties, and email/phone verification flags. Emits auth.user.created event. Requires auth:user.write scope."),
		core.RouteOperationID("auth__users__create"),
		core.RouteSDKGroupName("auth", "users"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[User](router, "/v1/_/auth/users/{user_id}", service.controlPlaneHandler.handleGetUser,
		core.RouteTag("Users"),
		core.RouteSummary("Get detailed application user record by UUID"),
		core.RouteDescription("Retrieves complete user record from auth.users by UUID, including verification timestamps, role, custom properties JSON, MFA status, lockout metadata, and linked authentication identities. Requires auth:user.read scope."),
		core.RouteOperationID("auth__users__get"),
		core.RouteSDKGroupName("auth", "users"),
		core.RouteSDKMethodName("get"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/auth/users/{user_id}", service.controlPlaneHandler.handleDeleteUser,
		core.RouteTag("Users"),
		core.RouteSummary("Permanently delete an application user and cascade sessions"),
		core.RouteDescription("Permanently deletes application user from auth.users, cascading deletion to all active sessions, linked identities, and passkeys. Emits auth.user.deleted event. Requires auth:user.write scope."),
		core.RouteNoContentResponse("User deleted"),
		core.RouteOperationID("auth__users__delete"),
		core.RouteSDKGroupName("auth", "users"),
		core.RouteSDKMethodName("delete"),
	)

	// 3. User Lock Management
	core.PostRoute[User, LockUserInput](router, "/v1/_/auth/users/{user_id}/lock", service.controlPlaneHandler.handleLockUser,
		core.RouteTag("Users"),
		core.RouteSummary("Lock application user account and revoke active sessions"),
		core.RouteDescription("Sets locked_until timestamp on auth.users to prevent further authentication, and immediately deletes all active sessions in auth.sessions. Emits auth.user.locked event. Requires auth:user.write scope."),
		core.RouteOperationID("auth__users__lock"),
		core.RouteSDKGroupName("auth", "users"),
		core.RouteSDKMethodName("lock"),
	)
	core.DeleteRoute[User](router, "/v1/_/auth/users/{user_id}/lock", service.controlPlaneHandler.handleUnlockUser,
		core.RouteTag("Users"),
		core.RouteSummary("Unlock application user account and restore login access"),
		core.RouteDescription("Clears locked_until timestamp in auth.users, restoring authentication capabilities for a user previously locked by control plane action or brute-force protection. Requires auth:user.write scope."),
		core.RouteOperationID("auth__users__unlock"),
		core.RouteSDKGroupName("auth", "users"),
		core.RouteSDKMethodName("unlock"),
	)

	// 4. Session Management
	core.GetRoute[ListUserSessionsResponse](router, "/v1/_/auth/users/{user_id}/sessions", service.controlPlaneHandler.handleListUserSessions,
		core.RouteTag("Users Sessions"),
		core.RouteSummary("List active sessions for an application user"),
		core.RouteDescription("Returns all active sessions from auth.sessions for the specified user ID, detailing client IP, user agent, expiration time, and creation timestamp. Requires auth:user.read scope."),
		core.RouteOperationID("auth__users__sessions__list"),
		core.RouteSDKGroupName("auth", "users", "sessions"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[core.Empty, core.Empty](router, "/v1/_/auth/users/{user_id}/sessions/revoke", service.controlPlaneHandler.handleRevokeUserSessions,
		core.RouteTag("Users Sessions"),
		core.RouteSummary("Revoke all active sessions for an application user"),
		core.RouteDescription("Deletes all active sessions from auth.sessions for the specified user ID, invalidating all outstanding refresh tokens and forcing re-authentication. Requires auth:user.write scope."),
		core.RouteNoRequestBody(),
		core.RouteNoContentResponse("Sessions revoked"),
		core.RouteOperationID("auth__users__sessions__revoke"),
		core.RouteSDKGroupName("auth", "users", "sessions"),
		core.RouteSDKMethodName("revoke"),
	)
}
