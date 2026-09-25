package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"
)

// HostHandlerFunc defines a function that handles an incoming HTTP request based on its host or domain.
// It returns true if the request was handled, or false if it should fall through to standard route dispatch.
type HostHandlerFunc func(responseWriter http.ResponseWriter, request *http.Request) bool

// Server coordinates the Layr HTTP gateway, probes, and API dispatch.
type Server struct {
	kernel             *Kernel
	baseRouter         *Router
	controlPlaneRouter *Router
	serveMux           *http.ServeMux
	server             *http.Server
	hostHandlers       []HostHandlerFunc
	uptime             time.Time     //nolint:namingclarity
	requestCount       atomic.Uint64 //nolint:namingclarity
}

// NewServer initializes the HTTP gateway with type-safe OpenAPI route controllers.
func NewServer(kernel *Kernel) *Server {
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
		kernel:             kernel,
		baseRouter:         baseRouter,
		controlPlaneRouter: controlPlaneRouter,
		serveMux:           serveMux,
		uptime:             time.Now(),
	}

	server.registerBaseRoutes()
	server.registerControlPlaneRoutes()

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
	serveMux.HandleFunc("/v1/spec.json", server.handleGetBaseSpecJSON)
	serveMux.HandleFunc("/v1/spec.yaml", server.handleGetBaseSpecYAML)

	// Control Plane OpenAPI 3.1 Spec (Unrestricted)
	serveMux.HandleFunc("/v1/_/spec.json", server.handleGetControlPlaneSpecJSON)
	serveMux.HandleFunc("/v1/_/spec.yaml", server.handleGetControlPlaneSpecYAML)

	// Mount Control Plane API Router
	serveMux.Handle("/v1/_/", controlPlaneRouter.Mux())

	// Mount Public API Router with Publishable Key Gate
	serveMux.Handle("/v1/", server.PublishableKeyMiddleware(baseRouter.Mux()))

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

// RegisterHostHandler registers a host-based request interceptor on the HTTP server.
func (server *Server) RegisterHostHandler(hostHandlerFunc HostHandlerFunc) {
	if server != nil && hostHandlerFunc != nil {
		server.hostHandlers = append(server.hostHandlers, hostHandlerFunc)
	}
}

// HostHandlers returns all registered host-based request interceptors on the HTTP server.
func (server *Server) HostHandlers() []HostHandlerFunc {
	if server == nil {
		return nil
	}
	return server.hostHandlers
}

// Mux returns the underlying HTTP ServeMux for route registration.
func (server *Server) Mux() *http.ServeMux {
	return server.serveMux
}

// Handler returns the root HTTP handler wrapped with core middleware (panic recovery, correlation ID, metrics, authentication).
func (server *Server) Handler() http.Handler {
	return server.server.Handler
}

// BaseRouter returns the base OpenAPI server router.
func (server *Server) BaseRouter() *Router {
	return server.baseRouter
}

// ControlPlaneRouter returns the control plane OpenAPI server router.
func (server *Server) ControlPlaneRouter() *Router {
	return server.controlPlaneRouter
}

// Kernel returns the parent kernel runtime.
func (server *Server) Kernel() *Kernel {
	return server.kernel
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
				WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("panic recovered in HTTP handler: %v", recovered))
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

		var authenticated bool

		// 1. JWT Request Verification (M2M JWT or User Access Token)
		token := ExtractRequestSessionToken(request)
		if token != "" {
			if jwtClaims, err := server.kernel.jwtSigner.VerifyAccessToken(token); err == nil && jwtClaims != nil && jwtClaims.Subject != "" {
				slugifier := NewSlugifier()
				handle := slugifier.Slugify(GetConfig().Project.Name)
				if handle == "" {
					handle = "layr"
				}
				layrAudience := handle + ":service_account"

				if jwtClaims.Role == "service_role" || strings.HasSuffix(jwtClaims.Audience, ":service_account") {
					if jwtClaims.Audience != layrAudience {
						log.Debugf("m2m token rejected: audience mismatch %q != expected %q", jwtClaims.Audience, layrAudience)
					} else {
						serviceAccountUUID, _ := uuid.Parse(jwtClaims.Subject)
						role := "service_role"
						jwtClaims.Role = role
						ctx = WithAuthContext(ctx, AuthContext{
							ServiceAccountID: jwtClaims.Subject,
							JWT:              *jwtClaims,
						})
						ctx = WithEventActor(ctx, EventActor{
							Type: "service_account",
							ID:   &serviceAccountUUID,
							Role: &role,
						})
						authenticated = true
					}
				} else {
					var currentRefreshTokenHash string
					if cookie, err := request.Cookie(SessionCookieNameSecure); err == nil && cookie.Value != "" {
						currentRefreshTokenHash = server.kernel.jwtSigner.HashRefreshToken(cookie.Value)
					} else if cookie, err := request.Cookie(SessionCookieNameInsecure); err == nil && cookie.Value != "" {
						currentRefreshTokenHash = server.kernel.jwtSigner.HashRefreshToken(cookie.Value)
					}
					if currentRefreshTokenHash == "" {
						if refreshTokenHeader := request.Header.Get("X-Refresh-Token"); refreshTokenHeader != "" {
							currentRefreshTokenHash = server.kernel.jwtSigner.HashRefreshToken(refreshTokenHeader)
						}
					}

					ctx = WithAuthContext(ctx, AuthContext{
						UserID:           jwtClaims.Subject,
						JWT:              *jwtClaims,
						RefreshTokenHash: currentRefreshTokenHash,
					})

					if parsedUserUUID, parseErr := uuid.Parse(jwtClaims.Subject); parseErr == nil {
						role := jwtClaims.Role
						ctx = WithEventActor(ctx, EventActor{
							Type: "user",
							ID:   &parsedUserUUID,
							Role: &role,
						})
					}
					authenticated = true
				}
			}
		}

		// 2. M2M / Service Account Request Verification
		if !authenticated {
			serviceAccountKey := ExtractRequestServiceAccountKey(request)
			if serviceAccountKey != "" {
				if serviceAccount, err := server.kernel.serviceAccountManager.Authenticate(ctx, serviceAccountKey, clientIP); err == nil && serviceAccount != nil {
					serviceAccountUUID, _ := uuid.Parse(serviceAccount.ID)
					role := "service_role"
					ctx = WithAuthContext(ctx, AuthContext{
						ServiceAccountID: serviceAccount.ID,
						JWT: JWTClaims{
							Subject: serviceAccount.ID,
							Role:    role,
							Scope:   strings.Join(serviceAccount.Scopes, " "),
						},
					})
					ctx = WithEventActor(ctx, EventActor{
						Type: "service_account",
						ID:   &serviceAccountUUID,
						Role: &role,
					})
					authenticated = true
				}
			}
		}

		// 3. Database Session Lookup (Cookie or X-Refresh-Token fallback)
		if !authenticated {
			var sessionRefreshToken string
			if cookie, err := request.Cookie(SessionCookieNameSecure); err == nil && cookie.Value != "" {
				sessionRefreshToken = cookie.Value
			} else if cookie, err := request.Cookie(SessionCookieNameInsecure); err == nil && cookie.Value != "" {
				sessionRefreshToken = cookie.Value
			}
			if sessionRefreshToken == "" {
				if tokenHeader := request.Header.Get("X-Refresh-Token"); tokenHeader != "" {
					sessionRefreshToken = tokenHeader
				}
			}

			if sessionRefreshToken != "" {
				refreshTokenHash := server.kernel.jwtSigner.HashRefreshToken(sessionRefreshToken)
				var sessionID, sessionUserID, email, phone, role string
				var isAnonymous bool
				queryErr := server.kernel.DB().QueryRow(ctx, `
					SELECT s.id, s.user_id, COALESCE(u.email, ''), COALESCE(u.phone, ''), COALESCE(u.role, 'authenticated'), COALESCE(u.is_anonymous, false)
					FROM auth.sessions s
					JOIN auth.users u ON u.id = s.user_id
					WHERE s.refresh_token_hash = $1 AND s.expires_at > clock_timestamp()
				`, refreshTokenHash).Scan(&sessionID, &sessionUserID, &email, &phone, &role, &isAnonymous)
				if queryErr == nil && sessionUserID != "" {
					jwtClaims := JWTClaims{
						Subject:     sessionUserID,
						SessionID:   sessionID,
						Email:       email,
						Phone:       phone,
						Role:        role,
						IsAnonymous: isAnonymous,
					}
					ctx = WithAuthContext(ctx, AuthContext{
						UserID:           sessionUserID,
						JWT:              jwtClaims,
						RefreshTokenHash: refreshTokenHash,
					})
					if parsedUserUUID, parseErr := uuid.Parse(sessionUserID); parseErr == nil {
						ctx = WithEventActor(ctx, EventActor{
							Type: "user",
							ID:   &parsedUserUUID,
							Role: &role,
						})
					}
				}
			}
		}

		for _, hostHandler := range server.hostHandlers {
			if hostHandler(responseWriter, request.WithContext(ctx)) {
				return
			}
		}

		handler.ServeHTTP(responseWriter, request.WithContext(ctx))
	})
}

// PublishableKeyMiddleware validates X-Layr-Client-Publishable-Key (or Service Account fallback) on public /v1/* routes.
func (server *Server) PublishableKeyMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		path := request.URL.Path
		log.Tracef("evaluating publishable key middleware for path %s", path)
		if path == "/v1/manifest" ||
			strings.HasPrefix(path, "/v1/_/") ||
			path == "/v1/_" ||
			path == "/v1/spec.json" ||
			path == "/v1/spec.yaml" ||
			strings.HasPrefix(path, "/.well-known/") {
			handler.ServeHTTP(responseWriter, request)
			return
		}

		publishableKey := request.Header.Get("X-Layr-Client-Publishable-Key")
		isValid := server.kernel.cryptoKeyManager.VerifyPublishableKey(publishableKey)
		if !isValid {
			authContext := GetAuthContext(request.Context())
			if authContext.IsAuthenticated() {
				isValid = true
			}
		}

		if !isValid {
			WriteErrorResponse(responseWriter, request, http.StatusUnauthorized, "invalid or missing publishable key")
			return
		}

		handler.ServeHTTP(responseWriter, request)
	})
}

func (server *Server) registerBaseRoutes() {
	baseRouter := server.baseRouter

	GetRoute[GetHealthResponse](baseRouter, "/healthz", server.handleGetHealth,
		RouteTag("Probes"),
		RouteSummary("Liveness probe"),
		RouteDescription("Returns 200 OK if the Layr gateway process is running and responsive."),
		RouteOperationID("core__healthz"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("healthz"),
	)
	GetRoute[GetReadinessResponse](baseRouter, "/readyz", server.handleGetReadiness,
		RouteTag("Probes"),
		RouteSummary("Readiness probe"),
		RouteDescription("Returns 200 OK if PostgreSQL connection db is healthy and accepting queries; returns 503 Service Unavailable if unready."),
		RouteOperationID("core__readyz"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("readyz"),
	)
	GetRoute[string](baseRouter, "/metrics", server.handleGetMetrics,
		RouteTag("Observability"),
		RouteSummary("Prometheus metrics exposition"),
		RouteDescription("Prometheus text exposition format (version 0.0.4) exposing process uptime, HTTP requests handled, and allocated heap memory."),
		RouteOperationID("core__metrics"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("metrics"),
	)
	GetRoute[GetManifestResponse](baseRouter, "/v1/manifest", server.handleGetManifest,
		RouteTag("Discovery"),
		RouteSummary("Get dynamic cluster manifest and enabled services"),
		RouteDescription("Returns dynamic cluster manifest, enabled service flags, project metadata, and publishable key for SDK initialization."),
		RouteOperationID("core__manifest"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("manifest"),
	)
}

func (server *Server) registerControlPlaneRoutes() {
	controlPlaneRouter := server.controlPlaneRouter

	// Register Core Routes on Control Plane Router
	GetRoute[ListServiceAccountsResponse](controlPlaneRouter, "/v1/_/core/service-accounts", server.handleListServiceAccounts,
		RouteTag("Core Control Plane"),
		RouteSummary("List all service accounts"),
		RouteDescription("Lists all machine service accounts with status, name, and permission scopes."),
		RouteOperationID("core__service_accounts__list"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("list"),
	)
	PostRoute[CreateServiceAccountResponse, CreateServiceAccountInput](controlPlaneRouter, "/v1/_/core/service-accounts", server.handleCreateServiceAccount,
		RouteTag("Core Control Plane"),
		RouteSummary("Create a new machine service account"),
		RouteDescription("Creates a machine service account, generates a 32-byte hex secret key, and hashes it."),
		RouteDefaultStatusCode(http.StatusCreated),
		RouteOperationID("core__service_accounts__create"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("create"),
	)
	GetRoute[ServiceAccount](controlPlaneRouter, "/v1/_/core/service-accounts/{service_account_id}", server.handleGetServiceAccount,
		RouteTag("Core Control Plane"),
		RouteSummary("Get service account by ID"),
		RouteDescription("Retrieves a service account by UUID."),
		RouteOperationID("core__service_accounts__get"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("get"),
	)
	PutRoute[ServiceAccount, UpdateServiceAccountInput](controlPlaneRouter, "/v1/_/core/service-accounts/{service_account_id}", server.handleUpdateServiceAccount,
		RouteTag("Core Control Plane"),
		RouteSummary("Update service account"),
		RouteDescription("Updates service account scopes, name, or enabled status while protecting root accounts."),
		RouteOperationID("core__service_accounts__update"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("update"),
	)
	DeleteRoute[Empty](controlPlaneRouter, "/v1/_/core/service-accounts/{service_account_id}", server.handleDeleteServiceAccount,
		RouteTag("Core Control Plane"),
		RouteSummary("Delete service account"),
		RouteDescription("Deletes a machine service account, enforcing invariant that at least one root account remains."),
		RouteNoContentResponse("Service account deleted"),
		RouteOperationID("core__service_accounts__delete"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("delete"),
	)

	// Event Hooks Routes
	GetRoute[ListEventHooksResponse](controlPlaneRouter, "/v1/_/core/event-hooks", server.handleListEventHooks,
		RouteTag("Core Control Plane"),
		RouteSummary("List all event hooks"),
		RouteDescription("Lists all active event hooks with driver, target URLs/functions, events, and retry policies."),
		RouteOperationID("core__event_hooks__list"),
		RouteSDKGroupName("core", "eventHooks"),
		RouteSDKMethodName("list"),
	)
	PostRoute[EventHook, CreateEventHookInput](controlPlaneRouter, "/v1/_/core/event-hooks", server.handleCreateEventHook,
		RouteTag("Core Control Plane"),
		RouteSummary("Create a new event hook"),
		RouteDescription("Creates an event hook subscription for SQL stored procedure or HTTP webhook dispatching."),
		RouteDefaultStatusCode(http.StatusCreated),
		RouteOperationID("core__event_hooks__create"),
		RouteSDKGroupName("core", "eventHooks"),
		RouteSDKMethodName("create"),
	)
	GetRoute[EventHook](controlPlaneRouter, "/v1/_/core/event-hooks/{event_hook_id}", server.handleGetEventHook,
		RouteTag("Core Control Plane"),
		RouteSummary("Get event hook by ID"),
		RouteDescription("Retrieves an event hook by UUID."),
		RouteOperationID("core__event_hooks__get"),
		RouteSDKGroupName("core", "eventHooks"),
		RouteSDKMethodName("get"),
	)
	PutRoute[EventHook, UpdateEventHookInput](controlPlaneRouter, "/v1/_/core/event-hooks/{event_hook_id}", server.handleUpdateEventHook,
		RouteTag("Core Control Plane"),
		RouteSummary("Update event hook"),
		RouteDescription("Updates event hook driver, targets, subscribed events, secret, or enabled status."),
		RouteOperationID("core__event_hooks__update"),
		RouteSDKGroupName("core", "eventHooks"),
		RouteSDKMethodName("update"),
	)
	DeleteRoute[Empty](controlPlaneRouter, "/v1/_/core/event-hooks/{event_hook_id}", server.handleDeleteEventHook,
		RouteTag("Core Control Plane"),
		RouteSummary("Delete event hook"),
		RouteDescription("Deletes an event hook."),
		RouteNoContentResponse("Event hook deleted"),
		RouteOperationID("core__event_hooks__delete"),
		RouteSDKGroupName("core", "eventHooks"),
		RouteSDKMethodName("delete"),
	)
	GetRoute[ListEventHookDeliveriesResponse](controlPlaneRouter, "/v1/_/core/event-hooks/{event_hook_id}/deliveries", server.handleListEventHookDeliveries,
		RouteTag("Core Control Plane"),
		RouteSummary("List event hook deliveries"),
		RouteDescription("Queries recent dispatch attempts, response status, and latency for an event hook."),
		RouteOperationID("core__event_hooks__deliveries__list"),
		RouteSDKGroupName("core", "eventHooks", "deliveries"),
		RouteSDKMethodName("list"),
	)
	PostRoute[EventHookDelivery, Empty](controlPlaneRouter, "/v1/_/core/event-hooks/{event_hook_id}/deliveries/{delivery_id}/retry", server.handleRetryEventHookDelivery,
		RouteTag("Core Control Plane"),
		RouteSummary("Retry event hook delivery"),
		RouteDescription("Manually redrives a past event hook delivery attempt."),
		RouteOperationID("core__event_hooks__deliveries__retry"),
		RouteSDKGroupName("core", "eventHooks", "deliveries"),
		RouteSDKMethodName("retry"),
	)

	// Events Routes
	GetRoute[ListEventsResponse](controlPlaneRouter, "/v1/_/core/events", server.handleListEvents,
		RouteTag("Core Control Plane"),
		RouteSummary("List events"),
		RouteDescription("Queries immutable system and domain events with multi-field filtering and pagination."),
		RouteOperationID("core__events__list"),
		RouteSDKGroupName("core", "events"),
		RouteSDKMethodName("list"),
	)
	GetRoute[Event](controlPlaneRouter, "/v1/_/core/events/{event_id}", server.handleGetEvent,
		RouteTag("Core Control Plane"),
		RouteSummary("Get event by ID"),
		RouteDescription("Retrieves an individual event entry by UUID."),
		RouteOperationID("core__events__get"),
		RouteSDKGroupName("core", "events"),
		RouteSDKMethodName("get"),
	)
}

// WriteJSONResponse writes a JSON response with the given status code and payload.
func WriteJSONResponse(responseWriter http.ResponseWriter, statusCode int, payload any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(payload)
}
