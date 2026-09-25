package function

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

// ConfigKey is the primary key used in function.config.
const ConfigKey = "runtime"

// ErrInvalidConfig is returned when runtime configuration validation fails.
var ErrInvalidConfig = errors.New("invalid function configuration")

// Named default constants for function runtime configuration.
const (
	DefaultRuntime                  = "workerd"
	DefaultMemoryLimitMB            = 128
	DefaultTimeoutSeconds           = 30
	DefaultMaxBundleSizeBytes       = 25 * 1024 * 1024 // 25 MB
	DefaultWorkerdCompatibilityDate = "2026-08-04"
)

// DefaultWorkerdCompatibilityFlags defines standard compatibility flags for workerd.
var DefaultWorkerdCompatibilityFlags = []string{"nodejs_compat"}

// WorkerdRuntimeConfig defines runtime configuration for workerd runners and deployments.
type WorkerdRuntimeConfig struct {
	CompatibilityDate  string   `json:"compatibility_date,omitempty"`
	CompatibilityFlags []string `json:"compatibility_flags,omitempty"`
}

// Config represents the dynamic operational runtime configuration for layr/function.
type Config struct {
	DefaultRuntime         string               `json:"default_runtime"`
	DefaultMemoryLimitMB   int                  `json:"default_memory_limit_mb"`
	DefaultTimeoutSeconds  int                  `json:"default_timeout_seconds"`
	MaxBundleSizeBytes     int64                `json:"max_bundle_size_bytes"`
	AllowedEnvironmentKeys []string             `json:"allowed_environment_keys"`
	WorkerdRuntime         WorkerdRuntimeConfig `json:"workerd_runtime"`
}

// DefaultConfig returns standard baseline configuration for functions.
func DefaultConfig() Config {
	return Config{
		DefaultRuntime:         DefaultRuntime,
		DefaultMemoryLimitMB:   DefaultMemoryLimitMB,
		DefaultTimeoutSeconds:  DefaultTimeoutSeconds,
		MaxBundleSizeBytes:     DefaultMaxBundleSizeBytes,
		AllowedEnvironmentKeys: []string{},
		WorkerdRuntime: WorkerdRuntimeConfig{
			CompatibilityDate:  DefaultWorkerdCompatibilityDate,
			CompatibilityFlags: append([]string(nil), DefaultWorkerdCompatibilityFlags...),
		},
	}
}

// Validate verifies whether the configuration settings are valid.
func (config Config) Validate() error {
	if config.DefaultRuntime == "" {
		return errors.New("default_runtime cannot be empty")
	}
	if config.DefaultMemoryLimitMB <= 0 {
		return fmt.Errorf("default_memory_limit_mb must be greater than 0, got %d", config.DefaultMemoryLimitMB)
	}
	if config.DefaultTimeoutSeconds <= 0 {
		return fmt.Errorf("default_timeout_seconds must be greater than 0, got %d", config.DefaultTimeoutSeconds)
	}
	if config.MaxBundleSizeBytes <= 0 {
		return fmt.Errorf("max_bundle_size_bytes must be greater than 0, got %d", config.MaxBundleSizeBytes)
	}
	if config.WorkerdRuntime.CompatibilityDate == "" {
		return errors.New("workerd_runtime.compatibility_date cannot be empty")
	}
	return nil
}

// ConfigManager handles in-memory caching and PostgreSQL synchronization for function.config.
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
	log.Debug("loading function configuration from database")
	var rawJSON []byte
	const selectSQLStatement = `SELECT value FROM function.config WHERE key = $1`
	scanErr := configManager.kernel.DB().QueryRow(ctx, selectSQLStatement, ConfigKey).Scan(&rawJSON)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			defaultConfig := DefaultConfig()
			return configManager.Set(ctx, defaultConfig)
		}
		return fmt.Errorf("failed to query function.config: %w", scanErr)
	}

	var parsedConfig Config
	if unmarshalErr := json.Unmarshal(rawJSON, &parsedConfig); unmarshalErr != nil {
		return fmt.Errorf("failed to parse dynamic function config JSON: %w", unmarshalErr)
	}

	if err := parsedConfig.Validate(); err != nil {
		return fmt.Errorf("stored function config is invalid: %w", err)
	}

	configManager.SetMemoryConfig(parsedConfig)
	return nil
}

// Set writes dynamic function configuration to PostgreSQL and updates lifetime memory.
func (configManager *ConfigManager) Set(ctx context.Context, newConfig Config) error {
	if validateErr := newConfig.Validate(); validateErr != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConfig, validateErr)
	}

	rawJSON, _ := json.Marshal(newConfig)

	const upsertSQLStatement = `
		INSERT INTO function.config (key, value, last_updated_at)
		VALUES ($1, $2, clock_timestamp())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value, last_updated_at = clock_timestamp();
	`

	_, execErr := configManager.kernel.DB().Exec(ctx, upsertSQLStatement, ConfigKey, rawJSON)
	if execErr != nil {
		return fmt.Errorf("failed to save function config to database: %w", execErr)
	}

	configManager.SetMemoryConfig(newConfig)
	configManager.kernel.EventBus().Publish(ctx, NewConfigUpdatedEvent("function.config", ConfigUpdatedEventData(newConfig)))

	return nil
}
