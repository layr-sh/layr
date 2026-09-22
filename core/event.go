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
	Metadata     map[string]interface{} `json:"metadata"`
	Data         map[string]interface{} `json:"data"`
	CreatedAt    time.Time              `json:"created_at"`
}

// WithActor sets the EventActor on the event.
func (event Event) WithActor(eventActor EventActor) Event {
	event.Actor = eventActor
	return event
}

// WithContext sets the EventContext on the event.
func (event Event) WithContext(eventContext EventContext) Event {
	event.Context = eventContext
	return event
}

// WithResourceID sets the ResourceID on the event.
func (event Event) WithResourceID(resourceID string) Event {
	if resourceID != "" {
		event.ResourceID = &resourceID
	}
	return event
}

// WithMetadata attaches or merges key-value metadata into the event.
func (event Event) WithMetadata(metadata map[string]interface{}) Event {
	if event.Metadata == nil {
		event.Metadata = make(map[string]interface{})
	}
	for key, value := range metadata {
		event.Metadata[key] = value
	}
	return event
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
	preparedEvent, err := prepareEvent(ctx, event)
	if err != nil {
		return Event{}, err
	}

	metadataJSON, err := json.Marshal(preparedEvent.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	dataJSON, err := json.Marshal(preparedEvent.Data)
	if err != nil {
		dataJSON = []byte("{}")
	}

	query := `
		INSERT INTO core.events (
			id, type, actor_id, actor_type, ip_address, user_agent, action, resource_type, resource_id, metadata, data, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		RETURNING id, type, actor_id, actor_type, ip_address::text, user_agent, action, resource_type, resource_id, metadata, data, created_at
	`

	var recordedEvent Event
	var actorID *uuid.UUID
	var ipAddress *string
	var userAgent *string
	var resourceID *string
	var metadataRaw []byte
	var dataRaw []byte

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
		metadataJSON,
		dataJSON,
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
		&metadataRaw,
		&dataRaw,
		&recordedEvent.CreatedAt,
	)
	if err != nil {
		return Event{}, fmt.Errorf("failed to record event: %w", err)
	}

	recordedEvent.Actor.ID = actorID
	recordedEvent.Context.IPAddress = ipAddress
	recordedEvent.Context.UserAgent = userAgent
	recordedEvent.ResourceID = resourceID
	_ = json.Unmarshal(metadataRaw, &recordedEvent.Metadata)
	_ = json.Unmarshal(dataRaw, &recordedEvent.Data)

	log.Tracef("recorded event %s (%s)", recordedEvent.ID, recordedEvent.Type)
	return recordedEvent, nil
}

// Get retrieves an event by its UUID from core.events.
func (eventManager *EventManager) Get(ctx context.Context, eventUUID uuid.UUID) (Event, error) {
	query := `
		SELECT id, type, actor_id, actor_type, ip_address::text, user_agent, action, resource_type, resource_id, metadata, data, created_at
		FROM core.events
		WHERE id = $1
	`

	var event Event
	var actorID *uuid.UUID
	var ipAddress *string
	var userAgent *string
	var resourceID *string
	var metadataRaw []byte
	var dataRaw []byte

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
		&metadataRaw,
		&dataRaw,
		&event.CreatedAt,
	)
	if err != nil {
		return Event{}, ErrEventNotFound
	}

	event.Actor.ID = actorID
	event.Context.IPAddress = ipAddress
	event.Context.UserAgent = userAgent
	event.ResourceID = resourceID
	_ = json.Unmarshal(metadataRaw, &event.Metadata)
	_ = json.Unmarshal(dataRaw, &event.Data)

	return event, nil
}

// List returns a list of events matching the provided filter criteria.
func (eventManager *EventManager) List(ctx context.Context, eventFilter EventFilter) ([]Event, error) {
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
		SELECT id, type, actor_id, actor_type, ip_address::text, user_agent, action, resource_type, resource_id, metadata, data, created_at
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
		var metadataRaw []byte
		var dataRaw []byte

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
			&metadataRaw,
			&dataRaw,
			&event.CreatedAt,
		)

		event.Actor.ID = actorID
		event.Context.IPAddress = ipAddress
		event.Context.UserAgent = userAgent
		event.ResourceID = resourceID
		_ = json.Unmarshal(metadataRaw, &event.Metadata)
		_ = json.Unmarshal(dataRaw, &event.Data)

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
		eventBus.eventHookManager = NewEventHookManager(db, cryptoKeyManager)
	}

	for index := 0; index < defaultEventWorkerCount; index++ {
		eventBus.waitGroup.Add(1)
		go eventBus.worker()
	}

	return eventBus
}

// EventManager returns the underlying EventManager instance.
func (eventBus *EventBus) EventManager() *EventManager {
	eventBus.rwMutex.RLock()
	defer eventBus.rwMutex.RUnlock()
	return eventBus.eventManager
}

// EventHookManager returns the underlying EventHookManager instance.
func (eventBus *EventBus) EventHookManager() *EventHookManager {
	eventBus.rwMutex.RLock()
	defer eventBus.rwMutex.RUnlock()
	return eventBus.eventHookManager
}

// Subscribe registers an in-memory handler for an event pattern.
func (eventBus *EventBus) Subscribe(pattern string, eventHandler EventHandler) {
	eventBus.rwMutex.Lock()
	defer eventBus.rwMutex.Unlock()
	eventBus.subscribers[pattern] = append(eventBus.subscribers[pattern], eventHandler)
}

// Publish enqueues an event for asynchronous persistence, in-memory notification, and hook dispatch.
func (eventBus *EventBus) Publish(ctx context.Context, event Event) {
	preparedEvent, err := prepareEvent(ctx, event)
	if err != nil {
		log.Errorf("failed to publish event: %v", err)
		return
	}
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
	preparedEvent, err := prepareEvent(ctx, event)
	if err != nil {
		log.Errorf("failed to synchronously publish event: %v", err)
		return
	}
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

func prepareEvent(ctx context.Context, event Event) (Event, error) {
	if event.Type == "" || event.ResourceType == "" || event.Action == "" || event.ResourceID == nil || *event.ResourceID == "" {
		return Event{}, errors.New("event requires non-empty type, resource_type, action, and resource_id")
	}

	if event.ID == uuid.Nil() {
		event.ID = uuid.NewV7()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if event.Actor.Type == "" {
		event.Actor = deriveEventActor(ctx)
	}
	if event.Context.RequestID == nil && event.Context.IPAddress == nil && event.Context.UserAgent == nil {
		event.Context = deriveEventContext(ctx)
	}
	if event.Metadata == nil {
		event.Metadata = make(map[string]interface{})
	}
	if contextMetadata, ok := GetEventMetadata(ctx); ok {
		for key, value := range contextMetadata {
			if _, exists := event.Metadata[key]; !exists {
				event.Metadata[key] = value
			}
		}
	}
	if event.Data == nil {
		event.Data = make(map[string]interface{})
	}
	return event, nil
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

type eventContextKey string

const (
	eventActorContextKey     eventContextKey = "core_event_actor"
	eventTransportContextKey eventContextKey = "core_event_transport_context"
	eventMetadataContextKey  eventContextKey = "core_event_metadata"
)

// WithEventActor injects an EventActor into context.
func WithEventActor(ctx context.Context, eventActor EventActor) context.Context {
	return context.WithValue(ctx, eventActorContextKey, eventActor)
}

// GetEventActor retrieves an EventActor from context if present.
func GetEventActor(ctx context.Context) (EventActor, bool) {
	if eventActor, ok := ctx.Value(eventActorContextKey).(EventActor); ok {
		return eventActor, true
	}
	return EventActor{}, false
}

// WithEventContext injects an EventContext into context.
func WithEventContext(ctx context.Context, eventContext EventContext) context.Context {
	return context.WithValue(ctx, eventTransportContextKey, eventContext)
}

// GetEventContext retrieves an EventContext from context if present.
func GetEventContext(ctx context.Context) (EventContext, bool) {
	if eventContext, ok := ctx.Value(eventTransportContextKey).(EventContext); ok {
		return eventContext, true
	}
	return EventContext{}, false
}

// WithEventMetadata returns a context carrying event metadata to be merged into dispatched events.
func WithEventMetadata(ctx context.Context, metadata map[string]interface{}) context.Context {
	return context.WithValue(ctx, eventMetadataContextKey, metadata)
}

// GetEventMetadata retrieves event metadata from context if present.
func GetEventMetadata(ctx context.Context) (map[string]interface{}, bool) {
	if metadata, ok := ctx.Value(eventMetadataContextKey).(map[string]interface{}); ok {
		return metadata, true
	}
	return nil, false
}

func deriveEventActor(ctx context.Context) EventActor {
	if eventActor, ok := GetEventActor(ctx); ok && eventActor.Type != "" {
		return eventActor
	}
	if serviceAccount := GetServiceAccount(ctx); serviceAccount != nil {
		if serviceAccount.ConsoleUserID != nil && *serviceAccount.ConsoleUserID != "" {
			var consoleUserID *uuid.UUID
			if parsedUUID, err := uuid.Parse(*serviceAccount.ConsoleUserID); err == nil {
				consoleUserID = &parsedUUID
			}
			role := "console_user"
			return EventActor{
				Type: "console_user",
				ID:   consoleUserID,
				Role: &role,
			}
		}
		var serviceAccountID *uuid.UUID
		if parsedUUID, err := uuid.Parse(serviceAccount.ID); err == nil {
			serviceAccountID = &parsedUUID
		}
		role := "service_account"
		if HasScope(serviceAccount.Scopes, "*") {
			role = "root"
		}
		return EventActor{
			Type: "service_account",
			ID:   serviceAccountID,
			Role: &role,
		}
	}
	return EventActor{
		Type: "system",
	}
}

func deriveEventContext(ctx context.Context) EventContext {
	if eventContext, ok := GetEventContext(ctx); ok {
		return eventContext
	}
	return EventContext{}
}

// NewEvent creates a new canonical Event from an eventType string and payload data.
func NewEvent(eventType string, data any) Event {
	lastDotIndex := strings.LastIndex(eventType, ".")
	var resourceType, action string
	if lastDotIndex > 0 && lastDotIndex < len(eventType)-1 {
		resourceType = eventType[:lastDotIndex]
		action = eventType[lastDotIndex+1:]
	}

	dataMap := make(map[string]interface{})
	dataBytes, _ := json.Marshal(data)
	_ = json.Unmarshal(dataBytes, &dataMap)

	return Event{
		ID:           uuid.NewV7(),
		Type:         eventType,
		Action:       action,
		ResourceType: resourceType,
		Metadata:     make(map[string]interface{}),
		Data:         dataMap,
		CreatedAt:    time.Now().UTC(),
	}
}

// ServiceAccountCreatedEventData represents the payload for core.service_account.created.
type ServiceAccountCreatedEventData ServiceAccount

// NewServiceAccountCreatedEvent creates a typed event for service account creation.
func NewServiceAccountCreatedEvent(id string, serviceAccountCreatedEventData ServiceAccountCreatedEventData) Event {
	return NewEvent("core.service_account.created", serviceAccountCreatedEventData).WithResourceID(id)
}

// ServiceAccountUpdatedEventData represents the payload for core.service_account.updated.
type ServiceAccountUpdatedEventData ServiceAccount

// NewServiceAccountUpdatedEvent creates a typed event for service account update.
func NewServiceAccountUpdatedEvent(id string, serviceAccountUpdatedEventData ServiceAccountUpdatedEventData) Event {
	return NewEvent("core.service_account.updated", serviceAccountUpdatedEventData).WithResourceID(id)
}

// ServiceAccountDeletedEventData represents the payload for core.service_account.deleted.
type ServiceAccountDeletedEventData ServiceAccount

// NewServiceAccountDeletedEvent creates a typed event for service account deletion.
func NewServiceAccountDeletedEvent(id string, serviceAccountDeletedEventData ServiceAccountDeletedEventData) Event {
	return NewEvent("core.service_account.deleted", serviceAccountDeletedEventData).WithResourceID(id)
}

// EventHookCreatedEventData represents the payload for core.event_hook.created.
type EventHookCreatedEventData EventHook

// NewEventHookCreatedEvent creates a typed event for event hook creation.
func NewEventHookCreatedEvent(id string, eventHookCreatedEventData EventHookCreatedEventData) Event {
	return NewEvent("core.event_hook.created", eventHookCreatedEventData).WithResourceID(id)
}

// EventHookUpdatedEventData represents the payload for core.event_hook.updated.
type EventHookUpdatedEventData EventHook

// NewEventHookUpdatedEvent creates a typed event for event hook update.
func NewEventHookUpdatedEvent(id string, eventHookUpdatedEventData EventHookUpdatedEventData) Event {
	return NewEvent("core.event_hook.updated", eventHookUpdatedEventData).WithResourceID(id)
}

// EventHookDeletedEventData represents the payload for core.event_hook.deleted.
type EventHookDeletedEventData EventHook

// NewEventHookDeletedEvent creates a typed event for event hook deletion.
func NewEventHookDeletedEvent(id string, eventHookDeletedEventData EventHookDeletedEventData) Event {
	return NewEvent("core.event_hook.deleted", eventHookDeletedEventData).WithResourceID(id)
}
