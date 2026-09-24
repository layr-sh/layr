package data

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"layr.sh/core"
	"layr.sh/data/graphql"
	"layr.sh/data/realtime"
	"layr.sh/data/referencedata"
	"layr.sh/data/rest"
)

func init() {
	core.RegisterServiceFactory("data", func(kernel *core.Kernel) (core.ServiceRunner, error) {
		return NewService(kernel), nil
	})
}

// Service encapsulates all layr/data engines (REST, GraphQL, Realtime CDC, Dynamic Config, Schema DDL).
type Service struct {
	kernel              *core.Kernel
	configManager       *ConfigManager
	ddlEngine           *DDLEngine
	realtimeHub         *realtime.Hub
	graphqlCompiler     *graphql.Compiler
	schemaIntrospector  *graphql.SchemaIntrospector
	tablesRWMutex       sync.RWMutex
	tables              map[string]rest.TableMetadata // key: "schema.table"
	saltSecret          string
	baseHandler         *BaseHandler
	controlPlaneHandler *ControlPlaneHandler
}

// NewService initializes the layr/data service coordinator.
func NewService(kernel *core.Kernel) *Service {
	configManager := NewConfigManager(kernel)
	ddlEngine := NewDDLEngine(kernel)
	schemaIntrospector := graphql.NewSchemaIntrospector(kernel)
	compiler := graphql.NewCompiler(schemaIntrospector, "public")
	hub := realtime.NewHub(kernel)

	service := &Service{
		kernel:             kernel,
		configManager:      configManager,
		ddlEngine:          ddlEngine,
		realtimeHub:        hub,
		graphqlCompiler:    compiler,
		schemaIntrospector: schemaIntrospector,
		tables:             make(map[string]rest.TableMetadata),
	}
	service.baseHandler = NewBaseHandler(service)
	service.controlPlaneHandler = NewControlPlaneHandler(service)

	return service
}

// Kernel returns the kernel instance.
func (service *Service) Kernel() *core.Kernel {
	return service.kernel
}

// SetSaltSecret updates the salt secret used for visitor hashing across data engines.
func (service *Service) SetSaltSecret(saltSecret string) {
	service.saltSecret = saltSecret
}

// SetTableMetadata registers cached table metadata for joins and primary keys.
func (service *Service) SetTableMetadata(tableMetadata rest.TableMetadata) {
	service.tablesRWMutex.Lock()
	defer service.tablesRWMutex.Unlock()
	tableKey := fmt.Sprintf("%s.%s", tableMetadata.Schema, tableMetadata.Table)
	service.tables[tableKey] = tableMetadata
}

// Start boots background data worker loops, listens to CDC triggers, and warms reflection caches.
func (service *Service) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before start: %w", err)
	}

	if err := service.configManager.Load(ctx); err != nil {
		return err
	}

	if err := referencedata.Seed(ctx, service.kernel); err != nil {
		log.Warnf("failed to seed reference data: %v", err)
	}

	catalogTTL := time.Duration(service.configManager.Get().Cache.CatalogTTLSeconds) * time.Second
	if catalogTTL <= 0 {
		catalogTTL = time.Hour
	}
	service.schemaIntrospector.SetCatalogTTL(catalogTTL)

	if service.configManager.Get().GraphQL.Enabled {
		_ = service.IntrospectSchemas(ctx)
	}

	if service.configManager.Get().Realtime.Enabled {
		service.realtimeHub.SetEventHandler(func(cdcEvent realtime.CDCEvent) {
			if service.configManager.Get().Cache.Enabled && service.configManager.Get().Cache.InvalidateOnCDC {
				service.InvalidateTableCache(ctx, cdcEvent.Schema, cdcEvent.Table)
			}
		})
		service.realtimeHub.Start(ctx)
	}

	return nil
}

// Stop gracefully shuts down active WebSocket connections and listeners.
func (service *Service) Stop() {
	service.realtimeHub.Stop()
}

// InvalidateCache evicts the specified cache keys from the KVStore and increments version numbers.
func (service *Service) InvalidateCache(ctx context.Context, invalidateCacheInput InvalidateCacheInput) {
	if invalidateCacheInput.All || invalidateCacheInput.Catalog {
		_ = service.kernel.KVStore().Delete(ctx, "cache:catalog")
		_ = service.kernel.KVStore().Delete(ctx, "cache:schema:catalog")
		_, _ = service.kernel.KVStore().Increment(ctx, "cache:v:global", 0)
		_ = service.kernel.KVStore().Delete(ctx, "cache:query_count")
	}
	if invalidateCacheInput.Schema != "" && invalidateCacheInput.Table != "" {
		_ = service.kernel.KVStore().Delete(ctx, "cache:"+invalidateCacheInput.Schema+"."+invalidateCacheInput.Table)
		_, _ = service.kernel.KVStore().Increment(ctx, fmt.Sprintf("cache:v:%s:%s", invalidateCacheInput.Schema, invalidateCacheInput.Table), 0)
	}
	if invalidateCacheInput.Pattern != "" {
		_ = service.kernel.KVStore().Delete(ctx, "cache:"+invalidateCacheInput.Pattern)
	}
}

// InvalidateCatalog purges the schema catalog from cache and triggers GraphQL introspection reload.
func (service *Service) InvalidateCatalog(ctx context.Context) {
	service.InvalidateCache(ctx, InvalidateCacheInput{Catalog: true, All: true})
	_ = service.schemaIntrospector.Introspect(ctx, service.configManager.Get().Schemas)
}

// InvalidateTableCache invalidates cached query results for a specific table.
func (service *Service) InvalidateTableCache(ctx context.Context, schema, table string) {
	cacheConfig := service.configManager.Get().Cache
	if !cacheConfig.Enabled || !cacheConfig.InvalidateOnMutation {
		return
	}
	versionKey := fmt.Sprintf("cache:v:%s:%s", schema, table)
	newVersion, err := service.kernel.KVStore().Increment(ctx, versionKey, 0)
	const maxSafeVersion int64 = 9_000_000_000_000_000
	if err != nil || newVersion >= maxSafeVersion {
		_ = service.kernel.KVStore().Set(ctx, versionKey, "1", 0)
	}
}

// ResetTableCacheVersion explicitly resets the table cache version back to zero.
func (service *Service) ResetTableCacheVersion(ctx context.Context, schema, table string) {
	versionKey := fmt.Sprintf("cache:v:%s:%s", schema, table)
	_ = service.kernel.KVStore().Delete(ctx, versionKey)
}

func (service *Service) getTableCacheVersion(ctx context.Context, schema, table string) int64 {
	versionString, getErr := service.kernel.KVStore().Get(ctx, fmt.Sprintf("cache:v:%s:%s", schema, table))
	if getErr != nil || versionString == "" {
		return 0
	}
	versionNumber, parseErr := strconv.ParseInt(versionString, 10, 64)
	if parseErr != nil {
		return 0
	}
	return versionNumber
}

// DDLEngine returns the underlying schema DDL engine.
func (service *Service) DDLEngine() *DDLEngine {
	return service.ddlEngine
}

// ConfigManager returns the dynamic config manager.
func (service *Service) ConfigManager() *ConfigManager {
	return service.configManager
}

// GetConfigManager returns the dynamic config manager.
func (service *Service) GetConfigManager() *ConfigManager {
	return service.configManager
}

// ControlPlaneHandler returns the control plane handler.
func (service *Service) ControlPlaneHandler() *ControlPlaneHandler {
	return service.controlPlaneHandler
}

// GetControlPlaneHandler returns the control plane handler.
func (service *Service) GetControlPlaneHandler() *ControlPlaneHandler {
	return service.controlPlaneHandler
}

// BaseHandler returns the public base HTTP handler.
func (service *Service) BaseHandler() *BaseHandler {
	return service.baseHandler
}

// RealtimeHub returns the Realtime CDC hub.
func (service *Service) RealtimeHub() *realtime.Hub {
	return service.realtimeHub
}

// GraphQLSchema returns the GraphQL schema introspector.
func (service *Service) GraphQLSchema() *graphql.SchemaIntrospector {
	return service.schemaIntrospector
}

// IntrospectSchemas refreshes the GraphQL schema introspection.
func (service *Service) IntrospectSchemas(ctx context.Context, schemas ...string) error {
	if len(schemas) == 0 {
		schemas = service.configManager.Get().Schemas
	}
	if err := service.schemaIntrospector.Introspect(ctx, schemas); err != nil {
		return fmt.Errorf("failed to introspect schemas: %w", err)
	}

	service.tablesRWMutex.Lock()
	for _, tableInfo := range service.schemaIntrospector.Tables() {
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
		service.tables[tableKey] = rest.TableMetadata{
			Schema:      tableInfo.Schema,
			Table:       tableInfo.Name,
			PrimaryKey:  tableInfo.PrimaryKey,
			Columns:     columns,
			ForeignKeys: foreignKeys,
		}
	}
	service.tablesRWMutex.Unlock()
	return nil
}

// IsRESTEnabled returns whether REST API is enabled.
func (service *Service) IsRESTEnabled() bool {
	return service.configManager.Get().REST.Enabled
}

// GetRESTMaxLimit returns the max limit for REST queries.
func (service *Service) GetRESTMaxLimit() int {
	return service.configManager.Get().REST.MaxLimit
}

// GetRESTDefaultLimit returns the default limit for REST queries.
func (service *Service) GetRESTDefaultLimit() int {
	return service.configManager.Get().REST.DefaultLimit
}

// GetExcludedTables returns the list of excluded tables.
func (service *Service) GetExcludedTables() []string {
	return service.configManager.Get().REST.ExcludedTables
}

// IsGraphQLEnabled returns whether GraphQL API is enabled.
func (service *Service) IsGraphQLEnabled() bool {
	return service.configManager.Get().GraphQL.Enabled
}

// GetGraphQLMaxDepth returns the max depth for GraphQL queries.
func (service *Service) GetGraphQLMaxDepth() int {
	return service.configManager.Get().GraphQL.MaxDepth
}

// IsRealtimeEnabled returns whether Realtime CDC is enabled.
func (service *Service) IsRealtimeEnabled() bool {
	return service.configManager.Get().Realtime.Enabled
}

// GetRealtimeHeartbeatIntervalMS returns heartbeat interval in milliseconds.
func (service *Service) GetRealtimeHeartbeatIntervalMS() int {
	return service.configManager.Get().Realtime.HeartbeatIntervalMS
}

// GetRealtimeMaxChannels returns the max channels per connection.
func (service *Service) GetRealtimeMaxChannels() int {
	return service.configManager.Get().Realtime.MaxChannelsPerConnection
}

// GetAllowedSchemas returns the configured schemas exposed to data operations.
func (service *Service) GetAllowedSchemas() []string {
	return service.configManager.Get().Schemas
}

// GetAllowedOrigins returns the configured CORS allowed origins.
func (service *Service) GetAllowedOrigins() []string {
	return service.configManager.Get().CORS.AllowedOrigins
}

// GetRESTHandler returns the base handler handling REST operations.
func (service *Service) GetRESTHandler() *BaseHandler {
	return service.baseHandler
}

// GetGraphQLHandler returns the base handler handling GraphQL operations.
func (service *Service) GetGraphQLHandler() *BaseHandler {
	return service.baseHandler
}

// GetRealtimeHandler returns the base handler handling Realtime CDC operations.
func (service *Service) GetRealtimeHandler() *BaseHandler {
	return service.baseHandler
}

// GetRealtimeHub returns the realtime CDC hub.
func (service *Service) GetRealtimeHub() *realtime.Hub {
	return service.realtimeHub
}

// handleFlushCache handles cache flush via the control plane handler.
func (service *Service) handleFlushCache(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling flush data cache request")
	service.controlPlaneHandler.handleFlushCache(responseWriter, request)
}

// handleInvalidateCache handles cache invalidation via the control plane handler.
func (service *Service) handleInvalidateCache(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling invalidate data cache request")
	service.controlPlaneHandler.handleInvalidateCache(responseWriter, request)
}
