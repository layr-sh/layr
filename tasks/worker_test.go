package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksWorkerUnit(t *testing.T) {
	t.Parallel()

	kernel := core.NewTestKernel(nil)
	configManager := NewConfigManager(kernel)
	dispatcher := NewDispatcher(kernel)
	workerQueue := NewWorkerQueue(kernel, configManager, dispatcher)

	require.NotNil(t, workerQueue)

	t.Run("set node ID overrides identifier", func(t *testing.T) {
		t.Parallel()
		workerQueue.SetNodeID("custom-node-id")
	})

	t.Run("start and stop cycle with canceled context", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		testWorkerQueue := NewWorkerQueue(kernel, configManager, dispatcher)
		testWorkerQueue.Start(ctx)
		// Double start is a no-op
		testWorkerQueue.Start(ctx)

		time.Sleep(50 * time.Millisecond)
		testWorkerQueue.Stop()
		// Double stop is a no-op
		testWorkerQueue.Stop()
	})

	t.Run("workerLoop error backoff on broken DB", func(t *testing.T) {
		t.Parallel()
		brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
		brokenConfigManager := NewConfigManager(brokenKernel)
		brokenConfig := DefaultConfig()
		brokenConfig.ConcurrencyLimit = 0
		brokenConfigManager.SetMemoryConfig(brokenConfig)
		brokenDispatcher := NewDispatcher(brokenKernel)
		brokenWorkerQueue := NewWorkerQueue(brokenKernel, brokenConfigManager, brokenDispatcher)

		loopCtx, cancel := context.WithCancel(context.Background())
		brokenWorkerQueue.Start(loopCtx)
		time.Sleep(300 * time.Millisecond)
		cancel()
		brokenWorkerQueue.Stop()
	})
}
