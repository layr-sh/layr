// Package function defines the serverless and edge function execution engine.
package function

import (
	"context"
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
	"uuid"
)

func TestFunctionServiceUnit(t *testing.T) {
	t.Parallel()

	t.Run("service factory registered and constructs service", func(t *testing.T) {
		t.Parallel()
		serviceFactory, ok := core.GetServiceFactory("function")
		require.True(t, ok)
		require.NotNil(t, serviceFactory)

		serviceRunner, createErr := serviceFactory(core.NewTestKernel(nil))
		require.NoError(t, createErr)
		require.NotNil(t, serviceRunner)

		constructedService, isService := serviceRunner.(*Service)
		require.True(t, isService)
		require.NotNil(t, constructedService.Kernel())
		require.NotNil(t, constructedService.ConfigManager())
		require.NotNil(t, constructedService.Engine())
		require.NotNil(t, constructedService.BaseHandler())
		require.NotNil(t, constructedService.ControlPlaneHandler())
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
}

func TestFunctionServiceLifecycleAndEventsUnit(t *testing.T) {
	// 1. Start with broken DB (configManager.Load fails)
	brokenKernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	brokenService := NewService(brokenKernel)
	require.Error(t, brokenService.Start(context.Background()))

	// 2. Start with failing engine runner
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	service := NewService(kernel)
	testRunner := &mockFailingRunner{startFail: true, stopFail: true}
	service.Engine().RegisterRunner(testRunner)

	startErr := service.Start(context.Background())
	require.Error(t, startErr)
	require.Contains(t, startErr.Error(), "failed to start execution engine")

	// 5. Stop with failing runner logs warning
	service.Stop()

	// 6. Start with failing active functions load logs warning but succeeds
	testRunner.startFail = false
	testRunner.stopFail = false
	_, _ = kernel.DB().Exec(context.Background(), "ALTER TABLE function.endpoints RENAME TO endpoints_temp")
	require.NoError(t, service.Start(context.Background()))
	service.Stop()
	_, _ = kernel.DB().Exec(context.Background(), "ALTER TABLE function.endpoints_temp RENAME TO endpoints")

	// 7. Start with active deployment containing workerd_runtime_config and deploy failure
	activeEndpoint := insertTestEndpoint(context.Background(), t, kernel, "startup-fn", true)
	const insertDeploymentSQL = `
		INSERT INTO function.deployments (endpoint_id, version, bundle_format, bundle_content, bundle_hash, bundle_files, environment_variables, workerd_runtime_config, status, created_at)
		VALUES ($1, 1, 'raw', 'export default {};', 'hash1', '{}', '{"ENV_VAR":"1"}', '{"compatibility_date":"2024-01-01"}', 'active', clock_timestamp())
		RETURNING id;
	`
	var activeDeploymentID uuid.UUID
	require.NoError(t, kernel.DB().QueryRow(context.Background(), insertDeploymentSQL, activeEndpoint.ID).Scan(&activeDeploymentID))
	_, updateEndpointErr := kernel.DB().Exec(context.Background(), "UPDATE function.endpoints SET active_deployment_id = $1 WHERE id = $2;", activeDeploymentID, activeEndpoint.ID)
	require.NoError(t, updateEndpointErr)

	testRunner.deployFail = true
	require.NoError(t, service.Start(context.Background()))
	service.Stop()

	testRunner.deployFail = false
	require.NoError(t, service.Start(context.Background()))
	service.Stop()
}
