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
	"layr.sh/filestorage"
)

func TestImageFullLifecycleE2E(t *testing.T) {
	allMigrations := append(filestorage.Migrations, Migrations...)
	kernel, cleanup := core.SetupTestKernel(t, allMigrations)
	defer cleanup()

	ctx := context.Background()

	// Initialize File Storage and Image Services
	fileStorageService := filestorage.NewService(kernel)
	require.NoError(t, fileStorageService.Start(ctx))
	defer fileStorageService.Stop()

	imageService := NewService(kernel)
	require.NoError(t, imageService.Start(ctx))
	defer imageService.Stop()

	coreServer := core.NewServer(kernel)
	fileStorageService.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())
	imageService.RegisterRoutes(coreServer.BaseRouter(), coreServer.ControlPlaneRouter())

	// 1. Create Service Account with full file storage and image scopes
	createdServiceAccount, accountErr := kernel.ServiceAccountManager().Create(ctx, core.CreateServiceAccountInput{
		Name: "e2e-tester",
		Scopes: []string{
			core.ScopeFileStorageBucketRead,
			core.ScopeFileStorageBucketWrite,
			core.ScopeFileStorageObjectRead,
			core.ScopeFileStorageObjectWrite,
			core.ScopeImageConfigRead,
			core.ScopeImageConfigWrite,
			core.ScopeImagePresetRead,
			core.ScopeImagePresetWrite,
			core.ScopeImageSignWrite,
			core.ScopeImageStatsRead,
		},
	})
	require.NoError(t, accountErr)
	authKey := createdServiceAccount.SecretKey

	// 2. Control Plane: Create storage bucket for source assets
	createBucketPayload := `{"name":"e2e-gallery","backend":"database","is_public":true}`
	createBucketRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/file-storage/buckets", bytes.NewReader([]byte(createBucketPayload)))
	createBucketRequest.Header.Set("Content-Type", "application/json")
	createBucketRequest.Header.Set("X-Service-Account-Key", authKey)
	createBucketResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(createBucketResponseRecorder, createBucketRequest)
	require.Equal(t, http.StatusCreated, createBucketResponseRecorder.Code)

	// 3. Data Plane: Upload source PNG image to storage
	pngBytes := createTestImagePNG(200, 150)
	uploadRequest := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/file-storage/objects/e2e-gallery/photos/banner.png", bytes.NewReader(pngBytes))
	uploadRequest.Header.Set("Content-Type", "image/png")
	uploadRequest.Header.Set("X-Service-Account-Key", authKey)
	uploadResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(uploadResponseRecorder, uploadRequest)
	require.Equal(t, http.StatusCreated, uploadResponseRecorder.Code)

	// 4. Control Plane: Create transformation preset
	createPresetPayload := `{"name":"banner_card","processing_options":"rs:fill:100:75/f:webp/q:85"}`
	createPresetRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/image/presets", bytes.NewReader([]byte(createPresetPayload)))
	createPresetRequest.Header.Set("Content-Type", "application/json")
	createPresetRequest.Header.Set("X-Service-Account-Key", authKey)
	createPresetResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(createPresetResponseRecorder, createPresetRequest)
	require.Equal(t, http.StatusCreated, createPresetResponseRecorder.Code)

	var createdPreset Preset
	require.NoError(t, json.Unmarshal(createPresetResponseRecorder.Body.Bytes(), &createdPreset))

	// 5. Control Plane: Sign Image Transformation Path using preset
	transformationPath := "/pr:banner_card/plain/e2e-gallery/photos/banner.png@webp"
	signPayload := `{"path":"` + transformationPath + `"}`
	signRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/_/image/sign", bytes.NewReader([]byte(signPayload)))
	signRequest.Header.Set("Content-Type", "application/json")
	signRequest.Header.Set("X-Service-Account-Key", authKey)
	signResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(signResponseRecorder, signRequest)
	require.Equal(t, http.StatusOK, signResponseRecorder.Code)

	var signURLResponse SignURLResponse
	require.NoError(t, json.Unmarshal(signResponseRecorder.Body.Bytes(), &signURLResponse))
	require.NotEmpty(t, signURLResponse.Signature)

	publishableKey := kernel.CryptoKeyManager().DerivePublishableKey()

	// 6. Data Plane: Execute Dynamic Image Transformation
	transformRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, signURLResponse.URL, nil)
	transformRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	transformResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(transformResponseRecorder, transformRequest)
	require.Equal(t, http.StatusOK, transformResponseRecorder.Code)
	require.Equal(t, "image/webp", transformResponseRecorder.Header().Get("Content-Type"))

	eTag := transformResponseRecorder.Header().Get("ETag")
	require.NotEmpty(t, eTag)

	// 7. Data Plane: Test ETag 304 Cache Revalidation
	revalidateRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, signURLResponse.URL, nil)
	revalidateRequest.Header.Set("If-None-Match", eTag)
	revalidateRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	revalidateResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(revalidateResponseRecorder, revalidateRequest)
	require.Equal(t, http.StatusNotModified, revalidateResponseRecorder.Code)

	// 8. Data Plane: Inspect metadata via POST /v1/image/info
	infoProbeRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/image/info", bytes.NewReader(pngBytes))
	infoProbeRequest.Header.Set("X-Layr-Client-Publishable-Key", publishableKey)
	infoProbeResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(infoProbeResponseRecorder, infoProbeRequest)
	require.Equal(t, http.StatusOK, infoProbeResponseRecorder.Code)

	var getInfoResponse GetInfoResponse
	require.NoError(t, json.Unmarshal(infoProbeResponseRecorder.Body.Bytes(), &getInfoResponse))
	require.Equal(t, 200, getInfoResponse.Width)
	require.Equal(t, 150, getInfoResponse.Height)

	// 9. Control Plane: Query Telemetry Statistics
	statsRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/_/image/stats", nil)
	statsRequest.Header.Set("X-Service-Account-Key", authKey)
	statsResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(statsResponseRecorder, statsRequest)
	require.Equal(t, http.StatusOK, statsResponseRecorder.Code)

	var getStatsResponse GetStatsResponse
	require.NoError(t, json.Unmarshal(statsResponseRecorder.Body.Bytes(), &getStatsResponse))
	require.GreaterOrEqual(t, getStatsResponse.CacheHits, int64(1))

	// 10. Control Plane: Delete Preset
	deletePresetRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/_/image/presets/"+createdPreset.ID.String(), nil)
	deletePresetRequest.Header.Set("X-Service-Account-Key", authKey)
	deletePresetResponseRecorder := httptest.NewRecorder()
	coreServer.Handler().ServeHTTP(deletePresetResponseRecorder, deletePresetRequest)
	require.Equal(t, http.StatusNoContent, deletePresetResponseRecorder.Code)
}
