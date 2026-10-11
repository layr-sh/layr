package filestorage

import "layr.sh/core"

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
	Data []Object `json:"data"`
	core.PaginationOutput
}
