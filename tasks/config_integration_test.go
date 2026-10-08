package tasks

import (
	"context"
	"sync"
	"testing"
	"time"

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

	// 2. Set custom configuration and verify roundtrip via Load and event emission
	var capturedEvents []core.Event
	var eventMutex sync.Mutex
	kernel.EventBus().Subscribe("tasks.config.updated", func(eventCtx context.Context, event core.Event) error {
		eventMutex.Lock()
		defer eventMutex.Unlock()
		capturedEvents = append(capturedEvents, event)
		return nil
	})

	customConfig := initialConfig
	customConfig.ConcurrencyLimit = 50
	customConfig.TimeoutSeconds = 60
	require.NoError(t, configManager.Set(ctx, customConfig))

	time.Sleep(50 * time.Millisecond)
	eventMutex.Lock()
	require.Len(t, capturedEvents, 1)
	require.NotNil(t, capturedEvents[0].ResourceID)
	require.Equal(t, "tasks.config", *capturedEvents[0].ResourceID)
	eventMutex.Unlock()

	freshConfigManager := NewConfigManager(kernel)
	require.NoError(t, freshConfigManager.Load(ctx))
	loadedConfig := freshConfigManager.Get()
	require.Equal(t, 50, loadedConfig.ConcurrencyLimit)
	require.Equal(t, 60, loadedConfig.TimeoutSeconds)

	// 3. Corrupt stored JSON -> Load returns error
	const corruptSQLStatement = `UPDATE tasks.config SET value = '{"concurrency_limit":"not-a-number"}' WHERE key = 'runtime'`
	_, execErr := kernel.DB().Exec(ctx, corruptSQLStatement)
	require.NoError(t, execErr)

	corruptedConfigManager := NewConfigManager(kernel)
	require.Error(t, corruptedConfigManager.Load(ctx))

	// 4. Invalid validation config in database -> Load returns error
	const invalidValueSQLStatement = `UPDATE tasks.config SET value = '{"concurrency_limit":-10}' WHERE key = 'runtime'`
	_, execValueErr := kernel.DB().Exec(ctx, invalidValueSQLStatement)
	require.NoError(t, execValueErr)

	invalidConfigManager := NewConfigManager(kernel)
	require.Error(t, invalidConfigManager.Load(ctx))

	// 5. Canceled context causes query errors on Load and Set
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, configManager.Load(canceledCtx))
	require.Error(t, configManager.Set(canceledCtx, customConfig))
}
