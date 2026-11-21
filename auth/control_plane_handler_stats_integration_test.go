package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestAuthControlPlaneHandlerStatsIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	service := NewService(kernel)
	ctx := core.WithAuthContext(context.Background(), core.AuthContext{ServiceAccountID: "stats-reader", JWT: core.JWTClaims{Scope: core.ScopeAuthStatsRead}})
	load := func() StatsResponse {
		responseRecorder := httptest.NewRecorder()
		service.controlPlaneHandler.handleGetStats(responseRecorder, httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/stats", nil))
		require.Equal(t, http.StatusOK, responseRecorder.Code, responseRecorder.Body.String())
		var statsResponse StatsResponse
		require.NoError(t, json.Unmarshal(responseRecorder.Body.Bytes(), &statsResponse))
		return statsResponse
	}
	statsResponse := load()
	require.Zero(t, statsResponse.TotalUsers)
	require.Zero(t, statsResponse.TotalSessions)
	require.Zero(t, statsResponse.TotalIdentities)
	require.Zero(t, statsResponse.TotalPasskeys)
	require.Empty(t, statsResponse.Providers)
	require.Empty(t, statsResponse.Roles)
	require.Len(t, statsResponse.Registrations, 14)
	for _, day := range statsResponse.Registrations {
		t.Run("empty day "+day.Day.Format(time.DateOnly), func(t *testing.T) {
			require.Zero(t, day.TotalUsers)
			require.Zero(t, day.RegisteredUsers)
			require.Zero(t, day.AnonymousUsers)
			require.Zero(t, day.Day.Hour())
		})
	}
	require.WithinDuration(t, time.Now(), statsResponse.GeneratedAt, time.Minute)

	registeredUserID, anonymousUserID, lockedUserID := uuid.NewV7().String(), uuid.NewV7().String(), uuid.NewV7().String()
	_, err := kernel.DB().Exec(ctx, `
		INSERT INTO auth.users (email, created_at)
		SELECT 'old-' || n || '@example.com', clock_timestamp() - interval '2 days' FROM generate_series(1,31) n;
		INSERT INTO auth.users (id, email, password_hash, mfa_enabled, email_verified_at, phone_verified_at)
		VALUES ($1, 'stats@example.com', 'configured-password', true, clock_timestamp(), clock_timestamp());
		INSERT INTO auth.users (id, is_anonymous) VALUES ($2, true);
		INSERT INTO auth.users (id, locked_until) VALUES ($3, clock_timestamp() + interval '1 hour');
	`, pgx.QueryExecModeSimpleProtocol, registeredUserID, anonymousUserID, lockedUserID)
	require.NoError(t, err)
	_, err = kernel.DB().Exec(ctx, `
		INSERT INTO auth.passkeys (user_id, credential_id, public_key)
		VALUES ($1, 'credential-one'::bytea, 'key'::bytea), ($1, 'credential-two'::bytea, 'key'::bytea);
		INSERT INTO auth.identities (user_id, provider, provider_user_id)
		VALUES ($1, 'google', 'one'), ($1, 'github', 'one'), ($2, 'google', 'two');
		INSERT INTO auth.sessions (user_id, refresh_token_hash, expires_at, last_used_at)
		VALUES ($1, 'active-one', clock_timestamp() + interval '1 hour', clock_timestamp()),
			($1, 'active-two', clock_timestamp() + interval '1 hour', clock_timestamp()),
			($2, 'active-guest', clock_timestamp() + interval '1 hour', clock_timestamp()),
			($3, 'locked', clock_timestamp() + interval '1 hour', clock_timestamp()),
			($1, 'expired', clock_timestamp() - interval '1 hour', clock_timestamp()),
			($1, 'idle', clock_timestamp() + interval '1 hour', clock_timestamp() - interval '2 hours');
	`, pgx.QueryExecModeSimpleProtocol, registeredUserID, anonymousUserID, lockedUserID)
	require.NoError(t, err)
	config := DefaultConfig()
	config.Sessions.IdleTimeoutSeconds = 3600
	service.configManager.SetMemoryConfig(config)
	statsResponse = load()
	require.Equal(t, int64(34), statsResponse.TotalUsers)
	require.Equal(t, int64(33), statsResponse.RegisteredUsers)
	require.Equal(t, int64(1), statsResponse.AnonymousUsers)
	require.Equal(t, int64(1), statsResponse.LockedUsers)
	require.Equal(t, int64(1), statsResponse.MFAUsers)
	require.Equal(t, int64(1), statsResponse.EmailVerifiedUsers)
	require.Equal(t, int64(1), statsResponse.PhoneVerifiedUsers)
	require.Equal(t, int64(1), statsResponse.PasswordUsers)
	require.Equal(t, int64(1), statsResponse.PasskeyUsers)
	require.Equal(t, int64(2), statsResponse.FederatedUsers)
	require.Equal(t, int64(3), statsResponse.NewUsersLast24Hours)
	require.Equal(t, int64(6), statsResponse.TotalSessions)
	require.Equal(t, int64(3), statsResponse.ActiveSessions)
	require.Equal(t, int64(2), statsResponse.ActiveSessionUsers)
	require.Equal(t, int64(2), statsResponse.TotalPasskeys)
	require.Equal(t, int64(3), statsResponse.TotalIdentities)
	require.Equal(t, int64(0), statsResponse.NewUsersPrevious24Hours)
	require.Equal(t, int64(34), statsResponse.NewUsersLast7Days)
	require.Equal(t, int64(31), statsResponse.EmailUnverifiedUsers)
	require.Zero(t, statsResponse.PhoneUnverifiedUsers)
	require.Equal(t, int64(32), statsResponse.UsersWithoutCredentials)
	require.Equal(t, int64(1), statsResponse.ExpiredSessions)
	require.Equal(t, int64(1), statsResponse.LockedSessions)
	require.Equal(t, int64(1), statsResponse.IdleSessions)
	require.Equal(t, statsResponse.TotalSessions, statsResponse.ActiveSessions+statsResponse.ExpiredSessions+statsResponse.LockedSessions+statsResponse.IdleSessions)
	require.Len(t, statsResponse.Providers, 2)
	require.Equal(t, "google", statsResponse.Providers[0].Provider)
	require.Equal(t, int64(2), statsResponse.Providers[0].IdentityCount)
	require.Equal(t, int64(2), statsResponse.Providers[0].UserCount)
	require.NotNil(t, statsResponse.Providers[0].LastSignedInAt)
	require.Equal(t, "github", statsResponse.Providers[1].Provider)
	require.Equal(t, []RoleStats{{Role: "authenticated", TotalUsers: 34}}, statsResponse.Roles)
	var registrations int64
	for index, day := range statsResponse.Registrations {
		t.Run(fmt.Sprintf("registration day %d", index), func(t *testing.T) {
			require.Equal(t, day.TotalUsers, day.RegisteredUsers+day.AnonymousUsers)
			if index > 0 {
				require.Equal(t, 24*time.Hour, day.Day.Sub(statsResponse.Registrations[index-1].Day))
			}
		})
		registrations += day.TotalUsers
	}
	require.Equal(t, int64(34), registrations)
	config.Sessions.IdleTimeoutSeconds = 0
	service.configManager.SetMemoryConfig(config)
	require.Equal(t, int64(4), load().ActiveSessions)

	server := core.NewServer(kernel)
	service.RegisterRoutes(server.BaseRouter(), server.ControlPlaneRouter(), server.Mux())
	serviceAccount, err := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{Name: "Auth stats reader", Scopes: []string{core.ScopeAuthStatsRead}})
	require.NoError(t, err)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/_/auth/stats", nil)
	request.Header.Set("Service-Account-Key", serviceAccount.SecretKey)
	responseRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(responseRecorder, request)
	require.Equal(t, http.StatusOK, responseRecorder.Code, responseRecorder.Body.String())
	require.NoError(t, json.Unmarshal(responseRecorder.Body.Bytes(), &statsResponse))
	require.Equal(t, int64(34), statsResponse.TotalUsers)
}

func TestAuthControlPlaneHandlerStatsWindowsIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	service := NewService(kernel)
	ctx := core.WithAuthContext(context.Background(), core.AuthContext{ServiceAccountID: "reader", JWT: core.JWTClaims{Scope: core.ScopeAuthStatsRead}})
	_, err := kernel.DB().Exec(ctx, `
		INSERT INTO auth.users (email,phone,role,is_anonymous,created_at) VALUES
		('today@example.com','+15551234567','admin',false,statement_timestamp()),
		(NULL,NULL,'authenticated',true,statement_timestamp()-interval '36 hours'),
		(NULL,NULL,'authenticated',false,statement_timestamp()-interval '6 days'),
		(NULL,NULL,'authenticated',false,statement_timestamp()-interval '20 days'),
		(NULL,NULL,'authenticated',false,statement_timestamp()+interval '1 day');
	`)
	require.NoError(t, err)
	responseRecorder := httptest.NewRecorder()
	service.controlPlaneHandler.handleGetStats(responseRecorder, httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/auth/stats", nil))
	require.Equal(t, http.StatusOK, responseRecorder.Code, responseRecorder.Body.String())
	var statsResponse StatsResponse
	require.NoError(t, json.Unmarshal(responseRecorder.Body.Bytes(), &statsResponse))
	require.Equal(t, int64(5), statsResponse.TotalUsers)
	require.Equal(t, int64(1), statsResponse.NewUsersLast24Hours)
	require.Equal(t, int64(1), statsResponse.NewUsersPrevious24Hours)
	require.Equal(t, int64(3), statsResponse.NewUsersLast7Days)
	require.Equal(t, int64(1), statsResponse.EmailUnverifiedUsers)
	require.Equal(t, int64(1), statsResponse.PhoneUnverifiedUsers)
	require.Equal(t, []RoleStats{{Role: "authenticated", TotalUsers: 4}, {Role: "admin", TotalUsers: 1}}, statsResponse.Roles)
	var total int64
	for _, day := range statsResponse.Registrations {
		total += day.TotalUsers
	}
	require.Equal(t, int64(3), total, "future and older-than-window registrations must not appear in the UTC trend")
	require.Equal(t, int64(1), statsResponse.Registrations[13].TotalUsers)
	require.Equal(t, int64(1), statsResponse.Registrations[13].RegisteredUsers)
}
