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

func TestKVKVStoreIntegration(t *testing.T) {
	ctx := context.Background()

	// 1. Boot Postgres testcontainer
	postgresContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("layr_cache_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("secret"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Skipf("docker not available: %v", err)
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

	// 2. Initialize KVStore backed by PostgreSQL
	databaseKVStore := core.NewDatabaseKVStore(ctx, db, 100*time.Millisecond)
	defer func() { _ = databaseKVStore.Close() }()

	// 3. Integration Scenario: Anonymous Visitor Isolation
	visitorOneRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/users", nil)
	visitorOneRequest.Header.Set("X-Forwarded-For", "198.51.100.1")
	visitorOneRequest.Header.Set("User-Agent", "VisitorOneBrowser/1.0")

	visitorTwoRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/users", nil)
	visitorTwoRequest.Header.Set("X-Forwarded-For", "203.0.113.50")
	visitorTwoRequest.Header.Set("User-Agent", "VisitorTwoBrowser/2.0")

	saltSecret := "integration-test-salt"
	visitorOneAuthContext := ExtractAuthContext(visitorOneRequest, saltSecret)
	visitorTwoAuthContext := ExtractAuthContext(visitorTwoRequest, saltSecret)

	restQueryKey := BuildRESTQueryKey("public", "users", []string{"id", "email"}, nil, []string{"is_active.eq.true"}, nil, 10, 0, false, "")
	visitorOneInternalKey := BuildInternalKey(visitorOneAuthContext, restQueryKey)
	visitorTwoInternalKey := BuildInternalKey(visitorTwoAuthContext, restQueryKey)

	if visitorOneInternalKey == visitorTwoInternalKey {
		t.Fatal("expected distinct internal cache keys for two different anonymous visitors")
	}

	// Store cached payload for Visitor 1
	cachedPayload := `[{"id": "usr_1", "email": "alice@example.com"}]`
	if setErr := databaseKVStore.Set(ctx, visitorOneInternalKey, cachedPayload, 60*time.Second); setErr != nil {
		t.Fatalf("failed to set cache entry in kvstore: %v", setErr)
	}

	// Visitor 1 gets cache hit
	retrievedPayload, err := databaseKVStore.Get(ctx, visitorOneInternalKey)
	if err != nil {
		t.Fatalf("expected cache hit for visitor 1: %v", err)
	}
	if retrievedPayload != cachedPayload {
		t.Fatalf("retrieved payload mismatch: got %s, expected %s", retrievedPayload, cachedPayload)
	}

	// Visitor 2 gets cache miss (isolation between anonymous visitors)
	_, err = databaseKVStore.Get(ctx, visitorTwoInternalKey)
	if !errors.Is(err, core.ErrKVStoreKeyNotFound) {
		t.Fatalf("expected ErrKVStoreKeyNotFound for visitor 2, got: %v", err)
	}

	// 4. Integration Scenario: Authenticated Subject and Role Isolation
	aliceEditorAuthContext := AuthContext{
		Role:    "editor",
		Subject: "usr_alice",
	}
	bobEditorAuthContext := AuthContext{
		Role:    "editor",
		Subject: "usr_bob",
	}
	aliceViewerAuthContext := AuthContext{
		Role:    "viewer",
		Subject: "usr_alice",
	}

	aliceEditorKey := BuildInternalKey(aliceEditorAuthContext, restQueryKey)
	bobEditorKey := BuildInternalKey(bobEditorAuthContext, restQueryKey)
	aliceViewerKey := BuildInternalKey(aliceViewerAuthContext, restQueryKey)

	alicePayload := `[{"secret_doc": "confidential_alice"}]`
	if setErr := databaseKVStore.Set(ctx, aliceEditorKey, alicePayload, 60*time.Second); setErr != nil {
		t.Fatalf("failed to store alice cache entry: %v", setErr)
	}

	// Bob cannot read Alice's cached query result
	if _, err = databaseKVStore.Get(ctx, bobEditorKey); !errors.Is(err, core.ErrKVStoreKeyNotFound) {
		t.Fatalf("expected cache miss for bob on alice's key, got: %v", err)
	}

	// Alice with viewer role cannot read Alice's editor cached result
	if _, err = databaseKVStore.Get(ctx, aliceViewerKey); !errors.Is(err, core.ErrKVStoreKeyNotFound) {
		t.Fatalf("expected cache miss for alice viewer role, got: %v", err)
	}

	// 5. Integration Scenario: GraphQL Query Key and Mutation Invalidation
	graphQLQueryKey := BuildGraphQLQueryKey("query GetFeed($limit: Int) { feed(limit: $limit) { id title } }", map[string]any{"limit": 20}, "")
	aliceGraphQLInternalKey := BuildInternalKey(aliceEditorAuthContext, graphQLQueryKey)
	graphQLPayload := `{"data": {"feed": [{"id": 1, "title": "News"}]}}`

	if setErr := databaseKVStore.Set(ctx, aliceGraphQLInternalKey, graphQLPayload, 60*time.Second); setErr != nil {
		t.Fatalf("failed to store graphql cache: %v", setErr)
	}

	if _, err = databaseKVStore.Get(ctx, aliceGraphQLInternalKey); err != nil {
		t.Fatalf("expected graphql cache hit: %v", err)
	}

	// Invalidation deletes key from store
	if delErr := databaseKVStore.Delete(ctx, aliceGraphQLInternalKey); delErr != nil {
		t.Fatalf("failed to invalidate cache key: %v", delErr)
	}

	if _, err = databaseKVStore.Get(ctx, aliceGraphQLInternalKey); !errors.Is(err, core.ErrKVStoreKeyNotFound) {
		t.Fatalf("expected ErrKVStoreKeyNotFound after deletion, got: %v", err)
	}

	// 6. Integration Scenario: Short TTL Expiration
	shortTTLKey := BuildInternalKey(aliceEditorAuthContext, "rest:temporary_counter")
	if setErr := databaseKVStore.Set(ctx, shortTTLKey, "42", 1*time.Second); setErr != nil {
		t.Fatalf("failed to set short TTL key: %v", setErr)
	}

	// Ensure immediate read succeeds
	if value, getErr := databaseKVStore.Get(ctx, shortTTLKey); getErr != nil || value != "42" {
		t.Fatalf("expected immediate read to succeed, got value: %s, err: %v", value, getErr)
	}

	// Sleep until TTL expires
	time.Sleep(1200 * time.Millisecond)
	if _, err = databaseKVStore.Get(ctx, shortTTLKey); !errors.Is(err, core.ErrKVStoreKeyNotFound) {
		t.Fatalf("expected expired key to return ErrKVStoreKeyNotFound, got: %v", err)
	}
}
