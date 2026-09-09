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
)

func TestCoreWebhookFullLifecycleIntegration(t *testing.T) {
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

	cryptoKeyManager, _ := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	webhookEventBus := NewWebhookEventBus(db, cryptoKeyManager)
	defer webhookEventBus.Close()

	webhookManager := NewWebhookManager(db, cryptoKeyManager, webhookEventBus)

	// 1. Create a local HTTP receiver server to test successful webhook delivery
	receivedChannel := make(chan string, 1)
	webhookServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		receivedChannel <- request.Header.Get("X-Layr-Event")
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"status":"ok"}`))
	}))
	defer webhookServer.Close()

	// In-memory subscription test
	inMemoryReceivedChannel := make(chan string, 10)
	webhookEventBus.Subscribe("core.*", func(ctx context.Context, webhookEventEnvelope WebhookEventEnvelope) error {
		select {
		case inMemoryReceivedChannel <- webhookEventEnvelope.Event:
		default:
		}
		return nil
	})

	// 1. Create Webhook pointing to local server
	retryCount := 1
	timeoutSeconds := 2
	webhook, err := webhookManager.Create(ctx, CreateWebhookInput{
		Name:           "Core Events",
		TargetURL:      webhookServer.URL,
		Events:         []string{"core.service_account.*", "auth.user.created"},
		SigningSecret:  "whsec_test_secret_12345",
		MaxRetries:     &retryCount,
		TimeoutSeconds: &timeoutSeconds,
	})
	if err != nil {
		t.Fatalf("failed to create webhook: %v", err)
	}
	if !webhook.SigningSecretMasked {
		t.Fatal("expected signing secret to be masked")
	}

	// 2. List
	list, err := webhookManager.List(ctx)
	if err != nil {
		t.Fatalf("failed to list webhooks: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 webhook, got: %d", len(list))
	}

	// 3. Get
	retrievedWebhook, err := webhookManager.Get(ctx, webhook.ID)
	if err != nil {
		t.Fatalf("failed to get webhook by id: %v", err)
	}
	if retrievedWebhook.ID != webhook.ID {
		t.Fatalf("id mismatch: %s != %s", retrievedWebhook.ID, webhook.ID)
	}

	// Get non-existent -> ErrWebhookNotFound
	if _, getNotFoundErr := webhookManager.Get(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(getNotFoundErr, ErrWebhookNotFound) {
		t.Fatalf("expected ErrWebhookNotFound, got: %v", getNotFoundErr)
	}

	// 4. Update
	newName := "Updated Webhook Subscription"
	updatedWebhook, err := webhookManager.Update(ctx, webhook.ID, UpdateWebhookInput{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("failed to update webhook: %v", err)
	}
	if updatedWebhook.Name != "Updated Webhook Subscription" {
		t.Fatalf("expected name 'Updated Webhook', got: %s", updatedWebhook.Name)
	}

	// 5. Publish Event & Deliver
	webhookEventBus.Publish(ctx, WebhookEventEnvelope{
		Event:    "core.service_account.created",
		Service:  "core",
		Resource: "service_account",
		Action:   "created",
		Data:     map[string]string{"name": "Test Service Account"},
	})

	// Verify in-memory handler received event
	select {
	case eventName := <-inMemoryReceivedChannel:
		if eventName != "core.service_account.created" {
			t.Fatalf("unexpected in-memory event: %s", eventName)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for in-memory subscriber")
	}

	// Verify HTTP webhook delivery received
	select {
	case eventName := <-receivedChannel:
		if eventName != "core.service_account.created" {
			t.Fatalf("unexpected webhook delivered event: %s", eventName)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for HTTP webhook delivery")
	}

	time.Sleep(200 * time.Millisecond)

	// 6. Check Deliveries
	deliveries, err := webhookManager.ListDeliveries(ctx, webhook.ID)
	if err != nil {
		t.Fatalf("failed to list deliveries: %v", err)
	}
	if len(deliveries) == 0 {
		t.Fatal("expected at least 1 delivery record in history")
	}
	if !deliveries[0].IsDelivered {
		t.Fatalf("expected delivery to be marked as is_delivered=true")
	}

	// 7. Delete
	if deleteErr := webhookManager.Delete(ctx, webhook.ID); deleteErr != nil {
		t.Fatalf("failed to delete webhook: %v", deleteErr)
	}

	// Delete non-existent webhook -> ErrWebhookNotFound
	if deleteNotFoundErr := webhookManager.Delete(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(deleteNotFoundErr, ErrWebhookNotFound) {
		t.Fatalf("expected ErrWebhookNotFound, got: %v", deleteNotFoundErr)
	}

	// 8. Test Failed Delivery & Broadcast to core.webhook.delivery_failed
	failChannel := make(chan string, 10)
	webhookEventBus.Subscribe("core.webhook.delivery_failed", func(ctx context.Context, webhookEventEnvelope WebhookEventEnvelope) error {
		select {
		case failChannel <- webhookEventEnvelope.Event:
		default:
		}
		return nil
	})

	failServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusInternalServerError)
		_, _ = responseWriter.Write([]byte(`internal error`))
	}))
	defer failServer.Close()

	failedRetryCount := 1
	failedTimeoutSeconds := 1
	failureWebhook, err := webhookManager.Create(ctx, CreateWebhookInput{
		Name:           "Failing Webhook",
		TargetURL:      failServer.URL,
		Events:         []string{"*"},
		MaxRetries:     &failedRetryCount,
		TimeoutSeconds: &failedTimeoutSeconds,
	})
	if err != nil {
		t.Fatalf("failed to create failure webhook: %v", err)
	}

	webhookEventBus.Publish(ctx, WebhookEventEnvelope{
		Event:   "auth.user.deleted",
		Service: "auth",
	})

	select {
	case eventName := <-failChannel:
		if eventName != "core.webhook.delivery_failed" {
			t.Fatalf("unexpected fail broadcast: %s", eventName)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for failure broadcast")
	}

	_ = webhookManager.Delete(ctx, failureWebhook.ID)

	// 9. WebhookManager nil db connection pool and error branches
	nilWebhookManager := NewWebhookManager(nil, nil, nil)
	if _, nilDBErr := nilWebhookManager.Create(ctx, CreateWebhookInput{Name: "X", TargetURL: "http://example.com", Events: []string{"*"}}); nilDBErr == nil {
		t.Fatal("expected error on nil db connection pool Create")
	}
	if _, nilDBErr := nilWebhookManager.List(ctx); nilDBErr == nil {
		t.Fatal("expected error on nil db connection pool List")
	}
	if _, nilDBErr := nilWebhookManager.Get(ctx, "00000000-0000-0000-0000-000000000000"); nilDBErr == nil {
		t.Fatal("expected error on nil db connection pool Get")
	}
	if _, nilDBErr := nilWebhookManager.Update(ctx, "00000000-0000-0000-0000-000000000000", UpdateWebhookInput{}); nilDBErr == nil {
		t.Fatal("expected error on nil db connection pool Update")
	}
	if nilDBErr := nilWebhookManager.Delete(ctx, "00000000-0000-0000-0000-000000000000"); nilDBErr == nil {
		t.Fatal("expected error on nil db connection pool Delete")
	}
	if _, nilDBErr := nilWebhookManager.ListDeliveries(ctx, "00000000-0000-0000-0000-000000000000"); nilDBErr == nil {
		t.Fatal("expected error on nil db connection pool ListDeliveries")
	}

	// Validation errors on Create
	if _, emptyNameErr := webhookManager.Create(ctx, CreateWebhookInput{Name: "", TargetURL: "http://example.com", Events: []string{"*"}}); emptyNameErr == nil {
		t.Fatal("expected error on empty name")
	}
	if _, emptyURLErr := webhookManager.Create(ctx, CreateWebhookInput{Name: "WH", TargetURL: "", Events: []string{"*"}}); emptyURLErr == nil {
		t.Fatal("expected error on empty target URL")
	}
	if _, emptyEventsErr := webhookManager.Create(ctx, CreateWebhookInput{Name: "WH", TargetURL: "http://example.com", Events: []string{}}); emptyEventsErr == nil {
		t.Fatal("expected error on empty events")
	}
	if _, ssrfURLErr := webhookManager.Create(ctx, CreateWebhookInput{Name: "WH", TargetURL: "http://169.254.169.254/latest", Events: []string{"*"}}); ssrfURLErr == nil {
		t.Fatal("expected error on SSRF target URL")
	}

	// Update all fields of webhook
	updateAllFieldsWebhook, err := webhookManager.Create(ctx, CreateWebhookInput{
		Name:      "Update Target",
		TargetURL: "http://example.com/target",
		Events:    []string{"core.*"},
	})
	if err != nil {
		t.Fatalf("failed to create webhook for update: %v", err)
	}

	updateTargetURL := "http://example.com/new-target"
	updateEvents := []string{"auth.*"}
	updateEnabled := false
	updateRetries := 5
	updateTimeout := 20
	updateSecret := "new_secret_12345"

	updatedAllFieldsWebhook, err := webhookManager.Update(ctx, updateAllFieldsWebhook.ID, UpdateWebhookInput{
		TargetURL:      &updateTargetURL,
		Events:         updateEvents,
		IsEnabled:      &updateEnabled,
		MaxRetries:     &updateRetries,
		TimeoutSeconds: &updateTimeout,
		SigningSecret:  &updateSecret,
	})
	if err != nil {
		t.Fatalf("failed to update all webhook fields: %v", err)
	}
	if updatedAllFieldsWebhook.TargetURL != updateTargetURL || updatedAllFieldsWebhook.IsEnabled != false || updatedAllFieldsWebhook.MaxRetries != 5 {
		t.Fatalf("expected updated webhook fields, got: %+v", updatedAllFieldsWebhook)
	}

	emptyURL := "   "
	if _, emptyURLErr := webhookManager.Update(ctx, updateAllFieldsWebhook.ID, UpdateWebhookInput{TargetURL: &emptyURL}); emptyURLErr == nil {
		t.Fatal("expected error on updating to empty target URL")
	}
	ssrfURL := "http://169.254.169.254/latest"
	if _, ssrfURLErr := webhookManager.Update(ctx, updateAllFieldsWebhook.ID, UpdateWebhookInput{TargetURL: &ssrfURL}); ssrfURLErr == nil {
		t.Fatal("expected error on updating to SSRF target URL")
	}

	// 10. EventBus direct dispatch and deliver edge cases
	nilWebhookEventBus := NewWebhookEventBus(nil, nil)
	nilWebhookEventBus.dispatch(ctx, WebhookEventEnvelope{Event: "test"})

	// Direct deliver with invalid URL & zero retries/timeouts
	webhookEventBus.deliver(ctx, "invalid-id", "://invalid-url", "", 0, 0, WebhookEventEnvelope{
		ID:        "01a02ef2-ce86-722a-bffb-879f4b430109",
		Event:     "test.event",
		Timestamp: time.Now().UTC(),
	})

	// Direct deliver with HTTP 500 status code to cover non-2xx error path
	errServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusInternalServerError)
		_, _ = responseWriter.Write([]byte(strings.Repeat("A", 2048)))
	}))
	defer errServer.Close()
	webhookEventBus.deliver(ctx, "err-id", errServer.URL, "", 1, 1, WebhookEventEnvelope{
		ID:        "01a02ef2-ce86-722a-bffb-879f4b430109",
		Event:     "test.event",
		Timestamp: time.Now().UTC(),
	})

	// Direct deliver with pre-canceled context to cover network error sleep cancellation break (line 318)
	preCanceledCtx, preCancel := context.WithCancel(context.Background())
	preCancel()
	webhookEventBus.deliver(preCanceledCtx, "pre-canceled-id", errServer.URL, "", 2, 5, WebhookEventEnvelope{
		ID:        "01a02ef2-ce86-722a-bffb-879f4b430109",
		Event:     "test.event",
		Timestamp: time.Now().UTC(),
	})

	// Direct deliver with HTTP 500 and canceled context during retry backoff to cover non-2xx sleep cancellation break (line 348)
	canceledDeliveryCtx, deliveryCancel := context.WithCancel(context.Background())
	errCancelServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		time.AfterFunc(10*time.Millisecond, deliveryCancel)
		responseWriter.WriteHeader(http.StatusInternalServerError)
	}))
	defer errCancelServer.Close()
	webhookEventBus.deliver(canceledDeliveryCtx, "err-id-canceled", errCancelServer.URL, "", 2, 5, WebhookEventEnvelope{
		ID:        "01a02ef2-ce86-722a-bffb-879f4b430109",
		Event:     "test.event",
		Timestamp: time.Now().UTC(),
	})
}
