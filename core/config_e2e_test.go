package core

import (
	"strings"
	"testing"
)

func TestCoreConfigQuickstartE2E(t *testing.T) {
	t.Cleanup(UnloadConfig)
	validHexKey := strings.Repeat("2", 64)

	// Scenario 1: Zero-Config Quickstart Generation
	// Running `layr up` uses defaults with database.url: ".layr/data" and environment overrides.
	t.Setenv("MASTER_ENCRYPTION_KEY", validHexKey)
	t.Setenv("LAYR__PROJECT__NAME", "quickstart-demo")
	t.Setenv("LAYR__PROJECT__DESCRIPTION", "Automated quickstart e2e validation")
	t.Setenv("LAYR__SERVER__LISTEN_ADDR", ":9090")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("failed to load quickstart config: %v", err)
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("validation failed for quickstart config: %v", err)
	}

	if config.Server.ListenAddr != ":9090" {
		t.Fatalf("expected listen addr overridden to :9090 by environment, got %s", config.Server.ListenAddr)
	}
	if config.Project.Name != "quickstart-demo" {
		t.Fatalf("expected project name 'quickstart-demo', got %s", config.Project.Name)
	}
	if !config.IsServiceEnabled("data") || !config.IsServiceEnabled("auth") || !config.IsServiceEnabled("file_storage") || !config.IsServiceEnabled("console") {
		t.Fatal("expected quickstart default services to be enabled")
	}
	if len(config.GetEnabledServices()) != 7 {
		t.Fatalf("expected 7 default services enabled, got %d", len(config.GetEnabledServices()))
	}
}

func TestCoreConfigMicroservicePodProfilesE2E(t *testing.T) {
	validHexKey := strings.Repeat("3", 64)

	// Scenario 2: Dedicated Auth Microservice Pod (`layr up auth`)
	authPodConfig := DefaultConfig()
	authPodConfig.Security.MasterEncryptionKey = validHexKey
	authPodConfig.SetEnabledServices([]string{"auth"})
	if err := authPodConfig.Validate(); err != nil {
		t.Fatalf("failed to validate auth pod config: %v", err)
	}
	if len(authPodConfig.GetFunctionalServices()) != 1 || authPodConfig.GetFunctionalServices()[0] != "auth" {
		t.Fatalf("expected exactly [auth] functional service, got %v", authPodConfig.GetFunctionalServices())
	}

	// Scenario 3: Dedicated FileStorage + Image Pod (`layr up file_storage image`)
	mediaPodConfig := DefaultConfig()
	mediaPodConfig.Security.MasterEncryptionKey = validHexKey
	mediaPodConfig.SetEnabledServices([]string{"file_storage", "image"})
	if err := mediaPodConfig.Validate(); err != nil {
		t.Fatalf("failed to validate media pod config: %v", err)
	}
	if len(mediaPodConfig.GetFunctionalServices()) != 2 {
		t.Fatalf("expected exactly 2 functional services, got %v", mediaPodConfig.GetFunctionalServices())
	}

	// Scenario 4: Full Monolith BaaS Pod (`layr up`)
	monolithConfig := DefaultConfig()
	monolithConfig.Security.MasterEncryptionKey = validHexKey
	monolithConfig.SetEnabledServices([]string{"data", "auth", "tasks", "file_storage", "image", "function"})
	if err := monolithConfig.Validate(); err != nil {
		t.Fatalf("failed to validate monolith config: %v", err)
	}
	if len(monolithConfig.GetFunctionalServices()) != 6 {
		t.Fatalf("expected exactly 6 functional services, got %v", monolithConfig.GetFunctionalServices())
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
	zeroServiceConfig.Security.MasterEncryptionKey = validHexKey
	zeroServiceConfig.SetEnabledServices([]string{"console"})

	err := zeroServiceConfig.Validate()
	if err == nil {
		t.Fatal("expected process startup validation to fail when zero functional services are enabled")
	}
	if !strings.Contains(err.Error(), "minimum functional service invariant violated") {
		t.Fatalf("expected error message mentioning minimum functional service invariant, got: %s", err.Error())
	}
}
