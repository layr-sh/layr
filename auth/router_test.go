package auth

import (
	"testing"

	"github.com/go-fuego/fuego"
	"layr.sh/core"
)

func TestAuthRouterUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)

	// 1. Register routes on routers
	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	service.RegisterRoutes(baseRouter, controlPlaneRouter)

	// 2. Verify SDK routes naming compliance (snake_case, operation ID namespacing)
	core.AssertRouterRoutesCompliance(t, baseRouter)
	core.AssertRouterRoutesCompliance(t, controlPlaneRouter)
}
