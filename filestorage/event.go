package filestorage

import (
	"layr.sh/core"
)

// ConfigUpdatedEventData represents the payload for file_storage.config.updated.
type ConfigUpdatedEventData Config

// NewConfigUpdatedEvent creates a typed event for file storage configuration updates.
func NewConfigUpdatedEvent(resourceID string, configUpdatedEventData ConfigUpdatedEventData) core.Event {
	return core.NewEvent("file_storage.config.updated", configUpdatedEventData).WithResourceID(resourceID)
}

// BucketCreatedEventData represents the payload for file_storage.bucket.created.
type BucketCreatedEventData Bucket

// NewBucketCreatedEvent creates a typed event for bucket creations.
func NewBucketCreatedEvent(resourceID string, bucketCreatedEventData BucketCreatedEventData) core.Event {
	return core.NewEvent("file_storage.bucket.created", bucketCreatedEventData).WithResourceID(resourceID)
}

// BucketUpdatedEventData represents the payload for file_storage.bucket.updated.
type BucketUpdatedEventData Bucket

// NewBucketUpdatedEvent creates a typed event for bucket updates.
func NewBucketUpdatedEvent(resourceID string, bucketUpdatedEventData BucketUpdatedEventData) core.Event {
	return core.NewEvent("file_storage.bucket.updated", bucketUpdatedEventData).WithResourceID(resourceID)
}

// BucketDeletedEventData represents the payload for file_storage.bucket.deleted.
type BucketDeletedEventData struct {
	BucketName string `json:"bucket_name"`
}

// NewBucketDeletedEvent creates a typed event for bucket deletions.
func NewBucketDeletedEvent(resourceID string, bucketDeletedEventData BucketDeletedEventData) core.Event {
	return core.NewEvent("file_storage.bucket.deleted", bucketDeletedEventData).WithResourceID(resourceID)
}
