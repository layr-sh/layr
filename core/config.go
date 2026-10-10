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
)

const (
	// ConfigFailureExitCode is the exit code for configuration failures according to sysexits.h
	ConfigFailureExitCode = 78
)

// Config represents the Tier 1 configuration (environment variables and defaults).
type Config struct {
	Version  string         `json:"version"`
	Project  ProjectConfig  `json:"project"`
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	Security SecurityConfig `json:"security"`
	KVStore  KVStoreConfig  `json:"kv_store"`
	RootUser RootUserConfig `json:"root_user"`

	enabledServices []string
}

// ProjectConfig holds general project metadata.
type ProjectConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ServerConfig defines the HTTP server network listener and base URL.
type ServerConfig struct {
	ListenAddr        string `json:"listen_addr"`
	BaseURL           string `json:"base_url"`
	TrustProxyHeaders bool   `json:"trust_proxy_headers"`
}

// DatabaseConfig defines PostgreSQL connection and pool parameters.
type DatabaseConfig struct {
	URL                 string `json:"url"`                    // postgres://... or .layr/data
	MaxConnections      int    `json:"max_connections"`        // Default: 25, Min: 1, Max: 1000
	MinConnections      int    `json:"min_connections"`        // Default: 2, Min: 0, Max: max_connections
	ConnectionTimeoutMs int    `json:"connection_timeout_ms"`  // Default: 5000ms, Max: 60000ms
	IdleTimeoutMs       int    `json:"idle_timeout_ms"`        // Default: 300000ms (5m)
	MaxLifetimeMs       int    `json:"max_lifetime_ms"`        // Default: 1800000ms (30m)
	HealthCheckPeriodMs int    `json:"health_check_period_ms"` // Default: 15000ms (15s)
	SSLMode             string `json:"ssl_mode"`               // disable | allow | prefer | require | verify-ca | verify-full
	SSLRootCert         string `json:"ssl_root_cert"`          // CA certificate path or inline PEM
	SSLCert             string `json:"ssl_cert"`               // Client certificate path or inline PEM
	SSLKey              string `json:"ssl_key"`                // Client private key path or inline PEM
}

// SecurityConfig contains cryptographic encryption settings.
type SecurityConfig struct {
	MasterEncryptionKey string `json:"master_encryption_key"`
}

// KVStoreConfig defines key-value store connection options.
type KVStoreConfig struct {
	Backend     string   `json:"backend"`      // database | redis
	URL         string   `json:"url"`          // Standalone Redis URL
	ClusterURLs []string `json:"cluster_urls"` // Cluster Redis URLs
}

// RootUserConfig defines the initial root user credentials for administrative access.
type RootUserConfig struct {
	Email    string `json:"email"`
	Password string `json:"password"`
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
			ListenAddr:        ":5297",
			BaseURL:           "http://localhost:5297",
			TrustProxyHeaders: true,
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
		RootUser: RootUserConfig{
			Email:    "root@layr.local",
			Password: "",
		},
		enabledServices: DefaultServices(),
	}
}

var (
	loadedConfig *Config
	rwMutex      sync.RWMutex
)

// LoadConfig loads configuration from environment variables (optionally loading an env file first)
// and caches the loaded configuration in lifetime memory.
func LoadConfig(optionalEnvFile ...string) (*Config, error) {
	if len(optionalEnvFile) > 0 && optionalEnvFile[0] != "" {
		envPath := optionalEnvFile[0]
		log.Debugf("loading configuration from env file %q", envPath)
		if err := LoadEnvFile(envPath); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
	}

	config := DefaultConfig()
	applyEnvConfigOverrides(config, true)

	rwMutex.Lock()
	loadedConfig = config
	rwMutex.Unlock()

	log.Tracef("configuration loaded successfully for project %s", config.Project.Name)
	return config, nil
}

// LoadEnvFile loads environment variables from a file into the process environment.
func LoadEnvFile(path string) error {
	fileData, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read env file %s: %w", path, err)
	}

	for _, line := range strings.Split(string(fileData), "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		key, value, ok := strings.Cut(trimmedLine, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if len(value) >= 2 {
			first := value[0]
			last := value[len(value)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		if existingValue, exists := os.LookupEnv(key); exists && existingValue != "" {
			continue
		}

		if setErr := os.Setenv(key, value); setErr != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", key, setErr)
		}
	}

	return nil
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

// ApplyEnvConfigOverrides applies LAYR__* and top-level alias environment variables without logging.
func ApplyEnvConfigOverrides(config *Config) {
	applyEnvConfigOverrides(config, false)
}

func applyEnvConfigOverrides(config *Config, shouldLog bool) {
	// Aliases
	if value := os.Getenv("DATABASE_URL"); value != "" {
		config.Database.URL = value
		if shouldLog {
			log.Debugf("Overriding %s from ENV", "database.url")
		}
	}
	if value := os.Getenv("MASTER_ENCRYPTION_KEY"); value != "" {
		config.Security.MasterEncryptionKey = value
		if shouldLog {
			log.Debugf("Overriding %s from ENV", "security.master_encryption_key")
		}
	}
	if value := os.Getenv("PORT"); value != "" {
		if _, err := strconv.Atoi(value); err == nil {
			if strings.Contains(config.Server.ListenAddr, ":") {
				host := config.Server.ListenAddr[:strings.LastIndex(config.Server.ListenAddr, ":")]
				config.Server.ListenAddr = host + ":" + value
			} else {
				config.Server.ListenAddr = ":" + value
			}
			if shouldLog {
				log.Debugf("Overriding %s from ENV", "server.port")
			}
		}
	}
	if value := os.Getenv("BASE_URL"); value != "" {
		config.Server.BaseURL = value
		if shouldLog {
			log.Debugf("Overriding %s from ENV", "server.base_url")
		}
	}

	// Dynamic LAYR__<SECTION>__<FIELD> overrides
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], "LAYR__") && parts[1] != "" {
			keyParts := strings.Split(parts[0], "__")
			section := strings.ToLower(keyParts[1])
			field := ""
			expectedKeyPartsLen := 3
			if len(keyParts) >= expectedKeyPartsLen {
				field = strings.ToLower(keyParts[2])
			}
			if canonicalTarget, ok := applyEnvConfigField(config, section, field, parts[1]); ok {
				if shouldLog {
					log.Debugf("Overriding %s from ENV", canonicalTarget)
				}
			}
		}
	}
}

func applyEnvConfigField(config *Config, section, field, value string) (string, bool) {
	switch section {
	case "project":
		switch field {
		case "name":
			config.Project.Name = value
			return "project.name", true
		case "description":
			config.Project.Description = value
			return "project.description", true
		}
	case "server":
		switch field {
		case "listen_addr", "listenaddr":
			config.Server.ListenAddr = value
			return "server.listen_addr", true
		case "base_url", "baseurl":
			config.Server.BaseURL = value
			return "server.base_url", true
		}
	case "database":
		switch field {
		case "url":
			config.Database.URL = value
			return "database.url", true
		case "max_connections":
			if number, err := strconv.Atoi(value); err == nil {
				config.Database.MaxConnections = number
				return "database.max_connections", true
			}
		case "min_connections":
			if number, err := strconv.Atoi(value); err == nil {
				config.Database.MinConnections = number
				return "database.min_connections", true
			}
		case "connection_timeout_ms":
			if number, err := strconv.Atoi(value); err == nil {
				config.Database.ConnectionTimeoutMs = number
				return "database.connection_timeout_ms", true
			}
		case "idle_timeout_ms":
			if number, err := strconv.Atoi(value); err == nil {
				config.Database.IdleTimeoutMs = number
				return "database.idle_timeout_ms", true
			}
		case "max_lifetime_ms":
			if number, err := strconv.Atoi(value); err == nil {
				config.Database.MaxLifetimeMs = number
				return "database.max_lifetime_ms", true
			}
		case "health_check_period_ms":
			if number, err := strconv.Atoi(value); err == nil {
				config.Database.HealthCheckPeriodMs = number
				return "database.health_check_period_ms", true
			}
		case "ssl_mode":
			config.Database.SSLMode = strings.ToLower(value)
			return "database.ssl_mode", true
		case "ssl_root_cert":
			config.Database.SSLRootCert = value
			return "database.ssl_root_cert", true
		case "ssl_cert":
			config.Database.SSLCert = value
			return "database.ssl_cert", true
		case "ssl_key":
			config.Database.SSLKey = value
			return "database.ssl_key", true
		}
	case "security":
		if field == "master_encryption_key" {
			config.Security.MasterEncryptionKey = value
			return "security.master_encryption_key", true
		}
	case "kv_store", "kvstore", "kv":
		switch field {
		case "backend":
			config.KVStore.Backend = strings.ToLower(value)
			return "kv_store.backend", true
		case "url":
			config.KVStore.URL = value
			return "kv_store.url", true
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
				return "kv_store.cluster_urls", true
			}
		}
	case "root_user", "rootuser":
		switch field {
		case "email":
			config.RootUser.Email = value
			return "root_user.email", true
		case "password":
			config.RootUser.Password = value
			return "root_user.password", true
		}
	}
	return "", false
}

// ServerBaseURL returns the configured base HTTP URL (e.g. http://localhost:5297).
func (config *Config) ServerBaseURL() string {
	if config == nil || config.Server.BaseURL == "" {
		return "http://localhost:5297"
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
		return errors.New("minimum functional service invariant violated: at least one functional backend service (data, auth, tasks, file_storage, image, function) must be enabled")
	}

	return nil
}

// DefaultServices returns the default service names enabled in monolithic mode.
func DefaultServices() []string {
	return []string{"data", "auth", "file_storage", "tasks", "image", "function", "console"}
}

// IsValidService checks whether the given service name is a recognized Layr service.
func IsValidService(serviceName string) bool {
	switch strings.ToLower(strings.TrimSpace(serviceName)) {
	case "data", "auth", "file_storage", "tasks", "image", "function", "console":
		return true
	default:
		return false
	}
}

// SetEnabledServices sets the list of active services for this configuration instance.
func (config *Config) SetEnabledServices(services []string) {
	if len(services) == 0 {
		config.enabledServices = make([]string, 0)
		return
	}
	config.enabledServices = make([]string, len(services))
	copy(config.enabledServices, services)
}

// GetEnabledServices returns a slice of active service names including console.
func (config *Config) GetEnabledServices() []string {
	if config.enabledServices == nil {
		return DefaultServices()
	}
	copied := make([]string, len(config.enabledServices))
	copy(copied, config.enabledServices)
	return copied
}

// GetFunctionalServices returns a slice of active functional backend service names (excluding console).
func (config *Config) GetFunctionalServices() []string {
	var list []string
	for _, service := range config.GetEnabledServices() {
		if strings.ToLower(service) != "console" {
			list = append(list, service)
		}
	}
	return list
}

// HasAnyFunctionalServiceEnabled checks if at least one functional service is active.
func (config *Config) HasAnyFunctionalServiceEnabled() bool {
	return len(config.GetFunctionalServices()) > 0
}

// IsServiceEnabled checks if a specific service is enabled in configuration.
func (config *Config) IsServiceEnabled(serviceName string) bool {
	target := strings.ToLower(strings.TrimSpace(serviceName))
	for _, service := range config.GetEnabledServices() {
		if strings.ToLower(service) == target {
			return true
		}
	}
	return false
}

// EnableService dynamically enables a service on config struct.
func (config *Config) EnableService(serviceName string) error {
	normalized := strings.ToLower(strings.TrimSpace(serviceName))
	if !IsValidService(normalized) {
		return fmt.Errorf("unknown service %q", serviceName)
	}
	if !config.IsServiceEnabled(normalized) {
		config.enabledServices = append(config.GetEnabledServices(), normalized)
	}
	return nil
}
