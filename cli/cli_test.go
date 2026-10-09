package cli

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"layr.sh/core"
)

func resetTestState(t *testing.T) {
	t.Helper()
	originalExitFunc := exitFunc
	originalStdoutWriter := stdoutWriter
	originalStderrWriter := stderrWriter

	t.Cleanup(func() {
		exitFunc = originalExitFunc
		stdoutWriter = originalStdoutWriter
		stderrWriter = originalStderrWriter
		core.UnloadConfig()
	})
}

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestCLIRunHelpUnit(t *testing.T) {
	resetTestState(t)
	outputBuffer := &bytes.Buffer{}
	var bufferWriter io.Writer = outputBuffer
	stdoutWriter = bufferWriter

	helpArgs := [][]string{
		{"-h"},
		{"--help"},
		{"help"},
		{"up", "-h"},
	}

	for _, args := range helpArgs {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			outputBuffer.Reset()
			if err := Run(args); err != nil {
				t.Fatalf("expected nil error for help, got: %v", err)
			}
			if !strings.Contains(outputBuffer.String(), "Usage: layr <command>") {
				t.Fatalf("expected usage help in output, got: %s", outputBuffer.String())
			}
		})
	}
}

func TestCLIParseArgsErrorsUnit(t *testing.T) {
	resetTestState(t)

	errorCases := []struct {
		name        string
		args        []string
		expectedErr string
	}{
		{
			name:        "empty arguments",
			args:        []string{},
			expectedErr: "missing command: expected 'up'",
		},
		{
			name:        "missing config argument for -c",
			args:        []string{"-c"},
			expectedErr: "flag -c requires an argument",
		},
		{
			name:        "missing config argument for --config",
			args:        []string{"--config"},
			expectedErr: "flag --config requires an argument",
		},
		{
			name:        "missing env file argument for --env-file",
			args:        []string{"--env-file"},
			expectedErr: "flag --env-file requires an argument",
		},
		{
			name:        "unknown flag",
			args:        []string{"--unknown-flag"},
			expectedErr: "unknown flag: --unknown-flag",
		},
		{
			name:        "disallowed start command",
			args:        []string{"start"},
			expectedErr: "unknown command \"start\": expected 'up'",
		},
		{
			name:        "unknown command",
			args:        []string{"restart"},
			expectedErr: "unknown command \"restart\": expected 'up'",
		},
	}

	for _, testCase := range errorCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := Run(testCase.args)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", testCase.expectedErr)
			}
			if !strings.Contains(err.Error(), testCase.expectedErr) {
				t.Fatalf("expected error containing %q, got %q", testCase.expectedErr, err.Error())
			}
		})
	}
}

func TestCLIParseArgsFlagVariationsUnit(t *testing.T) {
	resetTestState(t)

	temporaryDirectory := t.TempDir()
	configPath := filepath.Join(temporaryDirectory, "custom.yaml")
	validConfig := core.DefaultConfig()
	validConfig.Database.URL = "postgres://invalid:5432/db"
	validConfig.Security.MasterEncryptionKey = strings.Repeat("f", 64)
	if err := core.WriteConfigFile(configPath, *validConfig); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	flagCases := [][]string{
		{"up", "-c", configPath},
		{"up", "--config", configPath},
		{"up", "-c=" + configPath},
		{"up", "--config=" + configPath},
		{"-c", configPath, "up"},
	}

	for _, args := range flagCases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			_ = Execute(cancelledContext(), args)
		})
	}
}

func TestCLILoadEnvFileUnit(t *testing.T) {
	temporaryDirectory := t.TempDir()
	envPath := filepath.Join(temporaryDirectory, ".env.test")

	content := `
# Comment line
KEY_PLAIN=value_plain
KEY_DOUBLE="value_double"
KEY_SINGLE='value_single'
KEY_TRIMMED =  value_trimmed  
INVALID_LINE_NO_EQUALS
# Another comment
`
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write test env file: %v", err)
	}

	if err := LoadEnvFile(envPath); err != nil {
		t.Fatalf("LoadEnvFile failed: %v", err)
	}

	if envValue := os.Getenv("KEY_PLAIN"); envValue != "value_plain" {
		t.Fatalf("expected KEY_PLAIN to be value_plain, got %q", envValue)
	}
	if envValue := os.Getenv("KEY_DOUBLE"); envValue != "value_double" {
		t.Fatalf("expected KEY_DOUBLE to be value_double, got %q", envValue)
	}
	if envValue := os.Getenv("KEY_SINGLE"); envValue != "value_single" {
		t.Fatalf("expected KEY_SINGLE to be value_single, got %q", envValue)
	}
	if envValue := os.Getenv("KEY_TRIMMED"); envValue != "value_trimmed" {
		t.Fatalf("expected KEY_TRIMMED to be value_trimmed, got %q", envValue)
	}

	// Missing env file
	missingErr := LoadEnvFile(filepath.Join(temporaryDirectory, "nonexistent.env"))
	if missingErr == nil {
		t.Fatal("expected error reading nonexistent env file, got nil")
	}

	// Invalid environment variable key that fails os.Setenv (contains null byte)
	badKeyPath := filepath.Join(temporaryDirectory, ".env.bad")
	if err := os.WriteFile(badKeyPath, []byte("\x00BAD=value"), 0o600); err != nil {
		t.Fatalf("failed to write bad env file: %v", err)
	}
	badKeyErr := LoadEnvFile(badKeyPath)
	if badKeyErr == nil {
		t.Fatal("expected error setting invalid environment variable, got nil")
	}
}

func TestCLIRunEnvFileFlagVariationsUnit(t *testing.T) {
	resetTestState(t)

	temporaryDirectory := t.TempDir()
	envPath := filepath.Join(temporaryDirectory, ".env.prod")
	if err := os.WriteFile(envPath, []byte("LAYR__PROJECT__NAME=ProdApp\n"), 0o600); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	configPath := filepath.Join(temporaryDirectory, "layr.yaml")
	validConfig := core.DefaultConfig()
	validConfig.Database.URL = "postgres://invalid:5432/db"
	validConfig.Security.MasterEncryptionKey = strings.Repeat("f", 64)
	if err := core.WriteConfigFile(configPath, *validConfig); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Test --env-file flag
	_ = Execute(cancelledContext(), []string{"--env-file", envPath, "-c", configPath, "up"})

	// Test --env-file= flag
	_ = Execute(cancelledContext(), []string{"--env-file=" + envPath, "-c", configPath, "up"})

	// Test nonexistent env file error
	if err := Run([]string{"--env-file", "nonexistent.env", "-c", configPath, "up"}); err == nil {
		t.Fatal("expected error with nonexistent env file, got nil")
	}
}

func TestCLIResolveConfigPathUnit(t *testing.T) {
	resetTestState(t)

	// 1. Custom path explicitly provided
	path, err := resolveConfigPath("/custom/layr.yaml")
	if err != nil || path != "/custom/layr.yaml" {
		t.Fatalf("expected /custom/layr.yaml, got %s (err: %v)", path, err)
	}

	// 2. Local ./layr.yaml exists
	temporaryDirectory := t.TempDir()
	t.Chdir(temporaryDirectory)

	localConfigFile := filepath.Join(temporaryDirectory, "layr.yaml")
	if writeErr := os.WriteFile(localConfigFile, []byte("version: \"1\"\n"), 0o600); writeErr != nil {
		t.Fatalf("failed to write local config: %v", writeErr)
	}

	path, err = resolveConfigPath("")
	if err != nil || path != "layr.yaml" {
		t.Fatalf("expected layr.yaml, got %s (err: %v)", path, err)
	}

	// Remove local file
	_ = os.Remove(localConfigFile)

	// 3. DATABASE_URL is set in environment -> return empty path without bootstrap
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	path, err = resolveConfigPath("")
	if err != nil || path != "" {
		t.Fatalf("expected empty path when DATABASE_URL is set, got %s (err: %v)", path, err)
	}
	t.Setenv("DATABASE_URL", "")

	// 4. Zero-Config Bootstrap: neither file exists and DATABASE_URL is empty
	path, err = resolveConfigPath("")
	if err != nil || path != "layr.yaml" {
		t.Fatalf("expected bootstrapped layr.yaml, got %s (err: %v)", path, err)
	}
	if _, statErr := os.Stat("layr.yaml"); statErr != nil {
		t.Fatalf("expected bootstrapped layr.yaml to exist on disk: %v", statErr)
	}
}

func TestCLIRunMonolithModeUnit(t *testing.T) {
	resetTestState(t)
	temporaryDirectory := t.TempDir()
	t.Chdir(temporaryDirectory)

	configPath := filepath.Join(temporaryDirectory, "layr.yaml")
	validConfig := core.DefaultConfig()
	validConfig.Database.URL = "postgres://invalid:5432/db"
	validConfig.Security.MasterEncryptionKey = strings.Repeat("f", 64)
	if err := core.WriteConfigFile(configPath, *validConfig); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Zero arguments (monolith)
	_ = Execute(cancelledContext(), []string{"up"})

	// "all" argument (monolith)
	_ = Execute(cancelledContext(), []string{"up", "all"})
}

func TestCLIRunValidationFailureExitCodeUnit(t *testing.T) {
	resetTestState(t)
	temporaryDirectory := t.TempDir()
	t.Chdir(temporaryDirectory)

	configPath := filepath.Join(temporaryDirectory, "layr.yaml")
	if err := core.WriteConfigFile(configPath); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	var capturedExitCode int
	exitFunc = func(code int) {
		capturedExitCode = code
	}

	errorBuffer := &bytes.Buffer{}
	var bufferWriter io.Writer = errorBuffer
	stderrWriter = bufferWriter

	// 1. Service disabled by default in layr.yaml: tasks
	capturedExitCode = 0
	errorBuffer.Reset()
	err := Run([]string{"up", "tasks"})
	if err == nil {
		t.Fatal("expected error starting disabled service tasks, got nil")
	}
	if capturedExitCode != ExitCodeConfig {
		t.Fatalf("expected exit code %d, got %d", ExitCodeConfig, capturedExitCode)
	}
	if !strings.Contains(errorBuffer.String(), "service \"tasks\" is disabled in configuration") {
		t.Fatalf("expected error output for disabled service, got: %s", errorBuffer.String())
	}

	// 2. Service not in layr.yaml: notification
	capturedExitCode = 0
	errorBuffer.Reset()
	err = Run([]string{"up", "notification"})
	if err == nil {
		t.Fatal("expected error starting nonexistent service notification, got nil")
	}
	if capturedExitCode != ExitCodeConfig {
		t.Fatalf("expected exit code %d, got %d", ExitCodeConfig, capturedExitCode)
	}

	// 3. Service not in layr.yaml: analytics
	capturedExitCode = 0
	errorBuffer.Reset()
	err = Run([]string{"up", "analytics"})
	if err == nil {
		t.Fatal("expected error starting nonexistent service analytics, got nil")
	}
	if capturedExitCode != ExitCodeConfig {
		t.Fatalf("expected exit code %d, got %d", ExitCodeConfig, capturedExitCode)
	}

	// 4. Multiple services where one is disabled: data (enabled) + image (disabled)
	capturedExitCode = 0
	errorBuffer.Reset()
	err = Run([]string{"up", "data", "image"})
	if err == nil {
		t.Fatal("expected error starting subset with disabled image, got nil")
	}
	if capturedExitCode != ExitCodeConfig {
		t.Fatalf("expected exit code %d, got %d", ExitCodeConfig, capturedExitCode)
	}
}

func TestCLIRunSubsetDedicatedServiceUnit(t *testing.T) {
	resetTestState(t)
	temporaryDirectory := t.TempDir()
	t.Chdir(temporaryDirectory)

	// Create config with data, auth, file_storage, and tasks enabled
	validConfig := core.DefaultConfig()
	validConfig.Database.URL = "postgres://invalid:5432/db"
	validConfig.Data.Enabled = true
	validConfig.Auth.Enabled = true
	validConfig.FileStorage.Enabled = true
	validConfig.Tasks.Enabled = true
	validConfig.Security.MasterEncryptionKey = strings.Repeat("e", 64)

	configPath := filepath.Join(temporaryDirectory, "layr.yaml")
	if err := core.WriteConfigFile(configPath, *validConfig); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// 1. Single service argument: layr up auth -> only auth enabled
	_ = Execute(cancelledContext(), []string{"up", "auth"})
	activeConfigServices := core.GetConfig().GetEnabledServices()
	if len(activeConfigServices) != 1 || activeConfigServices[0] != "auth" {
		t.Fatalf("expected only auth enabled, got: %v", activeConfigServices)
	}

	// 2. Multiple service arguments: layr up file_storage tasks
	_ = Execute(cancelledContext(), []string{"up", "file_storage", "tasks"})
	activeConfigServices = core.GetConfig().GetEnabledServices()
	expectedMap := map[string]bool{"file_storage": true, "tasks": true}
	if len(activeConfigServices) != 2 {
		t.Fatalf("expected 2 active services, got: %v", activeConfigServices)
	}
	for index := 0; index < len(activeConfigServices); index++ {
		service := activeConfigServices[index]
		if !expectedMap[service] {
			t.Fatalf("unexpected active service: %s", service)
		}
	}
}

func TestCLIRunExecutionErrorsUnit(t *testing.T) {
	resetTestState(t)
	temporaryDirectory := t.TempDir()
	t.Chdir(temporaryDirectory)

	// 1. Corrupt yaml -> core.LoadConfig failure
	corruptConfigPath := filepath.Join(temporaryDirectory, "corrupt.yaml")
	if err := os.WriteFile(corruptConfigPath, []byte("invalid: yaml: : :"), 0o600); err != nil {
		t.Fatalf("failed to write corrupt yaml: %v", err)
	}
	loadErr := Run([]string{"-c", corruptConfigPath, "up"})
	if loadErr == nil || !strings.Contains(loadErr.Error(), "failed to load configuration") {
		t.Fatalf("expected load configuration error, got: %v", loadErr)
	}

	// 2. Invalid master encryption key -> core.NewKernel failure
	invalidKeyConfig := core.DefaultConfig()
	invalidKeyConfig.Security.MasterEncryptionKey = "too_short"
	invalidKeyPath := filepath.Join(temporaryDirectory, "invalid_key.yaml")
	if err := core.WriteConfigFile(invalidKeyPath, *invalidKeyConfig); err != nil {
		t.Fatalf("failed to write invalid key config: %v", err)
	}
	kernelErr := Run([]string{"-c", invalidKeyPath, "up"})
	if kernelErr == nil || !strings.Contains(kernelErr.Error(), "failed to initialize kernel") {
		t.Fatalf("expected kernel initialization error, got: %v", kernelErr)
	}
}

func TestCLIRunBootstrapWriteErrorUnit(t *testing.T) {
	resetTestState(t)
	temporaryDirectory := t.TempDir()
	readOnlyDirectory := filepath.Join(temporaryDirectory, "readonly")
	if err := os.Mkdir(readOnlyDirectory, 0o500); err != nil {
		t.Fatalf("failed to create read-only dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDirectory, 0o700)
	})
	t.Chdir(readOnlyDirectory)
	t.Setenv("DATABASE_URL", "")

	err := Run([]string{"up"})
	if err == nil || !strings.Contains(err.Error(), "failed to write bootstrap configuration") {
		t.Fatalf("expected bootstrap write error, got: %v", err)
	}
}

func TestCLIRunCleanShutdownUnit(t *testing.T) {
	resetTestState(t)
	temporaryDirectory := t.TempDir()
	t.Chdir(temporaryDirectory)

	configPath := filepath.Join(temporaryDirectory, "layr.yaml")
	validConfig := core.DefaultConfig()
	validConfig.Server.ListenAddr = ":18080"
	validConfig.Database.URL = filepath.Join(temporaryDirectory, "data")
	validConfig.Security.MasterEncryptionKey = strings.Repeat("a", 64)
	if err := core.WriteConfigFile(configPath, *validConfig); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errChannel := make(chan error, 1)

	go func() {
		errChannel <- Execute(ctx, []string{"up"})
	}()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(30 * time.Second)
	dialer := &net.Dialer{Timeout: 50 * time.Millisecond}

	for {
		select {
		case runErr := <-errChannel:
			if runErr != nil {
				t.Fatalf("unexpected early failure: %v", runErr)
			}
			return
		case <-timeout:
			t.Fatal("timed out waiting for kernel to start")
		case <-ticker.C:
			connection, dialErr := dialer.DialContext(ctx, "tcp", "127.0.0.1:18080")
			if dialErr == nil {
				_ = connection.Close()
				cancel()
				select {
				case runErr := <-errChannel:
					if runErr != nil {
						t.Fatalf("expected nil error on clean shutdown, got: %v", runErr)
					}
					return
				case <-time.After(10 * time.Second):
					t.Fatal("timed out waiting for shutdown")
				}
			}
		}
	}
}
