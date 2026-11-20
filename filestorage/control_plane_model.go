package filestorage

import (
	"time"

	"layr.sh/core"
)

// StatsResponse reports tracked logical usage, not physical database or upstream disk usage.
type StatsResponse struct {
	TotalBuckets       int64              `json:"total_buckets"`
	PublicBuckets      int64              `json:"public_buckets"`
	PrivateBuckets     int64              `json:"private_buckets"`
	DatabaseBuckets    int64              `json:"database_buckets"`
	S3Buckets          int64              `json:"s3_buckets"`
	TotalObjects       int64              `json:"total_objects"`
	TotalSizeBytes     int64              `json:"total_size_bytes"`
	DatabaseObjects    int64              `json:"database_objects"`
	DatabaseSizeBytes  int64              `json:"database_size_bytes"`
	S3Objects          int64              `json:"s3_objects"`
	S3SizeBytes        int64              `json:"s3_size_bytes"`
	DatabaseChunks     int64              `json:"database_chunks"`
	DatabaseChunkBytes int64              `json:"database_chunk_bytes"`
	MultipartUploads   int64              `json:"multipart_uploads"`
	MultipartParts     int64              `json:"multipart_parts"`
	MultipartSizeBytes int64              `json:"multipart_size_bytes"`
	LastUpdatedAt      *time.Time         `json:"last_updated_at"`
	Buckets            []BucketUsage      `json:"buckets"`
	ContentTypes       []ContentTypeUsage `json:"content_types"`
}

// BucketUsage summarizes the objects registered to a bucket.
type BucketUsage struct {
	Name          string     `json:"name"`
	Backend       string     `json:"backend"`
	IsPublic      bool       `json:"is_public"`
	ObjectCount   int64      `json:"object_count"`
	SizeBytes     int64      `json:"size_bytes"`
	LastUpdatedAt *time.Time `json:"last_updated_at"`
}

// ContentTypeUsage summarizes tracked objects by their stored content type.
type ContentTypeUsage struct {
	ContentType string `json:"content_type"`
	ObjectCount int64  `json:"object_count"`
	SizeBytes   int64  `json:"size_bytes"`
}

// CreateBucketInput defines the request body for creating a new bucket.
type CreateBucketInput struct {
	Name             string         `json:"name"`
	IsPublic         bool           `json:"is_public"`
	Backend          string         `json:"backend"`
	BackendConfig    map[string]any `json:"backend_config,omitempty"`
	AllowedMIMETypes []string       `json:"allowed_mime_types"`
	MaxFileSizeBytes int64          `json:"max_file_size_bytes"`
}

// UpdateBucketInput defines the request body for modifying an existing bucket.
type UpdateBucketInput struct {
	IsPublic         *bool          `json:"is_public,omitempty"`
	Backend          *string        `json:"backend,omitempty"`
	BackendConfig    map[string]any `json:"backend_config,omitempty"`
	AllowedMIMETypes []string       `json:"allowed_mime_types,omitempty"`
	MaxFileSizeBytes *int64         `json:"max_file_size_bytes,omitempty"`
}

// ListBucketsResponse represents the response containing multiple buckets.
type ListBucketsResponse struct {
	Data  []Bucket `json:"data"`
	Count int      `json:"count"`
}

// ListBucketObjectsResponse represents the response containing objects in a bucket.
type ListBucketObjectsResponse struct {
	Data []Object `json:"data"`
	core.PaginationOutput
}
