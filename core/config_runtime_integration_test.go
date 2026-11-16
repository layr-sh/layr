package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoreRuntimeConfigIntegration(t *testing.T) {
	kernel, cleanup := SetupTestKernel(t, nil)
	defer cleanup()
	ctx := t.Context()
	configManager := kernel.configManager
	require.Equal(t, DefaultRuntimeConfig(), configManager.Get())
	require.NoError(t, configManager.Load(ctx))
	runtimeConfig := configManager.Get()
	runtimeConfig.AllowedCIDRs = []string{"127.0.0.0/8", "::1"}
	runtimeConfig.AuditRetentionDays = 30
	require.NoError(t, configManager.Set(ctx, runtimeConfig))
	freshConfigManager := NewConfigManager(kernel)
	require.NoError(t, freshConfigManager.Load(ctx))
	require.Equal(t, runtimeConfig, freshConfigManager.Get())
	runtimeConfig.AllowedCIDRs = []string{"invalid"}
	require.ErrorIs(t, configManager.Set(ctx, runtimeConfig), ErrInvalidConfig)
	_, err := kernel.DB().Exec(ctx, `UPDATE core.config SET value = '{"audit_retention_days":"invalid"}' WHERE key = 'runtime'`)
	require.NoError(t, err)
	require.ErrorContains(t, configManager.Load(ctx), "failed to parse stored core config")
	require.ErrorContains(t, kernel.InitDatabase(ctx), "failed to load core config")
	_, err = kernel.DB().Exec(ctx, `UPDATE core.config SET value = '{"audit_retention_days":0}' WHERE key = 'runtime'`)
	require.NoError(t, err)
	require.ErrorContains(t, configManager.Load(ctx), "stored core config is invalid")
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorContains(t, configManager.Load(canceledCtx), "failed to query core.config")
	require.ErrorContains(t, configManager.Set(canceledCtx, DefaultRuntimeConfig()), "failed to persist core config")

	server := kernel.Server()
	configManager.SetMemoryConfig(RuntimeConfig{AllowedCIDRs: []string{"127.0.0.1"}, AuditRetentionDays: 90})
	for _, path := range []string{"/healthz", "/v1/_/core/events", "/v1/_/console/sign-in", "/v1/storage/buckets"} {
		t.Run("disallowed network "+path, func(t *testing.T) {
			request := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
			request.RemoteAddr = "192.0.2.1:1234"
			responseRecorder := httptest.NewRecorder()
			server.Handler().ServeHTTP(responseRecorder, request)
			require.Equal(t, http.StatusForbidden, responseRecorder.Code)
		})
	}
	for _, allowed := range []string{"127.0.0.1", "127.0.0.0/8"} {
		t.Run("allowed network "+allowed, func(t *testing.T) {
			configManager.SetMemoryConfig(RuntimeConfig{AllowedCIDRs: []string{allowed}, AuditRetentionDays: 90})
			request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/healthz", nil)
			request.RemoteAddr = "127.0.0.1:1234"
			responseRecorder := httptest.NewRecorder()
			server.Handler().ServeHTTP(responseRecorder, request)
			require.Equal(t, http.StatusOK, responseRecorder.Code)
		})
	}
}
