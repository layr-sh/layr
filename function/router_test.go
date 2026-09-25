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
	require.NotNil(t, baseOpenAPISpec.Paths.Value("/v1/function/{endpoint_name}/{path...}"))
	require.NotNil(t, baseOpenAPISpec.Paths.Value("/v1/function/{endpoint_name}"))

	controlPlaneOpenAPISpec := controlPlaneRouter.OutputOpenAPISpec()
	require.NotNil(t, controlPlaneOpenAPISpec)
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/config"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{endpoint_id}"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{endpoint_id}/deploy"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{endpoint_id}/deployments"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{endpoint_id}/rollback"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/executions"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{endpoint_id}/executions"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/stats"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/domains"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/domains/{domain_id}"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{endpoint_id}/domains"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/function/endpoints/{endpoint_id}/domains/{route_id}"))

	// Test nil router handling
	service.RegisterRoutes(nil, nil)
}
