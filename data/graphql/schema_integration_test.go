package graphql

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"layr.sh/core"
)

func TestGraphqlSchemaIntrospectionIntegration(t *testing.T) {
	ctx := context.Background()

	// 1. Boot Postgres testcontainer
	postgresContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("layr_schema_integration"),
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

	if err := db.RunMigrations(ctx, core.SystemDatabaseMigrations); err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	// 2. Create sample relational schema in public
	setupSchemaSQL := `
		CREATE TABLE public.authors (
			id UUID PRIMARY KEY DEFAULT uuidv7(),
			name TEXT NOT NULL,
			bio TEXT
		);

		CREATE TABLE public.posts (
			id UUID PRIMARY KEY DEFAULT uuidv7(),
			author_id UUID NOT NULL REFERENCES public.authors(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			content TEXT,
			created_at TIMESTAMPTZ DEFAULT now()
		);
	`
	if _, err := db.Exec(ctx, setupSchemaSQL); err != nil {
		t.Fatalf("failed to create sample tables: %v", err)
	}

	// 3. Introspect schema without KVStore cache
	schemaIntrospector := NewSchemaIntrospector(core.NewTestKernel(db))
	if err := schemaIntrospector.Introspect(ctx, []string{"public"}); err != nil {
		t.Fatalf("failed to introspect public schema: %v", err)
	}

	authorTableInfo, authorFound := schemaIntrospector.GetTable("public", "authors")
	if !authorFound || authorTableInfo == nil {
		t.Fatal("expected public.authors table to be introspected")
	}
	if authorTableInfo.PrimaryKey != "id" {
		t.Fatalf("expected author primary key id, got: %s", authorTableInfo.PrimaryKey)
	}
	if len(authorTableInfo.Columns) < 3 {
		t.Fatalf("expected at least 3 columns on author table, got: %d", len(authorTableInfo.Columns))
	}

	postTableInfo, postFound := schemaIntrospector.GetTable("public", "posts")
	if !postFound || postTableInfo == nil {
		t.Fatal("expected public.posts table to be introspected")
	}
	if postTableInfo.PrimaryKey != "id" {
		t.Fatalf("expected post primary key id, got: %s", postTableInfo.PrimaryKey)
	}

	// Verify foreign key relationship
	foundAuthorRelation := false
	for _, relation := range postTableInfo.ForeignKeys {
		if relation.ForeignTable == "authors" && relation.LocalColumn == "author_id" {
			foundAuthorRelation = true
			break
		}
	}
	if !foundAuthorRelation {
		t.Fatal("expected postTableInfo to have relationship referencing authors")
	}

	// 4. Introspect with KVStore caching
	databaseKVStore := core.NewDatabaseKVStore(ctx, db, 0)
	defer func() { _ = databaseKVStore.Close() }()

	cachedKernel := core.NewTestKernel(db, core.WithKVStore(databaseKVStore))
	cachingSchemaIntrospector := NewSchemaIntrospector(cachedKernel)
	// Second introspection writes to KVStore cache
	if err := cachingSchemaIntrospector.Introspect(ctx, []string{"public"}); err != nil {
		t.Fatalf("failed to introspect with KVStore: %v", err)
	}

	// A new introspector with KVStore should load from cache directly
	cachedSchemaIntrospector := NewSchemaIntrospector(cachedKernel)
	if err := cachedSchemaIntrospector.Introspect(ctx, []string{"public"}); err != nil {
		t.Fatalf("failed to introspect from cache: %v", err)
	}
	if _, ok := cachedSchemaIntrospector.GetTable("public", "authors"); !ok {
		t.Fatal("expected cached introspector to have public.authors loaded")
	}

	// 5. Edge cases: Empty schemas and non-existent schemas
	if err := schemaIntrospector.Introspect(ctx, nil); err != nil {
		t.Fatalf("expected nil schemas to succeed without error: %v", err)
	}
	if err := schemaIntrospector.Introspect(ctx, []string{"non_existent_schema"}); err != nil {
		t.Fatalf("expected non-existent schema to succeed with 0 tables: %v", err)
	}

	// 6. Introspect failure with canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	canceledSchemaIntrospector := NewSchemaIntrospector(core.NewTestKernel(db))
	if err := canceledSchemaIntrospector.Introspect(canceledCtx, []string{"public"}); err == nil {
		t.Fatal("expected error introspecting with canceled context")
	}
}
