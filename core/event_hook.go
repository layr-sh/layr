package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"uuid"
)

// Standard Event Hook errors.
var (
	ErrEventHookNotFound         = errors.New("event hook not found")
	ErrEventHookDeliveryNotFound = errors.New("event hook delivery not found")
)

const (
	// EventHookDriverSQL designates a PostgreSQL stored procedure hook.
	EventHookDriverSQL = "sql"
	// EventHookDriverHTTP designates an HTTP webhook endpoint.
	EventHookDriverHTTP = "http"

	maxLoggedResponseBodyBytes = 1024
	defaultHookRetries         = 3
	defaultHookTimeoutSeconds  = 10
	maxHookTimeoutSeconds      = 60
)

var sqlIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?$`)

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

// EventHookManager manages event hooks and coordinates delivery dispatching.
type EventHookManager struct {
	db               *DatabasePool
	cryptoKeyManager *CryptoKeyManager
	eventBus         *EventBus
	httpClient       *http.Client
}

// NewEventHookManager creates a new EventHookManager.
func NewEventHookManager(db *DatabasePool, cryptoKeyManager *CryptoKeyManager, eventBus *EventBus) *EventHookManager {
	return &EventHookManager{
		db:               db,
		cryptoKeyManager: cryptoKeyManager,
		eventBus:         eventBus,
	}
}

// SetHTTPClient overrides the default HTTP client used for HTTP hook dispatches (useful in testing).
func (eventHookManager *EventHookManager) SetHTTPClient(httpClient *http.Client) {
	eventHookManager.httpClient = httpClient
}

// Create inserts a new event hook into core.event_hooks.
func (eventHookManager *EventHookManager) Create(ctx context.Context, createEventHookInput CreateEventHookInput) (*EventHook, error) {
	log.Debugf("creating event hook %s", createEventHookInput.Name)

	name := strings.TrimSpace(createEventHookInput.Name)
	if name == "" {
		return nil, fmt.Errorf("event hook name is required")
	}

	driver := strings.ToLower(strings.TrimSpace(createEventHookInput.Driver))
	if err := validateHookDriver(driver); err != nil {
		return nil, err
	}

	var sqlFunctionName *string
	var httpTargetURL *string
	var encryptedSecret *string

	switch driver {
	case EventHookDriverSQL:
		if createEventHookInput.SQLFunctionName == nil || strings.TrimSpace(*createEventHookInput.SQLFunctionName) == "" {
			return nil, fmt.Errorf("sql_function_name is required for sql driver")
		}
		trimmedFunction := strings.TrimSpace(*createEventHookInput.SQLFunctionName)
		if err := validateSQLFunctionName(trimmedFunction); err != nil {
			return nil, err
		}
		sqlFunctionName = &trimmedFunction

		if createEventHookInput.HTTPTargetURL != nil && strings.TrimSpace(*createEventHookInput.HTTPTargetURL) != "" {
			return nil, fmt.Errorf("http_target_url must not be provided for sql driver")
		}
		if createEventHookInput.SigningSecret != "" {
			return nil, fmt.Errorf("signing_secret must not be provided for sql driver")
		}
	case EventHookDriverHTTP:
		if createEventHookInput.HTTPTargetURL == nil || strings.TrimSpace(*createEventHookInput.HTTPTargetURL) == "" {
			return nil, fmt.Errorf("http_target_url is required for http driver")
		}
		trimmedURL := strings.TrimSpace(*createEventHookInput.HTTPTargetURL)
		if err := validateHookTargetURL(trimmedURL); err != nil {
			return nil, err
		}
		httpTargetURL = &trimmedURL

		if createEventHookInput.SQLFunctionName != nil && strings.TrimSpace(*createEventHookInput.SQLFunctionName) != "" {
			return nil, fmt.Errorf("sql_function_name must not be provided for http driver")
		}

		if createEventHookInput.SigningSecret != "" {
			if eventHookManager.cryptoKeyManager == nil {
				return nil, fmt.Errorf("cryptographic key manager is not configured")
			}
			encrypted, encryptFieldErr := eventHookManager.cryptoKeyManager.EncryptField([]byte(createEventHookInput.SigningSecret))
			if encryptFieldErr != nil {
				return nil, fmt.Errorf("failed to encrypt signing secret: %w", encryptFieldErr)
			}
			encryptedSecret = &encrypted
		}
	}

	eventTypes := createEventHookInput.EventTypes
	if len(eventTypes) == 0 {
		return nil, fmt.Errorf("at least one event_type pattern is required")
	}
	for _, pattern := range eventTypes {
		if strings.TrimSpace(pattern) == "" {
			return nil, fmt.Errorf("event_types cannot contain empty patterns")
		}
	}

	isEnabled := true
	if createEventHookInput.IsEnabled != nil {
		isEnabled = *createEventHookInput.IsEnabled
	}

	maxRetries := defaultHookRetries
	if createEventHookInput.MaxRetries != nil {
		if *createEventHookInput.MaxRetries < 0 {
			maxRetries = 0
		} else if *createEventHookInput.MaxRetries > 10 {
			maxRetries = 10
		} else {
			maxRetries = *createEventHookInput.MaxRetries
		}
	}

	timeoutSeconds := defaultHookTimeoutSeconds
	if createEventHookInput.TimeoutSeconds != nil {
		if *createEventHookInput.TimeoutSeconds < 1 {
			timeoutSeconds = 1
		} else if *createEventHookInput.TimeoutSeconds > maxHookTimeoutSeconds {
			timeoutSeconds = maxHookTimeoutSeconds
		} else {
			timeoutSeconds = *createEventHookInput.TimeoutSeconds
		}
	}

	if eventHookManager.db == nil {
		return nil, fmt.Errorf("database pool is not available")
	}

	hookID := uuid.NewV7()
	now := time.Now().UTC()

	query := `
		INSERT INTO core.event_hooks (
			id, name, driver, sql_function_name, http_target_url, http_encrypted_signing_secret, event_types, is_enabled, max_retries, timeout_seconds, created_at, last_updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11
		)
		RETURNING id, name, driver, sql_function_name, http_target_url, http_encrypted_signing_secret, event_types, is_enabled, max_retries, timeout_seconds, created_at, last_updated_at
	`

	var eventHook EventHook
	err := eventHookManager.db.QueryRow(ctx, query,
		hookID, name, driver, sqlFunctionName, httpTargetURL, encryptedSecret, eventTypes, isEnabled, maxRetries, timeoutSeconds, now,
	).Scan(
		&eventHook.ID, &eventHook.Name, &eventHook.Driver, &eventHook.SQLFunctionName, &eventHook.HTTPTargetURL, &eventHook.HTTPEncryptedSigningSecret, &eventHook.EventTypes, &eventHook.IsEnabled, &eventHook.MaxRetries, &eventHook.TimeoutSeconds, &eventHook.CreatedAt, &eventHook.LastUpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert event hook: %w", err)
	}

	eventHook.SigningSecretConfigured = (eventHook.HTTPEncryptedSigningSecret != nil && *eventHook.HTTPEncryptedSigningSecret != "")
	log.Tracef("created event hook %s successfully", eventHook.ID)
	return &eventHook, nil
}

// Get retrieves an event hook by its UUID.
func (eventHookManager *EventHookManager) Get(ctx context.Context, eventHookUUID uuid.UUID) (*EventHook, error) {
	log.Tracef("retrieving event hook %s", eventHookUUID)
	if eventHookManager.db == nil {
		return nil, fmt.Errorf("database pool is not available")
	}

	query := `
		SELECT id, name, driver, sql_function_name, http_target_url, http_encrypted_signing_secret, event_types, is_enabled, max_retries, timeout_seconds, created_at, last_updated_at
		FROM core.event_hooks
		WHERE id = $1
	`

	var eventHook EventHook
	err := eventHookManager.db.QueryRow(ctx, query, eventHookUUID).Scan(
		&eventHook.ID, &eventHook.Name, &eventHook.Driver, &eventHook.SQLFunctionName, &eventHook.HTTPTargetURL, &eventHook.HTTPEncryptedSigningSecret, &eventHook.EventTypes, &eventHook.IsEnabled, &eventHook.MaxRetries, &eventHook.TimeoutSeconds, &eventHook.CreatedAt, &eventHook.LastUpdatedAt,
	)
	if err != nil {
		return nil, ErrEventHookNotFound
	}

	eventHook.SigningSecretConfigured = (eventHook.HTTPEncryptedSigningSecret != nil && *eventHook.HTTPEncryptedSigningSecret != "")
	return &eventHook, nil
}

// List queries event hooks with optional filtering by driver and enabled status.
func (eventHookManager *EventHookManager) List(ctx context.Context, eventHookFilter EventHookFilter) ([]EventHook, error) {
	log.Tracef("listing event hooks")
	if eventHookManager.db == nil {
		return nil, fmt.Errorf("database pool is not available")
	}

	var conditions []string
	var args []any
	argIndex := 1

	if eventHookFilter.Driver != nil && *eventHookFilter.Driver != "" {
		conditions = append(conditions, fmt.Sprintf("driver = $%d", argIndex))
		args = append(args, strings.ToLower(strings.TrimSpace(*eventHookFilter.Driver)))
		argIndex++
	}

	if eventHookFilter.IsEnabled != nil {
		conditions = append(conditions, fmt.Sprintf("is_enabled = $%d", argIndex))
		args = append(args, *eventHookFilter.IsEnabled)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, name, driver, sql_function_name, http_target_url, http_encrypted_signing_secret, event_types, is_enabled, max_retries, timeout_seconds, created_at, last_updated_at
		FROM core.event_hooks
		%s
		ORDER BY created_at DESC
	`, whereClause)

	rows, err := eventHookManager.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query event hooks: %w", err)
	}
	defer rows.Close()

	results := make([]EventHook, 0)
	for rows.Next() {
		var eventHook EventHook
		_ = rows.Scan(
			&eventHook.ID, &eventHook.Name, &eventHook.Driver, &eventHook.SQLFunctionName, &eventHook.HTTPTargetURL, &eventHook.HTTPEncryptedSigningSecret, &eventHook.EventTypes, &eventHook.IsEnabled, &eventHook.MaxRetries, &eventHook.TimeoutSeconds, &eventHook.CreatedAt, &eventHook.LastUpdatedAt,
		)
		eventHook.SigningSecretConfigured = (eventHook.HTTPEncryptedSigningSecret != nil && *eventHook.HTTPEncryptedSigningSecret != "")
		results = append(results, eventHook)
	}

	return results, nil
}

// Update modifies an existing event hook in core.event_hooks.
func (eventHookManager *EventHookManager) Update(ctx context.Context, eventHookUUID uuid.UUID, updateEventHookInput UpdateEventHookInput) (*EventHook, error) {
	log.Debugf("updating event hook %s", eventHookUUID)
	currentEventHook, err := eventHookManager.Get(ctx, eventHookUUID)
	if err != nil {
		return nil, err
	}

	name := currentEventHook.Name
	if updateEventHookInput.Name != nil {
		trimmedName := strings.TrimSpace(*updateEventHookInput.Name)
		if trimmedName == "" {
			return nil, fmt.Errorf("event hook name cannot be empty")
		}
		name = trimmedName
	}

	driver := currentEventHook.Driver
	if updateEventHookInput.Driver != nil {
		trimmedDriver := strings.ToLower(strings.TrimSpace(*updateEventHookInput.Driver))
		if driverErr := validateHookDriver(trimmedDriver); driverErr != nil {
			return nil, driverErr
		}
		driver = trimmedDriver
	}

	sqlFunctionName := currentEventHook.SQLFunctionName
	if updateEventHookInput.SQLFunctionName != nil {
		trimmedFunction := strings.TrimSpace(*updateEventHookInput.SQLFunctionName)
		if trimmedFunction != "" {
			if sqlErr := validateSQLFunctionName(trimmedFunction); sqlErr != nil {
				return nil, sqlErr
			}
			sqlFunctionName = &trimmedFunction
		} else {
			sqlFunctionName = nil
		}
	}

	httpTargetURL := currentEventHook.HTTPTargetURL
	if updateEventHookInput.HTTPTargetURL != nil {
		trimmedURL := strings.TrimSpace(*updateEventHookInput.HTTPTargetURL)
		if trimmedURL != "" {
			if urlErr := validateHookTargetURL(trimmedURL); urlErr != nil {
				return nil, urlErr
			}
			httpTargetURL = &trimmedURL
		} else {
			httpTargetURL = nil
		}
	}

	encryptedSecret := currentEventHook.HTTPEncryptedSigningSecret
	if updateEventHookInput.SigningSecret != nil {
		secretText := strings.TrimSpace(*updateEventHookInput.SigningSecret)
		if secretText != "" {
			if eventHookManager.cryptoKeyManager == nil {
				return nil, fmt.Errorf("cryptographic key manager is not configured")
			}
			encrypted, encryptFieldErr := eventHookManager.cryptoKeyManager.EncryptField([]byte(secretText))
			if encryptFieldErr != nil {
				return nil, fmt.Errorf("failed to encrypt signing secret: %w", encryptFieldErr)
			}
			encryptedSecret = &encrypted
		} else {
			encryptedSecret = nil
		}
	}

	// Validate driver consistency
	switch driver {
	case EventHookDriverSQL:
		if sqlFunctionName == nil || *sqlFunctionName == "" {
			return nil, fmt.Errorf("sql_function_name is required for sql driver")
		}
		httpTargetURL = nil
		encryptedSecret = nil
	case EventHookDriverHTTP:
		if httpTargetURL == nil || *httpTargetURL == "" {
			return nil, fmt.Errorf("http_target_url is required for http driver")
		}
		sqlFunctionName = nil
	}

	eventTypes := currentEventHook.EventTypes
	if updateEventHookInput.EventTypes != nil {
		eventTypes = updateEventHookInput.EventTypes
	}

	isEnabled := currentEventHook.IsEnabled
	if updateEventHookInput.IsEnabled != nil {
		isEnabled = *updateEventHookInput.IsEnabled
	}

	maxRetries := currentEventHook.MaxRetries
	if updateEventHookInput.MaxRetries != nil {
		if *updateEventHookInput.MaxRetries < 0 {
			maxRetries = 0
		} else if *updateEventHookInput.MaxRetries > 10 {
			maxRetries = 10
		} else {
			maxRetries = *updateEventHookInput.MaxRetries
		}
	}

	timeoutSeconds := currentEventHook.TimeoutSeconds
	if updateEventHookInput.TimeoutSeconds != nil {
		if *updateEventHookInput.TimeoutSeconds < 1 {
			timeoutSeconds = 1
		} else if *updateEventHookInput.TimeoutSeconds > maxHookTimeoutSeconds {
			timeoutSeconds = maxHookTimeoutSeconds
		} else {
			timeoutSeconds = *updateEventHookInput.TimeoutSeconds
		}
	}

	now := time.Now().UTC()
	query := `
		UPDATE core.event_hooks
		SET name = $1, driver = $2, sql_function_name = $3, http_target_url = $4, http_encrypted_signing_secret = $5, event_types = $6, is_enabled = $7, max_retries = $8, timeout_seconds = $9, last_updated_at = $10
		WHERE id = $11
		RETURNING id, name, driver, sql_function_name, http_target_url, http_encrypted_signing_secret, event_types, is_enabled, max_retries, timeout_seconds, created_at, last_updated_at
	`

	var updatedEventHook EventHook
	_ = eventHookManager.db.QueryRow(ctx, query,
		name, driver, sqlFunctionName, httpTargetURL, encryptedSecret, eventTypes, isEnabled, maxRetries, timeoutSeconds, now, eventHookUUID,
	).Scan(
		&updatedEventHook.ID, &updatedEventHook.Name, &updatedEventHook.Driver, &updatedEventHook.SQLFunctionName, &updatedEventHook.HTTPTargetURL, &updatedEventHook.HTTPEncryptedSigningSecret, &updatedEventHook.EventTypes, &updatedEventHook.IsEnabled, &updatedEventHook.MaxRetries, &updatedEventHook.TimeoutSeconds, &updatedEventHook.CreatedAt, &updatedEventHook.LastUpdatedAt,
	)

	updatedEventHook.SigningSecretConfigured = (updatedEventHook.HTTPEncryptedSigningSecret != nil && *updatedEventHook.HTTPEncryptedSigningSecret != "")
	log.Tracef("updated event hook %s successfully", eventHookUUID)
	return &updatedEventHook, nil
}

// Delete removes an event hook by UUID.
func (eventHookManager *EventHookManager) Delete(ctx context.Context, eventHookUUID uuid.UUID) error {
	log.Debugf("deleting event hook %s", eventHookUUID)
	if eventHookManager.db == nil {
		return fmt.Errorf("database pool is not available")
	}

	result, err := eventHookManager.db.Exec(ctx, "DELETE FROM core.event_hooks WHERE id = $1", eventHookUUID)
	if err != nil {
		return fmt.Errorf("failed to delete event hook: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrEventHookNotFound
	}

	log.Tracef("deleted event hook %s successfully", eventHookUUID)
	return nil
}

// ListDeliveries retrieves the delivery history for a specific event hook.
func (eventHookManager *EventHookManager) ListDeliveries(ctx context.Context, eventHookUUID uuid.UUID) ([]EventHookDelivery, error) {
	log.Tracef("listing deliveries for event hook %s", eventHookUUID)
	if eventHookManager.db == nil {
		return nil, fmt.Errorf("database pool is not available")
	}

	query := `
		SELECT id, event_hook_id, event_id, event_type, payload, http_response_status, result, error_message, attempt_count, duration_ms, is_delivered, delivered_at, created_at
		FROM core.event_hook_deliveries
		WHERE event_hook_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`

	rows, err := eventHookManager.db.Query(ctx, query, eventHookUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list hook deliveries: %w", err)
	}
	defer rows.Close()

	results := make([]EventHookDelivery, 0)
	for rows.Next() {
		var eventHookDelivery EventHookDelivery
		var payloadRaw []byte
		_ = rows.Scan(
			&eventHookDelivery.ID, &eventHookDelivery.EventHookID, &eventHookDelivery.EventID, &eventHookDelivery.EventType, &payloadRaw, &eventHookDelivery.HTTPResponseStatus, &eventHookDelivery.Result, &eventHookDelivery.ErrorMessage, &eventHookDelivery.AttemptCount, &eventHookDelivery.DurationMs, &eventHookDelivery.IsDelivered, &eventHookDelivery.DeliveredAt, &eventHookDelivery.CreatedAt,
		)
		_ = json.Unmarshal(payloadRaw, &eventHookDelivery.Payload)
		results = append(results, eventHookDelivery)
	}

	return results, nil
}

// GetDelivery retrieves a single delivery by hook ID and delivery ID.
func (eventHookManager *EventHookManager) GetDelivery(ctx context.Context, eventHookUUID, deliveryUUID uuid.UUID) (*EventHookDelivery, error) {
	log.Tracef("retrieving delivery %s for hook %s", deliveryUUID, eventHookUUID)
	if eventHookManager.db == nil {
		return nil, fmt.Errorf("database pool is not available")
	}

	query := `
		SELECT id, event_hook_id, event_id, event_type, payload, http_response_status, result, error_message, attempt_count, duration_ms, is_delivered, delivered_at, created_at
		FROM core.event_hook_deliveries
		WHERE id = $1 AND event_hook_id = $2
	`

	var eventHookDelivery EventHookDelivery
	var payloadRaw []byte
	err := eventHookManager.db.QueryRow(ctx, query, deliveryUUID, eventHookUUID).Scan(
		&eventHookDelivery.ID, &eventHookDelivery.EventHookID, &eventHookDelivery.EventID, &eventHookDelivery.EventType, &payloadRaw, &eventHookDelivery.HTTPResponseStatus, &eventHookDelivery.Result, &eventHookDelivery.ErrorMessage, &eventHookDelivery.AttemptCount, &eventHookDelivery.DurationMs, &eventHookDelivery.IsDelivered, &eventHookDelivery.DeliveredAt, &eventHookDelivery.CreatedAt,
	)
	if err != nil {
		return nil, ErrEventHookDeliveryNotFound
	}
	_ = json.Unmarshal(payloadRaw, &eventHookDelivery.Payload)
	return &eventHookDelivery, nil
}

// RetryDelivery manually redrives a past event hook delivery attempt.
func (eventHookManager *EventHookManager) RetryDelivery(ctx context.Context, deliveryUUID uuid.UUID) (*EventHookDelivery, error) {
	log.Debugf("retrying delivery %s", deliveryUUID)
	if eventHookManager.db == nil {
		return nil, fmt.Errorf("database pool is not available")
	}

	query := `
		SELECT id, event_hook_id, event_id, event_type, payload
		FROM core.event_hook_deliveries
		WHERE id = $1
	`

	var eventHookDelivery EventHookDelivery
	var payloadRaw []byte
	err := eventHookManager.db.QueryRow(ctx, query, deliveryUUID).Scan(
		&eventHookDelivery.ID, &eventHookDelivery.EventHookID, &eventHookDelivery.EventID, &eventHookDelivery.EventType, &payloadRaw,
	)
	if err != nil {
		return nil, ErrEventHookDeliveryNotFound
	}

	eventHook, err := eventHookManager.Get(ctx, eventHookDelivery.EventHookID)
	if err != nil {
		return nil, err
	}

	var event Event
	if unmarshalErr := json.Unmarshal(payloadRaw, &event); unmarshalErr != nil || event.ID == uuid.Nil() {
		event = Event{
			ID:   eventHookDelivery.EventID,
			Type: eventHookDelivery.EventType,
		}
	}

	return eventHookManager.DeliverWithID(ctx, *eventHook, event, deliveryUUID)
}

// Dispatch finds active matching event hooks and executes deliveries.
func (eventHookManager *EventHookManager) Dispatch(ctx context.Context, event Event) {
	if eventHookManager.db == nil {
		return
	}

	query := `
		SELECT id, name, driver, sql_function_name, http_target_url, http_encrypted_signing_secret, event_types, is_enabled, max_retries, timeout_seconds, created_at, last_updated_at
		FROM core.event_hooks
		WHERE is_enabled = true
	`

	rows, err := eventHookManager.db.Query(ctx, query)
	if err != nil {
		log.Debugf("failed to query event hooks for event %s (%s): %v", event.ID, event.Type, err)
		return
	}
	defer rows.Close()

	var matchedHooks []EventHook
	for rows.Next() {
		var eventHook EventHook
		_ = rows.Scan(
			&eventHook.ID, &eventHook.Name, &eventHook.Driver, &eventHook.SQLFunctionName, &eventHook.HTTPTargetURL, &eventHook.HTTPEncryptedSigningSecret, &eventHook.EventTypes, &eventHook.IsEnabled, &eventHook.MaxRetries, &eventHook.TimeoutSeconds, &eventHook.CreatedAt, &eventHook.LastUpdatedAt,
		)

		for _, pattern := range eventHook.EventTypes {
			if MatchEventPattern(pattern, event.Type) {
				matchedHooks = append(matchedHooks, eventHook)
				break
			}
		}
	}

	for _, matchedEventHook := range matchedHooks {
		candidateEventHook := matchedEventHook
		go func(hookCtx context.Context, targetEventHook EventHook) {
			_, _ = eventHookManager.Deliver(hookCtx, targetEventHook, event)
		}(context.WithoutCancel(ctx), candidateEventHook)
	}
}

// Deliver executes delivery attempts against the hook's driver with automatic retries and records the audit history.
func (eventHookManager *EventHookManager) Deliver(ctx context.Context, eventHook EventHook, event Event) (*EventHookDelivery, error) {
	deliveryID := uuid.NewV7()
	return eventHookManager.DeliverWithID(ctx, eventHook, event, deliveryID)
}

// DeliverWithID executes delivery against a specific delivery ID (used for initial dispatches and manual retries).
func (eventHookManager *EventHookManager) DeliverWithID(ctx context.Context, eventHook EventHook, event Event, deliveryID uuid.UUID) (*EventHookDelivery, error) {
	maxRetries := eventHook.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultHookRetries
	}
	timeoutSeconds := eventHook.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultHookTimeoutSeconds
	}

	eventBytes, _ := json.Marshal(event)

	start := time.Now()
	attempt := 0
	var lastStatus *int
	var lastResult *string
	var lastErr *string
	var isDelivered bool
	var deliveredAt *time.Time

	for attempt < maxRetries {
		attempt++
		log.Tracef("hook %s delivery attempt %d/%d (driver: %s)", eventHook.ID, attempt, maxRetries, eventHook.Driver)

		if eventHook.Driver == EventHookDriverSQL {
			status, resultText, err := eventHookManager.executeSQLAttempt(ctx, eventHook, eventBytes, timeoutSeconds)
			lastStatus = status
			lastResult = resultText
			if err == nil {
				isDelivered = true
				now := time.Now().UTC()
				deliveredAt = &now
				lastErr = nil
				break
			}

			errMsg := err.Error()
			lastErr = &errMsg
			if !isTransientDatabaseError(err) {
				log.Tracef("hook %s permanent sql error: %v, skipping retries", eventHook.ID, err)
				break
			}
			if !sleepWithContext(ctx, CalculateHookBackoff(attempt)) {
				break
			}
		} else if eventHook.Driver == EventHookDriverHTTP {
			status, responseText, isClientError, err := eventHookManager.executeHTTPAttempt(ctx, eventHook, event, eventBytes, deliveryID, timeoutSeconds)
			lastStatus = status
			lastResult = responseText

			if err == nil && status != nil && *status >= 200 && *status < 300 {
				isDelivered = true
				now := time.Now().UTC()
				deliveredAt = &now
				lastErr = nil
				break
			}

			if err != nil {
				errMsg := err.Error()
				lastErr = &errMsg
			} else if status != nil {
				errMsg := fmt.Sprintf("HTTP error status %d", *status)
				lastErr = &errMsg
			}

			if isClientError {
				log.Tracef("hook %s HTTP client error, skipping retries", eventHook.ID)
				break
			}

			if !sleepWithContext(ctx, CalculateHookBackoff(attempt)) {
				break
			}
		}
	}

	durationMs := time.Since(start).Milliseconds()

	// Persist delivery outcome using context.WithoutCancel so recording history is never cancelled
	historyCtx := context.WithoutCancel(ctx)
	if eventHookManager.db != nil {
		upsertQuery := `
			INSERT INTO core.event_hook_deliveries (
				id, event_hook_id, event_id, event_type, payload, http_response_status, result, error_message, attempt_count, duration_ms, is_delivered, delivered_at, created_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
			)
			ON CONFLICT (id) DO UPDATE SET
				http_response_status = EXCLUDED.http_response_status,
				result = EXCLUDED.result,
				error_message = EXCLUDED.error_message,
				attempt_count = EXCLUDED.attempt_count,
				duration_ms = EXCLUDED.duration_ms,
				is_delivered = EXCLUDED.is_delivered,
				delivered_at = EXCLUDED.delivered_at
		`
		_, _ = eventHookManager.db.Exec(historyCtx, upsertQuery,
			deliveryID, eventHook.ID, event.ID, event.Type, eventBytes, lastStatus, lastResult, lastErr, attempt, durationMs, isDelivered, deliveredAt, time.Now().UTC(),
		)
	}

	eventHookDelivery := &EventHookDelivery{
		ID:                 deliveryID,
		EventHookID:        eventHook.ID,
		EventID:            event.ID,
		EventType:          event.Type,
		Payload:            event,
		HTTPResponseStatus: lastStatus,
		Result:             lastResult,
		ErrorMessage:       lastErr,
		AttemptCount:       attempt,
		DurationMs:         durationMs,
		IsDelivered:        isDelivered,
		DeliveredAt:        deliveredAt,
		CreatedAt:          time.Now().UTC(),
	}

	if !isDelivered {
		if lastErr != nil {
			return eventHookDelivery, fmt.Errorf("hook delivery failed: %s", *lastErr) //nolint:nilnil
		}
		return eventHookDelivery, fmt.Errorf("hook delivery failed") //nolint:nilnil
	}

	return eventHookDelivery, nil
}

func (eventHookManager *EventHookManager) executeSQLAttempt(ctx context.Context, eventHook EventHook, eventBytes []byte, timeoutSeconds int) (*int, *string, error) {
	if eventHook.SQLFunctionName == nil || *eventHook.SQLFunctionName == "" {
		return nil, nil, fmt.Errorf("sql_function_name is missing")
	}
	if eventHookManager.db == nil {
		return nil, nil, fmt.Errorf("database pool is not available")
	}

	attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	query := fmt.Sprintf("SELECT * FROM %s($1::jsonb)", *eventHook.SQLFunctionName)
	rows, err := eventHookManager.db.Query(attemptCtx, query, string(eventBytes))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var resultString string
	if rows.Next() {
		var columnValue any
		if scanErr := rows.Scan(&columnValue); scanErr == nil && columnValue != nil {
			resultString = fmt.Sprintf("%v", columnValue)
		}
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, nil, rows.Err()
	}

	return nil, &resultString, nil
}

func (eventHookManager *EventHookManager) executeHTTPAttempt(ctx context.Context, eventHook EventHook, event Event, bodyBytes []byte, deliveryID uuid.UUID, timeoutSeconds int) (*int, *string, bool, error) {
	if eventHook.HTTPTargetURL == nil || *eventHook.HTTPTargetURL == "" {
		return nil, nil, true, fmt.Errorf("http_target_url is missing")
	}

	attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, *eventHook.HTTPTargetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, true, fmt.Errorf("failed to create http request: %w", err)
	}

	secret := ""
	if eventHook.HTTPEncryptedSigningSecret != nil && *eventHook.HTTPEncryptedSigningSecret != "" && eventHookManager.cryptoKeyManager != nil {
		decrypted, decryptFieldErr := eventHookManager.cryptoKeyManager.DecryptField(*eventHook.HTTPEncryptedSigningSecret)
		if decryptFieldErr == nil {
			secret = string(decrypted)
		}
	}

	timestamp := fmt.Sprintf("%d", event.CreatedAt.Unix())
	signature := ComputeHookSignature(secret, timestamp, bodyBytes)

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Layr-Event-Hook/1.0")
	request.Header.Set("X-Layr-Signature", signature)
	request.Header.Set("X-Layr-Timestamp", timestamp)
	request.Header.Set("X-Layr-Event", event.Type)
	request.Header.Set("X-Layr-Delivery-Id", deliveryID.String())

	client := eventHookManager.httpClient
	if client == nil {
		client = &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to execute http request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	statusCode := response.StatusCode
	var responseBuffer bytes.Buffer
	_, _ = responseBuffer.ReadFrom(response.Body)
	responseText := responseBuffer.String()
	if len(responseText) > maxLoggedResponseBodyBytes {
		responseText = responseText[:maxLoggedResponseBodyBytes]
	}

	isClientError := (statusCode >= 400 && statusCode < 500 && statusCode != http.StatusTooManyRequests)
	return &statusCode, &responseText, isClientError, nil
}

func validateHookDriver(driver string) error {
	if driver != EventHookDriverSQL && driver != EventHookDriverHTTP {
		return fmt.Errorf("invalid driver: %s (must be 'sql' or 'http')", driver)
	}
	return nil
}

func validateSQLFunctionName(functionName string) error {
	if !sqlIdentifierPattern.MatchString(functionName) {
		return fmt.Errorf("invalid sql_function_name identifier: %s", functionName)
	}
	return nil
}

func validateHookTargetURL(targetURL string) error {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("invalid webhook target url: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("webhook scheme must be http or https")
	}
	hostname := strings.ToLower(parsedURL.Hostname())
	if hostname == "" {
		return fmt.Errorf("webhook target hostname is required")
	}
	if hostname == "metadata.google.internal" || hostname == "metadata" {
		return fmt.Errorf("webhook target to metadata service is prohibited")
	}
	if parsedIP := net.ParseIP(hostname); parsedIP != nil {
		if parsedIP.IsLinkLocalUnicast() || parsedIP.IsLinkLocalMulticast() {
			return fmt.Errorf("webhook target to link-local address is prohibited")
		}
	}
	return nil
}

// ComputeHookSignature computes HMAC-SHA256 signature for payload verification.
func ComputeHookSignature(secret, timestamp string, payload []byte) string {
	message := fmt.Sprintf("%s.%s", timestamp, string(payload))
	hmacHash := hmac.New(sha256.New, []byte(secret))
	hmacHash.Write([]byte(message))
	return hex.EncodeToString(hmacHash.Sum(nil))
}

func isTransientDatabaseError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	// Transient PostgreSQL SQLSTATE error codes:
	// 40001: serialization_failure
	// 40P01: deadlock_detected
	// 57014: query_canceled
	if strings.Contains(errMsg, "40001") ||
		strings.Contains(errMsg, "40p01") ||
		strings.Contains(errMsg, "57014") ||
		strings.Contains(errMsg, "deadlock") ||
		strings.Contains(errMsg, "serialization failure") ||
		strings.Contains(errMsg, "connection reset") ||
		strings.Contains(errMsg, "broken pipe") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "server closed the connection") {
		return true
	}
	return false
}

func sleepWithContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
