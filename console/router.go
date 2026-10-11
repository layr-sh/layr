// Package console provides the administrative control plane, operator identity, and dynamic configuration engine.
package console

import (
	"net/http"

	"layr.sh/core"
)

// RegisterRoutes registers all Console control plane routes.
func (service *Service) RegisterRoutes(baseRouter *core.Router, controlPlaneRouter *core.Router) {
	service.registerControlPlaneRoutes(controlPlaneRouter)
}

func (service *Service) registerControlPlaneRoutes(router *core.Router) {
	// 1. Dynamic Runtime Configuration (Guarded by console:config.read / console:config.write)
	core.GetRoute[Config](router, "/v1/_/console/config", service.controlPlaneHandler.handleGetConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Get dynamic runtime console configuration"),
		core.RouteDescription("Returns active runtime console configuration settings including session duration, MFA enforcement policy, and allowed origins. Requires console:config.read scope."),
		core.RouteOperationID("console__config__get"),
		core.RouteSDKGroupName("console", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/v1/_/console/config", service.controlPlaneHandler.handleUpdateConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Update dynamic runtime console configuration"),
		core.RouteDescription("Mutates runtime console configuration parameters in console.config, validating structure and applying settings dynamically across cluster nodes. Requires console:config.write scope."),
		core.RouteOperationID("console__config__update"),
		core.RouteSDKGroupName("console", "config"),
		core.RouteSDKMethodName("update"),
	)

	// 2. Console Authentication & Session Lifecycle
	core.PostRoute[SignInConsoleResponse, SignInConsoleInput](router, "/v1/_/console/sign-in", service.controlPlaneHandler.handleSignInConsole,
		core.RouteHide(),
		core.RouteTag("Auth"),
		core.RouteSummary("Sign in as console user"),
		core.RouteDescription("Authenticates a console user with email and password, issuing session cookie and optional MFA challenge."),
		core.RouteOperationID("console__auth__sign_in"),
		core.RouteSDKGroupName("console", "auth"),
		core.RouteSDKMethodName("sign_in"),
	)
	core.PostRoute[core.Empty, core.Empty](router, "/v1/_/console/sign-out", service.controlPlaneHandler.handleSignOutConsole,
		core.RouteHide(),
		core.RouteTag("Auth"),
		core.RouteSummary("Sign out active console user session"),
		core.RouteDescription("Terminates active console user session and clears authentication cookies."),
		core.RouteNoRequestBody(),
		core.RouteNoContentResponse("Signed out"),
		core.RouteOperationID("console__auth__sign_out"),
		core.RouteSDKGroupName("console", "auth"),
		core.RouteSDKMethodName("sign_out"),
	)
	core.GetRoute[GetConsoleSessionResponse](router, "/v1/_/console/session", service.controlPlaneHandler.handleGetConsoleSession,
		core.RouteHide(),
		core.RouteTag("Auth"),
		core.RouteSummary("Get current console user session profile"),
		core.RouteDescription("Returns console user profile and granted scopes for active console user session."),
		core.RouteOperationID("console__auth__session"),
		core.RouteSDKGroupName("console", "auth"),
		core.RouteSDKMethodName("session"),
	)
	core.PostRoute[SetupConsoleMFAResponse, core.Empty](router, "/v1/_/console/mfa", service.controlPlaneHandler.handleSetupConsoleMFA,
		core.RouteHide(),
		core.RouteTag("MFA"),
		core.RouteSummary("Initiate TOTP MFA enrollment"),
		core.RouteDescription("Generates a TOTP secret and provisioning URI for authenticator setup."),
		core.RouteNoRequestBody(),
		core.RouteOperationID("console__mfa__setup"),
		core.RouteSDKGroupName("console", "mfa"),
		core.RouteSDKMethodName("setup"),
	)
	core.PostRoute[core.Empty, VerifyConsoleMFAInput](router, "/v1/_/console/mfa/verify", service.controlPlaneHandler.handleVerifyConsoleMFA,
		core.RouteHide(),
		core.RouteTag("MFA"),
		core.RouteSummary("Verify and activate TOTP MFA"),
		core.RouteDescription("Verifies initial passcode to complete MFA activation."),
		core.RouteOperationID("console__mfa__verify"),
		core.RouteSDKGroupName("console", "mfa"),
		core.RouteSDKMethodName("verify"),
	)
	core.PostRoute[SignInConsoleResponse, ChallengeConsoleMFAInput](router, "/v1/_/console/mfa/challenge", service.controlPlaneHandler.handleChallengeConsoleMFA,
		core.RouteHide(),
		core.RouteTag("MFA"),
		core.RouteSummary("Verify MFA challenge ticket with TOTP code"),
		core.RouteDescription("Verifies the short-lived MFA challenge ticket and TOTP code after password sign-in and issues an authenticated console user session."),
		core.RouteOperationID("console__mfa__challenge"),
		core.RouteSDKGroupName("console", "mfa"),
		core.RouteSDKMethodName("challenge"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/console/mfa", service.controlPlaneHandler.handleDisableConsoleMFA,
		core.RouteHide(),
		core.RouteTag("MFA"),
		core.RouteSummary("Disable TOTP MFA"),
		core.RouteDescription("Removes active multi-factor authentication configuration."),
		core.RouteNoContentResponse("MFA disabled"),
		core.RouteOperationID("console__mfa__disable"),
		core.RouteSDKGroupName("console", "mfa"),
		core.RouteSDKMethodName("disable"),
	)

	// 3. Console Operator Accounts Management (Guarded by console:user.read / console:user.write)
	core.GetRoute[ListConsoleUsersResponse](router, "/v1/_/console/users", service.controlPlaneHandler.handleListConsoleUsers,
		core.RouteTag("Users"),
		core.RouteSummary("List console user accounts"),
		core.RouteDescription("Returns all registered console user accounts from console.users with email, active status, MFA enrollment status, last login timestamp, and creation date. Requires console:user.read scope."),
		core.RouteOperationID("console__users__list"),
		core.RouteSDKGroupName("console", "users"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[User, CreateConsoleUserInput](router, "/v1/_/console/users", service.controlPlaneHandler.handleCreateConsoleUser,
		core.RouteTag("Users"),
		core.RouteSummary("Create console user account"),
		core.RouteDescription("Creates a new console user account in console.users with email and Argon2id password hash (minimum 8 characters). Publishes console.user.created event. Requires console:user.write scope."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("console__users__create"),
		core.RouteSDKGroupName("console", "users"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[User](router, "/v1/_/console/users/{user_id}", service.controlPlaneHandler.handleGetConsoleUser,
		core.RouteTag("Users"),
		core.RouteSummary("Get console user account details by UUID"),
		core.RouteDescription("Retrieves a single console user account from console.users by UUID, including email, enabled status, MFA enrollment flag, and timestamps. Requires console:user.read scope."),
		core.RouteOperationID("console__users__get"),
		core.RouteSDKGroupName("console", "users"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[User, UpdateConsoleUserInput](router, "/v1/_/console/users/{user_id}", service.controlPlaneHandler.handleUpdateConsoleUser,
		core.RouteTag("Users"),
		core.RouteSummary("Update console user account properties"),
		core.RouteDescription("Modifies email or enabled status of a console user in console.users. If account is disabled, immediately revokes all active sessions in console.sessions. Emits console.user.updated event. Requires console:user.write scope."),
		core.RouteOperationID("console__users__update"),
		core.RouteSDKGroupName("console", "users"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/console/users/{user_id}", service.controlPlaneHandler.handleDeleteConsoleUser,
		core.RouteTag("Users"),
		core.RouteSummary("Delete console user account and cascade sessions"),
		core.RouteDescription("Permanently deletes console user account from console.users, automatically cascading revocation of all active sessions in console.sessions. Emits console.user.deleted event. Requires console:user.write scope."),
		core.RouteNoContentResponse("User deleted"),
		core.RouteOperationID("console__users__delete"),
		core.RouteSDKGroupName("console", "users"),
		core.RouteSDKMethodName("delete"),
	)
	core.PostRoute[core.Empty, ResetConsoleUserPasswordInput](router, "/v1/_/console/users/{user_id}/password", service.controlPlaneHandler.handleResetConsoleUserPassword,
		core.RouteTag("Users"),
		core.RouteSummary("Reset console user password and revoke sessions"),
		core.RouteDescription("Updates password hash in console.users using Argon2id key derivation and revokes all active sessions in console.sessions, forcing re-login across all devices. Emits console.user.password_reset event. Requires console:user.write scope."),
		core.RouteOperationID("console__users__reset_password"),
		core.RouteSDKGroupName("console", "users"),
		core.RouteSDKMethodName("reset_password"),
	)

	// 4. Console Operator Active Sessions Management
	core.GetRoute[ListConsoleUserSessionsResponse](router, "/v1/_/console/users/{user_id}/sessions", service.controlPlaneHandler.handleListConsoleUserSessions,
		core.RouteTag("Users Sessions"),
		core.RouteSummary("List active sessions for a console user"),
		core.RouteDescription("Returns all active sessions from console.sessions for the specified console user, including client IP address, user-agent string, creation timestamp, and expiration time. Requires console:user.read scope."),
		core.RouteOperationID("console__users__sessions__list"),
		core.RouteSDKGroupName("console", "users", "sessions"),
		core.RouteSDKMethodName("list"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/console/users/{user_id}/sessions/{session_id}", service.controlPlaneHandler.handleDeleteConsoleUserSession,
		core.RouteTag("Users Sessions"),
		core.RouteSummary("Revoke a specific console user session"),
		core.RouteDescription("Terminates a single session by session UUID from console.sessions for the specified console user. Emits console.user.session.revoked event. Requires console:user.write scope."),
		core.RouteNoContentResponse("Session revoked"),
		core.RouteOperationID("console__users__sessions__delete"),
		core.RouteSDKGroupName("console", "users", "sessions"),
		core.RouteSDKMethodName("delete"),
	)
	core.PostRoute[core.Empty, core.Empty](router, "/v1/_/console/users/{user_id}/sessions/revoke", service.controlPlaneHandler.handleRevokeConsoleUserSessions,
		core.RouteTag("Users Sessions"),
		core.RouteSummary("Revoke all active sessions for a console user"),
		core.RouteDescription("Terminates all active sessions in console.sessions for the specified console user, immediately invalidating cookie tokens. Emits console.user.sessions.revoked_all event. Requires console:user.write scope."),
		core.RouteNoRequestBody(),
		core.RouteNoContentResponse("Sessions revoked"),
		core.RouteOperationID("console__users__sessions__revoke"),
		core.RouteSDKGroupName("console", "users", "sessions"),
		core.RouteSDKMethodName("revoke"),
	)
}
