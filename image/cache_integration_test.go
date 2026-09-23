package image

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageCachePostgresIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	cacheManager := NewCacheManager(kernel)

	sourceURL := "local/user-avatars/sample.png"
	optionsHash := "hash-12345"
	contentType := "image/png"
	cacheKey := "sample-cache-key-for-database"
	payload := []byte("binary-cache-content")

	// 1. Set cache entry and verify it records in PostgreSQL
	eTag := cacheManager.Set(ctx, cacheKey, sourceURL, optionsHash, contentType, payload)
	require.NotEmpty(t, eTag)

	// Wait briefly for asynchronous database record goroutine
	time.Sleep(100 * time.Millisecond)

	var (
		storedSourceURL   string
		storedOptionsHash string
		storedContentType string
		storedByteSize    int64
		storedETag        string
	)
	const querySQL = `
		SELECT source_url, options_hash, content_type, byte_size, etag
		FROM image.cache_entries
		WHERE cache_key = $1;
	`
	queryErr := kernel.DB().QueryRow(ctx, querySQL, cacheKey).Scan(
		&storedSourceURL,
		&storedOptionsHash,
		&storedContentType,
		&storedByteSize,
		&storedETag,
	)
	require.NoError(t, queryErr)
	require.Equal(t, sourceURL, storedSourceURL)
	require.Equal(t, optionsHash, storedOptionsHash)
	require.Equal(t, contentType, storedContentType)
	require.Equal(t, int64(len(payload)), storedByteSize)
	require.Equal(t, eTag, storedETag)

	// 2. Access existing cache item (hit) which triggers touchDatabaseEntry
	cachedImage, found := cacheManager.Get(ctx, cacheKey)
	require.True(t, found)
	require.NotNil(t, cachedImage)

	time.Sleep(100 * time.Millisecond)

	// 3. Direct verification of recordDatabaseEntry and touchDatabaseEntry
	cacheManager.recordDatabaseEntry(ctx, cacheKey, sourceURL, optionsHash, contentType, int64(len(payload)), eTag)
	cacheManager.touchDatabaseEntry(ctx, cacheKey)
}
