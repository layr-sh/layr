package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataEventConstructorsUnit(t *testing.T) {
	t.Run("ConfigUpdatedEvent", func(t *testing.T) {
		event := NewConfigUpdatedEvent("data.config", ConfigUpdatedEventData(DefaultConfig()))
		assert.Equal(t, "data.config.updated", event.Type)
		assert.Equal(t, "updated", event.Action)
		assert.Equal(t, "data.config", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "data.config", *event.ResourceID)
	})

	t.Run("CacheFlushedEvent", func(t *testing.T) {
		now := time.Now().UTC()
		event := NewCacheFlushedEvent("*", CacheFlushedEventData{
			Pattern:   "*",
			FlushedAt: now,
		})
		assert.Equal(t, "data.cache.flushed", event.Type)
		assert.Equal(t, "flushed", event.Action)
		assert.Equal(t, "data.cache", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "*", *event.ResourceID)
		assert.Equal(t, "*", event.Data["pattern"])
	})

	t.Run("CacheInvalidatedEvent", func(t *testing.T) {
		event := NewCacheInvalidatedEvent("public.users", CacheInvalidatedEventData{
			Pattern:           "public.users:*",
			InvalidationCount: 5,
		})
		assert.Equal(t, "data.cache.invalidated", event.Type)
		assert.Equal(t, "invalidated", event.Action)
		assert.Equal(t, "data.cache", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "public.users", *event.ResourceID)
	})

	t.Run("TableCreatedEvent", func(t *testing.T) {
		event := NewTableCreatedEvent("public.users", TableCreatedEventData{
			Schema: "public",
			Table:  "users",
		})
		assert.Equal(t, "data.table.created", event.Type)
		assert.Equal(t, "created", event.Action)
		assert.Equal(t, "data.table", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "public.users", *event.ResourceID)
		assert.Equal(t, "public", event.Data["schema"])
		assert.Equal(t, "users", event.Data["table"])
	})

	t.Run("TableUpdatedEvent", func(t *testing.T) {
		event := NewTableUpdatedEvent("public.users", TableUpdatedEventData{
			Schema: "public",
			Table:  "users",
			Action: "add_column",
			Detail: "email",
		})
		assert.Equal(t, "data.table.updated", event.Type)
		assert.Equal(t, "updated", event.Action)
		assert.Equal(t, "data.table", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "public.users", *event.ResourceID)
	})

	t.Run("TableDeletedEvent", func(t *testing.T) {
		event := NewTableDeletedEvent("public.users", TableDeletedEventData{
			Schema: "public",
			Table:  "users",
		})
		assert.Equal(t, "data.table.deleted", event.Type)
		assert.Equal(t, "deleted", event.Action)
		assert.Equal(t, "data.table", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "public.users", *event.ResourceID)
	})

	t.Run("RowCreatedEvent", func(t *testing.T) {
		event := NewRowCreatedEvent("user-123", RowCreatedEventData{
			Schema:     "public",
			Table:      "users",
			ID:         "user-123",
			Properties: map[string]string{"name": "Alice"},
		})
		assert.Equal(t, "data.row.created", event.Type)
		assert.Equal(t, "created", event.Action)
		assert.Equal(t, "data.row", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "user-123", *event.ResourceID)
	})

	t.Run("RowUpdatedEvent", func(t *testing.T) {
		event := NewRowUpdatedEvent("user-123", RowUpdatedEventData{
			Schema:     "public",
			Table:      "users",
			ID:         "user-123",
			Properties: map[string]string{"name": "Alice Cooper"},
		})
		assert.Equal(t, "data.row.updated", event.Type)
		assert.Equal(t, "updated", event.Action)
		assert.Equal(t, "data.row", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "user-123", *event.ResourceID)
	})

	t.Run("RowDeletedEvent", func(t *testing.T) {
		event := NewRowDeletedEvent("user-123", RowDeletedEventData{
			Schema: "public",
			Table:  "users",
			ID:     "user-123",
		})
		assert.Equal(t, "data.row.deleted", event.Type)
		assert.Equal(t, "deleted", event.Action)
		assert.Equal(t, "data.row", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "user-123", *event.ResourceID)
	})
}
