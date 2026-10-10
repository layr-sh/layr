package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreConfigLoadFromDiskExhaustiveIntegration(t *testing.T) {
	keysToCleanup := []string{
		"LAYR__PROJECT__NAME",
		"LAYR__PROJECT__DESCRIPTION",
		"LAYR__SERVER__LISTEN_ADDR",
		"LAYR__SERVER__BASE_URL",
		"LAYR__DATABASE__URL",
		"LAYR__DATABASE__MAX_CONNECTIONS",
		"LAYR__DATABASE__MIN_CONNECTIONS",
		"LAYR__DATABASE__CONNECTION_TIMEOUT_MS",
		"LAYR__DATABASE__IDLE_TIMEOUT_MS",
		"LAYR__DATABASE__MAX_LIFETIME_MS",
		"LAYR__DATABASE__HEALTH_CHECK_PERIOD_MS",
		"LAYR__DATABASE__SSL_MODE",
		"LAYR__SECURITY__MASTER_ENCRYPTION_KEY",
		"LAYR__KV_STORE__BACKEND",
		"LAYR__KV_STORE__URL",
		"LAYR__ROOT_USER__EMAIL",
		"LAYR__ROOT_USER__PASSWORD",
	}
	t.Cleanup(func() {
		UnloadConfig()
		for _, key := range keysToCleanup {
			_ = os.Unsetenv(key)
		}
	})
	validHexKey := strings.Repeat("0", 64)

	// 1. Non-existent file returns default config with zero error
	loadedDefaultConfig, defaultErr := LoadConfig("non_existent_file_path.env")
	if defaultErr != nil || loadedDefaultConfig == nil {
		t.Fatalf("expected default configuration on non-existent file, got %v", defaultErr)
	}
	if loadedDefaultConfig.Project.Name != "Layr App" {
		t.Fatalf("expected default project name 'Layr App', got '%s'", loadedDefaultConfig.Project.Name)
	}

	// 2. Empty string path returns default config
	loadedEmptyConfig, emptyErr := LoadConfig("")
	if emptyErr != nil || loadedEmptyConfig == nil {
		t.Fatalf("expected default configuration on empty path, got %v", emptyErr)
	}

	// 3. Directory path triggers read error
	temporaryDirectory := t.TempDir()
	if _, dirErr := LoadConfig(temporaryDirectory); dirErr == nil {
		t.Fatal("expected error when attempting to read directory as file")
	}

	// 4. Valid Complete .env configuration file
	completeEnvFile := filepath.Join(temporaryDirectory, "complete.env")
	completeEnvContent := `
LAYR__PROJECT__NAME=production-app
LAYR__PROJECT__DESCRIPTION="Enterprise multi-service setup"
LAYR__SERVER__LISTEN_ADDR=:8443
LAYR__SERVER__BASE_URL=https://production-app.internal:8443
LAYR__DATABASE__URL=postgres://dbuser:p4ssword@db-primary.internal:5432/layr_prod?sslmode=require
LAYR__DATABASE__MAX_CONNECTIONS=50
LAYR__DATABASE__MIN_CONNECTIONS=5
LAYR__DATABASE__CONNECTION_TIMEOUT_MS=10000
LAYR__DATABASE__IDLE_TIMEOUT_MS=600000
LAYR__DATABASE__MAX_LIFETIME_MS=3600000
LAYR__DATABASE__HEALTH_CHECK_PERIOD_MS=30000
LAYR__DATABASE__SSL_MODE=require
LAYR__SECURITY__MASTER_ENCRYPTION_KEY=` + validHexKey + `
LAYR__KV_STORE__BACKEND=redis
LAYR__KV_STORE__URL=redis://redis-master.internal:6379
LAYR__ROOT_USER__EMAIL=root_user@layr.sh
LAYR__ROOT_USER__PASSWORD=super_secure_initial_password
`
	if writeErr := os.WriteFile(completeEnvFile, []byte(completeEnvContent), 0600); writeErr != nil {
		t.Fatalf("failed to write complete env file: %v", writeErr)
	}

	loadedCompleteConfig, completeErr := LoadConfig(completeEnvFile)
	if completeErr != nil {
		t.Fatalf("failed to load complete env config: %v", completeErr)
	}
	if valueErr := loadedCompleteConfig.Validate(); valueErr != nil {
		t.Fatalf("validation failed on valid complete env config: %v", valueErr)
	}

	if loadedCompleteConfig.Project.Name != "production-app" ||
		loadedCompleteConfig.Server.ListenAddr != ":8443" ||
		loadedCompleteConfig.Database.MaxConnections != 50 ||
		loadedCompleteConfig.KVStore.Backend != "redis" ||
		loadedCompleteConfig.RootUser.Email != "root_user@layr.sh" ||
		loadedCompleteConfig.RootUser.Password != "super_secure_initial_password" ||
		len(loadedCompleteConfig.GetFunctionalServices()) != 6 {
		t.Fatalf("complete config fields mismatch: %+v", loadedCompleteConfig)
	}

	for _, key := range keysToCleanup {
		_ = os.Unsetenv(key)
	}

	// 5. Minimal .env configuration file (omitted fields use defaults)
	minimalEnvFile := filepath.Join(temporaryDirectory, "minimal.env")
	minimalEnvContent := `
MASTER_ENCRYPTION_KEY=` + validHexKey + `
`
	if minWriteErr := os.WriteFile(minimalEnvFile, []byte(minimalEnvContent), 0600); minWriteErr != nil {
		t.Fatalf("failed to write minimal env file: %v", minWriteErr)
	}

	loadedMinimalConfig, minErr := LoadConfig(minimalEnvFile)
	if minErr != nil {
		t.Fatalf("failed to load minimal env config: %v", minErr)
	}
	if minValueErr := loadedMinimalConfig.Validate(); minValueErr != nil {
		t.Fatalf("validation failed on valid minimal env config: %v", minValueErr)
	}
	if loadedMinimalConfig.Project.Name != "Layr App" || loadedMinimalConfig.Server.ListenAddr != ":5297" || loadedMinimalConfig.Database.URL != ".layr/data" {
		t.Fatal("expected minimal config to retain defaults for omitted fields")
	}
}

func TestCoreConfigLoadWithEnvironmentOverlayIntegration(t *testing.T) {
	t.Cleanup(func() {
		UnloadConfig()
		_ = os.Unsetenv("LAYR__DATABASE__URL")
		_ = os.Unsetenv("LAYR__SECURITY__MASTER_ENCRYPTION_KEY")
		_ = os.Unsetenv("LAYR__PROJECT__NAME")
		_ = os.Unsetenv("LAYR__SERVER__LISTEN_ADDR")
	})
	temporaryDirectory := t.TempDir()
	envFile := filepath.Join(temporaryDirectory, ".env")
	validHexKey := strings.Repeat("1", 64)

	envContent := `
LAYR__PROJECT__NAME=file-name
LAYR__SERVER__LISTEN_ADDR=:3000
LAYR__DATABASE__URL=postgres://file@localhost/db
LAYR__SECURITY__MASTER_ENCRYPTION_KEY=` + validHexKey + `
`
	if err := os.WriteFile(envFile, []byte(envContent), 0600); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	// Set environment variables to test overlay precedence over file values
	t.Setenv("LAYR__PROJECT__NAME", "env-precedence-name")
	t.Setenv("LAYR__SERVER__LISTEN_ADDR", ":5000")

	loadedConfig, err := LoadConfig(envFile)
	if err != nil {
		t.Fatalf("failed to load env config: %v", err)
	}

	// LAYR__SERVER__LISTEN_ADDR should take precedence over file value (:3000)
	if loadedConfig.Project.Name != "env-precedence-name" {
		t.Fatalf("expected project name 'env-precedence-name', got '%s'", loadedConfig.Project.Name)
	}
	if loadedConfig.Server.ListenAddr != ":5000" {
		t.Fatalf("expected server listen addr :5000 from LAYR__SERVER__LISTEN_ADDR, got %s", loadedConfig.Server.ListenAddr)
	}
}
