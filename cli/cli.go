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

// ExitCodeConfig represents the exit code for configuration errors (EX_CONFIG = 78).
const ExitCodeConfig = core.ConfigFailureExitCode

const usageHelp = `Usage: layr <command> [flags] [services...]

Commands:
  up [services...]     Starts the Layr runtime engine with specified services (monolith if none)

Services:
  data, auth, tasks, file_storage, image, function, console, all

Flags:
  -c, --config <path>    Custom path to layr.yaml (default: ./layr.yaml)
      --env-file <path>  Load environment variables from specific file
  -h, --help             Show help
`

var (
	exitFunc     = os.Exit
	stderrWriter = io.Writer(os.Stderr)
	stdoutWriter = io.Writer(os.Stdout)
)

type cliOptions struct {
	configPath  string
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
	}

	configPath, pathErr := resolveConfigPath(options.configPath)
	if pathErr != nil {
		return pathErr
	}

	config, loadErr := core.LoadConfig(configPath)
	if loadErr != nil {
		return fmt.Errorf("failed to load configuration: %w", loadErr)
	}

	if !isMonolithMode(options.services) {
		for _, serviceName := range options.services {
			if !config.IsServiceEnabled(serviceName) {
				_, _ = fmt.Fprintf(stderrWriter, "error: service %q is disabled in configuration\n", serviceName)
				exitFunc(ExitCodeConfig)
				return fmt.Errorf("service %q is disabled in configuration: exit %d", serviceName, ExitCodeConfig)
			}
		}

		config.Data.Enabled = false
		config.Auth.Enabled = false
		config.FileStorage.Enabled = false
		config.Tasks.Enabled = false
		config.Image.Enabled = false
		config.Function.Enabled = false
		config.Console.Enabled = false

		for _, serviceName := range options.services {
			_ = config.EnableService(serviceName)
		}
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
	fileData, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read env file %s: %w", path, err)
	}

	for _, line := range strings.Split(string(fileData), "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		key, value, ok := strings.Cut(trimmedLine, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if len(value) >= 2 {
			first := value[0]
			last := value[len(value)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		if setErr := os.Setenv(key, value); setErr != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", key, setErr)
		}
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
		case arg == "-c" || arg == "--config":
			if index+1 >= len(args) {
				return options, fmt.Errorf("flag %s requires an argument", arg)
			}
			index++
			options.configPath = args[index]
		case strings.HasPrefix(arg, "-c="):
			options.configPath = strings.TrimPrefix(arg, "-c=")
		case strings.HasPrefix(arg, "--config="):
			options.configPath = strings.TrimPrefix(arg, "--config=")
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

func resolveConfigPath(customPath string) (string, error) {
	if customPath != "" {
		return customPath, nil
	}

	if _, err := os.Stat("layr.yaml"); err == nil {
		return "layr.yaml", nil
	}

	if os.Getenv("DATABASE_URL") == "" {
		if err := core.WriteConfigFile("layr.yaml"); err != nil {
			return "", fmt.Errorf("failed to write bootstrap configuration: %w", err)
		}
		return "layr.yaml", nil
	}

	return "", nil
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
