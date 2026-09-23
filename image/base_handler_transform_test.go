package image

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
	"layr.sh/filestorage"
)

func TestImageBaseHandlerTransformUnit(t *testing.T) {
	allMigrations := append(filestorage.Migrations, Migrations...)
	kernel, cleanup := core.SetupTestKernel(t, allMigrations)
	defer cleanup()

	service := NewService(kernel)
	baseHandler := service.BaseHandler()
	ctx := context.Background()

	signingKey := service.ConfigManager().SigningKey()
	signingSalt := service.ConfigManager().SigningSalt()

	pngBytes := createTestImagePNG(100, 100)

	// Seed public bucket and image object
	publicBucketID := uuid.New()
	const insertPublicBucketSQL = `
		INSERT INTO file_storage.buckets (id, name, is_public, backend, backend_config, created_at, last_updated_at)
		VALUES ($1, 'transform-public', true, 'database', '{}', clock_timestamp(), clock_timestamp());
	`
	_, err := kernel.DB().Exec(ctx, insertPublicBucketSQL, publicBucketID)
	require.NoError(t, err)

	publicObjectID := uuid.New()
	const insertPublicObjectSQL = `
		INSERT INTO file_storage.objects (id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at)
		VALUES ($1, $2, 'photo.png', 'image/png', $3, 'chk-photo', '{}', clock_timestamp(), clock_timestamp());
	`
	_, err = kernel.DB().Exec(ctx, insertPublicObjectSQL, publicObjectID, publicBucketID, len(pngBytes))
	require.NoError(t, err)

	const insertPublicChunkSQL = `
		INSERT INTO file_storage.chunks (object_id, chunk_index, chunk_data)
		VALUES ($1, 0, $2);
	`
	_, err = kernel.DB().Exec(ctx, insertPublicChunkSQL, publicObjectID, pngBytes)
	require.NoError(t, err)

	// Seed fallback image in public bucket
	fallbackObjectID := uuid.New()
	const insertFallbackObjectSQL = `
		INSERT INTO file_storage.objects (id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at)
		VALUES ($1, $2, 'fallback.png', 'image/png', $3, 'chk-fallback', '{}', clock_timestamp(), clock_timestamp());
	`
	_, err = kernel.DB().Exec(ctx, insertFallbackObjectSQL, fallbackObjectID, publicBucketID, len(pngBytes))
	require.NoError(t, err)
	_, err = kernel.DB().Exec(ctx, insertPublicChunkSQL, fallbackObjectID, pngBytes)
	require.NoError(t, err)

	// Seed corrupt image object in public bucket for 422 test
	corruptBytes := []byte("not-a-valid-image-binary")
	corruptObjectID := uuid.New()
	const insertCorruptObjectSQL = `
		INSERT INTO file_storage.objects (id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at)
		VALUES ($1, $2, 'corrupt.png', 'image/png', $3, 'chk-corrupt', '{}', clock_timestamp(), clock_timestamp());
	`
	_, err = kernel.DB().Exec(ctx, insertCorruptObjectSQL, corruptObjectID, publicBucketID, len(corruptBytes))
	require.NoError(t, err)
	_, err = kernel.DB().Exec(ctx, insertPublicChunkSQL, corruptObjectID, corruptBytes)
	require.NoError(t, err)

	// Seed private bucket for 403 test
	privateBucketID := uuid.New()
	const insertPrivateBucketSQL = `
		INSERT INTO file_storage.buckets (id, name, is_public, backend, backend_config, created_at, last_updated_at)
		VALUES ($1, 'transform-private', false, 'database', '{}', clock_timestamp(), clock_timestamp());
	`
	_, err = kernel.DB().Exec(ctx, insertPrivateBucketSQL, privateBucketID)
	require.NoError(t, err)

	privateObjectID := uuid.New()
	const insertPrivateObjectSQL = `
		INSERT INTO file_storage.objects (id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at)
		VALUES ($1, $2, 'secret.png', 'image/png', $3, 'chk-secret', '{}', clock_timestamp(), clock_timestamp());
	`
	_, err = kernel.DB().Exec(ctx, insertPrivateObjectSQL, privateObjectID, privateBucketID, len(pngBytes))
	require.NoError(t, err)
	_, err = kernel.DB().Exec(ctx, insertPublicChunkSQL, privateObjectID, pngBytes)
	require.NoError(t, err)

	t.Run("missing signature or path returns 400", func(t *testing.T) {
		noSignatureRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image//path", nil)
		noSignatureResponseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(noSignatureResponseRecorder, noSignatureRequest)
		require.Equal(t, http.StatusBadRequest, noSignatureResponseRecorder.Code)
	})

	t.Run("invalid signature returns 403", func(t *testing.T) {
		badSignatureRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/bad/rs:fill:50:50/plain/b/k", nil)
		badSignatureRequest.SetPathValue("signature", "bad")
		badSignatureRequest.SetPathValue("path", "rs:fill:50:50/plain/b/k")
		badSignatureResponseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(badSignatureResponseRecorder, badSignatureRequest)
		require.Equal(t, http.StatusForbidden, badSignatureResponseRecorder.Code)
	})

	t.Run("parse error in options returns 400", func(t *testing.T) {
		badPresetPath := "/pr:unknown_preset/plain/b/k.jpg"
		badPresetSig := SignPath(signingKey, signingSalt, badPresetPath)
		badPresetRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+badPresetSig+badPresetPath, nil)
		badPresetRequest.SetPathValue("signature", badPresetSig)
		badPresetRequest.SetPathValue("path", badPresetPath)
		badPresetResponseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(badPresetResponseRecorder, badPresetRequest)
		require.Equal(t, http.StatusBadRequest, badPresetResponseRecorder.Code)
	})

	t.Run("bad path structure returns 400", func(t *testing.T) {
		badPath := "/plain/"
		badPathSig := SignPath(signingKey, signingSalt, badPath)
		badPathRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+badPathSig+badPath, nil)
		badPathRequest.SetPathValue("signature", badPathSig)
		badPathRequest.SetPathValue("path", badPath)
		badPathResponseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(badPathResponseRecorder, badPathRequest)
		require.Equal(t, http.StatusBadRequest, badPathResponseRecorder.Code)
	})

	t.Run("cached response returned on cache hit and not modified on etag match", func(t *testing.T) {
		sourceURL := "local/cache-bucket/avatar.png"
		targetPath := "/rs:fill:64:64/plain/" + sourceURL
		sig := SignPath(signingKey, signingSalt, targetPath)

		// Pre-populate cache
		processingOptions, _ := ParseProcessingOptions("rs:fill:64:64", nil)
		cacheKey, optionsHash := ComputeCacheKey(sourceURL, processingOptions)
		cachedData := []byte("cached-image-bytes")
		eTag := service.CacheManager().Set(ctx, cacheKey, sourceURL, optionsHash, "image/png", cachedData)

		// Pre-populate cache for query parameter variant
		sourceURLWithQuery := sourceURL + "?v=42"
		cacheKeyWithQuery, optionsHashWithQuery := ComputeCacheKey(sourceURLWithQuery, processingOptions)
		service.CacheManager().Set(ctx, cacheKeyWithQuery, sourceURLWithQuery, optionsHashWithQuery, "image/png", cachedData)

		// 1. Request without If-None-Match -> 200 OK with cached bytes
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+sig+targetPath, nil)
		request.SetPathValue("signature", sig)
		request.SetPathValue("path", targetPath)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(responseRecorder, request)
		require.Equal(t, http.StatusOK, responseRecorder.Code)
		require.Equal(t, cachedData, responseRecorder.Body.Bytes())
		require.Equal(t, eTag, responseRecorder.Header().Get("ETag"))

		// 2. Request with If-None-Match matching eTag -> 304 Not Modified
		notModifiedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+sig+targetPath, nil)
		notModifiedRequest.SetPathValue("signature", sig)
		notModifiedRequest.SetPathValue("path", targetPath)
		notModifiedRequest.Header.Set("If-None-Match", eTag)
		notModifiedResponseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(notModifiedResponseRecorder, notModifiedRequest)
		require.Equal(t, http.StatusNotModified, notModifiedResponseRecorder.Code)

		// 3. Request with query parameter hits cache
		queryTargetPath := targetPath + "?v=42"
		querySig := SignPath(signingKey, signingSalt, queryTargetPath)
		queryRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+querySig+targetPath+"?v=42", nil)
		queryRequest.SetPathValue("signature", querySig)
		queryRequest.SetPathValue("path", targetPath)
		queryResponseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(queryResponseRecorder, queryRequest)
		require.Equal(t, http.StatusOK, queryResponseRecorder.Code)
	})

	t.Run("successful transformation with extension override and caching", func(t *testing.T) {
		transformPath := "/rs:fill:50:50/plain/transform-public/photo.png@webp"
		sig := SignPath(signingKey, signingSalt, transformPath)
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+sig+transformPath, nil)
		request.SetPathValue("signature", sig)
		request.SetPathValue("path", transformPath)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(responseRecorder, request)
		require.Equal(t, http.StatusOK, responseRecorder.Code)
		require.Equal(t, "image/webp", responseRecorder.Header().Get("Content-Type"))
		require.NotEmpty(t, responseRecorder.Header().Get("ETag"))
	})

	t.Run("fallback image fetched when source asset fails", func(t *testing.T) {
		encodedFallback := base64.RawURLEncoding.EncodeToString([]byte("transform-public/fallback.png"))
		fallbackPath := "/fiu:" + encodedFallback + "/plain/transform-public/missing-source.png"
		sig := SignPath(signingKey, signingSalt, fallbackPath)
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+sig+fallbackPath, nil)
		request.SetPathValue("signature", sig)
		request.SetPathValue("path", fallbackPath)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(responseRecorder, request)
		require.Equal(t, http.StatusOK, responseRecorder.Code)
		require.NotEmpty(t, responseRecorder.Body.Bytes())
	})

	t.Run("storage object not found returns 404", func(t *testing.T) {
		notFoundPath := "/rs:fill:20:20/plain/transform-public/does-not-exist.png"
		sig := SignPath(signingKey, signingSalt, notFoundPath)
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+sig+notFoundPath, nil)
		request.SetPathValue("signature", sig)
		request.SetPathValue("path", notFoundPath)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(responseRecorder, request)
		require.Equal(t, http.StatusNotFound, responseRecorder.Code)
	})

	t.Run("storage access denied returns 403", func(t *testing.T) {
		deniedPath := "/rs:fill:20:20/plain/transform-private/secret.png"
		sig := SignPath(signingKey, signingSalt, deniedPath)
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+sig+deniedPath, nil)
		request.SetPathValue("signature", sig)
		request.SetPathValue("path", deniedPath)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)
	})

	t.Run("ssrf blocked returns 403", func(t *testing.T) {
		ssrfPath := "/rs:fill:10:10/plain/http://127.0.0.1/bad.jpg"
		sig := SignPath(signingKey, signingSalt, ssrfPath)
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+sig+ssrfPath, nil)
		request.SetPathValue("signature", sig)
		request.SetPathValue("path", ssrfPath)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(responseRecorder, request)
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)
	})

	t.Run("unsupported protocol returns 502", func(t *testing.T) {
		ftpPath := "/rs:fill:10:10/plain/ftp://example.com/bad.jpg"
		sig := SignPath(signingKey, signingSalt, ftpPath)
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+ftpPath, nil)
		request.SetPathValue("signature", sig)
		request.SetPathValue("path", ftpPath)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(responseRecorder, request)
		require.Equal(t, http.StatusBadGateway, responseRecorder.Code)
	})

	t.Run("corrupt image transformation returns 422", func(t *testing.T) {
		corruptPath := "/rs:fill:50:50/plain/transform-public/corrupt.png"
		sig := SignPath(signingKey, signingSalt, corruptPath)
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/image/"+sig+corruptPath, nil)
		request.SetPathValue("signature", sig)
		request.SetPathValue("path", corruptPath)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleTransform(responseRecorder, request)
		require.Equal(t, http.StatusUnprocessableEntity, responseRecorder.Code)
	})
}
