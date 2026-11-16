package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/jackc/pgx/v5"
)

// ConfigKey is the primary key in core.config.
const ConfigKey = "runtime"

// DefaultAuditRetentionDays is the default retention period for platform audit events.
const DefaultAuditRetentionDays = 90

// ErrInvalidConfig is returned when runtime configuration validation fails.
var ErrInvalidConfig = errors.New("invalid core configuration")

// RuntimeConfig contains dynamic Core settings, separately from the startup Config.
type RuntimeConfig struct {
	AllowedCIDRs       []string `json:"allowed_cidrs"`
	AuditRetentionDays int      `json:"audit_retention_days"`
}

// DefaultRuntimeConfig returns the default Core runtime configuration.
func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{AllowedCIDRs: []string{}, AuditRetentionDays: DefaultAuditRetentionDays}
}

// Validate checks runtime configuration constraints.
func (runtimeConfig RuntimeConfig) Validate() error {
	if runtimeConfig.AuditRetentionDays <= 0 {
		return errors.New("audit_retention_days must be positive")
	}
	for _, cidr := range runtimeConfig.AllowedCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			if net.ParseIP(cidr) == nil {
				return fmt.Errorf("invalid CIDR or IP address: %s", cidr)
			}
		}
	}
	return nil
}

// ConfigManager caches and persists Core runtime configuration.
type ConfigManager struct {
	kernel        *Kernel
	rwMutex       sync.RWMutex
	runtimeConfig RuntimeConfig
}

// NewConfigManager initializes the Core runtime configuration manager.
func NewConfigManager(kernel *Kernel) *ConfigManager {
	return &ConfigManager{kernel: kernel, runtimeConfig: DefaultRuntimeConfig()}
}

// Get returns the active runtime configuration.
func (configManager *ConfigManager) Get() RuntimeConfig {
	configManager.rwMutex.RLock()
	defer configManager.rwMutex.RUnlock()
	return configManager.runtimeConfig
}

// SetMemoryConfig updates the cached runtime configuration.
func (configManager *ConfigManager) SetMemoryConfig(runtimeConfig RuntimeConfig) {
	configManager.rwMutex.Lock()
	defer configManager.rwMutex.Unlock()
	configManager.runtimeConfig = runtimeConfig
}

// Load reads and validates persisted configuration or seeds defaults.
func (configManager *ConfigManager) Load(ctx context.Context) error {
	log.Debug("loading core runtime configuration from database")
	var rawJSON []byte
	err := configManager.kernel.DB().QueryRow(ctx, `SELECT value FROM core.config WHERE key = $1`, ConfigKey).Scan(&rawJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return configManager.Set(ctx, DefaultRuntimeConfig())
		}
		return fmt.Errorf("failed to query core.config: %w", err)
	}
	var runtimeConfig RuntimeConfig
	if err := json.Unmarshal(rawJSON, &runtimeConfig); err != nil {
		return fmt.Errorf("failed to parse stored core config: %w", err)
	}
	if err := runtimeConfig.Validate(); err != nil {
		return fmt.Errorf("stored core config is invalid: %w", err)
	}
	configManager.SetMemoryConfig(runtimeConfig)
	return nil
}

// Set validates, persists, and applies new Core runtime configuration.
func (configManager *ConfigManager) Set(ctx context.Context, runtimeConfig RuntimeConfig) error {
	if err := runtimeConfig.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}
	value, _ := json.Marshal(runtimeConfig)
	_, err := configManager.kernel.DB().Exec(ctx, `
		INSERT INTO core.config(key, value, last_updated_at) VALUES ($1, $2, clock_timestamp())
		ON CONFLICT(key) DO UPDATE SET value = EXCLUDED.value, last_updated_at = clock_timestamp()
	`, ConfigKey, value)
	if err != nil {
		return fmt.Errorf("failed to persist core config: %w", err)
	}
	configManager.SetMemoryConfig(runtimeConfig)
	configManager.kernel.EventBus().Publish(ctx, NewConfigUpdatedEvent("", ConfigUpdatedEventData(runtimeConfig)))
	return nil
}
