package core

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"

	"uuid"
)

func TestCoreEventMatchesPatternUnit(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		event         string
		expectedMatch bool
	}{
		{"wildcard all", "*", "auth.user.created", true},
		{"prefix wildcard", "auth.*", "auth.user.created", true},
		{"nested prefix wildcard", "auth.user.*", "auth.user.created", true},
		{"exact match", "auth.user.created", "auth.user.created", true},
		{"mismatch prefix", "data.*", "auth.user.created", false},
		{"suffix mismatch", "auth.user.deleted", "auth.user.created", false},
		{"exact prefix match without dot", "auth.*", "auth", true},
		{"unrelated event", "core.*", "auth.user.created", false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if match := MatchEventPattern(testCase.pattern, testCase.event); match != testCase.expectedMatch {
				t.Fatalf("expected MatchEventPattern(%q, %q) = %v, got %v", testCase.pattern, testCase.event, testCase.expectedMatch, match)
			}
		})
	}
}

func TestCoreEventCalculateBackoffUnit(t *testing.T) {
	backoffFirst := CalculateHookBackoff(1)
	backoffSecond := CalculateHookBackoff(2)
	backoffThird := CalculateHookBackoff(3)

	if backoffFirst <= 0 || backoffSecond <= backoffFirst || backoffThird <= backoffSecond {
		t.Fatalf("expected exponential increase in backoff: first=%v, second=%v, third=%v",
			backoffFirst, backoffSecond, backoffThird)
	}
}

func TestCoreEventPrepareUnit(t *testing.T) {
	ctx := context.Background()
	resourceID := "res_123"

	t.Run("invalid events missing required fields return errors", func(t *testing.T) {
		invalidEvents := []struct {
			name  string
			event Event
		}{
			{name: "empty event", event: Event{}},
			{name: "missing resource type and action", event: Event{Type: "test.event"}},
			{name: "missing action", event: Event{Type: "test.event", ResourceType: "test"}},
			{name: "missing resource ID", event: Event{Type: "test.event", ResourceType: "test", Action: "event"}},
		}
		for _, testCase := range invalidEvents {
			t.Run(testCase.name, func(t *testing.T) {
				_, err := prepareEvent(ctx, testCase.event)
				if err == nil {
					t.Fatalf("expected error on invalid event missing fields, got nil")
				}
			})
		}
	})

	t.Run("valid event receives defaults", func(t *testing.T) {
		validEvent := Event{
			Type:         "test.event",
			ResourceType: "test",
			Action:       "event",
			ResourceID:   &resourceID,
		}
		preparedEvent, err := prepareEvent(ctx, validEvent)
		if err != nil {
			t.Fatalf("unexpected error preparing event: %v", err)
		}
		if preparedEvent.ID == uuid.Nil() {
			t.Fatal("expected non-nil ID generated")
		}
		if preparedEvent.CreatedAt.IsZero() {
			t.Fatal("expected non-zero CreatedAt generated")
		}
		if preparedEvent.Actor.Type != "system" || preparedEvent.Actor.ID != nil || preparedEvent.Actor.Role != nil {
			t.Fatalf("expected default actor type 'system' with nil ID and Role, got: %+v", preparedEvent.Actor)
		}
		if preparedEvent.Metadata == nil {
			t.Fatal("expected initialized Metadata map")
		}
		if preparedEvent.Data == nil {
			t.Fatal("expected initialized Data map")
		}
	})

	t.Run("existing fields are preserved", func(t *testing.T) {
		customID := uuid.NewV7()
		customTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		existingEvent := Event{
			ID:           customID,
			Type:         "custom.event",
			Actor:        EventActor{Type: "user"},
			Action:       "create",
			ResourceType: "user",
			ResourceID:   &resourceID,
			Metadata:     map[string]interface{}{"key": "val"},
			Data:         map[string]interface{}{"foo": "bar"},
			CreatedAt:    customTime,
		}
		preservedEvent, err := prepareEvent(ctx, existingEvent)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if preservedEvent.ID != customID {
			t.Fatalf("expected preserved ID, got: %s", preservedEvent.ID)
		}
		if !preservedEvent.CreatedAt.Equal(customTime) {
			t.Fatalf("expected preserved CreatedAt, got: %v", preservedEvent.CreatedAt)
		}
		if preservedEvent.Actor.Type != "user" {
			t.Fatalf("expected preserved actor type 'user', got: %s", preservedEvent.Actor.Type)
		}
	})
}

func TestCoreEventManagerRecordInvalidEventUnit(t *testing.T) {
	eventManager := NewEventManager(nil)
	ctx := context.Background()

	_, err := eventManager.Record(ctx, Event{})
	if err == nil {
		t.Fatal("expected error recording empty event")
	}
}

func TestCoreEventBusLifecycleUnit(t *testing.T) {
	eventBus := NewEventBus(nil, nil)
	defer eventBus.Close()

	ctx := context.Background()
	receivedEvents := make(chan Event, 10)

	eventBus.Subscribe("test.*", func(eventCtx context.Context, event Event) error {
		receivedEvents <- event
		return nil
	})

	resourceID := "unit_123"
	testEvent := Event{
		Type:         "test.created",
		ResourceType: "test",
		Action:       "created",
		ResourceID:   &resourceID,
		Data:         map[string]interface{}{"name": "unit-test"},
	}

	eventBus.Publish(ctx, testEvent)

	select {
	case receivedEvent := <-receivedEvents:
		if receivedEvent.Type != "test.created" {
			t.Fatalf("expected event type test.created, got: %s", receivedEvent.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for in-memory event delivery")
	}

	// Test PublishSync
	var syncReceivedEvent Event
	eventBus.Subscribe("sync.event", func(eventCtx context.Context, event Event) error {
		syncReceivedEvent = event
		return nil
	})
	syncResourceID := "sync_123"
	eventBus.PublishSync(ctx, Event{
		Type:         "sync.event",
		ResourceType: "test",
		Action:       "sync",
		ResourceID:   &syncResourceID,
	})
	if syncReceivedEvent.Type != "sync.event" {
		t.Fatalf("expected sync event to be delivered, got: %s", syncReceivedEvent.Type)
	}

	// Test invalid events safely dropped in Publish and PublishSync
	eventBus.Publish(ctx, Event{})
	eventBus.PublishSync(ctx, Event{})

	// Test Getters
	_ = eventBus.EventManager()
	_ = eventBus.EventHookManager()

	// Test buffer full branch
	fullEventBus := &EventBus{
		dispatchChannel: make(chan Event, 1),
		subscribers:     make(map[string][]EventHandler),
	}
	overflowResourceID := "overflow_123"
	fullEventBus.dispatchChannel <- Event{ID: uuid.NewV7()}
	fullEventBus.Publish(ctx, Event{
		Type:         "overflow",
		ResourceType: "test",
		Action:       "overflow",
		ResourceID:   &overflowResourceID,
	})
}

func TestCoreEventNewEventUnit(t *testing.T) {
	ctx := context.Background()

	// 1. ServiceAccountCreated
	now := time.Now().UTC()
	consoleUserID := uuid.NewV7().String()
	desc := "SA Description"
	serviceAccountCreatedEvent := NewServiceAccountCreatedEvent("sa_123", ServiceAccountCreatedEventData{
		ID:            "sa_123",
		ConsoleUserID: &consoleUserID,
		Name:          "Test SA",
		Description:   &desc,
		KeyPrefix:     "pref_123",
		KeyHash:       "super-secret-hash",
		Scopes:        []string{"*"},
		IsEnabled:     true,
		AllowedIPs:    []string{"127.0.0.1"},
		CreatedAt:     now,
		LastUpdatedAt: now,
	})
	if serviceAccountCreatedEvent.ID == uuid.Nil() {
		t.Fatal("expected non-nil event ID")
	}
	if serviceAccountCreatedEvent.Type != "core.service_account.created" {
		t.Fatalf("unexpected event type: %s", serviceAccountCreatedEvent.Type)
	}
	if serviceAccountCreatedEvent.ResourceType != "core.service_account" {
		t.Fatalf("unexpected resource type: %s", serviceAccountCreatedEvent.ResourceType)
	}
	if serviceAccountCreatedEvent.Action != "created" {
		t.Fatalf("unexpected action: %s", serviceAccountCreatedEvent.Action)
	}
	if serviceAccountCreatedEvent.ResourceID == nil || *serviceAccountCreatedEvent.ResourceID != "sa_123" {
		t.Fatalf("unexpected resource ID: %v", serviceAccountCreatedEvent.ResourceID)
	}
	if serviceAccountCreatedEvent.Metadata == nil {
		t.Fatal("expected non-nil metadata")
	}
	if serviceAccountCreatedEvent.Data["name"] != "Test SA" {
		t.Fatalf("unexpected data: %v", serviceAccountCreatedEvent.Data)
	}
	if serviceAccountCreatedEvent.Data["key_prefix"] != "pref_123" {
		t.Fatalf("unexpected data key_prefix: %v", serviceAccountCreatedEvent.Data)
	}
	if _, hasKeyHash := serviceAccountCreatedEvent.Data["key_hash"]; hasKeyHash {
		t.Fatal("expected key_hash to be omitted from event data")
	}
	if serviceAccountCreatedEvent.CreatedAt.IsZero() {
		t.Fatal("expected non-zero created_at")
	}

	// 2. ServiceAccountUpdated
	serviceAccountUpdatedEvent := NewServiceAccountUpdatedEvent("sa_123", ServiceAccountUpdatedEventData{
		ID:        "sa_123",
		Name:      "Updated SA",
		Scopes:    []string{"read"},
		IsEnabled: true,
	})
	if serviceAccountUpdatedEvent.Action != "updated" || serviceAccountUpdatedEvent.Data["name"] != "Updated SA" {
		t.Fatalf("unexpected update event: %v", serviceAccountUpdatedEvent)
	}

	// 3. ServiceAccountDeleted
	serviceAccountDeletedEvent := NewServiceAccountDeletedEvent("sa_123", ServiceAccountDeletedEventData{
		ID:        "sa_123",
		Name:      "Deleted SA",
		IsEnabled: false,
	})
	if serviceAccountDeletedEvent.Action != "deleted" || serviceAccountDeletedEvent.Data["name"] != "Deleted SA" {
		t.Fatalf("unexpected delete event: %v", serviceAccountDeletedEvent)
	}

	// 4. EventHookCreated
	hookID := uuid.NewV7()
	secret := "encrypted-secret"
	hookCreatedEvent := NewEventHookCreatedEvent(hookID.String(), EventHookCreatedEventData{
		ID:                         hookID,
		Name:                       "Hook 1",
		Driver:                     "http",
		HTTPEncryptedSigningSecret: &secret,
		IsEnabled:                  true,
	})
	if hookCreatedEvent.ResourceType != "core.event_hook" || hookCreatedEvent.Action != "created" {
		t.Fatalf("unexpected hook created event: %v", hookCreatedEvent)
	}
	if hookCreatedEvent.Data["driver"] != "http" {
		t.Fatalf("unexpected driver: %v", hookCreatedEvent.Data["driver"])
	}
	if _, hasSecret := hookCreatedEvent.Data["http_encrypted_signing_secret"]; hasSecret {
		t.Fatal("expected http_encrypted_signing_secret to be omitted from event data")
	}

	// 5. EventHookUpdated
	hookUpdatedEvent := NewEventHookUpdatedEvent(hookID.String(), EventHookUpdatedEventData{
		ID:        hookID,
		Name:      "Hook Updated",
		Driver:    "sql",
		IsEnabled: true,
	})
	if hookUpdatedEvent.Action != "updated" || hookUpdatedEvent.Data["driver"] != "sql" {
		t.Fatalf("unexpected hook updated event: %v", hookUpdatedEvent)
	}

	// 6. EventHookDeleted
	hookDeletedEvent := NewEventHookDeletedEvent(hookID.String(), EventHookDeletedEventData{
		ID:   hookID,
		Name: "Deleted Hook",
	})
	if hookDeletedEvent.Action != "deleted" || hookDeletedEvent.Data["name"] != "Deleted Hook" {
		t.Fatalf("unexpected hook deleted event: %v", hookDeletedEvent)
	}

	// 7. EventHookDeliveryRetried
	deliveryID := uuid.NewV7()
	status200 := 200
	hookDeliveryRetriedEvent := NewEventHookDeliveryRetriedEvent(hookID.String(), EventHookDeliveryRetriedEventData{
		ID:                 deliveryID,
		EventHookID:        hookID,
		EventType:          "auth.user.created",
		HTTPResponseStatus: &status200,
		AttemptCount:       2,
		IsDelivered:        true,
	})
	if hookDeliveryRetriedEvent.Type != "core.event_hook.delivery_retried" || hookDeliveryRetriedEvent.ResourceType != "core.event_hook" || hookDeliveryRetriedEvent.Action != "delivery_retried" {
		t.Fatalf("unexpected delivery retried event: %v", hookDeliveryRetriedEvent)
	}
	if hookDeliveryRetriedEvent.ResourceID == nil || *hookDeliveryRetriedEvent.ResourceID != hookID.String() {
		t.Fatalf("unexpected resource ID: %v", hookDeliveryRetriedEvent.ResourceID)
	}

	// 8. NodeRegistered
	nodeUUID := uuid.NewV7()
	nodeRegisteredEvent := NewNodeRegisteredEvent(nodeUUID.String(), NodeRegisteredEventData{
		ID:              nodeUUID,
		NodeName:        "worker-node-1",
		EnabledServices: []string{"data", "auth"},
	})
	if nodeRegisteredEvent.Type != "core.node.registered" || nodeRegisteredEvent.ResourceType != "core.node" || nodeRegisteredEvent.Action != "registered" {
		t.Fatalf("unexpected node registered event: %v", nodeRegisteredEvent)
	}
	if nodeRegisteredEvent.Data["node_name"] != "worker-node-1" {
		t.Fatalf("unexpected node_name: %v", nodeRegisteredEvent.Data["node_name"])
	}

	// 9. NodeUnregistered
	nodeUnregisteredEvent := NewNodeUnregisteredEvent(nodeUUID.String(), NodeUnregisteredEventData{
		ID:       nodeUUID,
		NodeName: "worker-node-1",
	})
	if nodeUnregisteredEvent.Type != "core.node.unregistered" || nodeUnregisteredEvent.ResourceType != "core.node" || nodeUnregisteredEvent.Action != "unregistered" {
		t.Fatalf("unexpected node unregistered event: %v", nodeUnregisteredEvent)
	}
	if nodeUnregisteredEvent.Data["node_name"] != "worker-node-1" {
		t.Fatalf("unexpected node_name: %v", nodeUnregisteredEvent.Data["node_name"])
	}

	// 10. Fluent methods: WithMetadata, WithActor, WithContext, WithResourceID
	customEventActor := EventActor{Type: "custom_actor"}
	customEventContext := EventContext{}
	fluentEvent := NewServiceAccountCreatedEvent("sa_123", ServiceAccountCreatedEventData{
		Name:   "Fluent SA",
		Scopes: []string{"*"},
	}).WithMetadata(map[string]interface{}{
		"client": "web",
	}).WithActor(customEventActor).WithContext(customEventContext).WithResourceID("sa_custom")

	if fluentEvent.ResourceID == nil || *fluentEvent.ResourceID != "sa_custom" {
		t.Fatalf("expected resourceID sa_custom, got: %v", fluentEvent.ResourceID)
	}
	if fluentEvent.Metadata["client"] != "web" {
		t.Fatalf("expected metadata client=web, got: %v", fluentEvent.Metadata)
	}
	if fluentEvent.Actor.Type != "custom_actor" {
		t.Fatalf("expected actor type custom_actor, got: %s", fluentEvent.Actor.Type)
	}

	nilMetadataEvent := Event{}.WithMetadata(map[string]interface{}{"init": "true"})
	if nilMetadataEvent.Metadata["init"] != "true" {
		t.Fatalf("expected init=true, got: %v", nilMetadataEvent.Metadata)
	}

	// 8. Context metadata merging in prepareEvent
	contextMetadata := map[string]interface{}{
		"env":    "production",
		"client": "ignored_override",
	}
	metaCtx := WithEventMetadata(ctx, contextMetadata)
	preparedMetaEvent, err := prepareEvent(metaCtx, fluentEvent)
	if err != nil {
		t.Fatalf("unexpected error preparing event: %v", err)
	}
	if preparedMetaEvent.Metadata["env"] != "production" {
		t.Fatalf("expected merged env=production, got: %v", preparedMetaEvent.Metadata["env"])
	}
	if preparedMetaEvent.Metadata["client"] != "web" {
		t.Fatalf("expected preserved explicit client=web, got: %v", preparedMetaEvent.Metadata["client"])
	}

	// 9. NewEvent directly and WithResourceID empty branch
	rawEvent := NewEvent("custom.resource.action", map[string]string{"foo": "bar"}).WithResourceID("res_99")
	if rawEvent.Type != "custom.resource.action" || rawEvent.ResourceType != "custom.resource" || rawEvent.Action != "action" {
		t.Fatalf("unexpected rawEvent fields: %+v", rawEvent)
	}
	if rawEvent.ResourceID == nil || *rawEvent.ResourceID != "res_99" || rawEvent.Data["foo"] != "bar" {
		t.Fatalf("unexpected rawEvent data/resourceID: %+v", rawEvent)
	}

	emptyResourceEvent := NewEvent("malformed", nil).WithResourceID("")
	if emptyResourceEvent.ResourceID != nil {
		t.Fatalf("expected nil resource ID on empty input, got: %v", emptyResourceEvent.ResourceID)
	}
	if emptyResourceEvent.Action != "" || emptyResourceEvent.ResourceType != "" {
		t.Fatalf("expected empty action and resource type on malformed type string, got: %s / %s", emptyResourceEvent.Action, emptyResourceEvent.ResourceType)
	}
}

func TestCoreEventActorAndContextDerivationUnit(t *testing.T) {
	ctx := context.Background()
	resourceID := "sa_1"
	validEvent := Event{
		Type:         "core.service_account.deleted",
		ResourceType: "core.service_account",
		Action:       "deleted",
		ResourceID:   &resourceID,
	}

	// 1. Empty context -> system actor and empty context
	emptyEvent, err := prepareEvent(ctx, validEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if emptyEvent.Actor.Type != "system" || emptyEvent.Actor.ID != nil || emptyEvent.Actor.Role != nil {
		t.Fatalf("unexpected system actor: %+v", emptyEvent.Actor)
	}
	if emptyEvent.Context.IPAddress != nil || emptyEvent.Context.UserAgent != nil || emptyEvent.Context.RequestID != nil {
		t.Fatalf("expected empty context struct, got: %+v", emptyEvent.Context)
	}

	// 2. Machine ServiceAccount context
	serviceAccountID := uuid.NewV7().String()
	serviceAccountCtx := WithServiceAccount(ctx, &ServiceAccount{
		ID:     serviceAccountID,
		Name:   "machine-sa",
		Scopes: []string{"*"},
	})
	serviceAccountEvent, err := prepareEvent(serviceAccountCtx, validEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if serviceAccountEvent.Actor.Type != "service_account" || serviceAccountEvent.Actor.ID == nil || serviceAccountEvent.Actor.ID.String() != serviceAccountID {
		t.Fatalf("unexpected service account actor: %+v", serviceAccountEvent.Actor)
	}
	if serviceAccountEvent.Actor.Role == nil || *serviceAccountEvent.Actor.Role != "root" {
		t.Fatalf("expected root role, got: %v", serviceAccountEvent.Actor.Role)
	}

	// Non-root service account
	nonRootCtx := WithServiceAccount(ctx, &ServiceAccount{
		ID:     serviceAccountID,
		Name:   "regular-sa",
		Scopes: []string{"read"},
	})
	nonRootEvent, err := prepareEvent(nonRootCtx, validEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nonRootEvent.Actor.Role == nil || *nonRootEvent.Actor.Role != "service_account" {
		t.Fatalf("expected service_account role, got: %v", nonRootEvent.Actor.Role)
	}

	// Unparseable SA ID
	badServiceAccountCtx := WithServiceAccount(ctx, &ServiceAccount{
		ID:   "invalid-uuid",
		Name: "bad-sa",
	})
	badServiceAccountEvent, err := prepareEvent(badServiceAccountCtx, validEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if badServiceAccountEvent.Actor.ID != nil {
		t.Fatalf("expected nil actor ID on unparseable SA ID, got: %v", badServiceAccountEvent.Actor.ID)
	}

	// 3. Console User via ServiceAccount with ConsoleUserID
	consoleUserID := uuid.NewV7().String()
	consoleServiceAccountCtx := WithServiceAccount(ctx, &ServiceAccount{
		ID:            serviceAccountID,
		Name:          "console-session",
		ConsoleUserID: &consoleUserID,
	})
	consoleServiceAccountEvent, err := prepareEvent(consoleServiceAccountCtx, validEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consoleServiceAccountEvent.Actor.Type != "console_user" || consoleServiceAccountEvent.Actor.ID == nil || consoleServiceAccountEvent.Actor.ID.String() != consoleUserID {
		t.Fatalf("unexpected console user actor: %+v", consoleServiceAccountEvent.Actor)
	}
	if consoleServiceAccountEvent.Actor.Role == nil || *consoleServiceAccountEvent.Actor.Role != "console_user" {
		t.Fatalf("expected console_user role, got: %v", consoleServiceAccountEvent.Actor.Role)
	}

	// Unparseable ConsoleUserID
	badConsoleID := "invalid-uuid"
	badConsoleCtx := WithServiceAccount(ctx, &ServiceAccount{
		ID:            serviceAccountID,
		Name:          "bad-console-sa",
		ConsoleUserID: &badConsoleID,
	})
	badConsoleEvent, err := prepareEvent(badConsoleCtx, validEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if badConsoleEvent.Actor.ID != nil {
		t.Fatalf("expected nil actor ID on unparseable console user ID, got: %v", badConsoleEvent.Actor.ID)
	}

	// 4. Explicit EventActor (e.g. End User)
	userUUID := uuid.NewV7()
	userRole := "member"
	userCtx := WithEventActor(ctx, EventActor{
		Type: "user",
		ID:   &userUUID,
		Role: &userRole,
	})
	retrievedEventActor, ok := GetEventActor(userCtx)
	if !ok || retrievedEventActor.Type != "user" || retrievedEventActor.ID == nil || *retrievedEventActor.ID != userUUID {
		t.Fatalf("GetEventActor failed: %+v", retrievedEventActor)
	}
	userEvent, err := prepareEvent(userCtx, validEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userEvent.Actor.Type != "user" || userEvent.Actor.ID == nil || *userEvent.Actor.ID != userUUID {
		t.Fatalf("unexpected user actor: %+v", userEvent.Actor)
	}

	// Missing actor in GetEventActor
	if _, missingOk := GetEventActor(ctx); missingOk {
		t.Fatal("expected false from GetEventActor on empty context")
	}

	// 5. Explicit EventContext
	clientIP := "192.168.1.1"
	userAgent := "CustomClient/1.0"
	requestID := "req_custom_123"
	transportCtx := WithEventContext(ctx, EventContext{
		IPAddress: &clientIP,
		UserAgent: &userAgent,
		RequestID: &requestID,
	})
	retrievedEventContext, ctxOk := GetEventContext(transportCtx)
	if !ctxOk || *retrievedEventContext.IPAddress != clientIP || *retrievedEventContext.UserAgent != userAgent || *retrievedEventContext.RequestID != requestID {
		t.Fatalf("GetEventContext failed: %+v", retrievedEventContext)
	}
	transportEvent, err := prepareEvent(transportCtx, validEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transportEvent.Context.IPAddress == nil || *transportEvent.Context.IPAddress != clientIP {
		t.Fatalf("unexpected IP address in event context: %v", transportEvent.Context.IPAddress)
	}

	// Missing context in GetEventContext
	if _, missingCtxOk := GetEventContext(ctx); missingCtxOk {
		t.Fatal("expected false from GetEventContext on empty context")
	}

	// 6. Missing metadata in GetEventMetadata
	if _, missingMetaOk := GetEventMetadata(ctx); missingMetaOk {
		t.Fatal("expected false from GetEventMetadata on empty context")
	}
}

func TestCoreEventConstructorParameterSignaturesUnit(t *testing.T) {
	fileSet := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fileSet, "event.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse event.go: %v", err)
	}

	testedCount := 0
	for _, decl := range parsedFile.Decls {
		functionDeclaration, ok := decl.(*ast.FuncDecl)
		if !ok || functionDeclaration.Recv != nil {
			continue
		}
		name := functionDeclaration.Name.Name
		if !strings.HasPrefix(name, "New") || !strings.HasSuffix(name, "Event") || name == "NewEvent" {
			continue
		}

		testedCount++
		t.Run(name, func(t *testing.T) {
			params := functionDeclaration.Type.Params.List
			if len(params) == 0 {
				t.Fatalf("constructor %s has no parameters", name)
			}
			firstParamField := params[0]
			if len(firstParamField.Names) == 0 {
				t.Fatalf("constructor %s first parameter has no name", name)
			}
			paramName := firstParamField.Names[0].Name
			if paramName != "resourceID" {
				t.Errorf("constructor %s first parameter expected 'resourceID', got '%s'", name, paramName)
			}
		})
	}

	if testedCount == 0 {
		t.Fatal("no constructors found to test")
	}
}
