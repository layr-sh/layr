// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

// ConfigKey is the primary key used in tasks.config.
const ConfigKey = "runtime"

// ErrInvalidConfig is returned when runtime configuration validation fails.
var ErrInvalidConfig = errors.New("invalid tasks configuration")

const (
	defaultConcurrencyLimit         = 25
	defaultTimeoutSeconds           = 30
	defaultPollIntervalMs           = 1000
	defaultRetryInitialDelaySeconds = 5
	defaultRetryMaxDelaySeconds     = 300
	defaultRetryMaxAttempts         = 5
)

// Config represents dynamic operational configuration for the tasks service.
type Config struct {
	ConcurrencyLimit         int `json:"concurrency_limit"`
	TimeoutSeconds           int `json:"timeout_seconds"`
	PollIntervalMs           int `json:"poll_interval_ms"`
	RetryInitialDelaySeconds int `json:"retry_initial_delay_seconds"`
	RetryMaxDelaySeconds     int `json:"retry_max_delay_seconds"`
	RetryMaxAttempts         int `json:"retry_max_attempts"`
}

// DefaultConfig returns the default operational parameters.
func DefaultConfig() Config {
	return Config{
		ConcurrencyLimit:         defaultConcurrencyLimit,
		TimeoutSeconds:           defaultTimeoutSeconds,
		PollIntervalMs:           defaultPollIntervalMs,
		RetryInitialDelaySeconds: defaultRetryInitialDelaySeconds,
		RetryMaxDelaySeconds:     defaultRetryMaxDelaySeconds,
		RetryMaxAttempts:         defaultRetryMaxAttempts,
	}
}

// Validate checks configuration field constraints.
func (config Config) Validate() error {
	if config.ConcurrencyLimit < 1 || config.ConcurrencyLimit > 1000 {
		return errors.New("concurrency_limit must be between 1 and 1000")
	}
	if config.TimeoutSeconds < 1 || config.TimeoutSeconds > 3600 {
		return errors.New("timeout_seconds must be between 1 and 3600")
	}
	if config.PollIntervalMs < 100 || config.PollIntervalMs > 60000 {
		return errors.New("poll_interval_ms must be between 100 and 60000")
	}
	if config.RetryInitialDelaySeconds < 1 || config.RetryInitialDelaySeconds > 3600 {
		return errors.New("retry_initial_delay_seconds must be between 1 and 3600")
	}
	if config.RetryMaxDelaySeconds < config.RetryInitialDelaySeconds || config.RetryMaxDelaySeconds > 86400 {
		return errors.New("retry_max_delay_seconds must be greater than or equal to retry_initial_delay_seconds and at most 86400")
	}
	if config.RetryMaxAttempts < 1 || config.RetryMaxAttempts > 20 {
		return errors.New("retry_max_attempts must be between 1 and 20")
	}
	return nil
}

// ConfigManager handles in-memory caching and PostgreSQL persistence of tasks configuration.
type ConfigManager struct {
	kernel  *core.Kernel
	rwMutex sync.RWMutex
	config  Config
}

// NewConfigManager initializes a new ConfigManager.
func NewConfigManager(kernel *core.Kernel) *ConfigManager {
	return &ConfigManager{
		kernel: kernel,
		config: DefaultConfig(),
	}
}

// Get returns a thread-safe copy of the active tasks configuration.
func (configManager *ConfigManager) Get() Config {
	configManager.rwMutex.RLock()
	defer configManager.rwMutex.RUnlock()
	return configManager.config
}

// SetMemoryConfig updates the in-memory configuration directly (for testing).
func (configManager *ConfigManager) SetMemoryConfig(config Config) {
	configManager.rwMutex.Lock()
	defer configManager.rwMutex.Unlock()
	configManager.config = config
}

// Load fetches tasks configuration from PostgreSQL or seeds defaults.
func (configManager *ConfigManager) Load(ctx context.Context) error {
	log.Debug("loading tasks configuration from database")
	var rawJSON []byte
	const selectSQLStatement = `SELECT value FROM tasks.config WHERE key = $1`
	err := configManager.kernel.DB().QueryRow(ctx, selectSQLStatement, ConfigKey).Scan(&rawJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			defaultConfig := DefaultConfig()
			return configManager.Set(ctx, defaultConfig)
		}
		return fmt.Errorf("failed to query tasks.config: %w", err)
	}

	var loadedConfig Config
	if unmarshalErr := json.Unmarshal(rawJSON, &loadedConfig); unmarshalErr != nil {
		return fmt.Errorf("failed to parse stored tasks config: %w", unmarshalErr)
	}

	if validateErr := loadedConfig.Validate(); validateErr != nil {
		return fmt.Errorf("stored tasks config is invalid: %w", validateErr)
	}

	configManager.SetMemoryConfig(loadedConfig)
	return nil
}

// Set persists new configuration into PostgreSQL and updates memory.
func (configManager *ConfigManager) Set(ctx context.Context, newConfig Config) error {
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	configBytes, _ := json.Marshal(newConfig)

	const upsertSQLStatement = `
		INSERT INTO tasks.config (key, value, last_updated_at)
		VALUES ($1, $2, clock_timestamp())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, last_updated_at = clock_timestamp();
	`
	_, execErr := configManager.kernel.DB().Exec(ctx, upsertSQLStatement, ConfigKey, configBytes)
	if execErr != nil {
		return fmt.Errorf("failed to persist tasks config: %w", execErr)
	}

	configManager.SetMemoryConfig(newConfig)
	if configManager.kernel != nil && configManager.kernel.EventBus() != nil {
		configManager.kernel.EventBus().Publish(ctx, NewConfigUpdatedEvent("tasks.config", ConfigUpdatedEventData(newConfig)))
	}
	return nil
}
