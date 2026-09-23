package image

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
	"layr.sh/filestorage"
)

func TestImageFetcherPostgresIntegration(t *testing.T) {
	allMigrations := append(filestorage.Migrations, Migrations...)
	kernel, cleanup := core.SetupTestKernel(t, allMigrations)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(kernel)
	fetcher := NewFetcher(kernel, configManager)

	pngBytes := createTestImagePNG(80, 80)

	// 1. Seed public bucket with object and chunks
	publicBucketID := uuid.New()
	const insertPublicBucketSQL = `
		INSERT INTO file_storage.buckets (id, name, is_public, backend, backend_config, created_at, last_updated_at)
		VALUES ($1, 'public-photos', true, 'database', '{}', clock_timestamp(), clock_timestamp());
	`
	_, bucketErr := kernel.DB().Exec(ctx, insertPublicBucketSQL, publicBucketID)
	require.NoError(t, bucketErr)

	publicObjectID := uuid.New()
	const insertPublicObjectSQL = `
		INSERT INTO file_storage.objects (id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at)
		VALUES ($1, $2, 'avatar.png', 'image/png', $3, 'chk-avatar', '{}', clock_timestamp(), clock_timestamp());
	`
	_, objectErr := kernel.DB().Exec(ctx, insertPublicObjectSQL, publicObjectID, publicBucketID, len(pngBytes))
	require.NoError(t, objectErr)

	const insertPublicChunkSQL = `
		INSERT INTO file_storage.chunks (object_id, chunk_index, chunk_data)
		VALUES ($1, 0, $2);
	`
	_, chunkErr := kernel.DB().Exec(ctx, insertPublicChunkSQL, publicObjectID, pngBytes)
	require.NoError(t, chunkErr)

	// Fetch from public bucket
	publicRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	readCloser, length, contentType, fetchErr := fetcher.Fetch(ctx, "public-photos/avatar.png", publicRequest)
	require.NoError(t, fetchErr)
	require.Equal(t, "image/png", contentType)
	require.Equal(t, int64(len(pngBytes)), length)
	data, _ := io.ReadAll(readCloser)
	_ = readCloser.Close()
	require.Equal(t, pngBytes, data)

	// 2. Seed private bucket
	privateBucketID := uuid.New()
	const insertPrivateBucketSQL = `
		INSERT INTO file_storage.buckets (id, name, is_public, backend, backend_config, created_at, last_updated_at)
		VALUES ($1, 'vault-photos', false, 'database', '{}', clock_timestamp(), clock_timestamp());
	`
	_, privateBucketErr := kernel.DB().Exec(ctx, insertPrivateBucketSQL, privateBucketID)
	require.NoError(t, privateBucketErr)

	privateObjectID := uuid.New()
	const insertPrivateObjectSQL = `
		INSERT INTO file_storage.objects (id, bucket_id, object_key, content_type, size_bytes, checksum_sha256, metadata, created_at, last_updated_at)
		VALUES ($1, $2, 'secret.png', 'image/png', $3, 'chk-secret', '{}', clock_timestamp(), clock_timestamp());
	`
	_, privateObjectErr := kernel.DB().Exec(ctx, insertPrivateObjectSQL, privateObjectID, privateBucketID, len(pngBytes))
	require.NoError(t, privateObjectErr)

	const insertPrivateChunkSQL = `
		INSERT INTO file_storage.chunks (object_id, chunk_index, chunk_data)
		VALUES ($1, 0, $2);
	`
	_, privateChunkErr := kernel.DB().Exec(ctx, insertPrivateChunkSQL, privateObjectID, pngBytes)
	require.NoError(t, privateChunkErr)

	// Unauthorized access to private bucket -> ErrStorageAccessDenied
	unauthorizedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
	_, _, _, accessDeniedErr := fetcher.Fetch(ctx, "vault-photos/secret.png", unauthorizedRequest)
	require.Error(t, accessDeniedErr)
	require.ErrorIs(t, accessDeniedErr, ErrStorageAccessDenied)

	// Authorized access with valid presigned token
	expiresUnix := time.Now().Add(10 * time.Minute).Unix()
	presignKey := kernel.CryptoKeyManager().DeriveSubkey("layr-presign-signing-key")
	payload := fmt.Sprintf("vault-photos:secret.png:read:%d", expiresUnix)
	hmacHash := hmac.New(sha256.New, presignKey)
	hmacHash.Write([]byte(payload))
	presignToken := hex.EncodeToString(hmacHash.Sum(nil))

	authorizedURL := fmt.Sprintf("vault-photos/secret.png?token=%s&expires=%d&op=read", presignToken, expiresUnix)
	authorizedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test?"+authorizedURL, nil)
	authReadCloser, authLen, authType, authErr := fetcher.Fetch(ctx, authorizedURL, authorizedRequest)
	require.NoError(t, authErr)
	require.Equal(t, "image/png", authType)
	require.Equal(t, int64(len(pngBytes)), authLen)
	_ = authReadCloser.Close()

	// Nonexistent object in public bucket -> ErrStorageNotFound
	_, _, _, notFoundObjectErr := fetcher.Fetch(ctx, "public-photos/missing.png", publicRequest)
	require.Error(t, notFoundObjectErr)
	require.ErrorIs(t, notFoundObjectErr, ErrStorageNotFound)

	// Nonexistent bucket -> ErrStorageNotFound
	_, _, _, notFoundBucketErr := fetcher.Fetch(ctx, "ghost-bucket/avatar.png", publicRequest)
	require.Error(t, notFoundBucketErr)
	require.ErrorIs(t, notFoundBucketErr, ErrStorageNotFound)

	// Chunk query failure when chunks table is dropped
	_, dropErr := kernel.DB().Exec(ctx, "DROP TABLE file_storage.chunks CASCADE;")
	require.NoError(t, dropErr)
	_, _, _, chunkQueryErr := fetcher.Fetch(ctx, "public-photos/avatar.png", publicRequest)
	require.Error(t, chunkQueryErr)
	require.Contains(t, chunkQueryErr.Error(), "failed to query object chunks")
}
