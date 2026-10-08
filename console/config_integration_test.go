package console

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestConsoleConfigPostgresIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(kernel)

	// 1. Initial Load when key does not exist -> seeds DefaultConfig
	require.NoError(t, configManager.Load(ctx))
	initialConfig := configManager.Get()
	require.Equal(t, DefaultSessionIdleTimeoutSeconds, initialConfig.SessionIdleTimeoutSeconds)
	require.Equal(t, DefaultSessionMaxLifetimeSeconds, initialConfig.SessionMaxLifetimeSeconds)

	// 2. Set custom configuration and verify roundtrip via Load
	customConfig := initialConfig
	customConfig.SessionIdleTimeoutSeconds = 3600
	customConfig.MFAEnforced = true
	customConfig.AllowedCIDRs = []string{"10.0.0.0/8"}
	require.NoError(t, configManager.Set(ctx, customConfig))

	freshConfigManager := NewConfigManager(kernel)
	require.NoError(t, freshConfigManager.Load(ctx))
	loadedConfig := freshConfigManager.Get()
	require.Equal(t, 3600, loadedConfig.SessionIdleTimeoutSeconds)
	require.True(t, loadedConfig.MFAEnforced)
	require.Equal(t, []string{"10.0.0.0/8"}, loadedConfig.AllowedCIDRs)

	// 3. Stored corrupt JSON fails on Load
	const corruptSQLStatement = `UPDATE console.config SET value = '{"session_idle_timeout_seconds":"not-a-number"}' WHERE key = 'runtime'`
	_, execErr := kernel.DB().Exec(ctx, corruptSQLStatement)
	require.NoError(t, execErr)

	corruptedConfigManager := NewConfigManager(kernel)
	require.Error(t, corruptedConfigManager.Load(ctx))

	// 4. Stored config with invalid values fails validation on Load
	const invalidValuesSQL = `UPDATE console.config SET value = '{"session_idle_timeout_seconds": 0, "session_max_lifetime_seconds": 100}' WHERE key = 'runtime'`
	_, execErr = kernel.DB().Exec(ctx, invalidValuesSQL)
	require.NoError(t, execErr)

	invalidConfigManager := NewConfigManager(kernel)
	loadErr := invalidConfigManager.Load(ctx)
	require.Error(t, loadErr)
	require.Contains(t, loadErr.Error(), "stored console config is invalid")

	// 5. Canceled context causes query error on Load
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, freshConfigManager.Load(canceledCtx))
}
