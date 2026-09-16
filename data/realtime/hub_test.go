package realtime

import (
	"context"
	"testing"
	"time"
)

type unitMockKV struct {
	data map[string]string
}

func (mock *unitMockKV) Get(ctx context.Context, key string) (string, error) {
	return mock.data[key], nil
}
func (mock *unitMockKV) MGet(ctx context.Context, keys []string) (map[string]string, error) {
	return mock.data, nil
}
func (mock *unitMockKV) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	mock.data[key] = value
	return nil
}
func (mock *unitMockKV) MSet(ctx context.Context, entries map[string]string, expiry time.Duration) error {
	for k, v := range entries {
		mock.data[k] = v
	}
	return nil
}
func (mock *unitMockKV) SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error) {
	return true, nil
}
func (mock *unitMockKV) Delete(ctx context.Context, key string) error {
	delete(mock.data, key)
	return nil
}
func (mock *unitMockKV) Increment(ctx context.Context, key string, expiry time.Duration) (int64, error) {
	return 1, nil
}
func (mock *unitMockKV) IncrementBy(ctx context.Context, key string, delta int64, expiry time.Duration) (int64, error) {
	return delta, nil
}
func (mock *unitMockKV) Expire(ctx context.Context, key string, expiry time.Duration) error {
	return nil
}
func (mock *unitMockKV) Ping(ctx context.Context) error {
	return nil
}
func (mock *unitMockKV) Close() error {
	return nil
}

func TestRealtimeHubLifecycleAndFilteringUnit(t *testing.T) {
	ctx := context.Background()

	// 1. NewHub initialization
	hub := NewHub(nil)
	if hub == nil {
		t.Fatal("expected non-nil Hub")
	}
	if hub.presenceTTL != 60*time.Second {
		t.Fatalf("expected default presenceTTL 60s, got %v", hub.presenceTTL)
	}

	// 1b. Start with nil db
	if err := hub.Start(ctx); err == nil {
		t.Fatal("expected error starting hub with nil db")
	}

	// 2. SetPresenceTTL custom and fallback
	hub.SetPresenceTTL(120 * time.Second)
	if hub.presenceTTL != 120*time.Second {
		t.Fatalf("expected 120s presenceTTL, got %v", hub.presenceTTL)
	}
	hub.SetPresenceTTL(0)
	if hub.presenceTTL != 60*time.Second {
		t.Fatalf("expected 60s fallback presenceTTL, got %v", hub.presenceTTL)
	}

	// 3. SetPresence and RemovePresence with nil and mock store
	// nil store early return
	hub.SetPresence(ctx, "room_1", "client_1")
	hub.RemovePresence(ctx, "room_1", "client_1")

	// attach mock KV store
	mockKV := &unitMockKV{data: make(map[string]string)}
	hub.SetKVStore(mockKV)

	// empty inputs early return
	hub.SetPresence(ctx, "", "client_1")
	hub.SetPresence(ctx, "room_1", "")
	hub.RemovePresence(ctx, "", "client_1")
	hub.RemovePresence(ctx, "room_1", "")

	// valid set presence
	hub.SetPresence(ctx, "room_1", "client_1")
	if mockKV.data["data:presence:room_1:client_1"] != "client_1" {
		t.Fatalf("expected presence recorded in store, got %v", mockKV.data)
	}

	// valid remove presence
	hub.RemovePresence(ctx, "room_1", "client_1")
	if _, exists := mockKV.data["data:presence:room_1:client_1"]; exists {
		t.Fatal("expected presence removed from store")
	}

	// 4. RegisterClient, UnregisterClient, BroadcastEvent
	client := NewClient(hub, nil, 10)
	hub.RegisterClient(client)

	hub.clientsRWMutex.RLock()
	clientRegistered := hub.clients[client]
	hub.clientsRWMutex.RUnlock()
	if !clientRegistered {
		t.Fatal("expected client to be registered on hub")
	}

	var receivedCDCEvent CDCEvent
	hub.SetEventHandler(func(incomingCDCEvent CDCEvent) {
		receivedCDCEvent = incomingCDCEvent
	})

	hub.BroadcastEvent(CDCEvent{
		Schema: "public",
		Table:  "orders",
		Event:  "INSERT",
		Record: map[string]any{"id": "1"},
	})

	if receivedCDCEvent.Table != "orders" {
		t.Fatalf("expected registered eventHandler to receive event, got: %+v", receivedCDCEvent)
	}

	if hub.ClientCount() != 1 {
		t.Fatalf("expected ClientCount 1, got %d", hub.ClientCount())
	}

	hub.UnregisterClient(client)
	hub.clientsRWMutex.RLock()
	clientStillPresent := hub.clients[client]
	hub.clientsRWMutex.RUnlock()
	if clientStillPresent {
		t.Fatal("expected client to be unregistered from hub")
	}
	if hub.ClientCount() != 0 {
		t.Fatalf("expected ClientCount 0, got %d", hub.ClientCount())
	}

	// 5. EnsureTableTrigger identifier checks, protected schemas, and cached hit
	if err := hub.EnsureTableTrigger(ctx, "invalid-schema!", "orders"); err == nil {
		t.Fatal("expected error for invalid schema identifier")
	}
	if err := hub.EnsureTableTrigger(ctx, "public", "invalid-table!"); err == nil {
		t.Fatal("expected error for invalid table identifier")
	}
	if err := hub.EnsureTableTrigger(ctx, "core", "users"); err == nil {
		t.Fatal("expected error for protected schema core")
	}
	if err := hub.EnsureTableTrigger(ctx, "pg_catalog", "pg_class"); err == nil {
		t.Fatal("expected error for protected schema pg_catalog")
	}
	if err := hub.EnsureTableTrigger(ctx, "information_schema", "tables"); err == nil {
		t.Fatal("expected error for protected schema information_schema")
	}

	hub.installedTables["public.cached_orders"] = true
	if err := hub.EnsureTableTrigger(ctx, "public", "cached_orders"); err != nil {
		t.Fatalf("expected nil error on cached table, got: %v", err)
	}

	// 6. Stop on unstarted hub
	unstartedHub := NewHub(nil)
	unstartedHub.Stop()
	unstartedHub.Stop() // idempotent

	// 7. MatchFilter comprehensive edge cases
	record := map[string]any{
		"status":    "pending",
		"org_id":    "org_1",
		"count":     5,
		"f32":       float32(10.5),
		"f64":       float64(20.5),
		"i64":       int64(30),
		"i32":       int32(40),
		"u":         uint(50),
		"u64":       uint64(60),
		"num_str":   "70.5",
		"text":      "Hello World",
		"null_col":  nil,
		"is_active": true,
		"is_banned": false,
		"str_tag":   "beta",
	}

	// empty filter matches everything
	if !MatchFilter(record, "") {
		t.Fatal("empty filter should return true")
	}

	// invalid tokens without = or . are skipped
	if !MatchFilter(record, "malformed_token") {
		t.Fatal("token without '=' should be skipped")
	}
	if !MatchFilter(record, "   & &") {
		t.Fatal("empty whitespace token should be skipped")
	}
	if !MatchFilter(record, "status=no_dot_filter") {
		t.Fatal("token without '.' should be skipped")
	}

	// missing column returns false
	if MatchFilter(record, "non_existent_column=eq.value") {
		t.Fatal("missing column should return false")
	}

	// URL unescaping in column and value
	if !MatchFilter(record, "org%5Fid=eq.org%5F1") {
		t.Fatal("URL-escaped column and value should match")
	}

	// eq filterOp
	if !MatchFilter(record, "status=eq.pending") {
		t.Fatal("status=eq.pending should return true")
	}
	if MatchFilter(record, "status=eq.completed") {
		t.Fatal("status=eq.completed should return false")
	}
	if MatchFilter(record, "null_col=eq.null") {
		t.Fatal("nil value in eq should return false")
	}

	// neq filterOp
	if !MatchFilter(record, "status=neq.completed") {
		t.Fatal("status=neq.completed should return true")
	}
	if MatchFilter(record, "status=neq.pending") {
		t.Fatal("status=neq.pending should return false")
	}
	if MatchFilter(record, "null_col=neq.something") {
		t.Fatal("nil value in neq should return false")
	}

	// in filterOp
	if !MatchFilter(record, "status=in.(pending,done)") {
		t.Fatal("status=in.(pending,done) should return true")
	}
	if MatchFilter(record, "status=in.(done,cancelled)") {
		t.Fatal("status=in.(done,cancelled) should return false")
	}
	if MatchFilter(record, "null_col=in.(a,b)") {
		t.Fatal("nil value in in should return false")
	}

	// is filterOp (null, not.null, true, false, unknown)
	if !MatchFilter(record, "null_col=is.null") {
		t.Fatal("null_col=is.null should return true")
	}
	if !MatchFilter(record, "non_existent=is.null") {
		t.Fatal("non_existent=is.null should return true")
	}
	if MatchFilter(record, "status=is.null") {
		t.Fatal("non-null column with is.null should return false")
	}
	if !MatchFilter(record, "status=is.not.null") {
		t.Fatal("status=is.not.null should return true")
	}
	if MatchFilter(record, "null_col=is.not.null") {
		t.Fatal("null_col=is.not.null should return false")
	}
	if MatchFilter(record, "non_existent=is.not.null") {
		t.Fatal("non_existent=is.not.null should return false")
	}
	if !MatchFilter(record, "is_active=is.true") {
		t.Fatal("is_active=is.true should return true")
	}
	if MatchFilter(record, "is_banned=is.true") {
		t.Fatal("is_banned=is.true should return false")
	}
	if MatchFilter(record, "non_existent=is.true") {
		t.Fatal("non_existent=is.true should return false")
	}
	if !MatchFilter(record, "is_banned=is.false") {
		t.Fatal("is_banned=is.false should return true")
	}
	if MatchFilter(record, "is_active=is.false") {
		t.Fatal("is_active=is.false should return false")
	}
	if MatchFilter(record, "non_existent=is.false") {
		t.Fatal("non_existent=is.false should return false")
	}
	if MatchFilter(record, "status=is.unknown") {
		t.Fatal("unknown is target should return false")
	}

	// numeric comparisons: gt, gte, lt, lte
	if !MatchFilter(record, "count=gt.3") || MatchFilter(record, "count=gt.5") || MatchFilter(record, "count=gt.10") {
		t.Fatal("gt numeric comparison failure")
	}
	if !MatchFilter(record, "count=gte.5") || !MatchFilter(record, "count=gte.3") || MatchFilter(record, "count=gte.6") {
		t.Fatal("gte numeric comparison failure")
	}
	if !MatchFilter(record, "count=lt.10") || MatchFilter(record, "count=lt.5") || MatchFilter(record, "count=lt.2") {
		t.Fatal("lt numeric comparison failure")
	}
	if !MatchFilter(record, "count=lte.5") || !MatchFilter(record, "count=lte.10") || MatchFilter(record, "count=lte.4") {
		t.Fatal("lte numeric comparison failure")
	}
	if MatchFilter(record, "null_col=gt.0") {
		t.Fatal("null value in gt should return false")
	}

	// string comparisons for gt, gte, lt, lte
	if !MatchFilter(record, "str_tag=gt.alpha") || MatchFilter(record, "str_tag=gt.gamma") {
		t.Fatal("gt string comparison failure")
	}
	if !MatchFilter(record, "str_tag=gte.beta") || MatchFilter(record, "str_tag=gte.delta") {
		t.Fatal("gte string comparison failure")
	}
	if !MatchFilter(record, "str_tag=lt.delta") || MatchFilter(record, "str_tag=lt.alpha") {
		t.Fatal("lt string comparison failure")
	}
	if !MatchFilter(record, "str_tag=lte.beta") || MatchFilter(record, "str_tag=lte.alpha") {
		t.Fatal("lte string comparison failure")
	}

	// like and ilike filterOp
	if !MatchFilter(record, "text=like.Hello%") || MatchFilter(record, "text=like.hello%") {
		t.Fatal("like comparison failure")
	}
	if !MatchFilter(record, "text=ilike.hello*") || !MatchFilter(record, "text=ilike.*world") || MatchFilter(record, "text=ilike.foo%") {
		t.Fatal("ilike comparison failure")
	}
	if MatchFilter(record, "null_col=like.%") || MatchFilter(record, "null_col=ilike.%") {
		t.Fatal("null col with like/ilike should return false")
	}

	// unknown operator returns false
	if MatchFilter(record, "status=unknown_operator.pending") {
		t.Fatal("unknown operator should return false")
	}

	// compound filter (&)
	if !MatchFilter(record, "status=eq.pending&org_id=eq.org_1") {
		t.Fatal("compound filter should return true when all match")
	}
	if MatchFilter(record, "status=eq.pending&org_id=eq.org_2") {
		t.Fatal("compound filter should return false when one mismatches")
	}

	// 8. toFloat64 unit tests for all branches
	typesRecord := map[string]any{
		"f64": float64(1.0),
		"f32": float32(2.0),
		"i":   int(3),
		"i64": int64(4),
		"i32": int32(5),
		"u":   uint(6),
		"u64": uint64(7),
		"s":   "8.5",
		"bad": struct{}{},
	}
	for _, key := range []string{"f64", "f32", "i", "i64", "i32", "u", "u64", "s"} {
		if _, err := toFloat64(typesRecord[key]); err != nil {
			t.Fatalf("expected toFloat64 success for %s, got: %v", key, err)
		}
	}
	if _, err := toFloat64(typesRecord["bad"]); err == nil {
		t.Fatal("expected toFloat64 error for non-number")
	}

	// 9. matchLike with special regex chars
	if !matchLike("a.b+c?d", "a.b+c?d", false) {
		t.Fatal("expected matchLike with special regex characters to match")
	}
	if !matchLike("a_b", "a_b", false) {
		t.Fatal("expected matchLike with underscore wildcard to match")
	}
}
