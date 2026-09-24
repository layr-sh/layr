// Package data provides the core data service coordinator, REST auto-CRUD gateway, GraphQL engine, realtime CDC streaming, and DDL schema management.
package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"layr.sh/core"
)

// ConfigKey is the primary key in data.config table.
const ConfigKey = "runtime"

// ErrInvalidConfig is returned when runtime configuration validation fails.
var ErrInvalidConfig = errors.New("invalid data configuration")

// Config represents the dynamic runtime configuration for layr/data.
type Config struct {
	Schemas      []string           `json:"schemas"`
	REST         RESTConfig         `json:"rest"`
	GraphQL      GraphQLConfig      `json:"graphql"`
	Realtime     RealtimeConfig     `json:"realtime"`
	CORS         CORSConfig         `json:"cors"`
	Cache        CacheConfig        `json:"cache"`
	RateLimiting RateLimitingConfig `json:"rate_limiting"`
}

// CacheConfig defines dynamic query and schema catalog caching policies.
type CacheConfig struct {
	Enabled              bool `json:"enabled"`
	CatalogTTLSeconds    int  `json:"catalog_ttl_seconds"`
	QueryTTLSeconds      int  `json:"query_ttl_seconds"`
	MaxCachedQueries     int  `json:"max_cached_queries"`
	InvalidateOnMutation bool `json:"invalidate_on_mutation"`
	InvalidateOnCDC      bool `json:"invalidate_on_cdc"`
}

// RateLimitingConfig defines dynamic REST/GraphQL throughput policies.
type RateLimitingConfig struct {
	Enabled           bool `json:"enabled"`
	RequestsPerMinute int  `json:"requests_per_minute"`
	Burst             int  `json:"burst"`
}

// RESTConfig contains settings for the REST auto-CRUD gateway.
type RESTConfig struct {
	Enabled        bool     `json:"enabled"`
	MaxLimit       int      `json:"max_limit"`
	DefaultLimit   int      `json:"default_limit"`
	ExcludedTables []string `json:"excluded_tables"`
}

// GraphQLConfig contains settings for the GraphQL engine.
type GraphQLConfig struct {
	Enabled              bool `json:"enabled"`
	MaxDepth             int  `json:"max_depth"`
	MaxComplexity        int  `json:"max_complexity"`
	DefaultLimit         int  `json:"default_limit"`
	IntrospectionEnabled bool `json:"introspection_enabled"`
}

// RealtimeConfig contains settings for the WebSocket CDC engine.
type RealtimeConfig struct {
	Enabled                  bool `json:"enabled"`
	HeartbeatIntervalMS      int  `json:"heartbeat_interval_ms"`
	MaxChannelsPerConnection int  `json:"max_channels_per_connection"`
	MaxConnections           int  `json:"max_connections"`
}

// CORSConfig specifies allowed origins and headers.
type CORSConfig struct {
	AllowedOrigins []string `json:"allowed_origins"`
	AllowedHeaders []string `json:"allowed_headers"`
}

// DefaultConfig returns standard baseline configuration.
func DefaultConfig() Config {
	const (
		defaultRESTMaxLimit            = 1000
		defaultRESTDefaultLimit        = 50
		defaultGraphQLMaxDepth         = 8
		defaultGraphQLMaxComplexity    = 500
		defaultGraphQLDefaultLimit     = 100
		defaultRealtimeHeartbeatMS     = 30000
		defaultRealtimeMaxChannels     = 50
		defaultRealtimeMaxConnections  = 10000
		defaultCacheCatalogTTLSeconds  = 3600
		defaultCacheQueryTTLSeconds    = 30
		defaultCacheMaxQueries         = 10000
		defaultRateLimitRequestsPerMin = 600
		defaultRateLimitBurst          = 100
	)

	return Config{
		Schemas: []string{"public", "reference_data"},
		REST: RESTConfig{
			Enabled:        true,
			MaxLimit:       defaultRESTMaxLimit,
			DefaultLimit:   defaultRESTDefaultLimit,
			ExcludedTables: []string{"schema_migrations", "secrets"},
		},
		GraphQL: GraphQLConfig{
			Enabled:              true,
			MaxDepth:             defaultGraphQLMaxDepth,
			MaxComplexity:        defaultGraphQLMaxComplexity,
			DefaultLimit:         defaultGraphQLDefaultLimit,
			IntrospectionEnabled: true,
		},
		Realtime: RealtimeConfig{
			Enabled:                  true,
			HeartbeatIntervalMS:      defaultRealtimeHeartbeatMS,
			MaxChannelsPerConnection: defaultRealtimeMaxChannels,
			MaxConnections:           defaultRealtimeMaxConnections,
		},
		CORS: CORSConfig{
			AllowedOrigins: []string{"*"},
			AllowedHeaders: []string{"Authorization", "Content-Type", "X-Custom-Header"},
		},
		Cache: CacheConfig{
			Enabled:              true,
			CatalogTTLSeconds:    defaultCacheCatalogTTLSeconds,
			QueryTTLSeconds:      defaultCacheQueryTTLSeconds,
			MaxCachedQueries:     defaultCacheMaxQueries,
			InvalidateOnMutation: true,
			InvalidateOnCDC:      true,
		},
		RateLimiting: RateLimitingConfig{
			Enabled:           false,
			RequestsPerMinute: defaultRateLimitRequestsPerMin,
			Burst:             defaultRateLimitBurst,
		},
	}
}

// Validate verifies whether the configuration settings are valid.
func (config Config) Validate() error {
	if config.REST.MaxLimit <= 0 {
		return fmt.Errorf("rest max_limit must be greater than 0, got %d", config.REST.MaxLimit)
	}
	if config.REST.DefaultLimit <= 0 {
		return fmt.Errorf("rest default_limit must be greater than 0, got %d", config.REST.DefaultLimit)
	}
	if config.GraphQL.MaxDepth <= 0 {
		return fmt.Errorf("graphql max_depth must be greater than 0, got %d", config.GraphQL.MaxDepth)
	}
	if config.GraphQL.MaxComplexity <= 0 {
		return fmt.Errorf("graphql max_complexity must be greater than 0, got %d", config.GraphQL.MaxComplexity)
	}
	if config.Realtime.HeartbeatIntervalMS <= 0 {
		return fmt.Errorf("realtime heartbeat_interval_ms must be greater than 0, got %d", config.Realtime.HeartbeatIntervalMS)
	}
	if config.Realtime.MaxChannelsPerConnection <= 0 {
		return fmt.Errorf("realtime max_channels_per_connection must be greater than 0, got %d", config.Realtime.MaxChannelsPerConnection)
	}
	return nil
}

// ConfigManager handles in-memory caching and PostgreSQL synchronization for data.config.
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

// Load fetches the runtime config from PostgreSQL or initializes the default if not present.
func (configManager *ConfigManager) Load(ctx context.Context) error {
	var rawJSON []byte
	const selectSQLStatement = `SELECT value FROM data.config WHERE key = $1`
	scanErr := configManager.kernel.DB().QueryRow(ctx, selectSQLStatement, ConfigKey).Scan(&rawJSON)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			defaultConfig := DefaultConfig()
			return configManager.Set(ctx, defaultConfig)
		}
		return fmt.Errorf("failed to query data.config: %w", scanErr)
	}

	var parsedConfig Config
	if unmarshalErr := json.Unmarshal(rawJSON, &parsedConfig); unmarshalErr != nil {
		return fmt.Errorf("failed to parse dynamic data config JSON: %w", unmarshalErr)
	}

	if err := parsedConfig.Validate(); err != nil {
		return fmt.Errorf("stored data config is invalid: %w", err)
	}

	configManager.SetMemoryConfig(parsedConfig)
	return nil
}

// SetMemoryConfig updates the in-memory configuration directly without DB persistence.
func (configManager *ConfigManager) SetMemoryConfig(config Config) {
	configManager.rwMutex.Lock()
	defer configManager.rwMutex.Unlock()
	configManager.config = config
}

// Set writes new configuration to PostgreSQL, updates the in-memory cache, and publishes event.
func (configManager *ConfigManager) Set(ctx context.Context, config Config) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	if len(config.Schemas) == 0 {
		config.Schemas = []string{"public", "reference_data"}
	}

	configJSON, _ := json.Marshal(config)

	const upsertSQLStatement = `
		INSERT INTO data.config (key, value, last_updated_at)
		VALUES ($1, $2::jsonb, clock_timestamp())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value, last_updated_at = clock_timestamp();
	`
	if _, execErr := configManager.kernel.DB().Exec(ctx, upsertSQLStatement, ConfigKey, configJSON); execErr != nil {
		return fmt.Errorf("failed to save data config to database: %w", execErr)
	}

	configManager.SetMemoryConfig(config)
	if configManager.kernel != nil && configManager.kernel.EventBus() != nil {
		configManager.kernel.EventBus().Publish(ctx, NewConfigUpdatedEvent("data.config", ConfigUpdatedEventData(config)))
	}
	return nil
}
