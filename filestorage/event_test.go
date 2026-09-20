package filestorage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilestorageEventUnit(t *testing.T) {
	t.Parallel()

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
}
