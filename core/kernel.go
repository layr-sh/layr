package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"uuid"

	"layr.sh/auth/password"
)

const (
	defaultDatabaseConnectionTimeout = 30 * time.Second
	defaultKernelShutdownTimeout     = 10 * time.Second
)

// ServiceRunner defines the lifecycle interface for Layr modular services.
type ServiceRunner interface {
	Start(ctx context.Context) error
	Stop() error
	RegisterRoutes(router *Router, controlPlaneRouter *Router)
}

// ServiceFactory defines a constructor function for a modular service given an active kernel.
type ServiceFactory func(kernel *Kernel) (ServiceRunner, error)

var (
	serviceFactoriesRWMutex sync.RWMutex
	serviceFactories        = make(map[string]ServiceFactory)
)

// RegisterServiceFactory registers a named service constructor into the global registry.
func RegisterServiceFactory(name string, serviceFactory ServiceFactory) {
	serviceFactoriesRWMutex.Lock()
	defer serviceFactoriesRWMutex.Unlock()
	serviceFactories[name] = serviceFactory
}

// GetServiceFactory retrieves a named service constructor from the global registry.
func GetServiceFactory(name string) (ServiceFactory, bool) {
	serviceFactoriesRWMutex.RLock()
	defer serviceFactoriesRWMutex.RUnlock()
	serviceFactory, ok := serviceFactories[name]
	return serviceFactory, ok
}

// Kernel coordinates the full single-binary runtime lifecycle.
type Kernel struct {
	//nolint:namingclarity
	embeddedDB            *EmbeddedDatabase
	db                    *DatabasePool
	nodeRegistry          *NodeRegistry
	kvStore               KVStore
	cryptoKeyManager      *CryptoKeyManager
	webhookEventBus       *WebhookEventBus
	serviceAccountManager *ServiceAccountManager
	webhookManager        *WebhookManager
	server                *Server
	services              []ServiceRunner
	stopOnce              sync.Once
}

// NewKernel initializes the Layr Kernel.
func NewKernel() (*Kernel, error) {
	log.Debugf("initializing Kernel instance")
	config := GetConfig()
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration invariant violation: %w", err)
	}

	cryptoKeyManager, _ := NewCryptoKeyManager(config.Security.MasterEncryptionKey) // Infallible: config.Validate() guarantees 32-byte hex key
	kernel := &Kernel{
		cryptoKeyManager: cryptoKeyManager,
	}

	return kernel, nil
}

// PublishableKey returns the deterministic publishable key derived from the master encryption key.
func (kernel *Kernel) PublishableKey() string {
	if kernel.cryptoKeyManager == nil {
		return ""
	}
	return kernel.cryptoKeyManager.DerivePublishableKey()
}

// Start boots the database, applies migrations, launches heartbeats, and starts the HTTP gateway.
func (kernel *Kernel) Start(ctx context.Context) (err error) {
	log.Debugf("starting Kernel subsystem initialization")
	defer func() {
		if err != nil {
			_ = kernel.Stop(ctx)
		}
	}()

	config := GetConfig()
	databaseURL := config.Database.URL

	// Embedded PostgreSQL handling
	if IsEmbeddedDatabasePath(databaseURL) {
		kernel.embeddedDB = NewEmbeddedDatabase(databaseURL)
		bootedURL, bootErr := kernel.embeddedDB.Start(ctx)
		if bootErr != nil {
			return fmt.Errorf("embedded postgresql engine failed to boot: %w", bootErr)
		}
		databaseURL = bootedURL
	}

	// Connect pgxpool (derive pool lifecycle timeout from the caller's context)
	dbCtx, dbCancel := context.WithTimeout(ctx, defaultDatabaseConnectionTimeout)
	defer dbCancel()
	db, err := NewDatabasePool(dbCtx, databaseURL, DatabasePoolOptions{
		MaxConns:            int32(config.Database.MaxConnections),
		MinConns:            int32(config.Database.MinConnections),
		ConnectionTimeoutMs: config.Database.ConnectionTimeoutMs,
		MaxConnIdleTime:     time.Duration(config.Database.IdleTimeoutMs) * time.Millisecond,
		MaxConnLifetime:     time.Duration(config.Database.MaxLifetimeMs) * time.Millisecond,
		HealthCheckPeriod:   time.Duration(config.Database.HealthCheckPeriodMs) * time.Millisecond,
		SSLMode:             config.Database.SSLMode,
		SSLRootCert:         config.Database.SSLRootCert,
		SSLCert:             config.Database.SSLCert,
		SSLKey:              config.Database.SSLKey,
	})
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	kernel.db = db

	// Run Core Migrations
	log.Infof("Executing foundational migrations...")
	allMigrations := GetRegisteredDatabaseMigrations()

	if migrationErr := kernel.db.RunMigrations(ctx, allMigrations); migrationErr != nil {
		return fmt.Errorf("database migration failed: %w", migrationErr)
	}

	// Register Node in Cluster
	nodeName, _ := os.Hostname()
	kernel.nodeRegistry = NewNodeRegistry(kernel.db, nodeName, config.GetEnabledServices())
	_ = kernel.nodeRegistry.Register(ctx)

	// Initialize KV Store
	kvStore, err := NewKVStore(ctx, kernel.db)
	if err != nil {
		return fmt.Errorf("kv store initialization failed: %w", err)
	}
	kernel.kvStore = kvStore

	// Initialize WebhookEventBus and Core Managers
	kernel.webhookEventBus = NewWebhookEventBus(kernel.db, kernel.cryptoKeyManager)
	kernel.serviceAccountManager = NewServiceAccountManager(kernel.db)
	kernel.webhookManager = NewWebhookManager(kernel.db, kernel.cryptoKeyManager, kernel.webhookEventBus)

	// Bootstrap Initial Root Account (Console User + Linked Service Account)
	if err := kernel.bootstrapRootAccount(ctx); err != nil {
		return fmt.Errorf("root account bootstrap failed: %w", err)
	}

	// Initialize HTTP Gateway
	kernel.server = NewServer(kernel.db, kernel.cryptoKeyManager)

	// Register Core Routes
	kernel.registerCoreRoutes(kernel.server)

	// Auto-instantiate enabled modular services from registry if not manually registered
	if len(kernel.services) == 0 {
		for _, serviceName := range config.GetEnabledServices() {
			if serviceFactory, ok := GetServiceFactory(serviceName); ok {
				serviceRunner, err := serviceFactory(kernel)
				if err != nil {
					return fmt.Errorf("failed to initialize service %s: %w", serviceName, err)
				}
				kernel.RegisterService(serviceRunner)
			}
		}
	}

	// Start and register all attached modular services
	for _, serviceRunner := range kernel.services {
		if err := serviceRunner.Start(ctx); err != nil {
			return fmt.Errorf("failed to start service: %w", err)
		}
		serviceRunner.RegisterRoutes(kernel.server.Router(), kernel.server.ControlPlaneRouter())
	}

	log.Infof("Layr Gateway listening on %s (Services: %v)", config.Server.ListenAddr, config.GetEnabledServices())

	errChannel := make(chan error, 1)
	go func() {
		if err := kernel.server.Start(); err != nil && err != http.ErrServerClosed {
			errChannel <- err
		}
	}()

	// Signal handling
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChannel:
		return err
	case receivedSignal := <-signalChannel:
		log.Infof("Received signal %s, initiating graceful shutdown...", receivedSignal)
		return kernel.Stop(ctx)
	case <-ctx.Done():
		log.Infof("Context cancelled, initiating graceful shutdown...")
		return kernel.Stop(ctx)
	}
}

// DB returns the database connection pool.
func (kernel *Kernel) DB() *DatabasePool {
	if kernel == nil {
		return nil
	}
	return kernel.db
}

// CryptoKeyManager returns the master key manager.
func (kernel *Kernel) CryptoKeyManager() *CryptoKeyManager {
	if kernel == nil {
		return nil
	}
	return kernel.cryptoKeyManager
}

// KVStore returns the pluggable key-value store.
func (kernel *Kernel) KVStore() KVStore {
	if kernel == nil {
		return nil
	}
	return kernel.kvStore
}

// WebhookEventBus returns the platform event bus.
func (kernel *Kernel) WebhookEventBus() *WebhookEventBus {
	if kernel == nil {
		return nil
	}
	return kernel.webhookEventBus
}

// ServiceAccountManager returns the machine service account manager.
func (kernel *Kernel) ServiceAccountManager() *ServiceAccountManager {
	if kernel == nil {
		return nil
	}
	return kernel.serviceAccountManager
}

// WebhookManager returns the webhook delivery manager.
func (kernel *Kernel) WebhookManager() *WebhookManager {
	if kernel == nil {
		return nil
	}
	return kernel.webhookManager
}

// SetDB sets the database connection pool on the kernel.
func (kernel *Kernel) SetDB(db *DatabasePool) {
	if kernel != nil {
		kernel.db = db
	}
}

// SetKVStore sets the KV store on the kernel.
func (kernel *Kernel) SetKVStore(kvStore KVStore) {
	if kernel != nil {
		kernel.kvStore = kvStore
	}
}

// SetWebhookEventBus sets the platform event bus on the kernel.
func (kernel *Kernel) SetWebhookEventBus(webhookEventBus *WebhookEventBus) {
	if kernel != nil {
		kernel.webhookEventBus = webhookEventBus
	}
}

// SetServiceAccountManager sets the service account manager on the kernel.
func (kernel *Kernel) SetServiceAccountManager(serviceAccountManager *ServiceAccountManager) {
	if kernel != nil {
		kernel.serviceAccountManager = serviceAccountManager
	}
}

// RegisterService attaches a modular service runner to the kernel.
func (kernel *Kernel) RegisterService(serviceRunner ServiceRunner) {
	kernel.services = append(kernel.services, serviceRunner)
}

// Stop gracefully terminates all subsystems.
func (kernel *Kernel) Stop(ctx context.Context) error {
	kernel.stopOnce.Do(func() {
		log.Debugf("stopping Kernel services...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), defaultKernelShutdownTimeout)
		defer shutdownCancel()

		for i := len(kernel.services) - 1; i >= 0; i-- {
			_ = kernel.services[i].Stop()
		}
		if kernel.webhookEventBus != nil {
			kernel.webhookEventBus.Close()
		}
		if kernel.kvStore != nil {
			_ = kernel.kvStore.Close()
		}
		if kernel.server != nil {
			_ = kernel.server.Shutdown(shutdownCtx)
		}
		if kernel.nodeRegistry != nil {
			kernel.nodeRegistry.Close()
		}
		if kernel.db != nil {
			kernel.db.Close()
		}
		if kernel.embeddedDB != nil {
			_ = kernel.embeddedDB.Stop()
		}

		log.Tracef("all Kernel subsystems stopped")
		log.Infof("Layr process stopped cleanly.")
	})
	return nil
}

// Empty represents an empty JSON object.
type Empty struct{}

func (kernel *Kernel) registerCoreRoutes(server *Server) {
	serveMux := server.Mux()
	controlPlaneRouter := server.ControlPlaneRouter()

	// Register Core Routes on Control Plane Router
	GetRoute[[]ServiceAccount](controlPlaneRouter, "/api/v1/_/core/service-accounts", kernel.handleListServiceAccountsRequest,
		RouteTag("Core Control Plane"),
		RouteSummary("List all service accounts"),
		RouteDescription("Lists all machine service accounts with status, name, and permission scopes."),
		RouteOperationID("core__service_accounts__list"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("list"),
	)
	PostRoute[ServiceAccountWithSecretKey, CreateServiceAccountInput](controlPlaneRouter, "/api/v1/_/core/service-accounts", kernel.handleCreateServiceAccountRequest,
		RouteTag("Core Control Plane"),
		RouteSummary("Create a new machine service account"),
		RouteDescription("Creates a machine service account, generates a 32-byte hex secret key, and hashes it."),
		RouteDefaultStatusCode(http.StatusCreated),
		RouteOperationID("core__service_accounts__create"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("create"),
	)
	GetRoute[ServiceAccount](controlPlaneRouter, "/api/v1/_/core/service-accounts/{service_account_id}", kernel.handleGetServiceAccountRequest,
		RouteTag("Core Control Plane"),
		RouteSummary("Get service account by ID"),
		RouteDescription("Retrieves a service account by UUID."),
		RouteOperationID("core__service_accounts__get"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("get"),
	)
	PutRoute[ServiceAccount, UpdateServiceAccountInput](controlPlaneRouter, "/api/v1/_/core/service-accounts/{service_account_id}", kernel.handleUpdateServiceAccountRequest,
		RouteTag("Core Control Plane"),
		RouteSummary("Update service account"),
		RouteDescription("Updates service account scopes, name, or enabled status while protecting root accounts."),
		RouteOperationID("core__service_accounts__update"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("update"),
	)
	DeleteRoute[Empty](controlPlaneRouter, "/api/v1/_/core/service-accounts/{service_account_id}", kernel.handleDeleteServiceAccountRequest,
		RouteTag("Core Control Plane"),
		RouteSummary("Delete service account"),
		RouteDescription("Deletes a machine service account, enforcing invariant that at least one root account remains."),
		RouteNoContentResponse("Service account deleted"),
		RouteOperationID("core__service_accounts__delete"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("delete"),
	)

	GetRoute[[]Webhook](controlPlaneRouter, "/api/v1/_/core/webhooks", kernel.handleListWebhooksRequest,
		RouteTag("Core Control Plane"),
		RouteSummary("List all webhooks"),
		RouteDescription("Lists all active webhooks with target URLs, events, and retry policies."),
		RouteOperationID("core__webhooks__list"),
		RouteSDKGroupName("core", "webhooks"),
		RouteSDKMethodName("list"),
	)
	PostRoute[Webhook, CreateWebhookInput](controlPlaneRouter, "/api/v1/_/core/webhooks", kernel.handleCreateWebhookRequest,
		RouteTag("Core Control Plane"),
		RouteSummary("Create a new webhook"),
		RouteDescription("Creates an event subscription for system event dispatching with HMAC-SHA256 signature verification."),
		RouteDefaultStatusCode(http.StatusCreated),
		RouteOperationID("core__webhooks__create"),
		RouteSDKGroupName("core", "webhooks"),
		RouteSDKMethodName("create"),
	)
	GetRoute[Webhook](controlPlaneRouter, "/api/v1/_/core/webhooks/{webhook_id}", kernel.handleGetWebhookRequest,
		RouteTag("Core Control Plane"),
		RouteSummary("Get webhook by ID"),
		RouteDescription("Retrieves a webhook by UUID."),
		RouteOperationID("core__webhooks__get"),
		RouteSDKGroupName("core", "webhooks"),
		RouteSDKMethodName("get"),
	)
	PutRoute[Webhook, UpdateWebhookInput](controlPlaneRouter, "/api/v1/_/core/webhooks/{webhook_id}", kernel.handleUpdateWebhookRequest,
		RouteTag("Core Control Plane"),
		RouteSummary("Update webhook"),
		RouteDescription("Updates webhook target URL, subscribed events, secret, or enabled status."),
		RouteOperationID("core__webhooks__update"),
		RouteSDKGroupName("core", "webhooks"),
		RouteSDKMethodName("update"),
	)
	DeleteRoute[Empty](controlPlaneRouter, "/api/v1/_/core/webhooks/{webhook_id}", kernel.handleDeleteWebhookRequest,
		RouteTag("Core Control Plane"),
		RouteSummary("Delete webhook"),
		RouteDescription("Deletes a webhook and purges delivery workers."),
		RouteNoContentResponse("Webhook deleted"),
		RouteOperationID("core__webhooks__delete"),
		RouteSDKGroupName("core", "webhooks"),
		RouteSDKMethodName("delete"),
	)
	GetRoute[[]WebhookDelivery](controlPlaneRouter, "/api/v1/_/core/webhooks/{webhook_id}/deliveries", kernel.handleListWebhookDeliveriesRequest,
		RouteTag("Core Control Plane"),
		RouteSummary("List webhook deliveries"),
		RouteDescription("Queries recent dispatch attempts, response codes, and latency for a webhook."),
		RouteOperationID("core__webhooks__deliveries__list"),
		RouteSDKGroupName("core", "webhooks", "deliveries"),
		RouteSDKMethodName("list"),
	)

	// Mount Core Control Plane Router into root mux with Service Account authentication
	serveMux.Handle("/api/v1/_/core/", ServiceAccountAuthMiddleware(kernel.serviceAccountManager)(controlPlaneRouter.Mux()))
}

func (kernel *Kernel) writeJSON(responseWriter http.ResponseWriter, data any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(data)
}

func (kernel *Kernel) writeError(responseWriter http.ResponseWriter, status int, title string, detail string) {
	WriteErrorResponseProblem(responseWriter, nil, status, title, detail, "LAYR_CORE_001")
}

func (kernel *Kernel) handleListServiceAccountsRequest(responseWriter http.ResponseWriter, request *http.Request) {
	accounts, err := kernel.serviceAccountManager.List(request.Context())
	if err != nil {
		kernel.writeError(responseWriter, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}
	kernel.writeJSON(responseWriter, accounts)
}

func (kernel *Kernel) handleCreateServiceAccountRequest(responseWriter http.ResponseWriter, request *http.Request) {
	var createServiceAccountInput CreateServiceAccountInput
	if err := json.NewDecoder(request.Body).Decode(&createServiceAccountInput); err != nil {
		kernel.writeError(responseWriter, http.StatusBadRequest, "Invalid Request Body", err.Error())
		return
	}
	serviceAccount, err := kernel.serviceAccountManager.Create(request.Context(), createServiceAccountInput)
	if err != nil {
		kernel.writeError(responseWriter, http.StatusBadRequest, err.Error(), err.Error())
		return
	}
	if kernel.webhookEventBus != nil {
		kernel.webhookEventBus.Publish(request.Context(), WebhookEventEnvelope{
			Event:    "core.service_account.created",
			Service:  "core",
			Resource: "service_account",
			Action:   "created",
			Data:     serviceAccount.ServiceAccount,
		})
	}
	kernel.writeJSON(responseWriter, serviceAccount)
}

func (kernel *Kernel) handleGetServiceAccountRequest(responseWriter http.ResponseWriter, request *http.Request) {
	serviceAccountID := request.PathValue("service_account_id")
	serviceAccount, err := kernel.serviceAccountManager.Get(request.Context(), serviceAccountID)
	if err != nil {
		kernel.writeError(responseWriter, http.StatusNotFound, "Service Account Not Found", err.Error())
		return
	}
	kernel.writeJSON(responseWriter, serviceAccount)
}

func (kernel *Kernel) handleUpdateServiceAccountRequest(responseWriter http.ResponseWriter, request *http.Request) {
	serviceAccountID := request.PathValue("service_account_id")
	var updateServiceAccountInput UpdateServiceAccountInput
	if err := json.NewDecoder(request.Body).Decode(&updateServiceAccountInput); err != nil {
		kernel.writeError(responseWriter, http.StatusBadRequest, "Invalid Request Body", err.Error())
		return
	}
	serviceAccount, err := kernel.serviceAccountManager.Update(request.Context(), serviceAccountID, updateServiceAccountInput)
	if err != nil {
		if errors.Is(err, ErrServiceAccountNotFound) {
			kernel.writeError(responseWriter, http.StatusNotFound, "Service Account Not Found", err.Error())
			return
		}
		kernel.writeError(responseWriter, http.StatusForbidden, "Root Account Protected", err.Error())
		return
	}
	if kernel.webhookEventBus != nil {
		kernel.webhookEventBus.Publish(request.Context(), WebhookEventEnvelope{
			Event:    "core.service_account.updated",
			Service:  "core",
			Resource: "service_account",
			Action:   "updated",
			Data:     serviceAccount,
		})
	}
	kernel.writeJSON(responseWriter, serviceAccount)
}

func (kernel *Kernel) handleDeleteServiceAccountRequest(responseWriter http.ResponseWriter, request *http.Request) {
	serviceAccountID := request.PathValue("service_account_id")
	err := kernel.serviceAccountManager.Delete(request.Context(), serviceAccountID)
	if err != nil {
		if errors.Is(err, ErrServiceAccountNotFound) {
			kernel.writeError(responseWriter, http.StatusNotFound, "Service Account Not Found", err.Error())
			return
		}
		kernel.writeError(responseWriter, http.StatusForbidden, "Root Account Protected", err.Error())
		return
	}
	if kernel.webhookEventBus != nil {
		kernel.webhookEventBus.Publish(request.Context(), WebhookEventEnvelope{
			Event:    "core.service_account.deleted",
			Service:  "core",
			Resource: "service_account",
			Action:   "deleted",
			Data:     map[string]string{"id": serviceAccountID},
		})
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (kernel *Kernel) handleListWebhooksRequest(responseWriter http.ResponseWriter, request *http.Request) {
	webhooks, err := kernel.webhookManager.List(request.Context())
	if err != nil {
		kernel.writeError(responseWriter, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}
	kernel.writeJSON(responseWriter, webhooks)
}

func (kernel *Kernel) handleCreateWebhookRequest(responseWriter http.ResponseWriter, request *http.Request) {
	var createWebhookInput CreateWebhookInput
	if err := json.NewDecoder(request.Body).Decode(&createWebhookInput); err != nil {
		kernel.writeError(responseWriter, http.StatusBadRequest, "Invalid Request Body", err.Error())
		return
	}
	webhook, err := kernel.webhookManager.Create(request.Context(), createWebhookInput)
	if err != nil {
		kernel.writeError(responseWriter, http.StatusBadRequest, err.Error(), err.Error())
		return
	}
	if kernel.webhookEventBus != nil {
		kernel.webhookEventBus.Publish(request.Context(), WebhookEventEnvelope{
			Event:    "core.webhook.created",
			Service:  "core",
			Resource: "webhook",
			Action:   "created",
			Data:     webhook,
		})
	}
	kernel.writeJSON(responseWriter, webhook)
}

func (kernel *Kernel) handleGetWebhookRequest(responseWriter http.ResponseWriter, request *http.Request) {
	webhookID := request.PathValue("webhook_id")
	webhook, err := kernel.webhookManager.Get(request.Context(), webhookID)
	if err != nil {
		kernel.writeError(responseWriter, http.StatusNotFound, "Webhook Not Found", err.Error())
		return
	}
	kernel.writeJSON(responseWriter, webhook)
}

func (kernel *Kernel) handleUpdateWebhookRequest(responseWriter http.ResponseWriter, request *http.Request) {
	webhookID := request.PathValue("webhook_id")
	var updateWebhookInput UpdateWebhookInput
	if err := json.NewDecoder(request.Body).Decode(&updateWebhookInput); err != nil {
		kernel.writeError(responseWriter, http.StatusBadRequest, "Invalid Request Body", err.Error())
		return
	}
	webhook, err := kernel.webhookManager.Update(request.Context(), webhookID, updateWebhookInput)
	if err != nil {
		kernel.writeError(responseWriter, http.StatusNotFound, "Webhook Not Found", err.Error())
		return
	}
	if kernel.webhookEventBus != nil {
		kernel.webhookEventBus.Publish(request.Context(), WebhookEventEnvelope{
			Event:    "core.webhook.updated",
			Service:  "core",
			Resource: "webhook",
			Action:   "updated",
			Data:     webhook,
		})
	}
	kernel.writeJSON(responseWriter, webhook)
}

func (kernel *Kernel) handleDeleteWebhookRequest(responseWriter http.ResponseWriter, request *http.Request) {
	webhookID := request.PathValue("webhook_id")
	err := kernel.webhookManager.Delete(request.Context(), webhookID)
	if err != nil {
		kernel.writeError(responseWriter, http.StatusNotFound, "Webhook Not Found", err.Error())
		return
	}
	if kernel.webhookEventBus != nil {
		kernel.webhookEventBus.Publish(request.Context(), WebhookEventEnvelope{
			Event:    "core.webhook.deleted",
			Service:  "core",
			Resource: "webhook",
			Action:   "deleted",
			Data:     map[string]string{"id": webhookID},
		})
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (kernel *Kernel) handleListWebhookDeliveriesRequest(responseWriter http.ResponseWriter, request *http.Request) {
	webhookID := request.PathValue("webhook_id")
	deliveries, err := kernel.webhookManager.ListDeliveries(request.Context(), webhookID)
	if err != nil {
		kernel.writeError(responseWriter, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}
	kernel.writeJSON(responseWriter, deliveries)
}

var defaultPasswordHasher = password.NewHasher()

func (kernel *Kernel) bootstrapRootAccount(ctx context.Context) error {
	if kernel.db == nil || kernel.serviceAccountManager == nil {
		return errors.New("database pool or service account manager unavailable")
	}

	var count int
	err := kernel.db.QueryRow(ctx, "SELECT COUNT(*) FROM console.users").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count console users: %w", err)
	}

	log.Debugf("checking console root account status")
	if count > 0 {
		return nil
	}

	config := GetConfig()
	email := config.Console.InitialUserEmail
	plainPassword := config.Console.InitialUserPassword

	if email == "" {
		email = "root@layr.local"
	}
	if plainPassword == "" {
		randomPassword, _ := GenerateRandomCryptoEncryptionKeyHex()
		plainPassword = randomPassword[:16]
	}

	hash, err := defaultPasswordHasher.Hash(plainPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password for bootstrap user: %w", err)
	}

	rootUserID := uuid.NewV7().String()
	_, err = kernel.db.Exec(ctx, `
		INSERT INTO console.users (id, email, password_hash, is_enabled)
		VALUES ($1, $2, $3, true)
	`, rootUserID, email, hash)
	if err != nil {
		return fmt.Errorf("failed to insert bootstrap console user: %w", err)
	}

	rootDescription := "Root Service Account (" + email + ")"
	createdServiceAccount, err := kernel.serviceAccountManager.Create(ctx, CreateServiceAccountInput{
		ConsoleUserID: &rootUserID,
		Name:          "Root Service Account",
		Description:   &rootDescription,
		Scopes:        []string{ScopeRoot},
	})
	if err != nil {
		return fmt.Errorf("failed to create linked root service account: %w", err)
	}

	log.Tracef("created linked root service account (id: %s)", createdServiceAccount.ID)
	log.Infof("Initial console root account created:")
	log.Infof("  Email:               %s", email)
	log.Infof("  Password:            %s", plainPassword)
	log.Infof("  Service Account Key: %s", createdServiceAccount.SecretKey)

	return nil
}
