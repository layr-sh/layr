package image

import "layr.sh/core"

// ConfigUpdatedEventData represents the payload for image.config.updated.
type ConfigUpdatedEventData Config

// NewConfigUpdatedEvent creates a typed event for image configuration updates.
func NewConfigUpdatedEvent(resourceID string, configUpdatedEventData ConfigUpdatedEventData) core.Event {
	return core.NewEvent("image.config.updated", configUpdatedEventData).WithResourceID(resourceID)
}

// PresetCreatedEventData represents the payload for image.preset.created.
type PresetCreatedEventData Preset

// NewPresetCreatedEvent creates a typed event for preset creations.
func NewPresetCreatedEvent(resourceID string, presetCreatedEventData PresetCreatedEventData) core.Event {
	return core.NewEvent("image.preset.created", presetCreatedEventData).WithResourceID(resourceID)
}

// PresetUpdatedEventData represents the payload for image.preset.updated.
type PresetUpdatedEventData Preset

// NewPresetUpdatedEvent creates a typed event for preset updates.
func NewPresetUpdatedEvent(resourceID string, presetUpdatedEventData PresetUpdatedEventData) core.Event {
	return core.NewEvent("image.preset.updated", presetUpdatedEventData).WithResourceID(resourceID)
}

// PresetDeletedEventData represents the payload for image.preset.deleted.
type PresetDeletedEventData struct {
	PresetName string `json:"preset_name"`
}

// NewPresetDeletedEvent creates a typed event for preset deletions.
func NewPresetDeletedEvent(resourceID string, presetDeletedEventData PresetDeletedEventData) core.Event {
	return core.NewEvent("image.preset.deleted", presetDeletedEventData).WithResourceID(resourceID)
}

// TransformCompletedEventData represents the payload for image.transform.completed.
type TransformCompletedEventData struct {
	SourceURL string `json:"source_url"`
	Format    string `json:"format"`
	ByteSize  int64  `json:"byte_size"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	CacheHit  bool   `json:"cache_hit"`
}

// NewTransformCompletedEvent creates a typed event for image transformation completion.
func NewTransformCompletedEvent(resourceID string, transformCompletedEventData TransformCompletedEventData) core.Event {
	return core.NewEvent("image.transform.completed", transformCompletedEventData).WithResourceID(resourceID)
}

// TransformFailedEventData represents the payload for image.transform.failed.
type TransformFailedEventData struct {
	SourceURL  string `json:"source_url"`
	Reason     string `json:"reason"`
	StatusCode int    `json:"status_code"`
}

// NewTransformFailedEvent creates a typed event for image transformation failures.
func NewTransformFailedEvent(resourceID string, transformFailedEventData TransformFailedEventData) core.Event {
	return core.NewEvent("image.transform.failed", transformFailedEventData).WithResourceID(resourceID)
}

// InspectCompletedEventData represents the payload for image.inspect.completed.
type InspectCompletedEventData struct {
	SourceURL string `json:"source_url"`
	Format    string `json:"format"`
	ByteSize  int64  `json:"byte_size"`
}

// NewInspectCompletedEvent creates a typed event for image metadata introspection.
func NewInspectCompletedEvent(resourceID string, inspectCompletedEventData InspectCompletedEventData) core.Event {
	return core.NewEvent("image.inspect.completed", inspectCompletedEventData).WithResourceID(resourceID)
}

// CacheFlushedEventData represents the payload for image.cache.flushed.
type CacheFlushedEventData struct {
	CacheKey  string `json:"cache_key,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
}

// NewCacheFlushedEvent creates a typed event for cache clearing / flushing.
func NewCacheFlushedEvent(resourceID string, cacheFlushedEventData CacheFlushedEventData) core.Event {
	return core.NewEvent("image.cache.flushed", cacheFlushedEventData).WithResourceID(resourceID)
}

// CacheInvalidatedEventData represents the payload for image.cache.invalidated.
type CacheInvalidatedEventData struct {
	CacheKey  string `json:"cache_key"`
	SourceURL string `json:"source_url,omitempty"`
}

// NewCacheInvalidatedEvent creates a typed event for targeted cache invalidation.
func NewCacheInvalidatedEvent(resourceID string, cacheInvalidatedEventData CacheInvalidatedEventData) core.Event {
	return core.NewEvent("image.cache.invalidated", cacheInvalidatedEventData).WithResourceID(resourceID)
}

// InspectFailedEventData represents the payload for image.inspect.failed.
type InspectFailedEventData struct {
	SourceURL  string `json:"source_url"`
	Reason     string `json:"reason"`
	StatusCode int    `json:"status_code"`
}

// NewInspectFailedEvent creates a typed event for image inspection failures.
func NewInspectFailedEvent(resourceID string, inspectFailedEventData InspectFailedEventData) core.Event {
	return core.NewEvent("image.inspect.failed", inspectFailedEventData).WithResourceID(resourceID)
}

// ThreatSSRFBlockedEventData represents the payload for image.threat.ssrf_blocked.
type ThreatSSRFBlockedEventData struct {
	SourceURL  string `json:"source_url"`
	ResolvedIP string `json:"resolved_ip,omitempty"`
	Reason     string `json:"reason"`
}

// NewThreatSSRFBlockedEvent creates a typed event for SSRF security threat mitigations.
func NewThreatSSRFBlockedEvent(resourceID string, threatSSRFBlockedEventData ThreatSSRFBlockedEventData) core.Event {
	return core.NewEvent("image.threat.ssrf_blocked", threatSSRFBlockedEventData).WithResourceID(resourceID)
}

// URLSignedEventData represents the payload for image.url.signed.
type URLSignedEventData struct {
	Path      string `json:"path"`
	URL       string `json:"url"`
	Signature string `json:"signature"`
}

// NewURLSignedEvent creates a typed event for cryptographic URL signings.
func NewURLSignedEvent(resourceID string, urlSignedEventData URLSignedEventData) core.Event {
	return core.NewEvent("image.url.signed", urlSignedEventData).WithResourceID(resourceID)
}
