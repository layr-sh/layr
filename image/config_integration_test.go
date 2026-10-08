package image

import (
	"context"
	"sync"
	"testing"
	"time"

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

	// 2. Set custom configuration and verify roundtrip via Load and event emission
	var capturedEvents []core.Event
	var eventMutex sync.Mutex
	kernel.EventBus().Subscribe("image.config.updated", func(eventCtx context.Context, event core.Event) error {
		eventMutex.Lock()
		defer eventMutex.Unlock()
		capturedEvents = append(capturedEvents, event)
		return nil
	})

	customConfig := initialConfig
	customConfig.DefaultQuality = 92
	customConfig.AllowedDomains = []string{"cdn.example.com"}
	require.NoError(t, configManager.Set(ctx, customConfig))

	time.Sleep(50 * time.Millisecond)
	eventMutex.Lock()
	require.Len(t, capturedEvents, 1)
	require.NotNil(t, capturedEvents[0].ResourceID)
	require.Equal(t, "image.config", *capturedEvents[0].ResourceID)
	eventMutex.Unlock()

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

	// 4. Stored config with invalid values fails validation on Load
	const invalidValuesSQL = `UPDATE image.config SET value = '{"default_quality": 0, "max_src_resolution": 10, "max_animation_frames": 10, "cache_ttl_seconds": 10, "watermark_opacity": 0.5}' WHERE key = 'runtime'`
	_, execErr = kernel.DB().Exec(ctx, invalidValuesSQL)
	require.NoError(t, execErr)

	invalidConfigManager := NewConfigManager(kernel)
	loadErr := invalidConfigManager.Load(ctx)
	require.Error(t, loadErr)
	require.Contains(t, loadErr.Error(), "stored image config is invalid")

	// 5. Canceled context causes query errors on Load and Set
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, configManager.Load(canceledCtx))
	require.Error(t, configManager.Set(canceledCtx, customConfig))
}
