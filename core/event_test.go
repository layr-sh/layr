package core

import (
	"context"
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
	// Empty Event should receive defaults
	emptyEvent := Event{
		Type: "test.event",
	}
	preparedEvent := prepareEvent(emptyEvent)
	if preparedEvent.ID == uuid.Nil() {
		t.Fatal("expected non-nil ID generated")
	}
	if preparedEvent.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt generated")
	}
	if preparedEvent.Status != "success" {
		t.Fatalf("expected default status 'success', got: %s", preparedEvent.Status)
	}
	if preparedEvent.Actor.Type != "system" {
		t.Fatalf("expected default actor type 'system', got: %s", preparedEvent.Actor.Type)
	}
	if preparedEvent.Action != "event" {
		t.Fatalf("expected default action 'event', got: %s", preparedEvent.Action)
	}
	if preparedEvent.ResourceType != "event" {
		t.Fatalf("expected default resource type 'event', got: %s", preparedEvent.ResourceType)
	}
	if preparedEvent.Metadata == nil {
		t.Fatal("expected initialized Metadata map")
	}
	if preparedEvent.Payload == nil {
		t.Fatal("expected initialized Payload map")
	}

	// Existing fields should be preserved
	customID := uuid.NewV7()
	customTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	existingEvent := Event{
		ID:           customID,
		Type:         "custom.event",
		Actor:        EventActor{Type: "user"},
		Action:       "create",
		ResourceType: "user",
		Status:       "failed",
		Metadata:     map[string]interface{}{"key": "val"},
		Payload:      map[string]interface{}{"foo": "bar"},
		CreatedAt:    customTime,
	}
	preservedEvent := prepareEvent(existingEvent)
	if preservedEvent.ID != customID {
		t.Fatalf("expected preserved ID, got: %s", preservedEvent.ID)
	}
	if !preservedEvent.CreatedAt.Equal(customTime) {
		t.Fatalf("expected preserved CreatedAt, got: %v", preservedEvent.CreatedAt)
	}
	if preservedEvent.Status != "failed" {
		t.Fatalf("expected preserved status 'failed', got: %s", preservedEvent.Status)
	}
	if preservedEvent.Actor.Type != "user" {
		t.Fatalf("expected preserved actor type 'user', got: %s", preservedEvent.Actor.Type)
	}
}

func TestCoreEventManagerNilDBUnit(t *testing.T) {
	eventManager := NewEventManager(nil)
	ctx := context.Background()

	// 1. Record with nil db
	_, err := eventManager.Record(ctx, Event{Type: "test.event"})
	if err == nil {
		t.Fatal("expected error recording event with nil db")
	}

	// 2. Get with nil db
	_, err = eventManager.Get(ctx, uuid.NewV7())
	if err == nil {
		t.Fatal("expected error getting event with nil db")
	}

	// 3. List with nil db
	_, err = eventManager.List(ctx, EventFilter{})
	if err == nil {
		t.Fatal("expected error listing events with nil db")
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

	testEvent := Event{
		Type:         "test.created",
		ResourceType: "test",
		Payload:      map[string]interface{}{"name": "unit-test"},
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
	eventBus.PublishSync(ctx, Event{Type: "sync.event", ResourceType: "test"})
	if syncReceivedEvent.Type != "sync.event" {
		t.Fatalf("expected sync event to be delivered, got: %s", syncReceivedEvent.Type)
	}

	// Test Setters
	eventBus.SetEventManager(nil)
	eventBus.SetEventHookManager(nil)

	// Test buffer full branch
	fullEventBus := &EventBus{
		dispatchChannel: make(chan Event, 1),
		subscribers:     make(map[string][]EventHandler),
	}
	fullEventBus.dispatchChannel <- Event{ID: uuid.NewV7()}
	fullEventBus.Publish(ctx, Event{Type: "overflow"})
}
