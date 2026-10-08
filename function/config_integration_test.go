package function

import (
	"context"
	"sync"
	"testing"
	"time"

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

	// 2. Set custom configuration and verify roundtrip via Load and event emission
	var capturedEvents []core.Event
	var eventMutex sync.Mutex
	kernel.EventBus().Subscribe("function.config.updated", func(eventCtx context.Context, event core.Event) error {
		eventMutex.Lock()
		defer eventMutex.Unlock()
		capturedEvents = append(capturedEvents, event)
		return nil
	})

	customConfig := initialConfig
	customConfig.DefaultMemoryLimitMB = 256
	customConfig.WorkerdRuntime = WorkerdRuntimeConfig{
		CompatibilityDate:  "2026-02-01",
		CompatibilityFlags: []string{"nodejs_compat"},
	}
	require.NoError(t, configManager.Set(ctx, customConfig))

	time.Sleep(50 * time.Millisecond)
	eventMutex.Lock()
	require.Len(t, capturedEvents, 1)
	require.NotNil(t, capturedEvents[0].ResourceID)
	require.Equal(t, "function.config", *capturedEvents[0].ResourceID)
	eventMutex.Unlock()

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

	// 5. Canceled context causes query errors on Load and Set
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, configManager.Load(canceledCtx))
	require.Error(t, configManager.Set(canceledCtx, customConfig))
}
