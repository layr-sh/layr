package image

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"uuid"

	"layr.sh/core"
)

const (
	defaultMaxMemoryCacheEntries = 1000
	defaultMaxCachedItemBytes    = 10485760 // 10MB
	databaseTouchTimeout         = 3 * time.Second
	databaseRecordTimeout        = 5 * time.Second
)

// CachedImage represents a processed image asset held in memory.
type CachedImage struct {
	CacheKey    string
	ContentType string
	ETag        string
	Data        []byte
	CreatedAt   time.Time
}

type lruEntry struct {
	key         string
	cachedImage *CachedImage
}

// CacheManager manages in-memory LRU caching and PostgreSQL image.cache_entries index.
type CacheManager struct {
	kernel             *core.Kernel
	rwMutex            sync.RWMutex
	memoryCache        map[string]*list.Element
	lruList            *list.List
	maxEntries         int
	totalRequestsInt64 atomic.Int64
	cacheHitsInt64     atomic.Int64
	cacheMissesInt64   atomic.Int64
	bytesServedInt64   atomic.Int64
}

// NewCacheManager initializes a transformation cache manager.
func NewCacheManager(kernel *core.Kernel) *CacheManager {
	return &CacheManager{
		kernel:      kernel,
		memoryCache: make(map[string]*list.Element),
		lruList:     list.New(),
		maxEntries:  defaultMaxMemoryCacheEntries,
	}
}

// ComputeCacheKey generates a deterministic SHA256 hex string for a source asset and its processing parameters.
func ComputeCacheKey(sourceURL string, processingOptions ProcessingOptions) (string, string) {
	optionsPayload := fmt.Sprintf(
		"rs:%s:%d:%d:%v:%v|algo:%s|dpr:%.2f|g:%s:%.2f:%.2f|c:%.2f:%.2f:%s|t:%.2f:%s:%v:%v|pd:%d:%d:%d:%d|ar:%v|rot:%d|bg:%s:%.2f|bl:%.2f|sh:%.2f|pix:%d|wm:%.2f:%s:%d:%d:%.2f:%s|f:%s|q:%d|sm:%v|kcr:%v|scp:%v|z:%.2f:%.2f|cb:%s|raw:%v",
		processingOptions.ResizeType, processingOptions.Width, processingOptions.Height, processingOptions.Enlarge, processingOptions.Extend,
		processingOptions.ResizingAlgorithm, processingOptions.DPR,
		processingOptions.Gravity.Type, processingOptions.Gravity.XOffset, processingOptions.Gravity.YOffset,
		processingOptions.CropWidth, processingOptions.CropHeight, processingOptions.CropGravity.Type,
		processingOptions.TrimThreshold, processingOptions.TrimColor, processingOptions.TrimEqualHor, processingOptions.TrimEqualVer,
		processingOptions.PaddingTop, processingOptions.PaddingRight, processingOptions.PaddingBottom, processingOptions.PaddingLeft,
		processingOptions.AutoRotate, processingOptions.Rotate,
		processingOptions.Background, processingOptions.BackgroundAlpha,
		processingOptions.Blur, processingOptions.Sharpen, processingOptions.Pixelate,
		processingOptions.WatermarkOpacity, processingOptions.WatermarkGravity.Type, processingOptions.WatermarkXOffset, processingOptions.WatermarkYOffset, processingOptions.WatermarkScale, processingOptions.WatermarkURL,
		processingOptions.Format, processingOptions.Quality,
		processingOptions.StripMetadata, processingOptions.KeepCopyright, processingOptions.StripColorProfile,
		processingOptions.ZoomX, processingOptions.ZoomY,
		processingOptions.Cachebuster, processingOptions.RawMode,
	)

	optionsHashBytes := sha256.Sum256([]byte(optionsPayload))
	optionsHash := hex.EncodeToString(optionsHashBytes[:])

	fullPayload := sourceURL + "|" + optionsHash
	cacheKeyBytes := sha256.Sum256([]byte(fullPayload))
	cacheKey := hex.EncodeToString(cacheKeyBytes[:])

	return cacheKey, optionsHash
}

// Get checks the in-memory cache and returns the cached image if found.
func (cacheManager *CacheManager) Get(ctx context.Context, cacheKey string) (*CachedImage, bool) {
	cacheManager.totalRequestsInt64.Add(1)

	cacheManager.rwMutex.Lock()
	if element, found := cacheManager.memoryCache[cacheKey]; found {
		cacheManager.lruList.MoveToFront(element)
		entry := element.Value.(*lruEntry)
		cacheManager.cacheHitsInt64.Add(1)
		cacheManager.bytesServedInt64.Add(int64(len(entry.cachedImage.Data)))
		cacheManager.rwMutex.Unlock()

		// Asynchronously update last_accessed_at in DB
		go cacheManager.touchDatabaseEntry(ctx, cacheKey)

		return entry.cachedImage, true
	}
	cacheManager.rwMutex.Unlock()

	cacheManager.cacheMissesInt64.Add(1)
	return nil, false
}

// Set stores processed image bytes into memory LRU and records metadata into PostgreSQL.
func (cacheManager *CacheManager) Set(ctx context.Context, cacheKey string, sourceURL string, optionsHash string, contentType string, data []byte) string {
	eTagHash := sha256.Sum256(data)
	eTag := fmt.Sprintf("\"%s\"", hex.EncodeToString(eTagHash[:16]))

	cachedImage := &CachedImage{
		CacheKey:    cacheKey,
		ContentType: contentType,
		ETag:        eTag,
		Data:        data,
		CreatedAt:   time.Now(),
	}

	if len(data) <= defaultMaxCachedItemBytes {
		cacheManager.rwMutex.Lock()
		if element, found := cacheManager.memoryCache[cacheKey]; found {
			cacheManager.lruList.MoveToFront(element)
			element.Value.(*lruEntry).cachedImage = cachedImage
		} else {
			if cacheManager.lruList.Len() >= cacheManager.maxEntries {
				oldestElement := cacheManager.lruList.Back()
				cacheManager.lruList.Remove(oldestElement)
				oldestEntry := oldestElement.Value.(*lruEntry)
				delete(cacheManager.memoryCache, oldestEntry.key)
			}
			newElement := cacheManager.lruList.PushFront(&lruEntry{key: cacheKey, cachedImage: cachedImage})
			cacheManager.memoryCache[cacheKey] = newElement
		}
		cacheManager.rwMutex.Unlock()
	}

	// Persist cache metadata in PostgreSQL
	go cacheManager.recordDatabaseEntry(ctx, cacheKey, sourceURL, optionsHash, contentType, int64(len(data)), eTag)

	return eTag
}

// Stats returns real-time cache performance and memory allocation metrics.
func (cacheManager *CacheManager) Stats() GetStatsResponse {
	hits := cacheManager.cacheHitsInt64.Load()
	misses := cacheManager.cacheMissesInt64.Load()
	total := cacheManager.totalRequestsInt64.Load()

	var ratio float64
	if total > 0 {
		ratio = float64(hits) / float64(total)
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return GetStatsResponse{
		CacheHits:        hits,
		CacheMisses:      misses,
		CacheHitRatio:    ratio,
		TotalRequests:    total,
		BytesServed:      cacheManager.bytesServedInt64.Load(),
		MemoryUsageBytes: memStats.Alloc,
	}
}

func (cacheManager *CacheManager) touchDatabaseEntry(ctx context.Context, cacheKey string) {
	touchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), databaseTouchTimeout)
	defer cancel()

	const touchSQLStatement = `
		UPDATE image.cache_entries
		SET last_accessed_at = clock_timestamp()
		WHERE cache_key = $1;
	`
	_, _ = cacheManager.kernel.DB().Exec(touchCtx, touchSQLStatement, cacheKey)
}

func (cacheManager *CacheManager) recordDatabaseEntry(ctx context.Context, cacheKey, sourceURL, optionsHash, contentType string, byteSize int64, eTag string) {
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), databaseRecordTimeout)
	defer cancel()

	const upsertSQLStatement = `
		INSERT INTO image.cache_entries (id, cache_key, source_url, options_hash, content_type, byte_size, etag, last_accessed_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, clock_timestamp(), clock_timestamp())
		ON CONFLICT (cache_key) DO UPDATE
		SET last_accessed_at = clock_timestamp();
	`
	newID := uuid.New()
	_, _ = cacheManager.kernel.DB().Exec(recordCtx, upsertSQLStatement, newID, cacheKey, sourceURL, optionsHash, contentType, byteSize, eTag)
}

// Invalidate purges an image asset from in-memory LRU and PostgreSQL index, emitting image.cache.invalidated.
func (cacheManager *CacheManager) Invalidate(ctx context.Context, cacheKey string, sourceURL string) {
	cacheManager.rwMutex.Lock()
	if element, found := cacheManager.memoryCache[cacheKey]; found {
		cacheManager.lruList.Remove(element)
		delete(cacheManager.memoryCache, cacheKey)
	}
	cacheManager.rwMutex.Unlock()

	const deleteSQLStatement = `DELETE FROM image.cache_entries WHERE cache_key = $1;`
	_, _ = cacheManager.kernel.DB().Exec(ctx, deleteSQLStatement, cacheKey)

	cacheManager.kernel.EventBus().Publish(ctx, NewCacheInvalidatedEvent(cacheKey, CacheInvalidatedEventData{
		CacheKey:  cacheKey,
		SourceURL: sourceURL,
	}))
}

// Clear flushes all cached transformations from in-memory cache and PostgreSQL index, emitting image.cache.flushed.
func (cacheManager *CacheManager) Clear(ctx context.Context) {
	cacheManager.rwMutex.Lock()
	cacheManager.memoryCache = make(map[string]*list.Element)
	cacheManager.lruList = list.New()
	cacheManager.rwMutex.Unlock()

	const truncateSQLStatement = `DELETE FROM image.cache_entries;`
	_, _ = cacheManager.kernel.DB().Exec(ctx, truncateSQLStatement)

	cacheManager.kernel.EventBus().Publish(ctx, NewCacheFlushedEvent("*", CacheFlushedEventData{
		SourceURL: "*",
	}))
}
