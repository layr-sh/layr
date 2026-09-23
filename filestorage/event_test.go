package filestorage

import (
	"testing"
	"time"

	"uuid"

	"github.com/stretchr/testify/require"
)

func TestFilestorageEventUnit(t *testing.T) {
	t.Parallel()

	t.Run("creates config updated event", func(t *testing.T) {
		t.Parallel()
		configUpdatedEventData := ConfigUpdatedEventData(DefaultConfig())
		configUpdatedEvent := NewConfigUpdatedEvent("file_storage.config", configUpdatedEventData)
		require.Equal(t, "file_storage.config.updated", configUpdatedEvent.Type)
		require.NotNil(t, configUpdatedEvent.ResourceID)
		require.Equal(t, "file_storage.config", *configUpdatedEvent.ResourceID)
	})

	t.Run("creates bucket created event", func(t *testing.T) {
		t.Parallel()
		bucketCreatedEventData := BucketCreatedEventData{
			Name: "test-bucket",
		}
		bucketCreatedEvent := NewBucketCreatedEvent("bucket-1", bucketCreatedEventData)
		require.Equal(t, "file_storage.bucket.created", bucketCreatedEvent.Type)
		require.NotNil(t, bucketCreatedEvent.ResourceID)
		require.Equal(t, "bucket-1", *bucketCreatedEvent.ResourceID)
	})

	t.Run("creates bucket updated event", func(t *testing.T) {
		t.Parallel()
		bucketUpdatedEventData := BucketUpdatedEventData{
			Name: "test-bucket",
		}
		bucketUpdatedEvent := NewBucketUpdatedEvent("bucket-2", bucketUpdatedEventData)
		require.Equal(t, "file_storage.bucket.updated", bucketUpdatedEvent.Type)
		require.NotNil(t, bucketUpdatedEvent.ResourceID)
		require.Equal(t, "bucket-2", *bucketUpdatedEvent.ResourceID)
	})

	t.Run("creates bucket deleted event", func(t *testing.T) {
		t.Parallel()
		bucketDeletedEventData := BucketDeletedEventData{
			BucketName: "test-bucket",
		}
		bucketDeletedEvent := NewBucketDeletedEvent("bucket-3", bucketDeletedEventData)
		require.Equal(t, "file_storage.bucket.deleted", bucketDeletedEvent.Type)
		require.NotNil(t, bucketDeletedEvent.ResourceID)
		require.Equal(t, "bucket-3", *bucketDeletedEvent.ResourceID)
	})

	t.Run("creates object uploaded event", func(t *testing.T) {
		t.Parallel()
		objectID := uuid.NewV7()
		bucketID := uuid.NewV7()
		objectUploadedEventData := ObjectUploadedEventData{
			BucketID:       bucketID,
			BucketName:     "photos",
			ObjectKey:      "sample.png",
			ContentType:    "image/png",
			SizeBytes:      1024,
			ChecksumSHA256: "abc123sha",
		}
		objectUploadedEvent := NewObjectUploadedEvent(objectID.String(), objectUploadedEventData)
		require.Equal(t, "file_storage.object.uploaded", objectUploadedEvent.Type)
		require.NotNil(t, objectUploadedEvent.ResourceID)
		require.Equal(t, objectID.String(), *objectUploadedEvent.ResourceID)
	})

	t.Run("creates object downloaded event", func(t *testing.T) {
		t.Parallel()
		objectDownloadedEventData := ObjectDownloadedEventData{
			BucketName: "photos",
			ObjectKey:  "sample.png",
			SizeBytes:  1024,
		}
		objectDownloadedEvent := NewObjectDownloadedEvent("photos/sample.png", objectDownloadedEventData)
		require.Equal(t, "file_storage.object.downloaded", objectDownloadedEvent.Type)
		require.NotNil(t, objectDownloadedEvent.ResourceID)
		require.Equal(t, "photos/sample.png", *objectDownloadedEvent.ResourceID)
	})

	t.Run("creates object deleted event", func(t *testing.T) {
		t.Parallel()
		objectDeletedEventData := ObjectDeletedEventData{
			BucketName: "photos",
			ObjectKey:  "sample.png",
		}
		objectDeletedEvent := NewObjectDeletedEvent("photos/sample.png", objectDeletedEventData)
		require.Equal(t, "file_storage.object.deleted", objectDeletedEvent.Type)
		require.NotNil(t, objectDeletedEvent.ResourceID)
		require.Equal(t, "photos/sample.png", *objectDeletedEvent.ResourceID)
	})

	t.Run("creates upload failed event", func(t *testing.T) {
		t.Parallel()
		uploadFailedEventData := UploadFailedEventData{
			BucketName: "photos",
			ObjectKey:  "sample.png",
			Reason:     "payload too large",
			StatusCode: 413,
		}
		uploadFailedEvent := NewUploadFailedEvent("photos/sample.png", uploadFailedEventData)
		require.Equal(t, "file_storage.upload_failed", uploadFailedEvent.Type)
		require.NotNil(t, uploadFailedEvent.ResourceID)
		require.Equal(t, "photos/sample.png", *uploadFailedEvent.ResourceID)
	})

	t.Run("creates multipart initiated event", func(t *testing.T) {
		t.Parallel()
		multipartInitiatedEventData := MultipartInitiatedEventData{
			UploadID:   "upload-123",
			BucketName: "photos",
			ObjectKey:  "sample.png",
		}
		multipartInitiatedEvent := NewMultipartInitiatedEvent("upload-123", multipartInitiatedEventData)
		require.Equal(t, "file_storage.multipart.initiated", multipartInitiatedEvent.Type)
		require.NotNil(t, multipartInitiatedEvent.ResourceID)
		require.Equal(t, "upload-123", *multipartInitiatedEvent.ResourceID)
	})

	t.Run("creates multipart completed event", func(t *testing.T) {
		t.Parallel()
		multipartCompletedEventData := MultipartCompletedEventData{
			UploadID:       "upload-123",
			BucketName:     "photos",
			ObjectKey:      "sample.png",
			SizeBytes:      2048,
			ChecksumSHA256: "etag123",
		}
		multipartCompletedEvent := NewMultipartCompletedEvent("upload-123", multipartCompletedEventData)
		require.Equal(t, "file_storage.multipart.completed", multipartCompletedEvent.Type)
		require.NotNil(t, multipartCompletedEvent.ResourceID)
		require.Equal(t, "upload-123", *multipartCompletedEvent.ResourceID)
	})

	t.Run("creates multipart aborted event", func(t *testing.T) {
		t.Parallel()
		multipartAbortedEventData := MultipartAbortedEventData{
			UploadID:   "upload-123",
			BucketName: "photos",
			ObjectKey:  "sample.png",
		}
		multipartAbortedEvent := NewMultipartAbortedEvent("upload-123", multipartAbortedEventData)
		require.Equal(t, "file_storage.multipart.aborted", multipartAbortedEvent.Type)
		require.NotNil(t, multipartAbortedEvent.ResourceID)
		require.Equal(t, "upload-123", *multipartAbortedEvent.ResourceID)
	})

	t.Run("creates url presigned event", func(t *testing.T) {
		t.Parallel()
		now := time.Now().UTC()
		urlPresignedEventData := URLPresignedEventData{
			BucketName: "photos",
			ObjectKey:  "sample.png",
			Operation:  "read",
			URL:        "/v1/file-storage/objects/photos/sample.png?token=xyz",
			ExpiresAt:  now,
		}
		urlPresignedEvent := NewURLPresignedEvent("photos/sample.png", urlPresignedEventData)
		require.Equal(t, "file_storage.url.presigned", urlPresignedEvent.Type)
		require.NotNil(t, urlPresignedEvent.ResourceID)
		require.Equal(t, "photos/sample.png", *urlPresignedEvent.ResourceID)
	})
}
