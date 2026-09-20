package filestorage

import (
	"time"
	"uuid"
)

// Bucket represents a file storage bucket entity in file_storage.buckets.
type Bucket struct {
	ID               uuid.UUID      `json:"id"`
	Name             string         `json:"name"`
	IsPublic         bool           `json:"is_public"`
	Backend          string         `json:"backend"`
	BackendConfig    map[string]any `json:"backend_config,omitempty"`
	AllowedMIMETypes []string       `json:"allowed_mime_types"`
	MaxFileSizeBytes int64          `json:"max_file_size_bytes"`
	CreatedAt        time.Time      `json:"created_at"`
	LastUpdatedAt    time.Time      `json:"last_updated_at"`
}

// Chunk represents a binary chunk slice of an object in file_storage.chunks.
type Chunk struct {
	ObjectID   uuid.UUID `json:"object_id"`
	ChunkIndex int       `json:"chunk_index"`
	ChunkData  []byte    `json:"chunk_data,omitempty"`
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
	Buckets []Bucket `json:"buckets"`
	Count   int      `json:"count"`
}

// ListBucketObjectsResponse represents the response containing objects in a bucket.
type ListBucketObjectsResponse struct {
	Objects []Object `json:"objects"`
	Count   int      `json:"count"`
}
