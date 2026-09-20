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

// ListServiceAccountsResponse represents the response containing multiple service accounts.
type ListServiceAccountsResponse []ServiceAccount

// CreateServiceAccountInput holds the input parameters for creating a new Service Account.
type CreateServiceAccountInput struct {
	ConsoleUserID *string    `json:"console_user_id,omitempty"`
	Name          string     `json:"name"`
	Description   *string    `json:"description,omitempty"`
	Scopes        []string   `json:"scopes"`
	AllowedIPs    []string   `json:"allowed_ips,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

// UpdateServiceAccountInput holds the update payload for a Service Account.
type UpdateServiceAccountInput struct {
	Name        *string    `json:"name,omitempty"`
	Description *string    `json:"description,omitempty"`
	Scopes      []string   `json:"scopes,omitempty"`
	IsEnabled   *bool      `json:"is_enabled,omitempty"`
	AllowedIPs  []string   `json:"allowed_ips,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// CreateServiceAccountResponse returned when creating a new Service Account, containing the plaintext secret.
type CreateServiceAccountResponse struct {
	ServiceAccount
	SecretKey string `json:"secret_key"`
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

// CreateEventHookInput holds parameters to create a new event hook.
type CreateEventHookInput struct {
	Name            string   `json:"name"`
	Driver          string   `json:"driver"`
	SQLFunctionName *string  `json:"sql_function_name,omitempty"`
	HTTPTargetURL   *string  `json:"http_target_url,omitempty"`
	SigningSecret   string   `json:"signing_secret,omitempty"`
	EventTypes      []string `json:"event_types"`
	IsEnabled       *bool    `json:"is_enabled,omitempty"`
	MaxRetries      *int     `json:"max_retries,omitempty"`
	TimeoutSeconds  *int     `json:"timeout_seconds,omitempty"`
}

// UpdateEventHookInput holds parameters to update an existing event hook.
type UpdateEventHookInput struct {
	Name            *string  `json:"name,omitempty"`
	Driver          *string  `json:"driver,omitempty"`
	SQLFunctionName *string  `json:"sql_function_name,omitempty"`
	HTTPTargetURL   *string  `json:"http_target_url,omitempty"`
	SigningSecret   *string  `json:"signing_secret,omitempty"`
	EventTypes      []string `json:"event_types,omitempty"`
	IsEnabled       *bool    `json:"is_enabled,omitempty"`
	MaxRetries      *int     `json:"max_retries,omitempty"`
	TimeoutSeconds  *int     `json:"timeout_seconds,omitempty"`
}

// EventHookFilter specifies query options when listing event hooks.
type EventHookFilter struct {
	Driver    *string `json:"driver,omitempty"`
	IsEnabled *bool   `json:"is_enabled,omitempty"`
}

// ListEventHooksResponse represents the response containing multiple event hooks.
type ListEventHooksResponse []EventHook

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

// ListEventHookDeliveriesResponse represents the response containing multiple event hook deliveries.
type ListEventHookDeliveriesResponse []EventHookDelivery

// ListEventsResponse represents the response containing multiple events.
type ListEventsResponse []Event

// EventFilter provides multi-field filtering and pagination options for querying events.
type EventFilter struct {
	Type         *string    `json:"type,omitempty"`
	ActorType    *string    `json:"actor_type,omitempty"`
	ActorID      *uuid.UUID `json:"actor_id,omitempty"`
	ResourceType *string    `json:"resource_type,omitempty"`
	ResourceID   *string    `json:"resource_id,omitempty"`
	StartDate    *time.Time `json:"start_date,omitempty"`
	EndDate      *time.Time `json:"end_date,omitempty"`
	Limit        int        `json:"limit,omitempty"`
	Offset       int        `json:"offset,omitempty"`
}
