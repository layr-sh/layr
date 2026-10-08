package core

import (
	"time"

	"uuid"
)

// ServiceAccount represents a machine identity stored in core.service_accounts.
type ServiceAccount struct {
	ID            string     `json:"id"`
	ConsoleUserID *string    `json:"console_user_id,omitempty"`
	Name          string     `json:"name"`
	Description   *string    `json:"description,omitempty"`
	KeyPrefix     string     `json:"key_prefix"`
	KeyHash       string     `json:"-"`
	Scopes        []string   `json:"scopes"`
	IsEnabled     bool       `json:"is_enabled"`
	AllowedIPs    []string   `json:"allowed_ips,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LastUpdatedAt time.Time  `json:"last_updated_at"`
}

// EventHook represents a configured event hook stored in core.event_hooks.
type EventHook struct {
	ID                         uuid.UUID `json:"id"`
	Name                       string    `json:"name"`
	Driver                     string    `json:"driver"`
	SQLFunctionName            *string   `json:"sql_function_name,omitempty"`
	HTTPTargetURL              *string   `json:"http_target_url,omitempty"`
	HTTPEncryptedSigningSecret *string   `json:"-"`
	SigningSecretConfigured    bool      `json:"signing_secret_configured"`
	EventTypes                 []string  `json:"event_types"`
	IsEnabled                  bool      `json:"is_enabled"`
	MaxRetries                 int       `json:"max_retries"`
	TimeoutSeconds             int       `json:"timeout_seconds"`
	CreatedAt                  time.Time `json:"created_at"`
	LastUpdatedAt              time.Time `json:"last_updated_at"`
}

// EventHookDelivery represents an execution audit trail stored in core.event_hook_deliveries.
type EventHookDelivery struct {
	ID                 uuid.UUID  `json:"id"`
	EventHookID        uuid.UUID  `json:"event_hook_id"`
	EventID            uuid.UUID  `json:"event_id"`
	EventType          string     `json:"event_type"`
	Payload            any        `json:"payload"`
	HTTPResponseStatus *int       `json:"http_response_status,omitempty"`
	Result             *string    `json:"result,omitempty"`
	ErrorMessage       *string    `json:"error_message,omitempty"`
	AttemptCount       int        `json:"attempt_count"`
	DurationMs         int64      `json:"duration_ms"`
	IsDelivered        bool       `json:"is_delivered"`
	DeliveredAt        *time.Time `json:"delivered_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}
