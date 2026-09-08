// Package core provides configuration parsing, validation, and environment overrides for Layr.
package core

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ConfigFailureExitCode is the exit code for configuration failures according to sysexits.h
const ConfigFailureExitCode = 78

// Config represents the Tier 1 configuration (layr.yaml + LAYR__ env vars).
type Config struct {
	Version      string         `yaml:"version"`
	Project      ProjectConfig  `yaml:"project"`
	Server       ServerConfig   `yaml:"server"`
	Database     DatabaseConfig `yaml:"database"`
	Security     SecurityConfig `yaml:"security"`
	KVStore      KVStoreConfig  `yaml:"kv_store"`
	Data         ServiceConfig  `yaml:"data"`
	Auth         ServiceConfig  `yaml:"auth"`
	FileStorage  ServiceConfig  `yaml:"file_storage"`
	Tasks        ServiceConfig  `yaml:"tasks"`
	Notification ServiceConfig  `yaml:"notification"`
	Analytics    ServiceConfig  `yaml:"analytics"`
	Image        ServiceConfig  `yaml:"image"`
	Console      ConsoleConfig  `yaml:"console"`
}

// ProjectConfig holds general project metadata.
type ProjectConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Slug returns the URL/identifier-friendly slug of the project name.
func (project ProjectConfig) Slug() string {
	slugifier := NewSlugifier()
	slug := slugifier.Slugify(project.Name)
	if slug == "" {
		return "layr-app"
	}
	return slug
}

// ServerConfig defines the HTTP server network listener and base URL.
type ServerConfig struct {
	ListenAddr string `yaml:"listen_addr"`
	BaseURL    string `yaml:"base_url"`
}

// DatabaseConfig defines PostgreSQL connection and pool parameters.
type DatabaseConfig struct {
	URL                 string `yaml:"url"`                    // postgres://... or .layr/data
	MaxConnections      int    `yaml:"max_connections"`        // Default: 25, Min: 1, Max: 1000
	MinConnections      int    `yaml:"min_connections"`        // Default: 2, Min: 0, Max: max_connections
	ConnectionTimeoutMs int    `yaml:"connection_timeout_ms"`  // Default: 5000ms, Max: 60000ms
	IdleTimeoutMs       int    `yaml:"idle_timeout_ms"`        // Default: 300000ms (5m)
	MaxLifetimeMs       int    `yaml:"max_lifetime_ms"`        // Default: 1800000ms (30m)
	HealthCheckPeriodMs int    `yaml:"health_check_period_ms"` // Default: 15000ms (15s)
	SSLMode             string `yaml:"ssl_mode"`               // disable | allow | prefer | require | verify-ca | verify-full
	SSLRootCert         string `yaml:"ssl_root_cert"`          // CA certificate path or inline PEM
	SSLCert             string `yaml:"ssl_cert"`               // Client certificate path or inline PEM
	SSLKey              string `yaml:"ssl_key"`                // Client private key path or inline PEM
}

// SecurityConfig contains cryptographic encryption settings.
type SecurityConfig struct {
	MasterEncryptionKey string `yaml:"master_encryption_key"`
}

// KVStoreConfig defines key-value store connection options.
type KVStoreConfig struct {
	Backend     string   `yaml:"backend"`      // database | redis
	URL         string   `yaml:"url"`          // Standalone Redis URL
	ClusterURLs []string `yaml:"cluster_urls"` // Cluster Redis URLs
}

// ServiceConfig controls the enablement state of an individual functional service.
type ServiceConfig struct {
	Enabled bool `yaml:"enabled"`
}

// ConsoleConfig controls the embedded administrative console settings.
type ConsoleConfig struct {
	Enabled             bool   `yaml:"enabled"`
	InitialUserEmail    string `yaml:"initial_user_email"`
	InitialUserPassword string `yaml:"initial_user_password"`
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		Version: "1",
		Project: ProjectConfig{
			Name:        "layr-app",
			Description: "Layr Application",
		},
		Server: ServerConfig{
			ListenAddr: ":8080",
			BaseURL:    "http://localhost:8080",
		},
		Database: DatabaseConfig{
			URL:                 ".layr/data",
			MaxConnections:      25,
			MinConnections:      2,
			ConnectionTimeoutMs: 5000,
			IdleTimeoutMs:       300000,
			MaxLifetimeMs:       1800000,
			HealthCheckPeriodMs: 15000,
		},
		Security: SecurityConfig{
			MasterEncryptionKey: "",
		},
		KVStore: KVStoreConfig{
			Backend: "database",
		},
		Data:         ServiceConfig{Enabled: false},
		Auth:         ServiceConfig{Enabled: false},
		Tasks:        ServiceConfig{Enabled: false},
		FileStorage:  ServiceConfig{Enabled: false},
		Notification: ServiceConfig{Enabled: false},
		Analytics:    ServiceConfig{Enabled: false},
		Image:        ServiceConfig{Enabled: false},
		Console: ConsoleConfig{
			Enabled: true,
		},
	}
}

var (
	loadedConfig *Config
	rwMutex      sync.RWMutex
)

// LoadConfig loads config from a file path, applies environment variable overlays,
// and caches the loaded configuration in lifetime memory.
func LoadConfig(path string) (*Config, error) {
	config := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
			}
		} else {
			if err := yaml.Unmarshal(data, config); err != nil {
				return nil, fmt.Errorf("failed to parse config yaml: %w", err)
			}
		}
	}

	ApplyEnvConfigOverrides(config)

	rwMutex.Lock()
	loadedConfig = config
	rwMutex.Unlock()

	return config, nil
}

// GetConfig returns the loaded configuration from lifetime memory, or DefaultConfig() if not yet loaded.
func GetConfig() *Config {
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	if loadedConfig == nil {
		return DefaultConfig()
	}
	return loadedConfig
}

// SetLoadedConfig sets the in-memory configuration directly (for testing and custom setup).
func SetLoadedConfig(config *Config) {
	rwMutex.Lock()
	defer rwMutex.Unlock()
	loadedConfig = config
}

// UnloadConfig clears the cached in-memory configuration (for testing cleanup).
func UnloadConfig() {
	rwMutex.Lock()
	defer rwMutex.Unlock()
	loadedConfig = nil
}

// ApplyEnvConfigOverrides applies LAYR__* and top-level alias environment variables.
func ApplyEnvConfigOverrides(config *Config) {
	// Aliases
	if value := os.Getenv("DATABASE_URL"); value != "" {
		config.Database.URL = value
	}
	if value := os.Getenv("MASTER_ENCRYPTION_KEY"); value != "" {
		config.Security.MasterEncryptionKey = value
	}
	if value := os.Getenv("PORT"); value != "" {
		if _, err := strconv.Atoi(value); err == nil {
			if strings.Contains(config.Server.ListenAddr, ":") {
				host := config.Server.ListenAddr[:strings.LastIndex(config.Server.ListenAddr, ":")]
				config.Server.ListenAddr = host + ":" + value
			} else {
				config.Server.ListenAddr = ":" + value
			}
		}
	}
	if value := os.Getenv("BASE_URL"); value != "" {
		config.Server.BaseURL = value
	}

	// Dynamic LAYR__<SECTION>__<FIELD> overrides
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], "LAYR__") {
			keyParts := strings.Split(parts[0], "__")
			section := strings.ToLower(keyParts[1])
			field := ""
			expectedKeyPartsLen := 3
			if len(keyParts) >= expectedKeyPartsLen {
				field = strings.ToLower(keyParts[2])
			}
			applyEnvConfigField(config, section, field, parts[1])
		}
	}
}

func applyEnvConfigField(config *Config, section, field, value string) {
	switch section {
	case "project":
		switch field {
		case "name":
			config.Project.Name = value
		case "description":
			config.Project.Description = value
		}
	case "server":
		switch field {
		case "listen_addr", "listenaddr":
			config.Server.ListenAddr = value
		case "base_url", "baseurl":
			config.Server.BaseURL = value
		}
	case "database":
		switch field {
		case "url":
			config.Database.URL = value
		case "max_connections":
			if number, err := strconv.Atoi(value); err == nil {
				config.Database.MaxConnections = number
			}
		case "min_connections":
			if number, err := strconv.Atoi(value); err == nil {
				config.Database.MinConnections = number
			}
		case "connection_timeout_ms":
			if number, err := strconv.Atoi(value); err == nil {
				config.Database.ConnectionTimeoutMs = number
			}
		case "idle_timeout_ms":
			if number, err := strconv.Atoi(value); err == nil {
				config.Database.IdleTimeoutMs = number
			}
		case "max_lifetime_ms":
			if number, err := strconv.Atoi(value); err == nil {
				config.Database.MaxLifetimeMs = number
			}
		case "health_check_period_ms":
			if number, err := strconv.Atoi(value); err == nil {
				config.Database.HealthCheckPeriodMs = number
			}
		case "ssl_mode":
			config.Database.SSLMode = strings.ToLower(value)
		case "ssl_root_cert":
			config.Database.SSLRootCert = value
		case "ssl_cert":
			config.Database.SSLCert = value
		case "ssl_key":
			config.Database.SSLKey = value
		}
	case "security":
		if field == "master_encryption_key" {
			config.Security.MasterEncryptionKey = value
		}
	case "kv_store", "kvstore", "kv":
		switch field {
		case "backend":
			config.KVStore.Backend = strings.ToLower(value)
		case "url":
			config.KVStore.URL = value
		case "cluster_urls":
			if value != "" {
				rawSegments := strings.Split(value, ",")
				var cleaned []string
				for _, candidateURL := range rawSegments {
					if trimmed := strings.TrimSpace(candidateURL); trimmed != "" {
						cleaned = append(cleaned, trimmed)
					}
				}
				config.KVStore.ClusterURLs = cleaned
			}
		}
	case "data":
		if field == "enabled" {
			config.Data.Enabled = parseFlag(value)
		}
	case "auth":
		if field == "enabled" {
			config.Auth.Enabled = parseFlag(value)
		}
	case "tasks":
		if field == "enabled" {
			config.Tasks.Enabled = parseFlag(value)
		}
	case "file_storage", "filestorage":
		if field == "enabled" {
			config.FileStorage.Enabled = parseFlag(value)
		}
	case "notification":
		if field == "enabled" {
			config.Notification.Enabled = parseFlag(value)
		}
	case "analytics":
		if field == "enabled" {
			config.Analytics.Enabled = parseFlag(value)
		}
	case "image":
		if field == "enabled" {
			config.Image.Enabled = parseFlag(value)
		}
	case "console":
		switch field {
		case "enabled":
			config.Console.Enabled = parseFlag(value)
		case "initial_user_email":
			config.Console.InitialUserEmail = value
		case "initial_user_password":
			config.Console.InitialUserPassword = value
		}
	}
}

func parseFlag(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

// ServerBaseURL returns the configured base HTTP URL (e.g. http://localhost:8080).
func (config *Config) ServerBaseURL() string {
	if config == nil || config.Server.BaseURL == "" {
		return "http://localhost:8080"
	}
	return config.Server.BaseURL
}

// Validate asserts that the configuration satisfies all system invariants.
func (config *Config) Validate() error {
	if config.Version != "1" {
		return fmt.Errorf("invalid config version '%s', expected '1'", config.Version)
	}

	if strings.TrimSpace(config.Database.URL) == "" {
		return errors.New("database.url is required")
	}

	if config.Database.MaxConnections < 1 || config.Database.MaxConnections > 1000 {
		return fmt.Errorf("invalid database.max_connections '%d', must be between 1 and 1000", config.Database.MaxConnections)
	}

	if config.Database.MinConnections < 0 || config.Database.MinConnections > config.Database.MaxConnections {
		return fmt.Errorf("invalid database.min_connections '%d', must be between 0 and max_connections (%d)", config.Database.MinConnections, config.Database.MaxConnections)
	}

	if config.Database.ConnectionTimeoutMs < 100 || config.Database.ConnectionTimeoutMs > 60000 {
		return fmt.Errorf("invalid database.connection_timeout_ms '%d', must be between 100 and 60000", config.Database.ConnectionTimeoutMs)
	}

	if config.Database.IdleTimeoutMs < 1000 || config.Database.IdleTimeoutMs > 86400000 {
		return fmt.Errorf("invalid database.idle_timeout_ms '%d', must be between 1000 and 86400000", config.Database.IdleTimeoutMs)
	}

	if config.Database.MaxLifetimeMs < 1000 || config.Database.MaxLifetimeMs > 86400000 {
		return fmt.Errorf("invalid database.max_lifetime_ms '%d', must be between 1000 and 86400000", config.Database.MaxLifetimeMs)
	}

	if config.Database.HealthCheckPeriodMs < 1000 || config.Database.HealthCheckPeriodMs > 3600000 {
		return fmt.Errorf("invalid database.health_check_period_ms '%d', must be between 1000 and 3600000", config.Database.HealthCheckPeriodMs)
	}

	if config.Database.SSLMode != "" {
		validSSLModes := map[string]bool{
			"disable": true, "allow": true, "prefer": true, "require": true, "verify-ca": true, "verify-full": true,
		}
		if !validSSLModes[config.Database.SSLMode] {
			return fmt.Errorf("invalid database.ssl_mode '%s', expected disable, allow, prefer, require, verify-ca, or verify-full", config.Database.SSLMode)
		}
	}

	expectedMasterEncryptionKeyLength := 64
	encryptionKey := strings.TrimSpace(config.Security.MasterEncryptionKey)
	if encryptionKey == "" {
		return errors.New("security.master_encryption_key is required")
	}
	if len(encryptionKey) != expectedMasterEncryptionKeyLength {
		return fmt.Errorf("security.master_encryption_key must be a 64-character hex string (32 bytes), got length %d", len(encryptionKey))
	}
	encryptionKeyBytes := []byte(encryptionKey)
	decodedKey := make([]byte, hex.DecodedLen(len(encryptionKeyBytes)))
	if _, err := hex.Decode(decodedKey, encryptionKeyBytes); err != nil {
		return fmt.Errorf("security.master_encryption_key must be valid hex: %w", err)
	}

	if config.KVStore.Backend != "" && config.KVStore.Backend != "database" && config.KVStore.Backend != "redis" {
		return fmt.Errorf("invalid kv_store.backend '%s', expected 'database' or 'redis'", config.KVStore.Backend)
	}

	if config.KVStore.Backend == "redis" && config.KVStore.URL == "" && len(config.KVStore.ClusterURLs) == 0 {
		return errors.New("kv_store.url or kv_store.cluster_urls is required when kv_store.backend is 'redis'")
	}

	// Minimum Functional Service Invariant
	if !config.HasAnyFunctionalServiceEnabled() {
		return errors.New("minimum functional service invariant violated: at least one functional backend service (data, auth, tasks, file_storage, notification, analytics, image) must be enabled")
	}

	return nil
}

// HasAnyFunctionalServiceEnabled checks if at least one functional service is active.
func (config *Config) HasAnyFunctionalServiceEnabled() bool {
	return config.Data.Enabled ||
		config.Auth.Enabled ||
		config.Tasks.Enabled ||
		config.FileStorage.Enabled ||
		config.Notification.Enabled ||
		config.Analytics.Enabled ||
		config.Image.Enabled
}

// GetFunctionalServices returns a slice of active functional backend service names (excluding console).
func (config *Config) GetFunctionalServices() []string {
	var list []string
	if config.Data.Enabled {
		list = append(list, "data")
	}
	if config.Auth.Enabled {
		list = append(list, "auth")
	}
	if config.Tasks.Enabled {
		list = append(list, "tasks")
	}
	if config.FileStorage.Enabled {
		list = append(list, "file_storage")
	}
	if config.Notification.Enabled {
		list = append(list, "notification")
	}
	if config.Analytics.Enabled {
		list = append(list, "analytics")
	}
	if config.Image.Enabled {
		list = append(list, "image")
	}
	return list
}

// GetEnabledServices returns a slice of active service names including console.
func (config *Config) GetEnabledServices() []string {
	list := config.GetFunctionalServices()
	if config.Console.Enabled {
		list = append(list, "console")
	}
	return list
}

// IsServiceEnabled checks if a specific service is enabled in configuration.
func (config *Config) IsServiceEnabled(serviceName string) bool {
	switch strings.ToLower(serviceName) {
	case "data":
		return config.Data.Enabled
	case "auth":
		return config.Auth.Enabled
	case "tasks":
		return config.Tasks.Enabled
	case "file_storage", "filestorage":
		return config.FileStorage.Enabled
	case "notification":
		return config.Notification.Enabled
	case "analytics":
		return config.Analytics.Enabled
	case "image":
		return config.Image.Enabled
	case "console":
		return config.Console.Enabled
	default:
		return false
	}
}

// EnableService dynamically enables a service on config struct.
func (config *Config) EnableService(serviceName string) error {
	switch strings.ToLower(serviceName) {
	case "data":
		config.Data.Enabled = true
	case "auth":
		config.Auth.Enabled = true
	case "tasks":
		config.Tasks.Enabled = true
	case "file_storage", "filestorage":
		config.FileStorage.Enabled = true
	case "notification":
		config.Notification.Enabled = true
	case "analytics":
		config.Analytics.Enabled = true
	case "image":
		config.Image.Enabled = true
	case "console":
		config.Console.Enabled = true
	default:
		return fmt.Errorf("unknown service '%s'", serviceName)
	}
	return nil
}
