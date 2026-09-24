package filestorage

import (
	"context"
	"sync"
	"testing"
	"time"

	"layr.sh/core"
)

func TestFilestorageConfigPostgresIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	if kernel == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(kernel)

	// 1. Initial Load when key 'runtime' does not exist -> seeds DefaultConfig
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed initial Load: %v", err)
	}
	initialConfig := configManager.Get()
	if !initialConfig.Enabled || initialConfig.ChunkSizeBytes != DefaultChunkSizeBytes {
		t.Fatalf("unexpected initial settings: %+v", initialConfig)
	}

	// Subscribe to config updated events
	var capturedEvents []core.Event
	var eventMutex sync.Mutex
	kernel.EventBus().Subscribe("file_storage.config.updated", func(eventCtx context.Context, event core.Event) error {
		eventMutex.Lock()
		defer eventMutex.Unlock()
		capturedEvents = append(capturedEvents, event)
		return nil
	})

	// 2. Set with custom configuration and verify roundtrip via Load
	customConfig := initialConfig
	customConfig.ChunkSizeBytes = 1048576
	customConfig.DefaultMaxFileSizeBytes = 104857600
	customConfig.PresignTokenExpirySeconds = 7200

	if err := configManager.Set(ctx, customConfig); err != nil {
		t.Fatalf("failed to set custom config: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	eventMutex.Lock()
	if len(capturedEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(capturedEvents))
	}
	if capturedEvents[0].ResourceID == nil || *capturedEvents[0].ResourceID != "file_storage.config" {
		t.Fatalf("expected resourceID 'file_storage.config', got %v", capturedEvents[0].ResourceID)
	}
	eventMutex.Unlock()

	freshConfigManager := NewConfigManager(kernel)
	if err := freshConfigManager.Load(ctx); err != nil {
		t.Fatalf("failed fresh Load: %v", err)
	}
	loadedConfig := freshConfigManager.Get()
	if loadedConfig.ChunkSizeBytes != 1048576 || loadedConfig.DefaultMaxFileSizeBytes != 104857600 || loadedConfig.PresignTokenExpirySeconds != 7200 {
		t.Fatalf("mismatched loaded settings: %+v", loadedConfig)
	}

	// 3. Corrupt JSON in database causes Load to return error
	const corruptSQLStatement = `UPDATE file_storage.config SET value = '"invalid json string not object"'::jsonb WHERE key = 'runtime'`
	if _, execErr := kernel.DB().Exec(ctx, corruptSQLStatement); execErr != nil {
		t.Fatalf("failed to corrupt config JSON: %v", execErr)
	}

	corruptedConfigManager := NewConfigManager(kernel)
	if err := corruptedConfigManager.Load(ctx); err == nil {
		t.Fatal("expected error on Load with corrupted config JSON")
	}

	corruptService := NewService(kernel)
	if serviceStartErr := corruptService.Start(ctx); serviceStartErr == nil {
		t.Fatal("expected error on service Start with corrupted config JSON")
	}

	// 4. Invalid config values in database causes Load to return validation error
	const invalidValuesSQL = `UPDATE file_storage.config SET value = '{"chunk_size_bytes": 0, "default_max_file_size_bytes": 100, "presign_token_expiry_seconds": 100}'::jsonb WHERE key = 'runtime'`
	if _, execErr := kernel.DB().Exec(ctx, invalidValuesSQL); execErr != nil {
		t.Fatalf("failed to write invalid config values: %v", execErr)
	}

	invalidConfigManager := NewConfigManager(kernel)
	if err := invalidConfigManager.Load(ctx); err == nil {
		t.Fatal("expected error on Load with invalid stored config values")
	}

	// 5. Canceled context causes query errors on Load and Set
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if loadCancelErr := configManager.Load(canceledCtx); loadCancelErr == nil {
		t.Fatal("expected error on Load with canceled context")
	}
	if setCancelErr := configManager.Set(canceledCtx, customConfig); setCancelErr == nil {
		t.Fatal("expected error on Set with canceled context")
	}
}
