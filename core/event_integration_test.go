package core

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"uuid"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCoreEventConfigAndRetentionIntegration(t *testing.T) {
	kernel, cleanup := SetupTestKernel(t, nil)
	defer cleanup()
	ctx := t.Context()
	require.NoError(t, kernel.configManager.Load(ctx))
	require.Equal(t, 90, kernel.configManager.Get().AuditRetentionDays)
	require.ErrorIs(t, kernel.configManager.Set(ctx, RuntimeConfig{}), ErrInvalidConfig)
	recordedEvent, recordErr := kernel.EventManager().Record(ctx, NewConfigUpdatedEvent("", ConfigUpdatedEventData(DefaultRuntimeConfig())))
	require.NoError(t, recordErr)
	require.Empty(t, recordedEvent.ResourceType)
	require.NotNil(t, recordedEvent.ResourceID)
	require.Empty(t, *recordedEvent.ResourceID)
	storedEvent, getErr := kernel.EventManager().Get(ctx, recordedEvent.ID)
	require.NoError(t, getErr)
	require.Empty(t, storedEvent.ResourceType)
	require.NotNil(t, storedEvent.ResourceID)
	require.Empty(t, *storedEvent.ResourceID)
	var err error
	var eventID, hookID, deliveryID string
	require.NoError(t, kernel.DB().QueryRow(ctx, `
		INSERT INTO core.events(type, action, resource_type, resource_id)
		VALUES ('auth.user.created', 'created', 'auth.user', 'retention-test') RETURNING id
	`).Scan(&eventID))
	require.NoError(t, kernel.DB().QueryRow(ctx, `
		INSERT INTO core.event_hooks(name, driver, event_types, http_target_url)
		VALUES ('Retention Test', 'http', '{}', 'https://example.com/hook') RETURNING id
	`).Scan(&hookID))
	require.NoError(t, kernel.DB().QueryRow(ctx, `
		INSERT INTO core.event_hook_deliveries(event_hook_id, event_id, event_type, payload)
		VALUES ($1, $2, 'auth.user.created', '{}') RETURNING id
	`, hookID, eventID).Scan(&deliveryID))
	_, err = kernel.DB().Exec(ctx, "UPDATE core.events SET created_at = clock_timestamp() - interval '100 days' WHERE id = $1", eventID)
	require.NoError(t, err)
	require.NoError(t, kernel.EventManager().Prune(ctx))
	var exists bool
	require.NoError(t, kernel.DB().QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM core.events WHERE id = $1)", eventID).Scan(&exists))
	require.False(t, exists)
	require.NoError(t, kernel.DB().QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM core.event_hook_deliveries WHERE id = $1)", deliveryID).Scan(&exists))
	require.False(t, exists)

	server := NewServer(kernel)
	for _, testCase := range []struct {
		method, payload, scope string
		status                 int
	}{
		{http.MethodGet, "", "", http.StatusForbidden},
		{http.MethodPut, `{}`, ScopeCoreConfigRead, http.StatusForbidden},
		{http.MethodGet, "", ScopeCoreConfigRead, http.StatusOK},
		{http.MethodPut, `{invalid`, ScopeCoreConfigWrite, http.StatusBadRequest},
		{http.MethodPut, `{"audit_retention_days":0}`, ScopeCoreConfigWrite, http.StatusBadRequest},
		{http.MethodPut, `{"audit_retention_days":1}`, ScopeCoreConfigWrite, http.StatusOK},
	} {
		t.Run(testCase.method+testCase.payload+testCase.scope, func(t *testing.T) {
			request := httptest.NewRequestWithContext(WithAuthContext(ctx, AuthContext{ServiceAccountID: "test", JWT: JWTClaims{Scope: testCase.scope}}), testCase.method, "/v1/_/core/config", bytes.NewBufferString(testCase.payload))
			responseRecorder := httptest.NewRecorder()
			server.Handler().ServeHTTP(responseRecorder, request)
			require.Equal(t, testCase.status, responseRecorder.Code, responseRecorder.Body.String())
			if testCase.status == http.StatusOK {
				var runtimeConfig RuntimeConfig
				require.NoError(t, json.Unmarshal(responseRecorder.Body.Bytes(), &runtimeConfig))
				require.Positive(t, runtimeConfig.AuditRetentionDays)
			}
		})
	}
	_, err = kernel.DB().Exec(ctx, `INSERT INTO core.events(type, action, resource_type, resource_id, created_at) VALUES ('storage.object.created','created','storage.object','retention-test',clock_timestamp()-interval '2 days')`)
	require.NoError(t, err)
	retentionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	kernel.EventBus().waitGroup.Add(1)
	go func() {
		kernel.EventBus().runRetention(retentionCtx, 5*time.Millisecond)
		close(done)
	}()
	require.Eventually(t, func() bool {
		queryErr := kernel.DB().QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM core.events WHERE type='storage.object.created')").Scan(&exists)
		return queryErr == nil && !exists
	}, 5*time.Second, 5*time.Millisecond)
	cancel()
	<-done

	canceledCtx, canceledCancel := context.WithCancel(ctx)
	canceledCancel()
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		t.Run("unavailable configuration "+method, func(t *testing.T) {
			request := httptest.NewRequestWithContext(WithAuthContext(canceledCtx, AuthContext{ServiceAccountID: "test", JWT: JWTClaims{Scope: ScopeRoot}}), method, "/v1/_/core/config", bytes.NewBufferString(`{"audit_retention_days":30}`))
			responseRecorder := httptest.NewRecorder()
			server.Handler().ServeHTTP(responseRecorder, request)
			if method == http.MethodPut {
				require.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
			} else {
				require.Equal(t, http.StatusOK, responseRecorder.Code)
			}
		})
	}
	_, err = kernel.DB().Exec(ctx, "DROP TABLE core.config")
	require.NoError(t, err)
	failureCtx, failureCancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer failureCancel()
	kernel.EventBus().waitGroup.Add(1)
	kernel.EventBus().runRetention(failureCtx, time.Millisecond)
}

func TestCoreEventResourceLessWildcardDeliveryIntegration(t *testing.T) {
	kernel, cleanup := SetupTestKernel(t, nil)
	defer cleanup()
	ctx := t.Context()
	deliveredEvents := make(chan Event, 3)
	inMemoryEvents := make(chan Event, 3)
	webhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var event Event
		if err := json.NewDecoder(request.Body).Decode(&event); err != nil {
			responseWriter.WriteHeader(http.StatusBadRequest)
			return
		}
		deliveredEvents <- event
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()
	for _, pattern := range []string{"*", "core.*", "core.config.updated"} {
		t.Run("register "+pattern, func(t *testing.T) {
			_, err := kernel.EventBus().eventHookManager.Create(ctx, CreateEventHookInput{
				Name: pattern, Driver: EventHookDriverHTTP, HTTPTargetURL: &webhookServer.URL, EventTypes: []string{pattern},
			})
			require.NoError(t, err)
			kernel.EventBus().Subscribe(pattern, func(_ context.Context, event Event) error {
				inMemoryEvents <- event
				return nil
			})
		})
	}
	configEvent := NewConfigUpdatedEvent("", ConfigUpdatedEventData(DefaultRuntimeConfig()))
	kernel.EventBus().Publish(ctx, configEvent)
	for name, deliveries := range map[string]chan Event{"in memory": inMemoryEvents, "webhook": deliveredEvents} {
		t.Run(name, func(t *testing.T) {
			for range 3 {
				t.Run("delivery", func(t *testing.T) {
					select {
					case event := <-deliveries:
						require.Equal(t, configEvent.ID, event.ID)
						require.Equal(t, "core.config.updated", event.Type)
						require.Empty(t, event.ResourceType)
						require.NotNil(t, event.ResourceID)
						require.Empty(t, *event.ResourceID)
					case <-time.After(15 * time.Second):
						t.Fatal("timed out waiting for resource-less event delivery")
					}
				})
			}
		})
	}
	storedEvent, err := kernel.EventManager().Get(ctx, configEvent.ID)
	require.NoError(t, err)
	require.Empty(t, storedEvent.ResourceType)
	require.NotNil(t, storedEvent.ResourceID)
	require.Empty(t, *storedEvent.ResourceID)

	t.Run("unreachable webhook preserves resource-less payload", func(t *testing.T) {
		webhookServer.Close()
		eventHook, createErr := kernel.EventBus().eventHookManager.Create(ctx, CreateEventHookInput{
			Name: "Unavailable Hook", Driver: EventHookDriverHTTP, HTTPTargetURL: &webhookServer.URL, EventTypes: []string{"core.config.updated"},
		})
		require.NoError(t, createErr)
		eventHook.MaxRetries = 1
		eventHookDelivery, deliveryErr := kernel.EventBus().eventHookManager.DeliverWithID(ctx, *eventHook, storedEvent, uuid.NewV7())
		require.ErrorContains(t, deliveryErr, "failed to execute http request")
		require.NotNil(t, eventHookDelivery)
		require.False(t, eventHookDelivery.IsDelivered)
		require.Equal(t, storedEvent, eventHookDelivery.Payload)
	})
}

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
		httpHookDeliveredSignal <- request.Header.Get("Webhook-Event")
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
	fallbackResourceID := "fallback_123"
	_, unmarshalRecordErr := eventManager.Record(ctx, Event{
		Type:         "test.unmarshal.fallback",
		Action:       "fallback",
		ResourceType: "test",
		ResourceID:   &fallbackResourceID,
		Metadata:     map[string]interface{}{"invalid": make(chan int)},
		Data:         map[string]interface{}{"invalid": make(chan int)},
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

	filteredEvents, filterErr := eventManager.List(ctx, EventFilter{
		Type:         &sampleEvent.Type,
		ActorType:    &targetActorType,
		ActorID:      &actorID,
		ResourceType: &targetResourceType,
		ResourceID:   &targetResourceID,
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
