package core

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// ErrHostNotPermitted indicates an incoming SNI host is not allowed to acquire TLS certificates.
var ErrHostNotPermitted = errors.New("host is not permitted for TLS termination")

// HostHandlerFunc defines a function that handles an incoming HTTP request based on its host or domain.
// It returns true if the request was handled, or false if it should fall through to standard route dispatch.
type HostHandlerFunc func(responseWriter http.ResponseWriter, request *http.Request) bool

// HostValidatorFunc defines a function that validates whether an incoming SNI host is permitted for TLS termination.
type HostValidatorFunc func(ctx context.Context, host string) (bool, error)

// Server coordinates the Layr HTTP gateway, probes, and API dispatch.
type Server struct {
	kernel             *Kernel
	baseRouter         *Router
	controlPlaneRouter *Router
	serveMux           *http.ServeMux
	server             *http.Server
	httpServer         *http.Server
	autocertManager    *autocert.Manager
	dbCertCache        *DBCertCache
	hostHandlers       []HostHandlerFunc
	hostValidators     []HostValidatorFunc
	tlsEnabled         bool
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
		dbCertCache:        NewDBCertCache(kernel.DB(), kernel.CryptoKeyManager()),
	}

	server.registerBaseRoutes()
	server.registerControlPlaneRoutes()

	// Mount Public Probe Routes directly on root mux (No publishable key required)
	serveMux.Handle("/healthz", baseRouter.Mux())
	serveMux.Handle("/readyz", baseRouter.Mux())
	serveMux.Handle("/metrics", baseRouter.Mux())

	// OIDC & OAuth Discovery (Open access)
	serveMux.Handle("/.well-known/", baseRouter.Mux())

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

	server.initTLS(config)

	return server
}

// RegisterHostHandler registers a host-based request interceptor on the HTTP server.
func (server *Server) RegisterHostHandler(hostHandlerFunc HostHandlerFunc) {
	if hostHandlerFunc != nil {
		server.hostHandlers = append(server.hostHandlers, hostHandlerFunc)
	}
}

// HostHandlers returns all registered host-based request interceptors on the HTTP server.
func (server *Server) HostHandlers() []HostHandlerFunc {
	return server.hostHandlers
}

// RegisterHostValidator registers a host authorization validator for TLS SNI checks.
func (server *Server) RegisterHostValidator(hostValidatorFunc HostValidatorFunc) {
	if hostValidatorFunc != nil {
		server.hostValidators = append(server.hostValidators, hostValidatorFunc)
	}
}

// HostValidators returns all registered host validators on the HTTP server.
func (server *Server) HostValidators() []HostValidatorFunc {
	return server.hostValidators
}

// ValidateHost checks whether the given host is authorized for TLS termination.
// It checks the server base URL, configured static TLS domains, and dynamic host validators.
func (server *Server) ValidateHost(ctx context.Context, host string) (bool, error) {
	normalizedHost := strings.TrimSpace(strings.ToLower(host))
	if normalizedHost == "" {
		return false, nil
	}
	if strings.Contains(normalizedHost, ":") {
		if cleanHost, _, splitErr := net.SplitHostPort(normalizedHost); splitErr == nil {
			normalizedHost = cleanHost
		}
	}

	config := GetConfig()

	// 1. Match BaseURL hostname when BaseURL uses HTTPS (or does not explicitly specify plain http)
	if config.Server.BaseURL != "" {
		if parsedURL, err := url.Parse(config.Server.BaseURL); err == nil && parsedURL.Hostname() != "" {
			if !strings.EqualFold(parsedURL.Scheme, "http") && strings.EqualFold(parsedURL.Hostname(), normalizedHost) {
				return true, nil
			}
		}
	}

	// 2. Match static domains in Server.TLS.Domains
	for _, domain := range config.Server.TLS.Domains {
		cleanedDomain := strings.TrimSpace(strings.ToLower(domain))
		if cleanHost, _, splitErr := net.SplitHostPort(cleanedDomain); splitErr == nil {
			cleanedDomain = cleanHost
		}
		if cleanedDomain != "" && cleanedDomain == normalizedHost {
			return true, nil
		}
	}

	// 3. Match dynamic registered host validators
	for _, validator := range server.hostValidators {
		valid, err := validator(ctx, normalizedHost)
		if err != nil {
			return false, err
		}
		if valid {
			return true, nil
		}
	}

	return false, nil
}

// HostPolicy implements an autocert.HostPolicy that validates incoming hosts against the server.
func (server *Server) HostPolicy(ctx context.Context, host string) error {
	valid, err := server.ValidateHost(ctx, host)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("%w: %s", ErrHostNotPermitted, host)
	}
	return nil
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

// AutocertManager returns the ACME certificate manager when AutoTLS is active.
func (server *Server) AutocertManager() *autocert.Manager {
	return server.autocertManager
}

// DBCertCache returns the database-backed certificate cache when AutoTLS is active.
func (server *Server) DBCertCache() *DBCertCache {
	return server.dbCertCache
}

// TLSConfig returns the configured TLS configuration if TLS is enabled.
func (server *Server) TLSConfig() *tls.Config {
	return server.server.TLSConfig
}

// TLSEnabled reports whether TLS termination is active on the server.
func (server *Server) TLSEnabled() bool {
	return server.tlsEnabled
}

// HTTPServer returns the auxiliary HTTP server if configured for HTTP-to-HTTPS redirect.
func (server *Server) HTTPServer() *http.Server {
	return server.httpServer
}

// Start boots the HTTP or HTTPS server in background.
func (server *Server) Start() error {
	log.Debugf("starting server on %s", server.server.Addr)
	if server.tlsEnabled {
		if server.server.TLSConfig == nil {
			return errors.New("cannot start server: TLS is enabled but TLS configuration failed to initialize")
		}
		if server.httpServer != nil {
			go func() {
				if err := server.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Errorf("http redirect server failed to listen and serve: %v", err)
				}
			}()
		}
		if err := server.server.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("https server failed to listen and serve: %w", err)
		}
		return nil
	}
	if err := server.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server failed to listen and serve: %w", err)
	}
	return nil
}

// Shutdown initiates a graceful drain.
func (server *Server) Shutdown(ctx context.Context) error {
	log.Debugf("shutting down HTTP server")
	if server.httpServer != nil {
		_ = server.httpServer.Shutdown(ctx)
	}
	if err := server.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("http server failed to shut down cleanly: %w", err)
	}
	log.Tracef("HTTP server shutdown complete")
	return nil
}

func (server *Server) initTLS(config *Config) {
	server.tlsEnabled = config.Server.TLS.Enabled || config.Server.TLS.AutoTLS
	if !server.tlsEnabled {
		return
	}

	if config.Server.TLS.AutoTLS {
		manager := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      server.dbCertCache,
			HostPolicy: server.HostPolicy,
		}

		if config.Server.TLS.AcmeEmail != "" {
			manager.Email = config.Server.TLS.AcmeEmail
		}

		if config.Server.TLS.AcmeStaging {
			manager.Client = &acme.Client{
				DirectoryURL: "https://acme-staging-v02.api.letsencrypt.org/directory",
			}
		}

		server.autocertManager = manager

		if _, port, err := net.SplitHostPort(config.Server.ListenAddr); err == nil && port != "443" {
			log.Warnf("AutoTLS is enabled on non-standard port :%s; ensure external port 443 forwards to this listener for ACME TLS-ALPN-01 challenges to succeed", port)
		}

		server.server.TLSConfig = manager.TLSConfig()
	} else {
		// Manual TLS
		var certificate tls.Certificate
		var err error
		if config.Server.TLS.CertFile != "" && config.Server.TLS.KeyFile != "" {
			certificate, err = tls.LoadX509KeyPair(config.Server.TLS.CertFile, config.Server.TLS.KeyFile)
		} else if config.Server.TLS.CertPEM != "" && config.Server.TLS.KeyPEM != "" {
			certificate, err = tls.X509KeyPair([]byte(config.Server.TLS.CertPEM), []byte(config.Server.TLS.KeyPEM))
		}

		if err == nil {
			server.server.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{certificate},
				GetCertificate: func(clientHelloInfo *tls.ClientHelloInfo) (*tls.Certificate, error) { //nolint:contextcheck,nilnil
					if clientHelloInfo == nil || strings.TrimSpace(clientHelloInfo.ServerName) == "" {
						return nil, nil //nolint:nilnil
					}
					cleanName := strings.TrimSpace(strings.ToLower(clientHelloInfo.ServerName))
					if cleanHost, _, splitErr := net.SplitHostPort(cleanName); splitErr == nil {
						cleanName = cleanHost
					}
					cleanName = strings.TrimSuffix(cleanName, ".")
					ctx := clientHelloInfo.Context()
					if ctx == nil {
						ctx = context.Background()
					}
					if cachedBytes, getErr := server.dbCertCache.Get(ctx, cleanName); getErr == nil && len(cachedBytes) > 0 {
						if customCertificate, parseErr := tls.X509KeyPair(cachedBytes, cachedBytes); parseErr == nil {
							return &customCertificate, nil
						}
					}
					return nil, nil //nolint:nilnil
				},
			}
		} else {
			log.Errorf("failed to load TLS certificate/key pair: %v", err)
		}
	}

	if config.Server.TLS.HTTPRedirect {
		fallbackHandler := server.server.Handler
		if config.Server.BaseURL != "" {
			if parsedURL, parseErr := url.Parse(config.Server.BaseURL); parseErr == nil && strings.EqualFold(parsedURL.Scheme, "https") && parsedURL.Host != "" {
				staticRedirectHandler := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
					targetURL := "https://" + parsedURL.Host + request.URL.RequestURI()
					http.Redirect(responseWriter, request, targetURL, http.StatusMovedPermanently)
				})
				middlewareRedirectHandler := server.middleware(staticRedirectHandler)
				fallbackHandler = http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
					if len(server.hostHandlers) == 0 {
						staticRedirectHandler.ServeHTTP(responseWriter, request)
						return
					}
					middlewareRedirectHandler.ServeHTTP(responseWriter, request)
				})
			}
		}

		redirectHandler := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			host := request.Host
			if strings.Contains(host, ":") {
				if cleanHost, _, splitErr := net.SplitHostPort(host); splitErr == nil {
					host = cleanHost
				}
			}

			// 1. If this host is authorized for TLS, redirect to HTTPS
			valid, validateErr := server.ValidateHost(request.Context(), host)
			if validateErr == nil && valid {
				targetURL := "https://" + host + request.URL.RequestURI()
				http.Redirect(responseWriter, request, targetURL, http.StatusMovedPermanently)
				return
			}

			// 2. Delegate to middleware pipeline (handles hostHandlers with auth context, or fallback to BaseURL)
			fallbackHandler.ServeHTTP(responseWriter, request)
		})

		var httpHandler http.Handler = redirectHandler
		if server.autocertManager != nil {
			httpHandler = server.autocertManager.HTTPHandler(redirectHandler)
		} else if server.dbCertCache != nil {
			challengeManager := &autocert.Manager{
				Prompt: autocert.AcceptTOS,
				Cache:  server.dbCertCache,
			}
			httpHandler = challengeManager.HTTPHandler(redirectHandler)
		}

		httpListenAddr := strings.TrimSpace(config.Server.TLS.HTTPListenAddr)
		if httpListenAddr == "" {
			httpListenAddr = ":80"
		}

		server.httpServer = &http.Server{
			Addr:              httpListenAddr,
			Handler:           httpHandler,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
	}
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
		requestID := request.Header.Get("Request-ID")
		if requestID == "" {
			requestID = request.Header.Get("X-Request-ID")
		}
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
						refreshTokenHeader := request.Header.Get("Refresh-Token")
						if refreshTokenHeader == "" {
							refreshTokenHeader = request.Header.Get("X-Refresh-Token")
						}
						if refreshTokenHeader != "" {
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
				sessionRefreshToken = request.Header.Get("Refresh-Token")
				if sessionRefreshToken == "" {
					sessionRefreshToken = request.Header.Get("X-Refresh-Token")
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

// PublishableKeyMiddleware validates Publishable-Key (or fallback X-Publishable-Key / Service Account) on public /v1/* routes.
func (server *Server) PublishableKeyMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		path := request.URL.Path
		log.Tracef("evaluating publishable key middleware for path %s", path)
		if path == "/v1/manifest" ||
			strings.HasPrefix(path, "/v1/_/") ||
			path == "/v1/_" ||
			path == "/v1/spec.json" ||
			path == "/v1/spec.yaml" ||
			strings.HasPrefix(path, "/.well-known/") ||
			strings.HasPrefix(path, "/v1/function/") ||
			path == "/v1/function" {
			handler.ServeHTTP(responseWriter, request)
			return
		}

		publishableKey := request.Header.Get("Publishable-Key")
		if publishableKey == "" {
			publishableKey = request.Header.Get("X-Publishable-Key")
		}
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
		RouteHide(),
		RouteTag("Probes"),
		RouteSummary("Liveness probe"),
		RouteDescription("Returns 200 OK if the Layr gateway process is running and responsive."),
		RouteOperationID("core__healthz"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("healthz"),
	)
	GetRoute[GetReadinessResponse](baseRouter, "/readyz", server.handleGetReadiness,
		RouteHide(),
		RouteTag("Probes"),
		RouteSummary("Readiness probe"),
		RouteDescription("Returns 200 OK if PostgreSQL connection pool is healthy and accepting queries; returns 503 Service Unavailable if unready."),
		RouteOperationID("core__readyz"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("readyz"),
	)
	GetRoute[string](baseRouter, "/metrics", server.handleGetMetrics,
		RouteHide(),
		RouteTag("Observability"),
		RouteSummary("Prometheus metrics exposition"),
		RouteDescription("Prometheus text exposition format (version 0.0.4) exposing process uptime, HTTP requests handled, and allocated heap memory."),
		RouteOperationID("core__metrics"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("metrics"),
	)
	GetRoute[GetManifestResponse](baseRouter, "/v1/manifest", server.handleGetManifest,
		RouteTag("Manifest"),
		RouteSummary("Get dynamic cluster manifest and enabled services"),
		RouteDescription("Returns cluster metadata, list of enabled platform services, public server base URL, and the derived publishable key required for SDK client initialization. Unrestricted public endpoint."),
		RouteOperationID("core__manifest"),
		RouteSDKGroupName("core"),
		RouteSDKMethodName("manifest"),
	)
}

func (server *Server) registerControlPlaneRoutes() {
	controlPlaneRouter := server.controlPlaneRouter

	// Register Core Routes on Control Plane Router
	GetRoute[ListServiceAccountsResponse](controlPlaneRouter, "/v1/_/core/service-accounts", server.handleListServiceAccounts,
		RouteTag("Service Accounts"),
		RouteSummary("List machine service accounts"),
		RouteDescription("Lists all machine service accounts from core.service_accounts with name, key prefix, granted permission scopes, enabled status, IP allowlists, and expiration timestamps. Requires core:service-account.read scope."),
		RouteOperationID("core__service_accounts__list"),
		RouteSDKGroupName("core", "service_accounts"),
		RouteSDKMethodName("list"),
	)
	PostRoute[CreateServiceAccountResponse, CreateServiceAccountInput](controlPlaneRouter, "/v1/_/core/service-accounts", server.handleCreateServiceAccount,
		RouteTag("Service Accounts"),
		RouteSummary("Create a machine service account"),
		RouteDescription("Creates a machine service account, generates a high-entropy 32-byte hex secret key, hashes it with SHA-256 for storage in core.service_accounts, and returns the plaintext secret key once. Supports scope constraints and CIDR IP restrictions. Requires core:service-account.write scope."),
		RouteDefaultStatusCode(http.StatusCreated),
		RouteOperationID("core__service_accounts__create"),
		RouteSDKGroupName("core", "service_accounts"),
		RouteSDKMethodName("create"),
	)
	GetRoute[ServiceAccount](controlPlaneRouter, "/v1/_/core/service-accounts/{service_account_id}", server.handleGetServiceAccount,
		RouteTag("Service Accounts"),
		RouteSummary("Get machine service account details by UUID"),
		RouteDescription("Retrieves a machine service account by UUID from core.service_accounts, returning metadata, permission scopes, key prefix, and allowed IP patterns. Plaintext key is not recoverable. Requires core:service-account.read scope."),
		RouteOperationID("core__service_accounts__get"),
		RouteSDKGroupName("core", "service_accounts"),
		RouteSDKMethodName("get"),
	)
	PutRoute[ServiceAccount, UpdateServiceAccountInput](controlPlaneRouter, "/v1/_/core/service-accounts/{service_account_id}", server.handleUpdateServiceAccount,
		RouteTag("Service Accounts"),
		RouteSummary("Update machine service account properties and scopes"),
		RouteDescription("Modifies name, description, permission scopes, IP allowlist, or enabled status in core.service_accounts. Enforces safety invariant that at least one enabled root account with wildcard * scope remains. Requires core:service-account.write scope."),
		RouteOperationID("core__service_accounts__update"),
		RouteSDKGroupName("core", "service_accounts"),
		RouteSDKMethodName("update"),
	)
	DeleteRoute[Empty](controlPlaneRouter, "/v1/_/core/service-accounts/{service_account_id}", server.handleDeleteServiceAccount,
		RouteTag("Service Accounts"),
		RouteSummary("Delete machine service account"),
		RouteDescription("Deletes a machine service account from core.service_accounts. Enforces strict safety invariants: prevents deleting accounts owned by console users and ensures at least one active root account remains. Requires core:service-account.write scope."),
		RouteNoContentResponse("Service account deleted"),
		RouteOperationID("core__service_accounts__delete"),
		RouteSDKGroupName("core", "service_accounts"),
		RouteSDKMethodName("delete"),
	)

	// Event Hooks Routes
	GetRoute[ListEventHooksResponse](controlPlaneRouter, "/v1/_/core/event-hooks", server.handleListEventHooks,
		RouteTag("Event Hooks"),
		RouteSummary("List event hook subscriptions"),
		RouteDescription("Lists event hook subscriptions from core.event_hooks with driver (sql or http), target function or URL, subscribed event type patterns, retry limits, and enabled status. Requires core:event-hook.read scope."),
		RouteQueryParam("driver", "Filter event hooks by delivery driver ('sql' or 'http')"),
		RouteQueryBool("is_enabled", "Filter event hooks by active enabled status"),
		RouteOperationID("core__event_hooks__list"),
		RouteSDKGroupName("core", "event_hooks"),
		RouteSDKMethodName("list"),
	)
	PostRoute[EventHook, CreateEventHookInput](controlPlaneRouter, "/v1/_/core/event-hooks", server.handleCreateEventHook,
		RouteTag("Event Hooks"),
		RouteSummary("Create an event hook subscription"),
		RouteDescription("Creates an event hook subscription in core.event_hooks for SQL stored procedures (sql driver) or HTTP webhooks (http driver) with HMAC-SHA256 signing secret encryption, wildcard event patterns, timeout guards, and retry policies. Requires core:event-hook.write scope."),
		RouteDefaultStatusCode(http.StatusCreated),
		RouteOperationID("core__event_hooks__create"),
		RouteSDKGroupName("core", "event_hooks"),
		RouteSDKMethodName("create"),
	)
	GetRoute[EventHook](controlPlaneRouter, "/v1/_/core/event-hooks/{event_hook_id}", server.handleGetEventHook,
		RouteTag("Event Hooks"),
		RouteSummary("Get event hook subscription details by UUID"),
		RouteDescription("Retrieves event hook configuration by UUID from core.event_hooks, including driver type, target endpoint, subscribed event patterns, retry limits, and secret configuration indicator. Requires core:event-hook.read scope."),
		RouteOperationID("core__event_hooks__get"),
		RouteSDKGroupName("core", "event_hooks"),
		RouteSDKMethodName("get"),
	)
	PutRoute[EventHook, UpdateEventHookInput](controlPlaneRouter, "/v1/_/core/event-hooks/{event_hook_id}", server.handleUpdateEventHook,
		RouteTag("Event Hooks"),
		RouteSummary("Update event hook subscription"),
		RouteDescription("Modifies event hook parameters in core.event_hooks including target URL or stored procedure, subscribed event patterns, retry count (0-10), timeout (1-60s), signing secret, or enabled status. Requires core:event-hook.write scope."),
		RouteOperationID("core__event_hooks__update"),
		RouteSDKGroupName("core", "event_hooks"),
		RouteSDKMethodName("update"),
	)
	DeleteRoute[Empty](controlPlaneRouter, "/v1/_/core/event-hooks/{event_hook_id}", server.handleDeleteEventHook,
		RouteTag("Event Hooks"),
		RouteSummary("Delete event hook subscription"),
		RouteDescription("Permanently removes an event hook subscription from core.event_hooks, immediately unbinding event dispatches. Delivery history is preserved or cascaded according to retention rules. Requires core:event-hook.write scope."),
		RouteNoContentResponse("Event hook deleted"),
		RouteOperationID("core__event_hooks__delete"),
		RouteSDKGroupName("core", "event_hooks"),
		RouteSDKMethodName("delete"),
	)
	GetRoute[ListEventHookDeliveriesResponse](controlPlaneRouter, "/v1/_/core/event-hooks/{event_hook_id}/deliveries", server.handleListEventHookDeliveries,
		RouteTag("Event Hooks Deliveries"),
		RouteSummary("List delivery audit logs for an event hook"),
		RouteDescription("Queries the 100 most recent dispatch attempts for an event hook from core.event_hook_deliveries, detailing event ID, payload, HTTP status code, duration in milliseconds, error messages, and delivery state. Requires core:event-hook.read scope."),
		RouteOperationID("core__event_hooks__deliveries__list"),
		RouteSDKGroupName("core", "event_hooks", "deliveries"),
		RouteSDKMethodName("list"),
	)
	PostRoute[EventHookDelivery, Empty](controlPlaneRouter, "/v1/_/core/event-hooks/{event_hook_id}/deliveries/{delivery_id}/retry", server.handleRetryEventHookDelivery,
		RouteTag("Event Hooks Deliveries"),
		RouteSummary("Retry a past event hook delivery attempt"),
		RouteDescription("Manually redrives an event hook delivery using the recorded event payload, re-dispatching via the configured driver (SQL procedure or HTTP POST webhook) and recording the new attempt outcome. Requires core:event-hook.write scope."),
		RouteOperationID("core__event_hooks__deliveries__retry"),
		RouteSDKGroupName("core", "event_hooks", "deliveries"),
		RouteSDKMethodName("retry"),
	)

	// Events Routes
	GetRoute[ListEventsResponse](controlPlaneRouter, "/v1/_/core/events", server.handleListEvents,
		RouteTag("Events"),
		RouteSummary("Query immutable system and domain audit events"),
		RouteDescription("Queries immutable system and domain events from core.events with multi-field filtering by event type, actor type, date ranges, and pagination limits. Returns full event context, actor, and payload data. Requires core:event.read scope."),
		RouteDefaultPagination(),
		RouteQueryParam("type", "Filter by specific event type name"),
		RouteQueryParam("actor_type", "Filter by actor principal type (user, service_account, system)"),
		RouteQueryParam("actor_id", "Filter by actor principal UUID"),
		RouteQueryParam("resource_type", "Filter by target resource entity type"),
		RouteQueryParam("resource_id", "Filter by target resource unique identifier"),
		RouteOperationID("core__events__list"),
		RouteSDKGroupName("core", "events"),
		RouteSDKMethodName("list"),
	)
	GetRoute[Event](controlPlaneRouter, "/v1/_/core/events/{event_id}", server.handleGetEvent,
		RouteTag("Events"),
		RouteSummary("Get individual event record by UUID"),
		RouteDescription("Retrieves a specific canonical event record by UUID from core.events, including event type, actor principal, transport context (IP, user agent, request ID), and structured payload data. Requires core:event.read scope."),
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
