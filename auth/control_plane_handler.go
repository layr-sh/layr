package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"layr.sh/auth/password"
	"layr.sh/core"
)

// ControlPlaneHandler exposes protected user and session management endpoints for the control plane.
type ControlPlaneHandler struct {
	db                    *core.DatabasePool
	configManager         *ConfigManager
	hasher                *password.Hasher
	kvStore               core.KVStore
	serviceAccountManager *core.ServiceAccountManager
	eventBus              *core.EventBus
}

// NewControlPlaneHandler creates a control plane handler for auth endpoints.
func NewControlPlaneHandler(db *core.DatabasePool, configManager *ConfigManager) *ControlPlaneHandler {
	return &ControlPlaneHandler{
		db:            db,
		configManager: configManager,
		hasher:        password.NewHasher(),
	}
}

// SetKVStore sets the KV store for session cache invalidation.
func (controlPlaneHandler *ControlPlaneHandler) SetKVStore(kvStore core.KVStore) {
	controlPlaneHandler.kvStore = kvStore
}

// SetServiceAccountManager sets the service account manager for scope authorization.
func (controlPlaneHandler *ControlPlaneHandler) SetServiceAccountManager(serviceAccountManager *core.ServiceAccountManager) {
	controlPlaneHandler.serviceAccountManager = serviceAccountManager
}

// SetEventBus sets the system event bus.
func (controlPlaneHandler *ControlPlaneHandler) SetEventBus(eventBus *core.EventBus) {
	controlPlaneHandler.eventBus = eventBus
}

// SetHasher sets the password hasher.
func (controlPlaneHandler *ControlPlaneHandler) SetHasher(hasher *password.Hasher) {
	controlPlaneHandler.hasher = hasher
}

// checkScope verifies if the incoming request satisfies the required permission scope.
func (controlPlaneHandler *ControlPlaneHandler) checkScope(request *http.Request, requiredScope string) bool {
	if controlPlaneHandler.serviceAccountManager == nil {
		return true
	}
	secretKey := core.ExtractRequestServiceAccountKey(request)
	if secretKey == "" {
		return true
	}
	clientIP := core.ExtractRequestClientIP(request)
	serviceAccount, err := controlPlaneHandler.serviceAccountManager.Authenticate(request.Context(), secretKey, clientIP)
	if err != nil {
		return false
	}
	return core.HasScope(serviceAccount.Scopes, requiredScope)
}

func (controlPlaneHandler *ControlPlaneHandler) writeJSON(responseWriter http.ResponseWriter, statusCode int, payload any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(payload)
}

func (controlPlaneHandler *ControlPlaneHandler) writeError(responseWriter http.ResponseWriter, request *http.Request, statusCode int, message string, errorCode string) {
	core.WriteErrorResponse(responseWriter, request, statusCode, message, errorCode)
}

func (controlPlaneHandler *ControlPlaneHandler) extractUserID(request *http.Request) string {
	userID := request.PathValue("user_id")
	if userID != "" {
		return userID
	}
	pathSegments := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(pathSegments) >= 6 && pathSegments[4] == "users" {
		return pathSegments[5]
	}
	return ""
}
