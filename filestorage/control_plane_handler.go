package filestorage

import (
	"encoding/json"
	"net/http"

	"layr.sh/core"
)

// ControlPlaneHandler coordinates all administrative control plane HTTP routes under /v1/_/file-storage/*.
type ControlPlaneHandler struct {
	db                    *core.DatabasePool
	configManager         *ConfigManager
	serviceAccountManager *core.ServiceAccountManager
	cryptoKeyManager      *core.CryptoKeyManager
	eventBus              *core.EventBus
	kvStore               *core.KVStore
}

// NewControlPlaneHandler creates a new ControlPlaneHandler instance.
func NewControlPlaneHandler(
	db *core.DatabasePool,
	configManager *ConfigManager,
	cryptoKeyManager *core.CryptoKeyManager,
) *ControlPlaneHandler {
	return &ControlPlaneHandler{
		db:                    db,
		configManager:         configManager,
		cryptoKeyManager:      cryptoKeyManager,
		serviceAccountManager: core.NewServiceAccountManager(db),
	}
}

// SetServiceAccountManager wires the service account manager for scope authorization.
func (controlPlaneHandler *ControlPlaneHandler) SetServiceAccountManager(serviceAccountManager *core.ServiceAccountManager) {
	controlPlaneHandler.serviceAccountManager = serviceAccountManager
}

// SetEventBus wires the event bus for lifecycle events.
func (controlPlaneHandler *ControlPlaneHandler) SetEventBus(eventBus *core.EventBus) {
	controlPlaneHandler.eventBus = eventBus
}

// SetKVStore attaches the KV store reference.
func (controlPlaneHandler *ControlPlaneHandler) SetKVStore(kvStore *core.KVStore) {
	controlPlaneHandler.kvStore = kvStore
}

func (controlPlaneHandler *ControlPlaneHandler) checkScope(request *http.Request, requiredScope string) bool {
	authContext := core.GetAuthContext(request.Context())
	if authContext.IsServiceAccount() {
		return authContext.HasScope(requiredScope)
	}

	if controlPlaneHandler.serviceAccountManager == nil {
		return true
	}

	serviceAccountKey := core.ExtractRequestServiceAccountKey(request)
	if serviceAccountKey == "" {
		return true
	}

	clientIP := core.ExtractRequestClientIP(request)
	serviceAccount, authErr := controlPlaneHandler.serviceAccountManager.Authenticate(request.Context(), serviceAccountKey, clientIP)
	if authErr != nil {
		return false
	}

	return core.HasScope(serviceAccount.Scopes, requiredScope)
}

func (controlPlaneHandler *ControlPlaneHandler) writeJSON(responseWriter http.ResponseWriter, statusCode int, payload any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(payload)
}
