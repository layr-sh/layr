package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreConfigQuickstartE2E(t *testing.T) {
	temporaryDirectory := t.TempDir()
	configPath := filepath.Join(temporaryDirectory, "layr.yaml")
	validHexKey := strings.Repeat("2", 64)

	// Scenario 1: Zero-Config Quickstart Generation
	// Running `layr start` auto-generates ./layr.yaml with database.url: ".layr/data", master_encryption_key, and enabled services.
	quickstartContent := `
version: "1"
project:
  name: "quickstart-demo"
  description: "Automated quickstart e2e validation"
server:
  listen_addr: ":8080"
  base_url: "http://localhost:8080"
database:
  url: ".layr/data"
security:
  master_encryption_key: "` + validHexKey + `"
data:
  enabled: true
auth:
  enabled: true
file_storage:
  enabled: true
console:
  enabled: true
`
	if err := os.WriteFile(configPath, []byte(quickstartContent), 0600); err != nil {
		t.Fatalf("failed to write quickstart config file: %v", err)
	}

	// Set dynamic environment overrides to test end-to-end precedence
	t.Setenv("LAYR__SERVER__LISTEN_ADDR", ":9090")

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load quickstart config: %v", err)
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("validation failed for quickstart config: %v", err)
	}

	if config.Server.ListenAddr != ":9090" {
		t.Fatalf("expected listen addr overridden to :9090 by environment, got %s", config.Server.ListenAddr)
	}
	if !config.Data.Enabled || !config.Auth.Enabled || !config.FileStorage.Enabled || !config.Console.Enabled {
		t.Fatal("expected quickstart services to be enabled")
	}
	if config.Tasks.Enabled || config.Notification.Enabled || config.Analytics.Enabled || config.Image.Enabled {
		t.Fatal("expected non-quickstart services to remain disabled")
	}
}

func TestCoreConfigMicroservicePodProfilesE2E(t *testing.T) {
	validHexKey := strings.Repeat("3", 64)

	// Scenario 2: Dedicated Auth Microservice Pod (`layr start auth`)
	authPodConfig := DefaultConfig()
	authPodConfig.Data.Enabled = false
	authPodConfig.FileStorage.Enabled = false
	authPodConfig.Security.MasterEncryptionKey = validHexKey
	authPodConfig.Console.Enabled = false
	if err := authPodConfig.EnableService("auth"); err != nil {
		t.Fatalf("failed to enable auth service: %v", err)
	}
	if err := authPodConfig.Validate(); err != nil {
		t.Fatalf("failed to validate auth pod config: %v", err)
	}
	if len(authPodConfig.GetFunctionalServices()) != 1 || authPodConfig.GetFunctionalServices()[0] != "auth" {
		t.Fatalf("expected exactly [auth] functional service, got %v", authPodConfig.GetFunctionalServices())
	}

	// Scenario 3: Dedicated FileStorage + Image Pod (`layr start file_storage image`)
	mediaPodConfig := DefaultConfig()
	mediaPodConfig.Data.Enabled = false
	mediaPodConfig.Auth.Enabled = false
	mediaPodConfig.Security.MasterEncryptionKey = validHexKey
	mediaPodConfig.Console.Enabled = false
	_ = mediaPodConfig.EnableService("file_storage")
	_ = mediaPodConfig.EnableService("image")
	if err := mediaPodConfig.Validate(); err != nil {
		t.Fatalf("failed to validate media pod config: %v", err)
	}
	if len(mediaPodConfig.GetFunctionalServices()) != 2 {
		t.Fatalf("expected exactly 2 functional services, got %v", mediaPodConfig.GetFunctionalServices())
	}

	// Scenario 4: Full Monolith BaaS Pod (`layr start`)
	monolithConfig := DefaultConfig()
	monolithConfig.Security.MasterEncryptionKey = validHexKey
	for _, serviceName := range []string{"data", "auth", "tasks", "file_storage", "notification", "analytics", "image"} {
		_ = monolithConfig.EnableService(serviceName)
	}
	if err := monolithConfig.Validate(); err != nil {
		t.Fatalf("failed to validate monolith config: %v", err)
	}
	if len(monolithConfig.GetFunctionalServices()) != 7 {
		t.Fatalf("expected exactly 7 functional services, got %v", monolithConfig.GetFunctionalServices())
	}
}

func TestCoreConfigProcessStartupRejectionE2E(t *testing.T) {
	validHexKey := strings.Repeat("4", 64)

	// Scenario 5: Process startup rejected when zero functional services are enabled
	// Even if Console is enabled, Layr halts with ConfigFailureExitCode (78)
	if ConfigFailureExitCode != 78 {
		t.Fatalf("expected ConfigFailureExitCode to equal 78 according to sysexits.h, got %d", ConfigFailureExitCode)
	}

	zeroServiceConfig := DefaultConfig()
	zeroServiceConfig.Data.Enabled = false
	zeroServiceConfig.Auth.Enabled = false
	zeroServiceConfig.FileStorage.Enabled = false
	zeroServiceConfig.Security.MasterEncryptionKey = validHexKey
	zeroServiceConfig.Console.Enabled = true

	err := zeroServiceConfig.Validate()
	if err == nil {
		t.Fatal("expected process startup validation to fail when zero functional services are enabled")
	}
	if !strings.Contains(err.Error(), "minimum functional service invariant violated") {
		t.Fatalf("expected error message mentioning minimum functional service invariant, got: %s", err.Error())
	}
}
