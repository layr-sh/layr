package cli

import (
	"bytes"
	"context"
	"errors"
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
	originalGenerateKeyFunc := generateKeyFunc
	originalSetenvFunc := setenvFunc
	originalLoadConfigFunc := loadConfigFunc

	t.Cleanup(func() {
		exitFunc = originalExitFunc
		stdoutWriter = originalStdoutWriter
		stderrWriter = originalStderrWriter
		generateKeyFunc = originalGenerateKeyFunc
		setenvFunc = originalSetenvFunc
		loadConfigFunc = originalLoadConfigFunc
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
	envPath := filepath.Join(temporaryDirectory, "custom.env")
	validKey := strings.Repeat("f", 64)
	if err := os.WriteFile(envPath, []byte("MASTER_ENCRYPTION_KEY="+validKey+"\n"), 0o600); err != nil {
		t.Fatalf("failed to create env file: %v", err)
	}

	flagCases := [][]string{
		{"up", "--env-file", envPath},
		{"up", "--env-file=" + envPath},
		{"--env-file", envPath, "up"},
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
	validKey := strings.Repeat("f", 64)
	envContent := "MASTER_ENCRYPTION_KEY=" + validKey + "\nLAYR__PROJECT__NAME=ProdApp\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0o600); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	// Test --env-file flag
	_ = Execute(cancelledContext(), []string{"--env-file", envPath, "up"})

	// Test --env-file= flag
	_ = Execute(cancelledContext(), []string{"--env-file=" + envPath, "up"})

	// Test auto-loading .env from current directory when --env-file is not specified
	t.Chdir(temporaryDirectory)
	dotEnvPath := filepath.Join(temporaryDirectory, ".env")
	if err := os.WriteFile(dotEnvPath, []byte(envContent), 0o600); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}
	_ = Execute(cancelledContext(), []string{"up"})

	// Test nonexistent env file error
	if err := Run([]string{"--env-file", "nonexistent.env", "up"}); err == nil {
		t.Fatal("expected error with nonexistent env file, got nil")
	}
}

func TestCLIBootstrapEnvironmentUnit(t *testing.T) {
	resetTestState(t)
	temporaryDirectory := t.TempDir()
	t.Chdir(temporaryDirectory)

	// 1. Both already set via canonical env vars -> no-op
	validKey := strings.Repeat("a", 64)
	t.Setenv("LAYR__DATABASE__URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("LAYR__SECURITY__MASTER_ENCRYPTION_KEY", validKey)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MASTER_ENCRYPTION_KEY", "")
	if err := ensureBootstrapEnvironment(""); err != nil {
		t.Fatalf("expected nil when canonical vars already set, got: %v", err)
	}

	// 2. Both already set via alias env vars -> no-op
	t.Setenv("LAYR__DATABASE__URL", "")
	t.Setenv("LAYR__SECURITY__MASTER_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "postgres://alias:pass@localhost:5432/db")
	t.Setenv("MASTER_ENCRYPTION_KEY", validKey)
	if err := ensureBootstrapEnvironment(""); err != nil {
		t.Fatalf("expected nil when alias vars already set, got: %v", err)
	}

	// 3. Both unset -> bootstraps canonical vars into default .env
	t.Setenv("LAYR__DATABASE__URL", "")
	t.Setenv("LAYR__SECURITY__MASTER_ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MASTER_ENCRYPTION_KEY", "")
	if err := ensureBootstrapEnvironment(""); err != nil {
		t.Fatalf("failed to bootstrap environment: %v", err)
	}
	bootstrappedURL := os.Getenv("LAYR__DATABASE__URL")
	if bootstrappedURL != ".layr/data" {
		t.Fatalf("expected .layr/data, got %q", bootstrappedURL)
	}
	bootstrappedKey := os.Getenv("LAYR__SECURITY__MASTER_ENCRYPTION_KEY")
	if len(bootstrappedKey) != 64 {
		t.Fatalf("expected 64-character hex key, got %q", bootstrappedKey)
	}
	dotEnvData, readDotErr := os.ReadFile(".env")
	if readDotErr != nil {
		t.Fatalf("failed to read bootstrapped .env: %v", readDotErr)
	}
	dotEnvContent := string(dotEnvData)
	if !strings.Contains(dotEnvContent, "LAYR__DATABASE__URL=.layr/data") {
		t.Fatalf("expected .env to contain canonical LAYR__DATABASE__URL, got: %s", dotEnvContent)
	}
	if !strings.Contains(dotEnvContent, "LAYR__SECURITY__MASTER_ENCRYPTION_KEY="+bootstrappedKey) {
		t.Fatalf("expected .env to contain canonical LAYR__SECURITY__MASTER_ENCRYPTION_KEY, got: %s", dotEnvContent)
	}
	if strings.Contains("\n"+dotEnvContent, "\nDATABASE_URL=") || strings.Contains("\n"+dotEnvContent, "\nMASTER_ENCRYPTION_KEY=") {
		t.Fatalf("expected .env NOT to contain alias variables, got: %s", dotEnvContent)
	}

	// 4. Custom target env file with existing content without trailing newline
	customEnvPath := filepath.Join(temporaryDirectory, "custom.env")
	if writeCustomErr := os.WriteFile(customEnvPath, []byte("EXISTING_KEY=1"), 0o600); writeCustomErr != nil {
		t.Fatalf("failed to write custom env: %v", writeCustomErr)
	}
	t.Setenv("LAYR__DATABASE__URL", "")
	t.Setenv("LAYR__SECURITY__MASTER_ENCRYPTION_KEY", "")
	if bootstrapErr := ensureBootstrapEnvironment(customEnvPath); bootstrapErr != nil {
		t.Fatalf("failed to bootstrap environment to custom file: %v", bootstrapErr)
	}
	customEnvData, readCustomErr := os.ReadFile(customEnvPath)
	if readCustomErr != nil {
		t.Fatalf("failed to read custom env: %v", readCustomErr)
	}
	if !strings.HasPrefix(string(customEnvData), "EXISTING_KEY=1\nLAYR__DATABASE__URL=.layr/data\nLAYR__SECURITY__MASTER_ENCRYPTION_KEY=") {
		t.Fatalf("expected custom env to have newline appended properly with canonical keys, got: %s", string(customEnvData))
	}

	// 5. Database setenv failure
	t.Setenv("LAYR__DATABASE__URL", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("LAYR__SECURITY__MASTER_ENCRYPTION_KEY", "")
	t.Setenv("MASTER_ENCRYPTION_KEY", "")
	setenvFunc = func(key, value string) error {
		if key == "LAYR__DATABASE__URL" {
			return errors.New("simulated setenv database failure")
		}
		return os.Setenv(key, value)
	}
	if err := ensureBootstrapEnvironment(filepath.Join(temporaryDirectory, "test_err1.env")); err == nil || !strings.Contains(err.Error(), "failed to set database url") {
		t.Fatalf("expected database setenv error, got: %v", err)
	}

	// 6. Key generation failure
	setenvFunc = os.Setenv
	t.Setenv("LAYR__DATABASE__URL", ".layr/data")
	t.Setenv("LAYR__SECURITY__MASTER_ENCRYPTION_KEY", "")
	t.Setenv("MASTER_ENCRYPTION_KEY", "")
	generateKeyFunc = func() (string, error) {
		return "", errors.New("simulated crypto failure")
	}
	if err := ensureBootstrapEnvironment(filepath.Join(temporaryDirectory, "test_err2.env")); err == nil || !strings.Contains(err.Error(), "failed to generate encryption key") {
		t.Fatalf("expected key generation failure, got: %v", err)
	}

	// 7. Encryption key setenv failure
	generateKeyFunc = core.GenerateRandomCryptoEncryptionKeyHex
	t.Setenv("LAYR__DATABASE__URL", ".layr/data")
	t.Setenv("LAYR__SECURITY__MASTER_ENCRYPTION_KEY", "")
	t.Setenv("MASTER_ENCRYPTION_KEY", "")
	setenvFunc = func(key, value string) error {
		if key == "LAYR__SECURITY__MASTER_ENCRYPTION_KEY" {
			return errors.New("simulated setenv encryption failure")
		}
		return os.Setenv(key, value)
	}
	if err := ensureBootstrapEnvironment(filepath.Join(temporaryDirectory, "test_err3.env")); err == nil || !strings.Contains(err.Error(), "failed to set encryption key") {
		t.Fatalf("expected encryption key setenv error, got: %v", err)
	}

	// 8. Read directory error in appendOrWriteEnv
	if err := appendOrWriteEnv(temporaryDirectory, "KEY", "VAL"); err == nil || !strings.Contains(err.Error(), "failed to read env file") {
		t.Fatalf("expected read error on directory path, got: %v", err)
	}

	// 9. Existing file already ending in newline (tests appending without extra newline)
	newlineEnvPath := filepath.Join(temporaryDirectory, "newline.env")
	if err := os.WriteFile(newlineEnvPath, []byte("KEY1=VAL1\n"), 0o600); err != nil {
		t.Fatalf("failed to write newline env file: %v", err)
	}
	if err := appendOrWriteEnv(newlineEnvPath, "KEY2", "VAL2"); err != nil {
		t.Fatalf("failed to append to newline env file: %v", err)
	}

	// 10. Write error on existing file (read-only file permissions)
	readOnlyFilePath := filepath.Join(temporaryDirectory, "readonly.env")
	if err := os.WriteFile(readOnlyFilePath, []byte("INIT=1\n"), 0o400); err != nil {
		t.Fatalf("failed to write read-only file: %v", err)
	}
	if err := appendOrWriteEnv(readOnlyFilePath, "FAIL_KEY", "FAIL_VAL"); err == nil || !strings.Contains(err.Error(), "failed to write env file") {
		t.Fatalf("expected write error on read-only file, got: %v", err)
	}
}

func TestCLIRunMonolithModeUnit(t *testing.T) {
	resetTestState(t)
	temporaryDirectory := t.TempDir()
	t.Chdir(temporaryDirectory)

	validKey := strings.Repeat("f", 64)
	t.Setenv("MASTER_ENCRYPTION_KEY", validKey)
	t.Setenv("DATABASE_URL", "postgres://invalid:5432/db")

	// Zero arguments (monolith)
	_ = Execute(cancelledContext(), []string{"up"})

	// "all" argument (monolith)
	_ = Execute(cancelledContext(), []string{"up", "all"})
}

func TestCLIRunValidationFailureExitCodeUnit(t *testing.T) {
	resetTestState(t)
	temporaryDirectory := t.TempDir()
	t.Chdir(temporaryDirectory)

	validKey := strings.Repeat("f", 64)
	t.Setenv("MASTER_ENCRYPTION_KEY", validKey)
	t.Setenv("DATABASE_URL", "postgres://invalid:5432/db")

	var capturedExitCode int
	exitFunc = func(code int) {
		capturedExitCode = code
	}

	errorBuffer := &bytes.Buffer{}
	var bufferWriter io.Writer = errorBuffer
	stderrWriter = bufferWriter

	// 1. Unknown service: notification
	capturedExitCode = 0
	errorBuffer.Reset()
	err := Run([]string{"up", "notification"})
	if err == nil {
		t.Fatal("expected error starting nonexistent service notification, got nil")
	}
	if capturedExitCode != ExitCodeConfig {
		t.Fatalf("expected exit code %d, got %d", ExitCodeConfig, capturedExitCode)
	}
	if !strings.Contains(errorBuffer.String(), "error: unknown service \"notification\"") {
		t.Fatalf("expected error output for unknown service, got: %s", errorBuffer.String())
	}

	// 2. Unknown service: analytics
	capturedExitCode = 0
	errorBuffer.Reset()
	err = Run([]string{"up", "analytics"})
	if err == nil {
		t.Fatal("expected error starting nonexistent service analytics, got nil")
	}
	if capturedExitCode != ExitCodeConfig {
		t.Fatalf("expected exit code %d, got %d", ExitCodeConfig, capturedExitCode)
	}

	// 3. Multiple services where one is unknown: data (valid) + unknown_srv (invalid)
	capturedExitCode = 0
	errorBuffer.Reset()
	err = Run([]string{"up", "data", "unknown_srv"})
	if err == nil {
		t.Fatal("expected error starting subset with unknown service, got nil")
	}
	if capturedExitCode != ExitCodeConfig {
		t.Fatalf("expected exit code %d, got %d", ExitCodeConfig, capturedExitCode)
	}
}

func TestCLIRunSubsetDedicatedServiceUnit(t *testing.T) {
	resetTestState(t)
	temporaryDirectory := t.TempDir()
	t.Chdir(temporaryDirectory)

	validKey := strings.Repeat("e", 64)
	t.Setenv("MASTER_ENCRYPTION_KEY", validKey)
	t.Setenv("DATABASE_URL", "postgres://invalid:5432/db")

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

	// 3. Monolith all argument: layr up all -> all 7 services enabled
	_ = Execute(cancelledContext(), []string{"up", "all"})
	activeConfigServices = core.GetConfig().GetEnabledServices()
	if len(activeConfigServices) != 7 {
		t.Fatalf("expected 7 active services for 'up all', got: %v", activeConfigServices)
	}
}

func TestCLIRunExecutionErrorsUnit(t *testing.T) {
	resetTestState(t)
	temporaryDirectory := t.TempDir()
	t.Chdir(temporaryDirectory)

	// 1. Invalid master encryption key -> core.NewKernel failure
	t.Setenv("MASTER_ENCRYPTION_KEY", "too_short")
	t.Setenv("DATABASE_URL", "postgres://invalid:5432/db")
	kernelErr := Run([]string{"up"})
	if kernelErr == nil || !strings.Contains(kernelErr.Error(), "failed to initialize kernel") {
		t.Fatalf("expected kernel initialization error, got: %v", kernelErr)
	}

	// 2. Auto-loaded .env with invalid environment variable key (contains null byte)
	badDotEnvPath := filepath.Join(temporaryDirectory, ".env")
	if writeErr := os.WriteFile(badDotEnvPath, []byte("\x00BAD=value\n"), 0o600); writeErr != nil {
		t.Fatalf("failed to write bad .env: %v", writeErr)
	}
	badDotEnvErr := Run([]string{"up"})
	if badDotEnvErr == nil || !strings.Contains(badDotEnvErr.Error(), "failed to load env file") {
		t.Fatalf("expected error from bad auto-loaded .env, got: %v", badDotEnvErr)
	}
	_ = os.Remove(badDotEnvPath)

	// 3. LoadConfig failure
	loadConfigFunc = func() (*core.Config, error) {
		return nil, errors.New("simulated load config failure")
	}
	loadConfigErr := Run([]string{"up"})
	if loadConfigErr == nil || !strings.Contains(loadConfigErr.Error(), "failed to load configuration") {
		t.Fatalf("expected load config error, got: %v", loadConfigErr)
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
	t.Setenv("LAYR__DATABASE__URL", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MASTER_ENCRYPTION_KEY", "")
	t.Setenv("LAYR__SECURITY__MASTER_ENCRYPTION_KEY", "")

	// 1. Database URL write failure in read-only directory
	err := Run([]string{"up"})
	if err == nil || !strings.Contains(err.Error(), "failed to bootstrap environment") {
		t.Fatalf("expected bootstrap write error for database url, got: %v", err)
	}

	// 2. Encryption key write failure in read-only directory when database url is set
	t.Setenv("LAYR__DATABASE__URL", ".layr/data")
	keyErr := Run([]string{"up"})
	if keyErr == nil || !strings.Contains(keyErr.Error(), "failed to bootstrap environment") {
		t.Fatalf("expected bootstrap write error for encryption key, got: %v", keyErr)
	}
}

func TestCLIRunCleanShutdownUnit(t *testing.T) {
	resetTestState(t)
	temporaryDirectory := t.TempDir()
	t.Chdir(temporaryDirectory)

	validKey := strings.Repeat("a", 64)
	t.Setenv("MASTER_ENCRYPTION_KEY", validKey)
	t.Setenv("PORT", "18080")
	t.Setenv("DATABASE_URL", filepath.Join(temporaryDirectory, "data"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errChannel := make(chan error, 1)

	go func() {
		errChannel <- Execute(ctx, []string{"up"})
	}()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(90 * time.Second)
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
