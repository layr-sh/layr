package image

import (
	"time"

	"uuid"
)

// Preset represents a named transformation preset entity in image.presets.
type Preset struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	ProcessingOptions string    `json:"processing_options"`
	Description       *string   `json:"description,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CacheEntry represents transformation cache metadata stored in image.cache_entries.
type CacheEntry struct {
	ID             uuid.UUID `json:"id"`
	CacheKey       string    `json:"cache_key"`
	SourceURL      string    `json:"source_url"`
	OptionsHash    string    `json:"options_hash"`
	ContentType    string    `json:"content_type"`
	ByteSize       int64     `json:"byte_size"`
	ETag           string    `json:"etag"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
	CreatedAt      time.Time `json:"created_at"`
}
