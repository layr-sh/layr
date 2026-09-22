package core

import (
	"context"
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
	Stop()
	RegisterRoutes(baseRouter *Router, controlPlaneRouter *Router)
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
	kvStore               *KVStore
	cryptoKeyManager      *CryptoKeyManager
	jwtSigner             *JWTSigner
	serviceAccountManager *ServiceAccountManager
	eventBus              *EventBus
	eventManager          *EventManager
	eventHookManager      *EventHookManager
	node                  *Node
	server                *Server
	services              []ServiceRunner
	stopOnce              sync.Once
}

// NewKernel initializes the Layr Kernel with an established database pool and core subsystems.
func NewKernel(db *DatabasePool) (*Kernel, error) {
	log.Debugf("initializing Kernel instance")
	config := GetConfig()
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration invariant violation: %w", err)
	}

	cryptoKeyManager, _ := NewCryptoKeyManager(config.Security.MasterEncryptionKey) // Infallible: config.Validate() guarantees 32-byte hex key
	jwtSigner := NewJWTSigner(cryptoKeyManager)

	kernel := &Kernel{
		db:               db,
		cryptoKeyManager: cryptoKeyManager,
		jwtSigner:        jwtSigner,
	}

	if db != nil {
		kernel.serviceAccountManager = NewServiceAccountManager(db)
		kernel.eventBus = NewEventBus(db, cryptoKeyManager)
		kernel.eventManager = kernel.eventBus.EventManager()
		kernel.eventHookManager = kernel.eventBus.EventHookManager()
	}

	return kernel, nil
}

// PublishableKey returns the deterministic publishable key derived from the master encryption key.
func (kernel *Kernel) PublishableKey() string {
	return kernel.cryptoKeyManager.DerivePublishableKey()
}

// Start boots the database, applies migrations, launches heartbeats, and starts the HTTP gateway.
func (kernel *Kernel) Start(ctx context.Context) (err error) {
	log.Debugf("starting Kernel subsystem initialization")
	defer func() {
		if err != nil {
			kernel.Stop(ctx)
		}
	}()

	config := GetConfig()

	// Connect pgxpool if not provided upfront
	if kernel.db == nil {
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
		var connectErr error
		db, connectErr := NewDatabasePool(dbCtx, databaseURL, DatabasePoolOptions{
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
		if connectErr != nil {
			return fmt.Errorf("database connection failed: %w", connectErr)
		}
		kernel.db = db

		kernel.serviceAccountManager = NewServiceAccountManager(db)
		kernel.eventBus = NewEventBus(db, kernel.cryptoKeyManager)
		kernel.eventManager = kernel.eventBus.EventManager()
		kernel.eventHookManager = kernel.eventBus.EventHookManager()
	}

	// Run Core Migrations
	log.Infof("Executing foundational migrations...")
	allMigrations := GetRegisteredDatabaseMigrations()

	if migrationErr := kernel.db.RunMigrations(ctx, allMigrations); migrationErr != nil {
		return fmt.Errorf("database migration failed: %w", migrationErr)
	}

	// Register Node in Cluster
	nodeName, _ := os.Hostname()
	kernel.node = NewNode(kernel.db, nodeName, config.GetEnabledServices())
	_ = kernel.node.Register(ctx)

	// Initialize KV Store
	kvStore, err := NewKVStore(ctx, kernel.db)
	if err != nil {
		return fmt.Errorf("kv store initialization failed: %w", err)
	}
	kernel.kvStore = kvStore

	// Bootstrap Initial Root Account (Console User + Linked Service Account)
	if err := kernel.bootstrapRootAccount(ctx); err != nil {
		return fmt.Errorf("root account bootstrap failed: %w", err)
	}

	// Initialize HTTP Gateway
	kernel.server = NewServer(kernel)

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
		kernel.Stop(ctx)
		return nil
	case <-ctx.Done():
		log.Infof("Context cancelled, initiating graceful shutdown...")
		kernel.Stop(ctx)
		return nil
	}
}

// DB returns the database connection pool.
func (kernel *Kernel) DB() *DatabasePool {
	return kernel.db
}

// CryptoKeyManager returns the master key manager.
func (kernel *Kernel) CryptoKeyManager() *CryptoKeyManager {
	return kernel.cryptoKeyManager
}

// KVStore returns the pluggable key-value store.
func (kernel *Kernel) KVStore() *KVStore {
	return kernel.kvStore
}

// EventBus returns the platform event bus.
func (kernel *Kernel) EventBus() *EventBus {
	return kernel.eventBus
}

// EventManager returns the event manager.
func (kernel *Kernel) EventManager() *EventManager {
	return kernel.eventManager
}

// EventHookManager returns the event hook manager.
func (kernel *Kernel) EventHookManager() *EventHookManager {
	return kernel.eventHookManager
}

// JWTSigner returns the platform Ed25519 JWT signer.
func (kernel *Kernel) JWTSigner() *JWTSigner {
	return kernel.jwtSigner
}

// ServiceAccountManager returns the machine service account manager.
func (kernel *Kernel) ServiceAccountManager() *ServiceAccountManager {
	return kernel.serviceAccountManager
}

// Server returns the HTTP gateway server.
func (kernel *Kernel) Server() *Server {
	return kernel.server
}

// SetServer sets the HTTP gateway server on the kernel.
func (kernel *Kernel) SetServer(server *Server) {
	kernel.server = server
}

// ValidateSubsystems asserts that all required runtime modules on the Kernel are non-nil.
func (kernel *Kernel) ValidateSubsystems() error {
	if kernel == nil {
		return errors.New("kernel is nil")
	}
	if kernel.db == nil {
		return errors.New("kernel invariant violation: database pool is not initialized")
	}
	if kernel.cryptoKeyManager == nil {
		return errors.New("kernel invariant violation: crypto key manager is not initialized")
	}
	if kernel.node == nil {
		return errors.New("kernel invariant violation: node registry is not initialized")
	}
	if kernel.kvStore == nil {
		return errors.New("kernel invariant violation: kv store is not initialized")
	}
	if kernel.eventBus == nil {
		return errors.New("kernel invariant violation: event bus is not initialized")
	}
	if kernel.eventManager == nil {
		return errors.New("kernel invariant violation: event manager is not initialized")
	}
	if kernel.eventHookManager == nil {
		return errors.New("kernel invariant violation: event hook manager is not initialized")
	}
	if kernel.serviceAccountManager == nil {
		return errors.New("kernel invariant violation: service account manager is not initialized")
	}
	if kernel.jwtSigner == nil {
		return errors.New("kernel invariant violation: jwt signer is not initialized")
	}
	if kernel.server == nil {
		return errors.New("kernel invariant violation: server is not initialized")
	}
	return nil
}

// RegisterService attaches a modular service runner to the kernel.
func (kernel *Kernel) RegisterService(serviceRunner ServiceRunner) {
	kernel.services = append(kernel.services, serviceRunner)
}

// Stop gracefully terminates all subsystems.
func (kernel *Kernel) Stop(ctx context.Context) {
	kernel.stopOnce.Do(func() {
		log.Debugf("stopping Kernel services...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), defaultKernelShutdownTimeout)
		defer shutdownCancel()

		if kernel.server != nil {
			_ = kernel.server.Shutdown(shutdownCtx)
		}
		if kernel.node != nil {
			kernel.node.Close()
		}
		for i := len(kernel.services) - 1; i >= 0; i-- {
			kernel.services[i].Stop()
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
}

var defaultPasswordHasher = password.NewHasher()

func (kernel *Kernel) bootstrapRootAccount(ctx context.Context) error {
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
