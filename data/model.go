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
