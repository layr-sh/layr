package filestorage

import (
	"context"
	"fmt"

	"layr.sh/core"
)

func init() {
	factory := func(kernel *core.Kernel) (core.ServiceRunner, error) {
		service := NewService(kernel.DB(), kernel.CryptoKeyManager())
		service.SetKVStore(kernel.KVStore())
		service.SetEventBus(kernel.EventBus())
		service.SetServiceAccountManager(kernel.ServiceAccountManager())
		return service, nil
	}

	core.RegisterServiceFactory("file_storage", factory)
	core.RegisterServiceFactory("filestorage", factory)
}

// Service encapsulates the File Storage service coordinator and handlers.
type Service struct {
	db                    *core.DatabasePool
	cryptoKeyManager      *core.CryptoKeyManager
	configManager         *ConfigManager
	baseHandler           *BaseHandler
	controlPlaneHandler   *ControlPlaneHandler
	serviceAccountManager *core.ServiceAccountManager
	eventBus              *core.EventBus
	kvStore               *core.KVStore
}

// NewService initializes the File Storage service coordinator.
func NewService(db *core.DatabasePool, cryptoKeyManager *core.CryptoKeyManager) *Service {
	configManager := NewConfigManager(db)
	baseHandler := NewBaseHandler(db, configManager, cryptoKeyManager)
	controlPlaneHandler := NewControlPlaneHandler(db, configManager, cryptoKeyManager)

	return &Service{
		db:                  db,
		cryptoKeyManager:    cryptoKeyManager,
		configManager:       configManager,
		baseHandler:         baseHandler,
		controlPlaneHandler: controlPlaneHandler,
	}
}

// SetServiceAccountManager attaches the service account manager.
func (service *Service) SetServiceAccountManager(serviceAccountManager *core.ServiceAccountManager) {
	service.serviceAccountManager = serviceAccountManager
	if service.baseHandler != nil {
		service.baseHandler.SetServiceAccountManager(serviceAccountManager)
	}
	if service.controlPlaneHandler != nil {
		service.controlPlaneHandler.SetServiceAccountManager(serviceAccountManager)
	}
}

// SetEventBus attaches the platform event bus.
func (service *Service) SetEventBus(eventBus *core.EventBus) {
	service.eventBus = eventBus
	if service.baseHandler != nil {
		service.baseHandler.SetEventBus(eventBus)
	}
	if service.controlPlaneHandler != nil {
		service.controlPlaneHandler.SetEventBus(eventBus)
	}
}

// SetKVStore attaches the pluggable KVStore instance.
func (service *Service) SetKVStore(kvStore *core.KVStore) {
	service.kvStore = kvStore
	if service.baseHandler != nil {
		service.baseHandler.SetKVStore(kvStore)
	}
	if service.controlPlaneHandler != nil {
		service.controlPlaneHandler.SetKVStore(kvStore)
	}
}

// Start loads runtime configuration from PostgreSQL.
func (service *Service) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before start: %w", err)
	}

	if service.configManager != nil && service.db != nil {
		if err := service.configManager.Load(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Stop gracefully shuts down active workers or resources.
func (service *Service) Stop() error {
	return nil
}

// BaseHandler returns the underlying public HTTP base handler.
func (service *Service) BaseHandler() *BaseHandler {
	return service.baseHandler
}

// ControlPlaneHandler returns the underlying administrative control plane handler.
func (service *Service) ControlPlaneHandler() *ControlPlaneHandler {
	return service.controlPlaneHandler
}

// ConfigManager returns the dynamic runtime configuration manager.
func (service *Service) ConfigManager() *ConfigManager {
	return service.configManager
}
