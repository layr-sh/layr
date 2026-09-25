package function

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"layr.sh/core"
)

const (
	dirPermissions   = 0755
	filePermissions  = 0644
	maxPortAttempts  = 50
	portWaitInterval = 20 * time.Millisecond

	// WorkerdStorageDir defines the static directory for workerd storage and function files.
	WorkerdStorageDir = ".layr/function/workerd"
	// WorkerdBinaryPath defines the static path where the workerd executable resides.
	WorkerdBinaryPath = ".layr/bin/workerd"
	// WorkerdDownloadURL defines the static URL from which workerd releases are downloaded.
	WorkerdDownloadURL = "https://github.com/cloudflare/workerd/releases/latest/download"
)

var (
	netListen                            = net.Listen
	execCommandContext                   = exec.CommandContext
	downloadWorkerdBinaryForPlatformFunc = downloadWorkerdBinaryForPlatform
)

// WorkerdRunner manages execution of functions inside Cloudflare's workerd V8 runtime.
type WorkerdRunner struct {
	rwMutex           sync.RWMutex
	configManager     *ConfigManager
	isStarted         bool
	processCmd        *exec.Cmd
	activePorts       map[string]int
	activeDeployments map[string]*Deployment
	activeProxies     map[string]*httputil.ReverseProxy
}

// NewWorkerdRunner initializes a new WorkerdRunner instance with the given dynamic config manager.
func NewWorkerdRunner(configManager *ConfigManager) *WorkerdRunner {
	if configManager == nil {
		configManager = NewConfigManager(nil)
	}

	return &WorkerdRunner{
		configManager:     configManager,
		activePorts:       make(map[string]int),
		activeDeployments: make(map[string]*Deployment),
		activeProxies:     make(map[string]*httputil.ReverseProxy),
	}
}

// Name returns the runtime identifier "workerd".
func (runner *WorkerdRunner) Name() string {
	return "workerd"
}

// ResolveBinary locates or downloads the workerd binary executable using the static system paths.
func (runner *WorkerdRunner) ResolveBinary(ctx context.Context) (string, error) {
	return ResolveWorkerdBinary(ctx)
}

// ResolveWorkerdBinary locates or downloads the workerd binary executable using static system configuration.
func ResolveWorkerdBinary(ctx context.Context) (string, error) {
	return resolveWorkerdBinaryForPlatform(ctx, runtime.GOOS, runtime.GOARCH)
}

func resolveWorkerdBinaryForPlatform(ctx context.Context, targetOS, targetArch string) (string, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", fmt.Errorf("context canceled: %w", ctxErr)
	}
	if _, assetErr := resolveWorkerdReleaseAsset(targetOS, targetArch); assetErr != nil {
		return "", fmt.Errorf("%w: %w", ErrRunnerDownloadFailed, assetErr)
	}

	binaryFilename := workerdBinaryFilename(targetOS)
	targetPath := filepath.Join(".layr", "bin", binaryFilename)
	if targetOS == runtime.GOOS && targetArch == runtime.GOARCH {
		// 1. Check static target destination (.layr/bin/<binaryFilename>)
		if _, statErr := os.Stat(targetPath); statErr == nil {
			return targetPath, nil
		}

		// 2. Check system PATH
		if pathLookResult, lookErr := exec.LookPath(binaryFilename); lookErr == nil {
			return pathLookResult, nil
		}
	}

	// 3. Determine target destination and download from static URL
	downloadedPath, downloadErr := downloadWorkerdBinaryForPlatformFunc(ctx, WorkerdDownloadURL, targetOS, targetArch, targetPath)
	if downloadErr != nil {
		return "", fmt.Errorf("%w: %w", ErrRunnerDownloadFailed, downloadErr)
	}

	return downloadedPath, nil
}

func workerdBinaryFilename(targetOS string) string {
	if targetOS == "windows" {
		return "workerd.exe"
	}
	return "workerd"
}

func resolveWorkerdReleaseAsset(targetOS, targetArch string) (string, error) {
	switch targetOS {
	case "darwin":
		if targetArch == "arm64" {
			return "workerd-darwin-arm64.gz", nil
		}
		if targetArch == "amd64" {
			return "workerd-darwin-64.gz", nil
		}
		return "", fmt.Errorf("unsupported workerd architecture for darwin: %s", targetArch)
	case "linux":
		if targetArch == "arm64" {
			return "workerd-linux-arm64.gz", nil
		}
		if targetArch == "amd64" {
			return "workerd-linux-64.gz", nil
		}
		return "", fmt.Errorf("unsupported workerd architecture for linux: %s", targetArch)
	case "windows":
		if targetArch == "amd64" {
			return "workerd-windows-64.gz", nil
		}
		return "", fmt.Errorf("unsupported workerd architecture for windows: %s", targetArch)
	default:
		return "", fmt.Errorf("unsupported workerd platform: %s/%s", targetOS, targetArch)
	}
}

func downloadWorkerdBinary(ctx context.Context, targetPath string) (string, error) {
	return downloadWorkerdBinaryForPlatform(ctx, WorkerdDownloadURL, runtime.GOOS, runtime.GOARCH, targetPath)
}

func downloadWorkerdBinaryForPlatform(ctx context.Context, downloadBaseURL, targetOS, targetArch, targetPath string) (string, error) {
	assetName, assetErr := resolveWorkerdReleaseAsset(targetOS, targetArch)
	if assetErr != nil {
		return "", assetErr
	}

	downloadURL := fmt.Sprintf("%s/%s", strings.TrimRight(downloadBaseURL, "/"), assetName)

	log.Infof("downloading workerd binary from %s to %s", downloadURL, targetPath)
	if mkdirErr := os.MkdirAll(filepath.Dir(targetPath), dirPermissions); mkdirErr != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", mkdirErr)
	}

	httpRequest, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if requestErr != nil {
		return "", fmt.Errorf("failed to create download request: %w", requestErr)
	}

	httpClient := &http.Client{Timeout: 5 * time.Minute}
	httpResponse, httpErr := httpClient.Do(httpRequest)
	if httpErr != nil {
		return "", fmt.Errorf("download request failed: %w", httpErr)
	}
	defer func() {
		_ = httpResponse.Body.Close()
	}()

	if httpResponse.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code downloading workerd: %d", httpResponse.StatusCode)
	}

	gzipReader, gzipErr := gzip.NewReader(httpResponse.Body)
	if gzipErr != nil {
		return "", fmt.Errorf("failed to initialize gzip reader: %w", gzipErr)
	}
	defer func() {
		_ = gzipReader.Close()
	}()

	targetFile, createErr := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, dirPermissions)
	if createErr != nil {
		return "", fmt.Errorf("failed to create destination binary file: %w", createErr)
	}

	if _, copyErr := io.Copy(targetFile, gzipReader); copyErr != nil {
		_ = targetFile.Close()
		_ = os.Remove(targetPath)
		return "", fmt.Errorf("failed to decompress binary: %w", copyErr)
	}
	_ = targetFile.Close()

	log.Infof("successfully downloaded and verified workerd binary at %s", targetPath)
	return targetPath, nil
}

// Start boots the supervisor and verifies the runtime environment.
func (runner *WorkerdRunner) Start(ctx context.Context) error {
	runner.rwMutex.Lock()
	defer runner.rwMutex.Unlock()

	if runner.isStarted {
		return nil
	}

	if mkdirErr := os.MkdirAll(WorkerdStorageDir, dirPermissions); mkdirErr != nil {
		return fmt.Errorf("failed to create workerd directory: %w", mkdirErr)
	}

	runner.isStarted = true
	return nil
}

// Stop gracefully terminates the running workerd child process.
func (runner *WorkerdRunner) Stop() error {
	runner.rwMutex.Lock()
	defer runner.rwMutex.Unlock()

	runner.isStarted = false
	if runner.processCmd != nil && runner.processCmd.Process != nil {
		_ = runner.processCmd.Process.Kill()
		runner.processCmd = nil
	}

	return nil
}

// Deploy writes the code bundle and triggers zero-downtime hot reload.
func (runner *WorkerdRunner) Deploy(
	ctx context.Context,
	targetEndpoint *Endpoint,
	deployment *Deployment,
) error {
	if targetEndpoint == nil {
		return ErrEndpointNotFound
	}
	if deployment == nil {
		return ErrDeploymentNotFound
	}

	runner.rwMutex.Lock()
	defer runner.rwMutex.Unlock()

	endpointDir := filepath.Join(WorkerdStorageDir, "functions", targetEndpoint.Name)
	if mkdirErr := os.MkdirAll(endpointDir, dirPermissions); mkdirErr != nil {
		return fmt.Errorf("failed to create endpoint directory: %w", mkdirErr)
	}

	if extractErr := extractBundle(endpointDir, deployment, targetEndpoint.Entrypoint); extractErr != nil {
		return fmt.Errorf("failed to extract bundle: %w", extractErr)
	}

	runnerCode := BuildWorkerRunnerScript()
	runnerPath := filepath.Join(endpointDir, "runner.js")
	if writeErr := os.WriteFile(runnerPath, []byte(runnerCode), filePermissions); writeErr != nil {
		return fmt.Errorf("failed to write runner script: %w", writeErr)
	}

	port, exists := runner.activePorts[targetEndpoint.Name]
	if !exists {
		allocatedPort, portErr := runner.allocateFreeTCPPort(ctx)
		if portErr != nil {
			return fmt.Errorf("failed to allocate port: %w", portErr)
		}
		port = allocatedPort
		runner.activePorts[targetEndpoint.Name] = port
	}

	runner.activeDeployments[targetEndpoint.Name] = deployment

	targetURL, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(port))
	reverseProxy := httputil.NewSingleHostReverseProxy(targetURL)
	reverseProxy.ErrorHandler = func(responseWriter http.ResponseWriter, proxyRequest *http.Request, proxyErr error) {
		core.WriteErrorResponse(responseWriter, proxyRequest, http.StatusBadGateway, proxyErr.Error())
	}
	runner.activeProxies[targetEndpoint.Name] = reverseProxy

	return runner.regenerateAndReloadLocked(ctx)
}

func extractBundle(destDir string, deployment *Deployment, entrypoint string) error {
	switch deployment.BundleFormat {
	case "tar":
		tarReader := tar.NewReader(bytes.NewReader(deployment.BundleContent))
		for {
			header, err := tarReader.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fmt.Errorf("failed to read tar entry: %w", err)
			}
			cleanName := filepath.Clean(header.Name)
			if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
				continue
			}
			targetPath := filepath.Join(destDir, cleanName)
			if header.FileInfo().IsDir() {
				if err := os.MkdirAll(targetPath, dirPermissions); err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), dirPermissions); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			fileBytes, readErr := io.ReadAll(tarReader)
			if readErr != nil {
				return fmt.Errorf("failed to read tar file entry: %w", readErr)
			}
			if err := os.WriteFile(targetPath, fileBytes, filePermissions); err != nil {
				return fmt.Errorf("failed to write extracted file: %w", err)
			}
		}
	case "zip":
		zipReader, err := zip.NewReader(bytes.NewReader(deployment.BundleContent), int64(len(deployment.BundleContent)))
		if err != nil {
			return fmt.Errorf("failed to read zip archive: %w", err)
		}
		for _, file := range zipReader.File {
			cleanName := filepath.Clean(file.Name)
			if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
				continue
			}
			targetPath := filepath.Join(destDir, cleanName)
			if file.FileInfo().IsDir() {
				if err := os.MkdirAll(targetPath, dirPermissions); err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), dirPermissions); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			readCloser, openErr := file.Open()
			if openErr != nil {
				return fmt.Errorf("failed to open zip file entry: %w", openErr)
			}
			fileBytes, readErr := io.ReadAll(readCloser)
			_ = readCloser.Close()
			if readErr != nil {
				return fmt.Errorf("failed to read zip file entry: %w", readErr)
			}
			if err := os.WriteFile(targetPath, fileBytes, filePermissions); err != nil {
				return fmt.Errorf("failed to write extracted file: %w", err)
			}
		}
	default: // "raw"
		userCodePath := filepath.Join(destDir, "user_code.js")
		if writeErr := os.WriteFile(userCodePath, deployment.BundleContent, filePermissions); writeErr != nil {
			return fmt.Errorf("failed to write user code: %w", writeErr)
		}
	}

	// Ensure user_code.js exists for workerd runner
	userCodePath := filepath.Join(destDir, "user_code.js")
	if _, statErr := os.Stat(userCodePath); os.IsNotExist(statErr) {
		entrypointName := entrypoint
		if entrypointName == "" {
			entrypointName = "index.js"
		}
		entrypointRelativePath := "./" + filepath.ToSlash(entrypointName)
		wrapperCode := fmt.Sprintf("export * from %q;\nexport { default } from %q;\n", entrypointRelativePath, entrypointRelativePath)
		if writeErr := os.WriteFile(userCodePath, []byte(wrapperCode), filePermissions); writeErr != nil {
			return fmt.Errorf("failed to write user_code.js wrapper: %w", writeErr)
		}
	}

	return nil
}

// Undeploy removes an endpoint from the active runtime.
func (runner *WorkerdRunner) Undeploy(ctx context.Context, endpointName string) error {
	runner.rwMutex.Lock()
	defer runner.rwMutex.Unlock()

	delete(runner.activePorts, endpointName)
	delete(runner.activeDeployments, endpointName)
	delete(runner.activeProxies, endpointName)

	endpointDir := filepath.Join(WorkerdStorageDir, "functions", endpointName)
	_ = os.RemoveAll(endpointDir)

	return runner.regenerateAndReloadLocked(ctx)
}

func (runner *WorkerdRunner) regenerateAndReloadLocked(ctx context.Context) error {
	capnpContent := runner.BuildCapnpConfig()
	configPath := filepath.Join(WorkerdStorageDir, "config.capnp")
	if writeErr := os.WriteFile(configPath, []byte(capnpContent), filePermissions); writeErr != nil {
		return fmt.Errorf("failed to write config.capnp: %w", writeErr)
	}

	if len(runner.activePorts) == 0 {
		if runner.processCmd != nil && runner.processCmd.Process != nil {
			_ = runner.processCmd.Process.Kill()
			_ = runner.processCmd.Wait()
			runner.processCmd = nil
		}
		return nil
	}

	resolvedPath, resolveErr := runner.ResolveBinary(ctx)
	if resolveErr != nil {
		return resolveErr
	}

	if runner.processCmd != nil && runner.processCmd.Process != nil {
		_ = runner.processCmd.Process.Kill()
		_ = runner.processCmd.Wait()
		runner.processCmd = nil
	}

	absBinaryPath, _ := filepath.Abs(resolvedPath)
	absStorageDir, _ := filepath.Abs(WorkerdStorageDir)
	absConfigPath, _ := filepath.Abs(configPath)

	workerdCmd := execCommandContext(context.WithoutCancel(ctx), absBinaryPath, "serve", absConfigPath)
	workerdCmd.Dir = absStorageDir
	workerdCmd.Stderr = os.Stderr
	if startErr := workerdCmd.Start(); startErr != nil {
		return fmt.Errorf("failed to spawn workerd process: %w", startErr)
	}

	runner.processCmd = workerdCmd

	expectedBinary := workerdBinaryFilename(runtime.GOOS)
	if filepath.Base(resolvedPath) == expectedBinary {
		for _, port := range runner.activePorts {
			runner.waitForPortLocked(ctx, port)
		}
	}

	return nil
}

func (runner *WorkerdRunner) waitForPortLocked(ctx context.Context, port int) {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	dialer := &net.Dialer{Timeout: portWaitInterval}
	for attempt := 0; attempt < maxPortAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		tcpConnection, dialErr := dialer.DialContext(ctx, "tcp", address)
		if dialErr == nil {
			_ = tcpConnection.Close()
			return
		}
		time.Sleep(portWaitInterval)
	}
}

// Forward delegates request handling to the live endpoint reverse proxy.
// If the workerd binary is not yet downloaded, it lazily resolves or downloads the binary on invocation.
func (runner *WorkerdRunner) Forward(
	responseWriter http.ResponseWriter,
	request *http.Request,
	targetEndpoint *Endpoint,
) error {
	if targetEndpoint == nil {
		return ErrEndpointNotFound
	}

	if _, resolveErr := runner.ResolveBinary(request.Context()); resolveErr != nil {
		return fmt.Errorf("failed to download workerd binary on invocation: %w", resolveErr)
	}

	runner.rwMutex.RLock()
	reverseProxy, exists := runner.activeProxies[targetEndpoint.Name]
	runner.rwMutex.RUnlock()

	if !exists || reverseProxy == nil {
		return fmt.Errorf("%w: endpoint %s is not deployed", ErrDeploymentNotFound, targetEndpoint.Name)
	}

	reverseProxy.ServeHTTP(responseWriter, request)
	return nil
}

// Health returns runtime health metrics and readiness.
func (runner *WorkerdRunner) Health(ctx context.Context) (*RunnerHealth, error) {
	runner.rwMutex.RLock()
	defer runner.rwMutex.RUnlock()

	activeCount := len(runner.activePorts)
	runnerHealth := &RunnerHealth{
		Available:      runner.isStarted,
		ActiveIsolates: activeCount,
	}

	if runner.processCmd != nil && runner.processCmd.Process != nil {
		runnerHealth.ProcessPID = runner.processCmd.Process.Pid
	}

	return runnerHealth, nil
}

// BuildCapnpConfig compiles active worker definitions into Cap'n Proto format.
func (runner *WorkerdRunner) BuildCapnpConfig() string {
	var configBuilder strings.Builder
	configBuilder.WriteString("using Workerd = import \"/workerd/workerd.capnp\";\n\n")
	configBuilder.WriteString("const config :Workerd.Config = (\n")
	configBuilder.WriteString("  services = [\n")

	sortedEndpointNames := make([]string, 0, len(runner.activePorts))
	for endpointName := range runner.activePorts {
		sortedEndpointNames = append(sortedEndpointNames, endpointName)
	}
	sort.Strings(sortedEndpointNames)

	for _, endpointName := range sortedEndpointNames {
		sanitizedName := sanitizeCapnpIdentifier(endpointName)
		fmt.Fprintf(&configBuilder, "    (name = %q, worker = .%sWorker),\n", endpointName, sanitizedName)
	}
	configBuilder.WriteString("  ],\n")
	configBuilder.WriteString("  sockets = [\n")

	for _, endpointName := range sortedEndpointNames {
		allocatedPort := runner.activePorts[endpointName]
		sanitizedName := sanitizeCapnpIdentifier(endpointName)
		fmt.Fprintf(&configBuilder, "    (name = %q, address = %q, http = (), service = %q),\n",
			sanitizedName, fmt.Sprintf("127.0.0.1:%d", allocatedPort), endpointName)
	}
	configBuilder.WriteString("  ]\n")
	configBuilder.WriteString(");\n\n")

	for _, endpointName := range sortedEndpointNames {
		sanitizedName := sanitizeCapnpIdentifier(endpointName)
		activeDeployment := runner.activeDeployments[endpointName]
		runnerRelativePath := filepath.ToSlash(filepath.Join("functions", endpointName, "runner.js"))
		userRelativePath := filepath.ToSlash(filepath.Join("functions", endpointName, "user_code.js"))

		isServiceWorkerScript := false
		if activeDeployment != nil && bytes.Contains(activeDeployment.BundleContent, []byte("addEventListener")) {
			isServiceWorkerScript = true
		}

		fmt.Fprintf(&configBuilder, "const %sWorker :Workerd.Worker = (\n", sanitizedName)
		if isServiceWorkerScript {
			fmt.Fprintf(&configBuilder, "  serviceWorkerScript = embed %q,\n", userRelativePath)
		} else {
			configBuilder.WriteString("  modules = [\n")
			fmt.Fprintf(&configBuilder, "    (name = \"runner.js\", esModule = embed %q),\n", runnerRelativePath)
			fmt.Fprintf(&configBuilder, "    (name = \"./user_code.js\", esModule = embed %q),\n", userRelativePath)

			endpointDir := filepath.Join(WorkerdStorageDir, "functions", endpointName)
			_ = filepath.WalkDir(endpointDir, func(path string, dirEntry os.DirEntry, walkErr error) error {
				if walkErr != nil || dirEntry.IsDir() {
					return nil
				}
				relPath, _ := filepath.Rel(endpointDir, path)
				relPath = filepath.ToSlash(relPath)
				if relPath == "runner.js" || relPath == "user_code.js" {
					return nil
				}
				embedPath := filepath.ToSlash(filepath.Join("functions", endpointName, relPath))
				moduleName := "./" + relPath
				if strings.HasSuffix(relPath, ".json") {
					fmt.Fprintf(&configBuilder, "    (name = %q, json = embed %q),\n", moduleName, embedPath)
				} else {
					fmt.Fprintf(&configBuilder, "    (name = %q, esModule = embed %q),\n", moduleName, embedPath)
				}
				return nil
			})

			configBuilder.WriteString("  ],\n")
		}
		compatDate := runner.configManager.Get().WorkerdRuntime.CompatibilityDate
		if activeDeployment != nil && activeDeployment.WorkerdRuntimeConfig != nil && activeDeployment.WorkerdRuntimeConfig.CompatibilityDate != "" {
			compatDate = activeDeployment.WorkerdRuntimeConfig.CompatibilityDate
		}
		if compatDate == "" {
			compatDate = DefaultWorkerdCompatibilityDate
		}

		compatFlags := runner.configManager.Get().WorkerdRuntime.CompatibilityFlags
		if activeDeployment != nil && activeDeployment.WorkerdRuntimeConfig != nil && activeDeployment.WorkerdRuntimeConfig.CompatibilityFlags != nil {
			compatFlags = activeDeployment.WorkerdRuntimeConfig.CompatibilityFlags
		}

		fmt.Fprintf(&configBuilder, "  compatibilityDate = %q,\n", compatDate)
		if len(compatFlags) > 0 {
			configBuilder.WriteString("  compatibilityFlags = [")
			for flagIndex, compatibilityFlag := range compatFlags {
				if flagIndex > 0 {
					configBuilder.WriteString(", ")
				}
				fmt.Fprintf(&configBuilder, "%q", compatibilityFlag)
			}
			configBuilder.WriteString("],\n")
		}

		if activeDeployment != nil && len(activeDeployment.EnvironmentVariables) > 0 {
			configBuilder.WriteString("  bindings = [\n")
			sortedEnvKeys := make([]string, 0, len(activeDeployment.EnvironmentVariables))
			for envKey := range activeDeployment.EnvironmentVariables {
				sortedEnvKeys = append(sortedEnvKeys, envKey)
			}
			sort.Strings(sortedEnvKeys)

			for _, envKey := range sortedEnvKeys {
				envValue := activeDeployment.EnvironmentVariables[envKey]
				fmt.Fprintf(&configBuilder, "    (name = %q, text = %s),\n", envKey, strconv.Quote(envValue))
			}
			configBuilder.WriteString("  ],\n")
		}

		configBuilder.WriteString(");\n\n")
	}

	return configBuilder.String()
}

// BuildWorkerRunnerScript provides the ES module adapter supporting both pure function exports and standard worker objects.
func BuildWorkerRunnerScript() string {
	return `import userModule from "./user_code.js";

let asyncLocalStorage = null;
try {
  const asyncHooks = await import("node:async_hooks");
  if (asyncHooks && asyncHooks.AsyncLocalStorage) {
    asyncLocalStorage = new asyncHooks.AsyncLocalStorage();
  }
} catch (_) {}

let fallbackRequestStore = null;

const origLog = console.log;
const origInfo = console.info;
const origWarn = console.warn;
const origError = console.error;

function formatArg(arg) {
  if (typeof arg === "object" && arg !== null) {
    try {
      return JSON.stringify(arg);
    } catch (_) {
      return String(arg);
    }
  }
  return String(arg);
}

function getActiveStore() {
  if (asyncLocalStorage) {
    return asyncLocalStorage.getStore();
  }
  return fallbackRequestStore;
}

console.log = (...args) => {
  try { origLog(...args); } catch (_) {}
  const store = getActiveStore();
  if (store) {
    store.stdoutLogs.push(args.map(formatArg).join(" "));
  }
};
console.info = (...args) => {
  try { origInfo(...args); } catch (_) {}
  const store = getActiveStore();
  if (store) {
    store.stdoutLogs.push(args.map(formatArg).join(" "));
  }
};
console.warn = (...args) => {
  try { origWarn(...args); } catch (_) {}
  const store = getActiveStore();
  if (store) {
    store.stderrLogs.push(args.map(formatArg).join(" "));
  }
};
console.error = (...args) => {
  try { origError(...args); } catch (_) {}
  const store = getActiveStore();
  if (store) {
    store.stderrLogs.push(args.map(formatArg).join(" "));
  }
};

function checkScope(grantedScopes, requiredScope) {
  if (!grantedScopes || !grantedScopes.length) return false;
  if (!requiredScope) return true;

  function parseScope(scope) {
    scope = (scope || "").trim();
    if (scope === "*" || scope === "") return ["*", "*", "*"];
    const colonIdx = scope.indexOf(":");
    if (colonIdx === -1) return [scope, "*", "*"];
    const service = scope.slice(0, colonIdx);
    const rest = scope.slice(colonIdx + 1);
    if (rest === "*") return [service, "*", "*"];
    const dotIdx = rest.indexOf(".");
    if (dotIdx === -1) return [service, rest, "*"];
    return [service, rest.slice(0, dotIdx), rest.slice(dotIdx + 1)];
  }

  const [reqService, reqResource, reqAction] = parseScope(requiredScope);

  for (let granted of grantedScopes) {
    granted = (granted || "").trim();
    if (granted === "*" || granted === requiredScope) return true;
    const [gService, gResource, gAction] = parseScope(granted);
    if (gService === reqService && gResource === "*") return true;
    if (gService !== reqService && gService !== "*") continue;
    if (gResource !== reqResource && gResource !== "*") continue;
    if (gAction === reqAction || gAction === "*") return true;
    if (gAction === "write" && reqAction === "read") return true;
  }
  return false;
}

class AuthContext {
  constructor(data) {
    data = data || {};
    this.userId = data.user_id || "";
    this.serviceAccountId = data.service_account_id || "";
    this.jwt = data.jwt || {};
    this.refreshTokenHash = data.refresh_token_hash || "";
  }

  role() {
    return (this.jwt && this.jwt.role) || "";
  }

  hasScope(requiredScope) {
    if (!this.jwt || !this.jwt.scope) {
      return false;
    }
    const grantedScopes = typeof this.jwt.scope === "string" ? this.jwt.scope.trim().split(/\s+/) : [];
    return checkScope(grantedScopes, requiredScope);
  }

  isAuthenticated() {
    return Boolean(this.userId || this.serviceAccountId);
  }

  isUser() {
    return Boolean(this.userId);
  }

  isServiceAccount() {
    const aud = (this.jwt && this.jwt.aud) || "";
    return Boolean(
      this.serviceAccountId ||
      (this.jwt && this.jwt.role === "service_role") ||
      (typeof aud === "string" && aud.endsWith(":service_account"))
    );
  }
}

let resolvedHandler = null;
if (typeof userModule === "function") {
  resolvedHandler = userModule;
} else if (userModule && typeof userModule.fetch === "function") {
  resolvedHandler = userModule.fetch.bind(userModule);
} else if (userModule && userModule.default && typeof userModule.default.fetch === "function") {
  resolvedHandler = userModule.default.fetch.bind(userModule.default);
} else if (userModule && typeof userModule.default === "function") {
  resolvedHandler = userModule.default;
}

export default {
  async fetch(request, env, ctx) {
    const authHeader = request.headers.get("x-layr-auth-context");
    let authData = null;
    if (authHeader) {
      try {
        const parsed = JSON.parse(authHeader);
        if (parsed && typeof parsed === "object") {
          authData = parsed;
        }
      } catch (_) {}
    }
    const authContext = new AuthContext(authData);

    const executionCtx = ctx && typeof ctx.waitUntil === "function" ? ctx : {
      waitUntil(promise) {
        if (promise && typeof promise.then === "function") {
          promise.catch(() => {});
        }
      },
      passThroughOnException() {}
    };
    executionCtx.auth = authContext;
    try {
      request.auth = authContext;
    } catch (_) {}

    const store = { stdoutLogs: [], stderrLogs: [] };

    const executeFetch = async () => {
      let response;
      try {
        if (!resolvedHandler) {
          throw new TypeError("No valid fetch handler found in worker module");
        }
        response = await resolvedHandler(request, env, executionCtx);
        if (!(response instanceof Response)) {
          throw new TypeError("The script will never generate a response.");
        }
      } catch (err) {
        const errMsg = String((err && (err.stack || err.message)) || err);
        store.stderrLogs.push(errMsg);
        response = new Response("Worker threw exception\n" + errMsg, {
          status: 500,
          statusText: "Internal Server Error",
          headers: { "Content-Type": "text/plain;charset=UTF-8" }
        });
      }

      let finalResponse = response;
      const stdoutStr = store.stdoutLogs.length > 0 ? encodeURIComponent(store.stdoutLogs.join("\n")) : null;
      const stderrStr = store.stderrLogs.length > 0 ? encodeURIComponent(store.stderrLogs.join("\n")) : null;

      if (stdoutStr || stderrStr) {
        let modified = false;
        try {
          if (stdoutStr) finalResponse.headers.set("x-layr-stdout", stdoutStr);
          if (stderrStr) finalResponse.headers.set("x-layr-stderr", stderrStr);
          modified = true;
        } catch (_) {
          // Headers are immutable (e.g. from upstream fetch subrequest)
        }

        if (!modified) {
          const isNullBodyStatus = finalResponse.status === 101 || finalResponse.status === 204 || finalResponse.status === 205 || finalResponse.status === 304;
          const newHeaders = new Headers(finalResponse.headers);
          if (stdoutStr) newHeaders.set("x-layr-stdout", stdoutStr);
          if (stderrStr) newHeaders.set("x-layr-stderr", stderrStr);

          const responseInit = {
            status: finalResponse.status,
            statusText: finalResponse.statusText,
            headers: newHeaders
          };
          if (finalResponse.webSocket) {
            responseInit.webSocket = finalResponse.webSocket;
          }
          finalResponse = new Response(isNullBodyStatus ? null : finalResponse.body, responseInit);
        }
      }

      return finalResponse;
    };

    if (asyncLocalStorage) {
      return await asyncLocalStorage.run(store, executeFetch);
    } else {
      fallbackRequestStore = store;
      try {
        return await executeFetch();
      } finally {
        fallbackRequestStore = null;
      }
    }
  }
};
`
}

func sanitizeCapnpIdentifier(rawName string) string {
	var builder strings.Builder
	capitalizeNext := false

	for index, runeChar := range rawName {
		if (runeChar >= 'a' && runeChar <= 'z') || (runeChar >= 'A' && runeChar <= 'Z') || (runeChar >= '0' && runeChar <= '9') {
			if index == 0 && runeChar >= '0' && runeChar <= '9' {
				builder.WriteString("fn")
			}
			if capitalizeNext {
				builder.WriteString(strings.ToUpper(string(runeChar)))
				capitalizeNext = false
			} else {
				if index == 0 {
					builder.WriteString(strings.ToLower(string(runeChar)))
				} else {
					builder.WriteRune(runeChar)
				}
			}
		} else {
			capitalizeNext = true
		}
	}

	sanitized := builder.String()
	if sanitized == "" {
		return "fnDefault"
	}
	return sanitized
}

func (runner *WorkerdRunner) allocateFreeTCPPort(ctx context.Context) (int, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return 0, fmt.Errorf("context canceled before port allocation: %w", ctxErr)
	}
	listener, listenErr := netListen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		return 0, fmt.Errorf("failed to bind port listener: %w", listenErr)
	}
	defer func() {
		_ = listener.Close()
	}()

	return listener.Addr().(*net.TCPAddr).Port, nil
}
