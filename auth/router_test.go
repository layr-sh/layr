package auth

import (
	"testing"

	"github.com/go-fuego/fuego"
	"layr.sh/core"
)

func TestAuthRouterUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)

	// 1. Test nil safety
	service.RegisterRoutes(nil, nil)

	isolatedBaseRouter := core.NewRouter(fuego.NewServer())
	service.RegisterRoutes(isolatedBaseRouter, nil)

	isolatedControlPlaneRouter := core.NewRouter(fuego.NewServer())
	service.RegisterRoutes(nil, isolatedControlPlaneRouter)

	// 2. Test nil handler safety
	nilHandlerService := &Service{}
	nilHandlerBaseRouter := core.NewRouter(fuego.NewServer())
	nilHandlerControlPlaneRouter := core.NewRouter(fuego.NewServer())
	nilHandlerService.RegisterRoutes(nilHandlerBaseRouter, nilHandlerControlPlaneRouter)

	// 3. Register both public and control plane routes on fresh routers
	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	service.RegisterRoutes(baseRouter, controlPlaneRouter)

	// 4. Inspect generated OpenAPI specifications
	publicOpenAPISpec := baseRouter.OutputOpenAPISpec()
	if publicOpenAPISpec == nil {
		t.Fatal("expected non-nil public OpenAPI specification")
	}
	if publicOpenAPISpec.Paths.Value("/.well-known/openid-configuration") == nil {
		t.Fatal("expected /.well-known/openid-configuration route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/v1/auth/sign-in") == nil {
		t.Fatal("expected /v1/auth/sign-in route in public OpenAPI spec")
	}
	if publicOpenAPISpec.Paths.Value("/v1/auth/user") == nil {
		t.Fatal("expected /v1/auth/user route in public OpenAPI spec")
	}

	controlPlaneOpenAPISpec := controlPlaneRouter.OutputOpenAPISpec()
	if controlPlaneOpenAPISpec == nil {
		t.Fatal("expected non-nil control plane OpenAPI specification")
	}
	if controlPlaneOpenAPISpec.Paths.Value("/v1/_/auth/config") == nil {
		t.Fatal("expected /v1/_/auth/config route in control plane OpenAPI spec")
	}
	if controlPlaneOpenAPISpec.Paths.Value("/v1/_/auth/users") == nil {
		t.Fatal("expected /v1/_/auth/users route in control plane OpenAPI spec")
	}
}
