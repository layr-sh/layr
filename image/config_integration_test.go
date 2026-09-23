package image

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageConfigPostgresIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(kernel)

	// 1. Initial Load when key does not exist -> seeds DefaultConfig
	require.NoError(t, configManager.Load(ctx))
	initialConfig := configManager.Get()
	require.Equal(t, DefaultQuality, initialConfig.DefaultQuality)

	// 2. Set custom configuration and verify roundtrip via Load
	customConfig := initialConfig
	customConfig.DefaultQuality = 92
	customConfig.AllowedDomains = []string{"cdn.example.com"}
	require.NoError(t, configManager.Set(ctx, customConfig))

	freshConfigManager := NewConfigManager(kernel)
	require.NoError(t, freshConfigManager.Load(ctx))
	loadedConfig := freshConfigManager.Get()
	require.Equal(t, 92, loadedConfig.DefaultQuality)
	require.Contains(t, loadedConfig.AllowedDomains, "cdn.example.com")

	const corruptSQLStatement = `UPDATE image.config SET value = '{"default_quality":"not-a-number"}' WHERE key = 'runtime'`
	_, execErr := kernel.DB().Exec(ctx, corruptSQLStatement)
	require.NoError(t, execErr)

	corruptedConfigManager := NewConfigManager(kernel)
	require.Error(t, corruptedConfigManager.Load(ctx))
}
