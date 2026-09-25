// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestTasksRouterUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)

	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	service.RegisterRoutes(baseRouter, controlPlaneRouter)

	controlPlaneOpenAPISpec := controlPlaneRouter.OutputOpenAPISpec()
	require.NotNil(t, controlPlaneOpenAPISpec)
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/tasks/jobs"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/tasks/jobs/{job_id}"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/tasks/executions"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/tasks/dlq"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/tasks/dlq/{execution_id}/retry"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/tasks/dlq/{execution_id}"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/tasks/config"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/tasks/stats"))
}
