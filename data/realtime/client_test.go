package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestRealtimeClientDispatchAndMessageUnit(t *testing.T) {
	hub := NewHub(nil)
	hub.installedTables["public.orders"] = true

	// 1. NewClient defaults and custom channel limit
	defaultClient := NewClient(hub, nil, 0)
	if defaultClient.maxChannels != 50 {
		t.Fatalf("expected default maxChannels 50, got %d", defaultClient.maxChannels)
	}
	if defaultClient.ID() == "" {
		t.Fatal("expected non-empty client ID")
	}

	customClient := NewClient(hub, nil, 2)
	if customClient.maxChannels != 2 {
		t.Fatalf("expected custom maxChannels 2, got %d", customClient.maxChannels)
	}

	// 2. Client Close idempotency
	customClient.Close(context.Background())
	customClient.Close(context.Background())

	// 3. Dispatch filtering logic
	client := NewClient(hub, nil, 10)
	client.subscriptions["orders_pending"] = Subscription{
		Channel: "orders_pending",
		Schema:  "public",
		Table:   "orders",
		Event:   "INSERT",
		Filter:  "status=eq.pending",
	}
	client.subscriptions["orders_all"] = Subscription{
		Channel: "orders_all",
		Schema:  "public",
		Table:   "orders",
		Event:   "*",
	}
	client.subscriptions["empty_event"] = Subscription{
		Channel: "empty_event",
		Schema:  "",
		Table:   "",
		Event:   "",
	}

	// Mismatched schema
	client.Dispatch(CDCEvent{
		Schema: "private",
		Table:  "orders",
		Event:  "INSERT",
		Record: map[string]any{"status": "pending"},
	})
	// Mismatched table
	client.Dispatch(CDCEvent{
		Schema: "public",
		Table:  "products",
		Event:  "INSERT",
		Record: map[string]any{"status": "pending"},
	})
	// Mismatched event
	client.Dispatch(CDCEvent{
		Schema: "public",
		Table:  "orders",
		Event:  "DELETE",
		Record: map[string]any{"status": "pending"},
	})
	// Mismatched filter
	client.Dispatch(CDCEvent{
		Schema: "public",
		Table:  "orders",
		Event:  "INSERT",
		Record: map[string]any{"status": "completed"},
	})
	// Matching event
	client.Dispatch(CDCEvent{
		Schema:          "public",
		Table:           "orders",
		Event:           "INSERT",
		CommitTimestamp: "2026-09-03T12:00:00Z",
		Record:          map[string]any{"status": "pending"},
	})

	// 4. Backpressure drop on full sendChannel
	for range 300 {
		client.sendAck("channel", "ok")
		client.sendError("error")
		client.Dispatch(CDCEvent{
			Schema: "public",
			Table:  "orders",
			Event:  "*",
			Record: map[string]any{"status": "pending"},
		})
	}

	// 5. handleMessage unit tests
	ctx := context.Background()

	// Subscribe without channel or table
	emptyChannelClient := NewClient(hub, nil, 10)
	emptyChannelClient.handleMessage(ctx, ClientMessage{Action: "subscribe", Table: "orders"})
	emptyChannelClient.handleMessage(ctx, ClientMessage{Action: "subscribe", Channel: "ch"})

	// Topic subscriptions with various formats
	topicClient := NewClient(hub, nil, 10)
	topicClient.handleMessage(ctx, ClientMessage{Action: "subscribe", Topic: "public:orders"})
	topicClient.handleMessage(ctx, ClientMessage{Action: "subscribe", Topic: "public.orders"})
	topicClient.handleMessage(ctx, ClientMessage{Action: "subscribe", Topic: "orders"})
	topicClient.handleMessage(ctx, ClientMessage{Action: "subscribe", Channel: "public:orders"})
	topicClient.handleMessage(ctx, ClientMessage{Action: "subscribe", Channel: "public.orders"})

	// Validator rejection and acceptance
	validatingClient := NewClient(hub, nil, 10)
	validatingClient.SetValidator(func(schema, table string) error {
		if table == "restricted" {
			return fmt.Errorf("table restricted is blocked")
		}
		return nil
	})
	validatingClient.handleMessage(ctx, ClientMessage{Action: "subscribe", Channel: "ch_blocked", Table: "restricted"})
	validatingClient.handleMessage(ctx, ClientMessage{Action: "subscribe", Channel: "ch_allowed", Table: "orders"})

	// EnsureTableTrigger error propagation
	triggerErrClient := NewClient(hub, nil, 10)
	triggerErrClient.handleMessage(ctx, ClientMessage{Action: "subscribe", Channel: "invalid_trg", Table: "invalid-table!"})

	// Presence action
	presenceClient := NewClient(hub, nil, 10)
	presenceClient.handleMessage(ctx, ClientMessage{Action: "presence", Channel: "ch_presence"})
	presenceClient.handleMessage(ctx, ClientMessage{Action: "presence", Topic: "topic_presence"})
	presenceClient.handleMessage(ctx, ClientMessage{Action: "presence"})

	// Unsubscribe via topic and without channel/topic
	unsubClient := NewClient(hub, nil, 10)
	unsubClient.handleMessage(ctx, ClientMessage{Action: "subscribe", Channel: "ch_unsub", Table: "orders"})
	unsubClient.handleMessage(ctx, ClientMessage{Action: "unsubscribe", Topic: "ch_unsub"})
	unsubClient.handleMessage(ctx, ClientMessage{Action: "unsubscribe"})

	// Client Close with presence clean-up
	closingClient := NewClient(hub, nil, 10)
	closingClient.subscriptions["room_1"] = Subscription{Channel: "room_1", Table: "orders"}
	closingClient.Close(context.Background())

	// Subscribe with max channel limit exceeded
	limitedClient := NewClient(hub, nil, 1)
	limitedClient.handleMessage(ctx, ClientMessage{Action: "subscribe", Channel: "ch1", Table: "orders"})
	limitedClient.handleMessage(ctx, ClientMessage{Action: "subscribe", Channel: "ch2", Table: "orders"})

	// Unsubscribe without channel
	limitedClient.handleMessage(ctx, ClientMessage{Action: "unsubscribe"})

	// Valid unsubscribe
	limitedClient.handleMessage(ctx, ClientMessage{Action: "unsubscribe", Channel: "ch1"})

	// Ping action
	limitedClient.handleMessage(ctx, ClientMessage{Action: "ping"})

	// Unknown action
	limitedClient.handleMessage(ctx, ClientMessage{Action: "unknown_action"})

	// 6. ReadPump and WritePump unit tests via in-memory WebSocket pair
	upgrader := websocket.Upgrader{
		CheckOrigin: func(request *http.Request) bool { return true },
	}

	serverClients := make(chan *Client, 1)
	testServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		connection, err := upgrader.Upgrade(responseWriter, request, nil)
		if err != nil {
			return
		}
		pumpClient := NewClient(hub, connection, 5)
		serverClients <- pumpClient
		go pumpClient.WritePump(request.Context(), 10*time.Millisecond)
		pumpClient.ReadPump(request.Context(), 100*time.Millisecond)
	}))
	defer testServer.Close()

	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http")
	connection, dialResponse, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial in-memory websocket: %v", err)
	}
	if dialResponse != nil && dialResponse.Body != nil {
		_ = dialResponse.Body.Close()
	}
	defer func() { _ = connection.Close() }()

	serverClient := <-serverClients

	// Send Ping from client to server
	pingMessage, _ := json.Marshal(ClientMessage{Action: "ping"})
	_ = connection.WriteMessage(websocket.TextMessage, pingMessage)

	var pongResponse map[string]string
	_ = connection.ReadJSON(&pongResponse)
	if pongResponse["status"] != "pong" {
		t.Fatalf("expected pong status, got %v", pongResponse)
	}

	// Send invalid JSON from client
	_ = connection.WriteMessage(websocket.TextMessage, []byte("not valid json"))
	var errorResponse map[string]string
	_ = connection.ReadJSON(&errorResponse)
	if !strings.Contains(errorResponse["error"], "invalid JSON") {
		t.Fatalf("expected invalid JSON error, got %v", errorResponse)
	}

	// Let ticker fire ping from server WritePump
	time.Sleep(30 * time.Millisecond)

	// Send Pong back from client to server to satisfy SetPongHandler
	_ = connection.WriteControl(websocket.PongMessage, []byte("pong"), time.Now().Add(time.Second))
	_ = connection.WriteMessage(websocket.TextMessage, pingMessage)
	_ = connection.ReadJSON(&pongResponse)

	// Close connection to test ReadPump and WritePump clean termination
	_ = connection.Close()
	serverClient.Close(context.Background())

	// Test WritePump fallback on interval <= 0 and nil connection handling
	nilConnectionClient := NewClient(hub, nil, 5)
	nilConnectionClient.sendChannel <- []byte("message to nil connection")
	go nilConnectionClient.WritePump(context.Background(), 0)
	time.Sleep(10 * time.Millisecond)
	nilConnectionClient.Close(context.Background())
}
