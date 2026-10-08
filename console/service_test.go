package console

import (
	"context"
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestConsoleServiceUnit(t *testing.T) {
	t.Parallel()

	t.Run("service factory registered and constructs service", func(t *testing.T) {
		t.Parallel()
		serviceFactory, ok := core.GetServiceFactory("console")
		require.True(t, ok)
		require.NotNil(t, serviceFactory)

		serviceRunner, createErr := serviceFactory(core.NewTestKernel(nil))
		require.NoError(t, createErr)
		require.NotNil(t, serviceRunner)

		consoleService, isService := serviceRunner.(*Service)
		require.True(t, isService)
		require.NotNil(t, consoleService.Kernel())
		require.NotNil(t, consoleService.ConfigManager())
		require.NotNil(t, consoleService.ControlPlaneHandler())
		consoleService.Stop()
	})

	t.Run("start returns error when context is canceled", func(t *testing.T) {
		t.Parallel()
		consoleService := NewService(core.NewTestKernel(nil))
		require.NotNil(t, consoleService)

		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		startErr := consoleService.Start(canceledCtx)
		require.Error(t, startErr)
		require.Contains(t, startErr.Error(), "context canceled before start")

		consoleService.Stop()
	})

	t.Run("start returns error with broken database", func(t *testing.T) {
		t.Parallel()
		kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
		consoleService := NewService(kernel)
		require.NotNil(t, consoleService)

		startErr := consoleService.Start(context.Background())
		require.Error(t, startErr)

		consoleService.Stop()
	})

	t.Run("registers control plane routes successfully", func(t *testing.T) {
		t.Parallel()
		kernel := core.NewTestKernel(nil)
		consoleService := NewService(kernel)

		fuegoServer := fuego.NewServer()
		baseRouter := core.NewRouter(fuegoServer)
		controlPlaneRouter := core.NewRouter(fuegoServer)

		consoleService.RegisterRoutes(baseRouter, controlPlaneRouter)
	})
}
