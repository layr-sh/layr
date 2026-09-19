package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"layr.sh/core"
	"layr.sh/data/graphql"
	"layr.sh/data/realtime"
	"layr.sh/data/rest"
)

// BaseHandler coordinates all public data plane HTTP routes (REST, Ephemeral KV, GraphQL, and Realtime CDC).
type BaseHandler struct {
	db                    *core.DatabasePool
	configManager         *ConfigManager
	kvStore               *core.KVStore
	eventBus              *core.EventBus
	realtimeHub           *realtime.Hub
	graphqlCompiler       *graphql.Compiler
	schemaIntrospector    *graphql.SchemaIntrospector
	serviceAccountManager *core.ServiceAccountManager
	tablesRWMutex         sync.RWMutex
	tables                map[string]rest.TableMetadata // key: "schema.table"
	saltSecret            string
}

// Handler is an alias for BaseHandler, following layr.sh/auth conventions.
type Handler = BaseHandler

// NewBaseHandler initializes the BaseHandler with database pool and configuration manager.
func NewBaseHandler(db *core.DatabasePool, configManager *ConfigManager) *BaseHandler {
	log.Debug("initializing data base handler")
	schemaIntrospector := graphql.NewSchemaIntrospector(db)
	compiler := graphql.NewCompiler(schemaIntrospector, "public")

	hub := realtime.NewHub(db)

	baseHandler := &BaseHandler{
		db:                    db,
		configManager:         configManager,
		realtimeHub:           hub,
		graphqlCompiler:       compiler,
		schemaIntrospector:    schemaIntrospector,
		serviceAccountManager: core.NewServiceAccountManager(db),
		tables:                make(map[string]rest.TableMetadata),
	}

	return baseHandler
}

// NewHandler creates a new Data HTTP BaseHandler.
func NewHandler(db *core.DatabasePool, configManager *ConfigManager) *BaseHandler {
	return NewBaseHandler(db, configManager)
}

// SetKVStore attaches the key-value store for caching and ephemeral KV operations.
func (handler *BaseHandler) SetKVStore(kvStore *core.KVStore) {
	handler.kvStore = kvStore
	if handler.schemaIntrospector != nil {
		handler.schemaIntrospector.SetKVStore(kvStore)
	}
	if handler.realtimeHub != nil {
		handler.realtimeHub.SetKVStore(kvStore)
	}
}

// SetEventBus attaches the platform event bus for domain event dispatching.
func (handler *BaseHandler) SetEventBus(eventBus *core.EventBus) {
	handler.eventBus = eventBus
}

// SetRealtimeHub sets the realtime CDC hub.
func (handler *BaseHandler) SetRealtimeHub(hub *realtime.Hub) {
	handler.realtimeHub = hub
}

// SetSaltSecret configures the salt secret used for visitor hashing.
func (handler *BaseHandler) SetSaltSecret(saltSecret string) {
	handler.saltSecret = saltSecret
}

// SetServiceAccountManager attaches the service account manager for RLS bypass checking.
func (handler *BaseHandler) SetServiceAccountManager(serviceAccountManager *core.ServiceAccountManager) {
	handler.serviceAccountManager = serviceAccountManager
}

// SetTableMetadata registers cached table metadata for joins and primary keys.
func (handler *BaseHandler) SetTableMetadata(tableMetadata rest.TableMetadata) {
	handler.tablesRWMutex.Lock()
	defer handler.tablesRWMutex.Unlock()
	tableKey := fmt.Sprintf("%s.%s", tableMetadata.Schema, tableMetadata.Table)
	handler.tables[tableKey] = tableMetadata
}

// RealtimeHub returns the underlying Realtime CDC hub.
func (handler *BaseHandler) RealtimeHub() *realtime.Hub {
	return handler.realtimeHub
}

// GraphQLSchema returns the underlying GraphQL schema introspector.
func (handler *BaseHandler) GraphQLSchema() *graphql.SchemaIntrospector {
	return handler.schemaIntrospector
}

// IntrospectSchemas refreshes the GraphQL schema introspection.
func (handler *BaseHandler) IntrospectSchemas(ctx context.Context, schemas ...string) error {
	if handler.schemaIntrospector != nil && handler.db != nil {
		if len(schemas) == 0 && handler.configManager != nil {
			schemas = handler.configManager.Get().Schemas
		}
		if err := handler.schemaIntrospector.Introspect(ctx, schemas); err != nil {
			return fmt.Errorf("failed to introspect schemas: %w", err)
		}

		handler.tablesRWMutex.Lock()
		for _, tableInfo := range handler.schemaIntrospector.Tables() {
			columns := make([]string, 0, len(tableInfo.Columns))
			for columnName := range tableInfo.Columns {
				columns = append(columns, columnName)
			}
			foreignKeys := make(map[string]rest.RelationForeignKey, len(tableInfo.ForeignKeys))
			for relName, relInfo := range tableInfo.ForeignKeys {
				foreignKeys[relName] = rest.RelationForeignKey{
					FromColumn: relInfo.LocalColumn,
					ToTable:    relInfo.ForeignTable,
					ToColumn:   relInfo.TargetColumn,
				}
			}
			tableKey := fmt.Sprintf("%s.%s", tableInfo.Schema, tableInfo.Name)
			handler.tables[tableKey] = rest.TableMetadata{
				Schema:      tableInfo.Schema,
				Table:       tableInfo.Name,
				PrimaryKey:  tableInfo.PrimaryKey,
				Columns:     columns,
				ForeignKeys: foreignKeys,
			}
		}
		handler.tablesRWMutex.Unlock()
		return nil
	}
	return nil
}

// writeJSON writes a successful JSON response with status code.
func (handler *BaseHandler) writeJSON(responseWriter http.ResponseWriter, statusCode int, payload any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(payload)
}

// writeDBError maps PostgreSQL errors to appropriate RFC 9457 HTTP responses.
func (handler *BaseHandler) writeDBError(responseWriter http.ResponseWriter, request *http.Request, err error) {
	statusCode := http.StatusInternalServerError
	message := "Service temporarily unavailable"
	debugLog := err.Error()

	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "42501": // insufficient_privilege
			statusCode = http.StatusForbidden
			message = "Access denied"
		case "23505": // unique_violation
			statusCode = http.StatusConflict
			message = "Resource already exists"
		case "23503", "23001": // foreign_key_violation or restrict_violation
			statusCode = http.StatusConflict
			message = "Invalid reference"
		case "42P01", "42883": // undefined_table or undefined_function
			statusCode = http.StatusNotFound
			message = "Resource not found"
		case "42703": // undefined_column
			statusCode = http.StatusBadRequest
			message = "Invalid field specified"
		}
	}

	core.WriteErrorResponse(responseWriter, request, statusCode, message, debugLog)
}

// isRLSBypassed returns true if the request caller has the necessary service account scope to bypass RLS.
func (handler *BaseHandler) isRLSBypassed(request *http.Request, requiredScope string) bool {
	authContext := core.GetAuthContext(request.Context())
	if authContext.IsServiceAccount() && authContext.HasScope(requiredScope) {
		return true
	}
	if handler.serviceAccountManager == nil {
		return false
	}
	secretKey := core.ExtractRequestServiceAccountKey(request)
	if secretKey == "" {
		return false
	}
	clientIP := core.ExtractRequestClientIP(request)
	serviceAccount, err := handler.serviceAccountManager.Authenticate(request.Context(), secretKey, clientIP)
	if err != nil {
		return false
	}
	return core.HasScope(serviceAccount.Scopes, requiredScope)
}

// resolveTransaction commits or rolls back a transaction based on the error state.
func (handler *BaseHandler) resolveTransaction(ctx context.Context, tx pgx.Tx, err error) error {
	if tx == nil {
		return err
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

const (
	minPathSegments         = 2
	minRecordIDPathSegments = 3
)

// parsePath extracts schema, table, and optional record_id from the request URL.
func (handler *BaseHandler) parsePath(path string) (string, string, string, error) {
	trimmed := strings.TrimPrefix(path, "/api/v1/data/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < minPathSegments || parts[0] == "" || parts[1] == "" {
		return "", "", "", errors.New("path must include schema and table name")
	}
	recordID := ""
	if len(parts) >= minRecordIDPathSegments {
		recordID = parts[2]
	}
	return parts[0], parts[1], recordID, nil
}
