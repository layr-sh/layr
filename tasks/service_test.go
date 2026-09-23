package tasks

import (
	"context"
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksServiceUnit(t *testing.T) {
	t.Parallel()

	t.Run("service factory registered and constructs service", func(t *testing.T) {
		t.Parallel()
		serviceFactory, ok := core.GetServiceFactory("tasks")
		require.True(t, ok)
		require.NotNil(t, serviceFactory)

		serviceRunner, createErr := serviceFactory(core.NewTestKernel(nil))
		require.NoError(t, createErr)
		require.NotNil(t, serviceRunner)

		tasksService, isService := serviceRunner.(*Service)
		require.True(t, isService)
		require.NotNil(t, tasksService.Kernel())
		require.NotNil(t, tasksService.ConfigManager())
		require.NotNil(t, tasksService.JobManager())
		require.NotNil(t, tasksService.JobPoller())
		require.NotNil(t, tasksService.WorkerQueue())
		require.NotNil(t, tasksService.Dispatcher())
		require.NotNil(t, tasksService.ControlPlaneHandler())
	})

	t.Run("start returns error when context is canceled", func(t *testing.T) {
		t.Parallel()
		tasksService := NewService(core.NewTestKernel(nil))
		require.NotNil(t, tasksService)

		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		startErr := tasksService.Start(canceledCtx)
		require.Error(t, startErr)
		require.Contains(t, startErr.Error(), "context canceled before start")

		tasksService.Stop()
	})

	t.Run("start returns error with broken database", func(t *testing.T) {
		t.Parallel()
		kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
		tasksService := NewService(kernel)
		require.NotNil(t, tasksService)

		startErr := tasksService.Start(context.Background())
		require.Error(t, startErr)

		tasksService.Stop()
	})

	t.Run("registers control plane routes successfully", func(t *testing.T) {
		t.Parallel()
		kernel := core.NewTestKernel(nil)
		tasksService := NewService(kernel)

		fuegoServer := fuego.NewServer()
		baseRouter := core.NewRouter(fuegoServer)
		controlPlaneRouter := core.NewRouter(fuegoServer)

		tasksService.RegisterRoutes(baseRouter, controlPlaneRouter)
	})
}
