package console

import (
	"context"
	"sync"
	"testing"
	"time"

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

	// 2. Set custom configuration and verify roundtrip via Load and event emission
	var capturedEvents []core.Event
	var eventMutex sync.Mutex
	kernel.EventBus().Subscribe("console.config.updated", func(eventCtx context.Context, event core.Event) error {
		eventMutex.Lock()
		defer eventMutex.Unlock()
		capturedEvents = append(capturedEvents, event)
		return nil
	})

	customConfig := initialConfig
	customConfig.SessionIdleTimeoutSeconds = 3600
	customConfig.MFAEnforced = true
	customConfig.AllowedCIDRs = []string{"10.0.0.0/8"}
	require.NoError(t, configManager.Set(ctx, customConfig))

	time.Sleep(50 * time.Millisecond)
	eventMutex.Lock()
	require.Len(t, capturedEvents, 1)
	require.NotNil(t, capturedEvents[0].ResourceID)
	require.Equal(t, "console.config", *capturedEvents[0].ResourceID)
	eventMutex.Unlock()

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

	// 5. Canceled context causes query errors on Load and Set
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, freshConfigManager.Load(canceledCtx))
	require.Error(t, configManager.Set(canceledCtx, customConfig))
}
