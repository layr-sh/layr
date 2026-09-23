package image

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
	"layr.sh/filestorage"
	"uuid"
)

func TestImageBaseHandlerInfoIntegration(t *testing.T) {
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
		VALUES ($1, 'info-assets', true, 'database', '{}', clock_timestamp(), clock_timestamp());
	`
	_, bucketExecErr := kernel.DB().Exec(ctx, insertBucketSQL, bucketID)
	require.NoError(t, bucketExecErr)

	objectID := uuid.New()
	const insertObjectSQL = `
		INSERT INTO file_storage.objects (id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at)
		VALUES ($1, $2, 'sample.png', 'image/png', $3, 'checksum-sample', '{}', clock_timestamp(), clock_timestamp());
	`
	_, objectExecErr := kernel.DB().Exec(ctx, insertObjectSQL, objectID, bucketID, len(pngBytes))
	require.NoError(t, objectExecErr)

	const insertChunkSQL = `
		INSERT INTO file_storage.chunks (object_id, chunk_index, chunk_data)
		VALUES ($1, 0, $2);
	`
	_, chunkExecErr := kernel.DB().Exec(ctx, insertChunkSQL, objectID, pngBytes)
	require.NoError(t, chunkExecErr)

	signingKey := service.ConfigManager().SigningKey()
	signingSalt := service.ConfigManager().SigningSalt()

	t.Run("info inspects storage asset successfully", func(t *testing.T) {
		infoPath := "/plain/info-assets/sample.png"
		infoSignature := SignPath(signingKey, signingSalt, infoPath)

		infoRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/info/"+infoSignature+infoPath, nil)
		infoRequest.SetPathValue("signature", infoSignature)
		infoRequest.SetPathValue("path", infoPath)
		infoResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInfo(infoResponseRecorder, infoRequest)

		require.Equal(t, http.StatusOK, infoResponseRecorder.Code)
		var info Info
		require.NoError(t, json.Unmarshal(infoResponseRecorder.Body.Bytes(), &info))
		require.Equal(t, 100, info.Width)
		require.Equal(t, 100, info.Height)
		require.Equal(t, FormatPNG, info.Format)
	})

	t.Run("info returns 404 for missing storage asset", func(t *testing.T) {
		missingPath := "/plain/info-assets/nonexistent.png"
		missingSignature := SignPath(signingKey, signingSalt, missingPath)

		missingRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/info/"+missingSignature+missingPath, nil)
		missingRequest.SetPathValue("signature", missingSignature)
		missingRequest.SetPathValue("path", missingPath)
		missingResponseRecorder := httptest.NewRecorder()
		baseHandler.handleInfo(missingResponseRecorder, missingRequest)

		require.Equal(t, http.StatusNotFound, missingResponseRecorder.Code)
	})
}
