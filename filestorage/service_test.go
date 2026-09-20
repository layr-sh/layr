package filestorage

import (
	"context"
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFilestorageServiceUnit(t *testing.T) {
	t.Parallel()

	t.Run("service factory registered and constructs service", func(t *testing.T) {
		t.Parallel()
		serviceFactory, ok := core.GetServiceFactory("filestorage")
		require.True(t, ok)
		require.NotNil(t, serviceFactory)

		serviceRunner, createErr := serviceFactory(&core.Kernel{})
		require.NoError(t, createErr)
		require.NotNil(t, serviceRunner)

		constructedService, isService := serviceRunner.(*Service)
		require.True(t, isService)
		require.NotNil(t, constructedService.BaseHandler())
		require.NotNil(t, constructedService.ControlPlaneHandler())
		require.NotNil(t, constructedService.ConfigManager())
	})

	t.Run("file_storage alias factory registered", func(t *testing.T) {
		t.Parallel()
		serviceFactory, ok := core.GetServiceFactory("file_storage")
		require.True(t, ok)
		require.NotNil(t, serviceFactory)
	})

	t.Run("start returns error when context is canceled", func(t *testing.T) {
		t.Parallel()
		service := NewService(nil, nil)
		require.NotNil(t, service)

		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		startErr := service.Start(canceledCtx)
		require.Error(t, startErr)
		require.Contains(t, startErr.Error(), "context canceled before start")

		stopErr := service.Stop()
		require.NoError(t, stopErr)
	})

	t.Run("start succeeds without database connection", func(t *testing.T) {
		t.Parallel()
		service := NewService(nil, nil)
		require.NotNil(t, service)

		service.SetServiceAccountManager(nil)
		service.SetEventBus(nil)
		service.SetKVStore(nil)

		startErr := service.Start(context.Background())
		require.NoError(t, startErr)
	})

	t.Run("register routes with routers", func(t *testing.T) {
		t.Parallel()
		service := NewService(nil, nil)
		require.NotNil(t, service)

		service.RegisterRoutes(nil, nil)

		baseFuegoEngine := fuego.NewServer()
		controlPlaneFuegoEngine := fuego.NewServer()
		baseRouter := core.NewRouter(baseFuegoEngine)
		controlPlaneRouter := core.NewRouter(controlPlaneFuegoEngine)
		service.RegisterRoutes(baseRouter, controlPlaneRouter)
	})
}
