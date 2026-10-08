package console

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestConsoleConfigUnit(t *testing.T) {
	t.Parallel()

	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	t.Run("default config validity", func(t *testing.T) {
		t.Parallel()
		defaultConfig := DefaultConfig()
		require.NoError(t, defaultConfig.Validate())
		require.Equal(t, DefaultSessionIdleTimeoutSeconds, defaultConfig.SessionIdleTimeoutSeconds)
		require.Equal(t, DefaultSessionMaxLifetimeSeconds, defaultConfig.SessionMaxLifetimeSeconds)
		require.False(t, defaultConfig.MFAEnforced)
		require.Empty(t, defaultConfig.AllowedCIDRs)
		require.Equal(t, DefaultAuditRetentionDays, defaultConfig.AuditRetentionDays)
	})

	t.Run("config validation failure paths", func(t *testing.T) {
		t.Parallel()

		zeroIdleConfig := DefaultConfig()
		zeroIdleConfig.SessionIdleTimeoutSeconds = 0
		require.Error(t, zeroIdleConfig.Validate())

		zeroLifetimeConfig := DefaultConfig()
		zeroLifetimeConfig.SessionMaxLifetimeSeconds = 0
		require.Error(t, zeroLifetimeConfig.Validate())

		idleExceedsLifetimeConfig := DefaultConfig()
		idleExceedsLifetimeConfig.SessionIdleTimeoutSeconds = 50000
		idleExceedsLifetimeConfig.SessionMaxLifetimeSeconds = 1000
		require.Error(t, idleExceedsLifetimeConfig.Validate())

		zeroAuditDaysConfig := DefaultConfig()
		zeroAuditDaysConfig.AuditRetentionDays = 0
		require.Error(t, zeroAuditDaysConfig.Validate())

		invalidCIDRConfig := DefaultConfig()
		invalidCIDRConfig.AllowedCIDRs = []string{"not-a-cidr"}
		require.Error(t, invalidCIDRConfig.Validate())

		validCIDRConfig := DefaultConfig()
		validCIDRConfig.AllowedCIDRs = []string{"192.168.1.0/24", "10.0.0.1"}
		require.NoError(t, validCIDRConfig.Validate())
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
				newConfig.SessionIdleTimeoutSeconds = 1800 + iteration
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
		invalidConfig.SessionIdleTimeoutSeconds = 0
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
