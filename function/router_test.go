// Package function defines the serverless and edge function execution engine.
package function

import (
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFunctionRouterUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)

	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	service.RegisterRoutes(baseRouter, controlPlaneRouter)

	baseOpenAPISpec := baseRouter.OutputOpenAPISpec()
	require.NotNil(t, baseOpenAPISpec)
	require.NotNil(t, baseOpenAPISpec.Paths.Value("/v1/function/{name}/{path...}"))
	require.NotNil(t, baseOpenAPISpec.Paths.Value("/v1/function/{name}"))

	controlPlaneOpenAPISpec := controlPlaneRouter.OutputOpenAPISpec()
	require.NotNil(t, controlPlaneOpenAPISpec)
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/config"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{id}"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{id}/deploy"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{id}/deployments"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{id}/rollback"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/executions"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{id}/executions"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/stats"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/domains"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/domains/{id}"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{id}/domains"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{id}/domains/{route_id}"))

	// Test nil router handling
	service.RegisterRoutes(nil, nil)
}
