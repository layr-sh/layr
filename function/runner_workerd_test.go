package function

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"uuid"

	"github.com/stretchr/testify/require"
)

const (
	testDirPermissions  = 0755
	testFilePermissions = 0644
)

func TestFunctionWorkerdRunnerUnit(t *testing.T) {
	t.Run("runner initialization and name", func(t *testing.T) {
		workerdRunner := NewWorkerdRunner(nil)
		require.Equal(t, "workerd", workerdRunner.Name())
		require.Equal(t, filepath.Join(".layr", "function", "workerd"), WorkerdStorageDir)
	})

	t.Run("sanitizeCapnpIdentifier edge cases", func(t *testing.T) {
		require.Equal(t, "helloWorld", sanitizeCapnpIdentifier("hello-world"))
		require.Equal(t, "myFuncV1", sanitizeCapnpIdentifier("my.func.v1"))
		require.Equal(t, "fn123func", sanitizeCapnpIdentifier("123func"))
		require.Equal(t, "validName", sanitizeCapnpIdentifier("validName"))
		require.Equal(t, "fnDefault", sanitizeCapnpIdentifier(""))
	})

	t.Run("runner script generation", func(t *testing.T) {
		runnerScript := BuildWorkerRunnerScript()
		require.Contains(t, runnerScript, `import userModule from "./user_code.js";`)
		require.Contains(t, runnerScript, `class AuthContext {`)
		require.Contains(t, runnerScript, `isAuthenticated() {`)
		require.Contains(t, runnerScript, `isUser() {`)
		require.Contains(t, runnerScript, `isServiceAccount() {`)
		require.Contains(t, runnerScript, `role() {`)
		require.Contains(t, runnerScript, `hasScope(requiredScope) {`)
		require.Contains(t, runnerScript, `export default {`)
	})

	t.Run("capnp config compilation", func(t *testing.T) {
		baseConfigManager := NewConfigManager(nil)
		baseConfig := DefaultConfig()
		baseConfigManager.SetMemoryConfig(baseConfig)

		workerdRunner := NewWorkerdRunner(baseConfigManager)
		workerdRunner.activePorts["hello"] = 9001
		workerdRunner.activePorts["world-api"] = 9002

		endpointHelloDir := filepath.Join(WorkerdStorageDir, "functions", "hello")
		require.NoError(t, os.MkdirAll(endpointHelloDir, testDirPermissions))
		t.Cleanup(func() { _ = os.RemoveAll(endpointHelloDir) })
		require.NoError(t, os.WriteFile(filepath.Join(endpointHelloDir, "extra.js"), []byte("export const a = 1;"), testFilePermissions))
		require.NoError(t, os.WriteFile(filepath.Join(endpointHelloDir, "config.json"), []byte("{}"), testFilePermissions))

		capnpConfig := workerdRunner.BuildCapnpConfig()
		require.Contains(t, capnpConfig, `using Workerd = import "/workerd/workerd.capnp";`)
		require.Contains(t, capnpConfig, `(name = "hello", worker = .helloWorker)`)
		require.Contains(t, capnpConfig, `(name = "worldApi", address = "127.0.0.1:9002"`)
		require.Contains(t, capnpConfig, `const helloWorker :Workerd.Worker = (`)
		require.Contains(t, capnpConfig, `(name = "runner.js", esModule = embed "functions/hello/runner.js")`)
		require.Contains(t, capnpConfig, `(name = "./extra.js", esModule = embed "functions/hello/extra.js")`)
		require.Contains(t, capnpConfig, `(name = "./config.json", json = embed "functions/hello/config.json")`)
		require.Contains(t, capnpConfig, `compatibilityDate = "2026-08-04"`)
		require.Contains(t, capnpConfig, `compatibilityFlags = ["nodejs_compat"]`)

		// Global config override via ConfigManager
		configManager := NewConfigManager(nil)
		customConfig := DefaultConfig()
		customConfig.WorkerdRuntime.CompatibilityDate = "2024-09-23"
		customConfig.WorkerdRuntime.CompatibilityFlags = []string{"nodejs_compat", "nodejs_als"}
		configManager.SetMemoryConfig(customConfig)

		customCompatWorkerdRunner := NewWorkerdRunner(configManager)
		customCompatWorkerdRunner.activePorts["compat-worker"] = 9003
		customCapnp := customCompatWorkerdRunner.BuildCapnpConfig()
		require.Contains(t, customCapnp, `compatibilityDate = "2024-09-23"`)
		require.Contains(t, customCapnp, `compatibilityFlags = ["nodejs_compat", "nodejs_als"]`)

		// Deployment-specific override
		depWorkerdRunner := NewWorkerdRunner(configManager)
		depWorkerdRunner.activePorts["override-worker"] = 9004
		depWorkerdRunner.activeDeployments["override-worker"] = &Deployment{
			WorkerdRuntimeConfig: &WorkerdRuntimeConfig{
				CompatibilityDate:  "2025-01-01",
				CompatibilityFlags: []string{"streams_enable_constructors"},
			},
		}
		overrideCapnp := depWorkerdRunner.BuildCapnpConfig()
		require.Contains(t, overrideCapnp, `compatibilityDate = "2025-01-01"`)
		require.Contains(t, overrideCapnp, `compatibilityFlags = ["streams_enable_constructors"]`)

		// Empty compatibility flags
		emptyFlagsConfigManager := NewConfigManager(nil)
		emptyFlagsConfig := DefaultConfig()
		emptyFlagsConfig.WorkerdRuntime.CompatibilityFlags = []string{}
		emptyFlagsConfigManager.SetMemoryConfig(emptyFlagsConfig)

		emptyFlagsWorkerdRunner := NewWorkerdRunner(emptyFlagsConfigManager)
		emptyFlagsWorkerdRunner.activePorts["no-flags"] = 9005
		emptyCapnp := emptyFlagsWorkerdRunner.BuildCapnpConfig()
		require.NotContains(t, emptyCapnp, "compatibilityFlags =")
	})

	t.Run("free port allocation", func(t *testing.T) {
		workerdRunner := NewWorkerdRunner(nil)
		port, err := workerdRunner.allocateFreeTCPPort(context.Background())
		require.NoError(t, err)
		require.Greater(t, port, 1024)

		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = workerdRunner.allocateFreeTCPPort(canceledCtx)
		require.Error(t, err)
	})

	t.Run("binary resolution paths", func(t *testing.T) {
		ctx := context.Background()

		// 1. Existing binary resolution
		workerdRunner := NewWorkerdRunner(nil)
		resolved, err := workerdRunner.ResolveBinary(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, resolved)

		// 2. Download execution via mock HTTP server
		mockDownloadServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, serverRequest *http.Request) {
			responseWriter.WriteHeader(http.StatusOK)
			gzipWriter := gzip.NewWriter(responseWriter)
			_, _ = gzipWriter.Write([]byte("#!/bin/sh\necho workerd\n"))
			_ = gzipWriter.Close()
		}))
		defer mockDownloadServer.Close()

		tempTarget := filepath.Join(t.TempDir(), "downloaded-workerd")
		resolvedAuto, downloadErr := downloadWorkerdBinaryForPlatform(ctx, mockDownloadServer.URL, runtime.GOOS, runtime.GOARCH, tempTarget)
		require.NoError(t, downloadErr)
		require.Equal(t, tempTarget, resolvedAuto)

		// 3. Download failure on invalid path
		_, err = downloadWorkerdBinaryForPlatform(ctx, mockDownloadServer.URL, runtime.GOOS, runtime.GOARCH, "/dev/null/forbidden/workerd")
		require.Error(t, err)

		// 4. Empty path in downloadWorkerdBinary
		_, err = downloadWorkerdBinary(ctx, "")
		require.Error(t, err)

		// 5. Server returns HTTP 404
		notFoundServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, serverRequest *http.Request) {
			responseWriter.WriteHeader(http.StatusNotFound)
		}))
		defer notFoundServer.Close()
		_, err = downloadWorkerdBinaryForPlatform(ctx, notFoundServer.URL, runtime.GOOS, runtime.GOARCH, filepath.Join(t.TempDir(), "notfound"))
		require.Error(t, err)

		// 6. Invalid gzip stream
		invalidGzipServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, serverRequest *http.Request) {
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte("not a gzip stream"))
		}))
		defer invalidGzipServer.Close()
		_, err = downloadWorkerdBinaryForPlatform(ctx, invalidGzipServer.URL, runtime.GOOS, runtime.GOARCH, filepath.Join(t.TempDir(), "invalid"))
		require.Error(t, err)

		// 7. resolveWorkerdReleaseAsset edge cases
		asset, assetErr := resolveWorkerdReleaseAsset("darwin", "arm64")
		require.NoError(t, assetErr)
		require.Equal(t, "workerd-darwin-arm64.gz", asset)

		asset, assetErr = resolveWorkerdReleaseAsset("darwin", "amd64")
		require.NoError(t, assetErr)
		require.Equal(t, "workerd-darwin-64.gz", asset)

		_, assetErr = resolveWorkerdReleaseAsset("darwin", "mips")
		require.Error(t, assetErr)

		asset, assetErr = resolveWorkerdReleaseAsset("linux", "arm64")
		require.NoError(t, assetErr)
		require.Equal(t, "workerd-linux-arm64.gz", asset)

		asset, assetErr = resolveWorkerdReleaseAsset("linux", "amd64")
		require.NoError(t, assetErr)
		require.Equal(t, "workerd-linux-64.gz", asset)

		_, assetErr = resolveWorkerdReleaseAsset("linux", "s390x")
		require.Error(t, assetErr)

		asset, assetErr = resolveWorkerdReleaseAsset("windows", "amd64")
		require.NoError(t, assetErr)
		require.Equal(t, "workerd-windows-64.gz", asset)

		_, assetErr = resolveWorkerdReleaseAsset("windows", "arm64")
		require.Error(t, assetErr)

		_, assetErr = resolveWorkerdReleaseAsset("freebsd", "amd64")
		require.Error(t, assetErr)

		require.Equal(t, "workerd.exe", workerdBinaryFilename("windows"))
		require.Equal(t, "workerd", workerdBinaryFilename("darwin"))
		require.Equal(t, "workerd", workerdBinaryFilename("linux"))

		// 8. Unsupported platform in downloadWorkerdBinaryForPlatform
		_, err = downloadWorkerdBinaryForPlatform(ctx, WorkerdDownloadURL, "darwin", "mips", filepath.Join(t.TempDir(), "fail"))
		require.Error(t, err)

		// 9. Failing request with bad URL
		_, err = downloadWorkerdBinaryForPlatform(ctx, "http://invalid-url-that-does-not-exist.test:9999", runtime.GOOS, runtime.GOARCH, filepath.Join(t.TempDir(), "target"))
		require.Error(t, err)

		// 10. Canceled context causes download request execution failure
		canceledDownloadCtx, downloadCancel := context.WithCancel(context.Background())
		downloadCancel()
		_, err = downloadWorkerdBinaryForPlatform(canceledDownloadCtx, WorkerdDownloadURL, runtime.GOOS, runtime.GOARCH, filepath.Join(t.TempDir(), "fail"))
		require.Error(t, err)

		// 11. Target file creation failure (read-only dir)
		readOnlyDir := t.TempDir()
		require.NoError(t, os.Chmod(readOnlyDir, 0555))
		_, err = downloadWorkerdBinaryForPlatform(ctx, mockDownloadServer.URL, runtime.GOOS, runtime.GOARCH, filepath.Join(readOnlyDir, "workerd"))
		require.Error(t, err)

		// 13. Broken gzip decompression mid-copy
		corruptedGzipServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, serverRequest *http.Request) {
			responseWriter.WriteHeader(http.StatusOK)
			validGzipHeader := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff}
			_, _ = responseWriter.Write(validGzipHeader)
			_, _ = responseWriter.Write([]byte{0x01, 0x02, 0x03, 0x04})
		}))
		defer corruptedGzipServer.Close()
		_, err = downloadWorkerdBinaryForPlatform(ctx, corruptedGzipServer.URL, runtime.GOOS, runtime.GOARCH, filepath.Join(t.TempDir(), "fail"))
		require.Error(t, err)

		// 14. Unsupported platform in resolveWorkerdBinaryForPlatform
		_, err = resolveWorkerdBinaryForPlatform(ctx, "darwin", "mips")
		require.Error(t, err)

		// 15. waitForPortLocked with canceled context
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()
		workerdRunner.waitForPortLocked(canceledCtx, 9999)
	})

	t.Run("lifecycle start and stop", func(t *testing.T) {
		workerdRunner := NewWorkerdRunner(nil)
		ctx := context.Background()

		require.NoError(t, workerdRunner.Start(ctx))
		require.NoError(t, workerdRunner.Start(ctx)) // Idempotent

		runnerHealth, healthErr := workerdRunner.Health(ctx)
		require.NoError(t, healthErr)
		require.True(t, runnerHealth.Available)
		require.Equal(t, 0, runnerHealth.ActiveIsolates)

		require.NoError(t, workerdRunner.Stop())
		runnerHealth, _ = workerdRunner.Health(ctx)
		require.False(t, runnerHealth.Available)
	})

	t.Run("deploy validation and error paths", func(t *testing.T) {
		ctx := context.Background()

		workerdRunner := NewWorkerdRunner(nil)
		require.ErrorIs(t, workerdRunner.Deploy(ctx, nil, nil), ErrEndpointNotFound)
		require.ErrorIs(t, workerdRunner.Deploy(ctx, &Endpoint{Name: "test"}, nil), ErrDeploymentNotFound)
	})

	t.Run("forward undeployed endpoint returns ErrDeploymentNotFound", func(t *testing.T) {
		workerdRunner := NewWorkerdRunner(nil)
		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/function/not-deployed", nil)

		require.ErrorIs(t, workerdRunner.Forward(responseRecorder, request, nil), ErrEndpointNotFound)
		require.ErrorIs(t, workerdRunner.Forward(responseRecorder, request, &Endpoint{Name: "not-deployed"}), ErrDeploymentNotFound)
	})

	t.Run("forward fallback downloads workerd binary on invocation", func(t *testing.T) {
		ctx := context.Background()
		workerdRunner := NewWorkerdRunner(nil)
		testEndpoint := &Endpoint{Name: "active-func", Runtime: "workerd"}
		testDeployment := &Deployment{
			ID:            uuid.NewV7(),
			Version:       1,
			BundleContent: []byte("export default { async fetch() { return new Response('ok'); } };"),
		}
		require.NoError(t, workerdRunner.Deploy(ctx, testEndpoint, testDeployment))
		defer func() {
			_ = workerdRunner.Stop()
		}()

		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/function/active-func", nil)
		forwardErr := workerdRunner.Forward(responseRecorder, request, testEndpoint)
		require.NoError(t, forwardErr)
	})

	t.Run("deploy, forward, redeploy, and undeploy lifecycle with mock process", func(t *testing.T) {
		workerdRunner := NewWorkerdRunner(nil)
		ctx := context.Background()
		require.NoError(t, workerdRunner.Start(ctx))
		defer func() {
			_ = workerdRunner.Stop()
		}()

		firstEndpoint := &Endpoint{Name: "hello", Runtime: "workerd"}
		firstDeployment := &Deployment{
			Version:       1,
			BundleContent: []byte(`export default async (req) => new Response("Hello!");`),
		}

		require.NoError(t, workerdRunner.Deploy(ctx, firstEndpoint, firstDeployment))

		runnerHealth, healthErr := workerdRunner.Health(ctx)
		require.NoError(t, healthErr)
		require.Equal(t, 1, runnerHealth.ActiveIsolates)
		require.Greater(t, runnerHealth.ProcessPID, 0)

		// Deploy second endpoint while process is running (tests killing old and restarting new)
		secondEndpoint := &Endpoint{Name: "world", Runtime: "workerd"}
		secondDeployment := &Deployment{
			Version:       1,
			BundleContent: []byte(`export default async (req) => new Response("World!");`),
		}
		require.NoError(t, workerdRunner.Deploy(ctx, secondEndpoint, secondDeployment))

		runnerHealth, healthErr = workerdRunner.Health(ctx)
		require.NoError(t, healthErr)
		require.Equal(t, 2, runnerHealth.ActiveIsolates)

		// Test reverse proxy forward handler (mock workerd target via httptest server)
		targetServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, serverRequest *http.Request) {
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte("from-workerd-worker"))
		}))
		defer targetServer.Close()

		// Overwrite proxy to point to test server
		targetServerURL, targetServerURLErr := url.Parse(targetServer.URL)
		require.NoError(t, targetServerURLErr)
		targetReverseProxy := httputil.NewSingleHostReverseProxy(targetServerURL)
		targetReverseProxy.ErrorHandler = workerdRunner.activeProxies[firstEndpoint.Name].ErrorHandler
		workerdRunner.activeProxies[firstEndpoint.Name] = targetReverseProxy

		targetRequest, targetRequestErr := http.NewRequestWithContext(ctx, http.MethodGet, targetServer.URL, nil)
		require.NoError(t, targetRequestErr)
		responseRecorder := httptest.NewRecorder()
		forwardErr := workerdRunner.Forward(responseRecorder, targetRequest, firstEndpoint)
		require.NoError(t, forwardErr)
		require.Equal(t, "from-workerd-worker", responseRecorder.Body.String())

		// Trigger proxy error handler by pointing to closed server
		closedServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		closedServer.Close()
		closedServerURL, closedServerURLErr := url.Parse(closedServer.URL)
		require.NoError(t, closedServerURLErr)
		closedReverseProxy := httputil.NewSingleHostReverseProxy(closedServerURL)
		closedReverseProxy.ErrorHandler = targetReverseProxy.ErrorHandler
		workerdRunner.activeProxies[firstEndpoint.Name] = closedReverseProxy

		errorTargetRequest, errorTargetRequestErr := http.NewRequestWithContext(ctx, http.MethodGet, closedServer.URL, nil)
		require.NoError(t, errorTargetRequestErr)
		errorResponseRecorder := httptest.NewRecorder()
		_ = workerdRunner.Forward(errorResponseRecorder, errorTargetRequest, firstEndpoint)
		require.Equal(t, http.StatusBadGateway, errorResponseRecorder.Code)

		// Undeploy one endpoint
		require.NoError(t, workerdRunner.Undeploy(ctx, "hello"))
		runnerHealth, _ = workerdRunner.Health(ctx)
		require.Equal(t, 1, runnerHealth.ActiveIsolates)

		// Undeploy remaining endpoint so len(activePorts) reaches 0 and kills process
		require.NoError(t, workerdRunner.Undeploy(ctx, "world"))
		runnerHealth, _ = workerdRunner.Health(ctx)
		require.Equal(t, 0, runnerHealth.ActiveIsolates)

		// Redeploy so processCmd is actively running when Stop is invoked
		require.NoError(t, workerdRunner.Deploy(ctx, firstEndpoint, firstDeployment))

		// Stop while process is actively running (tests killing active processCmd)
		require.NoError(t, workerdRunner.Stop())
		runnerHealth, _ = workerdRunner.Health(ctx)
		require.False(t, runnerHealth.Available)
	})

	t.Run("extractBundle formats and wrappers", func(t *testing.T) {
		// 1. Raw bundle
		destRaw := t.TempDir()
		rawDeployment := &Deployment{
			BundleFormat:  "raw",
			BundleContent: []byte("export default { fetch() {} };"),
		}
		require.NoError(t, extractBundle(destRaw, rawDeployment, "index.js"))
		content, readErr := os.ReadFile(filepath.Join(destRaw, "user_code.js"))
		require.NoError(t, readErr)
		require.Equal(t, "export default { fetch() {} };", string(content))

		// 2. Tar bundle with directory and sub-file
		destTar := t.TempDir()
		var tarBuffer bytes.Buffer
		tarWriter := tar.NewWriter(&tarBuffer)
		require.NoError(t, tarWriter.WriteHeader(&tar.Header{
			Name:     "src/",
			Typeflag: tar.TypeDir,
			Mode:     0755,
		}))
		subContent := []byte("export default { val: 42 };")
		require.NoError(t, tarWriter.WriteHeader(&tar.Header{
			Name: "src/worker.js",
			Mode: 0644,
			Size: int64(len(subContent)),
		}))
		_, _ = tarWriter.Write(subContent)
		require.NoError(t, tarWriter.Close())

		tarDeployment := &Deployment{
			BundleFormat:  "tar",
			BundleContent: tarBuffer.Bytes(),
		}
		require.NoError(t, extractBundle(destTar, tarDeployment, "src/worker.js"))
		userCode, err := os.ReadFile(filepath.Join(destTar, "user_code.js"))
		require.NoError(t, err)
		require.Contains(t, string(userCode), `"./src/worker.js"`)

		// 3. Zip bundle
		destZip := t.TempDir()
		var zipBuffer bytes.Buffer
		zipWriter := zip.NewWriter(&zipBuffer)
		fileWriter, _ := zipWriter.Create("custom.js")
		_, _ = fileWriter.Write([]byte("export default 123;"))
		require.NoError(t, zipWriter.Close())

		zipDeployment := &Deployment{
			BundleFormat:  "zip",
			BundleContent: zipBuffer.Bytes(),
		}
		require.NoError(t, extractBundle(destZip, zipDeployment, "custom.js"))
		zipUserCode, err := os.ReadFile(filepath.Join(destZip, "user_code.js"))
		require.NoError(t, err)
		require.Contains(t, string(zipUserCode), `"./custom.js"`)

		// 4. Corrupt zip returns error
		corruptDeployment := &Deployment{
			BundleFormat:  "zip",
			BundleContent: []byte("not a zip file"),
		}
		require.Error(t, extractBundle(t.TempDir(), corruptDeployment, "custom.js"))

		// 5. Tar entry read error
		corruptTarDeployment := &Deployment{
			BundleFormat:  "tar",
			BundleContent: []byte("not a tar"),
		}
		require.Error(t, extractBundle(t.TempDir(), corruptTarDeployment, "index.js"))

		// 6. Tar path traversal and absolute path skipped
		destTraversal := t.TempDir()
		var traversalBuffer bytes.Buffer
		traversalWriter := tar.NewWriter(&traversalBuffer)
		_ = traversalWriter.WriteHeader(&tar.Header{Name: "../outside.js", Mode: 0644, Size: 4})
		_, _ = traversalWriter.Write([]byte("test"))
		_ = traversalWriter.WriteHeader(&tar.Header{Name: "/absolute.js", Mode: 0644, Size: 4})
		_, _ = traversalWriter.Write([]byte("test"))
		_ = traversalWriter.Close()
		require.NoError(t, extractBundle(destTraversal, &Deployment{BundleFormat: "tar", BundleContent: traversalBuffer.Bytes()}, "index.js"))

		// 7. Tar directory creation failure
		destDirFail := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(destDirFail, "folder"), []byte("file"), testFilePermissions))
		var dirTarBuffer bytes.Buffer
		dirTarWriter := tar.NewWriter(&dirTarBuffer)
		_ = dirTarWriter.WriteHeader(&tar.Header{Name: "folder/", Typeflag: tar.TypeDir, Mode: 0755})
		_ = dirTarWriter.Close()
		require.Error(t, extractBundle(destDirFail, &Deployment{BundleFormat: "tar", BundleContent: dirTarBuffer.Bytes()}, "index.js"))

		// 8. Tar parent directory creation failure
		destParentFail := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(destParentFail, "parent"), []byte("file"), testFilePermissions))
		var parentTarBuffer bytes.Buffer
		parentTarWriter := tar.NewWriter(&parentTarBuffer)
		_ = parentTarWriter.WriteHeader(&tar.Header{Name: "parent/child.js", Mode: 0644, Size: 4})
		_, _ = parentTarWriter.Write([]byte("test"))
		_ = parentTarWriter.Close()
		require.Error(t, extractBundle(destParentFail, &Deployment{BundleFormat: "tar", BundleContent: parentTarBuffer.Bytes()}, "index.js"))

		// 9. Tar read content error (truncated stream)
		var truncTarBuffer bytes.Buffer
		truncTarWriter := tar.NewWriter(&truncTarBuffer)
		_ = truncTarWriter.WriteHeader(&tar.Header{Name: "trunc.js", Mode: 0644, Size: 500})
		_, _ = truncTarBuffer.Write([]byte("short"))
		require.Error(t, extractBundle(t.TempDir(), &Deployment{BundleFormat: "tar", BundleContent: truncTarBuffer.Bytes()}, "index.js"))

		// 10. Tar write file error (target is a directory)
		destFileFail := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(destFileFail, "file.js"), testDirPermissions))
		var fileTarBuffer bytes.Buffer
		fileTarWriter := tar.NewWriter(&fileTarBuffer)
		_ = fileTarWriter.WriteHeader(&tar.Header{Name: "file.js", Mode: 0644, Size: 4})
		_, _ = fileTarWriter.Write([]byte("test"))
		_ = fileTarWriter.Close()
		require.Error(t, extractBundle(destFileFail, &Deployment{BundleFormat: "tar", BundleContent: fileTarBuffer.Bytes()}, "index.js"))

		// 11. Zip path traversal, absolute path and directory entry
		destZipTraversal := t.TempDir()
		var zipTraversalBuffer bytes.Buffer
		zipTraversalWriter := zip.NewWriter(&zipTraversalBuffer)
		_, _ = zipTraversalWriter.Create("../outside.js")
		_, _ = zipTraversalWriter.Create("/abs.js")
		_, _ = zipTraversalWriter.Create("dir/")
		entryWriter, _ := zipTraversalWriter.Create("valid.js")
		_, _ = entryWriter.Write([]byte("console.log(1);"))
		_ = zipTraversalWriter.Close()
		require.NoError(t, extractBundle(destZipTraversal, &Deployment{BundleFormat: "zip", BundleContent: zipTraversalBuffer.Bytes()}, "valid.js"))

		// 12. Zip directory creation failure
		destZipDirFail := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(destZipDirFail, "folder"), []byte("file"), testFilePermissions))
		var zipDirBuffer bytes.Buffer
		zipDirWriter := zip.NewWriter(&zipDirBuffer)
		_, _ = zipDirWriter.Create("folder/")
		_ = zipDirWriter.Close()
		require.Error(t, extractBundle(destZipDirFail, &Deployment{BundleFormat: "zip", BundleContent: zipDirBuffer.Bytes()}, "index.js"))

		// 13. Zip parent directory creation failure
		destZipParentFail := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(destZipParentFail, "parent"), []byte("file"), testFilePermissions))
		var zipParentBuffer bytes.Buffer
		zipParentWriter := zip.NewWriter(&zipParentBuffer)
		childWriter, _ := zipParentWriter.Create("parent/child.js")
		_, _ = childWriter.Write([]byte("child"))
		_ = zipParentWriter.Close()
		require.Error(t, extractBundle(destZipParentFail, &Deployment{BundleFormat: "zip", BundleContent: zipParentBuffer.Bytes()}, "index.js"))

		// 14. Zip unsupported compression method causes openErr
		var zipMethodBuffer bytes.Buffer
		zipMethodWriter := zip.NewWriter(&zipMethodBuffer)
		normalEntryWriter, _ := zipMethodWriter.Create("custom.js")
		_, _ = normalEntryWriter.Write([]byte("data"))
		_ = zipMethodWriter.Close()
		methodZipBytes := zipMethodBuffer.Bytes()
		for byteIndex := 0; byteIndex < len(methodZipBytes)-4; byteIndex++ {
			if methodZipBytes[byteIndex] == 0x50 && methodZipBytes[byteIndex+1] == 0x4b && methodZipBytes[byteIndex+2] == 0x03 && methodZipBytes[byteIndex+3] == 0x04 {
				methodZipBytes[byteIndex+8] = 0xFF
				methodZipBytes[byteIndex+9] = 0xFF
			}
			if methodZipBytes[byteIndex] == 0x50 && methodZipBytes[byteIndex+1] == 0x4b && methodZipBytes[byteIndex+2] == 0x01 && methodZipBytes[byteIndex+3] == 0x02 {
				methodZipBytes[byteIndex+10] = 0xFF
				methodZipBytes[byteIndex+11] = 0xFF
			}
		}
		require.Error(t, extractBundle(t.TempDir(), &Deployment{BundleFormat: "zip", BundleContent: methodZipBytes}, "index.js"))

		// 15. Zip read content error (corrupt deflate stream)
		var zipCorruptBuffer bytes.Buffer
		zipCorruptWriter := zip.NewWriter(&zipCorruptBuffer)
		deflateFileHeader := &zip.FileHeader{Name: "deflate.js", Method: zip.Deflate}
		deflateWriter, _ := zipCorruptWriter.CreateHeader(deflateFileHeader)
		_, _ = deflateWriter.Write([]byte("some compressable content that spans multiple bytes for deflate"))
		_ = zipCorruptWriter.Close()
		corruptZipBytes := zipCorruptBuffer.Bytes()
		for byteIndex := 35; byteIndex < 45 && byteIndex < len(corruptZipBytes); byteIndex++ {
			corruptZipBytes[byteIndex] = 0xFF
		}
		require.Error(t, extractBundle(t.TempDir(), &Deployment{BundleFormat: "zip", BundleContent: corruptZipBytes}, "index.js"))

		// 16. Zip write file failure (target is directory)
		destZipFileFail := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(destZipFileFail, "file.js"), testDirPermissions))
		var zipFileBuffer bytes.Buffer
		zipFileWriter := zip.NewWriter(&zipFileBuffer)
		fileEntryWriter, _ := zipFileWriter.Create("file.js")
		_, _ = fileEntryWriter.Write([]byte("content"))
		_ = zipFileWriter.Close()
		require.Error(t, extractBundle(destZipFileFail, &Deployment{BundleFormat: "zip", BundleContent: zipFileBuffer.Bytes()}, "index.js"))

		// 17. Empty entrypoint defaults to index.js
		destDefaultEp := t.TempDir()
		var emptyEpTarBuffer bytes.Buffer
		emptyEpTarWriter := tar.NewWriter(&emptyEpTarBuffer)
		_ = emptyEpTarWriter.WriteHeader(&tar.Header{Name: "other.js", Mode: 0644, Size: 4})
		_, _ = emptyEpTarWriter.Write([]byte("test"))
		_ = emptyEpTarWriter.Close()
		require.NoError(t, extractBundle(destDefaultEp, &Deployment{BundleFormat: "tar", BundleContent: emptyEpTarBuffer.Bytes()}, ""))
		userCodeDefaultContent, readUserCodeErr := os.ReadFile(filepath.Join(destDefaultEp, "user_code.js"))
		require.NoError(t, readUserCodeErr)
		require.Contains(t, string(userCodeDefaultContent), `"./index.js"`)

		// 18. Wrapper write failure (dest directory is read-only)
		destWrapperFail := t.TempDir()
		require.NoError(t, os.Chmod(destWrapperFail, 0555))
		require.Error(t, extractBundle(destWrapperFail, &Deployment{BundleFormat: "tar", BundleContent: traversalBuffer.Bytes()}, "missing.js"))
		require.NoError(t, os.Chmod(destWrapperFail, 0755))
	})

	t.Run("runner coverage edge cases", func(t *testing.T) {
		// 1. Static storage directory
		require.Equal(t, filepath.Join(".layr", "function", "workerd"), WorkerdStorageDir)

		// 2. ResolveWorkerdBinary standalone and PATH lookup
		testBinaryDirectory := t.TempDir()
		dummyBinaryPath := filepath.Join(testBinaryDirectory, workerdBinaryFilename(runtime.GOOS))
		require.NoError(t, os.WriteFile(dummyBinaryPath, []byte("#!/bin/sh\nexit 0\n"), testFilePermissions))
		t.Setenv("PATH", testBinaryDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
		resolvedBinaryPath, resolveBinaryErr := ResolveWorkerdBinary(context.Background())
		require.NoError(t, resolveBinaryErr)
		require.NotEmpty(t, resolvedBinaryPath)

		// 3. resolveWorkerdBinaryForPlatform with cancelled context
		cancelledCtx, lookupCancel := context.WithCancel(context.Background())
		lookupCancel()
		_, _ = resolveWorkerdBinaryForPlatform(cancelledCtx, runtime.GOOS, runtime.GOARCH)
		_, _ = resolveWorkerdBinaryForPlatform(cancelledCtx, "linux", "amd64")

		// 4. downloadWorkerdBinaryForPlatform with HTTP 404 response and invalid URL
		failingHTTPServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			responseWriter.WriteHeader(http.StatusNotFound)
		}))
		defer failingHTTPServer.Close()
		_, downloadErr := downloadWorkerdBinaryForPlatform(context.Background(), failingHTTPServer.URL, runtime.GOOS, runtime.GOARCH, filepath.Join(t.TempDir(), "bin"))
		require.Error(t, downloadErr)

		_, invalidRequestErr := downloadWorkerdBinaryForPlatform(context.Background(), "http://[::1]:namedport", runtime.GOOS, runtime.GOARCH, filepath.Join(t.TempDir(), "bin"))
		require.Error(t, invalidRequestErr)

		// 5. allocateFreeTCPPort with cancelled context
		workerdRunner := NewWorkerdRunner(nil)
		portCancelCtx, portCancel := context.WithCancel(context.Background())
		portCancel()
		_, portErr := workerdRunner.allocateFreeTCPPort(portCancelCtx)
		require.Error(t, portErr)

		// 6. Deploy with cancelled context (triggering allocateFreeTCPPort error)
		deployCancelCtx, deployCancel := context.WithCancel(context.Background())
		deployCancel()
		targetEndpoint := &Endpoint{Name: "test-cancelled-deploy"}
		testDeployment := &Deployment{BundleFormat: "raw", BundleContent: []byte("export default {};")}
		deploymentErr := workerdRunner.Deploy(deployCancelCtx, targetEndpoint, testDeployment)
		require.Error(t, deploymentErr)

		// 7. BuildCapnpConfig with empty compatibility date fallback
		capnpConfigManager := NewConfigManager(nil)
		capnpConfig := capnpConfigManager.Get()
		capnpConfig.WorkerdRuntime.CompatibilityDate = ""
		capnpConfigManager.SetMemoryConfig(capnpConfig)
		capnpWorkerdRunner := NewWorkerdRunner(capnpConfigManager)
		capnpWorkerdRunner.activeDeployments["testEndpoint"] = &Deployment{BundleFormat: "raw"}
		capnpWorkerdRunner.activePorts["testEndpoint"] = 8080
		compiledCapnpConfig := capnpWorkerdRunner.BuildCapnpConfig()
		require.Contains(t, compiledCapnpConfig, DefaultWorkerdCompatibilityDate)

		// 8. allocateFreeTCPPort with netListen failure
		previousNetListen := netListen
		netListen = func(network, address string) (net.Listener, error) {
			return nil, errors.New("simulated net listen failure")
		}
		_, mockNetListenErr := workerdRunner.allocateFreeTCPPort(context.Background())
		require.Error(t, mockNetListenErr)
		netListen = previousNetListen

		// 9. Forward with cancelled context triggering ResolveBinary failure
		cancelForwardCtx, cancelForwardCancel := context.WithCancel(context.Background())
		cancelForwardCancel()
		forwardRequest := httptest.NewRequestWithContext(cancelForwardCtx, http.MethodGet, "/test", nil)
		forwardResponseRecorder := httptest.NewRecorder()
		forwardErr := workerdRunner.Forward(forwardResponseRecorder, forwardRequest, &Endpoint{Name: "test-ep"})
		require.Error(t, forwardErr)

		// 10. regenerateAndReloadLocked with execCommandContext start failure
		previousExecCommandContext := execCommandContext
		execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "nonexistent-binary"))
		}
		reloadWorkerdRunner := NewWorkerdRunner(nil)
		reloadWorkerdRunner.activePorts["fail-spawn-ep"] = 9998
		reloadErr := reloadWorkerdRunner.regenerateAndReloadLocked(context.Background())
		require.Error(t, reloadErr)
		execCommandContext = previousExecCommandContext

		// 11. regenerateAndReloadLocked with cancelled context (ResolveBinary error)
		cancelReloadCtx, cancelReloadCancel := context.WithCancel(context.Background())
		cancelReloadCancel()
		reloadCancelWorkerdRunner := NewWorkerdRunner(nil)
		reloadCancelWorkerdRunner.activePorts["cancel-ep"] = 9997
		reloadCancelErr := reloadCancelWorkerdRunner.regenerateAndReloadLocked(cancelReloadCtx)
		require.Error(t, reloadCancelErr)

		// 12. Storage directory blocked by file for Start, Deploy, and regenerateAndReloadLocked
		storageBackupDir := WorkerdStorageDir + ".backup"
		_ = os.RemoveAll(storageBackupDir)
		if _, statErr := os.Stat(WorkerdStorageDir); statErr == nil {
			require.NoError(t, os.Rename(WorkerdStorageDir, storageBackupDir))
			defer func() {
				_ = os.RemoveAll(WorkerdStorageDir)
				_ = os.Rename(storageBackupDir, WorkerdStorageDir)
			}()
		}
		require.NoError(t, os.WriteFile(WorkerdStorageDir, []byte("blocked-as-file"), testFilePermissions))
		blockedWorkerdRunner := NewWorkerdRunner(nil)
		startBlockedErr := blockedWorkerdRunner.Start(context.Background())
		require.Error(t, startBlockedErr)

		deployBlockedErr := blockedWorkerdRunner.Deploy(context.Background(), &Endpoint{Name: "blocked-ep"}, &Deployment{BundleFormat: "raw"})
		require.Error(t, deployBlockedErr)

		reloadBlockedErr := blockedWorkerdRunner.regenerateAndReloadLocked(context.Background())
		require.Error(t, reloadBlockedErr)

		require.NoError(t, os.RemoveAll(WorkerdStorageDir))
		require.NoError(t, os.MkdirAll(WorkerdStorageDir, testDirPermissions))

		// 13. Deploy write errors with read-only endpoint directory
		functionsDir := filepath.Join(WorkerdStorageDir, "functions")
		require.NoError(t, os.MkdirAll(functionsDir, testDirPermissions))
		readOnlyEndpointDir := filepath.Join(functionsDir, "read-only-ep")
		require.NoError(t, os.MkdirAll(readOnlyEndpointDir, testDirPermissions))
		require.NoError(t, os.Chmod(readOnlyEndpointDir, 0555))
		defer func() {
			_ = os.Chmod(readOnlyEndpointDir, testDirPermissions)
			_ = os.RemoveAll(readOnlyEndpointDir)
		}()
		readOnlyEndpoint := &Endpoint{Name: "read-only-ep", Entrypoint: "index.js"}
		rawDeployment := &Deployment{BundleFormat: "raw", BundleContent: []byte("export default {};")}
		deployReadOnlyErr := workerdRunner.Deploy(context.Background(), readOnlyEndpoint, rawDeployment)
		require.Error(t, deployReadOnlyErr)

		tarDeployFailDeployment := &Deployment{BundleFormat: "tar", BundleContent: []byte("corrupted tar")}
		deployTarFailErr := workerdRunner.Deploy(context.Background(), &Endpoint{Name: "tar-fail-ep"}, tarDeployFailDeployment)
		require.Error(t, deployTarFailErr)

		// 14. ResolveWorkerdBinary via PATH and download
		binaryFilename := workerdBinaryFilename(runtime.GOOS)
		staticBinaryPath := filepath.Join(".layr", "bin", binaryFilename)
		backupBinaryPath := staticBinaryPath + ".backup"
		_ = os.RemoveAll(backupBinaryPath)
		if _, statErr := os.Stat(staticBinaryPath); statErr == nil {
			require.NoError(t, os.Rename(staticBinaryPath, backupBinaryPath))
			defer func() {
				_ = os.RemoveAll(staticBinaryPath)
				_ = os.Rename(backupBinaryPath, staticBinaryPath)
			}()

			// Test PATH lookup
			pathTestDir := t.TempDir()
			pathDummyBinary := filepath.Join(pathTestDir, binaryFilename)
			require.NoError(t, os.WriteFile(pathDummyBinary, []byte("#!/bin/sh\nexit 0\n"), 0755))
			t.Setenv("PATH", pathTestDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			pathResolved, pathResolveErr := resolveWorkerdBinaryForPlatform(context.Background(), runtime.GOOS, runtime.GOARCH)
			require.NoError(t, pathResolveErr)
			require.Equal(t, pathDummyBinary, pathResolved)

			// Test download to staticBinaryPath
			mockGzipDownloadServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
				responseWriter.WriteHeader(http.StatusOK)
				gzipWriter := gzip.NewWriter(responseWriter)
				_, _ = gzipWriter.Write([]byte("#!/bin/sh\nexit 0\n"))
				_ = gzipWriter.Close()
			}))
			defer mockGzipDownloadServer.Close()
			downloadResolved, downloadResolveErr := downloadWorkerdBinaryForPlatform(context.Background(), mockGzipDownloadServer.URL, runtime.GOOS, runtime.GOARCH, staticBinaryPath)
			require.NoError(t, downloadResolveErr)
			require.Equal(t, staticBinaryPath, downloadResolved)
			_ = os.Remove(staticBinaryPath)
		}

		// 15. resolveWorkerdBinaryForPlatform download paths
		previousDownloadFunc := downloadWorkerdBinaryForPlatformFunc
		defer func() {
			downloadWorkerdBinaryForPlatformFunc = previousDownloadFunc
		}()

		downloadWorkerdBinaryForPlatformFunc = func(ctx context.Context, downloadBaseURL, targetOS, targetArch, targetPath string) (string, error) {
			return "", errors.New("simulated download error")
		}
		_, resolveFailErr := resolveWorkerdBinaryForPlatform(context.Background(), "linux", "amd64")
		require.Error(t, resolveFailErr)

		downloadWorkerdBinaryForPlatformFunc = func(ctx context.Context, downloadBaseURL, targetOS, targetArch, targetPath string) (string, error) {
			return "/custom/path/workerd", nil
		}
		resolvedCustomPath, resolveSuccessErr := resolveWorkerdBinaryForPlatform(context.Background(), "linux", "amd64")
		require.NoError(t, resolveSuccessErr)
		require.Equal(t, "/custom/path/workerd", resolvedCustomPath)
		downloadWorkerdBinaryForPlatformFunc = previousDownloadFunc

		// 16. Deploy write error for runner.js
		runnerFailEndpointDir := filepath.Join(WorkerdStorageDir, "functions", "runner-fail-ep")
		require.NoError(t, os.MkdirAll(runnerFailEndpointDir, testDirPermissions))
		runnerFailPath := filepath.Join(runnerFailEndpointDir, "runner.js")
		require.NoError(t, os.MkdirAll(runnerFailPath, testDirPermissions))
		defer func() {
			_ = os.RemoveAll(runnerFailEndpointDir)
		}()
		runnerFailEndpoint := &Endpoint{Name: "runner-fail-ep", Entrypoint: "index.js"}
		runnerFailDeployment := &Deployment{BundleFormat: "raw", BundleContent: []byte("export default {};")}
		deployRunnerFailErr := workerdRunner.Deploy(context.Background(), runnerFailEndpoint, runnerFailDeployment)
		require.Error(t, deployRunnerFailErr)
	})
}
