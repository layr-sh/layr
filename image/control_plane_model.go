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

// CreatePresetInput defines the request body for creating a new preset.
type CreatePresetInput struct {
	Name              string  `json:"name"`
	ProcessingOptions string  `json:"processing_options"`
	Description       *string `json:"description,omitempty"`
}

// UpdatePresetInput defines the request body for modifying an existing preset.
type UpdatePresetInput struct {
	Name              *string `json:"name,omitempty"`
	ProcessingOptions *string `json:"processing_options,omitempty"`
	Description       *string `json:"description,omitempty"`
}

// ListPresetsResponse represents the response containing multiple presets.
type ListPresetsResponse struct {
	Presets []Preset `json:"presets"`
	Count   int      `json:"count"`
}

// SignURLInput defines the request body for generating a signed image URL.
type SignURLInput struct {
	Path string `json:"path"`
}

// SignURLResponse represents the generated signature and full signed URL.
type SignURLResponse struct {
	URL       string `json:"url"`
	Signature string `json:"signature"`
}

// GetStatsResponse represents real-time cache performance and throughput metrics.
type GetStatsResponse struct {
	CacheHits        int64   `json:"cache_hits"`
	CacheMisses      int64   `json:"cache_misses"`
	CacheHitRatio    float64 `json:"cache_hit_ratio"`
	TotalRequests    int64   `json:"total_requests"`
	BytesServed      int64   `json:"bytes_served"`
	MemoryUsageBytes uint64  `json:"memory_usage_bytes"`
}
