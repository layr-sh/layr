package realtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"layr.sh/core"
	"layr.sh/data/common"
)

// CDCEvent represents a change event payload received from PostgreSQL.
type CDCEvent struct {
	Schema          string         `json:"schema"`
	Table           string         `json:"table"`
	Event           string         `json:"event"` // INSERT, UPDATE, DELETE
	CommitTimestamp string         `json:"commit_timestamp"`
	Record          map[string]any `json:"record"`
	OldRecord       map[string]any `json:"old_record,omitempty"`
	Truncated       bool           `json:"truncated,omitempty"`
}

// Subscription represents an active subscription on a client.
type Subscription struct {
	Channel string `json:"channel"`
	Schema  string `json:"schema"`
	Table   string `json:"table"`
	Event   string `json:"event"` // INSERT, UPDATE, DELETE, *
	Filter  string `json:"filter"`
}

// Hub manages WebSocket clients, subscriptions, and PostgreSQL CDC notifications.
type Hub struct {
	kernel          *core.Kernel
	clients         map[*Client]bool
	clientsRWMutex  sync.RWMutex
	installedTables map[string]bool // "schema.table" -> bool
	tablesMutex     sync.Mutex
	stopChannel     chan struct{}
	listenDone      chan struct{}
	started         bool
	stopped         bool
	ready           chan struct{}
	readyOnce       sync.Once
	startMutex      sync.Mutex
	stopOnce        sync.Once
	presenceTTL     time.Duration
	onEvent         func(CDCEvent)
	eventRWMutex    sync.RWMutex
	initialBackoff  time.Duration
	maxBackoff      time.Duration
}

// NewHub initializes a real-time CDC hub.
func NewHub(kernel *core.Kernel) *Hub {
	return &Hub{
		kernel:          kernel,
		clients:         make(map[*Client]bool),
		installedTables: make(map[string]bool),
		stopChannel:     make(chan struct{}),
		listenDone:      make(chan struct{}),
		ready:           make(chan struct{}),
		presenceTTL:     60 * time.Second,
	}
}

// SetEventHandler registers a callback invoked whenever a CDC event is broadcast.
func (hub *Hub) SetEventHandler(handler func(CDCEvent)) {
	hub.eventRWMutex.Lock()
	defer hub.eventRWMutex.Unlock()
	hub.onEvent = handler
}

// SetPresenceTTL configures the TTL for presence keys in KVStore.
func (hub *Hub) SetPresenceTTL(expiry time.Duration) {
	hub.eventRWMutex.Lock()
	defer hub.eventRWMutex.Unlock()
	if expiry > 0 {
		hub.presenceTTL = expiry
	} else {
		hub.presenceTTL = 60 * time.Second
	}
}

// SetPresence records subscriber presence in KVStore.
func (hub *Hub) SetPresence(ctx context.Context, channel, clientID string) {
	if channel != "" && clientID != "" {
		hub.eventRWMutex.RLock()
		expiry := hub.presenceTTL
		hub.eventRWMutex.RUnlock()
		_ = hub.kernel.KVStore().Set(ctx, fmt.Sprintf("data:presence:%s:%s", channel, clientID), clientID, expiry)
	}
}

// RemovePresence cleans up subscriber presence in KVStore.
func (hub *Hub) RemovePresence(ctx context.Context, channel, clientID string) {
	if channel != "" && clientID != "" {
		_ = hub.kernel.KVStore().Delete(ctx, fmt.Sprintf("data:presence:%s:%s", channel, clientID))
	}
}

// Start launches the background LISTEN cdc loop.
func (hub *Hub) Start(ctx context.Context) {
	hub.startMutex.Lock()
	if !hub.started && !hub.stopped {
		hub.started = true
		go hub.listenLoop(ctx)
	}
	hub.startMutex.Unlock()
}

// WaitReady waits until LISTEN is active so subscriptions cannot acknowledge
// before the service is capable of receiving committed change events.
func (hub *Hub) WaitReady(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait for realtime listener: %w", ctx.Err())
	case <-hub.stopChannel:
		return fmt.Errorf("realtime hub is stopped")
	case <-hub.ready:
		return nil
	}
}

// Stop terminates the hub and closes all client connections.
func (hub *Hub) Stop() {
	hub.stopOnce.Do(func() {
		hub.startMutex.Lock()
		wasStarted := hub.started
		hub.stopped = true
		close(hub.stopChannel)
		hub.startMutex.Unlock()

		if wasStarted {
			<-hub.listenDone
		}

		hub.clientsRWMutex.Lock()
		clientsList := make([]*Client, 0, len(hub.clients))
		for client := range hub.clients {
			clientsList = append(clientsList, client)
		}
		hub.clients = make(map[*Client]bool)
		hub.clientsRWMutex.Unlock()

		for _, client := range clientsList {
			client.Close(context.Background())
		}
	})
}

// RegisterClient registers an active WebSocket client.
func (hub *Hub) RegisterClient(client *Client) {
	hub.clientsRWMutex.Lock()
	hub.clients[client] = true
	hub.clientsRWMutex.Unlock()
}

// UnregisterClient removes a client.
func (hub *Hub) UnregisterClient(client *Client) {
	hub.clientsRWMutex.Lock()
	delete(hub.clients, client)
	hub.clientsRWMutex.Unlock()
}

// ClientCount returns the number of currently active WebSocket clients.
func (hub *Hub) ClientCount() int {
	hub.clientsRWMutex.RLock()
	defer hub.clientsRWMutex.RUnlock()
	return len(hub.clients)
}

// ForgetTableTrigger clears a cached installation after table DDL or before a new subscription.
func (hub *Hub) ForgetTableTrigger(schema, table string) {
	hub.tablesMutex.Lock()
	delete(hub.installedTables, schema+"."+table)
	hub.tablesMutex.Unlock()
}

// EnsureTableTrigger verifies and installs the CDC trigger on a table.
func (hub *Hub) EnsureTableTrigger(ctx context.Context, schema, table string) error {
	if schema == "" {
		schema = "public"
	}
	if !common.IsValidIdentifier(schema) || !common.IsValidIdentifier(table) {
		return fmt.Errorf("invalid schema or table name for CDC trigger")
	}
	if common.IsProtectedSchema(schema) {
		return fmt.Errorf("cannot install CDC trigger on protected schema %q", schema)
	}

	tableKey := fmt.Sprintf("%s.%s", schema, table)

	hub.tablesMutex.Lock()
	defer hub.tablesMutex.Unlock()

	if hub.installedTables[tableKey] {
		return nil
	}

	fingerprint := sha256.Sum256([]byte(schema + "." + table))
	triggerName := fmt.Sprintf("trg_cdc_%x", fingerprint[:16])
	query := fmt.Sprintf(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_trigger WHERE tgname = '%s' AND tgrelid = '"%s"."%s"'::regclass
			) THEN
				CREATE TRIGGER %s
				AFTER INSERT OR UPDATE OR DELETE ON "%s"."%s"
				FOR EACH ROW EXECUTE FUNCTION data.notify_cdc();
			END IF;
		END $$;
	`, triggerName, schema, table, triggerName, schema, table)

	if _, err := hub.kernel.DB().Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to install CDC trigger on %s.%s: %w", schema, table, err)
	}

	hub.installedTables[tableKey] = true
	return nil
}

// BroadcastEvent delivers a change event to matching client subscriptions and registered handlers.
func (hub *Hub) BroadcastEvent(cdcEvent CDCEvent) {
	hub.clientsRWMutex.RLock()
	clients := make([]*Client, 0, len(hub.clients))
	for client := range hub.clients {
		clients = append(clients, client)
	}
	hub.clientsRWMutex.RUnlock()
	for _, client := range clients {
		client.Dispatch(cdcEvent)
	}

	hub.eventRWMutex.RLock()
	eventHandler := hub.onEvent
	hub.eventRWMutex.RUnlock()
	if eventHandler != nil {
		eventHandler(cdcEvent)
	}
}

func (hub *Hub) listenLoop(ctx context.Context) {
	defer close(hub.listenDone)

	loopCtx, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-hub.stopChannel:
			cancel()
		case <-loopCtx.Done():
		}
	}()
	defer cancel()

	backoff := hub.initialBackoff
	if backoff <= 0 {
		backoff = 100 * time.Millisecond
	}
	maxBackoff := hub.maxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 5 * time.Second
	}

	for {
		select {
		case <-loopCtx.Done():
			return
		default:
		}

		pooledConn, err := hub.kernel.DB().Acquire(loopCtx)
		if err != nil || pooledConn == nil {
			select {
			case <-loopCtx.Done():
				return
			case <-time.After(backoff):
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}
		}

		backoff = 100 * time.Millisecond

		if _, err := pooledConn.Exec(loopCtx, "LISTEN cdc"); err != nil {
			pooledConn.Release()
			select {
			case <-loopCtx.Done():
				return
			case <-time.After(backoff):
				continue
			}
		}
		hub.readyOnce.Do(func() { close(hub.ready) })

		for {
			notification, err := pooledConn.Conn().WaitForNotification(loopCtx)
			if err != nil {
				unlistenCtx, unlistenCancel := context.WithTimeout(context.WithoutCancel(loopCtx), time.Second)
				_, _ = pooledConn.Exec(unlistenCtx, "UNLISTEN cdc")
				unlistenCancel()
				pooledConn.Release()
				break
			}

			if notification != nil && notification.Channel == "cdc" {
				var cdcEvent CDCEvent
				decoder := json.NewDecoder(strings.NewReader(notification.Payload))
				decoder.UseNumber()
				if err := decoder.Decode(&cdcEvent); err == nil {
					hub.BroadcastEvent(cdcEvent)
				}
			}
		}
	}
}

// MatchFilter evaluates if a record satisfies a simple filter like status=eq.pending.
func MatchFilter(record map[string]any, filter string) bool {
	if filter == "" {
		return true
	}

	filterParts := strings.Split(filter, "&")
	for _, part := range filterParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		eqIndex := strings.Index(part, "=")
		if eqIndex == -1 {
			continue
		}
		columnName := part[:eqIndex]
		clausePayload := part[eqIndex+1:]

		if unescaped, err := url.QueryUnescape(columnName); err == nil {
			columnName = unescaped
		}
		if unescaped, err := url.QueryUnescape(clausePayload); err == nil {
			clausePayload = unescaped
		}

		dotIndex := strings.Index(clausePayload, ".")
		if dotIndex == -1 {
			continue
		}
		filterOp := clausePayload[:dotIndex]
		target := clausePayload[dotIndex+1:]

		recordItem, exists := record[columnName]

		switch filterOp {
		case "is":
			switch target {
			case "null":
				if exists && recordItem != nil {
					return false
				}
			case "not.null":
				if !exists || recordItem == nil {
					return false
				}
			case "true":
				if !exists || (recordItem != true && fmt.Sprintf("%v", recordItem) != "true") {
					return false
				}
			case "false":
				if !exists || (recordItem != false && fmt.Sprintf("%v", recordItem) != "false") {
					return false
				}
			default:
				return false
			}

		case "eq":
			if !exists || recordItem == nil {
				return false
			}
			if fmt.Sprintf("%v", recordItem) != target {
				return false
			}

		case "neq":
			if !exists || recordItem == nil {
				return false
			}
			if fmt.Sprintf("%v", recordItem) == target {
				return false
			}

		case "in":
			if !exists || recordItem == nil {
				return false
			}
			trimmedTarget := strings.TrimPrefix(target, "(")
			trimmedTarget = strings.TrimSuffix(trimmedTarget, ")")
			items := strings.Split(trimmedTarget, ",")
			itemString := fmt.Sprintf("%v", recordItem)
			found := false
			for _, item := range items {
				if strings.TrimSpace(item) == itemString {
					found = true
					break
				}
			}
			if !found {
				return false
			}

		case "gt", "gte", "lt", "lte":
			if !exists || recordItem == nil {
				return false
			}
			numericValue, numErr := toFloat64(recordItem)
			targetNum, targetErr := strconv.ParseFloat(target, 64)
			if numErr == nil && targetErr == nil {
				switch filterOp {
				case "gt":
					if numericValue <= targetNum {
						return false
					}
				case "gte":
					if numericValue < targetNum {
						return false
					}
				case "lt":
					if numericValue >= targetNum {
						return false
					}
				case "lte":
					if numericValue > targetNum {
						return false
					}
				}
			} else {
				itemString := fmt.Sprintf("%v", recordItem)
				switch filterOp {
				case "gt":
					if itemString <= target {
						return false
					}
				case "gte":
					if itemString < target {
						return false
					}
				case "lt":
					if itemString >= target {
						return false
					}
				case "lte":
					if itemString > target {
						return false
					}
				}
			}

		case "like":
			if !exists || recordItem == nil {
				return false
			}
			if !matchLike(fmt.Sprintf("%v", recordItem), target, false) {
				return false
			}

		case "ilike":
			if !exists || recordItem == nil {
				return false
			}
			if !matchLike(fmt.Sprintf("%v", recordItem), target, true) {
				return false
			}

		default:
			// Unsupported operator returns false (no false positives)
			return false
		}
	}

	return true
}

func matchLike(text, pattern string, caseInsensitive bool) bool {
	pattern = strings.ReplaceAll(pattern, "*", "%")
	var regexBuilder strings.Builder
	if caseInsensitive {
		regexBuilder.WriteString("(?i)")
	}
	regexBuilder.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		char := pattern[i]
		switch char {
		case '%':
			regexBuilder.WriteString(".*")
		case '_':
			regexBuilder.WriteString(".")
		case '.', '+', '?', '^', '$', '(', ')', '[', ']', '{', '}', '|', '\\':
			regexBuilder.WriteString("\\")
			regexBuilder.WriteByte(char)
		default:
			regexBuilder.WriteByte(char)
		}
	}
	regexBuilder.WriteString("$")
	compiledRegexp := regexp.MustCompile(regexBuilder.String())
	return compiledRegexp.MatchString(text)
}

func toFloat64(value any) (float64, error) {
	switch typedValue := value.(type) {
	case float64:
		return typedValue, nil
	case float32:
		return float64(typedValue), nil
	case int:
		return float64(typedValue), nil
	case int64:
		return float64(typedValue), nil
	case int32:
		return float64(typedValue), nil
	case uint:
		return float64(typedValue), nil
	case uint64:
		return float64(typedValue), nil
	case string:
		parsedFloat, parseErr := strconv.ParseFloat(typedValue, 64)
		if parseErr != nil {
			return 0, fmt.Errorf("failed to parse float: %w", parseErr)
		}
		return parsedFloat, nil
	default:
		return 0, fmt.Errorf("not a number")
	}
}
