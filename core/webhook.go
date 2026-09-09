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
	"strings"
	"sync"
	"time"
	"uuid"
)

// Standard Webhook Errors
var (
	ErrWebhookNotFound = errors.New("webhook not found")
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

// Webhook represents a webhook configuration stored in core.webhooks.
type Webhook struct {
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

// CreateWebhookInput holds parameters to create a new webhook.
type CreateWebhookInput struct {
	Name           string   `json:"name"`
	TargetURL      string   `json:"target_url"`
	Events         []string `json:"events"`
	SigningSecret  string   `json:"signing_secret"`
	MaxRetries     *int     `json:"max_retries,omitempty"`
	TimeoutSeconds *int     `json:"timeout_seconds,omitempty"`
}

// UpdateWebhookInput holds parameters to update an existing webhook.
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
type WebhookEventHandler func(ctx context.Context, webhookEventEnvelope WebhookEventEnvelope) error

// WebhookEventBus coordinates event publishing and dispatching to webhooks and in-memory listeners.
type WebhookEventBus struct {
	rwMutex          sync.RWMutex
	subscribers      map[string][]WebhookEventHandler
	db               *DatabasePool
	cryptoKeyManager *CryptoKeyManager
	httpClient       *http.Client
	dispatchChannel  chan WebhookEventEnvelope
	stopChannel      chan struct{}
	waitGroup        sync.WaitGroup
	ctx              context.Context
	ctxCancel        context.CancelFunc
}

// NewWebhookEventBus initializes the WebhookEventBus and starts background worker goroutines.
func NewWebhookEventBus(db *DatabasePool, cryptoKeyManager *CryptoKeyManager) *WebhookEventBus {
	log.Debugf("initializing WebhookEventBus")
	ctx, cancel := context.WithCancel(context.Background())
	webhookEventBus := &WebhookEventBus{
		subscribers:      make(map[string][]WebhookEventHandler),
		db:               db,
		cryptoKeyManager: cryptoKeyManager,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		dispatchChannel: make(chan WebhookEventEnvelope, defaultDispatchChannelCapacity),
		stopChannel:     make(chan struct{}),
		ctx:             ctx,
		ctxCancel:       cancel,
	}

	// Start 4 worker dispatchers
	for i := 0; i < 4; i++ {
		webhookEventBus.waitGroup.Add(1)
		go webhookEventBus.worker()
	}

	return webhookEventBus
}

// Subscribe registers an in-memory handler for an event pattern.
func (webhookEventBus *WebhookEventBus) Subscribe(pattern string, webhookEventHandler WebhookEventHandler) {
	webhookEventBus.rwMutex.Lock()
	defer webhookEventBus.rwMutex.Unlock()
	webhookEventBus.subscribers[pattern] = append(webhookEventBus.subscribers[pattern], webhookEventHandler)
}

// Publish broadcasts an event to in-memory listeners and enqueues it for webhook dispatch.
func (webhookEventBus *WebhookEventBus) Publish(ctx context.Context, webhookEventEnvelope WebhookEventEnvelope) {
	if webhookEventEnvelope.ID == "" {
		webhookEventEnvelope.ID = uuid.NewV7().String()
	}
	if webhookEventEnvelope.Timestamp.IsZero() {
		webhookEventEnvelope.Timestamp = time.Now().UTC()
	}
	log.Tracef("publishing webhook event %s (%s)", webhookEventEnvelope.ID, webhookEventEnvelope.Event)

	// Dispatch to in-memory subscribers synchronously or concurrently
	webhookEventBus.rwMutex.RLock()
	var matchedHandlers []WebhookEventHandler
	for pattern, handlers := range webhookEventBus.subscribers {
		if matchWebhookEventPattern(pattern, webhookEventEnvelope.Event) {
			matchedHandlers = append(matchedHandlers, handlers...)
		}
	}
	webhookEventBus.rwMutex.RUnlock()

	for _, subscriberHandler := range matchedHandlers {
		webhookEventBus.waitGroup.Add(1)
		go func(webhookEventHandler WebhookEventHandler) {
			defer webhookEventBus.waitGroup.Done()
			_ = webhookEventHandler(ctx, webhookEventEnvelope)
		}(subscriberHandler)
	}

	// Enqueue for webhook processing
	select {
	case webhookEventBus.dispatchChannel <- webhookEventEnvelope:
	default:
		log.Errorf("dispatch buffer full, dropping event %s (%s)", webhookEventEnvelope.ID, webhookEventEnvelope.Event)
	}
}

// Close stops all background dispatch workers cleanly.
func (webhookEventBus *WebhookEventBus) Close() {
	log.Debugf("closing WebhookEventBus")
	close(webhookEventBus.stopChannel)
	webhookEventBus.ctxCancel()
	webhookEventBus.waitGroup.Wait()
	log.Tracef("WebhookEventBus closed cleanly")
}

func (webhookEventBus *WebhookEventBus) worker() {
	defer webhookEventBus.waitGroup.Done()
	for {
		select {
		case <-webhookEventBus.stopChannel:
			return
		case webhookEventEnvelope := <-webhookEventBus.dispatchChannel:
			webhookEventBus.dispatch(webhookEventBus.ctx, webhookEventEnvelope)
		}
	}
}

func (webhookEventBus *WebhookEventBus) dispatch(ctx context.Context, webhookEventEnvelope WebhookEventEnvelope) {
	log.Tracef("dispatching webhook event %s (%s)", webhookEventEnvelope.ID, webhookEventEnvelope.Event)
	if webhookEventBus.db == nil {
		return
	}

	dispatchCtx, dispatchCancel := context.WithTimeout(ctx, defaultDispatchTimeout)
	defer dispatchCancel()

	// Query active webhooks matching this event
	query := `
		SELECT id, name, target_url, events, signing_secret_enc, max_retries, timeout_seconds
		FROM core.webhooks
		WHERE is_enabled = true
	`
	rows, err := webhookEventBus.db.Query(dispatchCtx, query)
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
			if matchWebhookEventPattern(subscriptionPattern, webhookEventEnvelope.Event) {
				matched = true
				break
			}
		}

		if matched {
			webhookEventBus.waitGroup.Add(1)
			go func(deliveryWebhookID, deliveryTargetURL, deliverySecretEncrypted string, deliveryMaxRetries, deliveryTimeoutSeconds int) {
				defer webhookEventBus.waitGroup.Done()
				webhookEventBus.deliver(ctx, deliveryWebhookID, deliveryTargetURL, deliverySecretEncrypted, deliveryMaxRetries, deliveryTimeoutSeconds, webhookEventEnvelope)
			}(webhookID, targetURL, secretEncrypted, maxRetries, timeoutSeconds)
		}
	}
}

func (webhookEventBus *WebhookEventBus) deliver(ctx context.Context, webhookID, targetURL, secretEncrypted string, maxRetries, timeoutSeconds int, webhookEventEnvelope WebhookEventEnvelope) {
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

	bodyBytes, _ := json.Marshal(webhookEventEnvelope)
	timestamp := fmt.Sprintf("%d", webhookEventEnvelope.Timestamp.Unix())
	signature := ComputeWebhookSignature(secret, timestamp, bodyBytes)

	deliveryID := uuid.NewV7().String()
	log.Debugf("dispatching webhook delivery %s for webhook %s (event: %s) to %s", deliveryID, webhookID, webhookEventEnvelope.Event, targetURL)
	attempt := 0
	var lastStatus *int
	var lastBody *string
	var lastErr *string
	var isDelivered bool
	var deliveredAt *time.Time

	start := time.Now()

	for attempt < maxRetries {
		attempt++
		log.Tracef("webhook %s delivery attempt %d/%d to %s", webhookID, attempt, maxRetries, targetURL)
		attemptCtx, attemptCancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
		request, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
		if err != nil {
			attemptCancel()
			log.Debugf("failed to create webhook HTTP request: %v", err)
			errMsg := err.Error()
			lastErr = &errMsg
			break
		}

		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("User-Agent", "Layr-Webhook-Dispatcher/1.0")
		request.Header.Set("X-Layr-Signature", signature)
		request.Header.Set("X-Layr-Timestamp", timestamp)
		request.Header.Set("X-Layr-Event", webhookEventEnvelope.Event)
		request.Header.Set("X-Layr-Delivery-Id", deliveryID)

		response, err := webhookEventBus.httpClient.Do(request)
		attemptCancel()

		if err != nil {
			log.Tracef("webhook %s delivery attempt %d network error: %v", webhookID, attempt, err)
			errMsg := err.Error()
			lastErr = &errMsg
			if !sleepWithContext(ctx, CalculateWebhookBackoff(attempt)) {
				break
			}
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
			log.Debugf("webhook %s delivery %s succeeded with status %d in %dms (%d attempts)", webhookID, deliveryID, status, time.Since(start).Milliseconds(), attempt)
			break
		}

		log.Tracef("webhook %s delivery attempt %d returned HTTP status %d", webhookID, attempt, status)
		errMsg := fmt.Sprintf("HTTP error status %d", status)
		lastErr = &errMsg
		if !sleepWithContext(ctx, CalculateWebhookBackoff(attempt)) {
			break
		}
	}

	durationMs := time.Since(start).Milliseconds()

	// Record delivery history in core.webhook_deliveries (survives parent cancellation)
	historyCtx := context.WithoutCancel(ctx)
	if webhookEventBus.db != nil {
		_, _ = webhookEventBus.db.Exec(historyCtx, `
			INSERT INTO core.webhook_deliveries (
				id, webhook_id, event_id, event_type, payload, response_status, response_body, error_message, attempt_count, duration_ms, is_delivered, delivered_at, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, deliveryID, webhookID, webhookEventEnvelope.ID, webhookEventEnvelope.Event, bodyBytes, lastStatus, lastBody, lastErr, attempt, durationMs, isDelivered, deliveredAt, time.Now().UTC())
	}

	if !isDelivered {
		log.Warnf("webhook %s delivery %s failed after %d attempts", webhookID, deliveryID, attempt)
		// Broadcast delivery failure to internal bus listeners
		webhookEventBus.rwMutex.RLock()
		for pattern, handlers := range webhookEventBus.subscribers {
			if matchWebhookEventPattern(pattern, "core.webhook.delivery_failed") {
				failureWebhookEventEnvelope := WebhookEventEnvelope{
					ID:        uuid.NewV7().String(),
					Event:     "core.webhook.delivery_failed",
					Service:   "core",
					Resource:  "webhook",
					Action:    "delivery_failed",
					Timestamp: time.Now().UTC(),
					Data: map[string]any{
						"webhook_id":  webhookID,
						"delivery_id": deliveryID,
						"event_id":    webhookEventEnvelope.ID,
						"event_type":  webhookEventEnvelope.Event,
						"attempts":    attempt,
						"last_error":  lastErr,
						"duration_ms": durationMs,
					},
				}
				for _, subscriberHandler := range handlers {
					_ = subscriberHandler(historyCtx, failureWebhookEventEnvelope)
				}
			}
		}
		webhookEventBus.rwMutex.RUnlock()
	}
}

// ComputeWebhookSignature generates the HMAC-SHA256 signature for Layr webhooks.
func ComputeWebhookSignature(secret, timestamp string, rawBody []byte) string {
	//nolint:namingclarity
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

// WebhookManager handles CRUD and test execution for Webhooks.
type WebhookManager struct {
	db               *DatabasePool
	cryptoKeyManager *CryptoKeyManager
	webhookEventBus  *WebhookEventBus
}

// NewWebhookManager initializes a new WebhookManager.
func NewWebhookManager(db *DatabasePool, cryptoKeyManager *CryptoKeyManager, webhookEventBus *WebhookEventBus) *WebhookManager {
	return &WebhookManager{
		db:               db,
		cryptoKeyManager: cryptoKeyManager,
		webhookEventBus:  webhookEventBus,
	}
}

// Create registers a new webhook.
func (webhookManager *WebhookManager) Create(ctx context.Context, createWebhookInput CreateWebhookInput) (*Webhook, error) {
	log.Debugf("creating webhook %q for %s", createWebhookInput.Name, createWebhookInput.TargetURL)
	if webhookManager.db == nil {
		return nil, fmt.Errorf("database pool is not available")
	}
	if strings.TrimSpace(createWebhookInput.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(createWebhookInput.TargetURL) == "" {
		return nil, fmt.Errorf("target_url is required")
	}
	if err := validateWebhookTargetURL(createWebhookInput.TargetURL); err != nil {
		return nil, err
	}
	if len(createWebhookInput.Events) == 0 {
		return nil, fmt.Errorf("at least one event must be specified")
	}

	secretEncrypted := ""
	if createWebhookInput.SigningSecret != "" && webhookManager.cryptoKeyManager != nil {
		encryptedSecret, _ := webhookManager.cryptoKeyManager.EncryptField([]byte(createWebhookInput.SigningSecret))
		secretEncrypted = encryptedSecret
	}

	maxRetries := 3
	if createWebhookInput.MaxRetries != nil {
		maxRetries = *createWebhookInput.MaxRetries
	}
	timeoutSeconds := 10
	if createWebhookInput.TimeoutSeconds != nil {
		timeoutSeconds = *createWebhookInput.TimeoutSeconds
	}

	eventsJSON, _ := json.Marshal(createWebhookInput.Events)
	webhookID := uuid.NewV7().String()
	now := time.Now().UTC()

	query := `
		INSERT INTO core.webhooks (
			id, name, target_url, events, is_enabled, signing_secret_enc, max_retries, timeout_seconds, created_at, last_updated_at
		) VALUES ($1, $2, $3, $4, true, $5, $6, $7, $8, $8)
		RETURNING id, name, target_url, events, is_enabled, max_retries, timeout_seconds, created_at, last_updated_at
	`

	var webhook Webhook
	var eventsRaw []byte

	err := webhookManager.db.QueryRow(ctx, query,
		webhookID, createWebhookInput.Name, createWebhookInput.TargetURL, eventsJSON, secretEncrypted, maxRetries, timeoutSeconds, now,
	).Scan(
		&webhook.ID, &webhook.Name, &webhook.TargetURL, &eventsRaw, &webhook.IsEnabled, &webhook.MaxRetries, &webhook.TimeoutSeconds, &webhook.CreatedAt, &webhook.LastUpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	_ = json.Unmarshal(eventsRaw, &webhook.Events)
	webhook.SigningSecretConfigured = (secretEncrypted != "")
	webhook.SigningSecretMasked = webhook.SigningSecretConfigured

	log.Tracef("created webhook %s successfully", webhookID)
	return &webhook, nil
}

// List returns all webhooks.
func (webhookManager *WebhookManager) List(ctx context.Context) ([]Webhook, error) {
	log.Trace("listing webhooks from database")
	if webhookManager.db == nil {
		return nil, fmt.Errorf("database pool is not available")
	}
	query := `
		SELECT id, name, target_url, events, is_enabled, signing_secret_enc, max_retries, timeout_seconds, created_at, last_updated_at
		FROM core.webhooks
		ORDER BY created_at DESC
	`
	rows, err := webhookManager.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}
	defer rows.Close()

	results := make([]Webhook, 0)
	for rows.Next() {
		var webhook Webhook
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
	log.Tracef("listed %d webhooks", len(results))
	return results, nil
}

// Get returns a webhook by ID.
func (webhookManager *WebhookManager) Get(ctx context.Context, webhookID string) (*Webhook, error) {
	log.Tracef("retrieving webhook %s", webhookID)
	if webhookManager.db == nil {
		return nil, fmt.Errorf("database pool is not available")
	}
	query := `
		SELECT id, name, target_url, events, is_enabled, signing_secret_enc, max_retries, timeout_seconds, created_at, last_updated_at
		FROM core.webhooks
		WHERE id = $1
	`
	var webhook Webhook
	var eventsRaw []byte
	var secretEncrypted string

	err := webhookManager.db.QueryRow(ctx, query, webhookID).Scan(
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
func (webhookManager *WebhookManager) Update(ctx context.Context, webhookID string, updateWebhookInput UpdateWebhookInput) (*Webhook, error) {
	log.Debugf("updating webhook %s", webhookID)
	currentWebhook, err := webhookManager.Get(ctx, webhookID)
	if err != nil {
		return nil, err
	}

	name := currentWebhook.Name
	if updateWebhookInput.Name != nil && strings.TrimSpace(*updateWebhookInput.Name) != "" {
		name = *updateWebhookInput.Name
	}
	targetURL := currentWebhook.TargetURL
	if updateWebhookInput.TargetURL != nil {
		if strings.TrimSpace(*updateWebhookInput.TargetURL) == "" {
			return nil, fmt.Errorf("target_url cannot be empty")
		}
		if err := validateWebhookTargetURL(*updateWebhookInput.TargetURL); err != nil {
			return nil, err
		}
		targetURL = *updateWebhookInput.TargetURL
	}
	events := currentWebhook.Events
	if len(updateWebhookInput.Events) > 0 {
		events = updateWebhookInput.Events
	}
	isEnabled := currentWebhook.IsEnabled
	if updateWebhookInput.IsEnabled != nil {
		isEnabled = *updateWebhookInput.IsEnabled
	}
	maxRetries := currentWebhook.MaxRetries
	if updateWebhookInput.MaxRetries != nil {
		maxRetries = *updateWebhookInput.MaxRetries
	}
	timeoutSeconds := currentWebhook.TimeoutSeconds
	if updateWebhookInput.TimeoutSeconds != nil {
		timeoutSeconds = *updateWebhookInput.TimeoutSeconds
	}

	secretEncrypted := currentWebhook.SigningSecretEncrypted
	if updateWebhookInput.SigningSecret != nil && webhookManager.cryptoKeyManager != nil {
		encryptedSecret, _ := webhookManager.cryptoKeyManager.EncryptField([]byte(*updateWebhookInput.SigningSecret))
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

	var webhook Webhook
	var eventsRaw []byte

	_ = webhookManager.db.QueryRow(ctx, query,
		name, targetURL, eventsJSON, isEnabled, secretEncrypted, maxRetries, timeoutSeconds, now, webhookID,
	).Scan(
		&webhook.ID, &webhook.Name, &webhook.TargetURL, &eventsRaw, &webhook.IsEnabled, &webhook.MaxRetries, &webhook.TimeoutSeconds, &webhook.CreatedAt, &webhook.LastUpdatedAt,
	)

	_ = json.Unmarshal(eventsRaw, &webhook.Events)
	webhook.SigningSecretConfigured = (secretEncrypted != "")
	webhook.SigningSecretMasked = webhook.SigningSecretConfigured
	log.Tracef("updated webhook %s successfully", webhookID)
	return &webhook, nil
}

// Delete removes a webhook.
func (webhookManager *WebhookManager) Delete(ctx context.Context, webhookID string) error {
	log.Debugf("deleting webhook %s", webhookID)
	if webhookManager.db == nil {
		return fmt.Errorf("database pool is not available")
	}
	result, err := webhookManager.db.Exec(ctx, "DELETE FROM core.webhooks WHERE id = $1", webhookID)
	if err != nil || result.RowsAffected() == 0 {
		return ErrWebhookNotFound
	}
	log.Tracef("deleted webhook %s successfully", webhookID)
	return nil
}

// ListDeliveries returns delivery history for a webhook.
func (webhookManager *WebhookManager) ListDeliveries(ctx context.Context, webhookID string) ([]WebhookDelivery, error) {
	log.Tracef("listing deliveries for webhook %s", webhookID)
	if webhookManager.db == nil {
		return nil, fmt.Errorf("database pool is not available")
	}
	query := `
		SELECT id, webhook_id, event_id, event_type, payload, response_status, response_body, error_message, attempt_count, duration_ms, is_delivered, delivered_at, created_at
		FROM core.webhook_deliveries
		WHERE webhook_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`
	rows, err := webhookManager.db.Query(ctx, query, webhookID)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhook deliveries: %w", err)
	}
	defer rows.Close()

	results := make([]WebhookDelivery, 0)
	for rows.Next() {
		var webhookDelivery WebhookDelivery
		var payloadRaw []byte
		_ = rows.Scan(
			&webhookDelivery.ID, &webhookDelivery.WebhookID, &webhookDelivery.EventID, &webhookDelivery.EventType, &payloadRaw, &webhookDelivery.ResponseStatus, &webhookDelivery.ResponseBody, &webhookDelivery.ErrorMessage, &webhookDelivery.AttemptCount, &webhookDelivery.DurationMs, &webhookDelivery.IsDelivered, &webhookDelivery.DeliveredAt, &webhookDelivery.CreatedAt,
		)
		_ = json.Unmarshal(payloadRaw, &webhookDelivery.Payload)
		results = append(results, webhookDelivery)
	}
	log.Tracef("listed %d webhook deliveries for %s", len(results), webhookID)
	return results, nil
}

func validateWebhookTargetURL(targetURL string) error {
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
