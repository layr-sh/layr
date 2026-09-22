package filestorage

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"layr.sh/core"
)

// ConfigKey is the primary key used in file_storage.config.
const ConfigKey = "runtime"

// Named default constants for file storage configuration.
const (
	DefaultMaxFileSizeBytes          int64 = 52428800 // 50MB
	DefaultChunkSizeBytes            int   = 524288   // 512KB
	DefaultPresignTokenExpirySeconds int   = 3600     // 1 hour
)

// Config represents the dynamic runtime configuration for layr/file-storage.
type Config struct {
	Enabled                   bool     `json:"enabled"`
	DefaultMaxFileSizeBytes   int64    `json:"default_max_file_size_bytes"`
	DefaultAllowedMIMETypes   []string `json:"default_allowed_mime_types"`
	ChunkSizeBytes            int      `json:"chunk_size_bytes"`
	PresignTokenExpirySeconds int      `json:"presign_token_expiry_seconds"`
}

// DefaultConfig returns standard baseline configuration for file storage.
func DefaultConfig() Config {
	return Config{
		Enabled:                   true,
		DefaultMaxFileSizeBytes:   DefaultMaxFileSizeBytes,
		DefaultAllowedMIMETypes:   []string{},
		ChunkSizeBytes:            DefaultChunkSizeBytes,
		PresignTokenExpirySeconds: DefaultPresignTokenExpirySeconds,
	}
}

// Validate verifies whether the configuration settings are valid.
func (config Config) Validate() error {
	if config.DefaultMaxFileSizeBytes <= 0 {
		return fmt.Errorf("default_max_file_size_bytes must be greater than 0, got %d", config.DefaultMaxFileSizeBytes)
	}
	if config.ChunkSizeBytes <= 0 {
		return fmt.Errorf("chunk_size_bytes must be greater than 0, got %d", config.ChunkSizeBytes)
	}
	if config.PresignTokenExpirySeconds <= 0 {
		return fmt.Errorf("presign_token_expiry_seconds must be greater than 0, got %d", config.PresignTokenExpirySeconds)
	}
	return nil
}

// ConfigManager handles in-memory caching and PostgreSQL synchronization for file_storage.config.
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

// Load fetches the runtime config from PostgreSQL or initializes the default if not present.
func (configManager *ConfigManager) Load(ctx context.Context) error {
	var rawJSON []byte
	const selectSQLStatement = `SELECT value FROM file_storage.config WHERE key = $1`
	scanErr := configManager.kernel.DB().QueryRow(ctx, selectSQLStatement, ConfigKey).Scan(&rawJSON)
	if scanErr != nil {
		defaultConfig := DefaultConfig()
		return configManager.Set(ctx, defaultConfig)
	}

	var parsedConfig Config
	if unmarshalErr := json.Unmarshal(rawJSON, &parsedConfig); unmarshalErr != nil {
		return fmt.Errorf("failed to parse dynamic file storage config JSON: %w", unmarshalErr)
	}

	configManager.rwMutex.Lock()
	configManager.config = parsedConfig
	configManager.rwMutex.Unlock()
	return nil
}

// Set saves the updated configuration to PostgreSQL and updates the in-memory cache.
func (configManager *ConfigManager) Set(ctx context.Context, config Config) error {
	if validateErr := config.Validate(); validateErr != nil {
		return validateErr
	}

	serializedJSON, _ := json.Marshal(config)

	const upsertSQLStatement = `
		INSERT INTO file_storage.config (key, value, last_updated_at)
		VALUES ($1, $2, clock_timestamp())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value,
		    last_updated_at = EXCLUDED.last_updated_at;
	`
	_, execErr := configManager.kernel.DB().Exec(ctx, upsertSQLStatement, ConfigKey, serializedJSON)
	if execErr != nil {
		return fmt.Errorf("failed to persist dynamic file storage config: %w", execErr)
	}

	configManager.rwMutex.Lock()
	configManager.config = config
	configManager.rwMutex.Unlock()
	return nil
}
