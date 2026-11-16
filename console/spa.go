package console

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"layr.sh/core"
)

const (
	defaultConsoleUIDevServerURL  = "http://127.0.0.1:3000"
	defaultConsoleUIDistDirectory = "console/dist"
	consoleUIDevServerNoticeHTML  = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Layr Console - Dev Server Not Running</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0a0a0a; color: #ededed; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
    .card { padding: 1.5rem; max-width: 600px; width: 100%; }
    h2 { margin-top: 0; font-size: 1.125rem; }
    p { color: #a1a1a1; font-size: 0.9rem; line-height: 1.5; }
    code { background: #262626; padding: 0.2rem 0.4rem; border-radius: 0px; font-size: 0.75rem; }
    pre { background: #171717; padding: 0.75rem; border: 1px solid #262626; border-radius: 0px; overflow-x: auto; color: #fff; font-family: monospace; font-size: 0.75rem; }
  </style>
</head>
<body>
  <div class="card">
    <h2>Console UI Development Server Not Found</h2>
    <p>Layr is proxying <code>/console</code> to the development server at <code>http://127.0.0.1:3000</code>, but no connection could be established.</p>
    <p>To start the Console UI development server, run:</p>
    <pre>just dev-console-ui</pre>
    <p>Or start both the backend and frontend together with:</p>
    <pre style="margin-bottom: 0px;">just dev</pre>
  </div>
</body>
</html>`
)

func (service *Service) registerBaseRoutes(router *core.Router) {
	if router == nil || router.Mux() == nil {
		return
	}
	router.Mux().HandleFunc("GET /console", service.handleConsoleRedirect)
	router.Mux().Handle("/console/", service.handleConsoleSPA())
}

func (service *Service) handleConsoleRedirect(responseWriter http.ResponseWriter, request *http.Request) {
	redirectTarget := "/console/"
	if request.URL.RawQuery != "" {
		redirectTarget += "?" + request.URL.RawQuery
	}
	http.Redirect(responseWriter, request, redirectTarget, http.StatusMovedPermanently)
}

func (service *Service) resolveConsoleUIDistDirectory() string {
	if customDir := os.Getenv("CONSOLE_UI_DIST_DIRECTORY"); customDir != "" {
		return customDir
	}
	if _, statErr := os.Stat(defaultConsoleUIDistDirectory); statErr == nil {
		return defaultConsoleUIDistDirectory
	}
	if _, statErr := os.Stat("dist"); statErr == nil {
		return "dist"
	}
	return ""
}

func (service *Service) resolveConsoleUIDevServerURL() string {
	if customURL := os.Getenv("CONSOLE_UI_DEV_SERVER_URL"); customURL != "" {
		return customURL
	}

	if distDirectory := service.resolveConsoleUIDistDirectory(); distDirectory != "" {
		return ""
	}
	return defaultConsoleUIDevServerURL
}

func (service *Service) handleConsoleSPA() http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		devServerURL := service.resolveConsoleUIDevServerURL()
		if devServerURL != "" {
			service.proxyToConsoleUIDevServer(responseWriter, request, devServerURL)
			return
		}
		service.serveStaticConsoleSPA(responseWriter, request, service.resolveConsoleUIDistDirectory())
	})
}

func (service *Service) proxyToConsoleUIDevServer(responseWriter http.ResponseWriter, request *http.Request, devServerURL string) {
	targetURL, parseErr := url.Parse(devServerURL)
	if parseErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadGateway, "Invalid console dev server URL")
		return
	}

	reverseProxy := &httputil.ReverseProxy{
		Rewrite: func(proxyRequest *httputil.ProxyRequest) {
			proxyRequest.SetURL(targetURL)
			proxyRequest.Out.Host = targetURL.Host
		},
		ErrorHandler: func(proxyResponseWriter http.ResponseWriter, proxyRequest *http.Request, proxyErr error) {
			log.Warnf("console ui dev server proxy error: %v", proxyErr)
			proxyResponseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
			proxyResponseWriter.WriteHeader(http.StatusBadGateway)
			_, _ = proxyResponseWriter.Write([]byte(consoleUIDevServerNoticeHTML))
		},
	}

	reverseProxy.ServeHTTP(responseWriter, request)
}

func (service *Service) serveStaticConsoleSPA(responseWriter http.ResponseWriter, request *http.Request, distDirectory string) {
	cleanedRelativePath := strings.TrimPrefix(request.URL.Path, "/console")
	cleanedRelativePath = strings.TrimPrefix(cleanedRelativePath, "/")

	targetFilePath := filepath.Join(distDirectory, filepath.Clean(cleanedRelativePath))
	fileInfo, statErr := os.Stat(targetFilePath)
	if statErr == nil && !fileInfo.IsDir() {
		http.ServeFile(responseWriter, request, targetFilePath)
		return
	}

	indexFilePath := filepath.Join(distDirectory, "index.html")
	if _, indexErr := os.Stat(indexFilePath); indexErr == nil {
		http.ServeFile(responseWriter, request, indexFilePath)
		return
	}

	core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Console bundle not found. Please build with 'just build-console'")
}
