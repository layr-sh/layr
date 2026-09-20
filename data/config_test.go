package data

import (
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
