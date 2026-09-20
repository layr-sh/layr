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
