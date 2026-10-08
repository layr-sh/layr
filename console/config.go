// Package console provides the administrative control plane, operator identity, and dynamic configuration engine.
package console

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

// ConfigKey is the primary key used in console.config.
const ConfigKey = "runtime"

// Default operational parameters for console configuration.
const (
	DefaultSessionIdleTimeoutSeconds = 1800  // 30 minutes
	DefaultSessionMaxLifetimeSeconds = 86400 // 24 hours
	DefaultAuditRetentionDays        = 90    // 90 days
)

// ErrInvalidConfig is returned when runtime configuration validation fails.
var ErrInvalidConfig = errors.New("invalid console configuration")

// Config represents dynamic operational configuration for the console control plane.
type Config struct {
	SessionIdleTimeoutSeconds int      `json:"session_idle_timeout_seconds"`
	SessionMaxLifetimeSeconds int      `json:"session_max_lifetime_seconds"`
	MFAEnforced               bool     `json:"mfa_enforced"`
	AllowedCIDRs              []string `json:"allowed_cidrs"`
	AuditRetentionDays        int      `json:"audit_retention_days"`
}

// DefaultConfig returns the default operational configuration for the console service.
func DefaultConfig() Config {
	return Config{
		SessionIdleTimeoutSeconds: DefaultSessionIdleTimeoutSeconds,
		SessionMaxLifetimeSeconds: DefaultSessionMaxLifetimeSeconds,
		MFAEnforced:               false,
		AllowedCIDRs:              []string{},
		AuditRetentionDays:        DefaultAuditRetentionDays,
	}
}

// Validate checks configuration field constraints.
func (config Config) Validate() error {
	if config.SessionIdleTimeoutSeconds <= 0 {
		return errors.New("session_idle_timeout_seconds must be positive")
	}
	if config.SessionMaxLifetimeSeconds <= 0 {
		return errors.New("session_max_lifetime_seconds must be positive")
	}
	if config.SessionIdleTimeoutSeconds > config.SessionMaxLifetimeSeconds {
		return errors.New("session_idle_timeout_seconds cannot exceed session_max_lifetime_seconds")
	}
	if config.AuditRetentionDays <= 0 {
		return errors.New("audit_retention_days must be positive")
	}
	for _, cidr := range config.AllowedCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			if net.ParseIP(cidr) == nil {
				return fmt.Errorf("invalid CIDR or IP address: %s", cidr)
			}
		}
	}
	return nil
}

// ConfigManager handles in-memory caching and PostgreSQL persistence of console configuration.
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

// Get returns a thread-safe copy of the active console configuration.
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

// Load fetches console configuration from PostgreSQL or seeds defaults.
func (configManager *ConfigManager) Load(ctx context.Context) error {
	log.Debug("loading console configuration from database")
	var rawJSON []byte
	const selectSQLStatement = `SELECT value FROM console.config WHERE key = $1`
	err := configManager.kernel.DB().QueryRow(ctx, selectSQLStatement, ConfigKey).Scan(&rawJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			defaultConfig := DefaultConfig()
			return configManager.Set(ctx, defaultConfig)
		}
		return fmt.Errorf("failed to query console.config: %w", err)
	}

	var loadedConfig Config
	if unmarshalErr := json.Unmarshal(rawJSON, &loadedConfig); unmarshalErr != nil {
		return fmt.Errorf("failed to parse stored console config: %w", unmarshalErr)
	}

	if validateErr := loadedConfig.Validate(); validateErr != nil {
		return fmt.Errorf("stored console config is invalid: %w", validateErr)
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
		INSERT INTO console.config (key, value, last_updated_at)
		VALUES ($1, $2, clock_timestamp())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, last_updated_at = clock_timestamp();
	`
	_, execErr := configManager.kernel.DB().Exec(ctx, upsertSQLStatement, ConfigKey, configBytes)
	if execErr != nil {
		return fmt.Errorf("failed to persist console config: %w", execErr)
	}

	configManager.SetMemoryConfig(newConfig)
	configManager.kernel.EventBus().Publish(ctx, NewConfigUpdatedEvent("console.config", ConfigUpdatedEventData(newConfig)))
	return nil
}
