package image

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageConfigUnit(t *testing.T) {
	t.Parallel()

	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	t.Run("default config validity", func(t *testing.T) {
		t.Parallel()
		defaultConfig := DefaultConfig()
		require.NoError(t, defaultConfig.Validate())
		require.Equal(t, DefaultQuality, defaultConfig.DefaultQuality)
		require.Equal(t, DefaultMaxSrcResolution, defaultConfig.MaxSrcResolution)
		require.Equal(t, DefaultMaxAnimationFrames, defaultConfig.MaxAnimationFrames)
		require.Equal(t, DefaultCacheTTLSeconds, defaultConfig.CacheTTLSeconds)
		require.Equal(t, DefaultWatermarkOpacity, defaultConfig.WatermarkOpacity)
	})

	t.Run("config validation failure paths", func(t *testing.T) {
		t.Parallel()
		lowQualityConfig := DefaultConfig()
		lowQualityConfig.DefaultQuality = 0
		require.Error(t, lowQualityConfig.Validate())

		highQualityConfig := DefaultConfig()
		highQualityConfig.DefaultQuality = 101
		require.Error(t, highQualityConfig.Validate())

		resolutionConfig := DefaultConfig()
		resolutionConfig.MaxSrcResolution = 0
		require.Error(t, resolutionConfig.Validate())

		animationConfig := DefaultConfig()
		animationConfig.MaxAnimationFrames = 0
		require.Error(t, animationConfig.Validate())

		cacheTTLConfig := DefaultConfig()
		cacheTTLConfig.CacheTTLSeconds = -1
		require.Error(t, cacheTTLConfig.Validate())

		lowOpacityConfig := DefaultConfig()
		lowOpacityConfig.WatermarkOpacity = -0.1
		require.Error(t, lowOpacityConfig.Validate())

		highOpacityConfig := DefaultConfig()
		highOpacityConfig.WatermarkOpacity = 1.1
		require.Error(t, highOpacityConfig.Validate())
	})

	t.Run("config manager load and set lifecycle", func(t *testing.T) {
		configManager := NewConfigManager(kernel)
		ctx := context.Background()

		// Initial load writes default to DB
		loadErr := configManager.Load(ctx)
		require.NoError(t, loadErr)

		// Set new config
		customConfig := DefaultConfig()
		customConfig.DefaultQuality = 88
		customConfig.AllowedDomains = []string{"cdn.example.com"}
		setErr := configManager.Set(ctx, customConfig)
		require.NoError(t, setErr)

		// Get from memory
		activeConfig := configManager.Get()
		require.Equal(t, 88, activeConfig.DefaultQuality)
		require.Contains(t, activeConfig.AllowedDomains, "cdn.example.com")

		// Reload from DB into a fresh manager
		reloadedConfigManager := NewConfigManager(kernel)
		reloadErr := reloadedConfigManager.Load(ctx)
		require.NoError(t, reloadErr)
		require.Equal(t, 88, reloadedConfigManager.Get().DefaultQuality)

		// Set memory config without DB write
		directConfig := DefaultConfig()
		directConfig.DefaultQuality = 92
		reloadedConfigManager.SetMemoryConfig(directConfig)
		require.Equal(t, 92, reloadedConfigManager.Get().DefaultQuality)

		// Derive subkeys
		require.NotEmpty(t, configManager.SigningKey())
		require.NotEmpty(t, configManager.SigningSalt())

		// Invalid config rejected by Set
		invalidConfig := DefaultConfig()
		invalidConfig.DefaultQuality = 0
		require.Error(t, configManager.Set(ctx, invalidConfig))

		// Corrupted JSON in image.config table (valid JSONB syntax, mismatched struct types)
		_, execErr := kernel.DB().Exec(ctx, "UPDATE image.config SET value = '{\"default_quality\":\"not-a-number\"}' WHERE key = $1", ConfigKey)
		require.NoError(t, execErr)
		corruptLoadErr := configManager.Load(ctx)
		require.Error(t, corruptLoadErr)
		require.Contains(t, corruptLoadErr.Error(), "failed to parse dynamic image config JSON")
	})

	t.Run("config manager with broken database fails gracefully", func(t *testing.T) {
		brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
		configManager := NewConfigManager(brokenKernel)
		ctx := context.Background()

		require.Error(t, configManager.Set(ctx, DefaultConfig()))
	})
}
