package data

import (
	"net/http"

	"layr.sh/core"
)

// RegisterRoutes registers all Data service REST and OpenAPI 3.1 endpoints on public and control plane routers.
func (service *Service) RegisterRoutes(baseRouter *core.Router, controlPlaneRouter *core.Router) {
	service.registerBaseRoutes(baseRouter)
	service.registerControlPlaneRoutes(controlPlaneRouter)
}

func (service *Service) registerBaseRoutes(router *core.Router) {
	// 1. REST Auto-CRUD Gateway
	core.GetRoute[ListRecordsResponse](router, "/v1/data/{schema_name}/{table_name}", service.baseHandler.handleListRecords,
		core.RouteTag("Records"),
		core.RouteSummary("Query table rows with dynamic filtering, sorting, and pagination"),
		core.RouteDescription("Queries PostgreSQL table rows with dynamic filter predicates (eq, gte, in, like), JSON path filtering, relational foreign-key embedding, cursor/offset pagination, and multi-column sorting. Injects caller JWT session claims into transaction-local variables for native Row-Level Security (RLS) enforcement. Supports optional KV result caching via ?cache_ttl."),
		core.RouteDefaultPagination(),
		core.RouteQueryParam("select", "Column projection and embedded relations (e.g. 'id,name,posts(*)')"),
		core.RouteQueryParam("order", "Ordering expression (e.g. 'created_at.desc,id.asc')"),
		core.RouteQueryInt("cache_ttl", "Cache duration in seconds"),
		core.RouteQueryParam("cache_key_suffix", "Custom suffix for cache key isolation"),
		core.RouteQueryParam("cache_key", "Alias for cache_key_suffix"),
		core.RouteOperationID("data__records__list"),
		core.RouteSDKGroupName("data", "records"),
		core.RouteSDKMethodName("list"),
	)
	core.GetRoute[GetRecordResponse](router, "/v1/data/{schema_name}/{table_name}/{record_id}", service.baseHandler.handleGetRecord,
		core.RouteTag("Records"),
		core.RouteSummary("Get a single row by primary key"),
		core.RouteDescription("Retrieves a single row from PostgreSQL by UUID or primary key, enforcing caller Row-Level Security (RLS) SELECT policies and executing within caller session claims."),
		core.RouteQueryParam("select", "Column projection (e.g. 'id,name')"),
		core.RouteOperationID("data__records__get"),
		core.RouteSDKGroupName("data", "records"),
		core.RouteSDKMethodName("get"),
	)
	core.PostRoute[CreateRecordResponse, CreateRecordInput](router, "/v1/data/{schema_name}/{table_name}", service.baseHandler.handleCreateRecord,
		core.RouteTag("Records"),
		core.RouteSummary("Insert single or bulk rows with RETURNING *"),
		core.RouteDescription("Inserts a single row or batch of JSON objects into PostgreSQL, automatically applying defaults and returning inserted tuples. Validates against RLS INSERT ... WITH CHECK policies, invalidates table query caches, and emits real-time CDC updates."),
		core.RouteQueryParam("on_conflict", "Target column or constraint for upsert on-conflict resolution"),
		core.RouteOperationID("data__records__create"),
		core.RouteSDKGroupName("data", "records"),
		core.RouteSDKMethodName("create"),
	)
	core.PatchRoute[UpdateRecordResponse, UpdateRecordInput](router, "/v1/data/{schema_name}/{table_name}/{record_id}", service.baseHandler.handleUpdateRecord,
		core.RouteTag("Records"),
		core.RouteSummary("Partially update a row by primary key"),
		core.RouteDescription("Applies partial column updates to a row identified by primary key, validating against RLS UPDATE ... USING ... WITH CHECK policies, invalidating cached queries, and broadcasting CDC changes."),
		core.RouteOperationID("data__records__update"),
		core.RouteSDKGroupName("data", "records"),
		core.RouteSDKMethodName("update"),
	)
	core.PutRoute[UpdateRecordResponse, UpdateRecordInput](router, "/v1/data/{schema_name}/{table_name}/{record_id}", service.baseHandler.handleUpdateRecord,
		core.RouteTag("Records"),
		core.RouteSummary("Update entire row by primary key"),
		core.RouteDescription("Replaces column values for a row identified by primary key, validating against RLS UPDATE policies, invalidating table query caches, and broadcasting CDC changes."),
		core.RouteOperationID("data__records__put"),
		core.RouteSDKGroupName("data", "records"),
		core.RouteSDKMethodName("put"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/data/{schema_name}/{table_name}/{record_id}", service.baseHandler.handleDeleteRecord,
		core.RouteTag("Records"),
		core.RouteSummary("Delete a row by primary key"),
		core.RouteDescription("Deletes a row by primary key from PostgreSQL subject to caller RLS DELETE ... USING policies, invalidating table query caches and broadcasting CDC events."),
		core.RouteNoContentResponse("Row deleted"),
		core.RouteOperationID("data__records__delete"),
		core.RouteSDKGroupName("data", "records"),
		core.RouteSDKMethodName("delete"),
	)
	core.GetRoute[ExecuteFunctionResponse](router, "/v1/data/{schema_name}/rpc/{function_name}", service.baseHandler.handleExecuteFunction,
		core.RouteTag("RPC"),
		core.RouteSummary("Execute read-only database stored function or procedure"),
		core.RouteDescription("Invokes a PostgreSQL stored function or procedure with arguments passed via query parameters, setting transaction-local caller RLS session claims."),
		core.RouteQueryParam("invalidate_tables", "Comma-separated table names whose cache should be invalidated"),
		core.RouteOperationID("data__rpc__query"),
		core.RouteSDKGroupName("data", "rpc"),
		core.RouteSDKMethodName("query"),
	)
	core.PostRoute[ExecuteFunctionResponse, ExecuteFunctionInput](router, "/v1/data/{schema_name}/rpc/{function_name}", service.baseHandler.handleExecuteFunction,
		core.RouteTag("RPC"),
		core.RouteSummary("Execute database stored function or procedure with JSON payload"),
		core.RouteDescription("Invokes a PostgreSQL stored procedure or function with JSON request body arguments, setting caller RLS session claims and returning execution results."),
		core.RouteQueryParam("invalidate_tables", "Comma-separated table names whose cache should be invalidated"),
		core.RouteOperationID("data__rpc__mutation"),
		core.RouteSDKGroupName("data", "rpc"),
		core.RouteSDKMethodName("mutation"),
	)

	// 2. Developer Ephemeral KV Endpoints
	core.PostRoute[GetMultipleKVResponse, GetMultipleKVInput](router, "/v1/data/kv/mget", service.baseHandler.handleGetMultipleKV,
		core.RouteTag("KV"),
		core.RouteSummary("Batch get multiple ephemeral KV entries"),
		core.RouteDescription("Fetches values for an array of keys from the distributed ephemeral KV cache in a single atomic pipelined round-trip, maintaining tenant/session key isolation."),
		core.RouteOperationID("data__kv__mget"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("mget"),
	)
	core.PostRoute[SetMultipleKVResponse, SetMultipleKVInput](router, "/v1/data/kv/mset", service.baseHandler.handleSetMultipleKV,
		core.RouteTag("KV"),
		core.RouteSummary("Batch set multiple ephemeral KV entries"),
		core.RouteDescription("Stores multiple key-value pairs with optional individual TTLs in the distributed KV cache in a single atomic round-trip."),
		core.RouteOperationID("data__kv__mset"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("mset"),
	)
	core.PostRoute[IncrementKVResponse, IncrementKVInput](router, "/v1/data/kv/increment", service.baseHandler.handleIncrementKV,
		core.RouteTag("KV"),
		core.RouteSummary("Atomically increment or decrement an integer counter"),
		core.RouteDescription("Atomically modifies an integer counter in the KV store by step value (positive or negative) with optional TTL preservation, returning the updated integer."),
		core.RouteOperationID("data__kv__increment"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("increment"),
	)
	core.GetRoute[GetKVResponse](router, "/v1/data/kv/{key}", service.baseHandler.handleGetKV,
		core.RouteTag("KV"),
		core.RouteSummary("Get an ephemeral KV entry by key"),
		core.RouteDescription("Retrieves a key-value entry with session/tenant isolation from the distributed KV store. Returns 404 if expired or not found."),
		core.RouteOperationID("data__kv__get"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("get"),
	)
	core.PostRoute[SetKVResponse, SetKVInput](router, "/v1/data/kv/{key}", service.baseHandler.handleSetKV,
		core.RouteTag("KV"),
		core.RouteSummary("Set an ephemeral KV entry with JSON envelope"),
		core.RouteDescription("Stores a value in the KV cache using a { value, ttl } JSON envelope with optional set_nx conditional insertion flag (maximum payload 2 MB)."),
		core.RouteQueryBool("nx", "Only set the key if it does not already exist"),
		core.RouteOperationID("data__kv__set"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("set"),
	)
	core.PutRoute[SetKVResponse, string](router, "/v1/data/kv/{key}", service.baseHandler.handleUpdateKV,
		core.RouteTag("KV"),
		core.RouteSummary("Put raw ephemeral KV entry with query TTL"),
		core.RouteDescription("Stores raw request body directly in the KV store with optional ?ttl= query parameter and optional set_nx conditional insertion flag."),
		core.RouteQueryInt("ttl", "Key time-to-live expiration in seconds"),
		core.RouteQueryBool("nx", "Only set the key if it does not already exist"),
		core.RouteOperationID("data__kv__put"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("put"),
	)
	core.PatchRoute[TouchKVResponse, TouchKVInput](router, "/v1/data/kv/{key}", service.baseHandler.handleTouchKV,
		core.RouteTag("KV"),
		core.RouteSummary("Refresh TTL of an existing KV entry"),
		core.RouteDescription("Updates the expiration TTL of an existing key in the KV cache without altering its stored payload."),
		core.RouteOperationID("data__kv__touch"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("touch"),
	)
	core.DeleteRoute[core.Empty](router, "/v1/data/kv/{key}", service.baseHandler.handleDeleteKV,
		core.RouteTag("KV"),
		core.RouteSummary("Delete an ephemeral KV entry"),
		core.RouteDescription("Evicts a key-value entry from the distributed KV cache."),
		core.RouteNoContentResponse("KV entry deleted"),
		core.RouteOperationID("data__kv__delete"),
		core.RouteSDKGroupName("data", "kv"),
		core.RouteSDKMethodName("delete"),
	)

	// 3. Dynamic GraphQL Engine
	core.PostRoute[ExecuteGraphQLResponse, ExecuteGraphQLInput](router, "/v1/graphql", service.baseHandler.handleExecuteGraphQL,
		core.RouteTag("GraphQL"),
		core.RouteSummary("Execute GraphQL query or mutation"),
		core.RouteDescription("Dynamic introspection and execution engine over PostgreSQL tables, supporting complex nested selections, arguments, variables, query depth limits (max depth guard), complexity budgets, and native RLS session context injection."),
		core.RouteOperationID("data__graphql"),
		core.RouteSDKGroupName("data"),
		core.RouteSDKMethodName("graphql"),
	)

	// 4. Real-Time CDC WebSocket
	core.GetRoute[core.Empty](router, "/v1/realtime", service.baseHandler.handleConnectRealtime,
		core.RouteHide(),
		core.RouteTag("Real-Time"),
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
	core.GetRoute[Config](router, "/v1/_/data/config", service.controlPlaneHandler.handleGetConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Retrieve dynamic data service configuration"),
		core.RouteDescription("Returns dynamic data service configuration settings including exposed schema names, connection pool sizes, REST limit defaults, and cache TTL parameters. Requires data:config.read scope."),
		core.RouteOperationID("data__config__get"),
		core.RouteSDKGroupName("data", "config"),
		core.RouteSDKMethodName("get"),
	)
	core.PutRoute[Config, Config](router, "/v1/_/data/config", service.controlPlaneHandler.handleUpdateConfig,
		core.RouteTag("Config"),
		core.RouteSummary("Update dynamic data service configuration"),
		core.RouteDescription("Updates operational data service parameters in data.config, dynamically tuning connection limits, exposed schemas, and cache behaviors without server downtime. Requires data:config.write scope."),
		core.RouteOperationID("data__config__update"),
		core.RouteSDKGroupName("data", "config"),
		core.RouteSDKMethodName("update"),
	)

	// 2. Cache Management
	core.PostRoute[FlushCacheResponse, core.Empty](router, "/v1/_/data/cache/flush", service.controlPlaneHandler.handleFlushCache,
		core.RouteTag("Cache"),
		core.RouteSummary("Flush all global query and schema reflection caches"),
		core.RouteDescription("Flushes in-memory and distributed table reflection metadata, query result caches, and composite version counters across all cluster nodes. Requires data:cache.write scope."),
		core.RouteNoRequestBody(),
		core.RouteOperationID("data__cache__flush"),
		core.RouteSDKGroupName("data", "cache"),
		core.RouteSDKMethodName("flush"),
	)
	core.PostRoute[InvalidateCacheResponse, InvalidateCacheInput](router, "/v1/_/data/cache/invalidate", service.controlPlaneHandler.handleInvalidateCache,
		core.RouteTag("Cache"),
		core.RouteSummary("Invalidate specific table or catalog caches"),
		core.RouteDescription("Invalidates schema reflection and query caches for a specific schema, table name, or pattern, bumping composite table cache versions. Requires data:cache.write scope."),
		core.RouteOperationID("data__cache__invalidate"),
		core.RouteSDKGroupName("data", "cache"),
		core.RouteSDKMethodName("invalidate"),
	)

	// 3. Visual Table Schema & DDL Management
	core.GetRoute[ListTablesResponse](router, "/v1/_/data/tables", service.controlPlaneHandler.handleListTables,
		core.RouteTag("Tables"),
		core.RouteSummary("List all database tables across configured schemas"),
		core.RouteDescription("Queries PostgreSQL information_schema and pg_catalog to return tables, primary key column names, approximate row counts, and RLS enabled statuses across exposed schemas. Requires data:schema.read scope."),
		core.RouteOperationID("data__tables__list"),
		core.RouteSDKGroupName("data", "tables"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[CreateTableResponse, CreateTableInput](router, "/v1/_/data/tables", service.controlPlaneHandler.handleCreateTable,
		core.RouteTag("Tables"),
		core.RouteSummary("Create a new PostgreSQL table with UUIDv7 primary key"),
		core.RouteDescription("Executes DDL to provision a table adhering to Layr conventions (UUIDv7 primary keys, created_at, and last_updated_at timestamps), invalidating catalog caches and emitting data.table.created event. Requires data:schema.write scope."),
		core.RouteDefaultStatusCode(http.StatusCreated),
		core.RouteOperationID("data__tables__create"),
		core.RouteSDKGroupName("data", "tables"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[Table](router, "/v1/_/data/tables/{schema_name}/{table_name}", service.controlPlaneHandler.handleGetTable,
		core.RouteTag("Tables"),
		core.RouteSummary("Inspect table schema, columns, indexes, and constraints"),
		core.RouteDescription("Retrieves complete architectural schema for a table, detailing columns (types, nullability, defaults), primary/foreign key constraints, indexes, and active RLS policies. Requires data:schema.read scope."),
		core.RouteOperationID("data__tables__get"),
		core.RouteSDKGroupName("data", "tables"),
		core.RouteSDKMethodName("get"),
	)
	core.DeleteRoute[DeleteTableResponse](router, "/v1/_/data/tables/{schema_name}/{table_name}", service.controlPlaneHandler.handleDeleteTable,
		core.RouteTag("Tables"),
		core.RouteSummary("Drop a table and cascade constraints"),
		core.RouteDescription("Executes DROP TABLE CASCADE in PostgreSQL, removing the table, associated indexes, triggers, and foreign key constraints. Invalidates catalog caches and emits data.table.deleted event. Requires data:schema.write scope."),
		core.RouteQueryBool("cascade", "Cascade table drop to dependent constraints and views"),
		core.RouteOperationID("data__tables__delete"),
		core.RouteSDKGroupName("data", "tables"),
		core.RouteSDKMethodName("delete"),
	)
	core.PostRoute[CreateColumnResponse, Column](router, "/v1/_/data/tables/{schema_name}/{table_name}/columns", service.controlPlaneHandler.handleCreateColumn,
		core.RouteTag("Tables Columns"),
		core.RouteSummary("Add a new column to a table schema"),
		core.RouteDescription("Executes ALTER TABLE ADD COLUMN with validated PostgreSQL data type, nullability constraint, and optional default expression. Invalidates schema reflection caches and emits data.table.updated event. Requires data:schema.write scope."),
		core.RouteOperationID("data__tables__columns__create"),
		core.RouteSDKGroupName("data", "tables", "columns"),
		core.RouteSDKMethodName("create"),
	)
	core.PatchRoute[UpdateColumnResponse, UpdateColumnInput](router, "/v1/_/data/tables/{schema_name}/{table_name}/columns/{column_name}", service.controlPlaneHandler.handleUpdateColumn,
		core.RouteTag("Tables Columns"),
		core.RouteSummary("Alter column type, default, nullability, or rename"),
		core.RouteDescription("Alters column attributes (data type conversion, set/drop default, set/drop NOT NULL) or renames the column. Invalidates table metadata caches and emits data.table.updated event. Requires data:schema.write scope."),
		core.RouteOperationID("data__tables__columns__update"),
		core.RouteSDKGroupName("data", "tables", "columns"),
		core.RouteSDKMethodName("update"),
	)
	core.DeleteRoute[DeleteColumnResponse](router, "/v1/_/data/tables/{schema_name}/{table_name}/columns/{column_name}", service.controlPlaneHandler.handleDeleteColumn,
		core.RouteTag("Tables Columns"),
		core.RouteSummary("Drop a column from a table schema"),
		core.RouteDescription("Executes ALTER TABLE DROP COLUMN CASCADE, dropping the column and dependent views or indexes. Invalidates reflection caches and emits data.table.updated event. Requires data:schema.write scope."),
		core.RouteQueryBool("cascade", "Cascade column drop to dependent views or indexes"),
		core.RouteOperationID("data__tables__columns__delete"),
		core.RouteSDKGroupName("data", "tables", "columns"),
		core.RouteSDKMethodName("delete"),
	)
	core.PostRoute[CreateIndexResponse, CreateIndexInput](router, "/v1/_/data/tables/{schema_name}/{table_name}/indexes", service.controlPlaneHandler.handleCreateIndex,
		core.RouteTag("Tables Indexes"),
		core.RouteSummary("Create an index on table columns"),
		core.RouteDescription("Executes CREATE INDEX supporting B-Tree, GIN, GiST, or BRIN index methods, unique constraints, and multi-column expressions. Emits data.table.updated event. Requires data:schema.write scope."),
		core.RouteOperationID("data__tables__indexes__create"),
		core.RouteSDKGroupName("data", "tables", "indexes"),
		core.RouteSDKMethodName("create"),
	)
	core.GetRoute[ListIndexesResponse](router, "/v1/_/data/tables/{schema_name}/{table_name}/indexes", service.controlPlaneHandler.handleListIndexes,
		core.RouteTag("Tables Indexes"),
		core.RouteSummary("List indexes defined on a table"),
		core.RouteDescription("Queries pg_indexes to return all index definitions, methods, uniqueness constraints, and column lists for the specified table. Requires data:schema.read scope."),
		core.RouteOperationID("data__tables__indexes__list"),
		core.RouteSDKGroupName("data", "tables", "indexes"),
		core.RouteSDKMethodName("list"),
	)
	core.DeleteRoute[DeleteIndexResponse](router, "/v1/_/data/tables/{schema_name}/{table_name}/indexes/{index_name}", service.controlPlaneHandler.handleDeleteIndex,
		core.RouteTag("Tables Indexes"),
		core.RouteSummary("Drop an index from a table"),
		core.RouteDescription("Executes DROP INDEX in PostgreSQL, removing the index and invalidating query planner metadata. Emits data.table.updated event. Requires data:schema.write scope."),
		core.RouteOperationID("data__tables__indexes__delete"),
		core.RouteSDKGroupName("data", "tables", "indexes"),
		core.RouteSDKMethodName("delete"),
	)
	core.GetRoute[ListPoliciesResponse](router, "/v1/_/data/tables/{schema_name}/{table_name}/policies", service.controlPlaneHandler.handleListPolicies,
		core.RouteTag("Tables Policies"),
		core.RouteSummary("List Row-Level Security policies on a table"),
		core.RouteDescription("Queries pg_policies to list active Row-Level Security (RLS) policies, detailing command targets (SELECT, INSERT, UPDATE, DELETE), roles, USING filters, and WITH CHECK expressions. Requires data:schema.read scope."),
		core.RouteOperationID("data__tables__policies__list"),
		core.RouteSDKGroupName("data", "tables", "policies"),
		core.RouteSDKMethodName("list"),
	)
	core.PostRoute[CreatePolicyResponse, CreatePolicyInput](router, "/v1/_/data/tables/{schema_name}/{table_name}/policies", service.controlPlaneHandler.handleCreatePolicy,
		core.RouteTag("Tables Policies"),
		core.RouteSummary("Create a Row-Level Security policy on a table"),
		core.RouteDescription("Executes CREATE POLICY in PostgreSQL, attaching declarative row-level access control rules for specific database roles and SQL commands with USING and WITH CHECK expressions. Emits data.table.updated event. Requires data:schema.write scope."),
		core.RouteOperationID("data__tables__policies__create"),
		core.RouteSDKGroupName("data", "tables", "policies"),
		core.RouteSDKMethodName("create"),
	)
	core.DeleteRoute[DeletePolicyResponse](router, "/v1/_/data/tables/{schema_name}/{table_name}/policies/{policy_name}", service.controlPlaneHandler.handleDeletePolicy,
		core.RouteTag("Tables Policies"),
		core.RouteSummary("Drop an RLS policy from a table"),
		core.RouteDescription("Executes DROP POLICY on the specified table, removing the security policy from PostgreSQL enforcement. Emits data.table.updated event. Requires data:schema.write scope."),
		core.RouteOperationID("data__tables__policies__delete"),
		core.RouteSDKGroupName("data", "tables", "policies"),
		core.RouteSDKMethodName("delete"),
	)
	core.PatchRoute[ToggleRLSResponse, ToggleRLSInput](router, "/v1/_/data/tables/{schema_name}/{table_name}/rls", service.controlPlaneHandler.handleToggleRLS,
		core.RouteTag("Tables RLS"),
		core.RouteSummary("Toggle Row-Level Security enforcement mode on a table"),
		core.RouteDescription("Executes ALTER TABLE to ENABLE, DISABLE, or FORCE Row-Level Security (RLS) enforcement on the table. Invalidates table metadata and emits data.table.updated event. Requires data:schema.write scope."),
		core.RouteQueryParam("action", "RLS enforcement mode ('ENABLE', 'DISABLE', or 'FORCE')"),
		core.RouteQueryBool("force", "Whether to enforce RLS for table owners when action is FORCE"),
		core.RouteOperationID("data__tables__rls__update"),
		core.RouteSDKGroupName("data", "tables", "rls"),
		core.RouteSDKMethodName("update"),
	)

	// 4. Console SQL Scratchpad
	core.PostRoute[ExecuteSQLResponse, ExecuteSQLInput](router, "/v1/_/data/sql", service.controlPlaneHandler.handleExecuteSQL,
		core.RouteHide(),
		core.RouteTag("SQL"),
		core.RouteSummary("Execute raw SQL query or DDL script from Console scratchpad"),
		core.RouteDescription("Executes arbitrary SQL queries or migrations within a controlled transaction block with execution timing."),
		core.RouteOperationID("data__sql"),
		core.RouteSDKGroupName("data"),
		core.RouteSDKMethodName("sql"),
	)
}
