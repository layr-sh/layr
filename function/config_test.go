package function

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionConfigUnit(t *testing.T) {
	t.Parallel()

	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	t.Run("default config validity", func(t *testing.T) {
		t.Parallel()
		defaultConfig := DefaultConfig()
		require.NoError(t, defaultConfig.Validate())
		require.Equal(t, DefaultRuntime, defaultConfig.DefaultRuntime)
		require.Equal(t, DefaultMemoryLimitMB, defaultConfig.DefaultMemoryLimitMB)
		require.Equal(t, DefaultTimeoutSeconds, defaultConfig.DefaultTimeoutSeconds)
		require.Equal(t, int64(DefaultMaxBundleSizeBytes), defaultConfig.MaxBundleSizeBytes)
		require.Equal(t, DefaultWorkerdCompatibilityDate, defaultConfig.WorkerdRuntime.CompatibilityDate)
		require.Equal(t, DefaultWorkerdCompatibilityFlags, defaultConfig.WorkerdRuntime.CompatibilityFlags)
	})

	t.Run("config validation failure paths", func(t *testing.T) {
		t.Parallel()
		emptyRuntimeConfig := DefaultConfig()
		emptyRuntimeConfig.DefaultRuntime = ""
		require.Error(t, emptyRuntimeConfig.Validate())

		zeroMemoryConfig := DefaultConfig()
		zeroMemoryConfig.DefaultMemoryLimitMB = 0
		require.Error(t, zeroMemoryConfig.Validate())

		zeroTimeoutConfig := DefaultConfig()
		zeroTimeoutConfig.DefaultTimeoutSeconds = 0
		require.Error(t, zeroTimeoutConfig.Validate())

		zeroBundleSizeConfig := DefaultConfig()
		zeroBundleSizeConfig.MaxBundleSizeBytes = 0
		require.Error(t, zeroBundleSizeConfig.Validate())

		emptyWorkerdDateConfig := DefaultConfig()
		emptyWorkerdDateConfig.WorkerdRuntime.CompatibilityDate = ""
		require.Error(t, emptyWorkerdDateConfig.Validate())
	})

	t.Run("config manager in-memory concurrency", func(t *testing.T) {
		t.Parallel()
		configManager := NewConfigManager(kernel)
		require.Equal(t, DefaultConfig(), configManager.Get())

		var waitGroup sync.WaitGroup
		for count := 0; count < 20; count++ {
			waitGroup.Add(2)
			go func(iteration int) {
				defer waitGroup.Done()
				newConfig := DefaultConfig()
				newConfig.DefaultMemoryLimitMB = 128 + iteration
				configManager.SetMemoryConfig(newConfig)
			}(count)
			go func() {
				defer waitGroup.Done()
				_ = configManager.Get()
			}()
		}
		waitGroup.Wait()
	})

	t.Run("set invalid config returns ErrInvalidConfig", func(t *testing.T) {
		t.Parallel()
		configManager := NewConfigManager(kernel)
		invalidConfig := DefaultConfig()
		invalidConfig.DefaultRuntime = ""
		err := configManager.Set(context.Background(), invalidConfig)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrInvalidConfig))
	})

	t.Run("database failure on load and set", func(t *testing.T) {
		t.Parallel()
		brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)

		configManager := NewConfigManager(brokenKernel)
		require.Error(t, configManager.Load(context.Background()))
		require.Error(t, configManager.Set(context.Background(), DefaultConfig()))
	})
}
