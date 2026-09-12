package auth

import (
	"context"
	"fmt"
	"net/http"

	"layr.sh/core"
)

func init() {
	core.RegisterServiceFactory("auth", func(kernel *core.Kernel) (core.ServiceRunner, error) {
		service := NewService(kernel.DB(), kernel.CryptoKeyManager())
		if kvStore := kernel.KVStore(); kvStore != nil {
			service.SetKVStore(kvStore)
		}
		if serviceAccountManager := kernel.ServiceAccountManager(); serviceAccountManager != nil {
			service.SetServiceAccountManager(serviceAccountManager)
		}
		if eventBus := kernel.EventBus(); eventBus != nil {
			service.SetEventBus(eventBus)
		}
		return service, nil
	})
}

// Service coordinates all authentication engines (OIDC, passwords, passkeys, OTP, OAuth).
type Service struct {
	db                    *core.DatabasePool
	cryptoKeyManager      *core.CryptoKeyManager
	configManager         *ConfigManager
	handler               *Handler
	controlPlaneHandler   *ControlPlaneHandler
	kvStore               core.KVStore
	serviceAccountManager *core.ServiceAccountManager
	eventBus              *core.EventBus
}

// NewService initializes the layr/auth service.
func NewService(db *core.DatabasePool, cryptoKeyManager *core.CryptoKeyManager) *Service {
	configManager := NewConfigManager(db, cryptoKeyManager)
	controlPlaneHandler := NewControlPlaneHandler(db, configManager)
	var handler *Handler
	if cryptoKeyManager != nil {
		handler = NewHandler(db, configManager, cryptoKeyManager)
	}
	return &Service{
		db:                  db,
		cryptoKeyManager:    cryptoKeyManager,
		configManager:       configManager,
		controlPlaneHandler: controlPlaneHandler,
		handler:             handler,
	}
}

// SetKVStore configures the pluggable KVStore for the auth service.
func (service *Service) SetKVStore(kvStore core.KVStore) {
	service.kvStore = kvStore
	if service.controlPlaneHandler != nil {
		service.controlPlaneHandler.SetKVStore(kvStore)
	}
	if service.handler != nil {
		service.handler.SetKVStore(kvStore)
	}
}

// SetServiceAccountManager sets the service account manager for the auth service.
func (service *Service) SetServiceAccountManager(serviceAccountManager *core.ServiceAccountManager) {
	service.serviceAccountManager = serviceAccountManager
	if service.configManager != nil {
		service.configManager.SetServiceAccountManager(serviceAccountManager)
	}
	if service.controlPlaneHandler != nil {
		service.controlPlaneHandler.SetServiceAccountManager(serviceAccountManager)
	}
	if service.handler != nil {
		service.handler.SetServiceAccountManager(serviceAccountManager)
	}
}

// SetEventBus sets the platform event bus for broadcasting auth events.
func (service *Service) SetEventBus(eventBus *core.EventBus) {
	service.eventBus = eventBus
	if service.configManager != nil {
		service.configManager.SetEventBus(eventBus)
	}
	if service.controlPlaneHandler != nil {
		service.controlPlaneHandler.SetEventBus(eventBus)
	}
	if service.handler != nil {
		service.handler.SetEventBus(eventBus)
	}
}

// CheckScope verifies if the request has the required scope permission.
func (service *Service) CheckScope(request *http.Request, requiredScope string) bool {
	if service.serviceAccountManager == nil {
		return true
	}
	secretKey := core.ExtractRequestServiceAccountKey(request)
	if secretKey == "" {
		return true
	}
	clientIP := core.ExtractRequestClientIP(request)
	serviceAccount, err := service.serviceAccountManager.Authenticate(request.Context(), secretKey, clientIP)
	if err != nil {
		return false
	}
	return core.HasScope(serviceAccount.Scopes, requiredScope)
}

// Start loads dynamic configuration and initializes HTTP handlers.
func (service *Service) Start(ctx context.Context) error {
	if err := service.configManager.Load(ctx); err != nil {
		return fmt.Errorf("failed to load auth config: %w", err)
	}

	if service.cryptoKeyManager != nil {
		service.handler = NewHandler(service.db, service.configManager, service.cryptoKeyManager)
		if service.kvStore != nil {
			service.handler.SetKVStore(service.kvStore)
		}
		if service.serviceAccountManager != nil {
			service.handler.SetServiceAccountManager(service.serviceAccountManager)
		}
		if service.eventBus != nil {
			service.handler.SetEventBus(service.eventBus)
		}
	}
	return nil
}

// Stop terminates active service routines.
func (service *Service) Stop() error {
	return nil
}

// GetConfigManager returns the dynamic config manager.
func (service *Service) GetConfigManager() *ConfigManager {
	return service.configManager
}

// GetHandler returns the active auth handler.
func (service *Service) GetHandler() *Handler {
	return service.handler
}

// GetControlPlaneHandler returns the control plane handler.
func (service *Service) GetControlPlaneHandler() *ControlPlaneHandler {
	return service.controlPlaneHandler
}
