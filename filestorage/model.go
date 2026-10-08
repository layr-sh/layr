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

// Object represents a stored binary asset entity in file_storage.objects.
type Object struct {
	ID             uuid.UUID      `json:"id"`
	BucketID       uuid.UUID      `json:"bucket_id"`
	ObjectKey      string         `json:"object_key"`
	ContentType    string         `json:"content_type"`
	SizeBytes      int64          `json:"size_bytes"`
	ChecksumSHA256 string         `json:"checksum_sha256"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	LastUpdatedAt  time.Time      `json:"last_updated_at"`
}

// Chunk represents a binary chunk slice of an object in file_storage.chunks.
type Chunk struct {
	ObjectID   uuid.UUID `json:"object_id"`
	ChunkIndex int       `json:"chunk_index"`
	ChunkData  []byte    `json:"chunk_data,omitempty"`
}
