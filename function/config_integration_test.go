package function

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionConfigPostgresIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(kernel)

	// 1. Initial Load when key does not exist -> seeds DefaultConfig
	require.NoError(t, configManager.Load(ctx))
	initialConfig := configManager.Get()
	require.Equal(t, DefaultRuntime, initialConfig.DefaultRuntime)
	require.Equal(t, DefaultMemoryLimitMB, initialConfig.DefaultMemoryLimitMB)

	// 2. Set custom configuration and verify roundtrip via Load
	customConfig := initialConfig
	customConfig.DefaultMemoryLimitMB = 256
	customConfig.WorkerdRuntime = WorkerdRuntimeConfig{
		CompatibilityDate:  "2026-02-01",
		CompatibilityFlags: []string{"nodejs_compat"},
	}
	require.NoError(t, configManager.Set(ctx, customConfig))

	freshConfigManager := NewConfigManager(kernel)
	require.NoError(t, freshConfigManager.Load(ctx))
	loadedConfig := freshConfigManager.Get()
	require.Equal(t, 256, loadedConfig.DefaultMemoryLimitMB)
	require.Equal(t, "2026-02-01", loadedConfig.WorkerdRuntime.CompatibilityDate)
	require.Equal(t, []string{"nodejs_compat"}, loadedConfig.WorkerdRuntime.CompatibilityFlags)

	// 3. Stored corrupt JSON fails on Load
	const corruptSQLStatement = `UPDATE function.config SET value = '{"default_memory_limit_mb":"not-a-number"}' WHERE key = 'runtime'`
	_, execErr := kernel.DB().Exec(ctx, corruptSQLStatement)
	require.NoError(t, execErr)

	corruptedConfigManager := NewConfigManager(kernel)
	require.Error(t, corruptedConfigManager.Load(ctx))

	// 4. Stored config with invalid values fails validation on Load
	const invalidValuesSQL = `UPDATE function.config SET value = '{"default_runtime": "", "default_memory_limit_mb": 128, "default_timeout_seconds": 30, "max_bundle_size_bytes": 1000}' WHERE key = 'runtime'`
	_, execErr = kernel.DB().Exec(ctx, invalidValuesSQL)
	require.NoError(t, execErr)

	invalidConfigManager := NewConfigManager(kernel)
	loadErr := invalidConfigManager.Load(ctx)
	require.Error(t, loadErr)
	require.Contains(t, loadErr.Error(), "stored function config is invalid")

	// 5. Canceled context causes query error on Load
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, configManager.Load(canceledCtx))
}
