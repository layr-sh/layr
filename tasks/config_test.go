package tasks

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksConfigUnit(t *testing.T) {
	t.Parallel()

	t.Run("default config has expected operational defaults", func(t *testing.T) {
		t.Parallel()
		tasksConfig := DefaultConfig()
		require.Equal(t, 25, tasksConfig.ConcurrencyLimit)
		require.Equal(t, 30, tasksConfig.TimeoutSeconds)
		require.Equal(t, 1000, tasksConfig.PollIntervalMs)
		require.Equal(t, 5, tasksConfig.RetryInitialDelaySeconds)
		require.Equal(t, 300, tasksConfig.RetryMaxDelaySeconds)
		require.Equal(t, 5, tasksConfig.RetryMaxAttempts)
		require.NoError(t, tasksConfig.Validate())
	})

	t.Run("validate rejects invalid concurrency limits", func(t *testing.T) {
		t.Parallel()
		tasksConfig := DefaultConfig()
		tasksConfig.ConcurrencyLimit = 0
		require.Error(t, tasksConfig.Validate())

		tasksConfig.ConcurrencyLimit = 1001
		require.Error(t, tasksConfig.Validate())
	})

	t.Run("validate rejects invalid timeout limits", func(t *testing.T) {
		t.Parallel()
		tasksConfig := DefaultConfig()
		tasksConfig.TimeoutSeconds = 0
		require.Error(t, tasksConfig.Validate())

		tasksConfig.TimeoutSeconds = 3601
		require.Error(t, tasksConfig.Validate())
	})

	t.Run("validate rejects invalid poll intervals", func(t *testing.T) {
		t.Parallel()
		tasksConfig := DefaultConfig()
		tasksConfig.PollIntervalMs = 99
		require.Error(t, tasksConfig.Validate())

		tasksConfig.PollIntervalMs = 60001
		require.Error(t, tasksConfig.Validate())
	})

	t.Run("validate rejects invalid initial delay seconds", func(t *testing.T) {
		t.Parallel()
		tasksConfig := DefaultConfig()
		tasksConfig.RetryInitialDelaySeconds = 0
		require.Error(t, tasksConfig.Validate())

		tasksConfig.RetryInitialDelaySeconds = 3601
		require.Error(t, tasksConfig.Validate())
	})

	t.Run("validate rejects invalid max delay seconds", func(t *testing.T) {
		t.Parallel()
		tasksConfig := DefaultConfig()
		tasksConfig.RetryMaxDelaySeconds = tasksConfig.RetryInitialDelaySeconds - 1
		require.Error(t, tasksConfig.Validate())

		tasksConfig.RetryMaxDelaySeconds = 86401
		require.Error(t, tasksConfig.Validate())
	})

	t.Run("validate rejects invalid retry max attempts", func(t *testing.T) {
		t.Parallel()
		tasksConfig := DefaultConfig()
		tasksConfig.RetryMaxAttempts = 0
		require.Error(t, tasksConfig.Validate())

		tasksConfig.RetryMaxAttempts = 21
		require.Error(t, tasksConfig.Validate())
	})

	t.Run("config manager gets and sets in memory", func(t *testing.T) {
		t.Parallel()
		kernel := core.NewTestKernel(nil)
		configManager := NewConfigManager(kernel)
		require.NotNil(t, configManager)

		initialConfig := configManager.Get()
		require.Equal(t, 25, initialConfig.ConcurrencyLimit)

		updatedConfig := initialConfig
		updatedConfig.ConcurrencyLimit = 50
		configManager.SetMemoryConfig(updatedConfig)

		retrievedConfig := configManager.Get()
		require.Equal(t, 50, retrievedConfig.ConcurrencyLimit)
	})

	t.Run("set rejects invalid config before database write", func(t *testing.T) {
		t.Parallel()
		kernel := core.NewTestKernel(nil)
		configManager := NewConfigManager(kernel)

		invalidConfig := DefaultConfig()
		invalidConfig.ConcurrencyLimit = -1
		err := configManager.Set(context.Background(), invalidConfig)
		require.Error(t, err)
		require.Contains(t, err.Error(), "concurrency_limit must be between")
	})

	t.Run("load and set return error with broken database", func(t *testing.T) {
		t.Parallel()
		kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
		configManager := NewConfigManager(kernel)

		loadErr := configManager.Load(context.Background())
		require.Error(t, loadErr)

		setErr := configManager.Set(context.Background(), DefaultConfig())
		require.Error(t, setErr)
	})
}

func TestTasksConfigEventAndContractUnit(t *testing.T) {
	require.Equal(t, "runtime", ConfigKey)

	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	configManager := NewConfigManager(kernel)
	ctx := context.Background()

	var capturedEvents []core.Event
	var eventMutex sync.Mutex
	kernel.EventBus().Subscribe("tasks.config.updated", func(eventCtx context.Context, event core.Event) error {
		eventMutex.Lock()
		defer eventMutex.Unlock()
		capturedEvents = append(capturedEvents, event)
		return nil
	})

	// Invalid config returns ErrInvalidConfig
	invalidConfig := DefaultConfig()
	invalidConfig.ConcurrencyLimit = 0
	setErr := configManager.Set(ctx, invalidConfig)
	require.Error(t, setErr)
	require.True(t, errors.Is(setErr, ErrInvalidConfig))

	// Valid config publishes event
	validConfig := DefaultConfig()
	validConfig.ConcurrencyLimit = 30
	require.NoError(t, configManager.Set(ctx, validConfig))

	time.Sleep(50 * time.Millisecond)
	eventMutex.Lock()
	require.Len(t, capturedEvents, 1)
	require.NotNil(t, capturedEvents[0].ResourceID)
	require.Equal(t, "tasks.config", *capturedEvents[0].ResourceID)
	eventMutex.Unlock()
}
