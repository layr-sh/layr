package data

import (
	"context"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"layr.sh/core"
	"layr.sh/data/realtime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDataBaseHandlerRealtimeIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()

	ctx := context.Background()
	service := NewService(kernel)
	_ = service.Start(ctx)
	defer func() { service.Stop() }()

	server := httptest.NewServer(http.HandlerFunc(service.BaseHandler().handleConnectRealtime))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect WebSocket
	conn, httpResponse, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	if httpResponse != nil {
		_ = httpResponse.Body.Close()
	}
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()

	// Send ping / client message
	err = conn.WriteJSON(RealtimeClientMessage{
		Action: "subscribe",
		Topic:  "public:test_table",
	})
	assert.NoError(t, err)

	// Allow connection loop to register
	time.Sleep(50 * time.Millisecond)
}

func TestDataRealtimeCDCRecreatedTableIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	ctx := context.Background()
	service := NewService(kernel)
	if err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer service.Stop()
	if _, err := kernel.DB().Exec(ctx, `CREATE TABLE public.review_stream(tenant integer, key text, payload text, PRIMARY KEY(tenant,key))`); err != nil {
		t.Fatal(err)
	}
	service.InvalidateCatalog(ctx)
	// Recreating a table must not leave the hub's previous trigger cache authoritative.
	if _, err := kernel.DB().Exec(ctx, `DROP TABLE public.review_stream; CREATE TABLE public.review_stream(tenant integer, key text, payload text, PRIMARY KEY(tenant,key))`); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(service.BaseHandler().handleConnectRealtime))
	defer server.Close()
	connection, httpResponse, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if httpResponse != nil {
		_ = httpResponse.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.Close() }()
	if err := connection.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := connection.WriteJSON(realtime.ClientMessage{Action: "subscribe", Channel: "changes", Schema: "public", Table: "review_stream", Event: "insert"}); err != nil {
		t.Fatal(err)
	}
	var acknowledgement map[string]any
	if err := connection.ReadJSON(&acknowledgement); err != nil {
		t.Fatal(err)
	}
	if acknowledgement["status"] != "subscribed" {
		t.Fatalf("subscription failed: %+v", acknowledgement)
	}
	if _, err := kernel.DB().Exec(ctx, `INSERT INTO public.review_stream VALUES(42,'custom-key',repeat('x',12000))`); err != nil {
		t.Fatal(err)
	}
	var broadcastMessage realtime.BroadcastMessage
	if err := connection.ReadJSON(&broadcastMessage); err != nil {
		t.Fatal(err)
	}
	if !broadcastMessage.Truncated || broadcastMessage.Record["tenant"] != float64(42) || broadcastMessage.Record["key"] != "custom-key" || len(broadcastMessage.Record) != 2 {
		t.Fatalf("custom composite PK was not preserved: %+v", broadcastMessage)
	}
}
