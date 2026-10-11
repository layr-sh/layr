package core

import (
	"time"
	"uuid"
)

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

// ListEventHookDeliveriesResponse represents the response containing multiple event hook deliveries.
type ListEventHookDeliveriesResponse []EventHookDelivery

// ListEventsResponse represents the standardized paginated response containing multiple events.
type ListEventsResponse struct {
	Data []Event `json:"data"`
	PaginationOutput
}

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
