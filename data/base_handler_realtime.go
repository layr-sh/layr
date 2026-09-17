package data

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"layr.sh/core"
	"layr.sh/data/realtime"
)

// HandleRealtime upgrades HTTP connection to WebSocket and attaches to the CDC Hub.
func (handler *BaseHandler) HandleRealtime(responseWriter http.ResponseWriter, request *http.Request) {
	config := handler.configManager.Get()
	if !config.Realtime.Enabled {
		core.WriteErrorResponseProblem(responseWriter, request, http.StatusForbidden, "Real-Time API Disabled", "Real-Time API is disabled", "LAYR_DATA_003")
		return
	}

	if handler.realtimeHub == nil {
		core.WriteErrorResponseProblem(responseWriter, request, http.StatusServiceUnavailable, "Real-Time Service Unavailable", "Realtime hub is not initialized", "LAYR_DATA_005")
		return
	}

	maxConnections := config.Realtime.MaxConnections
	if maxConnections <= 0 {
		maxConnections = 10000
	}
	if handler.realtimeHub.ClientCount() >= maxConnections {
		core.WriteErrorResponseProblem(responseWriter, request, http.StatusTooManyRequests, "Connection Limit Exceeded", "Too many active realtime connections", "LAYR_DATA_004")
		return
	}

	jwtClaims := core.GetAuthContext(request.Context()).JWT
	_ = jwtClaims

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(incomingRequest *http.Request) bool {
			origin := incomingRequest.Header.Get("Origin")
			if origin == "" {
				return true
			}
			allowedOrigins := config.CORS.AllowedOrigins
			if len(allowedOrigins) == 0 {
				return true
			}
			for _, allowed := range allowedOrigins {
				if allowed == "*" || allowed == origin {
					return true
				}
			}
			return false
		},
	}

	connection, err := upgrader.Upgrade(responseWriter, request, nil)
	if err != nil {
		return
	}

	heartbeatMS := config.Realtime.HeartbeatIntervalMS
	if heartbeatMS <= 0 {
		heartbeatMS = 30000
	}
	heartbeatInterval := time.Duration(heartbeatMS) * time.Millisecond

	maxChannels := config.Realtime.MaxChannelsPerConnection
	if maxChannels <= 0 {
		maxChannels = 50
	}

	client := realtime.NewClient(handler.realtimeHub, connection, maxChannels)
	client.SetValidator(func(schema, table string) error {
		currentConfig := handler.configManager.Get()

		if schema == "core" || schema == "pg_catalog" || schema == "information_schema" {
			return fmt.Errorf("schema %q is protected and cannot be subscribed to", schema)
		}

		if len(currentConfig.Schemas) > 0 {
			allowed := false
			for _, allowedSchema := range currentConfig.Schemas {
				if allowedSchema == schema {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("schema %q is not exposed for realtime operations", schema)
			}
		}

		for _, excludedTable := range currentConfig.REST.ExcludedTables {
			if excludedTable == table || excludedTable == fmt.Sprintf("%s.%s", schema, table) {
				return fmt.Errorf("table %q is excluded from realtime operations", table)
			}
		}

		return nil
	})

	handler.realtimeHub.RegisterClient(client)

	clientCtx := context.WithoutCancel(request.Context())

	go client.WritePump(clientCtx, heartbeatInterval)
	go client.ReadPump(clientCtx, heartbeatInterval)
}
