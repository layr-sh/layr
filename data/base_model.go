package data

import (
	"encoding/json"
	"time"
)

// ConfigRecord represents a row in the data.config table.
type ConfigRecord struct {
	Key           string          `json:"key"`
	Value         json.RawMessage `json:"value"`
	LastUpdatedAt time.Time       `json:"last_updated_at"`
}

// Record represents a dynamic row record in a database table.
type Record struct {
	ID         string            `json:"id,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

// ListRecordsResponse represents dynamic row query results.
type ListRecordsResponse struct {
	Data  []Record `json:"data"`
	Count *int     `json:"count,omitempty"`
}

// GetRecordResponse represents a single row result.
type GetRecordResponse struct {
	Data Record `json:"data"`
}

// CreateRecordInput represents single or bulk row insert data.
type CreateRecordInput struct {
	Data []Record `json:"data,omitempty"`
}

// CreateRecordResponse represents single or bulk row insert results.
type CreateRecordResponse struct {
	Data  []Record `json:"data"`
	Count *int     `json:"count,omitempty"`
}

// UpdateRecordInput represents row partial update data.
type UpdateRecordInput struct {
	Data Record `json:"data,omitempty"`
}

// UpdateRecordResponse represents row update results.
type UpdateRecordResponse struct {
	Data Record `json:"data"`
}

// ExecuteFunctionInput represents stored function invocation parameters.
type ExecuteFunctionInput map[string]any

// ExecuteFunctionResponse represents stored function invocation results.
type ExecuteFunctionResponse any

// GetKVResponse represents an ephemeral KV retrieval response.
type GetKVResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SetKVInput represents an ephemeral KV set payload.
type SetKVInput struct {
	Value string `json:"value"`
	TTL   int    `json:"ttl,omitempty"`
}

// SetKVResponse represents an ephemeral KV set response.
type SetKVResponse struct {
	Key     string `json:"key"`
	Status  string `json:"status"`
	Created bool   `json:"created,omitempty"`
	TTL     int    `json:"ttl,omitempty"`
}

// GetMultipleKVInput represents a multi-key get payload.
type GetMultipleKVInput struct {
	Keys []string `json:"keys"`
}

// GetMultipleKVResponse represents a multi-key get response.
type GetMultipleKVResponse struct {
	Values map[string]string `json:"values"`
}

// SetMultipleKVInput represents a batch key-value set payload.
type SetMultipleKVInput struct {
	Entries map[string]string `json:"entries"`
	TTL     int               `json:"ttl,omitempty"`
}

// SetMultipleKVResponse represents a batch key-value set response.
type SetMultipleKVResponse struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// IncrementKVInput represents an atomic increment payload.
type IncrementKVInput struct {
	Key  string `json:"key"`
	Step *int64 `json:"step,omitempty"`
	TTL  int    `json:"ttl,omitempty"`
}

// IncrementKVResponse represents an atomic increment response.
type IncrementKVResponse struct {
	Key   string `json:"key"`
	Value int64  `json:"value"`
}

// TouchKVInput represents a TTL refresh payload for PATCH.
type TouchKVInput struct {
	TTL int `json:"ttl"`
}

// TouchKVResponse represents a TTL refresh response.
type TouchKVResponse struct {
	Key    string `json:"key"`
	Status string `json:"status"`
	TTL    int    `json:"ttl"`
}

// ExecuteGraphQLInput represents standard GraphQL HTTP request payload.
type ExecuteGraphQLInput struct {
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

// ExecuteGraphQLResponse represents standard GraphQL JSON response envelope.
type ExecuteGraphQLResponse struct {
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
