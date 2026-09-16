package data

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"layr.sh/core"
)

// ControlPlaneHandler handles control plane endpoints under /api/v1/_/data/*.
type ControlPlaneHandler struct {
	ddlEngine             *DDLEngine
	service               *Service
	configManager         *ConfigManager
	serviceAccountManager *core.ServiceAccountManager
	eventBus              *core.EventBus
	kvStore               core.KVStore
}

// NewControlPlaneHandler creates an HTTP handler for data control plane management.
func NewControlPlaneHandler(ddlEngine *DDLEngine, service *Service, configManager *ConfigManager) *ControlPlaneHandler {
	return &ControlPlaneHandler{
		ddlEngine:     ddlEngine,
		service:       service,
		configManager: configManager,
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
func (controlPlaneHandler *ControlPlaneHandler) SetKVStore(kvStore core.KVStore) {
	controlPlaneHandler.kvStore = kvStore
}

func (controlPlaneHandler *ControlPlaneHandler) checkScope(request *http.Request, requiredScope string) bool {
	if controlPlaneHandler.serviceAccountManager == nil {
		if controlPlaneHandler.service != nil && controlPlaneHandler.service.serviceAccountManager != nil {
			controlPlaneHandler.serviceAccountManager = controlPlaneHandler.service.serviceAccountManager
		} else {
			return true
		}
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

func (controlPlaneHandler *ControlPlaneHandler) invalidateCache(ctx context.Context) {
	if controlPlaneHandler.service != nil {
		controlPlaneHandler.service.InvalidateCatalog(ctx)
	}
}

func (controlPlaneHandler *ControlPlaneHandler) invalidateTableCache(ctx context.Context, schema, table string) {
	if controlPlaneHandler.service != nil {
		controlPlaneHandler.service.InvalidateTableCache(ctx, schema, table)
	}
}

func (controlPlaneHandler *ControlPlaneHandler) writeJSON(responseWriter http.ResponseWriter, status int, data any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	_ = json.NewEncoder(responseWriter).Encode(data)
}

func (controlPlaneHandler *ControlPlaneHandler) writeError(responseWriter http.ResponseWriter, request *http.Request, status int, detail string) {
	core.WriteErrorResponse(responseWriter, request, status, detail, "LAYR_DATA_001")
}

func (controlPlaneHandler *ControlPlaneHandler) writeForbidden(responseWriter http.ResponseWriter, request *http.Request) {
	core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation", "LAYR_DATA_003")
}

func (controlPlaneHandler *ControlPlaneHandler) extractSchemaAndTable(request *http.Request) (string, string) {
	schema := request.PathValue("schema_name")
	table := request.PathValue("table_name")
	if schema != "" && table != "" {
		return schema, table
	}
	path := strings.TrimPrefix(request.URL.Path, "/api/v1/_/data/tables")
	path = strings.TrimPrefix(path, "/api/v1/_/data")
	cleaned := strings.Trim(path, "/")
	if cleaned == "" {
		return schema, table
	}
	const (
		minSegments = 1
		twoSegments = 2
	)
	parts := strings.Split(cleaned, "/")
	if len(parts) >= minSegments && schema == "" {
		schema = parts[0]
	}
	if len(parts) >= twoSegments && table == "" {
		table = parts[1]
	}
	return schema, table
}

func (controlPlaneHandler *ControlPlaneHandler) extractColumnName(request *http.Request) string {
	columnName := request.PathValue("column_name")
	if columnName != "" {
		return columnName
	}
	path := strings.TrimPrefix(request.URL.Path, "/api/v1/_/data/tables")
	path = strings.TrimPrefix(path, "/api/v1/_/data")
	cleaned := strings.Trim(path, "/")
	if cleaned == "" {
		return ""
	}
	const (
		requiredSegments    = 4
		segmentActionIndex  = 2
		segmentElementIndex = 3
	)
	parts := strings.Split(cleaned, "/")
	if len(parts) >= requiredSegments && parts[segmentActionIndex] == "columns" {
		return parts[segmentElementIndex]
	}
	return ""
}

func (controlPlaneHandler *ControlPlaneHandler) extractIndexName(request *http.Request) string {
	indexName := request.PathValue("index_name")
	if indexName != "" {
		return indexName
	}
	path := strings.TrimPrefix(request.URL.Path, "/api/v1/_/data/tables")
	path = strings.TrimPrefix(path, "/api/v1/_/data")
	cleaned := strings.Trim(path, "/")
	if cleaned == "" {
		return ""
	}
	const (
		requiredSegments    = 4
		segmentActionIndex  = 2
		segmentElementIndex = 3
	)
	parts := strings.Split(cleaned, "/")
	if len(parts) >= requiredSegments && parts[segmentActionIndex] == "indexes" {
		return parts[segmentElementIndex]
	}
	return ""
}

func (controlPlaneHandler *ControlPlaneHandler) extractPolicyName(request *http.Request) string {
	policyName := request.PathValue("policy_name")
	if policyName != "" {
		return policyName
	}
	path := strings.TrimPrefix(request.URL.Path, "/api/v1/_/data/tables")
	path = strings.TrimPrefix(path, "/api/v1/_/data")
	cleaned := strings.Trim(path, "/")
	if cleaned == "" {
		return ""
	}
	const (
		requiredSegments    = 4
		segmentActionIndex  = 2
		segmentElementIndex = 3
	)
	parts := strings.Split(cleaned, "/")
	if len(parts) >= requiredSegments && parts[segmentActionIndex] == "policies" {
		return parts[segmentElementIndex]
	}
	return ""
}
