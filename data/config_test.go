package data

import (
	"context"
	"errors"
	"layr.sh/core"
	"testing"
)

func TestDataConfigDefaultsUnit(t *testing.T) {
	dataConfig := DefaultConfig()

	if !dataConfig.REST.Enabled {
		t.Fatal("expected REST to be enabled by default")
	}
	if !dataConfig.GraphQL.Enabled {
		t.Fatal("expected GraphQL to be enabled by default")
	}
	if !dataConfig.Realtime.Enabled {
		t.Fatal("expected Realtime to be enabled by default")
	}
	if !dataConfig.Cache.Enabled {
		t.Fatal("expected Cache to be enabled by default")
	}
	if dataConfig.RateLimiting.Enabled {
		t.Fatal("expected RateLimiting to be disabled by default")
	}
	if dataConfig.REST.DefaultLimit != 50 {
		t.Fatalf("expected REST default limit 50, got %d", dataConfig.REST.DefaultLimit)
	}
	if dataConfig.REST.MaxLimit != 1000 {
		t.Fatalf("expected REST max limit 1000, got %d", dataConfig.REST.MaxLimit)
	}
	if dataConfig.GraphQL.MaxDepth != 8 {
		t.Fatalf("expected GraphQL max depth 8, got %d", dataConfig.GraphQL.MaxDepth)
	}
	if dataConfig.Realtime.HeartbeatIntervalMS != 30000 {
		t.Fatalf("expected HeartbeatIntervalMS 30000, got %d", dataConfig.Realtime.HeartbeatIntervalMS)
	}
	if dataConfig.Realtime.MaxChannelsPerConnection != 50 {
		t.Fatalf("expected MaxChannelsPerConnection 50, got %d", dataConfig.Realtime.MaxChannelsPerConnection)
	}
}

func TestDataConfigValidationUnit(t *testing.T) {
	validConfig := DefaultConfig()
	if err := validConfig.Validate(); err != nil {
		t.Fatalf("expected valid default config, got: %v", err)
	}

	// 1. Invalid REST max limit
	invalidRESTMaxConfig := validConfig
	invalidRESTMaxConfig.REST.MaxLimit = 0
	if err := invalidRESTMaxConfig.Validate(); err == nil {
		t.Fatal("expected error on REST max limit <= 0")
	}

	// 2. Invalid REST default limit
	invalidRESTDefaultConfig := validConfig
	invalidRESTDefaultConfig.REST.DefaultLimit = -5
	if err := invalidRESTDefaultConfig.Validate(); err == nil {
		t.Fatal("expected error on REST default limit <= 0")
	}

	// 3. Invalid GraphQL max depth
	invalidGraphQLDepthConfig := validConfig
	invalidGraphQLDepthConfig.GraphQL.MaxDepth = 0
	if err := invalidGraphQLDepthConfig.Validate(); err == nil {
		t.Fatal("expected error on GraphQL max depth <= 0")
	}

	// 3b. Invalid GraphQL max complexity
	invalidGraphQLComplexityConfig := validConfig
	invalidGraphQLComplexityConfig.GraphQL.MaxComplexity = 0
	if err := invalidGraphQLComplexityConfig.Validate(); err == nil {
		t.Fatal("expected error on GraphQL max complexity <= 0")
	}

	// 4. Invalid Realtime heartbeat
	invalidHeartbeatConfig := validConfig
	invalidHeartbeatConfig.Realtime.HeartbeatIntervalMS = 0
	if err := invalidHeartbeatConfig.Validate(); err == nil {
		t.Fatal("expected error on Realtime heartbeat interval <= 0")
	}

	// 5. Invalid Realtime max channels
	invalidMaxChannelsConfig := validConfig
	invalidMaxChannelsConfig.Realtime.MaxChannelsPerConnection = -1
	if err := invalidMaxChannelsConfig.Validate(); err == nil {
		t.Fatal("expected error on Realtime max channels <= 0")
	}
}

func TestDataConfigManagerMemoryUnit(t *testing.T) {
	configManager := NewConfigManager(nil)
	if configManager == nil {
		t.Fatal("expected non-nil config manager")
	}

	currentConfig := configManager.Get()
	if !currentConfig.REST.Enabled || !currentConfig.GraphQL.Enabled {
		t.Fatal("expected default settings in new config manager")
	}
}

func TestDataConfigManagerSetValidationUnit(t *testing.T) {
	if ConfigKey != "runtime" {
		t.Fatalf("expected ConfigKey to be 'runtime', got %q", ConfigKey)
	}

	configManager := NewConfigManager(nil)
	invalidConfig := DefaultConfig()
	invalidConfig.REST.MaxLimit = 0

	err := configManager.Set(context.Background(), invalidConfig)
	if err == nil {
		t.Fatal("expected Set to fail on invalid configuration, but got nil")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected error to wrap ErrInvalidConfig, got: %v", err)
	}
}

func TestDataConfigSnapshotIsolationUnit(t *testing.T) {
	configManager := NewConfigManager(core.NewTestKernel(nil))
	config := DefaultConfig()
	config.REST.ExcludedTables = []string{"private"}
	configManager.SetMemoryConfig(config)
	config.Schemas[0] = "changed"
	config.REST.ExcludedTables[0] = "changed"
	config.CORS.AllowedOrigins[0] = "changed"
	config.CORS.AllowedHeaders[0] = "changed"
	snapshotConfig := configManager.Get()
	if snapshotConfig.Schemas[0] != "public" || snapshotConfig.REST.ExcludedTables[0] != "private" || snapshotConfig.CORS.AllowedOrigins[0] != "*" || snapshotConfig.CORS.AllowedHeaders[0] != "Authorization" {
		t.Fatal("configuration retained caller-owned slices")
	}
	snapshotConfig.Schemas[0] = "mutated"
	snapshotConfig.REST.ExcludedTables[0] = "mutated"
	snapshotConfig.CORS.AllowedOrigins[0] = "mutated"
	snapshotConfig.CORS.AllowedHeaders[0] = "mutated"
	nextConfig := configManager.Get()
	if nextConfig.Schemas[0] != "public" || nextConfig.REST.ExcludedTables[0] != "private" || nextConfig.CORS.AllowedOrigins[0] != "*" || nextConfig.CORS.AllowedHeaders[0] != "Authorization" {
		t.Fatal("configuration exposed mutable internal slices")
	}
}

func TestDataAdditionalConfigValidationUnit(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		change func(*Config)
	}{
		{"default limit", func(config *Config) { config.REST.DefaultLimit = config.REST.MaxLimit + 1 }},
		{"graphql limit", func(config *Config) { config.GraphQL.DefaultLimit = 0 }},
		{"connections", func(config *Config) { config.Realtime.MaxConnections = 0 }},
		{"catalog ttl", func(config *Config) { config.Cache.CatalogTTLSeconds = -1 }},
		{"query ttl", func(config *Config) { config.Cache.QueryTTLSeconds = -1 }},
		{"cache count", func(config *Config) { config.Cache.MaxCachedQueries = -1 }},
		{"minute quota", func(config *Config) { config.RateLimiting.Enabled = true; config.RateLimiting.RequestsPerMinute = 0 }},
		{"burst quota", func(config *Config) { config.RateLimiting.Enabled = true; config.RateLimiting.Burst = 0 }},
		{"protected schema", func(config *Config) { config.Schemas = []string{"auth"} }},
		{"invalid schema", func(config *Config) { config.Schemas = []string{"bad-name"} }},
		{"empty exclusion", func(config *Config) { config.REST.ExcludedTables = []string{""} }},
		{"malformed origin", func(config *Config) { config.CORS.AllowedOrigins = []string{"https://%"} }},
		{"origin scheme", func(config *Config) { config.CORS.AllowedOrigins = []string{"ftp://app.test"} }},
		{"origin host", func(config *Config) { config.CORS.AllowedOrigins = []string{"https:"} }},
		{"origin credentials", func(config *Config) { config.CORS.AllowedOrigins = []string{"https://user@app.test"} }},
		{"origin path", func(config *Config) { config.CORS.AllowedOrigins = []string{"https://app.test/path"} }},
		{"origin query", func(config *Config) { config.CORS.AllowedOrigins = []string{"https://app.test?x=1"} }},
		{"origin fragment", func(config *Config) { config.CORS.AllowedOrigins = []string{"https://app.test#x"} }},
		{"header injection", func(config *Config) { config.CORS.AllowedHeaders = []string{"X-Test\r\nBad: value"} }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			config := DefaultConfig()
			testCase.change(&config)
			if err := config.Validate(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	config := DefaultConfig()
	config.CORS.AllowedOrigins = []string{"https://app.test", "null", "*"}
	config.CORS.AllowedHeaders = []string{"Authorization", "*"}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
}
