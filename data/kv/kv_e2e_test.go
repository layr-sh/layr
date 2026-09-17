package kv

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"layr.sh/core"
)

func TestKVLifecycleE2E(t *testing.T) {
	ctx := context.Background()

	// 1. Boot Postgres testcontainer to serve as production-like environment
	postgresContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("layr_cache_e2e"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("secret"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Skipf("docker not available for e2e test: %v", err)
		return
	}
	defer func() { _ = postgresContainer.Terminate(ctx) }()

	databaseURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := core.NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create database pool: %v", err)
	}
	defer db.Close()

	if migErr := db.RunMigrations(ctx, core.SystemDatabaseMigrations); migErr != nil {
		t.Fatalf("failed to apply migrations: %v", migErr)
	}

	databaseKVStore := core.NewDatabaseKVStore(ctx, db, 100*time.Millisecond)
	defer func() { _ = databaseKVStore.Close() }()

	saltSecret := "production-e2e-salt"

	// 2. Journey 1: Anonymous Visitor querying public catalog
	// First request: Cache Miss -> compute, cache, return
	catalogQueryRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/products?select=id,title,price&price=gte.10", nil)
	catalogQueryRequest.Header.Set("X-Forwarded-For", "198.51.100.25")
	catalogQueryRequest.Header.Set("User-Agent", "MobileSafari/17.0")

	catalogAuthContext := ExtractAuthContext(catalogQueryRequest, saltSecret)
	catalogQueryKey := BuildRESTQueryKey("public", "products", []string{"id", "title", "price"}, nil, []string{"price.gte.10"}, nil, 20, 0, false, "")
	catalogInternalCacheKey := BuildInternalKey(catalogAuthContext, catalogQueryKey)

	// Verify Cache Miss
	_, err = databaseKVStore.Get(ctx, catalogInternalCacheKey)
	if !errors.Is(err, core.ErrKVStoreKeyNotFound) {
		t.Fatalf("expected initial cache miss for anonymous catalog query, got: %v", err)
	}

	// Backend produces response and stores in cache
	catalogPayload := `[{"id": 1, "title": "Smartphone", "price": 499}]`
	if setErr := databaseKVStore.Set(ctx, catalogInternalCacheKey, catalogPayload, 300*time.Second); setErr != nil {
		t.Fatalf("failed to write catalog cache entry: %v", setErr)
	}

	// Repeated request from same anonymous visitor gets Cache Hit
	cachedCatalogResponse, err := databaseKVStore.Get(ctx, catalogInternalCacheKey)
	if err != nil {
		t.Fatalf("expected cache hit on repeated query, got error: %v", err)
	}
	if cachedCatalogResponse != catalogPayload {
		t.Fatalf("expected cached catalog response %s, got: %s", catalogPayload, cachedCatalogResponse)
	}

	// 3. Journey 2: Privacy-Preserving Daily Rotation
	// 24 hours later, the visitor makes the identical request from the same IP
	yesterdayTime := time.Now().UTC().Add(-25 * time.Hour)
	yesterdayHash := ComputeVisitorHash("198.51.100.25", "MobileSafari/17.0", yesterdayTime, saltSecret)
	yesterdayAuthContext := AuthContext{
		Role:        "anon",
		VisitorHash: yesterdayHash,
	}
	yesterdayInternalKey := BuildInternalKey(yesterdayAuthContext, catalogQueryKey)

	// The current key does not match yesterday's key
	if yesterdayInternalKey == catalogInternalCacheKey {
		t.Fatal("expected daily rotation to produce different internal key across 24h intervals")
	}

	// 4. Journey 3: Authenticated User Alice querying personal data
	aliceCtx := core.WithAuthContext(ctx, core.AuthContext{
		JWT: core.JWTClaims{
			Subject: "usr_alice",
			Role:    "customer",
		},
	})
	aliceOrdersRequest := httptest.NewRequestWithContext(aliceCtx, http.MethodGet, "/api/v1/data/orders?customer_id=eq.alice", nil)

	aliceAuthContext := ExtractAuthContext(aliceOrdersRequest, saltSecret)
	ordersQueryKey := BuildRESTQueryKey("public", "orders", []string{"id", "total"}, nil, []string{"customer_id.eq.alice"}, nil, 10, 0, false, "")
	aliceInternalOrdersKey := BuildInternalKey(aliceAuthContext, ordersQueryKey)

	// Alice initial miss
	if _, err = databaseKVStore.Get(ctx, aliceInternalOrdersKey); !errors.Is(err, core.ErrKVStoreKeyNotFound) {
		t.Fatalf("expected cache miss on alice first query: %v", err)
	}

	// Cache Alice's orders
	aliceOrdersPayload := `[{"id": 101, "total": 99.50}]`
	if setErr := databaseKVStore.Set(ctx, aliceInternalOrdersKey, aliceOrdersPayload, 600*time.Second); setErr != nil {
		t.Fatalf("failed to cache alice orders: %v", setErr)
	}

	// Repeated query -> Cache Hit
	cachedOrders, err := databaseKVStore.Get(ctx, aliceInternalOrdersKey)
	if err != nil || cachedOrders != aliceOrdersPayload {
		t.Fatalf("expected cache hit for alice orders, got: %s (err: %v)", cachedOrders, err)
	}

	// 5. Journey 4: User Bob querying orders cannot access Alice's cached orders
	bobCtx := core.WithAuthContext(ctx, core.AuthContext{
		JWT: core.JWTClaims{
			Subject: "usr_bob",
			Role:    "customer",
		},
	})
	bobOrdersRequest := httptest.NewRequestWithContext(bobCtx, http.MethodGet, "/api/v1/data/orders?customer_id=eq.alice", nil)

	bobAuthContext := ExtractAuthContext(bobOrdersRequest, saltSecret)
	bobInternalOrdersKey := BuildInternalKey(bobAuthContext, ordersQueryKey)

	// Bob must get Cache Miss even though query parameters are identical
	if _, err = databaseKVStore.Get(ctx, bobInternalOrdersKey); !errors.Is(err, core.ErrKVStoreKeyNotFound) {
		t.Fatalf("security violation: bob accessed alice's cached orders: %v", err)
	}

	// 6. Journey 5: GraphQL Query Caching and Mutation Invalidation
	graphQLProfileQuery := "query GetProfile { profile { name avatar } }"
	profileQueryKey := BuildGraphQLQueryKey(graphQLProfileQuery, nil, "")
	aliceProfileInternalKey := BuildInternalKey(aliceAuthContext, profileQueryKey)

	// Profile query initial miss -> store response
	profilePayload := `{"data": {"profile": {"name": "Alice", "avatar": "alice.png"}}}`
	if setErr := databaseKVStore.Set(ctx, aliceProfileInternalKey, profilePayload, 300*time.Second); setErr != nil {
		t.Fatalf("failed to store profile cache: %v", setErr)
	}

	// Profile query hit
	cachedProfile, err := databaseKVStore.Get(ctx, aliceProfileInternalKey)
	if err != nil || cachedProfile != profilePayload {
		t.Fatalf("expected profile cache hit: %v", err)
	}

	// Mutation occurs -> User updates avatar -> Invalidate cached profile query
	if delErr := databaseKVStore.Delete(ctx, aliceProfileInternalKey); delErr != nil {
		t.Fatalf("failed to invalidate profile query: %v", delErr)
	}

	// Subsequent query must be Cache Miss, guaranteeing fresh data
	if _, err = databaseKVStore.Get(ctx, aliceProfileInternalKey); !errors.Is(err, core.ErrKVStoreKeyNotFound) {
		t.Fatalf("expected cache miss after profile invalidation, got: %v", err)
	}
}
