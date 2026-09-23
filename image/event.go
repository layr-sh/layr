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

// TransformedEventData represents the payload for image.transformed.
type TransformedEventData struct {
	SourceURL string `json:"source_url"`
	Format    string `json:"format"`
	ByteSize  int64  `json:"byte_size"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	CacheHit  bool   `json:"cache_hit"`
}

// NewTransformedEvent creates a typed event for image transformation.
func NewTransformedEvent(resourceID string, transformedEventData TransformedEventData) core.Event {
	return core.NewEvent("image.transformed", transformedEventData).WithResourceID(resourceID)
}

// TransformFailedEventData represents the payload for image.transform_failed.
type TransformFailedEventData struct {
	SourceURL  string `json:"source_url"`
	Reason     string `json:"reason"`
	StatusCode int    `json:"status_code"`
}

// NewTransformFailedEvent creates a typed event for image transformation failures.
func NewTransformFailedEvent(resourceID string, transformFailedEventData TransformFailedEventData) core.Event {
	return core.NewEvent("image.transform_failed", transformFailedEventData).WithResourceID(resourceID)
}

// InspectedEventData represents the payload for image.inspected.
type InspectedEventData struct {
	SourceURL string `json:"source_url"`
	Format    string `json:"format"`
	ByteSize  int64  `json:"byte_size"`
}

// NewInspectedEvent creates a typed event for image metadata introspection.
func NewInspectedEvent(resourceID string, inspectedEventData InspectedEventData) core.Event {
	return core.NewEvent("image.inspected", inspectedEventData).WithResourceID(resourceID)
}

// CacheClearedEventData represents the payload for image.cache.cleared.
type CacheClearedEventData struct {
	CacheKey  string `json:"cache_key,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
}

// NewCacheClearedEvent creates a typed event for cache clearing.
func NewCacheClearedEvent(resourceID string, cacheClearedEventData CacheClearedEventData) core.Event {
	return core.NewEvent("image.cache.cleared", cacheClearedEventData).WithResourceID(resourceID)
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

// InspectFailedEventData represents the payload for image.inspect_failed.
type InspectFailedEventData struct {
	SourceURL  string `json:"source_url"`
	Reason     string `json:"reason"`
	StatusCode int    `json:"status_code"`
}

// NewInspectFailedEvent creates a typed event for image inspection failures.
func NewInspectFailedEvent(resourceID string, inspectFailedEventData InspectFailedEventData) core.Event {
	return core.NewEvent("image.inspect_failed", inspectFailedEventData).WithResourceID(resourceID)
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
