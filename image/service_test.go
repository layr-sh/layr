package image

import (
	"context"
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageServiceUnit(t *testing.T) {
	t.Parallel()

	t.Run("service factory registered and constructs service", func(t *testing.T) {
		t.Parallel()
		serviceFactory, ok := core.GetServiceFactory("image")
		require.True(t, ok)
		require.NotNil(t, serviceFactory)

		serviceRunner, createErr := serviceFactory(core.NewTestKernel(nil))
		require.NoError(t, createErr)
		require.NotNil(t, serviceRunner)

		constructedService, isService := serviceRunner.(*Service)
		require.True(t, isService)
		require.NotNil(t, constructedService.BaseHandler())
		require.NotNil(t, constructedService.ControlPlaneHandler())
		require.NotNil(t, constructedService.ConfigManager())
		require.NotNil(t, constructedService.PresetManager())
		require.NotNil(t, constructedService.CacheManager())
		require.NotNil(t, constructedService.Engine())
		require.NotNil(t, constructedService.Fetcher())
		require.NotNil(t, constructedService.Kernel())
	})

	t.Run("start returns error when context is canceled", func(t *testing.T) {
		t.Parallel()
		service := NewService(core.NewTestKernel(nil))
		require.NotNil(t, service)

		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		startErr := service.Start(canceledCtx)
		require.Error(t, startErr)
		require.Contains(t, startErr.Error(), "context canceled before start")

		service.Stop()
	})

	t.Run("register routes with routers", func(t *testing.T) {
		t.Parallel()
		service := NewService(core.NewTestKernel(nil))
		require.NotNil(t, service)

		baseFuegoEngine := fuego.NewServer()
		controlPlaneFuegoEngine := fuego.NewServer()
		baseRouter := core.NewRouter(baseFuegoEngine)
		controlPlaneRouter := core.NewRouter(controlPlaneFuegoEngine)
		service.RegisterRoutes(baseRouter, controlPlaneRouter)
	})

	t.Run("start returns error with broken database", func(t *testing.T) {
		brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
		service := NewService(brokenKernel)
		require.NotNil(t, service)

		startErr := service.Start(context.Background())
		require.Error(t, startErr)
		service.Stop()
	})

	t.Run("start returns error when preset load fails", func(t *testing.T) {
		kernel, cleanup := core.SetupTestKernel(t, Migrations)
		defer cleanup()

		_, dropErr := kernel.DB().Exec(context.Background(), "DROP TABLE image.presets")
		require.NoError(t, dropErr)

		service := NewService(kernel)
		startErr := service.Start(context.Background())
		require.Error(t, startErr)
		require.Contains(t, startErr.Error(), "failed to load presets")
	})

	t.Run("start succeeds with valid database", func(t *testing.T) {
		kernel, cleanup := core.SetupTestKernel(t, Migrations)
		defer cleanup()

		service := NewService(kernel)
		startErr := service.Start(context.Background())
		require.NoError(t, startErr)
	})
}
