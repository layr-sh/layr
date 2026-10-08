package console

import (
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestConsoleRouterUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)

	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	service.RegisterRoutes(baseRouter, controlPlaneRouter)

	controlPlaneOpenAPISpec := controlPlaneRouter.OutputOpenAPISpec()
	require.NotNil(t, controlPlaneOpenAPISpec)
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/console/config"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/console/sign-in"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/console/sign-out"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/console/session"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/console/mfa/setup"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/console/mfa/verify"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/console/mfa"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/console/users"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/console/users/{user_id}"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/console/users/{user_id}/password"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/console/users/{user_id}/sessions"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/console/users/{user_id}/sessions/{session_id}"))
	require.NotNil(t, controlPlaneOpenAPISpec.Paths.Value("/v1/_/console/users/{user_id}/sessions/revoke"))

	// Test nil controlPlaneRouter branch
	service.RegisterRoutes(nil, nil)
}
