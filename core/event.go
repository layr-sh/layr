package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"uuid"
)

// Standard Event errors.
var (
	ErrEventNotFound = errors.New("event not found")
)

const (
	defaultEventDispatchChannelCapacity = 1000
	defaultEventDispatchTimeout         = 30 * time.Second
	defaultEventWorkerCount             = 4
	defaultEventListLimit               = 50
	maxEventListLimit                   = 200
)

// EventActor represents the principal that triggered the event.
type EventActor struct {
	Type string     `json:"type"` // "user", "service_account", "console_user", "system"
	ID   *uuid.UUID `json:"id,omitempty"`
	Role *string    `json:"role,omitempty"`
}

// EventContext captures transport metadata associated with the event origin.
type EventContext struct {
	IPAddress *string `json:"ip_address,omitempty"`
	UserAgent *string `json:"user_agent,omitempty"`
	RequestID *string `json:"request_id,omitempty"`
}

// Event represents a canonical system or domain event stored in core.events.
type Event struct {
	ID           uuid.UUID              `json:"id"`
	Type         string                 `json:"type"`
	Actor        EventActor             `json:"actor"`
	Context      EventContext           `json:"context"`
	Action       string                 `json:"action"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   *string                `json:"resource_id,omitempty"`
	Status       string                 `json:"status"`
	Reason       *string                `json:"reason,omitempty"`
	Metadata     map[string]interface{} `json:"metadata"`
	Payload      map[string]interface{} `json:"payload"`
	CreatedAt    time.Time              `json:"created_at"`
}

// EventFilter provides multi-field filtering and pagination options for querying events.
type EventFilter struct {
	Type         *string    `json:"type,omitempty"`
	ActorType    *string    `json:"actor_type,omitempty"`
	ActorID      *uuid.UUID `json:"actor_id,omitempty"`
	ResourceType *string    `json:"resource_type,omitempty"`
	ResourceID   *string    `json:"resource_id,omitempty"`
	Status       *string    `json:"status,omitempty"`
	StartDate    *time.Time `json:"start_date,omitempty"`
	EndDate      *time.Time `json:"end_date,omitempty"`
	Limit        int        `json:"limit,omitempty"`
	Offset       int        `json:"offset,omitempty"`
}

// EventHandler handles in-process event subscriptions.
type EventHandler func(ctx context.Context, event Event) error

// EventManager handles persistence and querying for core.events.
type EventManager struct {
	db *DatabasePool
}

// NewEventManager creates a new EventManager backed by the database pool.
func NewEventManager(db *DatabasePool) *EventManager {
	return &EventManager{db: db}
}

// Record inserts a new event record into core.events.
func (eventManager *EventManager) Record(ctx context.Context, event Event) (Event, error) {
	if eventManager.db == nil {
		return Event{}, fmt.Errorf("database pool is not available")
	}

	preparedEvent := prepareEvent(event)

	metadataJSON, err := json.Marshal(preparedEvent.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	payloadJSON, err := json.Marshal(preparedEvent.Payload)
	if err != nil {
		payloadJSON = []byte("{}")
	}

	query := `
		INSERT INTO core.events (
			id, type, actor_id, actor_type, ip_address, user_agent, action, resource_type, resource_id, status, reason, metadata, payload, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
		RETURNING id, type, actor_id, actor_type, ip_address::text, user_agent, action, resource_type, resource_id, status, reason, metadata, payload, created_at
	`

	var recordedEvent Event
	var actorID *uuid.UUID
	var ipAddress *string
	var userAgent *string
	var resourceID *string
	var reason *string
	var metadataRaw []byte
	var payloadRaw []byte

	err = eventManager.db.QueryRow(ctx, query,
		preparedEvent.ID,
		preparedEvent.Type,
		preparedEvent.Actor.ID,
		preparedEvent.Actor.Type,
		preparedEvent.Context.IPAddress,
		preparedEvent.Context.UserAgent,
		preparedEvent.Action,
		preparedEvent.ResourceType,
		preparedEvent.ResourceID,
		preparedEvent.Status,
		preparedEvent.Reason,
		metadataJSON,
		payloadJSON,
		preparedEvent.CreatedAt,
	).Scan(
		&recordedEvent.ID,
		&recordedEvent.Type,
		&actorID,
		&recordedEvent.Actor.Type,
		&ipAddress,
		&userAgent,
		&recordedEvent.Action,
		&recordedEvent.ResourceType,
		&resourceID,
		&recordedEvent.Status,
		&reason,
		&metadataRaw,
		&payloadRaw,
		&recordedEvent.CreatedAt,
	)
	if err != nil {
		return Event{}, fmt.Errorf("failed to record event: %w", err)
	}

	recordedEvent.Actor.ID = actorID
	recordedEvent.Context.IPAddress = ipAddress
	recordedEvent.Context.UserAgent = userAgent
	recordedEvent.ResourceID = resourceID
	recordedEvent.Reason = reason
	_ = json.Unmarshal(metadataRaw, &recordedEvent.Metadata)
	_ = json.Unmarshal(payloadRaw, &recordedEvent.Payload)

	log.Tracef("recorded event %s (%s)", recordedEvent.ID, recordedEvent.Type)
	return recordedEvent, nil
}

// Get retrieves an event by its UUID from core.events.
func (eventManager *EventManager) Get(ctx context.Context, eventUUID uuid.UUID) (Event, error) {
	if eventManager.db == nil {
		return Event{}, fmt.Errorf("database pool is not available")
	}

	query := `
		SELECT id, type, actor_id, actor_type, ip_address::text, user_agent, action, resource_type, resource_id, status, reason, metadata, payload, created_at
		FROM core.events
		WHERE id = $1
	`

	var event Event
	var actorID *uuid.UUID
	var ipAddress *string
	var userAgent *string
	var resourceID *string
	var reason *string
	var metadataRaw []byte
	var payloadRaw []byte

	err := eventManager.db.QueryRow(ctx, query, eventUUID).Scan(
		&event.ID,
		&event.Type,
		&actorID,
		&event.Actor.Type,
		&ipAddress,
		&userAgent,
		&event.Action,
		&event.ResourceType,
		&resourceID,
		&event.Status,
		&reason,
		&metadataRaw,
		&payloadRaw,
		&event.CreatedAt,
	)
	if err != nil {
		return Event{}, ErrEventNotFound
	}

	event.Actor.ID = actorID
	event.Context.IPAddress = ipAddress
	event.Context.UserAgent = userAgent
	event.ResourceID = resourceID
	event.Reason = reason
	_ = json.Unmarshal(metadataRaw, &event.Metadata)
	_ = json.Unmarshal(payloadRaw, &event.Payload)

	return event, nil
}

// List returns a list of events matching the provided filter criteria.
func (eventManager *EventManager) List(ctx context.Context, eventFilter EventFilter) ([]Event, error) {
	if eventManager.db == nil {
		return nil, fmt.Errorf("database pool is not available")
	}

	limit := eventFilter.Limit
	if limit <= 0 {
		limit = defaultEventListLimit
	}
	if limit > maxEventListLimit {
		limit = maxEventListLimit
	}

	offset := eventFilter.Offset
	if offset < 0 {
		offset = 0
	}

	var conditions []string
	var args []any
	argIndex := 1

	if eventFilter.Type != nil && *eventFilter.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIndex))
		args = append(args, *eventFilter.Type)
		argIndex++
	}
	if eventFilter.ActorType != nil && *eventFilter.ActorType != "" {
		conditions = append(conditions, fmt.Sprintf("actor_type = $%d", argIndex))
		args = append(args, *eventFilter.ActorType)
		argIndex++
	}
	if eventFilter.ActorID != nil {
		conditions = append(conditions, fmt.Sprintf("actor_id = $%d", argIndex))
		args = append(args, *eventFilter.ActorID)
		argIndex++
	}
	if eventFilter.ResourceType != nil && *eventFilter.ResourceType != "" {
		conditions = append(conditions, fmt.Sprintf("resource_type = $%d", argIndex))
		args = append(args, *eventFilter.ResourceType)
		argIndex++
	}
	if eventFilter.ResourceID != nil && *eventFilter.ResourceID != "" {
		conditions = append(conditions, fmt.Sprintf("resource_id = $%d", argIndex))
		args = append(args, *eventFilter.ResourceID)
		argIndex++
	}
	if eventFilter.Status != nil && *eventFilter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *eventFilter.Status)
		argIndex++
	}
	if eventFilter.StartDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *eventFilter.StartDate)
		argIndex++
	}
	if eventFilter.EndDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *eventFilter.EndDate)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, type, actor_id, actor_type, ip_address::text, user_agent, action, resource_type, resource_id, status, reason, metadata, payload, created_at
		FROM core.events
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := eventManager.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	results := make([]Event, 0)
	for rows.Next() {
		var event Event
		var actorID *uuid.UUID
		var ipAddress *string
		var userAgent *string
		var resourceID *string
		var reason *string
		var metadataRaw []byte
		var payloadRaw []byte

		_ = rows.Scan(
			&event.ID,
			&event.Type,
			&actorID,
			&event.Actor.Type,
			&ipAddress,
			&userAgent,
			&event.Action,
			&event.ResourceType,
			&resourceID,
			&event.Status,
			&reason,
			&metadataRaw,
			&payloadRaw,
			&event.CreatedAt,
		)

		event.Actor.ID = actorID
		event.Context.IPAddress = ipAddress
		event.Context.UserAgent = userAgent
		event.ResourceID = resourceID
		event.Reason = reason
		_ = json.Unmarshal(metadataRaw, &event.Metadata)
		_ = json.Unmarshal(payloadRaw, &event.Payload)

		results = append(results, event)
	}

	return results, nil
}

// EventBus coordinates event publishing and dispatching to in-memory listeners, database events, and event hooks.
type EventBus struct {
	rwMutex          sync.RWMutex
	subscribers      map[string][]EventHandler
	db               *DatabasePool
	cryptoKeyManager *CryptoKeyManager
	eventManager     *EventManager
	eventHookManager *EventHookManager
	dispatchChannel  chan Event
	stopChannel      chan struct{}
	waitGroup        sync.WaitGroup
	ctx              context.Context
	ctxCancel        context.CancelFunc
}

// NewEventBus initializes the universal EventBus and starts background worker goroutines.
func NewEventBus(db *DatabasePool, cryptoKeyManager *CryptoKeyManager) *EventBus {
	log.Debugf("initializing EventBus")
	eventBusCtx, eventBusCancel := context.WithCancel(context.Background())
	eventBus := &EventBus{
		subscribers:      make(map[string][]EventHandler),
		db:               db,
		cryptoKeyManager: cryptoKeyManager,
		dispatchChannel:  make(chan Event, defaultEventDispatchChannelCapacity),
		stopChannel:      make(chan struct{}),
		ctx:              eventBusCtx,
		ctxCancel:        eventBusCancel,
	}

	if db != nil {
		eventBus.eventManager = NewEventManager(db)
		eventBus.eventHookManager = NewEventHookManager(db, cryptoKeyManager, eventBus)
	}

	for index := 0; index < defaultEventWorkerCount; index++ {
		eventBus.waitGroup.Add(1)
		go eventBus.worker()
	}

	return eventBus
}

// SetEventManager overrides the EventManager instance.
func (eventBus *EventBus) SetEventManager(eventManager *EventManager) {
	eventBus.rwMutex.Lock()
	defer eventBus.rwMutex.Unlock()
	eventBus.eventManager = eventManager
}

// SetEventHookManager overrides the EventHookManager instance.
func (eventBus *EventBus) SetEventHookManager(eventHookManager *EventHookManager) {
	eventBus.rwMutex.Lock()
	defer eventBus.rwMutex.Unlock()
	eventBus.eventHookManager = eventHookManager
}

// Subscribe registers an in-memory handler for an event pattern.
func (eventBus *EventBus) Subscribe(pattern string, eventHandler EventHandler) {
	eventBus.rwMutex.Lock()
	defer eventBus.rwMutex.Unlock()
	eventBus.subscribers[pattern] = append(eventBus.subscribers[pattern], eventHandler)
}

// Publish enqueues an event for asynchronous persistence, in-memory notification, and hook dispatch.
func (eventBus *EventBus) Publish(ctx context.Context, event Event) {
	preparedEvent := prepareEvent(event)
	log.Tracef("publishing event %s (%s)", preparedEvent.ID, preparedEvent.Type)

	eventBus.dispatchInMemory(ctx, preparedEvent)

	select {
	case eventBus.dispatchChannel <- preparedEvent:
	default:
		log.Errorf("event dispatch buffer full, dropping event %s (%s)", preparedEvent.ID, preparedEvent.Type)
	}
}

// PublishSync executes event persistence and dispatch synchronously in the caller context.
func (eventBus *EventBus) PublishSync(ctx context.Context, event Event) {
	preparedEvent := prepareEvent(event)
	log.Tracef("synchronously publishing event %s (%s)", preparedEvent.ID, preparedEvent.Type)

	eventBus.rwMutex.RLock()
	var matchedHandlers []EventHandler
	for pattern, handlers := range eventBus.subscribers {
		if MatchEventPattern(pattern, preparedEvent.Type) {
			matchedHandlers = append(matchedHandlers, handlers...)
		}
	}
	eventBus.rwMutex.RUnlock()

	for _, subscriberHandler := range matchedHandlers {
		_ = subscriberHandler(ctx, preparedEvent)
	}

	eventBus.dispatch(ctx, preparedEvent)
}

func prepareEvent(event Event) Event {
	if event.ID == uuid.Nil() {
		event.ID = uuid.NewV7()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if event.Status == "" {
		event.Status = "success"
	}
	if event.Actor.Type == "" {
		event.Actor.Type = "system"
	}
	if event.Action == "" {
		event.Action = "event"
	}
	if event.ResourceType == "" {
		event.ResourceType = "event"
	}
	if event.Metadata == nil {
		event.Metadata = make(map[string]interface{})
	}
	if event.Payload == nil {
		event.Payload = make(map[string]interface{})
	}
	return event
}

func (eventBus *EventBus) dispatchInMemory(ctx context.Context, event Event) {
	eventBus.rwMutex.RLock()
	var matchedHandlers []EventHandler
	for pattern, handlers := range eventBus.subscribers {
		if MatchEventPattern(pattern, event.Type) {
			matchedHandlers = append(matchedHandlers, handlers...)
		}
	}
	eventBus.rwMutex.RUnlock()

	for _, subscriberHandler := range matchedHandlers {
		eventBus.waitGroup.Add(1)
		go func(eventHandler EventHandler) {
			defer eventBus.waitGroup.Done()
			_ = eventHandler(ctx, event)
		}(subscriberHandler)
	}
}

// Close stops all background dispatch workers cleanly.
func (eventBus *EventBus) Close() {
	log.Debugf("closing EventBus")
	close(eventBus.stopChannel)
	eventBus.ctxCancel()
	eventBus.waitGroup.Wait()
	log.Tracef("EventBus closed cleanly")
}

func (eventBus *EventBus) worker() {
	defer eventBus.waitGroup.Done()
	for {
		select {
		case <-eventBus.stopChannel:
			return
		case event := <-eventBus.dispatchChannel:
			eventBus.dispatch(eventBus.ctx, event)
		}
	}
}

func (eventBus *EventBus) dispatch(ctx context.Context, event Event) {
	log.Tracef("dispatching event hooks for %s (%s)", event.ID, event.Type)
	dispatchCtx, dispatchCancel := context.WithTimeout(ctx, defaultEventDispatchTimeout)
	defer dispatchCancel()

	eventBus.rwMutex.RLock()
	currentEventManager := eventBus.eventManager
	currentEventHookManager := eventBus.eventHookManager
	eventBus.rwMutex.RUnlock()

	// 1. Record event in core.events first to satisfy foreign key constraints on deliveries
	if currentEventManager != nil {
		if _, recordErr := currentEventManager.Record(dispatchCtx, event); recordErr != nil {
			log.Errorf("failed to record event %s in core.events: %v", event.ID, recordErr)
		}
	}

	// 2. Dispatch to matching event hooks (SQL / HTTP)
	if currentEventHookManager != nil {
		currentEventHookManager.Dispatch(dispatchCtx, event)
	}
}

// MatchEventPattern checks whether an eventName satisfies a pattern with wildcard support.
func MatchEventPattern(pattern, eventName string) bool {
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

// CalculateHookBackoff computes exponential backoff duration for delivery attempt index.
func CalculateHookBackoff(attempt int) time.Duration {
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}
