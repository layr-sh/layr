package filestorage

import (
	"context"
	"testing"
)

func TestFilestorageConfigPostgresIntegration(t *testing.T) {
	db, cleanup := setupTestFileStorageDatabase(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	configManager := NewConfigManager(db)

	// 1. Initial Load when key 'runtime' does not exist -> seeds DefaultConfig
	if err := configManager.Load(ctx); err != nil {
		t.Fatalf("failed initial Load: %v", err)
	}
	initialConfig := configManager.Get()
	if !initialConfig.Enabled || initialConfig.ChunkSizeBytes != DefaultChunkSizeBytes {
		t.Fatalf("unexpected initial settings: %+v", initialConfig)
	}

	// 2. Set with custom configuration and verify roundtrip via Load
	customConfig := initialConfig
	customConfig.ChunkSizeBytes = 1048576
	customConfig.DefaultMaxFileSizeBytes = 104857600
	customConfig.PresignTokenExpirySeconds = 7200

	if err := configManager.Set(ctx, customConfig); err != nil {
		t.Fatalf("failed to set custom config: %v", err)
	}

	freshConfigManager := NewConfigManager(db)
	if err := freshConfigManager.Load(ctx); err != nil {
		t.Fatalf("failed fresh Load: %v", err)
	}
	loadedConfig := freshConfigManager.Get()
	if loadedConfig.ChunkSizeBytes != 1048576 || loadedConfig.DefaultMaxFileSizeBytes != 104857600 || loadedConfig.PresignTokenExpirySeconds != 7200 {
		t.Fatalf("mismatched loaded settings: %+v", loadedConfig)
	}

	// 3. Corrupt JSON in database causes Load to return error
	const corruptSQLStatement = `UPDATE file_storage.config SET value = '"invalid json string not object"'::jsonb WHERE key = 'runtime'`
	if _, execErr := db.Exec(ctx, corruptSQLStatement); execErr != nil {
		t.Fatalf("failed to corrupt config JSON: %v", execErr)
	}

	corruptedConfigManager := NewConfigManager(db)
	if err := corruptedConfigManager.Load(ctx); err == nil {
		t.Fatal("expected error on Load with corrupted config JSON")
	}

	corruptService := NewService(db, nil)
	if serviceStartErr := corruptService.Start(ctx); serviceStartErr == nil {
		t.Fatal("expected error on service Start with corrupted config JSON")
	}

	// 4. Set with canceled context fails with exec error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if setCancelErr := configManager.Set(canceledCtx, customConfig); setCancelErr == nil {
		t.Fatal("expected error on Set with canceled context")
	}
}
