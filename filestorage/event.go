package filestorage

import (
	"time"

	"uuid"

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

// ObjectUploadedEventData represents the payload for file_storage.object.uploaded.
type ObjectUploadedEventData struct {
	BucketID       uuid.UUID `json:"bucket_id"`
	BucketName     string    `json:"bucket_name"`
	ObjectKey      string    `json:"object_key"`
	ContentType    string    `json:"content_type"`
	SizeBytes      int64     `json:"size_bytes"`
	ChecksumSHA256 string    `json:"checksum_sha256"`
}

// NewObjectUploadedEvent creates a typed event for object uploads.
func NewObjectUploadedEvent(resourceID string, objectUploadedEventData ObjectUploadedEventData) core.Event {
	return core.NewEvent("file_storage.object.uploaded", objectUploadedEventData).WithResourceID(resourceID)
}

// ObjectDownloadedEventData represents the payload for file_storage.object.downloaded.
type ObjectDownloadedEventData struct {
	BucketName string `json:"bucket_name"`
	ObjectKey  string `json:"object_key"`
	SizeBytes  int64  `json:"size_bytes"`
}

// NewObjectDownloadedEvent creates a typed event for object downloads.
func NewObjectDownloadedEvent(resourceID string, objectDownloadedEventData ObjectDownloadedEventData) core.Event {
	return core.NewEvent("file_storage.object.downloaded", objectDownloadedEventData).WithResourceID(resourceID)
}

// ObjectDeletedEventData represents the payload for file_storage.object.deleted.
type ObjectDeletedEventData struct {
	BucketName string `json:"bucket_name"`
	ObjectKey  string `json:"object_key"`
}

// NewObjectDeletedEvent creates a typed event for object deletions.
func NewObjectDeletedEvent(resourceID string, objectDeletedEventData ObjectDeletedEventData) core.Event {
	return core.NewEvent("file_storage.object.deleted", objectDeletedEventData).WithResourceID(resourceID)
}

// UploadFailedEventData represents the payload for file_storage.upload_failed.
type UploadFailedEventData struct {
	BucketName string `json:"bucket_name"`
	ObjectKey  string `json:"object_key"`
	Reason     string `json:"reason"`
	StatusCode int    `json:"status_code"`
}

// NewUploadFailedEvent creates a typed event for failed uploads.
func NewUploadFailedEvent(resourceID string, uploadFailedEventData UploadFailedEventData) core.Event {
	return core.NewEvent("file_storage.upload_failed", uploadFailedEventData).WithResourceID(resourceID)
}

// MultipartInitiatedEventData represents the payload for file_storage.multipart.initiated.
type MultipartInitiatedEventData struct {
	UploadID   string `json:"upload_id"`
	BucketName string `json:"bucket_name"`
	ObjectKey  string `json:"object_key"`
}

// NewMultipartInitiatedEvent creates a typed event for initiated multipart uploads.
func NewMultipartInitiatedEvent(resourceID string, multipartInitiatedEventData MultipartInitiatedEventData) core.Event {
	return core.NewEvent("file_storage.multipart.initiated", multipartInitiatedEventData).WithResourceID(resourceID)
}

// MultipartCompletedEventData represents the payload for file_storage.multipart.completed.
type MultipartCompletedEventData struct {
	UploadID       string `json:"upload_id"`
	BucketName     string `json:"bucket_name"`
	ObjectKey      string `json:"object_key"`
	SizeBytes      int64  `json:"size_bytes"`
	ChecksumSHA256 string `json:"checksum_sha256"`
}

// NewMultipartCompletedEvent creates a typed event for completed multipart uploads.
func NewMultipartCompletedEvent(resourceID string, multipartCompletedEventData MultipartCompletedEventData) core.Event {
	return core.NewEvent("file_storage.multipart.completed", multipartCompletedEventData).WithResourceID(resourceID)
}

// MultipartAbortedEventData represents the payload for file_storage.multipart.aborted.
type MultipartAbortedEventData struct {
	UploadID   string `json:"upload_id"`
	BucketName string `json:"bucket_name"`
	ObjectKey  string `json:"object_key"`
}

// NewMultipartAbortedEvent creates a typed event for aborted multipart uploads.
func NewMultipartAbortedEvent(resourceID string, multipartAbortedEventData MultipartAbortedEventData) core.Event {
	return core.NewEvent("file_storage.multipart.aborted", multipartAbortedEventData).WithResourceID(resourceID)
}

// URLPresignedEventData represents the payload for file_storage.url.presigned.
type URLPresignedEventData struct {
	BucketName string    `json:"bucket_name"`
	ObjectKey  string    `json:"object_key"`
	Operation  string    `json:"operation"`
	URL        string    `json:"url"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// NewURLPresignedEvent creates a typed event for presigned URL generation.
func NewURLPresignedEvent(resourceID string, urlPresignedEventData URLPresignedEventData) core.Event {
	return core.NewEvent("file_storage.url.presigned", urlPresignedEventData).WithResourceID(resourceID)
}
