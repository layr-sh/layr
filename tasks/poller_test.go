package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksPollerUnit(t *testing.T) {
	t.Parallel()

	kernel := core.NewTestKernel(nil)
	configManager := NewConfigManager(kernel)
	jobPoller := NewJobPoller(kernel, configManager)
	require.NotNil(t, jobPoller)

	t.Run("start and stop cycle with canceled context", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		lifecycleJobPoller := NewJobPoller(kernel, configManager)
		lifecycleJobPoller.Start(ctx)
		// Double start is a no-op
		lifecycleJobPoller.Start(ctx)

		time.Sleep(50 * time.Millisecond)
		lifecycleJobPoller.Stop()
		// Double stop is a no-op
		lifecycleJobPoller.Stop()
	})

	t.Run("poll once returns error on broken db", func(t *testing.T) {
		t.Parallel()
		brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
		brokenConfigManager := NewConfigManager(brokenKernel)
		brokenJobPoller := NewJobPoller(brokenKernel, brokenConfigManager)

		count, err := brokenJobPoller.PollOnce(context.Background())
		require.Error(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("pollLoop executes timer tick with interval under 100ms and logs poll error", func(t *testing.T) {
		t.Parallel()
		brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
		brokenConfigManager := NewConfigManager(brokenKernel)
		brokenConfig := DefaultConfig()
		brokenConfig.PollIntervalMs = 10
		brokenConfigManager.SetMemoryConfig(brokenConfig)
		loopJobPoller := NewJobPoller(brokenKernel, brokenConfigManager)

		ctx, cancel := context.WithCancel(context.Background())
		loopJobPoller.Start(ctx)
		time.Sleep(150 * time.Millisecond)
		cancel()
		loopJobPoller.Stop()
	})
}
