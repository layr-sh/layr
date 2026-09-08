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
	"net/http"
	"strings"
	"sync"
	"time"
	"uuid"
)

// Standard Webhook Errors
var (
	ErrWebhookNotFound = errors.New("webhook subscription not found")
)

const (
	defaultDispatchChannelCapacity = 1000
	defaultDispatchTimeout         = 30 * time.Second
	maxLoggedResponseBodyBytes     = 1024
)

// WebhookEventEnvelope represents the canonical event emitted on the Layr Platform Event Bus.
type WebhookEventEnvelope struct {
	ID        string        `json:"id"`
	Event     string        `json:"event"`
	Timestamp time.Time     `json:"timestamp"`
	Service   string        `json:"service"`
	Resource  string        `json:"resource"`
	Action    string        `json:"action"`
	Actor     *WebhookActor `json:"actor,omitempty"`
	Data      any           `json:"data"`
}

// WebhookActor represents the principal that triggered the event.
type WebhookActor struct {
	Type string  `json:"type"` // "user", "service_account", "system"
	ID   string  `json:"id"`
	Role *string `json:"role,omitempty"`
}

// WebhookSubscription represents a webhook configuration stored in core.webhooks.
type WebhookSubscription struct {
	ID                      string    `json:"id"`
	Name                    string    `json:"name"`
	TargetURL               string    `json:"target_url"`
	Events                  []string  `json:"events"`
	IsEnabled               bool      `json:"is_enabled"`
	SigningSecretConfigured bool      `json:"signing_secret_configured"`
	SigningSecretMasked     bool      `json:"-"`
	SigningSecretEncrypted  string    `json:"-"`
	MaxRetries              int       `json:"max_retries"`
	TimeoutSeconds          int       `json:"timeout_seconds"`
	CreatedAt               time.Time `json:"created_at"`
	LastUpdatedAt           time.Time `json:"last_updated_at"`
}

// CreateWebhookInput holds parameters to create a new webhook subscription.
type CreateWebhookInput struct {
	Name           string   `json:"name"`
	TargetURL      string   `json:"target_url"`
	Events         []string `json:"events"`
	SigningSecret  string   `json:"signing_secret"`
	MaxRetries     *int     `json:"max_retries,omitempty"`
	TimeoutSeconds *int     `json:"timeout_seconds,omitempty"`
}

// UpdateWebhookInput holds parameters to update an existing webhook subscription.
type UpdateWebhookInput struct {
	Name           *string  `json:"name,omitempty"`
	TargetURL      *string  `json:"target_url,omitempty"`
	Events         []string `json:"events,omitempty"`
	SigningSecret  *string  `json:"signing_secret,omitempty"`
	IsEnabled      *bool    `json:"is_enabled,omitempty"`
	MaxRetries     *int     `json:"max_retries,omitempty"`
	TimeoutSeconds *int     `json:"timeout_seconds,omitempty"`
}

// WebhookDelivery represents an execution audit trail stored in core.webhook_deliveries.
type WebhookDelivery struct {
	ID             string     `json:"id"`
	WebhookID      string     `json:"webhook_id"`
	EventID        string     `json:"event_id"`
	EventType      string     `json:"event_type"`
	Payload        any        `json:"payload"`
	ResponseStatus *int       `json:"response_status,omitempty"`
	ResponseBody   *string    `json:"response_body,omitempty"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	AttemptCount   int        `json:"attempt_count"`
	DurationMs     int64      `json:"duration_ms"`
	IsDelivered    bool       `json:"is_delivered"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// WebhookEventHandler handles in-process event subscriptions.
type WebhookEventHandler func(ctx context.Context, event WebhookEventEnvelope) error

// WebhookEventBus coordinates event publishing and dispatching to webhooks and in-memory listeners.
type WebhookEventBus struct {
	rwMutex          sync.RWMutex
	subscribers      map[string][]WebhookEventHandler
	pool             *DatabasePool
	cryptoKeyManager *CryptoKeyManager
	httpClient       *http.Client
	dispatchChannel  chan WebhookEventEnvelope
	stopChannel      chan struct{}
	waitGroup        sync.WaitGroup
	context          context.Context
	contextCancel    context.CancelFunc
}

// NewWebhookEventBus initializes the WebhookEventBus and starts background worker goroutines.
func NewWebhookEventBus(pool *DatabasePool, cryptoKeyManager *CryptoKeyManager) *WebhookEventBus {
	ctx, cancel := context.WithCancel(context.Background())
	webhookEventBus := &WebhookEventBus{
		subscribers:      make(map[string][]WebhookEventHandler),
		pool:             pool,
		cryptoKeyManager: cryptoKeyManager,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		dispatchChannel: make(chan WebhookEventEnvelope, defaultDispatchChannelCapacity),
		stopChannel:     make(chan struct{}),
		context:         ctx,
		contextCancel:   cancel,
	}

	// Start 4 worker dispatchers
	for i := 0; i < 4; i++ {
		webhookEventBus.waitGroup.Add(1)
		go webhookEventBus.worker()
	}

	return webhookEventBus
}

// Subscribe registers an in-memory handler for an event pattern.
func (webhookEventBus *WebhookEventBus) Subscribe(pattern string, handler WebhookEventHandler) {
	webhookEventBus.rwMutex.Lock()
	defer webhookEventBus.rwMutex.Unlock()
	webhookEventBus.subscribers[pattern] = append(webhookEventBus.subscribers[pattern], handler)
}

// Publish broadcasts an event to in-memory listeners and enqueues it for webhook dispatch.
func (webhookEventBus *WebhookEventBus) Publish(ctx context.Context, event WebhookEventEnvelope) {
	if event.ID == "" {
		event.ID = uuid.NewV7().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Dispatch to in-memory subscribers synchronously or concurrently
	webhookEventBus.rwMutex.RLock()
	var matchedHandlers []WebhookEventHandler
	for pattern, handlers := range webhookEventBus.subscribers {
		if matchWebhookEventPattern(pattern, event.Event) {
			matchedHandlers = append(matchedHandlers, handlers...)
		}
	}
	webhookEventBus.rwMutex.RUnlock()

	for _, subscriberHandler := range matchedHandlers {
		webhookEventBus.waitGroup.Add(1)
		go func(handler WebhookEventHandler) {
			defer webhookEventBus.waitGroup.Done()
			_ = handler(ctx, event)
		}(subscriberHandler)
	}

	// Enqueue for webhook processing
	select {
	case webhookEventBus.dispatchChannel <- event:
	default:
		log.Warnf("dispatch buffer full, dropping event %s (%s)", event.ID, event.Event)
	}
}

// Close stops all background dispatch workers cleanly.
func (webhookEventBus *WebhookEventBus) Close() {
	close(webhookEventBus.stopChannel)
	webhookEventBus.contextCancel()
	webhookEventBus.waitGroup.Wait()
}

func (webhookEventBus *WebhookEventBus) worker() {
	defer webhookEventBus.waitGroup.Done()
	for {
		select {
		case <-webhookEventBus.stopChannel:
			return
		case event := <-webhookEventBus.dispatchChannel:
			webhookEventBus.dispatch(webhookEventBus.context, event)
		}
	}
}

func (webhookEventBus *WebhookEventBus) dispatch(ctx context.Context, event WebhookEventEnvelope) {
	if webhookEventBus.pool == nil {
		return
	}

	dispatchContext, dispatchCancel := context.WithTimeout(ctx, defaultDispatchTimeout)
	defer dispatchCancel()

	// Query active webhooks matching this event
	query := `
		SELECT id, name, target_url, events, signing_secret_enc, max_retries, timeout_seconds
		FROM core.webhooks
		WHERE is_enabled = true
	`
	rows, err := webhookEventBus.pool.Query(dispatchContext, query)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var webhookID, name, targetURL, secretEncrypted string
		var eventsRaw []byte
		var maxRetries, timeoutSeconds int
		_ = rows.Scan(&webhookID, &name, &targetURL, &eventsRaw, &secretEncrypted, &maxRetries, &timeoutSeconds)

		var events []string
		_ = json.Unmarshal(eventsRaw, &events)

		matched := false
		for _, subscriptionPattern := range events {
			if matchWebhookEventPattern(subscriptionPattern, event.Event) {
				matched = true
				break
			}
		}

		if matched {
			go webhookEventBus.deliverWebhook(ctx, webhookID, targetURL, secretEncrypted, maxRetries, timeoutSeconds, event)
		}
	}
}

func (webhookEventBus *WebhookEventBus) deliverWebhook(ctx context.Context, webhookID, targetURL, secretEncrypted string, maxRetries, timeoutSeconds int, event WebhookEventEnvelope) {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}

	secret := ""
	if secretEncrypted != "" && webhookEventBus.cryptoKeyManager != nil {
		decryptedSecret, err := webhookEventBus.cryptoKeyManager.DecryptField(secretEncrypted)
		if err == nil {
			secret = string(decryptedSecret)
		}
	}

	bodyBytes, _ := json.Marshal(event)
	timestamp := fmt.Sprintf("%d", event.Timestamp.Unix())
	signature := ComputeWebhookSignature(secret, timestamp, bodyBytes)

	deliveryID := uuid.NewV7().String()
	attempt := 0
	var lastStatus *int
	var lastBody *string
	var lastErr *string
	var isDelivered bool
	var deliveredAt *time.Time

	start := time.Now()

	for attempt < maxRetries {
		attempt++
		attemptContext, attemptCancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
		request, err := http.NewRequestWithContext(attemptContext, http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
		if err != nil {
			attemptCancel()
			errMsg := err.Error()
			lastErr = &errMsg
			break
		}

		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("User-Agent", "Layr-Webhook-Dispatcher/1.0")
		request.Header.Set("X-Layr-Signature", signature)
		request.Header.Set("X-Layr-Timestamp", timestamp)
		request.Header.Set("X-Layr-Event", event.Event)
		request.Header.Set("X-Layr-Delivery-Id", deliveryID)

		response, err := webhookEventBus.httpClient.Do(request)
		attemptCancel()

		if err != nil {
			errMsg := err.Error()
			lastErr = &errMsg
			time.Sleep(CalculateWebhookBackoff(attempt))
			continue
		}

		status := response.StatusCode
		lastStatus = &status
		var responseBuffer bytes.Buffer
		_, _ = responseBuffer.ReadFrom(response.Body)
		_ = response.Body.Close()
		responseText := responseBuffer.String()
		if len(responseText) > maxLoggedResponseBodyBytes {
			responseText = responseText[:maxLoggedResponseBodyBytes]
		}
		lastBody = &responseText

		if status >= 200 && status < 300 {
			isDelivered = true
			now := time.Now().UTC()
			deliveredAt = &now
			lastErr = nil
			break
		}

		errMsg := fmt.Sprintf("HTTP error status %d", status)
		lastErr = &errMsg
		time.Sleep(CalculateWebhookBackoff(attempt))
	}

	durationMs := time.Since(start).Milliseconds()

	// Record delivery history in core.webhook_deliveries (survives parent cancellation)
	historyContext := context.WithoutCancel(ctx)
	if webhookEventBus.pool != nil {
		_, _ = webhookEventBus.pool.Exec(historyContext, `
			INSERT INTO core.webhook_deliveries (
				id, webhook_id, event_id, event_type, payload, response_status, response_body, error_message, attempt_count, duration_ms, is_delivered, delivered_at, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, deliveryID, webhookID, event.ID, event.Event, bodyBytes, lastStatus, lastBody, lastErr, attempt, durationMs, isDelivered, deliveredAt, time.Now().UTC())
	}

	if !isDelivered {
		// Broadcast delivery failure to internal bus listeners
		webhookEventBus.rwMutex.RLock()
		for pattern, handlers := range webhookEventBus.subscribers {
			if matchWebhookEventPattern(pattern, "core.webhook.delivery_failed") {
				failureEvent := WebhookEventEnvelope{
					ID:        uuid.NewV7().String(),
					Event:     "core.webhook.delivery_failed",
					Service:   "core",
					Resource:  "webhook",
					Action:    "delivery_failed",
					Timestamp: time.Now().UTC(),
					Data: map[string]any{
						"webhook_id":  webhookID,
						"delivery_id": deliveryID,
						"event_id":    event.ID,
						"event_type":  event.Event,
						"attempts":    attempt,
						"last_error":  lastErr,
						"duration_ms": durationMs,
					},
				}
				for _, subscriberHandler := range handlers {
					_ = subscriberHandler(historyContext, failureEvent)
				}
			}
		}
		webhookEventBus.rwMutex.RUnlock()
	}
}

// ComputeWebhookSignature generates the HMAC-SHA256 signature for Layr webhooks.
func ComputeWebhookSignature(secret, timestamp string, rawBody []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(rawBody)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// CalculateWebhookBackoff computes exponential backoff for a retry attempt.
func CalculateWebhookBackoff(attempt int) time.Duration {
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}

func matchWebhookEventPattern(pattern, eventName string) bool {
	pattern = strings.TrimSpace(pattern)
	eventName = strings.TrimSpace(eventName)
	if pattern == "*" || pattern == eventName {
		return true
	}
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(eventName, prefix+".") || eventName == prefix
	}
	return false
}

// WebhookManager handles CRUD and test execution for Webhook subscriptions.
type WebhookManager struct {
	pool             *DatabasePool
	cryptoKeyManager *CryptoKeyManager
	webhookEventBus  *WebhookEventBus
}

// NewWebhookManager initializes a new WebhookManager.
func NewWebhookManager(pool *DatabasePool, cryptoKeyManager *CryptoKeyManager, webhookEventBus *WebhookEventBus) *WebhookManager {
	return &WebhookManager{
		pool:             pool,
		cryptoKeyManager: cryptoKeyManager,
		webhookEventBus:  webhookEventBus,
	}
}

// Create registers a new webhook subscription.
func (webhookManager *WebhookManager) Create(ctx context.Context, input CreateWebhookInput) (*WebhookSubscription, error) {
	if webhookManager.pool == nil {
		return nil, fmt.Errorf("database pool is not available")
	}
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(input.TargetURL) == "" {
		return nil, fmt.Errorf("target_url is required")
	}
	if len(input.Events) == 0 {
		return nil, fmt.Errorf("at least one event must be specified")
	}

	secretEncrypted := ""
	if input.SigningSecret != "" && webhookManager.cryptoKeyManager != nil {
		encryptedSecret, _ := webhookManager.cryptoKeyManager.EncryptField([]byte(input.SigningSecret))
		secretEncrypted = encryptedSecret
	}

	maxRetries := 3
	if input.MaxRetries != nil {
		maxRetries = *input.MaxRetries
	}
	timeoutSeconds := 10
	if input.TimeoutSeconds != nil {
		timeoutSeconds = *input.TimeoutSeconds
	}

	eventsJSON, _ := json.Marshal(input.Events)
	webhookID := uuid.NewV7().String()
	now := time.Now().UTC()

	query := `
		INSERT INTO core.webhooks (
			id, name, target_url, events, is_enabled, signing_secret_enc, max_retries, timeout_seconds, created_at, last_updated_at
		) VALUES ($1, $2, $3, $4, true, $5, $6, $7, $8, $8)
		RETURNING id, name, target_url, events, is_enabled, max_retries, timeout_seconds, created_at, last_updated_at
	`

	var webhook WebhookSubscription
	var eventsRaw []byte

	err := webhookManager.pool.QueryRow(ctx, query,
		webhookID, input.Name, input.TargetURL, eventsJSON, secretEncrypted, maxRetries, timeoutSeconds, now,
	).Scan(
		&webhook.ID, &webhook.Name, &webhook.TargetURL, &eventsRaw, &webhook.IsEnabled, &webhook.MaxRetries, &webhook.TimeoutSeconds, &webhook.CreatedAt, &webhook.LastUpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	_ = json.Unmarshal(eventsRaw, &webhook.Events)
	webhook.SigningSecretConfigured = (secretEncrypted != "")
	webhook.SigningSecretMasked = webhook.SigningSecretConfigured

	return &webhook, nil
}

// List returns all webhook subscriptions.
func (webhookManager *WebhookManager) List(ctx context.Context) ([]WebhookSubscription, error) {
	if webhookManager.pool == nil {
		return nil, fmt.Errorf("database pool is not available")
	}
	query := `
		SELECT id, name, target_url, events, is_enabled, signing_secret_enc, max_retries, timeout_seconds, created_at, last_updated_at
		FROM core.webhooks
		ORDER BY created_at DESC
	`
	rows, err := webhookManager.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}
	defer rows.Close()

	results := make([]WebhookSubscription, 0)
	for rows.Next() {
		var webhook WebhookSubscription
		var eventsRaw []byte
		var secretEncrypted string
		_ = rows.Scan(
			&webhook.ID, &webhook.Name, &webhook.TargetURL, &eventsRaw, &webhook.IsEnabled, &secretEncrypted, &webhook.MaxRetries, &webhook.TimeoutSeconds, &webhook.CreatedAt, &webhook.LastUpdatedAt,
		)
		_ = json.Unmarshal(eventsRaw, &webhook.Events)
		webhook.SigningSecretConfigured = (secretEncrypted != "")
		webhook.SigningSecretMasked = webhook.SigningSecretConfigured
		results = append(results, webhook)
	}
	return results, nil
}

// Get returns a webhook by ID.
func (webhookManager *WebhookManager) Get(ctx context.Context, webhookID string) (*WebhookSubscription, error) {
	if webhookManager.pool == nil {
		return nil, fmt.Errorf("database pool is not available")
	}
	query := `
		SELECT id, name, target_url, events, is_enabled, signing_secret_enc, max_retries, timeout_seconds, created_at, last_updated_at
		FROM core.webhooks
		WHERE id = $1
	`
	var webhook WebhookSubscription
	var eventsRaw []byte
	var secretEncrypted string

	err := webhookManager.pool.QueryRow(ctx, query, webhookID).Scan(
		&webhook.ID, &webhook.Name, &webhook.TargetURL, &eventsRaw, &webhook.IsEnabled, &secretEncrypted, &webhook.MaxRetries, &webhook.TimeoutSeconds, &webhook.CreatedAt, &webhook.LastUpdatedAt,
	)
	if err != nil {
		return nil, ErrWebhookNotFound
	}
	_ = json.Unmarshal(eventsRaw, &webhook.Events)
	webhook.SigningSecretConfigured = (secretEncrypted != "")
	webhook.SigningSecretMasked = webhook.SigningSecretConfigured
	webhook.SigningSecretEncrypted = secretEncrypted
	return &webhook, nil
}

// Update modifies an existing webhook.
func (webhookManager *WebhookManager) Update(ctx context.Context, webhookID string, input UpdateWebhookInput) (*WebhookSubscription, error) {
	current, err := webhookManager.Get(ctx, webhookID)
	if err != nil {
		return nil, err
	}

	name := current.Name
	if input.Name != nil && strings.TrimSpace(*input.Name) != "" {
		name = *input.Name
	}
	targetURL := current.TargetURL
	if input.TargetURL != nil && strings.TrimSpace(*input.TargetURL) != "" {
		targetURL = *input.TargetURL
	}
	events := current.Events
	if len(input.Events) > 0 {
		events = input.Events
	}
	isEnabled := current.IsEnabled
	if input.IsEnabled != nil {
		isEnabled = *input.IsEnabled
	}
	maxRetries := current.MaxRetries
	if input.MaxRetries != nil {
		maxRetries = *input.MaxRetries
	}
	timeoutSeconds := current.TimeoutSeconds
	if input.TimeoutSeconds != nil {
		timeoutSeconds = *input.TimeoutSeconds
	}

	secretEncrypted := current.SigningSecretEncrypted
	if input.SigningSecret != nil && webhookManager.cryptoKeyManager != nil {
		encryptedSecret, _ := webhookManager.cryptoKeyManager.EncryptField([]byte(*input.SigningSecret))
		secretEncrypted = encryptedSecret
	}

	eventsJSON, _ := json.Marshal(events)
	now := time.Now().UTC()

	query := `
		UPDATE core.webhooks
		SET name = $1, target_url = $2, events = $3, is_enabled = $4, signing_secret_enc = $5, max_retries = $6, timeout_seconds = $7, last_updated_at = $8
		WHERE id = $9
		RETURNING id, name, target_url, events, is_enabled, max_retries, timeout_seconds, created_at, last_updated_at
	`

	var webhook WebhookSubscription
	var eventsRaw []byte

	_ = webhookManager.pool.QueryRow(ctx, query,
		name, targetURL, eventsJSON, isEnabled, secretEncrypted, maxRetries, timeoutSeconds, now, webhookID,
	).Scan(
		&webhook.ID, &webhook.Name, &webhook.TargetURL, &eventsRaw, &webhook.IsEnabled, &webhook.MaxRetries, &webhook.TimeoutSeconds, &webhook.CreatedAt, &webhook.LastUpdatedAt,
	)

	_ = json.Unmarshal(eventsRaw, &webhook.Events)
	webhook.SigningSecretConfigured = (secretEncrypted != "")
	webhook.SigningSecretMasked = webhook.SigningSecretConfigured
	return &webhook, nil
}

// Delete removes a webhook subscription.
func (webhookManager *WebhookManager) Delete(ctx context.Context, webhookID string) error {
	if webhookManager.pool == nil {
		return fmt.Errorf("database pool is not available")
	}
	result, err := webhookManager.pool.Exec(ctx, "DELETE FROM core.webhooks WHERE id = $1", webhookID)
	if err != nil || result.RowsAffected() == 0 {
		return ErrWebhookNotFound
	}
	return nil
}

// ListDeliveries returns delivery history for a webhook.
func (webhookManager *WebhookManager) ListDeliveries(ctx context.Context, webhookID string) ([]WebhookDelivery, error) {
	if webhookManager.pool == nil {
		return nil, fmt.Errorf("database pool is not available")
	}
	query := `
		SELECT id, webhook_id, event_id, event_type, payload, response_status, response_body, error_message, attempt_count, duration_ms, is_delivered, delivered_at, created_at
		FROM core.webhook_deliveries
		WHERE webhook_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`
	rows, err := webhookManager.pool.Query(ctx, query, webhookID)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhook deliveries: %w", err)
	}
	defer rows.Close()

	results := make([]WebhookDelivery, 0)
	for rows.Next() {
		var delivery WebhookDelivery
		var payloadRaw []byte
		_ = rows.Scan(
			&delivery.ID, &delivery.WebhookID, &delivery.EventID, &delivery.EventType, &payloadRaw, &delivery.ResponseStatus, &delivery.ResponseBody, &delivery.ErrorMessage, &delivery.AttemptCount, &delivery.DurationMs, &delivery.IsDelivered, &delivery.DeliveredAt, &delivery.CreatedAt,
		)
		_ = json.Unmarshal(payloadRaw, &delivery.Payload)
		results = append(results, delivery)
	}
	return results, nil
}
