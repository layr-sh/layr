package data

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"layr.sh/core"
	"layr.sh/data/graphql"
	"layr.sh/data/realtime"
	"layr.sh/data/referencedata"
)

func init() {
	core.RegisterServiceFactory("data", func(kernel *core.Kernel) (core.ServiceRunner, error) {
		service := NewService(kernel.DB())
		service.SetKVStore(kernel.KVStore())
		service.SetEventBus(kernel.EventBus())
		service.SetServiceAccountManager(kernel.ServiceAccountManager())
		return service, nil
	})
}

// Service encapsulates all layr/data engines (REST, GraphQL, Realtime CDC, Dynamic Config, Schema DDL).
type Service struct {
	db                    *core.DatabasePool
	kvStore               *core.KVStore
	configManager         *ConfigManager
	baseHandler           *BaseHandler
	controlPlaneHandler   *ControlPlaneHandler
	ddlEngine             *DDLEngine
	serviceAccountManager *core.ServiceAccountManager
	eventBus              *core.EventBus
}

// NewService initializes the layr/data service coordinator.
func NewService(db *core.DatabasePool) *Service {
	configManager := NewConfigManager(db)
	baseHandler := NewBaseHandler(db, configManager)
	ddlEngine := NewDDLEngine(db)

	service := &Service{
		db:                    db,
		configManager:         configManager,
		baseHandler:           baseHandler,
		ddlEngine:             ddlEngine,
		controlPlaneHandler:   NewControlPlaneHandler(ddlEngine, nil, configManager),
		serviceAccountManager: core.NewServiceAccountManager(db),
	}
	service.controlPlaneHandler.service = service

	return service
}

// SetServiceAccountManager sets the service account manager for the data service.
func (service *Service) SetServiceAccountManager(serviceAccountManager *core.ServiceAccountManager) {
	service.serviceAccountManager = serviceAccountManager
	if service.controlPlaneHandler != nil {
		service.controlPlaneHandler.SetServiceAccountManager(serviceAccountManager)
	}
	if service.baseHandler != nil {
		service.baseHandler.SetServiceAccountManager(serviceAccountManager)
	}
}

// SetEventBus sets the platform event bus for broadcasting data lifecycle events.
func (service *Service) SetEventBus(eventBus *core.EventBus) {
	service.eventBus = eventBus
	if service.controlPlaneHandler != nil {
		service.controlPlaneHandler.SetEventBus(eventBus)
	}
	if service.baseHandler != nil {
		service.baseHandler.SetEventBus(eventBus)
	}
}

// CheckScope verifies if the request has the required scope permission.
func (service *Service) CheckScope(request *http.Request, requiredScope string) bool {
	return service.controlPlaneHandler.checkScope(request, requiredScope)
}

// SetKVStore attaches the pluggable KVStore instance across all data engines.
func (service *Service) SetKVStore(kvStore *core.KVStore) {
	service.kvStore = kvStore
	if service.baseHandler != nil {
		service.baseHandler.SetKVStore(kvStore)
	}
	if service.controlPlaneHandler != nil {
		service.controlPlaneHandler.SetKVStore(kvStore)
	}
}

// SetSaltSecret updates the salt secret used for visitor hashing across data engines.
func (service *Service) SetSaltSecret(saltSecret string) {
	if service.baseHandler != nil {
		service.baseHandler.SetSaltSecret(saltSecret)
	}
}

// Start boots background data worker loops, listens to CDC triggers, and warms reflection caches.
func (service *Service) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before start: %w", err)
	}

	if service.configManager != nil && service.db != nil {
		if err := service.configManager.Load(ctx); err != nil {
			return err
		}
	}

	if service.db != nil {
		if err := referencedata.Seed(ctx, service.db); err != nil {
			log.Warnf("failed to seed reference data: %v", err)
		}
	}

	if service.baseHandler != nil && service.baseHandler.GraphQLSchema() != nil {
		catalogTTL := time.Duration(service.configManager.Get().Cache.CatalogTTLSeconds) * time.Second
		if catalogTTL <= 0 {
			catalogTTL = time.Hour
		}
		service.baseHandler.GraphQLSchema().SetCatalogTTL(catalogTTL)
	}

	if service.configManager.Get().GraphQL.Enabled {
		_ = service.IntrospectSchemas(ctx)
	}

	if service.configManager.Get().Realtime.Enabled && service.baseHandler != nil && service.baseHandler.RealtimeHub() != nil {
		log.Infof("starting PostgreSQL CDC listener on channel %q", CDCNotificationChannel)
		service.baseHandler.RealtimeHub().SetEventHandler(func(cdcEvent realtime.CDCEvent) {
			if service.configManager.Get().Cache.Enabled && service.configManager.Get().Cache.InvalidateOnCDC && service.baseHandler != nil {
				service.baseHandler.InvalidateTableCache(ctx, cdcEvent.Schema, cdcEvent.Table)
			}
		})
		if err := service.baseHandler.RealtimeHub().Start(ctx); err != nil {
			log.Warnf("failed to start CDC listener: %v", err)
		}
	}

	return nil
}

// Stop gracefully shuts down active WebSocket connections and listeners.
func (service *Service) Stop() error {
	if service.baseHandler != nil && service.baseHandler.RealtimeHub() != nil {
		service.baseHandler.RealtimeHub().Stop()
	}
	return nil
}

// InvalidateCache evicts the specified cache keys from the KVStore and increments version numbers.
func (service *Service) InvalidateCache(ctx context.Context, invalidateCacheInput InvalidateCacheInput) {
	if service.kvStore == nil {
		return
	}
	if invalidateCacheInput.All || invalidateCacheInput.Catalog {
		_ = service.kvStore.Delete(ctx, "cache:catalog")
		_ = service.kvStore.Delete(ctx, "cache:schema:catalog")
		_, _ = service.kvStore.Increment(ctx, "cache:v:global", 0)
		_ = service.kvStore.Delete(ctx, "cache:query_count")
	}
	if invalidateCacheInput.Schema != "" && invalidateCacheInput.Table != "" {
		_ = service.kvStore.Delete(ctx, "cache:"+invalidateCacheInput.Schema+"."+invalidateCacheInput.Table)
		_, _ = service.kvStore.Increment(ctx, fmt.Sprintf("cache:v:%s:%s", invalidateCacheInput.Schema, invalidateCacheInput.Table), 0)
	}
	if invalidateCacheInput.Pattern != "" {
		_ = service.kvStore.Delete(ctx, "cache:"+invalidateCacheInput.Pattern)
	}
}

// InvalidateCatalog purges the schema catalog from cache and triggers GraphQL introspection reload.
func (service *Service) InvalidateCatalog(ctx context.Context) {
	service.InvalidateCache(ctx, InvalidateCacheInput{Catalog: true, All: true})
	if service.baseHandler != nil && service.baseHandler.GraphQLSchema() != nil && service.db != nil {
		_ = service.baseHandler.GraphQLSchema().Introspect(ctx, service.configManager.Get().Schemas)
	}
}

// InvalidateTableCache invalidates cached query results for a specific table.
func (service *Service) InvalidateTableCache(ctx context.Context, schema, table string) {
	if service.baseHandler != nil {
		service.baseHandler.InvalidateTableCache(ctx, schema, table)
	}
}

// ResetTableCacheVersion explicitly resets the table cache version back to zero.
func (service *Service) ResetTableCacheVersion(ctx context.Context, schema, table string) {
	if service.baseHandler != nil {
		service.baseHandler.ResetTableCacheVersion(ctx, schema, table)
	}
}

// DDLEngine returns the underlying schema DDL engine.
func (service *Service) DDLEngine() *DDLEngine {
	return service.ddlEngine
}

// GetConfigManager returns the dynamic config manager.
func (service *Service) GetConfigManager() *ConfigManager {
	return service.configManager
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
	if service.baseHandler != nil {
		return service.baseHandler.RealtimeHub()
	}
	return nil
}

// GraphQLSchema returns the GraphQL schema introspector.
func (service *Service) GraphQLSchema() *graphql.SchemaIntrospector {
	if service.baseHandler != nil {
		return service.baseHandler.GraphQLSchema()
	}
	return nil
}

// IntrospectSchemas refreshes the GraphQL schema introspection.
func (service *Service) IntrospectSchemas(ctx context.Context, schemas ...string) error {
	if service.baseHandler != nil {
		return service.baseHandler.IntrospectSchemas(ctx, schemas...)
	}
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
	return service.RealtimeHub()
}

// handleFlushCache handles cache flush via the control plane handler.
func (service *Service) handleFlushCache(responseWriter http.ResponseWriter, request *http.Request) {
	if service.controlPlaneHandler != nil {
		service.controlPlaneHandler.handleFlushCache(responseWriter, request)
	}
}

// handleInvalidateCache handles cache invalidation via the control plane handler.
func (service *Service) handleInvalidateCache(responseWriter http.ResponseWriter, request *http.Request) {
	if service.controlPlaneHandler != nil {
		service.controlPlaneHandler.handleInvalidateCache(responseWriter, request)
	}
}
