package filestorage

import (
	"context"
	"errors"
	"testing"

	"layr.sh/core"
)

func TestFilestorageConfigDefaultsUnit(t *testing.T) {
	storageConfig := DefaultConfig()

	if !storageConfig.Enabled {
		t.Fatal("expected file storage to be enabled by default")
	}
	if storageConfig.DefaultMaxFileSizeBytes != DefaultMaxFileSizeBytes {
		t.Fatalf("expected default max file size %d, got %d", DefaultMaxFileSizeBytes, storageConfig.DefaultMaxFileSizeBytes)
	}
	if storageConfig.ChunkSizeBytes != DefaultChunkSizeBytes {
		t.Fatalf("expected default chunk size %d, got %d", DefaultChunkSizeBytes, storageConfig.ChunkSizeBytes)
	}
	if storageConfig.PresignTokenExpirySeconds != DefaultPresignTokenExpirySeconds {
		t.Fatalf("expected default presign expiry %d, got %d", DefaultPresignTokenExpirySeconds, storageConfig.PresignTokenExpirySeconds)
	}
	if len(storageConfig.DefaultAllowedMIMETypes) != 0 {
		t.Fatal("expected empty allowed MIME types by default")
	}
}

func TestFilestorageConfigValidationUnit(t *testing.T) {
	validConfig := DefaultConfig()
	if err := validConfig.Validate(); err != nil {
		t.Fatalf("expected valid default config, got: %v", err)
	}

	// Invalid max file size
	invalidMaxFileConfig := validConfig
	invalidMaxFileConfig.DefaultMaxFileSizeBytes = 0
	if err := invalidMaxFileConfig.Validate(); err == nil {
		t.Fatal("expected error on DefaultMaxFileSizeBytes <= 0")
	}

	// Invalid chunk size
	invalidChunkConfig := validConfig
	invalidChunkConfig.ChunkSizeBytes = 0
	if err := invalidChunkConfig.Validate(); err == nil {
		t.Fatal("expected error on ChunkSizeBytes <= 0")
	}

	// Invalid presign expiry
	invalidPresignConfig := validConfig
	invalidPresignConfig.PresignTokenExpirySeconds = -10
	if err := invalidPresignConfig.Validate(); err == nil {
		t.Fatal("expected error on PresignTokenExpirySeconds <= 0")
	}
}

func TestFilestorageConfigManagerMemoryUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	configManager := NewConfigManager(kernel)
	if configManager == nil {
		t.Fatal("expected non-nil config manager")
	}

	currentConfig := configManager.Get()
	if !currentConfig.Enabled {
		t.Fatal("expected default settings in new config manager")
	}

	updatedConfig := currentConfig
	updatedConfig.ChunkSizeBytes = 1048576 // 1MB
	configManager.SetMemoryConfig(updatedConfig)

	if configManager.Get().ChunkSizeBytes != 1048576 {
		t.Fatalf("expected updated chunk size 1048576, got %d", configManager.Get().ChunkSizeBytes)
	}

	// Test Set with invalid config
	invalidConfig := updatedConfig
	invalidConfig.ChunkSizeBytes = -1
	if err := configManager.Set(context.Background(), invalidConfig); err == nil {
		t.Fatal("expected error when setting invalid config")
	}
}

func TestFilestorageConfigManagerEventAndContractUnit(t *testing.T) {
	if ConfigKey != "runtime" {
		t.Fatalf("expected ConfigKey to be 'runtime', got %q", ConfigKey)
	}

	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	configManager := NewConfigManager(kernel)

	invalidConfig := DefaultConfig()
	invalidConfig.ChunkSizeBytes = 0
	err := configManager.Set(context.Background(), invalidConfig)
	if err == nil || !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got: %v", err)
	}
}
