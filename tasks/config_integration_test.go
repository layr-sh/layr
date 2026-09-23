package tasks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksConfigPostgresIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(kernel)

	// 1. Initial Load when key does not exist -> seeds DefaultConfig
	require.NoError(t, configManager.Load(ctx))
	initialConfig := configManager.Get()
	require.Equal(t, defaultConcurrencyLimit, initialConfig.ConcurrencyLimit)

	// 2. Set custom configuration and verify roundtrip via Load
	customConfig := initialConfig
	customConfig.ConcurrencyLimit = 50
	customConfig.TimeoutSeconds = 60
	require.NoError(t, configManager.Set(ctx, customConfig))

	freshConfigManager := NewConfigManager(kernel)
	require.NoError(t, freshConfigManager.Load(ctx))
	loadedConfig := freshConfigManager.Get()
	require.Equal(t, 50, loadedConfig.ConcurrencyLimit)
	require.Equal(t, 60, loadedConfig.TimeoutSeconds)

	// 3. Corrupt stored JSON -> Load returns error
	const corruptSQLStatement = `UPDATE tasks.config SET value = '{"concurrency_limit":"not-a-number"}' WHERE key = 'tasks_config'`
	_, execErr := kernel.DB().Exec(ctx, corruptSQLStatement)
	require.NoError(t, execErr)

	corruptedConfigManager := NewConfigManager(kernel)
	require.Error(t, corruptedConfigManager.Load(ctx))

	// 4. Invalid validation config in database -> Load returns error
	const invalidValueSQLStatement = `UPDATE tasks.config SET value = '{"concurrency_limit":-10}' WHERE key = 'tasks_config'`
	_, execValueErr := kernel.DB().Exec(ctx, invalidValueSQLStatement)
	require.NoError(t, execValueErr)

	invalidConfigManager := NewConfigManager(kernel)
	require.Error(t, invalidConfigManager.Load(ctx))
}
