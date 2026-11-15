package console

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"layr.sh/core"
)

func TestConsoleRouterUnit(t *testing.T) {
	kernel := core.SetupTestKernelWithBrokenDB(t, Migrations)
	service := NewService(kernel)

	// 1. Register routes on routers
	baseRouter := core.NewRouter(fuego.NewServer())
	controlPlaneRouter := core.NewRouter(fuego.NewServer())
	gatewayServeMux := http.NewServeMux()
	service.RegisterRoutes(baseRouter, controlPlaneRouter, gatewayServeMux)
	gatewayServeMux.Handle("/v1/_/", controlPlaneRouter.Mux())
	gatewayServeMux.Handle("/v1/", baseRouter.Mux())

	for prefix, router := range map[string]*core.Router{
		"/v1/console/":   baseRouter,
		"/v1/_/console/": controlPlaneRouter,
	} {
		for extension, contentType := range map[string]string{"json": "application/json", "yaml": "application/yaml"} {
			t.Run(prefix+extension, func(t *testing.T) {
				responseRecorder := httptest.NewRecorder()
				request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, prefix+"spec."+extension, nil)
				gatewayServeMux.ServeHTTP(responseRecorder, request)
				assert.Equal(t, http.StatusOK, responseRecorder.Code)
				assert.Equal(t, contentType, responseRecorder.Header().Get("Content-Type"))
				var expectedPayload []byte
				var err error
				if extension == "json" {
					expectedPayload, err = json.Marshal(router.OutputOpenAPISpec())
					expectedPayload = append(expectedPayload, '\n')
				} else {
					expectedPayload, err = yaml.Marshal(router.OutputOpenAPISpec())
				}
				require.NoError(t, err)
				assert.Equal(t, string(expectedPayload), responseRecorder.Body.String())
				assert.Nil(t, router.OutputOpenAPISpec().Paths.Find(prefix+"spec."+extension))
			})
		}
	}

	// Test nil router handling in registerBaseRoutes
	service.registerBaseRoutes(nil)

	// 2. Verify SDK routes naming compliance (snake_case, operation ID namespacing)
	core.AssertRouterRoutesCompliance(t, controlPlaneRouter)
}

func TestConsoleRedirectUnit(t *testing.T) {
	t.Parallel()

	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)

	t.Run("redirect without query parameters", func(t *testing.T) {
		t.Parallel()
		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/console", nil)

		service.handleConsoleRedirect(responseRecorder, request)

		assert.Equal(t, http.StatusMovedPermanently, responseRecorder.Code)
		assert.Equal(t, "/console/", responseRecorder.Header().Get("Location"))
	})

	t.Run("redirect with query parameters", func(t *testing.T) {
		t.Parallel()
		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/console?filter=active&sort=desc", nil)

		service.handleConsoleRedirect(responseRecorder, request)

		assert.Equal(t, http.StatusMovedPermanently, responseRecorder.Code)
		assert.Equal(t, "/console/?filter=active&sort=desc", responseRecorder.Header().Get("Location"))
	})
}

func TestConsoleResolveUIDistDirectoryUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)

	t.Run("custom environment variable CONSOLE_UI_DIST_DIRECTORY", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("CONSOLE_UI_DIST_DIRECTORY", tempDir)

		resolvedPath := service.resolveConsoleUIDistDirectory()
		assert.Equal(t, tempDir, resolvedPath)
	})

	t.Run("default console ui dist fallback", func(t *testing.T) {
		t.Setenv("CONSOLE_UI_DIST_DIRECTORY", "")

		mkdirErr := os.MkdirAll(defaultConsoleUIDistDirectory, 0755)
		require.NoError(t, mkdirErr)
		t.Cleanup(func() {
			_ = os.RemoveAll(defaultConsoleUIDistDirectory)
		})

		resolvedPath := service.resolveConsoleUIDistDirectory()
		assert.Equal(t, defaultConsoleUIDistDirectory, resolvedPath)
	})

	t.Run("local dist fallback", func(t *testing.T) {
		t.Setenv("CONSOLE_UI_DIST_DIRECTORY", "")
		_ = os.RemoveAll(defaultConsoleUIDistDirectory)

		mkdirErr := os.MkdirAll("dist", 0755)
		require.NoError(t, mkdirErr)
		t.Cleanup(func() {
			_ = os.RemoveAll("dist")
		})

		resolvedPath := service.resolveConsoleUIDistDirectory()
		assert.Equal(t, "dist", resolvedPath)
	})

	t.Run("no dist directory available", func(t *testing.T) {
		t.Setenv("CONSOLE_UI_DIST_DIRECTORY", "")
		_ = os.RemoveAll(defaultConsoleUIDistDirectory)
		_ = os.RemoveAll("dist")

		resolvedPath := service.resolveConsoleUIDistDirectory()
		assert.Empty(t, resolvedPath)
	})
}

func TestConsoleResolveUIDevServerURLUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)

	t.Run("custom dev server url via CONSOLE_UI_DEV_SERVER_URL", func(t *testing.T) {
		t.Setenv("CONSOLE_UI_DEV_SERVER_URL", "http://localhost:4000")

		resolvedURL := service.resolveConsoleUIDevServerURL()
		assert.Equal(t, "http://localhost:4000", resolvedURL)
	})

	t.Run("fallback to default dev server url when no dist", func(t *testing.T) {
		t.Setenv("CONSOLE_UI_DEV_SERVER_URL", "")
		t.Setenv("CONSOLE_UI_DIST_DIRECTORY", "")
		_ = os.RemoveAll(defaultConsoleUIDistDirectory)
		_ = os.RemoveAll("dist")

		resolvedURL := service.resolveConsoleUIDevServerURL()
		assert.Equal(t, defaultConsoleUIDevServerURL, resolvedURL)
	})

	t.Run("returns empty when dist directory exists", func(t *testing.T) {
		t.Setenv("CONSOLE_UI_DEV_SERVER_URL", "")
		tempDir := t.TempDir()
		t.Setenv("CONSOLE_UI_DIST_DIRECTORY", tempDir)

		resolvedURL := service.resolveConsoleUIDevServerURL()
		assert.Empty(t, resolvedURL)
	})
}

func TestConsoleProxyUIDevServerUnit(t *testing.T) {
	t.Parallel()

	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)

	t.Run("invalid dev server URL", func(t *testing.T) {
		t.Parallel()
		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/console/", nil)

		service.proxyToConsoleUIDevServer(responseRecorder, request, "http://[::1]:bad_port")

		assert.Equal(t, http.StatusBadGateway, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "Invalid console dev server URL")
	})

	t.Run("successful reverse proxy upstream", func(t *testing.T) {
		t.Parallel()
		mockUpstreamServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			responseWriter.Header().Set("X-Custom-Header", "proxy-test")
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte("ok from upstream dev server"))
		}))
		t.Cleanup(mockUpstreamServer.Close)

		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/console/dashboard", nil)

		service.proxyToConsoleUIDevServer(responseRecorder, request, mockUpstreamServer.URL)

		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Equal(t, "proxy-test", responseRecorder.Header().Get("X-Custom-Header"))
		assert.Equal(t, "ok from upstream dev server", responseRecorder.Body.String())
	})

	t.Run("offline dev server upstream failure", func(t *testing.T) {
		t.Parallel()
		mockUpstreamServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {}))
		mockUpstreamServerURL := mockUpstreamServer.URL
		mockUpstreamServer.Close() // Immediately close so connections fail

		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/console/dashboard", nil)

		service.proxyToConsoleUIDevServer(responseRecorder, request, mockUpstreamServerURL)

		assert.Equal(t, http.StatusBadGateway, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Header().Get("Content-Type"), "text/html")
		assert.Contains(t, responseRecorder.Body.String(), "Console UI Development Server Not Found")
	})
}

func TestConsoleServeStaticSPAUnit(t *testing.T) {
	t.Parallel()

	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)

	distDir := t.TempDir()
	indexHTMLContent := "<!DOCTYPE html><html><body>Layr Console SPA</body></html>"
	assetJSContent := "console.log('static asset');"

	require.NoError(t, os.WriteFile(filepath.Join(distDir, "index.html"), []byte(indexHTMLContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "app.js"), []byte(assetJSContent), 0644))

	t.Run("serves existing static asset file", func(t *testing.T) {
		t.Parallel()
		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/console/app.js", nil)

		service.serveStaticConsoleSPA(responseRecorder, request, distDir)

		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Equal(t, assetJSContent, responseRecorder.Body.String())
	})

	t.Run("falls back to index.html for SPA route", func(t *testing.T) {
		t.Parallel()
		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/console/users/settings", nil)

		service.serveStaticConsoleSPA(responseRecorder, request, distDir)

		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Equal(t, indexHTMLContent, responseRecorder.Body.String())
	})

	t.Run("returns not found when index.html is missing", func(t *testing.T) {
		t.Parallel()
		emptyDir := t.TempDir()
		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/console/users", nil)

		service.serveStaticConsoleSPA(responseRecorder, request, emptyDir)

		assert.Equal(t, http.StatusNotFound, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "Console bundle not found")
	})
}

func TestConsoleHandleSPAUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)

	t.Run("proxies when dev server is configured", func(t *testing.T) {
		mockUpstreamServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = responseWriter.Write([]byte("spa dev server response"))
		}))
		t.Cleanup(mockUpstreamServer.Close)

		t.Setenv("CONSOLE_UI_DEV_SERVER_URL", mockUpstreamServer.URL)

		handler := service.handleConsoleSPA()
		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/console/", nil)

		handler.ServeHTTP(responseRecorder, request)

		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Equal(t, "spa dev server response", responseRecorder.Body.String())
	})

	t.Run("serves static SPA when dist directory is configured", func(t *testing.T) {
		distDir := t.TempDir()
		indexHTMLContent := "<!DOCTYPE html><html><body>Static SPA Mode</body></html>"
		require.NoError(t, os.WriteFile(filepath.Join(distDir, "index.html"), []byte(indexHTMLContent), 0644))

		t.Setenv("CONSOLE_UI_DEV_SERVER_URL", "")
		t.Setenv("CONSOLE_UI_DIST_DIRECTORY", distDir)

		handler := service.handleConsoleSPA()
		responseRecorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/console/auth/login", nil)

		handler.ServeHTTP(responseRecorder, request)

		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Equal(t, indexHTMLContent, responseRecorder.Body.String())
	})
}
