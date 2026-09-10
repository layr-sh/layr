package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"uuid"
)

func TestCoreEventHookFullLifecycleIntegration(t *testing.T) {
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
		CREATE OR REPLACE FUNCTION public.custom_hook_proc(event jsonb) RETURNS jsonb LANGUAGE plpgsql AS $$
		BEGIN
			RETURN json_build_object('ok', true, 'received_type', event->>'type')::jsonb;
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
	eventManager := eventBus.eventManager

	// Record a base event in core.events so foreign key constraints on deliveries succeed
	baseEvent, err := eventManager.Record(ctx, Event{
		Type:         "test.event.fired",
		Action:       "fired",
		ResourceType: "test",
		Data:         map[string]interface{}{"key": "value"},
	})
	if err != nil {
		t.Fatalf("failed to record base event: %v", err)
	}

	// 2. HTTP Hook Tests
	receivedEvents := make(chan string, 10)
	lastSignature := ""
	lastTimestamp := ""

	httpServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		lastSignature = request.Header.Get("X-Layr-Signature")
		lastTimestamp = request.Header.Get("X-Layr-Timestamp")
		receivedEvents <- request.Header.Get("X-Layr-Event")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"success":true}`))
	}))
	defer httpServer.Close()

	retries := 2
	timeout := 5
	secret := "test_signing_secret_123"

	// Create HTTP hook with signing secret
	httpEventHook, err := eventHookManager.Create(ctx, CreateEventHookInput{
		Name:           "Test HTTP Hook",
		Driver:         EventHookDriverHTTP,
		HTTPTargetURL:  &httpServer.URL,
		SigningSecret:  secret,
		EventTypes:     []string{"test.*"},
		MaxRetries:     &retries,
		TimeoutSeconds: &timeout,
	})
	if err != nil {
		t.Fatalf("failed to create HTTP hook: %v", err)
	}
	if !httpEventHook.SigningSecretConfigured {
		t.Fatal("expected SigningSecretConfigured to be true")
	}

	// Create SQL hook
	sqlProcName := "public.custom_hook_proc"
	sqlEventHook, err := eventHookManager.Create(ctx, CreateEventHookInput{
		Name:            "Test SQL Hook",
		Driver:          EventHookDriverSQL,
		SQLFunctionName: &sqlProcName,
		EventTypes:      []string{"test.*"},
		MaxRetries:      &retries,
		TimeoutSeconds:  &timeout,
	})
	if err != nil {
		t.Fatalf("failed to create SQL hook: %v", err)
	}

	// List hooks
	allHooks, err := eventHookManager.List(ctx, EventHookFilter{})
	if err != nil || len(allHooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d, err: %v", len(allHooks), err)
	}

	// Filter by driver
	sqlOnly, err := eventHookManager.List(ctx, EventHookFilter{Driver: stringPointer("sql")})
	if err != nil || len(sqlOnly) != 1 || sqlOnly[0].ID != sqlEventHook.ID {
		t.Fatalf("expected 1 sql hook from filter, got: %v", sqlOnly)
	}

	// Get hook by ID
	retrievedHTTPEventHook, err := eventHookManager.Get(ctx, httpEventHook.ID)
	if err != nil || retrievedHTTPEventHook.ID != httpEventHook.ID {
		t.Fatalf("failed to get HTTP hook by ID: %v", err)
	}

	// Get non-existent
	_, err = eventHookManager.Get(ctx, uuid.NewV7())
	if !errors.Is(err, ErrEventHookNotFound) {
		t.Fatalf("expected ErrEventHookNotFound, got: %v", err)
	}

	// Update hook
	updatedName := "Updated HTTP Hook"
	disabled := false
	updatedEventHook, err := eventHookManager.Update(ctx, httpEventHook.ID, UpdateEventHookInput{
		Name:      &updatedName,
		IsEnabled: &disabled,
	})
	if err != nil || updatedEventHook.Name != updatedName || updatedEventHook.IsEnabled != false {
		t.Fatalf("failed to update hook: %v", err)
	}

	// Re-enable hook
	enabled := true
	_, err = eventHookManager.Update(ctx, httpEventHook.ID, UpdateEventHookInput{
		IsEnabled: &enabled,
	})
	if err != nil {
		t.Fatalf("failed to re-enable hook: %v", err)
	}

	// Deliver to HTTP Hook
	httpEventHookDelivery, err := eventHookManager.Deliver(ctx, *httpEventHook, baseEvent)
	if err != nil {
		t.Fatalf("failed to deliver to HTTP hook: %v", err)
	}
	if !httpEventHookDelivery.IsDelivered || httpEventHookDelivery.HTTPResponseStatus == nil || *httpEventHookDelivery.HTTPResponseStatus != 200 {
		t.Fatalf("expected successful delivery, got: %+v", httpEventHookDelivery)
	}
	if len(lastSignature) == 0 || len(lastTimestamp) == 0 {
		t.Fatal("expected X-Layr-Signature and X-Layr-Timestamp headers to be sent")
	}

	// Deliver to SQL Hook
	sqlEventHookDelivery, err := eventHookManager.Deliver(ctx, *sqlEventHook, baseEvent)
	if err != nil {
		t.Fatalf("failed to deliver to SQL hook: %v", err)
	}
	if !sqlEventHookDelivery.IsDelivered {
		t.Fatalf("expected successful SQL delivery, got: %+v", sqlEventHookDelivery)
	}
	if sqlEventHookDelivery.Result == nil || *sqlEventHookDelivery.Result == "" {
		t.Fatal("expected non-empty SQL result")
	}

	// List Deliveries
	deliveries, err := eventHookManager.ListDeliveries(ctx, httpEventHook.ID)
	if err != nil || len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery in list, got %d, err: %v", len(deliveries), err)
	}

	// Get Delivery
	singleEventHookDelivery, err := eventHookManager.GetDelivery(ctx, httpEventHook.ID, httpEventHookDelivery.ID)
	if err != nil || singleEventHookDelivery.ID != httpEventHookDelivery.ID {
		t.Fatalf("expected delivery retrieved by ID, got: %v", err)
	}

	// Get non-existent delivery
	_, err = eventHookManager.GetDelivery(ctx, httpEventHook.ID, uuid.NewV7())
	if !errors.Is(err, ErrEventHookDeliveryNotFound) {
		t.Fatalf("expected ErrEventHookDeliveryNotFound, got: %v", err)
	}

	// Retry Delivery
	redrivenEventHookDelivery, err := eventHookManager.RetryDelivery(ctx, httpEventHookDelivery.ID)
	if err != nil || !redrivenEventHookDelivery.IsDelivered {
		t.Fatalf("failed to retry delivery: %v", err)
	}
	if redrivenEventHookDelivery.ID != httpEventHookDelivery.ID {
		t.Fatalf("expected redrive to reuse delivery ID %s, got %s", httpEventHookDelivery.ID, redrivenEventHookDelivery.ID)
	}

	// Test non-existent RetryDelivery
	_, err = eventHookManager.RetryDelivery(ctx, uuid.NewV7())
	if !errors.Is(err, ErrEventHookDeliveryNotFound) {
		t.Fatalf("expected ErrEventHookDeliveryNotFound, got: %v", err)
	}

	// Test HTTP Client Error (400) -> Non-retryable immediate failure
	clientErrorServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusBadRequest)
		_, _ = responseWriter.Write([]byte(`{"error":"bad_request"}`))
	}))
	defer clientErrorServer.Close()

	badRequestEventHook, err := eventHookManager.Create(ctx, CreateEventHookInput{
		Name:          "Bad Request Hook",
		Driver:        EventHookDriverHTTP,
		HTTPTargetURL: &clientErrorServer.URL,
		EventTypes:    []string{"test.*"},
	})
	if err != nil {
		t.Fatalf("failed to create bad request hook: %v", err)
	}

	failedEventHookDelivery, err := eventHookManager.Deliver(ctx, *badRequestEventHook, baseEvent)
	if err == nil {
		t.Fatal("expected error on 400 delivery")
	}
	if failedEventHookDelivery.IsDelivered {
		t.Fatal("expected delivery to be marked undelivered")
	}
	if failedEventHookDelivery.AttemptCount != 1 {
		t.Fatalf("expected client error to abort after 1 attempt, got %d", failedEventHookDelivery.AttemptCount)
	}

	// 11. Test Update validations and transitions
	if _, updateErr := eventHookManager.Update(ctx, httpEventHook.ID, UpdateEventHookInput{Name: stringPointer("  ")}); updateErr == nil {
		t.Fatal("expected error on empty name")
	}
	if _, updateErr := eventHookManager.Update(ctx, httpEventHook.ID, UpdateEventHookInput{Driver: stringPointer("invalid")}); updateErr == nil {
		t.Fatal("expected error on invalid driver")
	}
	if _, updateErr := eventHookManager.Update(ctx, sqlEventHook.ID, UpdateEventHookInput{SQLFunctionName: stringPointer("invalid fn;")}); updateErr == nil {
		t.Fatal("expected error on invalid sql function name")
	}
	if _, updateErr := eventHookManager.Update(ctx, sqlEventHook.ID, UpdateEventHookInput{SQLFunctionName: stringPointer("")}); updateErr == nil {
		t.Fatal("expected error when setting empty sql function name on sql driver")
	}
	if _, updateErr := eventHookManager.Update(ctx, httpEventHook.ID, UpdateEventHookInput{HTTPTargetURL: stringPointer("ftp://bad")}); updateErr == nil {
		t.Fatal("expected error on invalid http target url")
	}
	if _, updateErr := eventHookManager.Update(ctx, httpEventHook.ID, UpdateEventHookInput{HTTPTargetURL: stringPointer("")}); updateErr == nil {
		t.Fatal("expected error when setting empty http target url on http driver")
	}

	// Switch HTTP hook to SQL hook
	switchedSQLEventHook, updateErr := eventHookManager.Update(ctx, httpEventHook.ID, UpdateEventHookInput{
		Driver:          stringPointer("sql"),
		SQLFunctionName: stringPointer("public.custom_hook_proc"),
	})
	if updateErr != nil || switchedSQLEventHook.Driver != EventHookDriverSQL {
		t.Fatalf("expected switch to SQL driver, got: %v", updateErr)
	}

	// Switch back to HTTP hook with bounds and secret clearing
	isHookEnabled := true
	negativeRetries := -1
	excessiveTimeout := 100
	switchedHTTPEventHook, updateErr := eventHookManager.Update(ctx, httpEventHook.ID, UpdateEventHookInput{
		Driver:         stringPointer("http"),
		HTTPTargetURL:  &httpServer.URL,
		SigningSecret:  stringPointer("new-secret"),
		EventTypes:     []string{"*"},
		IsEnabled:      &isHookEnabled,
		MaxRetries:     &negativeRetries,
		TimeoutSeconds: &excessiveTimeout,
	})
	if updateErr != nil || switchedHTTPEventHook.Driver != EventHookDriverHTTP {
		t.Fatalf("expected switch back to HTTP driver, got: %v", updateErr)
	}

	// Update with empty secret to clear it, and bounds tests (maxRetries > 10, timeout < 1)
	excessiveRetries := 20
	zeroTimeout := 0
	_, updateErr = eventHookManager.Update(ctx, httpEventHook.ID, UpdateEventHookInput{
		SigningSecret:  stringPointer(""),
		MaxRetries:     &excessiveRetries,
		TimeoutSeconds: &zeroTimeout,
	})
	if updateErr != nil {
		t.Fatalf("failed to update hook bounds and clear secret: %v", updateErr)
	}

	// Update with valid bounded values
	validRetries := 5
	validTimeout := 15
	_, updateErr = eventHookManager.Update(ctx, httpEventHook.ID, UpdateEventHookInput{
		MaxRetries:     &validRetries,
		TimeoutSeconds: &validTimeout,
	})
	if updateErr != nil {
		t.Fatalf("failed to update hook with valid bounds: %v", updateErr)
	}

	// Update signing secret with nil cryptoKeyManager
	noCryptoEventHookManager := NewEventHookManager(db, nil, nil)
	if _, noCryptoErr := noCryptoEventHookManager.Update(ctx, httpEventHook.ID, UpdateEventHookInput{SigningSecret: stringPointer("secret")}); noCryptoErr == nil {
		t.Fatal("expected error when updating signing secret with nil cryptoKeyManager")
	}

	// Update signing secret with failing cryptoKeyManager
	failingCryptoKeyManager, cryptoKeyErr := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if cryptoKeyErr != nil {
		t.Fatalf("unexpected cryptoKeyManager init error: %v", cryptoKeyErr)
	}
	failingCryptoKeyManager.randomReader = &simulatedFailingReader{}
	failingCryptoEventHookManager := NewEventHookManager(db, failingCryptoKeyManager, nil)
	if _, updateEncErr := failingCryptoEventHookManager.Update(ctx, httpEventHook.ID, UpdateEventHookInput{SigningSecret: stringPointer("secret")}); updateEncErr == nil {
		t.Fatal("expected error when EncryptField fails in Update")
	}

	// 12. Test List with filters
	driverHTTP := EventHookDriverHTTP
	driverSQL := EventHookDriverSQL
	httpHooks, listErr := eventHookManager.List(ctx, EventHookFilter{Driver: &driverHTTP})
	if listErr != nil || len(httpHooks) == 0 {
		t.Fatalf("expected listed http hooks, got: %v", listErr)
	}
	sqlHooks, listErr := eventHookManager.List(ctx, EventHookFilter{Driver: &driverSQL})
	if listErr != nil || len(sqlHooks) == 0 {
		t.Fatalf("expected listed sql hooks, got: %v", listErr)
	}
	enabledHooks, listErr := eventHookManager.List(ctx, EventHookFilter{IsEnabled: &isHookEnabled})
	if listErr != nil || len(enabledHooks) == 0 {
		t.Fatalf("expected listed enabled hooks, got: %v", listErr)
	}

	// 13. SQL Hook permanent error path (non-existent function aborts after 1 attempt)
	badSQLFunctionName := "public.non_existent_procedure"
	badSQLEventHook, createErr := eventHookManager.Create(ctx, CreateEventHookInput{
		Name:            "Bad SQL Hook",
		Driver:          EventHookDriverSQL,
		SQLFunctionName: &badSQLFunctionName,
		EventTypes:      []string{"*"},
	})
	if createErr != nil {
		t.Fatalf("failed to create bad sql hook: %v", createErr)
	}
	failedSQLEventHookDelivery, deliverErr := eventHookManager.Deliver(ctx, *badSQLEventHook, baseEvent)
	if deliverErr == nil {
		t.Fatal("expected error on permanent sql error")
	}
	if failedSQLEventHookDelivery.AttemptCount != 1 {
		t.Fatalf("expected permanent sql error to stop after 1 attempt, got %d", failedSQLEventHookDelivery.AttemptCount)
	}

	// SQL Hook transient error with canceled context in sleep
	transientProcSQL := `
		CREATE OR REPLACE FUNCTION public.transient_fail_proc(event jsonb) RETURNS jsonb LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'deadlock detected: 40p01';
		END;
		$$;
	`
	_, _ = db.Exec(ctx, transientProcSQL)
	transientProcName := "public.transient_fail_proc"
	transientSQLEventHook, _ := eventHookManager.Create(ctx, CreateEventHookInput{
		Name:            "Transient SQL Hook",
		Driver:          EventHookDriverSQL,
		SQLFunctionName: &transientProcName,
		EventTypes:      []string{"*"},
	})
	transientCancelCtx, transientCancel := context.WithCancel(ctx)
	time.AfterFunc(100*time.Millisecond, transientCancel)
	transientEventHookDelivery, transientDeliverErr := eventHookManager.Deliver(transientCancelCtx, *transientSQLEventHook, baseEvent)
	if transientDeliverErr == nil {
		t.Fatal("expected error on transient SQL hook delivery with cancel")
	}
	if transientEventHookDelivery == nil || transientEventHookDelivery.ErrorMessage == nil || !strings.Contains(*transientEventHookDelivery.ErrorMessage, "deadlock") {
		t.Fatalf("expected transient deadlock error, got delivery: %+v, err: %v", transientEventHookDelivery, transientDeliverErr)
	}
	_ = eventHookManager.Delete(ctx, transientSQLEventHook.ID)

	// 14. HTTP response body truncation (>1024 bytes) and context cancellation in retry sleep
	errCancelCtx, errCancel := context.WithCancel(ctx)
	err500Server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		time.AfterFunc(10*time.Millisecond, errCancel)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		_, _ = responseWriter.Write([]byte(strings.Repeat("A", 2048)))
	}))
	defer err500Server.Close()

	err500EventHook, createErr := eventHookManager.Create(ctx, CreateEventHookInput{
		Name:          "500 Hook",
		Driver:        EventHookDriverHTTP,
		HTTPTargetURL: &err500Server.URL,
		EventTypes:    []string{"*"},
	})
	if createErr != nil {
		t.Fatalf("failed to create 500 hook: %v", createErr)
	}

	canceledEventHookDelivery, deliverErr := eventHookManager.Deliver(errCancelCtx, *err500EventHook, baseEvent)
	if deliverErr == nil {
		t.Fatal("expected delivery error with 500 server")
	}
	if canceledEventHookDelivery.Result == nil || len(*canceledEventHookDelivery.Result) != maxLoggedResponseBodyBytes {
		t.Fatalf("expected response body truncated to %d bytes", maxLoggedResponseBodyBytes)
	}

	// 15. Decrypt fallback on RetryDelivery when secret ciphertext is corrupted
	_, _ = db.Exec(ctx, "UPDATE core.event_hooks SET http_encrypted_signing_secret = 'corrupted-ciphertext' WHERE id = $1", httpEventHook.ID)
	if _, retryErr := eventHookManager.RetryDelivery(ctx, httpEventHookDelivery.ID); retryErr != nil {
		t.Fatalf("expected retry delivery to proceed even if secret decryption falls back, got error: %v", retryErr)
	}

	// RetryDelivery when hook does not exist
	_, _ = db.Exec(ctx, "SET session_replication_role = 'replica'")
	_, _ = db.Exec(ctx, "ALTER TABLE core.event_hook_deliveries DISABLE TRIGGER ALL")
	_, _ = db.Exec(ctx, "UPDATE core.event_hook_deliveries SET event_hook_id = $1 WHERE id = $2", uuid.NewV7(), httpEventHookDelivery.ID)
	if _, missingHookErr := eventHookManager.RetryDelivery(ctx, httpEventHookDelivery.ID); missingHookErr == nil {
		t.Fatal("expected error when hook does not exist on retry delivery")
	}
	_, _ = db.Exec(ctx, "UPDATE core.event_hook_deliveries SET event_hook_id = $1 WHERE id = $2", httpEventHook.ID, httpEventHookDelivery.ID)
	_, _ = db.Exec(ctx, "ALTER TABLE core.event_hook_deliveries ENABLE TRIGGER ALL")
	_, _ = db.Exec(ctx, "SET session_replication_role = 'origin'")

	// RetryDelivery with canceled context
	canceledRetryCtx, retryCancel := context.WithCancel(ctx)
	retryCancel()
	if _, retryErr := eventHookManager.RetryDelivery(canceledRetryCtx, httpEventHookDelivery.ID); retryErr == nil {
		t.Fatal("expected error on retry delivery with canceled context")
	}

	// RetryDelivery with nil UUID payload fallback
	_, _ = db.Exec(ctx, "UPDATE core.event_hook_deliveries SET payload = '{\"id\":\"00000000-0000-0000-0000-000000000000\"}' WHERE id = $1", httpEventHookDelivery.ID)
	_, _ = eventHookManager.RetryDelivery(ctx, httpEventHookDelivery.ID)

	// Delete with canceled context
	canceledDeleteCtx, deleteCancel := context.WithCancel(ctx)
	deleteCancel()
	if deleteErr := eventHookManager.Delete(canceledDeleteCtx, sqlEventHook.ID); deleteErr == nil {
		t.Fatal("expected error on delete with canceled context")
	}

	// Delete Hooks
	if err := eventHookManager.Delete(ctx, httpEventHook.ID); err != nil {
		t.Fatalf("failed to delete HTTP hook: %v", err)
	}
	if err := eventHookManager.Delete(ctx, sqlEventHook.ID); err != nil {
		t.Fatalf("failed to delete SQL hook: %v", err)
	}
	if err := eventHookManager.Delete(ctx, badSQLEventHook.ID); err != nil {
		t.Fatalf("failed to delete bad SQL hook: %v", err)
	}
	if err := eventHookManager.Delete(ctx, err500EventHook.ID); err != nil {
		t.Fatalf("failed to delete 500 hook: %v", err)
	}

	// Delete non-existent
	if err := eventHookManager.Delete(ctx, uuid.NewV7()); !errors.Is(err, ErrEventHookNotFound) {
		t.Fatalf("expected ErrEventHookNotFound on delete, got: %v", err)
	}
}

func stringPointer(value string) *string {
	return &value
}
