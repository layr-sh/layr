package image

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageCacheUnit(t *testing.T) {
	t.Parallel()

	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	t.Run("compute cache key determinism", func(t *testing.T) {
		firstProcessingOptions := ProcessingOptions{Width: 300, Height: 200, Format: FormatWebP}
		secondProcessingOptions := ProcessingOptions{Width: 300, Height: 200, Format: FormatWebP}
		thirdProcessingOptions := ProcessingOptions{Width: 400, Height: 200, Format: FormatWebP}

		key1, hash1 := ComputeCacheKey("local/bucket/img.jpg", firstProcessingOptions)
		key2, hash2 := ComputeCacheKey("local/bucket/img.jpg", secondProcessingOptions)
		key3, hash3 := ComputeCacheKey("local/bucket/img.jpg", thirdProcessingOptions)

		require.Equal(t, key1, key2)
		require.Equal(t, hash1, hash2)
		require.NotEqual(t, key1, key3)
		require.NotEqual(t, hash1, hash3)
	})

	t.Run("get and set operations with stats", func(t *testing.T) {
		cacheManager := NewCacheManager(kernel)
		ctx := context.Background()

		// Initial get -> miss
		cachedImage, found := cacheManager.Get(ctx, "nonexistent-key")
		require.False(t, found)
		require.Nil(t, cachedImage)

		// Set image
		payload := []byte("fake-image-bytes")
		eTag := cacheManager.Set(ctx, "test-key-1", "local/bucket/pic.png", "hash1", "image/png", payload)
		require.NotEmpty(t, eTag)

		// Get image -> hit
		retrievedCachedImage, hit := cacheManager.Get(ctx, "test-key-1")
		require.True(t, hit)
		require.NotNil(t, retrievedCachedImage)
		require.Equal(t, "image/png", retrievedCachedImage.ContentType)
		require.Equal(t, payload, retrievedCachedImage.Data)
		require.Equal(t, eTag, retrievedCachedImage.ETag)

		// Check Stats
		getStatsResponse := cacheManager.Stats()
		require.Equal(t, int64(1), getStatsResponse.CacheHits)
		require.Equal(t, int64(1), getStatsResponse.CacheMisses)
		require.Equal(t, int64(2), getStatsResponse.TotalRequests)
		require.Equal(t, 0.5, getStatsResponse.CacheHitRatio)
		require.Equal(t, int64(len(payload)), getStatsResponse.BytesServed)
	})

	t.Run("lru eviction policy", func(t *testing.T) {
		cacheManager := NewCacheManager(kernel)
		cacheManager.maxEntries = 3
		ctx := context.Background()

		for index := 1; index <= 4; index++ {
			key := fmt.Sprintf("key-%d", index)
			cacheManager.Set(ctx, key, "url", "hash", "image/jpeg", []byte("data"))
		}

		// Key-1 should have been evicted
		_, found1 := cacheManager.Get(ctx, "key-1")
		require.False(t, found1)

		// Key-4 should be present
		_, found4 := cacheManager.Get(ctx, "key-4")
		require.True(t, found4)
	})

	t.Run("update existing cache key", func(t *testing.T) {
		cacheManager := NewCacheManager(kernel)
		ctx := context.Background()

		cacheManager.Set(ctx, "duplicate-key", "url", "hash", "image/png", []byte("data1"))
		cacheManager.Set(ctx, "duplicate-key", "url", "hash", "image/png", []byte("data2"))

		cachedImage, found := cacheManager.Get(ctx, "duplicate-key")
		require.True(t, found)
		require.Equal(t, []byte("data2"), cachedImage.Data)
	})

	t.Run("invalidate cache key", func(t *testing.T) {
		cacheManager := NewCacheManager(kernel)
		ctx := context.Background()

		cacheManager.Set(ctx, "key-to-invalidate", "url", "hash", "image/png", []byte("data"))
		_, found := cacheManager.Get(ctx, "key-to-invalidate")
		require.True(t, found)

		cacheManager.Invalidate(ctx, "key-to-invalidate", "url")
		_, foundAfter := cacheManager.Get(ctx, "key-to-invalidate")
		require.False(t, foundAfter)

		// Invalidate non-existent key
		cacheManager.Invalidate(ctx, "non-existent-key", "url")
	})

	t.Run("clear cache", func(t *testing.T) {
		cacheManager := NewCacheManager(kernel)
		ctx := context.Background()

		cacheManager.Set(ctx, "key-1", "url1", "hash1", "image/png", []byte("data1"))
		cacheManager.Set(ctx, "key-2", "url2", "hash2", "image/png", []byte("data2"))

		cacheManager.Clear(ctx)

		_, found1 := cacheManager.Get(ctx, "key-1")
		require.False(t, found1)
		_, found2 := cacheManager.Get(ctx, "key-2")
		require.False(t, found2)
	})
}
