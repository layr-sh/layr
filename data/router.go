package data

import (
	"net/http"

	"layr.sh/core"
)

// RegisterRoutes registers all Data service REST and OpenAPI 3.1 endpoints on public and control plane routers.
func (service *Service) RegisterRoutes(baseRouter *core.Router, controlPlaneRouter *core.Router) {
	if baseRouter != nil {
		service.registerBaseRoutes(baseRouter)
	}
	if controlPlaneRouter != nil {
		service.registerControlPlaneRoutes(controlPlaneRouter)
	}
}

func (service *Service) registerBaseRoutes(router *core.Router) {
	// 1. REST Auto-CRUD Gateway
	core.GetRoute[ListRecordsResponse](router, "/api/v1/data/{schema_name}/{table_name}", service.baseHandler.handleListRecords,
		core.RouteTag("Data REST Gateway"),
		core.RouteSummary("Query multiple rows with pagination, filtering, and sorting"),
		core.RouteDescription("Queries PostgreSQL table with dynamic filters, JSON path filtering, pagination, and sorting with RLS enforcement."),
		core.RouteOperationID("data__records__list"),
		core.RouteSDKGroupName("data", "records"),
		core.RouteSDKMethodName("list"),
	)
	core.GetRoute[GetRecordResponse](router, "/api/v1/data/{schema_name}/{table_name}/{record_id}", service.baseHandler.handleGetRecord,
		core.RouteTag("Data REST Gateway"),
		core.RouteSummary("Get a single row by primary key"),
		core.RouteDescription("Retrieves a single row from PostgreSQL by its UUID or primary key with RLS enforcement."),
		core.RouteOperationID("data__records__get"),
		core.RouteSDKGroupName("data", "records"),
		core.RouteSDKMethodName("get"),
	)
	core.PostRoute[CreateRecordResponse, CreateRecordInput](router, "/api/v1/data/{schema_name}/{table_name}", service.baseHandler.handleCreateRecord,
		core.RouteTag("Data REST Gateway"),
		core.RouteSummary("Insert single or bulk rows with RETURNING *"),
		core.RouteDescription("Inserts a single row or batch of records into PostgreSQL, automatically applying defaults and returning inserted tuples."),
		core.RouteOperationID("data__records__create"),
		core.RouteSDKGroupName("data", "records"),
		core.RouteSDKMethodName("create"),
	)
	core.PatchRoute[UpdateRecordResponse, UpdateRecordInput](router, "/api/v1/data/{schema_name}/{table_name}/{record_id}", service.baseHandler.handleUpdateRecord,
		core.RouteTag("Data REST Gateway"),
		core.RouteSummary("Partially update a row by primary key"),
		core.RouteDescription("Applies a partial update to a row identified by primary key, validating against RLS UPDATE policies."),
		core.RouteOperationID("data__records__update"),
		core.RouteSDKGroupName("data", "records"),
		core.RouteSDKMethodName("update"),
	)
	core.PutRoute[UpdateRecordResponse, UpdateRecordInput](router, "/api/v1/data/{schema_name}/{table_name}/{record_id}", service.baseHandler.handleUpdateRecord,
		core.RouteTag("Data REST Gateway"),
		core.RouteSummary("Update a row by primary key"),
		core.RouteDescription("Updates a row identified by primary key, validating against RLS UPDATE policies."),
		core.RouteOperationID("data__records__put"),
		core.RouteSDKGroupName("data", "records"),
		core.RouteSDKMethodName("put"),
	)
	core.DeleteRoute[core.Empty](router, "/api/v1/data/{schema_name}/{table_name}/{record_id}", service.baseHandler.handleDeleteRecord,
		core.RouteTag("Data REST Gateway"),
		core.RouteSummary("Delete a row by primary key"),
		core.RouteDescription("Deletes a row by primary key from PostgreSQL subject to RLS DELETE policies."),
		core.RouteNoContentResponse("Row deleted"),
		core.RouteOperationID("data__records__delete"),
		core.RouteSDKGroupName("data", "records"),
		core.RouteSDKMethodName("delete"),
	)
	core.GetRoute[ExecuteFunctionResponse](router, "/api/v1/data/{schema_name}/rpc/{function_name}", service.baseHandler.handleExecuteFunction,
		core.RouteTag("Data REST Gateway"),
		core.RouteSummary("Execute database stored function or procedure (read-only)"),
		core.RouteDescription("Invokes a PostgreSQL stored procedure or RPC with caller RLS session claims using query parameters."),
		core.RouteOperationID("data__rpc__query"),
		core.RouteSDKGroupName("data", "rpc"),
		core.RouteSDKMethodName("query"),
	)
	core.PostRoute[ExecuteFunctionResponse, ExecuteFunctionInput](router, "/api/v1/data/{schema_name}/rpc/{function_name}", service.baseHandler.handleExecuteFunction,
		core.RouteTag("Data REST Gateway"),
		core.RouteSummary("Execute database stored function or procedure"),
		core.RouteDescription("Invokes a PostgreSQL stored procedure or RPC with caller RLS session claims."),
		core.RouteOperationID("data__rpc__mutation"),
		core.RouteSDKGroupName("data", "rpc"),
		core.RouteSDKMethodName("mutation"),
	)

	// 2. Developer Ephemeral KV Endpoints
	core.PostRoute[GetMultipleKVResponse, GetMultipleKVInput](router, "/api/v1/data/kv/mget", service.baseHandler.handleGetMultipleKV,
		core.RouteTag("Data KV"),
		core.RouteSummary("Multi-get KV entries by list of keys"),
		core.RouteDescription("Batch fetches multiple keys from the ephemeral KV cache in a single atomic pipelined round-trip."),
		core.RouteOperationID("data__kv__mget"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("mget"),
	)
	core.PostRoute[SetMultipleKVResponse, SetMultipleKVInput](router, "/api/v1/data/kv/mset", service.baseHandler.handleSetMultipleKV,
		core.RouteTag("Data KV"),
		core.RouteSummary("Batch set multiple KV entries"),
		core.RouteDescription("Batch stores multiple key-value entries in the ephemeral KV store in a single round-trip."),
		core.RouteOperationID("data__kv__mset"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("mset"),
	)
	core.PostRoute[IncrementKVResponse, IncrementKVInput](router, "/api/v1/data/kv/increment", service.baseHandler.handleIncrementKV,
		core.RouteTag("Data KV"),
		core.RouteSummary("Atomically increment or decrement a KV key"),
		core.RouteDescription("Atomically increments or decrements an integer counter in the KV cache with custom step and TTL preservation."),
		core.RouteOperationID("data__kv__increment"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("increment"),
	)
	core.GetRoute[GetKVResponse](router, "/api/v1/data/kv/{key}", service.baseHandler.handleGetKV,
		core.RouteTag("Data KV"),
		core.RouteSummary("Get an ephemeral KV entry by key"),
		core.RouteDescription("Retrieves an ephemeral key-value entry with session/tenant isolation from the distributed KV store."),
		core.RouteOperationID("data__kv__get"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("get"),
	)
	core.PostRoute[SetKVResponse, SetKVInput](router, "/api/v1/data/kv/{key}", service.baseHandler.handleSetKV,
		core.RouteTag("Data KV"),
		core.RouteSummary("Set an ephemeral KV entry with envelope"),
		core.RouteDescription("Stores a key-value entry in the ephemeral KV store with { value, ttl } JSON envelope and optional SetNX."),
		core.RouteOperationID("data__kv__set"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("set"),
	)
	core.PutRoute[SetKVResponse, string](router, "/api/v1/data/kv/{key}", service.baseHandler.handleUpdateKV,
		core.RouteTag("Data KV"),
		core.RouteSummary("Put raw ephemeral KV entry"),
		core.RouteDescription("Stores raw request body directly in the ephemeral KV store with optional ?ttl= query parameter and SetNX."),
		core.RouteOperationID("data__kv__put"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("put"),
	)
	core.PatchRoute[TouchKVResponse, TouchKVInput](router, "/api/v1/data/kv/{key}", service.baseHandler.handleTouchKV,
		core.RouteTag("Data KV"),
		core.RouteSummary("Refresh TTL of an ephemeral KV entry"),
		core.RouteDescription("Updates expiration TTL of an existing key without modifying its value."),
		core.RouteOperationID("data__kv__touch"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("touch"),
	)
	core.DeleteRoute[core.Empty](router, "/api/v1/data/kv/{key}", service.baseHandler.handleDeleteKV,
		core.RouteTag("Data KV"),
		core.RouteSummary("Delete an ephemeral KV entry"),
		core.RouteDescription("Evicts an entry from the ephemeral key-value cache."),
		core.RouteNoContentResponse("KV entry deleted"),
		core.RouteOperationID("data__kv__delete"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("delete"),
	)

	// 3. Dynamic GraphQL Engine
	core.PostRoute[ExecuteGraphQLResponse, ExecuteGraphQLInput](router, "/api/v1/graphql", service.baseHandler.handleExecuteGraphQL,
		core.RouteTag("Data GraphQL"),
		core.RouteSummary("Execute GraphQL query or mutation"),
		core.RouteDescription("Dynamic GraphQL introspection and query engine over PostgreSQL catalog."),
		core.RouteOperationID("data__graphql"),
		core.RouteSDKGroupName("data"),
		core.RouteSDKMethodName("graphql"),
	)

	// 4. Real-Time CDC WebSocket
	core.GetRoute[core.Empty](router, "/api/v1/realtime", service.baseHandler.handleConnectRealtime,
		core.RouteTag("Data Real-Time"),
		core.RouteSummary("Connect to real-time Change Data Capture (CDC) WebSocket"),
		core.RouteDescription("Live bidirectional WebSocket channel streaming PostgreSQL CDC events."),
		core.RouteResponseHeader("Upgrade", "websocket"),
		core.RouteNoContentResponse("Realtime connection established"),
		core.RouteOperationID("data__realtime"),
		core.RouteSDKGroupName("data"),
		core.RouteSDKMethodName("realtime"),
	)
}

func (service *Service) registerControlPlaneRoutes(router *core.Router) {
	// 1. Dynamic Config
	core.GetRoute[Config](router, "/api/v1/_/data/config", service.controlPlaneHandler.handleGetConfig,
		core.RouteTag("Data Control Plane"),
		core.RouteSummary("Retrieve dynamic data service configuration"),
		core.RouteDescription("Retrieves dynamic runtime data service settings including exposed schemas, pool sizes, and cache TTL rules."),
		core.RouteOperationID("data__config__get"),
		core.RouteSDKGroupName("data", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/api/v1/_/data/config", service.controlPlaneHandler.handleUpdateConfig,
		core.RouteTag("Data Control Plane"),
		core.RouteSummary("Update dynamic data service configuration"),
		core.RouteDescription("Updates dynamic runtime data service configuration without requiring server restarts."),
		core.RouteOperationID("data__config__update"),
		core.RouteSDKGroupName("data", "config"),
		core.RouteSDKMethodName("update"),
	)

	// 2. Cache Management
	core.PostRoute[FlushCacheResponse, core.Empty](router, "/api/v1/_/data/cache/flush", service.controlPlaneHandler.handleFlushCache,
		core.RouteTag("Data Control Plane"),
		core.RouteSummary("Flush all global query and schema reflection caches"),
		core.RouteDescription("Flushes in-memory and distributed query result caches across all cluster nodes."),
		core.RouteNoRequestBody(),
		core.RouteOperationID("data__cache__flush"),
		core.RouteSDKGroupName("data", "cache"),
		core.RouteSDKMethodName("flush"),
	)
	core.PostRoute[InvalidateCacheResponse, InvalidateCacheInput](router, "/api/v1/_/data/cache/invalidate", service.controlPlaneHandler.handleInvalidateCache,
		core.RouteTag("Data Control Plane"),
		core.RouteSummary("Programmatically invalidate specific table or catalog caches"),
		core.RouteDescription("Invalidates reflection and query caches for a specific table or schema pattern."),
		core.RouteOperationID("data__cache__invalidate"),
		core.RouteSDKGroupName("data", "cache"),
		core.RouteSDKMethodName("invalidate"),
	)

	// 3. Visual Table Schema & DDL Management
	core.GetRoute[ListTablesResponse](router, "/api/v1/_/data/tables", service.controlPlaneHandler.handleListTables,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("List all user tables across configured schemas"),
		core.RouteDescription("Queries PostgreSQL information_schema and pg_catalog to list tables, primary keys, and approximate row counts."),
		core.RouteOperationID("data__tables__list"),
		core.RouteSDKGroupName("data", "tables"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[CreateTableResponse, CreateTableInput](router, "/api/v1/_/data/tables", service.controlPlaneHandler.handleCreateTable,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("Create a new PostgreSQL table with UUIDv7 primary key"),
		core.RouteDescription("Executes DDL to create a table adhering to Layr invariants (UUIDv7 primary keys and timestamps)."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("data__tables__create"),
		core.RouteSDKGroupName("data", "tables"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[Table](router, "/api/v1/_/data/tables/{schema_name}/{table_name}", service.controlPlaneHandler.handleGetTable,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("Inspect table schema, columns, indexes, and constraints"),
		core.RouteDescription("Retrieves column definitions, foreign keys, indexes, and RLS policies for a specific table."),
		core.RouteOperationID("data__tables__get"),
		core.RouteSDKGroupName("data", "tables"),
		core.RouteSDKMethodName("get"),
	)
	core.DeleteRoute[DeleteTableResponse](router, "/api/v1/_/data/tables/{schema_name}/{table_name}", service.controlPlaneHandler.handleDeleteTable,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("Drop a table"),
		core.RouteDescription("Drops a table and its associated constraints, indexes, and triggers from PostgreSQL."),
		core.RouteOperationID("data__tables__delete"),
		core.RouteSDKGroupName("data", "tables"),
		core.RouteSDKMethodName("delete"),
	)
	core.PostRoute[CreateColumnResponse, Column](router, "/api/v1/_/data/tables/{schema_name}/{table_name}/columns", service.controlPlaneHandler.handleCreateColumn,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("Add a new column to a table"),
		core.RouteDescription("Alters table schema to add a new column with type validation and optional default values."),
		core.RouteOperationID("data__tables__columns__create"),
		core.RouteSDKGroupName("data", "tables", "columns"),
		core.RouteSDKMethodName("create"),
	)
	core.PatchRoute[UpdateColumnResponse, UpdateColumnInput](router, "/api/v1/_/data/tables/{schema_name}/{table_name}/columns/{column_name}", service.controlPlaneHandler.handleUpdateColumn,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("Alter column type, default, nullable, or rename"),
		core.RouteDescription("Alters column attributes or renames a column."),
		core.RouteOperationID("data__tables__columns__update"),
		core.RouteSDKGroupName("data", "tables", "columns"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[DeleteColumnResponse](router, "/api/v1/_/data/tables/{schema_name}/{table_name}/columns/{column_name}", service.controlPlaneHandler.handleDeleteColumn,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("Drop a column from a table"),
		core.RouteDescription("Drops a column from a table schema."),
		core.RouteOperationID("data__tables__columns__delete"),
		core.RouteSDKGroupName("data", "tables", "columns"),
		core.RouteSDKMethodName("delete"),
	)
	core.PostRoute[CreateIndexResponse, CreateIndexInput](router, "/api/v1/_/data/tables/{schema_name}/{table_name}/indexes", service.controlPlaneHandler.handleCreateIndex,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("Create an index on a table"),
		core.RouteDescription("Creates a B-Tree, GIN, GiST, or BRIN index on table columns."),
		core.RouteOperationID("data__tables__indexes__create"),
		core.RouteSDKGroupName("data", "tables", "indexes"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[ListIndexesResponse](router, "/api/v1/_/data/tables/{schema_name}/{table_name}/indexes", service.controlPlaneHandler.handleListIndexes,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("List indexes on a table"),
		core.RouteDescription("Lists all indexes defined on the specified table."),
		core.RouteOperationID("data__tables__indexes__list"),
		core.RouteSDKGroupName("data", "tables", "indexes"),
		core.RouteSDKMethodName("list"),
	)
	core.DeleteRoute[DeleteIndexResponse](router, "/api/v1/_/data/tables/{schema_name}/{table_name}/indexes/{index_name}", service.controlPlaneHandler.handleDeleteIndex,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("Drop an index from a table"),
		core.RouteDescription("Drops an index from PostgreSQL."),
		core.RouteOperationID("data__tables__indexes__delete"),
		core.RouteSDKGroupName("data", "tables", "indexes"),
		core.RouteSDKMethodName("delete"),
	)
	core.GetRoute[ListPoliciesResponse](router, "/api/v1/_/data/tables/{schema_name}/{table_name}/policies", service.controlPlaneHandler.handleListPolicies,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("List RLS policies on a table"),
		core.RouteDescription("Lists Row-Level Security policies with USING and WITH CHECK SQL expressions."),
		core.RouteOperationID("data__tables__policies__list"),
		core.RouteSDKGroupName("data", "tables", "policies"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[CreatePolicyResponse, CreatePolicyInput](router, "/api/v1/_/data/tables/{schema_name}/{table_name}/policies", service.controlPlaneHandler.handleCreatePolicy,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("Create a Row-Level Security policy on a table"),
		core.RouteDescription("Creates an RLS policy defining declarative access rules for roles and commands."),
		core.RouteOperationID("data__tables__policies__create"),
		core.RouteSDKGroupName("data", "tables", "policies"),
		core.RouteSDKMethodName("create"),
	)
	core.DeleteRoute[DeletePolicyResponse](router, "/api/v1/_/data/tables/{schema_name}/{table_name}/policies/{policy_name}", service.controlPlaneHandler.handleDeletePolicy,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("Drop an RLS policy from a table"),
		core.RouteDescription("Drops an RLS policy from a table."),
		core.RouteOperationID("data__tables__policies__delete"),
		core.RouteSDKGroupName("data", "tables", "policies"),
		core.RouteSDKMethodName("delete"),
	)
	core.PatchRoute[ToggleRLSResponse, ToggleRLSInput](router, "/api/v1/_/data/tables/{schema_name}/{table_name}/rls", service.controlPlaneHandler.handleToggleRLS,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("Toggle Row-Level Security (ENABLE / DISABLE / FORCE)"),
		core.RouteDescription("Toggles Row-Level Security enforcement mode on a table."),
		core.RouteOperationID("data__tables__rls__update"),
		core.RouteSDKGroupName("data", "tables", "rls"),
		core.RouteSDKMethodName("update"),
	)

	// 4. Console SQL Scratchpad
	core.PostRoute[ExecuteSQLResponse, ExecuteSQLInput](router, "/api/v1/_/data/sql", service.controlPlaneHandler.handleExecuteSQL,
		core.RouteTag("Data Schema DDL"),
		core.RouteSummary("Execute raw SQL query or DDL script from Console scratchpad"),
		core.RouteDescription("Executes arbitrary SQL queries or migrations within a controlled transaction block with execution timing."),
		core.RouteOperationID("data__sql"),
		core.RouteSDKGroupName("data"),
		core.RouteSDKMethodName("sql"),
	)
}
