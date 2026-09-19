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
	"layr.sh/data/realtime"
)

func TestDataBaseHandlerRealtimeUnit(t *testing.T) {
	configManager := NewConfigManager(nil)
	baseHandler := NewBaseHandler(nil, configManager)

	t.Run("DisabledRealtime", func(t *testing.T) {
		config := configManager.Get()
		config.Realtime.Enabled = false
		configManager.SetMemoryConfig(config)
		defer func() {
			config.Realtime.Enabled = true
			configManager.SetMemoryConfig(config)
		}()

		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/realtime", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.HandleRealtime(responseRecorder, request)
		assert.Equal(t, http.StatusForbidden, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "Access denied")
	})

	t.Run("InvalidWebSocketUpgrade", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/realtime", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.HandleRealtime(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	})

	t.Run("ValidWebSocketConnectionAndOriginChecks", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(baseHandler.HandleRealtime))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

		// 1. Connect without origin
		conn, httpResponse, err := websocket.DefaultDialer.Dial(wsURL, nil)
		assert.NoError(t, err)
		if httpResponse != nil {
			_ = httpResponse.Body.Close()
		}
		if conn != nil {
			_ = conn.Close()
		}

		// 2. Connect with allowed wildcard origin
		header := make(http.Header)
		header.Set("Origin", "http://example.com")
		conn, httpResponse, err = websocket.DefaultDialer.Dial(wsURL, header)
		assert.NoError(t, err)
		if httpResponse != nil {
			_ = httpResponse.Body.Close()
		}
		if conn != nil {
			_ = conn.Close()
		}

		// 3. Connect with restricted allowed origins
		config := configManager.Get()
		config.CORS.AllowedOrigins = []string{"http://allowed.com"}
		configManager.SetMemoryConfig(config)
		defer func() {
			config.CORS.AllowedOrigins = []string{"*"}
			configManager.SetMemoryConfig(config)
		}()

		// Disallowed origin
		badHeader := make(http.Header)
		badHeader.Set("Origin", "http://disallowed.com")
		conn, httpResponse, err = websocket.DefaultDialer.Dial(wsURL, badHeader)
		assert.Error(t, err)
		if httpResponse != nil {
			_ = httpResponse.Body.Close()
		}
		if conn != nil {
			_ = conn.Close()
		}

		// Allowed specific origin
		goodHeader := make(http.Header)
		goodHeader.Set("Origin", "http://allowed.com")
		conn, httpResponse, err = websocket.DefaultDialer.Dial(wsURL, goodHeader)
		assert.NoError(t, err)
		if httpResponse != nil {
			_ = httpResponse.Body.Close()
		}
		if conn != nil {
			_ = conn.Close()
		}

		// Empty allowed origins list defaults to true
		config.CORS.AllowedOrigins = []string{}
		configManager.SetMemoryConfig(config)
		conn, httpResponse, err = websocket.DefaultDialer.Dial(wsURL, badHeader)
		assert.NoError(t, err)
		if httpResponse != nil {
			_ = httpResponse.Body.Close()
		}
		if conn != nil {
			_ = conn.Close()
		}
	})

	t.Run("FallbacksForHeartbeatAndChannels", func(t *testing.T) {
		config := configManager.Get()
		config.Realtime.HeartbeatIntervalMS = -1
		config.Realtime.MaxChannelsPerConnection = -1
		config.Realtime.MaxConnections = -1
		configManager.SetMemoryConfig(config)
		defer func() {
			config.Realtime.HeartbeatIntervalMS = 30000
			config.Realtime.MaxChannelsPerConnection = 50
			config.Realtime.MaxConnections = 10000
			configManager.SetMemoryConfig(config)
		}()

		server := httptest.NewServer(http.HandlerFunc(baseHandler.HandleRealtime))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, httpResponse, err := websocket.DefaultDialer.Dial(wsURL, nil)
		assert.NoError(t, err)
		if httpResponse != nil {
			_ = httpResponse.Body.Close()
		}
		if conn != nil {
			time.Sleep(10 * time.Millisecond)
			_ = conn.Close()
		}
	})

	t.Run("NilRealtimeHub", func(t *testing.T) {
		nilHubBaseHandler := &BaseHandler{
			configManager: configManager,
			realtimeHub:   nil,
		}
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/realtime", nil)
		responseRecorder := httptest.NewRecorder()
		nilHubBaseHandler.HandleRealtime(responseRecorder, request)
		assert.Equal(t, http.StatusServiceUnavailable, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "Service temporarily unavailable")
	})

	t.Run("MaxConnectionsThrottling", func(t *testing.T) {
		config := configManager.Get()
		config.Realtime.MaxConnections = 1
		configManager.SetMemoryConfig(config)
		defer func() {
			config.Realtime.MaxConnections = 10000
			configManager.SetMemoryConfig(config)
		}()

		dummyClient := realtime.NewClient(baseHandler.realtimeHub, nil, 10)
		baseHandler.realtimeHub.RegisterClient(dummyClient)
		defer baseHandler.realtimeHub.UnregisterClient(dummyClient)

		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/realtime", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.HandleRealtime(responseRecorder, request)
		assert.Equal(t, http.StatusTooManyRequests, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "Too many requests. Please try again later.")
	})

	t.Run("ValidatorAccessControlViaWebSocket", func(t *testing.T) {
		config := configManager.Get()
		config.Schemas = []string{"public"}
		config.REST.ExcludedTables = []string{"secrets", "public.hidden"}
		configManager.SetMemoryConfig(config)
		defer func() {
			config.Schemas = []string{"public"}
			config.REST.ExcludedTables = []string{"schema_migrations", "secrets"}
			configManager.SetMemoryConfig(config)
		}()

		server := httptest.NewServer(http.HandlerFunc(baseHandler.HandleRealtime))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
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

		// 1. Protected schema
		_ = conn.WriteJSON(RealtimeClientMessage{Action: "subscribe", Schema: "core", Table: "users", Channel: "ch_core"})
		var errMsg1 map[string]string
		_ = conn.ReadJSON(&errMsg1)
		assert.Contains(t, errMsg1["error"], "protected")

		// 2. Disallowed schema
		_ = conn.WriteJSON(RealtimeClientMessage{Action: "subscribe", Schema: "private", Table: "orders", Channel: "ch_priv"})
		var errMsg2 map[string]string
		_ = conn.ReadJSON(&errMsg2)
		assert.Contains(t, errMsg2["error"], "not exposed")

		// 3. Excluded table
		_ = conn.WriteJSON(RealtimeClientMessage{Action: "subscribe", Schema: "public", Table: "secrets", Channel: "ch_sec"})
		var errMsg3 map[string]string
		_ = conn.ReadJSON(&errMsg3)
		assert.Contains(t, errMsg3["error"], "excluded")

		// 4. Excluded table with schema.table format
		_ = conn.WriteJSON(RealtimeClientMessage{Action: "subscribe", Schema: "public", Table: "hidden", Channel: "ch_hid"})
		var errMsg4 map[string]string
		_ = conn.ReadJSON(&errMsg4)
		assert.Contains(t, errMsg4["error"], "excluded")
	})
}
