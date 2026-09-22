package data

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"layr.sh/core"
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
