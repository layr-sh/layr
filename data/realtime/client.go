// Package realtime provides real-time change data capture (CDC) subscription streaming over WebSockets.
package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"uuid"
)

const (
	sendChannelBufferSize = 256
	maxMessageReadLimit   = 65536
)

// ClientMessage represents incoming subscription requests from WebSocket clients.
type ClientMessage struct {
	Action  string `json:"action"` // "subscribe", "unsubscribe", "ping", "presence"
	Topic   string `json:"topic,omitempty"`
	Channel string `json:"channel,omitempty"`
	Schema  string `json:"schema,omitempty"`
	Table   string `json:"table,omitempty"`
	Event   string `json:"event,omitempty"`
	Filter  string `json:"filter,omitempty"`
}

// BroadcastMessage represents outgoing event delivery to WebSocket clients.
type BroadcastMessage struct {
	Channel         string         `json:"channel,omitempty"`
	Topic           string         `json:"topic,omitempty"`
	Event           string         `json:"event"`
	Schema          string         `json:"schema"`
	Table           string         `json:"table"`
	CommitTimestamp string         `json:"commit_timestamp"`
	Record          map[string]any `json:"record"`
	OldRecord       map[string]any `json:"old_record,omitempty"`
	Truncated       bool           `json:"truncated,omitempty"`
}

// Client represents a connected WebSocket client session.
type Client struct {
	id                   string
	hub                  *Hub
	connection           *websocket.Conn
	sendChannel          chan []byte
	subscriptions        map[string]Subscription // channel -> Subscription
	subscriptionsRWMutex sync.RWMutex
	maxChannels          int
	closeOnce            sync.Once
	closedChannel        chan struct{}
	validator            func(schema, table string) error
}

// NewClient initializes a client session.
func NewClient(hub *Hub, connection *websocket.Conn, maxChannels int) *Client {
	if maxChannels <= 0 {
		maxChannels = 50
	}
	return &Client{
		id:            uuid.NewV7().String(),
		hub:           hub,
		connection:    connection,
		sendChannel:   make(chan []byte, sendChannelBufferSize),
		subscriptions: make(map[string]Subscription),
		maxChannels:   maxChannels,
		closedChannel: make(chan struct{}),
	}
}

// ID returns the client's unique identifier.
func (client *Client) ID() string {
	return client.id
}

// SetValidator attaches a validation callback to authorize subscriptions before installation.
func (client *Client) SetValidator(validator func(schema, table string) error) {
	client.validator = validator
}

// Close terminates the client session cleanly.
func (client *Client) Close(ctx context.Context) {
	client.closeOnce.Do(func() {
		close(client.closedChannel)
		client.hub.UnregisterClient(client)
		client.subscriptionsRWMutex.Lock()
		for channel := range client.subscriptions {
			client.hub.RemovePresence(ctx, channel, client.id)
		}
		client.subscriptions = make(map[string]Subscription)
		client.subscriptionsRWMutex.Unlock()
		if client.connection != nil {
			_ = client.connection.Close()
		}
	})
}

// Dispatch tests matching subscriptions and queues the event to sendChannel.
func (client *Client) Dispatch(cdcEvent CDCEvent) {
	client.subscriptionsRWMutex.RLock()
	defer client.subscriptionsRWMutex.RUnlock()

	for _, subscription := range client.subscriptions {
		if subscription.Schema != "" && subscription.Schema != cdcEvent.Schema {
			continue
		}
		if subscription.Table != "" && subscription.Table != cdcEvent.Table {
			continue
		}
		if subscription.Event != "*" && subscription.Event != "" && subscription.Event != cdcEvent.Event {
			continue
		}
		if !MatchFilter(cdcEvent.Record, subscription.Filter) {
			continue
		}

		broadcastMessage := BroadcastMessage{
			Channel:         subscription.Channel,
			Topic:           subscription.Channel,
			Event:           cdcEvent.Event,
			Schema:          cdcEvent.Schema,
			Table:           cdcEvent.Table,
			CommitTimestamp: cdcEvent.CommitTimestamp,
			Record:          cdcEvent.Record,
			OldRecord:       cdcEvent.OldRecord,
			Truncated:       cdcEvent.Truncated,
		}

		broadcastJSON, err := json.Marshal(broadcastMessage)
		if err == nil {
			select {
			case client.sendChannel <- broadcastJSON:
			default:
				// Backpressure dropped
			}
		}
	}
}

// ReadPump listens for client messages (subscriptions/unsubscriptions).
func (client *Client) ReadPump(ctx context.Context, heartbeatInterval time.Duration) {
	defer client.Close(ctx)

	client.connection.SetReadLimit(maxMessageReadLimit)
	_ = client.connection.SetReadDeadline(time.Now().Add(heartbeatInterval * 2))
	client.connection.SetPongHandler(func(string) error {
		_ = client.connection.SetReadDeadline(time.Now().Add(heartbeatInterval * 2))
		return nil
	})

	for {
		_, message, err := client.connection.ReadMessage()
		if err != nil {
			return
		}

		var clientMessage ClientMessage
		if err := json.Unmarshal(message, &clientMessage); err != nil {
			client.sendError("invalid JSON message")
			continue
		}

		client.handleMessage(ctx, clientMessage)
	}
}

// WritePump writes queued messages and heartbeats to the WebSocket connection.
func (client *Client) WritePump(ctx context.Context, heartbeatInterval time.Duration) {
	if heartbeatInterval <= 0 {
		heartbeatInterval = 30 * time.Second
	}

	ticker := time.NewTicker(heartbeatInterval)
	defer func() {
		ticker.Stop()
		client.Close(ctx)
	}()

	for {
		select {
		case <-client.closedChannel:
			return

		case message := <-client.sendChannel:
			if client.connection != nil {
				_ = client.connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
				_ = client.connection.WriteMessage(websocket.TextMessage, message)
			}

		case <-ticker.C:
			if client.connection != nil {
				_ = client.connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
				_ = client.connection.WriteMessage(websocket.PingMessage, nil)
			}
		}
	}
}

func (client *Client) handleMessage(ctx context.Context, clientMessage ClientMessage) {
	switch clientMessage.Action {
	case "subscribe":
		// Normalize Channel and Topic
		if clientMessage.Channel == "" && clientMessage.Topic != "" {
			clientMessage.Channel = clientMessage.Topic
		}

		// If Table is empty, derive from Topic (or Channel if it contains ':' or '.')
		if clientMessage.Table == "" {
			source := clientMessage.Topic
			if source == "" && (strings.Contains(clientMessage.Channel, ":") || strings.Contains(clientMessage.Channel, ".")) {
				source = clientMessage.Channel
			}
			if source != "" {
				if strings.Contains(source, ":") {
					parts := strings.SplitN(source, ":", 2)
					if clientMessage.Schema == "" {
						clientMessage.Schema = parts[0]
					}
					clientMessage.Table = parts[1]
				} else if strings.Contains(source, ".") {
					parts := strings.SplitN(source, ".", 2)
					if clientMessage.Schema == "" {
						clientMessage.Schema = parts[0]
					}
					clientMessage.Table = parts[1]
				} else {
					clientMessage.Table = source
				}
			}
		}
		if clientMessage.Schema == "" {
			clientMessage.Schema = "public"
		}

		if clientMessage.Channel == "" || clientMessage.Table == "" {
			client.sendError("subscribe requires channel and table")
			return
		}

		// Validate schema and table through authorizer if present
		if client.validator != nil {
			if err := client.validator(clientMessage.Schema, clientMessage.Table); err != nil {
				client.sendError(err.Error())
				return
			}
		}

		client.subscriptionsRWMutex.Lock()
		if len(client.subscriptions) >= client.maxChannels {
			client.subscriptionsRWMutex.Unlock()
			client.sendError(fmt.Sprintf("maximum subscription limit reached (%d)", client.maxChannels))
			return
		}

		client.subscriptions[clientMessage.Channel] = Subscription{
			Channel: clientMessage.Channel,
			Schema:  clientMessage.Schema,
			Table:   clientMessage.Table,
			Event:   clientMessage.Event,
			Filter:  clientMessage.Filter,
		}
		client.subscriptionsRWMutex.Unlock()

		// Ensure table trigger exists and propagate error
		if err := client.hub.EnsureTableTrigger(ctx, clientMessage.Schema, clientMessage.Table); err != nil {
			client.subscriptionsRWMutex.Lock()
			delete(client.subscriptions, clientMessage.Channel)
			client.subscriptionsRWMutex.Unlock()
			client.sendError(fmt.Sprintf("failed to setup realtime trigger: %v", err))
			return
		}

		client.hub.SetPresence(ctx, clientMessage.Channel, client.id)
		client.sendAck(clientMessage.Channel, "subscribed")

	case "unsubscribe":
		channel := clientMessage.Channel
		if channel == "" {
			channel = clientMessage.Topic
		}
		if channel == "" {
			client.sendError("unsubscribe requires channel")
			return
		}
		client.subscriptionsRWMutex.Lock()
		delete(client.subscriptions, channel)
		client.subscriptionsRWMutex.Unlock()
		client.hub.RemovePresence(ctx, channel, client.id)
		client.sendAck(channel, "unsubscribed")

	case "presence":
		channel := clientMessage.Channel
		if channel == "" {
			channel = clientMessage.Topic
		}
		if channel == "" {
			client.sendError("presence requires channel")
			return
		}
		client.hub.SetPresence(ctx, channel, client.id)
		client.sendAck(channel, "presence_ok")

	case "ping":
		client.sendAck("", "pong")

	default:
		client.sendError(fmt.Sprintf("unknown action: %s", clientMessage.Action))
	}
}

func (client *Client) sendAck(channel, status string) {
	response := map[string]string{
		"channel": channel,
		"topic":   channel,
		"status":  status,
	}
	responseJSON, _ := json.Marshal(response)
	select {
	case client.sendChannel <- responseJSON:
	default:
	}
}

func (client *Client) sendError(detail string) {
	response := map[string]string{
		"error":  detail,
		"status": "error",
	}
	errorJSON, _ := json.Marshal(response)
	select {
	case client.sendChannel <- errorJSON:
	default:
	}
}
