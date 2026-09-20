package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdlog "log"
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
	embeddedDB            *EmbeddedDatabase //nolint:namingclarity
	db                    *DatabasePool
	nodeRegistry          *NodeRegistry
	kvStore               *KVStore
	cryptoKeyManager      *CryptoKeyManager
	eventBus              *EventBus
	eventManager          *EventManager
	eventHookManager      *EventHookManager
	serviceAccountManager *ServiceAccountManager
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

	// Initialize EventBus and Core Managers
	kernel.eventBus = NewEventBus(kernel.db, kernel.cryptoKeyManager)
	kernel.eventManager = NewEventManager(kernel.db)
	kernel.eventHookManager = NewEventHookManager(kernel.db, kernel.cryptoKeyManager, kernel.eventBus)
	kernel.serviceAccountManager = NewServiceAccountManager(kernel.db)

	kernel.eventBus.SetEventManager(kernel.eventManager)
	kernel.eventBus.SetEventHookManager(kernel.eventHookManager)

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
		serviceRunner.RegisterRoutes(kernel.server.BaseRouter(), kernel.server.ControlPlaneRouter())
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
func (kernel *Kernel) KVStore() *KVStore {
	if kernel == nil {
		return nil
	}
	return kernel.kvStore
}

// EventBus returns the platform event bus.
func (kernel *Kernel) EventBus() *EventBus {
	if kernel == nil {
		return nil
	}
	return kernel.eventBus
}

// EventManager returns the event manager.
func (kernel *Kernel) EventManager() *EventManager {
	if kernel == nil {
		return nil
	}
	return kernel.eventManager
}

// EventHookManager returns the event hook manager.
func (kernel *Kernel) EventHookManager() *EventHookManager {
	if kernel == nil {
		return nil
	}
	return kernel.eventHookManager
}

// ServiceAccountManager returns the machine service account manager.
func (kernel *Kernel) ServiceAccountManager() *ServiceAccountManager {
	if kernel == nil {
		return nil
	}
	return kernel.serviceAccountManager
}

// SetDB sets the database connection pool on the kernel.
func (kernel *Kernel) SetDB(db *DatabasePool) {
	if kernel != nil {
		kernel.db = db
	}
}

// SetKVStore sets the KV store on the kernel.
func (kernel *Kernel) SetKVStore(kvStore *KVStore) {
	if kernel != nil {
		kernel.kvStore = kvStore
	}
}

// SetEventBus sets the platform event bus on the kernel.
func (kernel *Kernel) SetEventBus(eventBus *EventBus) {
	if kernel != nil {
		kernel.eventBus = eventBus
	}
}

// SetEventManager sets the event manager on the kernel.
func (kernel *Kernel) SetEventManager(eventManager *EventManager) {
	if kernel != nil {
		kernel.eventManager = eventManager
	}
}

// SetEventHookManager sets the event hook manager on the kernel.
func (kernel *Kernel) SetEventHookManager(eventHookManager *EventHookManager) {
	if kernel != nil {
		kernel.eventHookManager = eventHookManager
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

		if kernel.server != nil {
			_ = kernel.server.Shutdown(shutdownCtx)
		}
		if kernel.nodeRegistry != nil {
			kernel.nodeRegistry.Close()
		}
		for i := len(kernel.services) - 1; i >= 0; i-- {
			_ = kernel.services[i].Stop()
		}
		if kernel.eventBus != nil {
			kernel.eventBus.Close()
		}
		if kernel.kvStore != nil {
			_ = kernel.kvStore.Close()
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

func (kernel *Kernel) registerCoreRoutes(server *Server) {
	serveMux := server.Mux()
	controlPlaneRouter := server.ControlPlaneRouter()

	// Register Core Routes on Control Plane Router
	GetRoute[ListServiceAccountsResponse](controlPlaneRouter, "/api/v1/_/core/service-accounts", kernel.handleListServiceAccounts,
		RouteTag("Core Control Plane"),
		RouteSummary("List all service accounts"),
		RouteDescription("Lists all machine service accounts with status, name, and permission scopes."),
		RouteOperationID("core__service_accounts__list"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("list"),
	)
	PostRoute[CreateServiceAccountResponse, CreateServiceAccountInput](controlPlaneRouter, "/api/v1/_/core/service-accounts", kernel.handleCreateServiceAccount,
		RouteTag("Core Control Plane"),
		RouteSummary("Create a new machine service account"),
		RouteDescription("Creates a machine service account, generates a 32-byte hex secret key, and hashes it."),
		RouteDefaultStatusCode(http.StatusCreated),
		RouteOperationID("core__service_accounts__create"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("create"),
	)
	GetRoute[ServiceAccount](controlPlaneRouter, "/api/v1/_/core/service-accounts/{service_account_id}", kernel.handleGetServiceAccount,
		RouteTag("Core Control Plane"),
		RouteSummary("Get service account by ID"),
		RouteDescription("Retrieves a service account by UUID."),
		RouteOperationID("core__service_accounts__get"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("get"),
	)
	PutRoute[ServiceAccount, UpdateServiceAccountInput](controlPlaneRouter, "/api/v1/_/core/service-accounts/{service_account_id}", kernel.handleUpdateServiceAccount,
		RouteTag("Core Control Plane"),
		RouteSummary("Update service account"),
		RouteDescription("Updates service account scopes, name, or enabled status while protecting root accounts."),
		RouteOperationID("core__service_accounts__update"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("update"),
	)
	DeleteRoute[Empty](controlPlaneRouter, "/api/v1/_/core/service-accounts/{service_account_id}", kernel.handleDeleteServiceAccount,
		RouteTag("Core Control Plane"),
		RouteSummary("Delete service account"),
		RouteDescription("Deletes a machine service account, enforcing invariant that at least one root account remains."),
		RouteNoContentResponse("Service account deleted"),
		RouteOperationID("core__service_accounts__delete"),
		RouteSDKGroupName("core", "serviceAccounts"),
		RouteSDKMethodName("delete"),
	)

	// Event Hooks Routes
	GetRoute[ListEventHooksResponse](controlPlaneRouter, "/api/v1/_/core/event-hooks", kernel.handleListEventHooks,
		RouteTag("Core Control Plane"),
		RouteSummary("List all event hooks"),
		RouteDescription("Lists all active event hooks with driver, target URLs/functions, events, and retry policies."),
		RouteOperationID("core__event_hooks__list"),
		RouteSDKGroupName("core", "eventHooks"),
		RouteSDKMethodName("list"),
	)
	PostRoute[EventHook, CreateEventHookInput](controlPlaneRouter, "/api/v1/_/core/event-hooks", kernel.handleCreateEventHook,
		RouteTag("Core Control Plane"),
		RouteSummary("Create a new event hook"),
		RouteDescription("Creates an event hook subscription for SQL stored procedure or HTTP webhook dispatching."),
		RouteDefaultStatusCode(http.StatusCreated),
		RouteOperationID("core__event_hooks__create"),
		RouteSDKGroupName("core", "eventHooks"),
		RouteSDKMethodName("create"),
	)
	GetRoute[EventHook](controlPlaneRouter, "/api/v1/_/core/event-hooks/{event_hook_id}", kernel.handleGetEventHook,
		RouteTag("Core Control Plane"),
		RouteSummary("Get event hook by ID"),
		RouteDescription("Retrieves an event hook by UUID."),
		RouteOperationID("core__event_hooks__get"),
		RouteSDKGroupName("core", "eventHooks"),
		RouteSDKMethodName("get"),
	)
	PutRoute[EventHook, UpdateEventHookInput](controlPlaneRouter, "/api/v1/_/core/event-hooks/{event_hook_id}", kernel.handleUpdateEventHook,
		RouteTag("Core Control Plane"),
		RouteSummary("Update event hook"),
		RouteDescription("Updates event hook driver, targets, subscribed events, secret, or enabled status."),
		RouteOperationID("core__event_hooks__update"),
		RouteSDKGroupName("core", "eventHooks"),
		RouteSDKMethodName("update"),
	)
	DeleteRoute[Empty](controlPlaneRouter, "/api/v1/_/core/event-hooks/{event_hook_id}", kernel.handleDeleteEventHook,
		RouteTag("Core Control Plane"),
		RouteSummary("Delete event hook"),
		RouteDescription("Deletes an event hook."),
		RouteNoContentResponse("Event hook deleted"),
		RouteOperationID("core__event_hooks__delete"),
		RouteSDKGroupName("core", "eventHooks"),
		RouteSDKMethodName("delete"),
	)
	GetRoute[ListEventHookDeliveriesResponse](controlPlaneRouter, "/api/v1/_/core/event-hooks/{event_hook_id}/deliveries", kernel.handleListEventHookDeliveries,
		RouteTag("Core Control Plane"),
		RouteSummary("List event hook deliveries"),
		RouteDescription("Queries recent dispatch attempts, response status, and latency for an event hook."),
		RouteOperationID("core__event_hooks__deliveries__list"),
		RouteSDKGroupName("core", "eventHooks", "deliveries"),
		RouteSDKMethodName("list"),
	)
	PostRoute[EventHookDelivery, Empty](controlPlaneRouter, "/api/v1/_/core/event-hooks/{event_hook_id}/deliveries/{delivery_id}/retry", kernel.handleRetryEventHookDelivery,
		RouteTag("Core Control Plane"),
		RouteSummary("Retry event hook delivery"),
		RouteDescription("Manually redrives a past event hook delivery attempt."),
		RouteOperationID("core__event_hooks__deliveries__retry"),
		RouteSDKGroupName("core", "eventHooks", "deliveries"),
		RouteSDKMethodName("retry"),
	)

	// Events Routes
	GetRoute[ListEventsResponse](controlPlaneRouter, "/api/v1/_/core/events", kernel.handleListEvents,
		RouteTag("Core Control Plane"),
		RouteSummary("List events"),
		RouteDescription("Queries immutable system and domain events with multi-field filtering and pagination."),
		RouteOperationID("core__events__list"),
		RouteSDKGroupName("core", "events"),
		RouteSDKMethodName("list"),
	)
	GetRoute[Event](controlPlaneRouter, "/api/v1/_/core/events/{event_id}", kernel.handleGetEvent,
		RouteTag("Core Control Plane"),
		RouteSummary("Get event by ID"),
		RouteDescription("Retrieves an individual event entry by UUID."),
		RouteOperationID("core__events__get"),
		RouteSDKGroupName("core", "events"),
		RouteSDKMethodName("get"),
	)

	// Mount Core Control Plane Router into root mux with Service Account authentication
	serveMux.Handle("/api/v1/_/core/", ServiceAccountAuthMiddleware(kernel.serviceAccountManager)(controlPlaneRouter.Mux()))
}

func (kernel *Kernel) writeJSON(responseWriter http.ResponseWriter, data any) {
	kernel.writeJSONWithStatus(responseWriter, http.StatusOK, data)
}

func (kernel *Kernel) writeJSONWithStatus(responseWriter http.ResponseWriter, statusCode int, data any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(data)
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
	stdlog.Printf("Initial console root account created:")
	stdlog.Printf("  Email:               %s", email)
	stdlog.Printf("  Password:            %s", plainPassword)
	stdlog.Printf("  Service Account Key: %s", createdServiceAccount.SecretKey)

	return nil
}
