package data

import (
	"time"

	"layr.sh/core"
)

// ConfigUpdatedEventData represents the payload for data.config.updated.
type ConfigUpdatedEventData Config

// NewConfigUpdatedEvent creates a typed event for data configuration updates.
func NewConfigUpdatedEvent(resourceID string, configUpdatedEventData ConfigUpdatedEventData) core.Event {
	return core.NewEvent("data.config.updated", configUpdatedEventData).WithResourceID(resourceID)
}

// CacheFlushedEventData represents the payload for data.cache.flushed.
type CacheFlushedEventData struct {
	Pattern   string    `json:"pattern"`
	FlushedAt time.Time `json:"flushed_at"`
}

// NewCacheFlushedEvent creates a typed event for global cache flushes.
func NewCacheFlushedEvent(resourceID string, cacheFlushedEventData CacheFlushedEventData) core.Event {
	return core.NewEvent("data.cache.flushed", cacheFlushedEventData).WithResourceID(resourceID)
}

// CacheInvalidatedEventData represents the payload for data.cache.invalidated.
type CacheInvalidatedEventData struct {
	Pattern           string `json:"pattern"`
	InvalidationCount int    `json:"invalidation_count,omitempty"`
}

// NewCacheInvalidatedEvent creates a typed event for targeted cache invalidations.
func NewCacheInvalidatedEvent(resourceID string, cacheInvalidatedEventData CacheInvalidatedEventData) core.Event {
	return core.NewEvent("data.cache.invalidated", cacheInvalidatedEventData).WithResourceID(resourceID)
}

// TableCreatedEventData represents the payload for data.table.created.
type TableCreatedEventData struct {
	Schema              string       `json:"schema"`
	Table               string       `json:"table"`
	Columns             []Column     `json:"columns,omitempty"`
	PrimaryKeys         []string     `json:"primary_keys,omitempty"`
	ForeignKeys         []ForeignKey `json:"foreign_keys,omitempty"`
	Indexes             []Index      `json:"indexes,omitempty"`
	Policies            []Policy     `json:"policies,omitempty"`
	ApproximateRowCount int64        `json:"approximate_row_count,omitempty"`
	RLSEnabled          bool         `json:"rls_enabled"`
}

// NewTableCreatedEvent creates a typed event for table creations.
func NewTableCreatedEvent(resourceID string, tableCreatedEventData TableCreatedEventData) core.Event {
	return core.NewEvent("data.table.created", tableCreatedEventData).WithResourceID(resourceID)
}

// TableUpdatedEventData represents the payload for data.table.updated.
type TableUpdatedEventData struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
	Action string `json:"action"` // "add_column", "alter_column", "drop_column", "create_index", etc.
	Detail any    `json:"detail,omitempty"`
}

// NewTableUpdatedEvent creates a typed event for table alterations, column modifications, or index changes.
func NewTableUpdatedEvent(resourceID string, tableUpdatedEventData TableUpdatedEventData) core.Event {
	return core.NewEvent("data.table.updated", tableUpdatedEventData).WithResourceID(resourceID)
}

// TableDeletedEventData represents the payload for data.table.deleted.
type TableDeletedEventData struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
}

// NewTableDeletedEvent creates a typed event for table drops.
func NewTableDeletedEvent(resourceID string, tableDeletedEventData TableDeletedEventData) core.Event {
	return core.NewEvent("data.table.deleted", tableDeletedEventData).WithResourceID(resourceID)
}

// RowCreatedEventData represents the payload for data.row.created.
type RowCreatedEventData struct {
	Schema     string            `json:"schema"`
	Table      string            `json:"table"`
	ID         string            `json:"id"`
	Properties map[string]string `json:"properties,omitempty"`
}

// NewRowCreatedEvent creates a typed event for record insertions.
func NewRowCreatedEvent(resourceID string, rowCreatedEventData RowCreatedEventData) core.Event {
	return core.NewEvent("data.row.created", rowCreatedEventData).WithResourceID(resourceID)
}

// RowUpdatedEventData represents the payload for data.row.updated.
type RowUpdatedEventData struct {
	Schema     string            `json:"schema"`
	Table      string            `json:"table"`
	ID         string            `json:"id"`
	Properties map[string]string `json:"properties,omitempty"`
}

// NewRowUpdatedEvent creates a typed event for record updates.
func NewRowUpdatedEvent(resourceID string, rowUpdatedEventData RowUpdatedEventData) core.Event {
	return core.NewEvent("data.row.updated", rowUpdatedEventData).WithResourceID(resourceID)
}

// RowDeletedEventData represents the payload for data.row.deleted.
type RowDeletedEventData struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
	ID     string `json:"id"`
}

// NewRowDeletedEvent creates a typed event for record deletions.
func NewRowDeletedEvent(resourceID string, rowDeletedEventData RowDeletedEventData) core.Event {
	return core.NewEvent("data.row.deleted", rowDeletedEventData).WithResourceID(resourceID)
}

// TableTruncatedEventData represents the payload for data.table.truncated.
type TableTruncatedEventData struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
}

// NewTableTruncatedEvent creates a typed event for table truncation.
func NewTableTruncatedEvent(resourceID string, tableTruncatedEventData TableTruncatedEventData) core.Event {
	return core.NewEvent("data.table.truncated", tableTruncatedEventData).WithResourceID(resourceID)
}

// SQLExecutedEventData represents the payload for data.sql.executed.
type SQLExecutedEventData struct {
	Query        string `json:"query"`
	RowsAffected int64  `json:"rows_affected"`
}

// NewSQLExecutedEvent creates a typed event for SQL scratchpad execution.
func NewSQLExecutedEvent(resourceID string, sqlExecutedEventData SQLExecutedEventData) core.Event {
	return core.NewEvent("data.sql.executed", sqlExecutedEventData).WithResourceID(resourceID)
}

// FunctionExecutedEventData represents the payload for data.function.executed.
type FunctionExecutedEventData struct {
	Schema   string         `json:"schema"`
	Function string         `json:"function"`
	Args     map[string]any `json:"args,omitempty"`
}

// NewFunctionExecutedEvent creates a typed event for stored RPC function execution.
func NewFunctionExecutedEvent(resourceID string, functionExecutedEventData FunctionExecutedEventData) core.Event {
	return core.NewEvent("data.function.executed", functionExecutedEventData).WithResourceID(resourceID)
}

// KVSetEventData represents the payload for data.kv.set.
type KVSetEventData struct {
	Key string `json:"key"`
	TTL int    `json:"ttl,omitempty"`
}

// NewKVSetEvent creates a typed event for KV key creation or update.
func NewKVSetEvent(resourceID string, kvSetEventData KVSetEventData) core.Event {
	return core.NewEvent("data.kv.set", kvSetEventData).WithResourceID(resourceID)
}

// KVDeletedEventData represents the payload for data.kv.deleted.
type KVDeletedEventData struct {
	Key string `json:"key"`
}

// NewKVDeletedEvent creates a typed event for KV key deletion.
func NewKVDeletedEvent(resourceID string, kvDeletedEventData KVDeletedEventData) core.Event {
	return core.NewEvent("data.kv.deleted", kvDeletedEventData).WithResourceID(resourceID)
}

// KVTouchedEventData represents the payload for data.kv.touched.
type KVTouchedEventData struct {
	Key string `json:"key"`
	TTL int    `json:"ttl"`
}

// NewKVTouchedEvent creates a typed event for KV key expiration updates.
func NewKVTouchedEvent(resourceID string, kvTouchedEventData KVTouchedEventData) core.Event {
	return core.NewEvent("data.kv.touched", kvTouchedEventData).WithResourceID(resourceID)
}
