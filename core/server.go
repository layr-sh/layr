package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"
	"gopkg.in/yaml.v3"
)

// HealthResponse represents the /healthz probe response model.
type HealthResponse struct {
	Status        string  `json:"status"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	Timestamp     string  `json:"timestamp"`
}

// ReadyResponse represents the /readyz probe response model.
type ReadyResponse struct {
	Status          string   `json:"status"`
	Database        string   `json:"database"`
	EnabledServices []string `json:"enabled_services,omitempty"`
}

// TopologyProjectInfo represents project metadata in topology discovery.
type TopologyProjectInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// TopologyServerInfo represents host and port in topology discovery.
type TopologyServerInfo struct {
	ListenAddr string `json:"listen_addr"`
	BaseURL    string `json:"base_url"`
}

// TopologyResponse represents the dynamic topology discovery model.
type TopologyResponse struct {
	Project         TopologyProjectInfo `json:"project"`
	EnabledServices []string            `json:"enabled_services"`
	Server          TopologyServerInfo  `json:"server"`
	PublishableKey  string              `json:"publishable_key,omitempty"`
}

// HTTPServer coordinates the Layr HTTP gateway, probes, and API dispatch.
type HTTPServer struct {
	db                 *DatabasePool
	cryptoKeyManager   *CryptoKeyManager
	router             *Router
	controlPlaneRouter *Router
	mux                *http.ServeMux
	server             *http.Server
	uptime             time.Time
	requestCount       atomic.Uint64
}

// NewServer initializes the HTTP gateway with type-safe OpenAPI route controllers.
func NewServer(db *DatabasePool, cryptoKeyManager *CryptoKeyManager) *HTTPServer {
	mux := http.NewServeMux()
	config := GetConfig()

	router := NewRouter(fuego.NewServer(
		fuego.WithEngineOptions(
			fuego.WithOpenAPIConfig(fuego.OpenAPIConfig{
				DisableLocalSave:     true,
				DisableSwaggerUI:     true,
				DisableDefaultServer: true,
				Info: &openapi3.Info{
					Title:       "Layr Client API Engine",
					Version:     "1.0.0",
					Description: "Public Application & Client APIs for Layr Platform",
				},
			}),
		),
	))

	controlPlaneRouter := NewRouter(fuego.NewServer(
		fuego.WithEngineOptions(
			fuego.WithOpenAPIConfig(fuego.OpenAPIConfig{
				DisableLocalSave:     true,
				DisableSwaggerUI:     true,
				DisableDefaultServer: true,
				Info: &openapi3.Info{
					Title:       "Layr Control Plane API Engine",
					Version:     "1.0.0",
					Description: "Protected Control Plane APIs for Layr Platform",
				},
			}),
		),
	))

	log.Debugf("initializing HTTPServer gateway")
	server := &HTTPServer{
		db:                 db,
		cryptoKeyManager:   cryptoKeyManager,
		router:             router,
		controlPlaneRouter: controlPlaneRouter,
		mux:                mux,
		uptime:             time.Now(),
	}

	// Register Core Routes on Public Router
	GetRoute[HealthResponse](router, "/healthz", server.handleHealthzRequest,
		RouteTag("Probes"),
		RouteSummary("Liveness probe"),
		RouteDescription("Returns 200 OK if the Layr gateway process is running and responsive."),
		RouteOperationID("core__healthz"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("healthz"),
	)
	GetRoute[ReadyResponse](router, "/readyz", server.handleReadyzRequest,
		RouteTag("Probes"),
		RouteSummary("Readiness probe"),
		RouteDescription("Returns 200 OK if PostgreSQL connection db is healthy and accepting queries; returns 503 Service Unavailable if unready."),
		RouteOperationID("core__readyz"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("readyz"),
	)
	GetRoute[string](router, "/metrics", server.handleMetricsRequest,
		RouteTag("Observability"),
		RouteSummary("Prometheus metrics exposition"),
		RouteDescription("Prometheus text exposition format (version 0.0.4) exposing process uptime, HTTP requests handled, and allocated heap memory."),
		RouteOperationID("core__metrics"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("metrics"),
	)
	GetRoute[TopologyResponse](router, "/api/v1/topology", server.handleTopologyRequest,
		RouteTag("Discovery"),
		RouteSummary("Get dynamic cluster topology and enabled services"),
		RouteDescription("Returns dynamic cluster topology, enabled service flags, project metadata, and publishable key for SDK initialization."),
		RouteOperationID("core__topology"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("topology"),
	)

	// Mount Public Probe Routes directly on root mux (No publishable key required)
	mux.Handle("/healthz", router.Mux())
	mux.Handle("/readyz", router.Mux())
	mux.Handle("/metrics", router.Mux())

	// OIDC & OAuth Discovery (Open access)
	mux.Handle("/.well-known/", router.Mux())

	// Lightweight Analytics Tracking Script (Open access)
	mux.Handle("/analytics-script.js", router.Mux())

	// Embedded Console SPA (Open access)
	mux.Handle("/console", router.Mux())
	mux.Handle("/console/", router.Mux())

	// Public OpenAPI 3.1 Spec (Unrestricted)
	mux.HandleFunc("/api/v1/spec.json", server.handleSpecJSONRequest)
	mux.HandleFunc("/api/v1/spec.yaml", server.handleSpecYAMLRequest)

	// Control Plane OpenAPI 3.1 Spec (Unrestricted)
	mux.HandleFunc("/api/v1/_/spec.json", server.handleControlPlaneSpecJSONRequest)
	mux.HandleFunc("/api/v1/_/spec.yaml", server.handleControlPlaneSpecYAMLRequest)

	// Mount Control Plane API Router
	mux.Handle("/api/v1/_/", controlPlaneRouter.Mux())

	// Mount Public API Router with Publishable Key Gate
	mux.Handle("/api/v1/", server.PublishableKeyMiddleware(router.Mux()))

	server.server = &http.Server{
		Addr:              config.Server.ListenAddr,
		Handler:           server.middleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return server
}

// Mux returns the underlying HTTP ServeMux for route registration.
func (server *HTTPServer) Mux() *http.ServeMux {
	return server.mux
}

// Router returns the public OpenAPI server router.
func (server *HTTPServer) Router() *Router {
	return server.router
}

// ControlPlaneRouter returns the control plane OpenAPI server router.
func (server *HTTPServer) ControlPlaneRouter() *Router {
	return server.controlPlaneRouter
}

// Start boots the HTTP server in background.
func (server *HTTPServer) Start() error {
	log.Debugf("starting HTTP server on %s", server.server.Addr)
	if err := server.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server failed to listen and serve: %w", err)
	}
	return nil
}

// Shutdown initiates a graceful drain.
func (server *HTTPServer) Shutdown(ctx context.Context) error {
	log.Debugf("shutting down HTTP server")
	if err := server.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("http server failed to shut down cleanly: %w", err)
	}
	log.Tracef("HTTP server shutdown complete")
	return nil
}

func (server *HTTPServer) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		log.Tracef("incoming HTTP request %s %s", request.Method, request.URL.Path)
		server.requestCount.Add(1)
		// responseWriter.Header().Set("Layr-Version", "TODO")
		next.ServeHTTP(responseWriter, request)
	})
}

// /healthz - Liveness probe
func (server *HTTPServer) handleHealthzRequest(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(HealthResponse{
		Status:        "healthy",
		UptimeSeconds: time.Since(server.uptime).Seconds(),
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	})
}

// /readyz - Readiness probe
func (server *HTTPServer) handleReadyzRequest(responseWriter http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()

	databaseStatus := "ok"
	statusCode := http.StatusOK
	if server.db != nil {
		if err := server.db.Ping(ctx); err != nil {
			databaseStatus = fmt.Sprintf("error: %v", err)
			statusCode = http.StatusServiceUnavailable
		}
	}

	statusText := "ready"
	if statusCode != http.StatusOK {
		statusText = "unready"
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(ReadyResponse{
		Status:          statusText,
		Database:        databaseStatus,
		EnabledServices: GetConfig().GetEnabledServices(),
	})
}

// /metrics - Minimal Prometheus exposition
func (server *HTTPServer) handleMetricsRequest(responseWriter http.ResponseWriter, request *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	responseWriter.Header().Set("Content-Type", "text/plain; version=0.0.4")
	responseWriter.WriteHeader(http.StatusOK)
	output := fmt.Sprintf("# HELP uptime_seconds Process uptime\n# TYPE uptime_seconds gauge\nuptime_seconds %f\n# HELP http_requests_total Total HTTP requests handled\n# TYPE http_requests_total counter\nhttp_requests_total %d\n# HELP memory_alloc_bytes Allocated heap bytes\n# TYPE memory_alloc_bytes gauge\nmemory_alloc_bytes %d\n",
		time.Since(server.uptime).Seconds(),
		server.requestCount.Load(),
		memStats.Alloc,
	)
	_, _ = responseWriter.Write([]byte(output))
}

// PublishableKeyMiddleware validates X-Layr-Client-Publishable-Key (or Service Account fallback) on public /api/v1/* routes.
func (server *HTTPServer) PublishableKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		path := request.URL.Path
		log.Tracef("evaluating publishable key middleware for path %s", path)
		if path == "/api/v1/topology" ||
			strings.HasPrefix(path, "/api/v1/_/") ||
			path == "/api/v1/_" ||
			path == "/api/v1/spec.json" ||
			path == "/api/v1/spec.yaml" ||
			strings.HasPrefix(path, "/.well-known/") {
			next.ServeHTTP(responseWriter, request)
			return
		}

		if server.cryptoKeyManager != nil {
			publishableKey := request.Header.Get("X-Layr-Client-Publishable-Key")
			serviceAccountKey := request.Header.Get("X-Layr-Service-Account-Key")
			authHeader := request.Header.Get("Authorization")

			isValid := server.cryptoKeyManager.VerifyPublishableKey(publishableKey)
			if !isValid && (serviceAccountKey != "" || authHeader != "") {
				isValid = true
			}

			if !isValid {
				WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "invalid or missing publishable key", "LAYR_CORE_006")
				return
			}
		}

		next.ServeHTTP(responseWriter, request)
	})
}

// /api/v1/topology - Dynamic cluster & topology discovery
func (server *HTTPServer) handleTopologyRequest(responseWriter http.ResponseWriter, request *http.Request) {
	publishableKey := ""
	if server.cryptoKeyManager != nil {
		publishableKey = server.cryptoKeyManager.DerivePublishableKey()
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	config := GetConfig()
	_ = json.NewEncoder(responseWriter).Encode(TopologyResponse{
		Project: TopologyProjectInfo{
			Name:        config.Project.Name,
			Description: config.Project.Description,
		},
		EnabledServices: config.GetEnabledServices(),
		Server: TopologyServerInfo{
			ListenAddr: config.Server.ListenAddr,
			BaseURL:    config.Server.BaseURL,
		},
		PublishableKey: publishableKey,
	})
}

// /api/v1/spec.json - Client OpenAPI 3.1 JSON
func (server *HTTPServer) handleSpecJSONRequest(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	spec := server.router.OutputOpenAPISpec()
	_ = json.NewEncoder(responseWriter).Encode(spec)
}

// /api/v1/spec.yaml - Client OpenAPI 3.1 YAML
func (server *HTTPServer) handleSpecYAMLRequest(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/yaml")
	responseWriter.WriteHeader(http.StatusOK)
	spec := server.router.OutputOpenAPISpec()
	_ = yaml.NewEncoder(responseWriter).Encode(spec)
}

// /api/v1/_/spec.json - Protected Control Plane OpenAPI 3.1 JSON
func (server *HTTPServer) handleControlPlaneSpecJSONRequest(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	spec := server.controlPlaneRouter.OutputOpenAPISpec()
	_ = json.NewEncoder(responseWriter).Encode(spec)
}

// /api/v1/_/spec.yaml - Protected Control Plane OpenAPI 3.1 YAML
func (server *HTTPServer) handleControlPlaneSpecYAMLRequest(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/yaml")
	responseWriter.WriteHeader(http.StatusOK)
	spec := server.controlPlaneRouter.OutputOpenAPISpec()
	_ = yaml.NewEncoder(responseWriter).Encode(spec)
}
