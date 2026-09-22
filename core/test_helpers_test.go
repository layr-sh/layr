package core

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCoreTestInMemoryKVDriverUnit(t *testing.T) {
	ctx := context.Background()
	testInMemoryKVDriver := NewTestInMemoryKVDriver()
	kvStore := NewInMemoryKVStore()
	if kvStore == nil {
		t.Fatal("expected non-nil KVStore")
	}

	// 1. Get non-existent
	if _, err := testInMemoryKVDriver.Get(ctx, "nonexistent"); err != ErrKVStoreKeyNotFound {
		t.Fatalf("expected ErrKVStoreKeyNotFound, got %v", err)
	}

	// 2. Set with no expiry and Get
	if setErr := testInMemoryKVDriver.Set(ctx, "k1", "v1", 0); setErr != nil {
		t.Fatalf("unexpected Set error: %v", setErr)
	}
	if retrievedValue, getErr := testInMemoryKVDriver.Get(ctx, "k1"); getErr != nil || retrievedValue != "v1" {
		t.Fatalf("expected v1, got retrievedValue=%s, err=%v", retrievedValue, getErr)
	}

	// 3. Set with short expiry and isExpired
	if setExpErr := testInMemoryKVDriver.Set(ctx, "exp1", "retrievedValue", 10*time.Millisecond); setExpErr != nil {
		t.Fatalf("unexpected Set error: %v", setExpErr)
	}
	time.Sleep(25 * time.Millisecond)
	if _, expiredGetErr := testInMemoryKVDriver.Get(ctx, "exp1"); expiredGetErr != ErrKVStoreKeyNotFound {
		t.Fatalf("expected ErrKVStoreKeyNotFound for expired key, got %v", expiredGetErr)
	}

	// 4. MSet and MGet
	if msetErr := testInMemoryKVDriver.MSet(ctx, map[string]string{"m1": "v1", "m2": "v2"}, 0); msetErr != nil {
		t.Fatalf("unexpected MSet error: %v", msetErr)
	}
	if msetExpErr := testInMemoryKVDriver.MSet(ctx, map[string]string{"exp2": "v"}, 10*time.Millisecond); msetExpErr != nil {
		t.Fatalf("unexpected MSet error: %v", msetExpErr)
	}
	time.Sleep(25 * time.Millisecond)
	mgetResults, mgetErr := testInMemoryKVDriver.MGet(ctx, []string{"m1", "m2", "exp2", "nonexistent"})
	if mgetErr != nil {
		t.Fatalf("unexpected MGet error: %v", mgetErr)
	}
	if mgetResults["m1"] != "v1" || mgetResults["m2"] != "v2" || len(mgetResults) != 2 {
		t.Fatalf("unexpected MGet results: %+v", mgetResults)
	}

	// 5. SetNX
	setNXOk, setNXErr := testInMemoryKVDriver.SetNX(ctx, "m1", "newval", 0)
	if setNXErr != nil || setNXOk {
		t.Fatalf("expected false on existing key SetNX, got ok=%v, err=%v", setNXOk, setNXErr)
	}
	if setExp3Err := testInMemoryKVDriver.Set(ctx, "exp3", "v", 10*time.Millisecond); setExp3Err != nil {
		t.Fatalf("unexpected Set error: %v", setExp3Err)
	}
	time.Sleep(25 * time.Millisecond)
	setNXOk, setNXErr = testInMemoryKVDriver.SetNX(ctx, "exp3", "fresh", 100*time.Millisecond)
	if setNXErr != nil || !setNXOk {
		t.Fatalf("expected true on expired key SetNX, got ok=%v, err=%v", setNXOk, setNXErr)
	}
	setNXNoExpOk, setNXNoExpErr := testInMemoryKVDriver.SetNX(ctx, "fresh_no_exp", "val", 0)
	if setNXNoExpErr != nil || !setNXNoExpOk {
		t.Fatalf("expected true on fresh key SetNX with no expiry, got ok=%v, err=%v", setNXNoExpOk, setNXNoExpErr)
	}

	// 6. Increment and IncrementBy
	incrementedValue, incErr := testInMemoryKVDriver.Increment(ctx, "counter", 0)
	if incErr != nil || incrementedValue != 1 {
		t.Fatalf("expected 1 from Increment, got %d, err=%v", incrementedValue, incErr)
	}
	incrementedValue, incByErr := testInMemoryKVDriver.IncrementBy(ctx, "counter", 5, 100*time.Millisecond)
	if incByErr != nil || incrementedValue != 6 {
		t.Fatalf("expected 6 from IncrementBy, got %d, err=%v", incrementedValue, incByErr)
	}
	time.Sleep(120 * time.Millisecond)
	incrementedValue, incByExpiredErr := testInMemoryKVDriver.IncrementBy(ctx, "counter", 2, 0)
	if incByExpiredErr != nil || incrementedValue != 2 {
		t.Fatalf("expected 2 from IncrementBy on expired counter, got %d, err=%v", incrementedValue, incByExpiredErr)
	}

	// 7. Expire
	if expireNonExistentErr := testInMemoryKVDriver.Expire(ctx, "nonexistent", 10*time.Second); expireNonExistentErr != ErrKVStoreKeyNotFound {
		t.Fatalf("expected ErrKVStoreKeyNotFound, got %v", expireNonExistentErr)
	}
	if setExp4Err := testInMemoryKVDriver.Set(ctx, "exp4", "retrievedValue", 10*time.Millisecond); setExp4Err != nil {
		t.Fatalf("unexpected Set error: %v", setExp4Err)
	}
	time.Sleep(25 * time.Millisecond)
	if expireExpiredErr := testInMemoryKVDriver.Expire(ctx, "exp4", 10*time.Second); expireExpiredErr != ErrKVStoreKeyNotFound {
		t.Fatalf("expected ErrKVStoreKeyNotFound on expired key, got %v", expireExpiredErr)
	}
	if setPermErr := testInMemoryKVDriver.Set(ctx, "perm", "retrievedValue", 0); setPermErr != nil {
		t.Fatalf("unexpected Set error: %v", setPermErr)
	}
	if expirePermErr := testInMemoryKVDriver.Expire(ctx, "perm", 10*time.Second); expirePermErr != nil {
		t.Fatalf("unexpected Expire error: %v", expirePermErr)
	}
	if expireZeroErr := testInMemoryKVDriver.Expire(ctx, "perm", 0); expireZeroErr != nil {
		t.Fatalf("unexpected Expire 0 error: %v", expireZeroErr)
	}

	// 8. Sweep
	if setSweepErr := testInMemoryKVDriver.Set(ctx, "to_sweep", "retrievedValue", 10*time.Millisecond); setSweepErr != nil {
		t.Fatalf("unexpected Set error: %v", setSweepErr)
	}
	time.Sleep(25 * time.Millisecond)
	purgedCount, sweepErr := testInMemoryKVDriver.Sweep(ctx)
	if sweepErr != nil || purgedCount < 1 {
		t.Fatalf("expected at least 1 purged key, got %d, err=%v", purgedCount, sweepErr)
	}

	// 9. Ping, Delete, Close
	if pingErr := testInMemoryKVDriver.Ping(ctx); pingErr != nil {
		t.Fatalf("unexpected Ping error: %v", pingErr)
	}
	if deleteErr := testInMemoryKVDriver.Delete(ctx, "perm"); deleteErr != nil {
		t.Fatalf("unexpected Delete error: %v", deleteErr)
	}
	if closeErr := testInMemoryKVDriver.Close(); closeErr != nil {
		t.Fatalf("unexpected Close error: %v", closeErr)
	}
}

func TestCoreTestKernelOptionsAndAuthHelpersUnit(t *testing.T) {
	// Fallback master key branch
	defaultKeyKernel := NewTestKernel(nil)
	if defaultKeyKernel == nil {
		t.Fatal("expected non-nil defaultKeyKernel")
	}

	config := DefaultConfig()
	config.Security.MasterEncryptionKey = TestMasterEncryptionKeyHex
	SetLoadedConfig(config)
	defer UnloadConfig()

	cryptoKeyManager, _ := NewCryptoKeyManager(TestMasterEncryptionKeyHex)
	jwtSigner := NewJWTSigner(cryptoKeyManager)
	kvStore := NewInMemoryKVStore()

	kernel := NewTestKernel(nil,
		WithDB(nil),
		WithKVStore(kvStore),
		WithCryptoKeyManager(cryptoKeyManager),
		WithJWTSigner(jwtSigner),
		WithServiceAccountManager(nil),
		WithEventBus(nil),
		WithEventManager(nil),
		WithEventHookManager(nil),
		WithNode(nil),
		WithServer(nil),
	)
	if kernel.KVStore() != kvStore {
		t.Fatal("expected configured kvStore")
	}

	// WithEventBus non-nil
	eventBus := NewEventBus(nil, cryptoKeyManager)
	WithEventBus(eventBus)(kernel)
	if kernel.eventBus != eventBus {
		t.Fatal("expected configured eventBus")
	}

	// WithTestAuthContext
	testRequest := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
	authTestRequest := WithTestAuthContext(testRequest, "usr-123", "authenticated", true)
	retrievedAuthContext := GetAuthContext(authTestRequest.Context())
	if retrievedAuthContext.UserID != "usr-123" || !retrievedAuthContext.JWT.IsAnonymous {
		t.Fatalf("unexpected retrievedAuthContext: %+v", retrievedAuthContext)
	}

	// WithTestAuthClaims
	customJWTClaims := JWTClaims{Subject: "usr-456", Role: "admin"}
	claimsTestRequest := WithTestAuthClaims(testRequest, customJWTClaims)
	claimsAuthContext := GetAuthContext(claimsTestRequest.Context())
	if claimsAuthContext.UserID != "usr-456" || claimsAuthContext.JWT.Role != "admin" {
		t.Fatalf("unexpected claimsAuthContext: %+v", claimsAuthContext)
	}
}
