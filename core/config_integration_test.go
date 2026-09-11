package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreConfigLoadFromDiskExhaustiveIntegration(t *testing.T) {
	validHexKey := strings.Repeat("0", 64)

	// 1. Non-existent file returns default config with zero error
	loadedDefaultConfig, defaultErr := LoadConfig("non_existent_file_path.yaml")
	if defaultErr != nil || loadedDefaultConfig == nil {
		t.Fatalf("expected default configuration on non-existent file, got %v", defaultErr)
	}
	if loadedDefaultConfig.Project.Name != "layr-app" {
		t.Fatalf("expected default project name 'layr-app', got '%s'", loadedDefaultConfig.Project.Name)
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

	// 4. Valid Complete YAML configuration file
	completeYAMLFile := filepath.Join(temporaryDirectory, "complete_layr.yaml")
	completeYAMLContent := `
version: "1"
project:
  name: "production-app"
  description: "Enterprise multi-service setup"
server:
  listen_addr: ":8443"
  base_url: "https://production-app.internal:8443"
database:
  url: "postgres://dbuser:p4ssword@db-primary.internal:5432/layr_prod?sslmode=require"
  max_connections: 50
  min_connections: 5
  connection_timeout_ms: 10000
  idle_timeout_ms: 600000
  max_lifetime_ms: 3600000
  health_check_period_ms: 30000
  ssl_mode: "require"
security:
  master_encryption_key: "` + validHexKey + `"
kv_store:
  backend: "redis"
  url: "redis://redis-master.internal:6379"
data:
  enabled: true
auth:
  enabled: true
tasks:
  enabled: true
file_storage:
  enabled: true
notification:
  enabled: true
analytics:
  enabled: true
image:
  enabled: true
console:
  enabled: true
  initial_user_email: "console_user@layr.sh"
  initial_user_password: "super_secure_initial_password"
`
	if writeErr := os.WriteFile(completeYAMLFile, []byte(completeYAMLContent), 0600); writeErr != nil {
		t.Fatalf("failed to write complete yaml file: %v", writeErr)
	}

	loadedCompleteConfig, completeErr := LoadConfig(completeYAMLFile)
	if completeErr != nil {
		t.Fatalf("failed to load complete yaml config: %v", completeErr)
	}
	if valueErr := loadedCompleteConfig.Validate(); valueErr != nil {
		t.Fatalf("validation failed on valid complete yaml config: %v", valueErr)
	}

	if loadedCompleteConfig.Project.Name != "production-app" ||
		loadedCompleteConfig.Server.ListenAddr != ":8443" ||
		loadedCompleteConfig.Database.MaxConnections != 50 ||
		loadedCompleteConfig.KVStore.Backend != "redis" ||
		len(loadedCompleteConfig.GetFunctionalServices()) != 7 {
		t.Fatalf("complete config fields mismatch: %+v", loadedCompleteConfig)
	}

	// 5. Minimal YAML configuration file (omitted fields use defaults)
	minimalYAMLFile := filepath.Join(temporaryDirectory, "minimal_layr.yaml")
	minimalYAMLContent := `
version: "1"
security:
  master_encryption_key: "` + validHexKey + `"
data:
  enabled: true
`
	if minWriteErr := os.WriteFile(minimalYAMLFile, []byte(minimalYAMLContent), 0600); minWriteErr != nil {
		t.Fatalf("failed to write minimal yaml file: %v", minWriteErr)
	}

	loadedMinimalConfig, minErr := LoadConfig(minimalYAMLFile)
	if minErr != nil {
		t.Fatalf("failed to load minimal yaml config: %v", minErr)
	}
	if minValueErr := loadedMinimalConfig.Validate(); minValueErr != nil {
		t.Fatalf("validation failed on valid minimal yaml config: %v", minValueErr)
	}
	if loadedMinimalConfig.Project.Name != "layr-app" || loadedMinimalConfig.Server.ListenAddr != ":8080" || loadedMinimalConfig.Database.URL != ".layr/data" {
		t.Fatal("expected minimal config to retain defaults for omitted fields")
	}

	// 6. Malformed YAML syntax error
	malformedYAMLFile := filepath.Join(temporaryDirectory, "malformed.yaml")
	_ = os.WriteFile(malformedYAMLFile, []byte("version: [unclosed_sequence\nproject: {invalid"), 0600)
	if _, err := LoadConfig(malformedYAMLFile); err == nil {
		t.Fatal("expected error on malformed YAML syntax")
	}

	// 7. YAML with invalid data types (e.g. string for integer max_connections)
	invalidTypesYAMLFile := filepath.Join(temporaryDirectory, "invalid_types.yaml")
	_ = os.WriteFile(invalidTypesYAMLFile, []byte("version: '1'\ndatabase:\n  max_connections: 'not_an_int'"), 0600)
	if _, err := LoadConfig(invalidTypesYAMLFile); err == nil {
		t.Fatal("expected error when unmarshaling invalid data type into max_connections")
	}
}

func TestCoreConfigLoadWithEnvironmentOverlayIntegration(t *testing.T) {
	temporaryDirectory := t.TempDir()
	yamlFile := filepath.Join(temporaryDirectory, "layr.yaml")
	validHexKey := strings.Repeat("1", 64)

	yamlContent := `
version: "1"
project:
  name: "file-name"
server:
  listen_addr: ":3000"
database:
  url: "postgres://file@localhost/db"
security:
  master_encryption_key: "` + validHexKey + `"
data:
  enabled: true
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	// Set environment variables to test overlay precedence over file values
	t.Setenv("LAYR__PROJECT__NAME", "env-precedence-name")
	t.Setenv("LAYR__SERVER__LISTEN_ADDR", ":5000")

	loadedConfig, err := LoadConfig(yamlFile)
	if err != nil {
		t.Fatalf("failed to load yaml: %v", err)
	}

	// LAYR__SERVER__LISTEN_ADDR should take precedence over file value (:3000)
	if loadedConfig.Project.Name != "env-precedence-name" {
		t.Fatalf("expected project name 'env-precedence-name', got '%s'", loadedConfig.Project.Name)
	}
	if loadedConfig.Server.ListenAddr != ":5000" {
		t.Fatalf("expected server listen addr :5000 from LAYR__SERVER__LISTEN_ADDR, got %s", loadedConfig.Server.ListenAddr)
	}
}
