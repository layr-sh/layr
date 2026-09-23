package image

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"layr.sh/core"
)

// ConfigKey is the primary key used in image.config.
const ConfigKey = "runtime"

// Named default constants for image transformation configuration.
const (
	DefaultQuality            = 80
	DefaultMaxSrcResolution   = 50 // In Megapixels (MP)
	DefaultMaxAnimationFrames = 128
	DefaultCacheTTLSeconds    = 86400 // 24 hours
	DefaultWatermarkOpacity   = 0.5
)

// Config represents the dynamic operational runtime configuration for layr/image.
type Config struct {
	AllowInsecure      bool     `json:"allow_insecure"`
	AllowedDomains     []string `json:"allowed_domains"`
	DefaultQuality     int      `json:"default_quality"`
	MaxSrcResolution   int      `json:"max_src_resolution"`
	MaxAnimationFrames int      `json:"max_animation_frames"`
	JPEGProgressive    bool     `json:"jpeg_progressive"`
	PNGInterlaced      bool     `json:"png_interlaced"`
	StripMetadata      bool     `json:"strip_metadata"`
	KeepCopyright      bool     `json:"keep_copyright"`
	StripColorProfile  bool     `json:"strip_color_profile"`
	CacheTTLSeconds    int      `json:"cache_ttl_seconds"`
	WatermarkURL       string   `json:"watermark_url"`
	WatermarkOpacity   float64  `json:"watermark_opacity"`
}

// DefaultConfig returns standard baseline configuration for image processing.
func DefaultConfig() Config {
	return Config{
		AllowInsecure:      false,
		AllowedDomains:     []string{},
		DefaultQuality:     DefaultQuality,
		MaxSrcResolution:   DefaultMaxSrcResolution,
		MaxAnimationFrames: DefaultMaxAnimationFrames,
		JPEGProgressive:    true,
		PNGInterlaced:      false,
		StripMetadata:      true,
		KeepCopyright:      true,
		StripColorProfile:  true,
		CacheTTLSeconds:    DefaultCacheTTLSeconds,
		WatermarkURL:       "",
		WatermarkOpacity:   DefaultWatermarkOpacity,
	}
}

// Validate verifies whether the configuration settings are valid.
func (config Config) Validate() error {
	if config.DefaultQuality < 1 || config.DefaultQuality > 100 {
		return fmt.Errorf("default_quality must be between 1 and 100, got %d", config.DefaultQuality)
	}
	if config.MaxSrcResolution <= 0 {
		return fmt.Errorf("max_src_resolution must be greater than 0, got %d", config.MaxSrcResolution)
	}
	if config.MaxAnimationFrames <= 0 {
		return fmt.Errorf("max_animation_frames must be greater than 0, got %d", config.MaxAnimationFrames)
	}
	if config.CacheTTLSeconds < 0 {
		return fmt.Errorf("cache_ttl_seconds must be non-negative, got %d", config.CacheTTLSeconds)
	}
	if config.WatermarkOpacity < 0.0 || config.WatermarkOpacity > 1.0 {
		return fmt.Errorf("watermark_opacity must be between 0.0 and 1.0, got %f", config.WatermarkOpacity)
	}
	return nil
}

// ConfigManager handles in-memory caching and PostgreSQL synchronization for image.config.
type ConfigManager struct {
	kernel  *core.Kernel
	rwMutex sync.RWMutex
	config  Config
}

// NewConfigManager initializes a new dynamic configuration manager.
func NewConfigManager(kernel *core.Kernel) *ConfigManager {
	return &ConfigManager{
		kernel: kernel,
		config: DefaultConfig(),
	}
}

// Get returns the current active configuration snapshot in memory.
func (configManager *ConfigManager) Get() Config {
	configManager.rwMutex.RLock()
	defer configManager.rwMutex.RUnlock()
	return configManager.config
}

// SetMemoryConfig updates the in-memory configuration directly without DB persistence.
func (configManager *ConfigManager) SetMemoryConfig(config Config) {
	configManager.rwMutex.Lock()
	defer configManager.rwMutex.Unlock()
	configManager.config = config
}

// SigningKey returns the 32-byte binary HMAC key derived from the master encryption key.
func (configManager *ConfigManager) SigningKey() []byte {
	return configManager.kernel.CryptoKeyManager().DeriveSubkey(core.CryptoContextImageSigningKey)
}

// SigningSalt returns the 32-byte binary HMAC salt derived from the master encryption key.
func (configManager *ConfigManager) SigningSalt() []byte {
	return configManager.kernel.CryptoKeyManager().DeriveSubkey(core.CryptoContextImageSigningSalt)
}

// Load fetches the runtime config from PostgreSQL or initializes the default if not present.
func (configManager *ConfigManager) Load(ctx context.Context) error {
	var rawJSON []byte
	const selectSQLStatement = `SELECT value FROM image.config WHERE key = $1`
	scanErr := configManager.kernel.DB().QueryRow(ctx, selectSQLStatement, ConfigKey).Scan(&rawJSON)
	if scanErr != nil {
		defaultConfig := DefaultConfig()
		return configManager.Set(ctx, defaultConfig)
	}

	var parsedConfig Config
	if unmarshalErr := json.Unmarshal(rawJSON, &parsedConfig); unmarshalErr != nil {
		return fmt.Errorf("failed to parse dynamic image config JSON: %w", unmarshalErr)
	}

	configManager.rwMutex.Lock()
	configManager.config = parsedConfig
	configManager.rwMutex.Unlock()

	return nil
}

// Set writes dynamic image configuration to PostgreSQL and updates lifetime memory.
func (configManager *ConfigManager) Set(ctx context.Context, newConfig Config) error {
	if validateErr := newConfig.Validate(); validateErr != nil {
		return fmt.Errorf("invalid image configuration: %w", validateErr)
	}

	rawJSON, _ := json.Marshal(newConfig)

	const upsertSQLStatement = `
		INSERT INTO image.config (key, value, last_updated_at)
		VALUES ($1, $2, clock_timestamp())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value, last_updated_at = clock_timestamp();
	`

	_, execErr := configManager.kernel.DB().Exec(ctx, upsertSQLStatement, ConfigKey, rawJSON)
	if execErr != nil {
		return fmt.Errorf("failed to save image config to database: %w", execErr)
	}

	configManager.rwMutex.Lock()
	configManager.config = newConfig
	configManager.rwMutex.Unlock()

	return nil
}
