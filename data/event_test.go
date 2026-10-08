package data

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataEventUnit(t *testing.T) {
	t.Parallel()

	t.Run("ConfigUpdatedEvent", func(t *testing.T) {
		t.Parallel()
		event := NewConfigUpdatedEvent("data.config", ConfigUpdatedEventData(DefaultConfig()))
		assert.Equal(t, "data.config.updated", event.Type)
		assert.Equal(t, "updated", event.Action)
		assert.Equal(t, "data.config", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "data.config", *event.ResourceID)
	})

	t.Run("CacheFlushedEvent", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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

	t.Run("TableTruncatedEvent", func(t *testing.T) {
		t.Parallel()
		event := NewTableTruncatedEvent("public.users", TableTruncatedEventData{
			Schema: "public",
			Table:  "users",
		})
		assert.Equal(t, "data.table.truncated", event.Type)
		assert.Equal(t, "truncated", event.Action)
		assert.Equal(t, "data.table", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "public.users", *event.ResourceID)
		assert.Equal(t, "public", event.Data["schema"])
		assert.Equal(t, "users", event.Data["table"])
	})

	t.Run("SQLExecutedEvent", func(t *testing.T) {
		t.Parallel()
		event := NewSQLExecutedEvent("raw_sql", SQLExecutedEventData{
			Query:        "SELECT * FROM users",
			RowsAffected: 10,
		})
		assert.Equal(t, "data.sql.executed", event.Type)
		assert.Equal(t, "executed", event.Action)
		assert.Equal(t, "data.sql", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "raw_sql", *event.ResourceID)
		assert.Equal(t, "SELECT * FROM users", event.Data["query"])
	})

	t.Run("FunctionExecutedEvent", func(t *testing.T) {
		t.Parallel()
		event := NewFunctionExecutedEvent("public.get_user", FunctionExecutedEventData{
			Schema:   "public",
			Function: "get_user",
			Args:     map[string]any{"id": 42},
		})
		assert.Equal(t, "data.function.executed", event.Type)
		assert.Equal(t, "executed", event.Action)
		assert.Equal(t, "data.function", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "public.get_user", *event.ResourceID)
		assert.Equal(t, "public", event.Data["schema"])
		assert.Equal(t, "get_user", event.Data["function"])
	})

	t.Run("KVSetEvent", func(t *testing.T) {
		t.Parallel()
		event := NewKVSetEvent("session:123", KVSetEventData{
			Key: "session:123",
			TTL: 3600,
		})
		assert.Equal(t, "data.kv.set", event.Type)
		assert.Equal(t, "set", event.Action)
		assert.Equal(t, "data.kv", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "session:123", *event.ResourceID)
		assert.Equal(t, "session:123", event.Data["key"])
	})

	t.Run("KVDeletedEvent", func(t *testing.T) {
		t.Parallel()
		event := NewKVDeletedEvent("session:123", KVDeletedEventData{
			Key: "session:123",
		})
		assert.Equal(t, "data.kv.deleted", event.Type)
		assert.Equal(t, "deleted", event.Action)
		assert.Equal(t, "data.kv", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "session:123", *event.ResourceID)
		assert.Equal(t, "session:123", event.Data["key"])
	})

	t.Run("KVTouchedEvent", func(t *testing.T) {
		t.Parallel()
		event := NewKVTouchedEvent("session:123", KVTouchedEventData{
			Key: "session:123",
			TTL: 7200,
		})
		assert.Equal(t, "data.kv.touched", event.Type)
		assert.Equal(t, "touched", event.Action)
		assert.Equal(t, "data.kv", event.ResourceType)
		require.NotNil(t, event.ResourceID)
		assert.Equal(t, "session:123", *event.ResourceID)
		assert.Equal(t, "session:123", event.Data["key"])
	})
}

func TestDataEventConstructorParameterSignaturesUnit(t *testing.T) {
	fileSet := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fileSet, "event.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse event.go: %v", err)
	}

	testedCount := 0
	for _, decl := range parsedFile.Decls {
		functionDeclaration, ok := decl.(*ast.FuncDecl)
		if !ok || functionDeclaration.Recv != nil {
			continue
		}
		name := functionDeclaration.Name.Name
		if !strings.HasPrefix(name, "New") || !strings.HasSuffix(name, "Event") || name == "NewEvent" {
			continue
		}

		testedCount++
		t.Run(name, func(t *testing.T) {
			params := functionDeclaration.Type.Params.List
			if len(params) == 0 {
				t.Fatalf("constructor %s has no parameters", name)
			}
			firstParamField := params[0]
			if len(firstParamField.Names) == 0 {
				t.Fatalf("constructor %s first parameter has no name", name)
			}
			paramName := firstParamField.Names[0].Name
			if paramName != "resourceID" {
				t.Errorf("constructor %s first parameter expected 'resourceID', got '%s'", name, paramName)
			}
		})
	}

	if testedCount == 0 {
		t.Fatal("no constructors found to test")
	}
}
