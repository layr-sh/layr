package data

import (
	"encoding/json"
	"time"

	"uuid"
)

// ConfigRecord represents a row in the data.config table.
type ConfigRecord struct {
	ID            uuid.UUID       `json:"id"`
	Key           string          `json:"key"`
	Value         json.RawMessage `json:"value"`
	LastUpdatedAt time.Time       `json:"last_updated_at"`
}

// TableRecord represents a dynamic row record in a database table.
type TableRecord struct {
	ID         string            `json:"id,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

// TableRowsResponse represents dynamic row query results.
type TableRowsResponse struct {
	Data  []TableRecord `json:"data"`
	Count *int          `json:"count,omitempty"`
}

// TableRowResponse represents a single row result.
type TableRowResponse struct {
	Data TableRecord `json:"data"`
}

// InsertRowPayload represents single or bulk row insert data.
type InsertRowPayload struct {
	Data []TableRecord `json:"data,omitempty"`
}

// UpdateRowPayload represents row partial update data.
type UpdateRowPayload struct {
	Data TableRecord `json:"data,omitempty"`
}

// ExecuteFunctionRequest represents stored function invocation parameters.
type ExecuteFunctionRequest struct {
	Args map[string]any `json:"args,omitempty"`
}

// ExecuteFunctionResponse represents stored function invocation results.
type ExecuteFunctionResponse struct {
	Result any `json:"result"`
}

// KVGetResponse represents an ephemeral KV retrieval response.
type KVGetResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// KVSetRequest represents an ephemeral KV set payload.
type KVSetRequest struct {
	Value string `json:"value"`
	TTL   int    `json:"ttl,omitempty"`
}

// KVSetResponse represents an ephemeral KV set response.
type KVSetResponse struct {
	Key     string `json:"key"`
	Status  string `json:"status"`
	Created bool   `json:"created,omitempty"`
	TTL     int    `json:"ttl,omitempty"`
}

// KVMGetRequest represents a multi-key get payload.
type KVMGetRequest struct {
	Keys []string `json:"keys"`
}

// KVMGetResponse represents a multi-key get response.
type KVMGetResponse struct {
	Values map[string]string `json:"values"`
}

// KVMSetRequest represents a batch key-value set payload.
type KVMSetRequest struct {
	Entries map[string]string `json:"entries"`
	TTL     int               `json:"ttl,omitempty"`
}

// KVMSetResponse represents a batch key-value set response.
type KVMSetResponse struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// KVIncrementRequest represents an atomic increment payload.
type KVIncrementRequest struct {
	Key  string `json:"key"`
	Step *int64 `json:"step,omitempty"`
	TTL  int    `json:"ttl,omitempty"`
}

// KVIncrementResponse represents an atomic increment response.
type KVIncrementResponse struct {
	Key   string `json:"key"`
	Value int64  `json:"value"`
}

// KVTouchRequest represents a TTL refresh payload for PATCH.
type KVTouchRequest struct {
	TTL int `json:"ttl"`
}

// KVTouchResponse represents a TTL refresh response.
type KVTouchResponse struct {
	Key    string `json:"key"`
	Status string `json:"status"`
	TTL    int    `json:"ttl"`
}

// GraphQLRequest represents standard GraphQL HTTP request payload.
type GraphQLRequest struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName,omitempty"`
	Variables     map[string]any `json:"variables,omitempty"`
}

// GraphQLLocation represents line and column in GraphQL query documents.
type GraphQLLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// GraphQLError represents a GraphQL execution or validation error.
type GraphQLError struct {
	Message    string            `json:"message"`
	Locations  []GraphQLLocation `json:"locations,omitempty"`
	Path       []any             `json:"path,omitempty"`
	Extensions map[string]any    `json:"extensions,omitempty"`
}

// GraphQLResponse represents standard GraphQL JSON response envelope.
type GraphQLResponse struct {
	Data   any            `json:"data,omitempty"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

// CDCEvent represents a PostgreSQL Change Data Capture event payload.
type CDCEvent struct {
	Schema          string         `json:"schema"`
	Table           string         `json:"table"`
	Action          string         `json:"action,omitempty"` // "INSERT", "UPDATE", "DELETE"
	Event           string         `json:"event,omitempty"`  // "INSERT", "UPDATE", "DELETE"
	CommitTimestamp time.Time      `json:"commit_timestamp"`
	Record          map[string]any `json:"record,omitempty"`
	OldRecord       map[string]any `json:"old_record,omitempty"`
	Truncated       bool           `json:"truncated,omitempty"`
}

// RealtimeClientMessage represents a message sent from WebSocket client to server.
type RealtimeClientMessage struct {
	Action  string          `json:"action"` // "subscribe", "unsubscribe", "ping", "presence"
	Topic   string          `json:"topic,omitempty"`
	Channel string          `json:"channel,omitempty"`
	Schema  string          `json:"schema,omitempty"`
	Table   string          `json:"table,omitempty"`
	Event   string          `json:"event,omitempty"`
	Filter  string          `json:"filter,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// RealtimeServerMessage represents a message sent from WebSocket server to client.
type RealtimeServerMessage struct {
	Type      string         `json:"type,omitempty"` // "event", "subscribed", "unsubscribed", "pong", "presence", "error"
	Status    string         `json:"status,omitempty"`
	Topic     string         `json:"topic,omitempty"`
	Channel   string         `json:"channel,omitempty"`
	Error     string         `json:"error,omitempty"`
	Event     string         `json:"event,omitempty"`
	Schema    string         `json:"schema,omitempty"`
	Table     string         `json:"table,omitempty"`
	Record    map[string]any `json:"record,omitempty"`
	OldRecord map[string]any `json:"old_record,omitempty"`
	Truncated bool           `json:"truncated,omitempty"`
	Payload   any            `json:"payload,omitempty"`
}
