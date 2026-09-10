package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"uuid"
)

func TestCoreEventUniversalPipelineIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	postgresContainer, containerErr := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if containerErr != nil {
		t.Skip("docker not available")
		return
	}
	defer func() { _ = postgresContainer.Terminate(ctx) }()

	databaseURL, _ := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	db, err := NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create db connection pool: %v", err)
	}
	defer db.Close()

	if migrationErr := db.RunMigrations(ctx, SystemDatabaseMigrations); migrationErr != nil {
		t.Fatalf("failed to run migrations: %v", migrationErr)
	}

	// 1. Create a test PostgreSQL procedure for SQL hook
	procSQL := `
		CREATE OR REPLACE FUNCTION public.test_event_proc(event jsonb) RETURNS jsonb LANGUAGE plpgsql AS $$
		BEGIN
			RETURN json_build_object('handled', true, 'type', event->>'type')::jsonb;
		END;
		$$;
	`
	if _, execErr := db.Exec(ctx, procSQL); execErr != nil {
		t.Fatalf("failed to create test procedure: %v", execErr)
	}

	cryptoKeyManager, _ := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	eventBus := NewEventBus(db, cryptoKeyManager)
	defer eventBus.Close()

	eventHookManager := eventBus.eventHookManager

	// 2. Set up HTTP hook receiver
	httpHookDeliveredSignal := make(chan string, 1)
	httpHookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		httpHookDeliveredSignal <- request.Header.Get("X-Layr-Event")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"status":"received"}`))
	}))
	defer httpHookServer.Close()

	// Register HTTP Event Hook
	createdHTTPEventHook, err := eventHookManager.Create(ctx, CreateEventHookInput{
		Name:          "Integration HTTP Hook",
		Driver:        EventHookDriverHTTP,
		HTTPTargetURL: &httpHookServer.URL,
		EventTypes:    []string{"auth.*"},
	})
	if err != nil {
		t.Fatalf("failed to create HTTP event hook: %v", err)
	}

	// Register SQL Event Hook
	sqlProcName := "public.test_event_proc"
	createdSQLEventHook, err := eventHookManager.Create(ctx, CreateEventHookInput{
		Name:            "Integration SQL Hook",
		Driver:          EventHookDriverSQL,
		SQLFunctionName: &sqlProcName,
		EventTypes:      []string{"auth.*"},
	})
	if err != nil {
		t.Fatalf("failed to create SQL event hook: %v", err)
	}

	// Subscribe In-Memory Listener
	inMemoryDeliveredSignal := make(chan string, 1)
	eventBus.Subscribe("auth.*", func(eventCtx context.Context, event Event) error {
		inMemoryDeliveredSignal <- event.Type
		return nil
	})

	// 3. Publish Event
	actorID := uuid.NewV7()
	actorRole := "admin"
	ipAddress := "192.168.1.100"
	userAgent := "Mozilla/5.0 TestAgent"
	requestID := "req_xyz789"
	resourceID := "usr_123456"

	sampleEvent := Event{
		Type:         "auth.user.created",
		Action:       "created",
		ResourceType: "user",
		ResourceID:   &resourceID,
		Actor: EventActor{
			Type: "user",
			ID:   &actorID,
			Role: &actorRole,
		},
		Context: EventContext{
			IPAddress: &ipAddress,
			UserAgent: &userAgent,
			RequestID: &requestID,
		},
		Metadata: map[string]interface{}{"source": "registration_form"},
		Data:     map[string]interface{}{"email": "user@example.com"},
	}

	eventBus.Publish(ctx, sampleEvent)

	// Assert In-Memory Delivery
	select {
	case inMemoryEventType := <-inMemoryDeliveredSignal:
		if inMemoryEventType != "auth.user.created" {
			t.Fatalf("unexpected in-memory event: %s", inMemoryEventType)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for in-memory delivery")
	}

	// Assert HTTP Hook Delivery
	select {
	case hookEventType := <-httpHookDeliveredSignal:
		if hookEventType != "auth.user.created" {
			t.Fatalf("unexpected hook event: %s", hookEventType)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for HTTP hook delivery")
	}

	// Assert Event Persistence in core.events
	eventManager := eventBus.eventManager
	eventsList, err := eventManager.List(ctx, EventFilter{
		Type:         &sampleEvent.Type,
		ResourceType: &sampleEvent.ResourceType,
	})
	if err != nil {
		t.Fatalf("failed to list events: %v", err)
	}
	if len(eventsList) == 0 {
		t.Fatal("expected at least 1 recorded event in core.events")
	}

	recordedEvent, err := eventManager.Get(ctx, eventsList[0].ID)
	if err != nil {
		t.Fatalf("failed to get event by ID: %v", err)
	}
	if recordedEvent.Type != "auth.user.created" {
		t.Fatalf("expected event type 'auth.user.created', got: %s", recordedEvent.Type)
	}
	if recordedEvent.ResourceID == nil || *recordedEvent.ResourceID != "usr_123456" {
		t.Fatalf("expected resource ID 'usr_123456', got: %v", recordedEvent.ResourceID)
	}

	// Assert Deliveries recorded in core.event_hook_deliveries
	var httpDeliveryCount int
	for retries := 0; retries < 20; retries++ {
		deliveries, listErr := eventHookManager.ListDeliveries(ctx, createdHTTPEventHook.ID)
		if listErr == nil && len(deliveries) > 0 && deliveries[0].IsDelivered {
			httpDeliveryCount = len(deliveries)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if httpDeliveryCount == 0 {
		t.Fatal("expected successful delivery recorded for HTTP hook")
	}

	var sqlDeliveryCount int
	for retries := 0; retries < 20; retries++ {
		deliveries, listErr := eventHookManager.ListDeliveries(ctx, createdSQLEventHook.ID)
		if listErr == nil && len(deliveries) > 0 && deliveries[0].IsDelivered {
			sqlDeliveryCount = len(deliveries)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if sqlDeliveryCount == 0 {
		t.Fatal("expected successful delivery recorded for SQL hook")
	}

	// 4. Test EventManager.Record unmarshal fallback with non-marshalable values
	_, unmarshalRecordErr := eventManager.Record(ctx, Event{
		Type:     "test.unmarshal.fallback",
		Metadata: map[string]interface{}{"invalid": make(chan int)},
		Data:     map[string]interface{}{"invalid": make(chan int)},
	})
	if unmarshalRecordErr != nil {
		t.Fatalf("expected record to succeed with json fallback, got: %v", unmarshalRecordErr)
	}

	// 5. Test EventManager.List filters, limit cap, negative offset
	now := time.Now().UTC()
	startTime := now.Add(-1 * time.Hour)
	endTime := now.Add(1 * time.Hour)
	targetActorType := "user"
	targetResourceType := "user"
	targetResourceID := "usr_123456"
	targetStatus := "success"

	filteredEvents, filterErr := eventManager.List(ctx, EventFilter{
		Type:         &sampleEvent.Type,
		ActorType:    &targetActorType,
		ActorID:      &actorID,
		ResourceType: &targetResourceType,
		ResourceID:   &targetResourceID,
		Status:       &targetStatus,
		StartDate:    &startTime,
		EndDate:      &endTime,
		Limit:        500, // exceeds maxEventListLimit -> tests limit cap
		Offset:       -10, // negative offset -> tests offset < 0
	})
	if filterErr != nil {
		t.Fatalf("failed to list filtered events: %v", filterErr)
	}
	if len(filteredEvents) == 0 {
		t.Fatal("expected at least one event matching all filters")
	}

	// Query error branch with canceled context
	canceledCtx, listCancel := context.WithCancel(ctx)
	listCancel()
	if _, queryErr := eventManager.List(canceledCtx, EventFilter{}); queryErr == nil {
		t.Fatal("expected query error on canceled context")
	}
}
