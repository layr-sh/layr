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

func TestImageControlPlaneHandlerPresetIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	require.NoError(t, service.Start(ctx))
	defer service.Stop()

	coreServer := core.NewServer(kernel)
	service.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	serviceAccount, accountErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "image-preset-manager",
		Scopes: []string{
			core.ScopeImagePresetRead,
			core.ScopeImagePresetWrite,
		},
	})
	require.NoError(t, accountErr)
	authKey := serviceAccount.SecretKey

	t.Run("presets control plane CRUD lifecycle", func(t *testing.T) {
		// 1. Create preset
		createPayload := `{"name":"profile_thumbnail","processing_options":"rs:fill:128:128/q:85"}`
		createRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/image/presets", bytes.NewReader([]byte(createPayload)))
		createRequest.Header.Set("Content-Type", "application/json")
		createRequest.Header.Set("X-Service-Account-Key", authKey)
		createResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(createResponseRecorder, createRequest)
		require.Equal(t, http.StatusCreated, createResponseRecorder.Code)

		var createdPreset Preset
		require.NoError(t, json.Unmarshal(createResponseRecorder.Body.Bytes(), &createdPreset))
		require.Equal(t, "profile_thumbnail", createdPreset.Name)

		// 2. List presets
		listRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/image/presets", nil)
		listRequest.Header.Set("X-Service-Account-Key", authKey)
		listResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(listResponseRecorder, listRequest)
		require.Equal(t, http.StatusOK, listResponseRecorder.Code)

		var listPresetsResponse ListPresetsResponse
		require.NoError(t, json.Unmarshal(listResponseRecorder.Body.Bytes(), &listPresetsResponse))
		require.GreaterOrEqual(t, listPresetsResponse.Count, 1)

		// 3. Get preset by ID
		presetIDString := createdPreset.ID.String()
		getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/image/presets/"+presetIDString, nil)
		getRequest.Header.Set("X-Service-Account-Key", authKey)
		getResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(getResponseRecorder, getRequest)
		require.Equal(t, http.StatusOK, getResponseRecorder.Code)

		// 4. Update preset
		updatePayload := `{"processing_options":"rs:fill:256:256/q:90"}`
		updateRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/_/image/presets/"+presetIDString, bytes.NewReader([]byte(updatePayload)))
		updateRequest.Header.Set("Content-Type", "application/json")
		updateRequest.Header.Set("X-Service-Account-Key", authKey)
		updateResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(updateResponseRecorder, updateRequest)
		require.Equal(t, http.StatusOK, updateResponseRecorder.Code)

		// 5. Delete preset
		deleteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/image/presets/"+presetIDString, nil)
		deleteRequest.Header.Set("X-Service-Account-Key", authKey)
		deleteResponseRecorder := httptest.NewRecorder()
		coreServer.Handler().ServeHTTP(deleteResponseRecorder, deleteRequest)
		require.Equal(t, http.StatusNoContent, deleteResponseRecorder.Code)
	})
}
