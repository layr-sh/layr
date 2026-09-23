package image

import (
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageRouterUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)

	// 1. Register both public and control plane routes on fresh routers
	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	service.RegisterRoutes(baseRouter, controlPlaneRouter)

	// 4. Inspect generated OpenAPI specifications
	baseOpenAPISpec := baseRouter.OutputOpenAPISpec()
	require.NotNil(t, baseOpenAPISpec)
	require.NotNil(t, baseOpenAPISpec.Paths.Value("/v1/image/info"))
	require.NotNil(t, baseOpenAPISpec.Paths.Value("/v1/image/info/{signature}/{path...}"))
	require.NotNil(t, baseOpenAPISpec.Paths.Value("/v1/image/{signature}/{path...}"))

	controlPlaneOpenAPISpec := controlPlaneRouter.OutputOpenAPISpec()
	require.NotNil(t, controlPlaneOpenAPISpec)
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/image/config"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/image/presets"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/image/presets/{id}"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/image/sign"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/image/stats"))
}
