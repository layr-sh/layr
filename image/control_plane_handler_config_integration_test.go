package image

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageControlPlaneHandlerConfigIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(ctx))
	defer service.Stop()

	coreServer := core.NewServer(kernel)
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "image-config-manager",
		Scopes: []string{
			core.ScopeImageConfigRead,
			core.ScopeImageConfigWrite,
		},
	})
	require.NoError(t, accountErr)
	authKey := serviceAccount.SecretKey

	t.Run("config control plane lifecycle", func(t *testing.T) {
		// 1. Get initial config
		getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/image/config", nil)
		getRequest.Header.Set("X-Service-Account-Key", authKey)
		getResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(getResponseRecorder, getRequest)
		require.Equal(t, http.StatusOK, getResponseRecorder.Code)

		var activeConfig Config
		require.NoError(t, json.Unmarshal(getResponseRecorder.Body.Bytes(), &activeConfig))
		require.Equal(t, DefaultQuality, activeConfig.DefaultQuality)

		// 2. Update config
		updatePayload := `{"default_quality":95,"max_src_resolution":40,"max_animation_frames":64,"allow_insecure":true}`
		putRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/image/config", bytes.NewReader([]byte(updatePayload)))
		putRequest.Header.Set("Content-Type", "application/json")
		putRequest.Header.Set("X-Service-Account-Key", authKey)
		putResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(putResponseRecorder, putRequest)
		require.Equal(t, http.StatusOK, putResponseRecorder.Code)

		var updatedConfig Config
		require.NoError(t, json.Unmarshal(putResponseRecorder.Body.Bytes(), &updatedConfig))
		require.Equal(t, 95, updatedConfig.DefaultQuality)
		require.True(t, updatedConfig.AllowInsecure)
	})
}
