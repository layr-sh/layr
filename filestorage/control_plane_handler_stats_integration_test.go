package filestorage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFilestorageControlPlaneHandlerStatsIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	service := NewService(kernel)
	ctx := core.WithAuthContext(context.Background(), core.AuthContext{ServiceAccountID: "storage-admin", JWT: core.JWTClaims{Scope: "storage:*", Role: "service_role", Audience: "test:service_account"}})
	controlPlaneHandler := service.ControlPlaneHandler()

	t.Run("empty and unauthorized statistics", func(t *testing.T) {
		responseRecorder := httptest.NewRecorder()
		controlPlaneHandler.handleGetStats(responseRecorder, httptest.NewRequestWithContext(ctx, http.MethodGet, "/stats", nil))
		require.Equal(t, http.StatusOK, responseRecorder.Code, responseRecorder.Body.String())
		var statsResponse StatsResponse
		require.NoError(t, json.Unmarshal(responseRecorder.Body.Bytes(), &statsResponse))
		require.Zero(t, statsResponse.TotalObjects)
		require.Empty(t, statsResponse.Buckets)
		require.Empty(t, statsResponse.ContentTypes)
		require.Nil(t, statsResponse.LastUpdatedAt)
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleGetStats(responseRecorder, httptest.NewRequestWithContext(core.WithAuthContext(ctx, core.AuthContext{}), http.MethodGet, "/stats", nil))
		require.Equal(t, http.StatusForbidden, responseRecorder.Code)
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		responseRecorder = httptest.NewRecorder()
		controlPlaneHandler.handleGetStats(responseRecorder, httptest.NewRequestWithContext(canceledCtx, http.MethodGet, "/stats", nil))
		require.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})

	config := DefaultConfig()
	config.DefaultMaxFileSizeBytes = 100
	config.DefaultAllowedMIMETypes = []string{"image/*"}
	config.ChunkSizeBytes = 3
	config.PresignTokenExpirySeconds = 120
	service.ConfigManager().SetMemoryConfig(config)
	responseRecorder := httptest.NewRecorder()
	controlPlaneHandler.handleCreateBucket(responseRecorder, httptest.NewRequestWithContext(ctx, http.MethodPost, "/buckets", strings.NewReader(`{"name":"runtime-bucket"}`)))
	require.Equal(t, http.StatusCreated, responseRecorder.Code, responseRecorder.Body.String())
	var bucket Bucket
	require.NoError(t, json.Unmarshal(responseRecorder.Body.Bytes(), &bucket))
	require.Equal(t, int64(100), bucket.MaxFileSizeBytes)
	require.Equal(t, []string{"image/*"}, bucket.AllowedMIMETypes)
	_, err := service.DatabaseEngine().Upload(ctx, bucket, "denied.txt", strings.NewReader("x"), 1, "text/plain")
	require.ErrorIs(t, err, ErrContentTypeNotAllowed)
	object, err := service.DatabaseEngine().Upload(ctx, bucket, "old.png", strings.NewReader("12345678"), 8, "image/png; name=test")
	require.NoError(t, err)
	var chunks int
	require.NoError(t, kernel.DB().QueryRow(ctx, `SELECT COUNT(*) FROM file_storage.chunks WHERE object_id = $1`, object.ID).Scan(&chunks))
	require.Equal(t, 3, chunks)
	config.ChunkSizeBytes = 2
	service.ConfigManager().SetMemoryConfig(config)
	downloadReadCloser, size, err := service.DatabaseEngine().Download(ctx, bucket, "old.png", &ContentRange{Start: 1, End: 5, IsPartial: true})
	require.NoError(t, err)
	body, err := io.ReadAll(downloadReadCloser)
	require.NoError(t, err)
	require.NoError(t, downloadReadCloser.Close())
	require.Equal(t, int64(5), size)
	require.Equal(t, "23456", string(body))
	_, err = service.DatabaseEngine().Upload(ctx, bucket, "new.png", strings.NewReader("abcd"), 4, "image/png")
	require.NoError(t, err)

	responseRecorder = httptest.NewRecorder()
	controlPlaneHandler.handleCreateBucket(responseRecorder, httptest.NewRequestWithContext(ctx, http.MethodPost, "/buckets", strings.NewReader(`{"name":"explicit-bucket","allowed_mime_types":[],"max_file_size_bytes":200}`)))
	require.Equal(t, http.StatusCreated, responseRecorder.Code, responseRecorder.Body.String())
	var explicitBucket Bucket
	require.NoError(t, json.Unmarshal(responseRecorder.Body.Bytes(), &explicitBucket))
	require.Empty(t, explicitBucket.AllowedMIMETypes)
	require.Equal(t, int64(200), explicitBucket.MaxFileSizeBytes)

	responseRecorder = httptest.NewRecorder()
	service.BaseHandler().handlePresignURL(responseRecorder, httptest.NewRequestWithContext(ctx, http.MethodPost, "/presign", strings.NewReader(`{"bucket":"runtime-bucket","key":"reports/#?.png","operation":"read"}`)))
	require.Equal(t, http.StatusOK, responseRecorder.Code, responseRecorder.Body.String())
	var presignURLResponse PresignURLResponse
	require.NoError(t, json.Unmarshal(responseRecorder.Body.Bytes(), &presignURLResponse))
	require.WithinDuration(t, time.Now().Add(120*time.Second), presignURLResponse.ExpiresAt, 3*time.Second)
	require.Contains(t, presignURLResponse.URL, "%23%3F")

	var upstreamMutex sync.Mutex
	var failDelete bool
	var deletes int32
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		upstreamMutex.Lock()
		defer upstreamMutex.Unlock()
		switch request.Method {
		case http.MethodPut:
			_, _ = io.Copy(io.Discard, request.Body)
			responseWriter.Header().Set("ETag", `"test-etag"`)
		case http.MethodDelete:
			require.Equal(t, "/upstream/tracked.txt", request.URL.Path)
			if failDelete {
				http.Error(responseWriter, "unavailable", http.StatusForbidden)
				return
			}
			deletes++
			responseWriter.WriteHeader(http.StatusNoContent)
		default:
			responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer upstreamServer.Close()
	s3ID := uuid.NewV7()
	backend, err := json.Marshal(map[string]any{"endpoint": upstreamServer.URL, "bucket": "upstream", "access_key_id": "test", "secret_access_key": "secret"})
	require.NoError(t, err)
	_, err = kernel.DB().Exec(ctx, `INSERT INTO file_storage.buckets (id,name,backend,backend_config,max_file_size_bytes) VALUES ($1,'tracked-s3','s3',$2,100)`, s3ID, backend)
	require.NoError(t, err)
	s3Bucket, err := service.BaseHandler().resolveBucket(ctx, "tracked-s3")
	require.NoError(t, err)
	_, err = service.S3Engine().Upload(ctx, *s3Bucket, "tracked.txt", strings.NewReader("hello"), -1, "text/plain")
	require.NoError(t, err)
	_, err = service.S3Engine().Upload(ctx, *s3Bucket, "too-large.txt", strings.NewReader(strings.Repeat("x", 101)), -1, "text/plain")
	require.ErrorIs(t, err, ErrPayloadTooLarge)
	uploadID := uuid.NewV7()
	_, err = kernel.DB().Exec(ctx, `INSERT INTO file_storage.multipart_uploads (id,bucket_id,object_key,content_type) VALUES ($1,$2,'pending.png','image/png')`, uploadID, bucket.ID)
	require.NoError(t, err)
	_, err = kernel.DB().Exec(ctx, `INSERT INTO file_storage.multipart_parts (upload_id,part_number,etag,size_bytes,chunk_data) VALUES ($1,1,'part',3,$2)`, uploadID, []byte("abc"))
	require.NoError(t, err)
	responseRecorder = httptest.NewRecorder()
	controlPlaneHandler.handleGetStats(responseRecorder, httptest.NewRequestWithContext(ctx, http.MethodGet, "/stats", nil))
	require.Equal(t, http.StatusOK, responseRecorder.Code, responseRecorder.Body.String())
	var statsResponse StatsResponse
	require.NoError(t, json.Unmarshal(responseRecorder.Body.Bytes(), &statsResponse))
	require.Equal(t, int64(3), statsResponse.TotalBuckets)
	require.Equal(t, int64(3), statsResponse.TotalObjects)
	require.Equal(t, int64(17), statsResponse.TotalSizeBytes)
	require.Equal(t, int64(12), statsResponse.DatabaseSizeBytes)
	require.Equal(t, int64(5), statsResponse.S3SizeBytes)
	require.Equal(t, int64(5), statsResponse.DatabaseChunks)
	require.Equal(t, int64(12), statsResponse.DatabaseChunkBytes)
	require.Equal(t, int64(1), statsResponse.MultipartUploads)
	require.Equal(t, int64(1), statsResponse.MultipartParts)
	require.Equal(t, int64(3), statsResponse.MultipartSizeBytes)
	require.NotNil(t, statsResponse.LastUpdatedAt)
	require.Len(t, statsResponse.ContentTypes, 3)
	require.Len(t, statsResponse.Buckets, 3)

	upstreamMutex.Lock()
	failDelete = true
	upstreamMutex.Unlock()
	request := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/buckets/tracked-s3", nil)
	request.SetPathValue("bucket", "tracked-s3")
	responseRecorder = httptest.NewRecorder()
	controlPlaneHandler.handleDeleteBucket(responseRecorder, request)
	require.Equal(t, http.StatusInternalServerError, responseRecorder.Code, responseRecorder.Body.String())
	_, err = service.BaseHandler().resolveBucket(ctx, "tracked-s3")
	require.NoError(t, err)
	var tracked int
	require.NoError(t, kernel.DB().QueryRow(ctx, `SELECT COUNT(*) FROM file_storage.objects WHERE bucket_id = $1`, s3ID).Scan(&tracked))
	require.Equal(t, 1, tracked)
	upstreamMutex.Lock()
	failDelete = false
	upstreamMutex.Unlock()
	responseRecorder = httptest.NewRecorder()
	controlPlaneHandler.handleDeleteBucket(responseRecorder, request)
	require.Equal(t, http.StatusNoContent, responseRecorder.Code, responseRecorder.Body.String())
	upstreamMutex.Lock()
	require.Equal(t, int32(1), deletes)
	upstreamMutex.Unlock()
	_, err = service.BaseHandler().resolveBucket(ctx, "tracked-s3")
	require.ErrorIs(t, err, ErrBucketNotFound)
}
