package data

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"layr.sh/core"
)

func TestDataConfigPostgresIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(kernel)

	// 1. Initial Load when key 'runtime' does not exist -> seeds DefaultConfig
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed initial Load: %v", err)
	}
	initialConfig := configManager.Get()
	if !initialConfig.REST.Enabled || initialConfig.REST.MaxLimit != 1000 {
		t.Fatalf("unexpected initial settings: %+v", initialConfig)
	}

	// 2. Set with custom configuration and verify roundtrip via Load and event emission
	var capturedEvents []core.Event
	var eventsMutex sync.Mutex
	kernel.EventBus().Subscribe("data.config.updated", func(_ context.Context, event core.Event) error {
		eventsMutex.Lock()
		capturedEvents = append(capturedEvents, event)
		eventsMutex.Unlock()
		return nil
	})

	customConfig := initialConfig
	customConfig.Schemas = nil
	customConfig.REST.MaxLimit = 500
	customConfig.REST.DefaultLimit = 25
	customConfig.GraphQL.MaxDepth = 12
	customConfig.Realtime.HeartbeatIntervalMS = 15000

	if err := configManager.Set(ctx, customConfig); err != nil {
		t.Fatalf("failed to set custom config: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	eventsMutex.Lock()
	if len(capturedEvents) != 1 {
		t.Fatalf("expected 1 event published, got %d", len(capturedEvents))
	}
	if capturedEvents[0].ResourceID == nil || *capturedEvents[0].ResourceID != "data.config" {
		t.Fatalf("expected resource ID data.config, got: %v", capturedEvents[0].ResourceID)
	}
	eventsMutex.Unlock()

	freshConfigManager := NewConfigManager(kernel)
	if err := freshConfigManager.Load(ctx); err != nil {
		t.Fatalf("failed fresh Load: %v", err)
	}
	loadedConfig := freshConfigManager.Get()
	if loadedConfig.REST.MaxLimit != 500 || loadedConfig.REST.DefaultLimit != 25 || loadedConfig.GraphQL.MaxDepth != 12 || len(loadedConfig.Schemas) != 2 {
		t.Fatalf("mismatched loaded settings: %+v", loadedConfig)
	}

	// 3. Set with zero/negative fields fails validation
	zeroConfig := Config{}
	if err := configManager.Set(ctx, zeroConfig); err == nil || !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig on zero config, got: %v", err)
	}

	// 4. Corrupt JSON in database causes Load to return error
	const corruptSQLStatement = `UPDATE data.config SET value = '"invalid json string not object"'::jsonb WHERE key = 'runtime'`
	if _, execErr := kernel.DB().Exec(ctx, corruptSQLStatement); execErr != nil {
		t.Fatalf("failed to corrupt config JSON: %v", execErr)
	}
	if err := configManager.Load(ctx); err == nil {
		t.Fatal("expected Load error on invalid JSON structure in DB")
	}

	// 4b. Invalid config values in database causes Load to return validation error
	const invalidConfigSQLStatement = `UPDATE data.config SET value = '{"rest":{"max_limit":-1,"default_limit":50},"graphql":{"max_depth":8,"max_complexity":500},"realtime":{"heartbeat_interval_ms":30000,"max_channels_per_connection":50}}'::jsonb WHERE key = 'runtime'`
	if _, execErr := kernel.DB().Exec(ctx, invalidConfigSQLStatement); execErr != nil {
		t.Fatalf("failed to write invalid config values: %v", execErr)
	}
	if err := configManager.Load(ctx); err == nil {
		t.Fatal("expected Load error on invalid config values in DB")
	}

	// 5. DB execution error on Set (e.g. canceled context)
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err := configManager.Set(canceledCtx, customConfig); err == nil {
		t.Fatal("expected error on Set with canceled context")
	}

	// 6. DB execution error on Load (canceled context) returns error (not default)
	if err := configManager.Load(canceledCtx); err == nil {
		t.Fatal("expected error on Load with canceled context")
	}
}
