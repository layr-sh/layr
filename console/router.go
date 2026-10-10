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
		core.RouteTag("Console Config"),
		core.RouteSummary("Get runtime configuration"),
		core.RouteDescription("Returns active dynamic console configuration settings."),
		core.RouteOperationID("console__config__get"),
		core.RouteSDKGroupName("console", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/v1/_/console/config", service.controlPlaneHandler.handleUpdateConfig,
		core.RouteTag("Console Config"),
		core.RouteSummary("Update runtime configuration"),
		core.RouteDescription("Mutates dynamic runtime console configuration."),
		core.RouteOperationID("console__config__update"),
		core.RouteSDKGroupName("console", "config"),
		core.RouteSDKMethodName("update"),
	)

	// 2. Console Authentication & Session Lifecycle
	core.PostRoute[SignInConsoleResponse, SignInConsoleInput](router, "/v1/_/console/sign-in", service.controlPlaneHandler.handleSignInConsole,
		core.RouteHide(),
		core.RouteTag("Console Auth"),
		core.RouteSummary("Sign in as console operator"),
		core.RouteDescription("Authenticates a console user with credentials and optional MFA, issuing session cookie."),
		core.RouteOperationID("console__auth__sign_in"),
		core.RouteSDKGroupName("console", "auth"),
		core.RouteSDKMethodName("sign_in"),
	)
	core.PostRoute[core.Empty, core.Empty](router, "/v1/_/console/sign-out", service.controlPlaneHandler.handleSignOutConsole,
		core.RouteHide(),
		core.RouteTag("Console Auth"),
		core.RouteSummary("Sign out active console session"),
		core.RouteDescription("Terminates active console session and clears authentication cookies."),
		core.RouteNoRequestBody(),
		core.RouteNoContentResponse("Signed out"),
		core.RouteOperationID("console__auth__sign_out"),
		core.RouteSDKGroupName("console", "auth"),
		core.RouteSDKMethodName("sign_out"),
	)
	core.GetRoute[GetConsoleSessionResponse](router, "/v1/_/console/session", service.controlPlaneHandler.handleGetConsoleSession,
		core.RouteHide(),
		core.RouteTag("Console Auth"),
		core.RouteSummary("Get current console session profile"),
		core.RouteDescription("Returns operator profile and granted scopes for active console session."),
		core.RouteOperationID("console__auth__session"),
		core.RouteSDKGroupName("console", "auth"),
		core.RouteSDKMethodName("session"),
	)
	core.PostRoute[SetupConsoleMFAResponse, core.Empty](router, "/v1/_/console/mfa", service.controlPlaneHandler.handleSetupConsoleMFA,
		core.RouteHide(),
		core.RouteTag("Console MFA"),
		core.RouteSummary("Initiate TOTP MFA enrollment"),
		core.RouteDescription("Generates a TOTP secret and provisioning URI for authenticator setup."),
		core.RouteNoRequestBody(),
		core.RouteOperationID("console__mfa__setup"),
		core.RouteSDKGroupName("console", "mfa"),
		core.RouteSDKMethodName("setup"),
	)
	core.PostRoute[core.Empty, VerifyConsoleMFAInput](router, "/v1/_/console/mfa/verify", service.controlPlaneHandler.handleVerifyConsoleMFA,
		core.RouteHide(),
		core.RouteTag("Console MFA"),
		core.RouteSummary("Verify and activate TOTP MFA"),
		core.RouteDescription("Verifies initial passcode to complete MFA activation."),
		core.RouteOperationID("console__mfa__verify"),
		core.RouteSDKGroupName("console", "mfa"),
		core.RouteSDKMethodName("verify"),
	)
	core.PostRoute[SignInConsoleResponse, ChallengeConsoleMFAInput](router, "/v1/_/console/mfa/challenge", service.controlPlaneHandler.handleChallengeConsoleMFA,
		core.RouteHide(),
		core.RouteTag("Console MFA"),
		core.RouteSummary("Verify MFA challenge ticket with TOTP code"),
		core.RouteDescription("Verifies the short-lived MFA challenge ticket and TOTP code after password sign-in and issues an authenticated console session."),
		core.RouteOperationID("console__mfa__challenge"),
		core.RouteSDKGroupName("console", "mfa"),
		core.RouteSDKMethodName("challenge"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/console/mfa", service.controlPlaneHandler.handleDisableConsoleMFA,
		core.RouteHide(),
		core.RouteTag("Console MFA"),
		core.RouteSummary("Disable TOTP MFA"),
		core.RouteDescription("Removes active multi-factor authentication configuration."),
		core.RouteNoContentResponse("MFA disabled"),
		core.RouteOperationID("console__mfa__disable"),
		core.RouteSDKGroupName("console", "mfa"),
		core.RouteSDKMethodName("disable"),
	)

	// 3. Console Operator Accounts Management (Guarded by console:user.read / console:user.write)
	core.GetRoute[ListConsoleUsersResponse](router, "/v1/_/console/users", service.controlPlaneHandler.handleListConsoleUsers,
		core.RouteTag("Console Users"),
		core.RouteSummary("List console operator accounts"),
		core.RouteDescription("Returns all registered console operator users."),
		core.RouteOperationID("console__users__list"),
		core.RouteSDKGroupName("console", "users"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[User, CreateConsoleUserInput](router, "/v1/_/console/users", service.controlPlaneHandler.handleCreateConsoleUser,
		core.RouteTag("Console Users"),
		core.RouteSummary("Create console operator account"),
		core.RouteDescription("Creates a new console operator with credentials."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("console__users__create"),
		core.RouteSDKGroupName("console", "users"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[User](router, "/v1/_/console/users/{user_id}", service.controlPlaneHandler.handleGetConsoleUser,
		core.RouteTag("Console Users"),
		core.RouteSummary("Get console operator account details"),
		core.RouteDescription("Returns a single console operator user by ID."),
		core.RouteOperationID("console__users__get"),
		core.RouteSDKGroupName("console", "users"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[User, UpdateConsoleUserInput](router, "/v1/_/console/users/{user_id}", service.controlPlaneHandler.handleUpdateConsoleUser,
		core.RouteTag("Console Users"),
		core.RouteSummary("Update console operator account"),
		core.RouteDescription("Modifies email or active status of a console user."),
		core.RouteOperationID("console__users__update"),
		core.RouteSDKGroupName("console", "users"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/console/users/{user_id}", service.controlPlaneHandler.handleDeleteConsoleUser,
		core.RouteTag("Console Users"),
		core.RouteSummary("Delete console operator account"),
		core.RouteDescription("Deletes a console operator and cascades session revocation."),
		core.RouteNoContentResponse("User deleted"),
		core.RouteOperationID("console__users__delete"),
		core.RouteSDKGroupName("console", "users"),
		core.RouteSDKMethodName("delete"),
	)
	core.PostRoute[core.Empty, ResetConsoleUserPasswordInput](router, "/v1/_/console/users/{user_id}/password", service.controlPlaneHandler.handleResetConsoleUserPassword,
		core.RouteTag("Console Users"),
		core.RouteSummary("Reset console operator password"),
		core.RouteDescription("Rotates user password and revokes all active operator sessions."),
		core.RouteOperationID("console__users__reset_password"),
		core.RouteSDKGroupName("console", "users"),
		core.RouteSDKMethodName("reset_password"),
	)

	// 4. Console Operator Active Sessions Management
	core.GetRoute[ListConsoleUserSessionsResponse](router, "/v1/_/console/users/{user_id}/sessions", service.controlPlaneHandler.handleListConsoleUserSessions,
		core.RouteTag("Console Users"),
		core.RouteSummary("List active operator sessions"),
		core.RouteDescription("Returns active sessions for a console user."),
		core.RouteOperationID("console__users__sessions__list"),
		core.RouteSDKGroupName("console", "users", "sessions"),
		core.RouteSDKMethodName("list"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/_/console/users/{user_id}/sessions/{session_id}", service.controlPlaneHandler.handleDeleteConsoleUserSession,
		core.RouteTag("Console Users"),
		core.RouteSummary("Revoke specific operator session"),
		core.RouteDescription("Terminates a single console session."),
		core.RouteNoContentResponse("Session revoked"),
		core.RouteOperationID("console__users__sessions__delete"),
		core.RouteSDKGroupName("console", "users", "sessions"),
		core.RouteSDKMethodName("delete"),
	)
	core.PostRoute[core.Empty, core.Empty](router, "/v1/_/console/users/{user_id}/sessions/revoke", service.controlPlaneHandler.handleRevokeConsoleUserSessions,
		core.RouteTag("Console Users"),
		core.RouteSummary("Revoke all operator sessions"),
		core.RouteDescription("Terminates all active sessions for a console operator."),
		core.RouteNoRequestBody(),
		core.RouteNoContentResponse("Sessions revoked"),
		core.RouteOperationID("console__users__sessions__revoke"),
		core.RouteSDKGroupName("console", "users", "sessions"),
		core.RouteSDKMethodName("revoke"),
	)
}
