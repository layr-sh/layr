// Package cli provides the command-line interface for the Layr runtime engine.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"layr.sh/core"

	// Register modular service factories
	_ "layr.sh/auth"        // registers auth service factory
	_ "layr.sh/console"     // registers console service factory
	_ "layr.sh/data"        // registers data service factory
	_ "layr.sh/filestorage" // registers file_storage service factory
	_ "layr.sh/function"    // registers function service factory
	_ "layr.sh/image"       // registers image service factory
	_ "layr.sh/tasks"       // registers tasks service factory
)

const (
	// ExitCodeConfig represents the exit code for configuration errors (EX_CONFIG = 78).
	ExitCodeConfig = core.ConfigFailureExitCode

	defaultEnvFilePermission = 0o600
)

const usageHelp = `Usage: layr <command> [flags] [services...]

Commands:
  up [services...]     Starts the Layr runtime engine with specified services (monolith if none)

Services:
  data, auth, tasks, file_storage, image, function, console, all

Flags:
      --env-file <path>  Load environment variables from specific file
  -h, --help             Show help
`

var (
	exitFunc        = os.Exit
	stderrWriter    = io.Writer(os.Stderr)
	stdoutWriter    = io.Writer(os.Stdout)
	generateKeyFunc = core.GenerateRandomCryptoEncryptionKeyHex
	setenvFunc      = os.Setenv
	loadConfigFunc  = func() (*core.Config, error) { return core.LoadConfig() }
)

type cliOptions struct {
	envFilePath string
	command     string
	services    []string
	showHelp    bool
}

// Run parses command-line arguments and executes the Layr CLI.
func Run(args []string) error {
	return Execute(context.Background(), args)
}

// Execute executes the Layr CLI with a provided context and argument slice.
func Execute(ctx context.Context, args []string) error {
	options, err := parseArgs(args)
	if err != nil {
		return err
	}

	if options.showHelp {
		_, _ = fmt.Fprint(stdoutWriter, usageHelp)
		return nil
	}

	if options.envFilePath != "" {
		if envErr := LoadEnvFile(options.envFilePath); envErr != nil {
			return envErr
		}
	} else if _, err := os.Stat(".env"); err == nil {
		if envErr := LoadEnvFile(".env"); envErr != nil {
			return envErr
		}
	}

	if err := ensureBootstrapEnvironment(options.envFilePath); err != nil {
		return fmt.Errorf("failed to bootstrap environment: %w", err)
	}

	config, loadErr := loadConfigFunc()
	if loadErr != nil {
		return fmt.Errorf("failed to load configuration: %w", loadErr)
	}

	if !isMonolithMode(options.services) {
		var activeServices []string
		for _, serviceName := range options.services {
			serviceName = strings.ToLower(strings.TrimSpace(serviceName))
			if !core.IsValidService(serviceName) {
				_, _ = fmt.Fprintf(stderrWriter, "error: unknown service %q\n", serviceName)
				exitFunc(ExitCodeConfig)
				return fmt.Errorf("unknown service %q: exit %d", serviceName, ExitCodeConfig)
			}
			activeServices = append(activeServices, serviceName)
		}

		config.SetEnabledServices(activeServices)
		core.SetLoadedConfig(config)
	}

	kernel, kernelErr := core.NewKernel(nil)
	if kernelErr != nil {
		return fmt.Errorf("failed to initialize kernel: %w", kernelErr)
	}

	if startErr := kernel.Start(ctx); startErr != nil {
		return fmt.Errorf("kernel execution failed: %w", startErr)
	}

	return nil
}

// LoadEnvFile loads environment variables from a file into the process environment.
func LoadEnvFile(path string) error {
	if err := core.LoadEnvFile(path); err != nil {
		return fmt.Errorf("failed to load env file: %w", err)
	}
	return nil
}

func ensureBootstrapEnvironment(targetEnvFile string) error {
	if targetEnvFile == "" {
		targetEnvFile = ".env"
	}

	databaseURL := os.Getenv("LAYR__DATABASE__URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if databaseURL == "" {
		const defaultDatabaseURL = ".layr/data"
		if err := appendOrWriteEnv(targetEnvFile, "LAYR__DATABASE__URL", defaultDatabaseURL); err != nil {
			return fmt.Errorf("failed to persist database url: %w", err)
		}
		if err := setenvFunc("LAYR__DATABASE__URL", defaultDatabaseURL); err != nil {
			return fmt.Errorf("failed to set database url environment variable: %w", err)
		}
	}

	masterEncryptionKey := os.Getenv("LAYR__SECURITY__MASTER_ENCRYPTION_KEY")
	if masterEncryptionKey == "" {
		masterEncryptionKey = os.Getenv("MASTER_ENCRYPTION_KEY")
	}
	if masterEncryptionKey == "" {
		key, err := generateKeyFunc()
		if err != nil {
			return fmt.Errorf("failed to generate encryption key: %w", err)
		}
		if err := appendOrWriteEnv(targetEnvFile, "LAYR__SECURITY__MASTER_ENCRYPTION_KEY", key); err != nil {
			return fmt.Errorf("failed to persist encryption key: %w", err)
		}
		if err := setenvFunc("LAYR__SECURITY__MASTER_ENCRYPTION_KEY", key); err != nil {
			return fmt.Errorf("failed to set encryption key environment variable: %w", err)
		}
	}

	return nil
}

func appendOrWriteEnv(path, key, value string) error {
	line := fmt.Sprintf("%s=%s\n", key, value)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if writeErr := os.WriteFile(path, []byte(line), defaultEnvFilePermission); writeErr != nil {
				return fmt.Errorf("failed to write env file: %w", writeErr)
			}
			return nil
		}
		return fmt.Errorf("failed to read env file: %w", err)
	}
	content := string(data)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += line
	if writeErr := os.WriteFile(path, []byte(content), defaultEnvFilePermission); writeErr != nil {
		return fmt.Errorf("failed to write env file: %w", writeErr)
	}
	return nil
}

func parseArgs(args []string) (cliOptions, error) {
	var options cliOptions
	var positional []string

	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "-h" || arg == "--help" || arg == "help":
			options.showHelp = true
		case arg == "--env-file":
			if index+1 >= len(args) {
				return options, fmt.Errorf("flag %s requires an argument", arg)
			}
			index++
			options.envFilePath = args[index]
		case strings.HasPrefix(arg, "--env-file="):
			options.envFilePath = strings.TrimPrefix(arg, "--env-file=")
		case strings.HasPrefix(arg, "-"):
			return options, fmt.Errorf("unknown flag: %s", arg)
		default:
			positional = append(positional, arg)
		}
	}

	if options.showHelp {
		return options, nil
	}

	if len(positional) == 0 {
		return options, errors.New("missing command: expected 'up'")
	}

	options.command = positional[0]
	if options.command != "up" {
		return options, fmt.Errorf("unknown command %q: expected 'up'", options.command)
	}

	options.services = positional[1:]
	return options, nil
}

func isMonolithMode(services []string) bool {
	if len(services) == 0 {
		return true
	}
	for _, service := range services {
		if strings.EqualFold(service, "all") {
			return true
		}
	}
	return false
}
