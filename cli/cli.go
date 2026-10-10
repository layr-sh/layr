// Package cli provides the command-line interface for the Layr runtime engine.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

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

	minArgsConsoleRootUserAction       = 3
	minArgsConsoleRootUserEmail        = 4
	minArgsConsoleRootUserWithPassword = 5
	tabWriterPadding                   = 2
)

const usageHelp = `Usage: layr <command> [flags] [services...]

Commands:
  up [services...]                                   Starts the Layr runtime engine with specified services (monolith if none)
  console root-user <subcommand> [flags] [args...]   Manage console root user accounts

Console Root User Subcommands:
  create <email> [password]                          Create a new console root user
  list                                               List all console root users
  reset-password <email> [new-password]              Reset password for a console root user
  delete <email>                                     Delete a console root user

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
	newKernelFunc   = func(db *core.DatabasePool) (*core.Kernel, error) { return core.NewKernel(db) }
)

type cliOptions struct {
	showHelp                bool
	envFilePath             string
	command                 string
	upServices              []string
	consoleRootUserAction   string
	consoleRootUserEmail    string
	consoleRootUserPassword string
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

	if options.command == "console" {
		return executeConsoleRootUser(ctx, options)
	}

	if !isMonolithMode(options.upServices) {
		var activeServices []string
		for _, serviceName := range options.upServices {
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
		return options, errors.New("missing command: expected 'up' or 'console'")
	}

	options.command = positional[0]
	switch options.command {
	case "up":
		options.upServices = positional[1:]
		return options, nil
	case "console":
		if len(positional) < 2 {
			return options, errors.New("missing console subcommand: expected 'root-user'")
		}
		if positional[1] != "root-user" {
			return options, fmt.Errorf("unknown console subcommand %q: expected 'root-user'", positional[1])
		}
		if len(positional) < minArgsConsoleRootUserAction {
			return options, errors.New("missing root-user action: expected 'create', 'list', 'reset-password', or 'delete'")
		}
		action := positional[2]
		options.consoleRootUserAction = action
		switch action {
		case "create":
			if len(positional) < minArgsConsoleRootUserEmail || strings.TrimSpace(positional[3]) == "" {
				return options, errors.New("missing email argument for create")
			}
			options.consoleRootUserEmail = positional[3]
			if len(positional) >= minArgsConsoleRootUserWithPassword {
				options.consoleRootUserPassword = positional[4]
			}
		case "list":
			// no additional arguments required
		case "reset-password":
			if len(positional) < minArgsConsoleRootUserEmail || strings.TrimSpace(positional[3]) == "" {
				return options, errors.New("missing email argument for reset-password")
			}
			options.consoleRootUserEmail = positional[3]
			if len(positional) >= minArgsConsoleRootUserWithPassword {
				options.consoleRootUserPassword = positional[4]
			}
		case "delete":
			if len(positional) < minArgsConsoleRootUserEmail || strings.TrimSpace(positional[3]) == "" {
				return options, errors.New("missing email argument for delete")
			}
			options.consoleRootUserEmail = positional[3]
		default:
			return options, fmt.Errorf("unknown root-user action %q: expected 'create', 'list', 'reset-password', or 'delete'", action)
		}
		return options, nil
	default:
		return options, fmt.Errorf("unknown command %q: expected 'up' or 'console'", options.command)
	}
}

func executeConsoleRootUser(ctx context.Context, options cliOptions) error {
	kernel, kernelErr := newKernelFunc(nil)
	if kernelErr != nil {
		return fmt.Errorf("failed to initialize kernel: %w", kernelErr)
	}

	if initErr := kernel.InitDatabase(ctx); initErr != nil {
		return fmt.Errorf("failed to initialize database: %w", initErr)
	}

	switch options.consoleRootUserAction {
	case "create":
		consoleRootUser, serviceAccount, plainPassword, createErr := kernel.CreateConsoleRootUser(ctx, options.consoleRootUserEmail, options.consoleRootUserPassword)
		if createErr != nil {
			return fmt.Errorf("failed to create console root user: %w", createErr)
		}
		_, _ = fmt.Fprintln(stdoutWriter, "Console root user created:")
		_, _ = fmt.Fprintf(stdoutWriter, "  Email:               %s\n", consoleRootUser.Email)
		_, _ = fmt.Fprintf(stdoutWriter, "  Password:            %s\n", plainPassword)
		_, _ = fmt.Fprintf(stdoutWriter, "  Service Account Key: %s\n", serviceAccount.SecretKey)
		return nil

	case "list":
		users, _ := kernel.ListConsoleRootUsers(ctx)
		if len(users) == 0 {
			_, _ = fmt.Fprintln(stdoutWriter, "No console root users found.")
			return nil
		}
		writer := tabwriter.NewWriter(stdoutWriter, 0, 0, tabWriterPadding, ' ', 0)
		_, _ = fmt.Fprintln(writer, "EMAIL\tENABLED\tCREATED AT")
		for _, consoleRootUser := range users {
			_, _ = fmt.Fprintf(writer, "%s\t%t\t%s\n", consoleRootUser.Email, consoleRootUser.IsEnabled, consoleRootUser.CreatedAt.UTC().Format("2006-01-02 15:04:05"))
		}
		_ = writer.Flush()
		return nil

	case "reset-password":
		plainPassword, resetErr := kernel.ResetConsoleRootUserPassword(ctx, options.consoleRootUserEmail, options.consoleRootUserPassword)
		if resetErr != nil {
			return fmt.Errorf("failed to reset console root user password: %w", resetErr)
		}
		_, _ = fmt.Fprintln(stdoutWriter, "Console root user password reset successfully:")
		_, _ = fmt.Fprintf(stdoutWriter, "  Email:    %s\n", options.consoleRootUserEmail)
		_, _ = fmt.Fprintf(stdoutWriter, "  Password: %s\n", plainPassword)
		return nil

	case "delete":
		if deleteErr := kernel.DeleteConsoleRootUser(ctx, options.consoleRootUserEmail); deleteErr != nil {
			return fmt.Errorf("failed to delete console root user: %w", deleteErr)
		}
		_, _ = fmt.Fprintf(stdoutWriter, "Console root user %q deleted successfully.\n", options.consoleRootUserEmail)
		return nil

	default:
		return fmt.Errorf("unsupported root-user action %q", options.consoleRootUserAction)
	}
}

func isMonolithMode(upServices []string) bool {
	if len(upServices) == 0 {
		return true
	}
	for _, service := range upServices {
		if strings.EqualFold(service, "all") {
			return true
		}
	}
	return false
}
