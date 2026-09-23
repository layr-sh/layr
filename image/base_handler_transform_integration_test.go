package image

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
	"layr.sh/filestorage"
	"uuid"
)

func TestImageBaseHandlerTransformIntegration(t *testing.T) {
	allMigrations := append(filestorage.Migrations, Migrations...)
	kernel, cleanup := core.SetupTestKernel(t, allMigrations)
	defer cleanup()

	service := NewService(kernel)
	ctx := context.Background()
	require.NoError(t, service.Start(ctx))
	defer service.Stop()

	baseHandler := service.BaseHandler()
	pngBytes := createTestImagePNG(100, 100)

	// 1. Seed File Storage Database with a public bucket, object, and chunk
	bucketID := uuid.New()
	const insertBucketSQL = `
		INSERT INTO file_storage.buckets (id, name, is_public, backend, backend_config, created_at, last_updated_at)
		VALUES ($1, 'test-assets', true, 'database', '{}', clock_timestamp(), clock_timestamp());
	`
	_, bucketExecErr := kernel.DB().Exec(ctx, insertBucketSQL, bucketID)
	require.NoError(t, bucketExecErr)

	objectID := uuid.New()
	const insertObjectSQL = `
		INSERT INTO file_storage.objects (id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at)
		VALUES ($1, $2, 'avatar.png', 'image/png', $3, 'checksum123', '{}', clock_timestamp(), clock_timestamp());
	`
	_, objectExecErr := kernel.DB().Exec(ctx, insertObjectSQL, objectID, bucketID, len(pngBytes))
	require.NoError(t, objectExecErr)

	const insertChunkSQL = `
		INSERT INTO file_storage.chunks (object_id, chunk_index, chunk_data)
		VALUES ($1, 0, $2);
	`
	_, chunkExecErr := kernel.DB().Exec(ctx, insertChunkSQL, objectID, pngBytes)
	require.NoError(t, chunkExecErr)

	// 2. Generate cryptographic signature
	signingKey := service.ConfigManager().SigningKey()
	signingSalt := service.ConfigManager().SigningSalt()

	targetPath := "/rs:fill:40:40/plain/test-assets/avatar.png@webp"
	signature := SignPath(signingKey, signingSalt, targetPath)

	t.Run("transform asset from storage and verify webp and etag", func(t *testing.T) {
		transformRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+signature+targetPath, nil)
		transformRequest.SetPathValue("signature", signature)
		transformRequest.SetPathValue("path", targetPath)
		transformResponseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(transformResponseRecorder, transformRequest)

		require.Equal(t, http.StatusOK, transformResponseRecorder.Code)
		require.Equal(t, "image/webp", transformResponseRecorder.Header().Get("Content-Type"))
		eTag := transformResponseRecorder.Header().Get("ETag")
		require.NotEmpty(t, eTag)

		// Verify If-None-Match 304 response
		cachedTransformRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+signature+targetPath, nil)
		cachedTransformRequest.SetPathValue("signature", signature)
		cachedTransformRequest.SetPathValue("path", targetPath)
		cachedTransformRequest.Header.Set("If-None-Match", eTag)
		cachedTransformResponseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(cachedTransformResponseRecorder, cachedTransformRequest)
		require.Equal(t, http.StatusNotModified, cachedTransformResponseRecorder.Code)
	})

	t.Run("transform with database preset", func(t *testing.T) {
		presetID := uuid.New()
		const insertPresetSQL = `
			INSERT INTO image.presets (id, name, processing_options, created_at, updated_at)
			VALUES ($1, 'avatar_thumb', 'rs:fill:32:32/q:80', clock_timestamp(), clock_timestamp());
		`
		_, insertPresetErr := kernel.DB().Exec(ctx, insertPresetSQL, presetID)
		require.NoError(t, insertPresetErr)

		// Invalidate preset memory cache so it picks up the newly inserted preset
		require.NoError(t, service.PresetManager().Load(ctx))

		presetPath := "/pr:avatar_thumb/plain/test-assets/avatar.png@webp"
		presetSignature := SignPath(signingKey, signingSalt, presetPath)

		presetRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+presetSignature+presetPath, nil)
		presetRequest.SetPathValue("signature", presetSignature)
		presetRequest.SetPathValue("path", presetPath)
		presetResponseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(presetResponseRecorder, presetRequest)

		require.Equal(t, http.StatusOK, presetResponseRecorder.Code)
		require.Equal(t, "image/webp", presetResponseRecorder.Header().Get("Content-Type"))
	})

	t.Run("transform not found error from storage", func(t *testing.T) {
		notFoundPath := "/rs:fill:20:20/plain/test-assets/missing.png"
		notFoundSignature := SignPath(signingKey, signingSalt, notFoundPath)

		notFoundRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+notFoundSignature+notFoundPath, nil)
		notFoundRequest.SetPathValue("signature", notFoundSignature)
		notFoundRequest.SetPathValue("path", notFoundPath)
		notFoundResponseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(notFoundResponseRecorder, notFoundRequest)

		require.Equal(t, http.StatusNotFound, notFoundResponseRecorder.Code)
	})
}
