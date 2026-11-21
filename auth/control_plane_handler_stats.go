package auth

import (
	"net/http"

	"layr.sh/core"
)

// handleGetStats handles GET /v1/_/auth/stats.
func (controlPlaneHandler *ControlPlaneHandler) handleGetStats(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeAuthStatsRead) {
		return
	}

	// Aggregate each relation independently; joining credentials to sessions would multiply user counts.
	const querySQL = `
	WITH snapshot AS MATERIALIZED (SELECT statement_timestamp() AS generated_at),
	user_directory AS MATERIALIZED (
		SELECT u.id, u.role, u.is_anonymous, u.created_at, u.locked_until,
			u.mfa_enabled, u.email_verified_at, u.phone_verified_at, u.email, u.phone,
			(u.password_hash IS NOT NULL AND u.password_hash <> '') AS has_password,
			EXISTS (SELECT 1 FROM auth.passkeys p WHERE p.user_id = u.id) AS has_passkey,
			EXISTS (SELECT 1 FROM auth.identities i WHERE i.user_id = u.id) AS has_identity
		FROM auth.users u
	), user_stats AS (
		SELECT COUNT(*) AS total_users,
			COUNT(*) FILTER (WHERE NOT is_anonymous) AS registered_users,
			COUNT(*) FILTER (WHERE is_anonymous) AS anonymous_users,
			COUNT(*) FILTER (WHERE locked_until > snapshot.generated_at) AS locked_users,
			COUNT(*) FILTER (WHERE mfa_enabled) AS mfa_users,
			COUNT(*) FILTER (WHERE email_verified_at IS NOT NULL) AS email_verified_users,
			COUNT(*) FILTER (WHERE phone_verified_at IS NOT NULL) AS phone_verified_users,
			COUNT(*) FILTER (WHERE has_password) AS password_users,
			COUNT(*) FILTER (WHERE has_passkey) AS passkey_users,
			COUNT(*) FILTER (WHERE has_identity) AS federated_users,
			COUNT(*) FILTER (WHERE created_at >= snapshot.generated_at - interval '24 hours' AND created_at <= snapshot.generated_at) AS new_users_last_24_hours,
			COUNT(*) FILTER (WHERE created_at >= snapshot.generated_at - interval '48 hours' AND created_at < snapshot.generated_at - interval '24 hours') AS new_users_previous_24_hours,
			COUNT(*) FILTER (WHERE created_at >= snapshot.generated_at - interval '7 days' AND created_at <= snapshot.generated_at) AS new_users_last_7_days,
			COUNT(*) FILTER (WHERE email IS NOT NULL AND email <> '' AND email_verified_at IS NULL) AS email_unverified_users,
			COUNT(*) FILTER (WHERE phone IS NOT NULL AND phone <> '' AND phone_verified_at IS NULL) AS phone_unverified_users,
			COUNT(*) FILTER (WHERE NOT is_anonymous AND NOT has_password AND NOT has_passkey AND NOT has_identity) AS users_without_credentials
		FROM user_directory CROSS JOIN snapshot
	), session_states AS (
		SELECT s.user_id,
			CASE WHEN s.expires_at <= snapshot.generated_at THEN 'expired'
				WHEN u.locked_until > snapshot.generated_at THEN 'locked'
				WHEN $1 > 0 AND s.last_used_at <= snapshot.generated_at - make_interval(secs => $1) THEN 'idle'
				ELSE 'active' END AS state
		FROM auth.sessions s JOIN user_directory u ON u.id = s.user_id CROSS JOIN snapshot
	), session_stats AS (
		SELECT COUNT(*) AS total_sessions,
			COUNT(*) FILTER (WHERE state = 'active') AS active_sessions,
			COUNT(DISTINCT user_id) FILTER (WHERE state = 'active') AS active_session_users,
			COUNT(*) FILTER (WHERE state = 'expired') AS expired_sessions,
			COUNT(*) FILTER (WHERE state = 'locked') AS locked_sessions,
			COUNT(*) FILTER (WHERE state = 'idle') AS idle_sessions
		FROM session_states
	), provider_stats AS (
		SELECT provider, COUNT(*) AS identity_count, COUNT(DISTINCT user_id) AS user_count,
			MAX(last_signed_in_at) AS last_signed_in_at
		FROM auth.identities GROUP BY provider
	), role_stats AS (
		SELECT role, COUNT(*) AS total_users FROM user_directory GROUP BY role
	), registration_days AS (
		SELECT day FROM snapshot CROSS JOIN LATERAL generate_series(
			date_trunc('day', generated_at AT TIME ZONE 'UTC') - interval '13 days',
			date_trunc('day', generated_at AT TIME ZONE 'UTC'), interval '1 day') day
	), registration_stats AS (
		SELECT d.day AT TIME ZONE 'UTC' AS day, COUNT(u.id) AS total_users,
			COUNT(u.id) FILTER (WHERE NOT u.is_anonymous) AS registered_users,
			COUNT(u.id) FILTER (WHERE u.is_anonymous) AS anonymous_users
		FROM registration_days d CROSS JOIN snapshot LEFT JOIN user_directory u
			ON u.created_at >= d.day AT TIME ZONE 'UTC'
			AND u.created_at < (d.day + interval '1 day') AT TIME ZONE 'UTC'
			AND u.created_at <= snapshot.generated_at
		GROUP BY d.day
	)
	SELECT total_users, registered_users, anonymous_users, locked_users, mfa_users,
		email_verified_users, phone_verified_users, password_users, passkey_users, federated_users,
		new_users_last_24_hours, total_sessions, active_sessions, active_session_users,
		(SELECT COUNT(*) FROM auth.passkeys), (SELECT COUNT(*) FROM auth.identities), generated_at,
		new_users_previous_24_hours, new_users_last_7_days, email_unverified_users, phone_unverified_users,
		users_without_credentials, expired_sessions, locked_sessions, idle_sessions,
		(SELECT COALESCE(jsonb_agg(to_jsonb(provider_stats) ORDER BY identity_count DESC, provider), '[]'::jsonb) FROM provider_stats),
		(SELECT COALESCE(jsonb_agg(to_jsonb(role_stats) ORDER BY total_users DESC, role), '[]'::jsonb) FROM role_stats),
		(SELECT jsonb_agg(to_jsonb(registration_stats) ORDER BY day) FROM registration_stats)
	FROM user_stats CROSS JOIN session_stats CROSS JOIN snapshot`

	var statsResponse StatsResponse
	if err := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), querySQL, controlPlaneHandler.configManager.Get().Sessions.IdleTimeoutSeconds).Scan(
		&statsResponse.TotalUsers, &statsResponse.RegisteredUsers, &statsResponse.AnonymousUsers,
		&statsResponse.LockedUsers, &statsResponse.MFAUsers, &statsResponse.EmailVerifiedUsers,
		&statsResponse.PhoneVerifiedUsers, &statsResponse.PasswordUsers, &statsResponse.PasskeyUsers,
		&statsResponse.FederatedUsers, &statsResponse.NewUsersLast24Hours,
		&statsResponse.TotalSessions, &statsResponse.ActiveSessions, &statsResponse.ActiveSessionUsers,
		&statsResponse.TotalPasskeys, &statsResponse.TotalIdentities, &statsResponse.GeneratedAt,
		&statsResponse.NewUsersPrevious24Hours, &statsResponse.NewUsersLast7Days,
		&statsResponse.EmailUnverifiedUsers, &statsResponse.PhoneUnverifiedUsers, &statsResponse.UsersWithoutCredentials,
		&statsResponse.ExpiredSessions, &statsResponse.LockedSessions, &statsResponse.IdleSessions,
		&statsResponse.Providers, &statsResponse.Roles, &statsResponse.Registrations,
	); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to load authentication statistics", err.Error())
		return
	}
	core.WriteJSONResponse(responseWriter, http.StatusOK, statsResponse)
}
