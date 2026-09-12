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
	"uuid"

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

// Server coordinates the Layr HTTP gateway, probes, and API dispatch.
type Server struct {
	db                 *DatabasePool
	cryptoKeyManager   *CryptoKeyManager
	baseRouter         *Router
	controlPlaneRouter *Router
	serveMux           *http.ServeMux
	server             *http.Server
	uptime             time.Time     //nolint:namingclarity
	requestCount       atomic.Uint64 //nolint:namingclarity
}

// NewServer initializes the HTTP gateway with type-safe OpenAPI route controllers.
func NewServer(db *DatabasePool, cryptoKeyManager *CryptoKeyManager) *Server {
	serveMux := http.NewServeMux()
	config := GetConfig()

	baseRouter := NewRouter(fuego.NewServer(
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

	log.Debugf("initializing Server gateway")
	server := &Server{
		db:                 db,
		cryptoKeyManager:   cryptoKeyManager,
		baseRouter:         baseRouter,
		controlPlaneRouter: controlPlaneRouter,
		serveMux:           serveMux,
		uptime:             time.Now(),
	}

	// Register Core Routes on Public Router
	GetRoute[HealthResponse](baseRouter, "/healthz", server.handleHealthzRequest,
		RouteTag("Probes"),
		RouteSummary("Liveness probe"),
		RouteDescription("Returns 200 OK if the Layr gateway process is running and responsive."),
		RouteOperationID("core__healthz"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("healthz"),
	)
	GetRoute[ReadyResponse](baseRouter, "/readyz", server.handleReadyzRequest,
		RouteTag("Probes"),
		RouteSummary("Readiness probe"),
		RouteDescription("Returns 200 OK if PostgreSQL connection db is healthy and accepting queries; returns 503 Service Unavailable if unready."),
		RouteOperationID("core__readyz"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("readyz"),
	)
	GetRoute[string](baseRouter, "/metrics", server.handleMetricsRequest,
		RouteTag("Observability"),
		RouteSummary("Prometheus metrics exposition"),
		RouteDescription("Prometheus text exposition format (version 0.0.4) exposing process uptime, HTTP requests handled, and allocated heap memory."),
		RouteOperationID("core__metrics"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("metrics"),
	)
	GetRoute[TopologyResponse](baseRouter, "/api/v1/topology", server.handleTopologyRequest,
		RouteTag("Discovery"),
		RouteSummary("Get dynamic cluster topology and enabled services"),
		RouteDescription("Returns dynamic cluster topology, enabled service flags, project metadata, and publishable key for SDK initialization."),
		RouteOperationID("core__topology"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("topology"),
	)

	// Mount Public Probe Routes directly on root mux (No publishable key required)
	serveMux.Handle("/healthz", baseRouter.Mux())
	serveMux.Handle("/readyz", baseRouter.Mux())
	serveMux.Handle("/metrics", baseRouter.Mux())

	// OIDC & OAuth Discovery (Open access)
	serveMux.Handle("/.well-known/", baseRouter.Mux())

	// Lightweight Analytics Tracking Script (Open access)
	serveMux.Handle("/analytics-script.js", baseRouter.Mux())

	// Embedded Console SPA (Open access)
	serveMux.Handle("/console", baseRouter.Mux())
	serveMux.Handle("/console/", baseRouter.Mux())

	// Public OpenAPI 3.1 Spec (Unrestricted)
	serveMux.HandleFunc("/api/v1/spec.json", server.handleBaseSpecJSONRequest)
	serveMux.HandleFunc("/api/v1/spec.yaml", server.handleBaseSpecYAMLRequest)

	// Control Plane OpenAPI 3.1 Spec (Unrestricted)
	serveMux.HandleFunc("/api/v1/_/spec.json", server.handleControlPlaneSpecJSONRequest)
	serveMux.HandleFunc("/api/v1/_/spec.yaml", server.handleControlPlaneSpecYAMLRequest)

	// Mount Control Plane API Router
	serveMux.Handle("/api/v1/_/", controlPlaneRouter.Mux())

	// Mount Public API Router with Publishable Key Gate
	serveMux.Handle("/api/v1/", server.PublishableKeyMiddleware(baseRouter.Mux()))

	server.server = &http.Server{
		Addr:              config.Server.ListenAddr,
		Handler:           server.middleware(serveMux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return server
}

// Mux returns the underlying HTTP ServeMux for route registration.
func (server *Server) Mux() *http.ServeMux {
	return server.serveMux
}

// BaseRouter returns the base OpenAPI server router.
func (server *Server) BaseRouter() *Router {
	return server.baseRouter
}

// ControlPlaneRouter returns the control plane OpenAPI server router.
func (server *Server) ControlPlaneRouter() *Router {
	return server.controlPlaneRouter
}

// Start boots the HTTP server in background.
func (server *Server) Start() error {
	log.Debugf("starting HTTP server on %s", server.server.Addr)
	if err := server.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server failed to listen and serve: %w", err)
	}
	return nil
}

// Shutdown initiates a graceful drain.
func (server *Server) Shutdown(ctx context.Context) error {
	log.Debugf("shutting down HTTP server")
	if err := server.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("http server failed to shut down cleanly: %w", err)
	}
	log.Tracef("HTTP server shutdown complete")
	return nil
}

func (server *Server) middleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Errorf("panic recovered in HTTP handler: %v", recovered)
				WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "internal server error", "LAYR_CORE_500")
			}
		}()
		log.Tracef("incoming HTTP request %s %s", request.Method, request.URL.Path)
		server.requestCount.Add(1)

		clientIP := ExtractRequestClientIP(request)
		userAgent := request.UserAgent()
		requestID := request.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewV7().String()
		}

		ctx := WithEventContext(request.Context(), EventContext{
			IPAddress: &clientIP,
			UserAgent: &userAgent,
			RequestID: &requestID,
		})
		handler.ServeHTTP(responseWriter, request.WithContext(ctx))
	})
}

// /healthz - Liveness probe
func (server *Server) handleHealthzRequest(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(HealthResponse{
		Status:        "healthy",
		UptimeSeconds: time.Since(server.uptime).Seconds(),
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	})
}

// /readyz - Readiness probe
func (server *Server) handleReadyzRequest(responseWriter http.ResponseWriter, request *http.Request) {
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
func (server *Server) handleMetricsRequest(responseWriter http.ResponseWriter, request *http.Request) {
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
func (server *Server) PublishableKeyMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		path := request.URL.Path
		log.Tracef("evaluating publishable key middleware for path %s", path)
		if path == "/api/v1/topology" ||
			strings.HasPrefix(path, "/api/v1/_/") ||
			path == "/api/v1/_" ||
			path == "/api/v1/spec.json" ||
			path == "/api/v1/spec.yaml" ||
			strings.HasPrefix(path, "/.well-known/") {
			handler.ServeHTTP(responseWriter, request)
			return
		}

		if server.cryptoKeyManager != nil {
			publishableKey := request.Header.Get("X-Layr-Client-Publishable-Key")
			serviceAccountKey := request.Header.Get("X-Layr-Service-Account-Key")
			authHeader := request.Header.Get("Authorization")

			isValid := server.cryptoKeyManager.VerifyPublishableKey(publishableKey)
			if !isValid {
				hasValidServiceKey := strings.HasPrefix(serviceAccountKey, "sec_")
				hasValidAuthHeader := strings.HasPrefix(authHeader, "Bearer ") || strings.HasPrefix(authHeader, "Basic ")
				if hasValidServiceKey || hasValidAuthHeader {
					isValid = true
				}
			}

			if !isValid {
				WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "invalid or missing publishable key", "LAYR_CORE_006")
				return
			}
		}

		handler.ServeHTTP(responseWriter, request)
	})
}

// /api/v1/topology - Dynamic cluster & topology discovery
func (server *Server) handleTopologyRequest(responseWriter http.ResponseWriter, request *http.Request) {
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
func (server *Server) handleBaseSpecJSONRequest(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	openAPISpec := server.baseRouter.OutputOpenAPISpec()
	_ = json.NewEncoder(responseWriter).Encode(openAPISpec)
}

// /api/v1/spec.yaml - Client OpenAPI 3.1 YAML
func (server *Server) handleBaseSpecYAMLRequest(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/yaml")
	responseWriter.WriteHeader(http.StatusOK)
	openAPISpec := server.baseRouter.OutputOpenAPISpec()
	_ = yaml.NewEncoder(responseWriter).Encode(openAPISpec)
}

// /api/v1/_/spec.json - Protected Control Plane OpenAPI 3.1 JSON
func (server *Server) handleControlPlaneSpecJSONRequest(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	openAPISpec := server.controlPlaneRouter.OutputOpenAPISpec()
	_ = json.NewEncoder(responseWriter).Encode(openAPISpec)
}

// /api/v1/_/spec.yaml - Protected Control Plane OpenAPI 3.1 YAML
func (server *Server) handleControlPlaneSpecYAMLRequest(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/yaml")
	responseWriter.WriteHeader(http.StatusOK)
	openAPISpec := server.controlPlaneRouter.OutputOpenAPISpec()
	_ = yaml.NewEncoder(responseWriter).Encode(openAPISpec)
}
