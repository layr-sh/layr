package common

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"layr.sh/core"
)

func TestCommonRLSSecurityPolicyE2E(t *testing.T) {
	ctx := context.Background()

	// 1. Boot Postgres testcontainer to serve as production-like environment
	postgresContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("layr_common_e2e"),
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

	// 2. Set up RLS protected table and non-superuser role
	// Standard PostgreSQL superuser bypasses RLS by default, so we create an unprivileged app_user
	setupSQL := `
		CREATE ROLE app_user WITH LOGIN PASSWORD 'app_secret';
		GRANT CONNECT ON DATABASE layr_common_e2e TO app_user;
		GRANT USAGE ON SCHEMA public TO app_user;

		CREATE TABLE public.documents (
			id UUID PRIMARY KEY DEFAULT uuidv7(),
			owner TEXT NOT NULL,
			tenant TEXT NOT NULL,
			title TEXT NOT NULL
		);

		ALTER TABLE public.documents ENABLE ROW LEVEL SECURITY;

		CREATE POLICY tenant_isolation_policy ON public.documents
			FOR ALL
			TO app_user
			USING (
				owner = current_setting('request.jwt.sub', true)
				AND
				tenant = current_setting('request.jwt.tenant_id', true)
			);

		GRANT SELECT, INSERT, UPDATE, DELETE ON public.documents TO app_user;

		INSERT INTO public.documents (owner, tenant, title) VALUES
			('alice', 'tenant_alpha', 'Alice Confidential Alpha'),
			('bob', 'tenant_beta', 'Bob Private Beta');
	`
	if _, execErr := db.Exec(ctx, setupSQL); execErr != nil {
		t.Fatalf("failed to setup RLS table and role: %v", execErr)
	}

	// Connect as unprivileged app_user to strictly enforce RLS policies
	appUserURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable", "user=app_user", "password=app_secret")
	if err != nil {
		t.Fatalf("failed to get app_user connection string: %v", err)
	}

	appDB, err := core.NewDatabasePool(ctx, appUserURL)
	if err != nil {
		t.Fatalf("failed to create app_user database pool: %v", err)
	}
	defer appDB.Close()

	// 3. User Journey 1: Alice sends HTTP request with JWT headers
	aliceRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/documents", nil)
	aliceRequest.Header.Set("X-JWT-Sub", "alice")
	aliceRequest.Header.Set("X-JWT-Role", "authenticated")
	aliceRequest.Header.Set("X-JWT-Claim-Tenant_Id", "tenant_alpha")

	aliceAuthClaims := ExtractClaims(aliceRequest)
	if aliceAuthClaims.Subject != "alice" || aliceAuthClaims.Claims["tenant_id"] != "tenant_alpha" {
		t.Fatalf("unexpected alice claims: %+v", aliceAuthClaims)
	}

	aliceTx, err := appDB.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin alice transaction: %v", err)
	}
	defer func() { _ = aliceTx.Rollback(ctx) }()

	ApplyRLS(ctx, aliceTx, aliceAuthClaims)

	aliceRows, err := aliceTx.Query(ctx, "SELECT title FROM public.documents")
	if err != nil {
		t.Fatalf("failed to query documents for alice: %v", err)
	}
	defer aliceRows.Close()

	var aliceTitles []string
	for aliceRows.Next() {
		var title string
		if scanErr := aliceRows.Scan(&title); scanErr != nil {
			t.Fatalf("failed to scan alice row: %v", scanErr)
		}
		aliceTitles = append(aliceTitles, title)
	}

	if len(aliceTitles) != 1 || aliceTitles[0] != "Alice Confidential Alpha" {
		t.Fatalf("expected alice to see only Alice Confidential Alpha, got: %v", aliceTitles)
	}

	// 4. User Journey 2: Bob sends HTTP request with JWT headers
	bobRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/documents", nil)
	bobRequest.Header.Set("X-JWT-Sub", "bob")
	bobRequest.Header.Set("X-JWT-Role", "authenticated")
	bobRequest.Header.Set("X-JWT-Claim-Tenant_Id", "tenant_beta")

	bobAuthClaims := ExtractClaims(bobRequest)
	bobTx, err := appDB.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin bob transaction: %v", err)
	}
	defer func() { _ = bobTx.Rollback(ctx) }()

	ApplyRLS(ctx, bobTx, bobAuthClaims)

	bobRows, err := bobTx.Query(ctx, "SELECT title FROM public.documents")
	if err != nil {
		t.Fatalf("failed to query documents for bob: %v", err)
	}
	defer bobRows.Close()

	var bobTitles []string
	for bobRows.Next() {
		var title string
		if scanErr := bobRows.Scan(&title); scanErr != nil {
			t.Fatalf("failed to scan bob row: %v", scanErr)
		}
		bobTitles = append(bobTitles, title)
	}

	if len(bobTitles) != 1 || bobTitles[0] != "Bob Private Beta" {
		t.Fatalf("expected bob to see only Bob Private Beta, got: %v", bobTitles)
	}

	// 5. User Journey 3: Charlie sends request for tenant_alpha but wrong subject
	charlieRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/documents", nil)
	charlieRequest.Header.Set("X-JWT-Sub", "charlie")
	charlieRequest.Header.Set("X-JWT-Role", "authenticated")
	charlieRequest.Header.Set("X-JWT-Claim-Tenant_Id", "tenant_alpha")

	charlieAuthClaims := ExtractClaims(charlieRequest)
	charlieTx, err := appDB.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin charlie transaction: %v", err)
	}
	defer func() { _ = charlieTx.Rollback(ctx) }()

	ApplyRLS(ctx, charlieTx, charlieAuthClaims)

	charlieRows, err := charlieTx.Query(ctx, "SELECT title FROM public.documents")
	if err != nil {
		t.Fatalf("failed to query documents for charlie: %v", err)
	}
	defer charlieRows.Close()

	var charlieTitles []string
	for charlieRows.Next() {
		var title string
		if scanErr := charlieRows.Scan(&title); scanErr != nil {
			t.Fatalf("failed to scan charlie row: %v", scanErr)
		}
		charlieTitles = append(charlieTitles, title)
	}

	if len(charlieTitles) != 0 {
		t.Fatalf("expected charlie to see 0 documents due to RLS, got: %v", charlieTitles)
	}

	// 6. Security Journey: Malicious header injection attempt
	maliciousRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/documents", nil)
	maliciousRequest.Header.Set("X-JWT-Sub", "alice")
	maliciousRequest.Header.Set("X-JWT-Claim-Evil'; DROP TABLE public.documents; --", "malicious")

	maliciousAuthClaims := ExtractClaims(maliciousRequest)
	maliciousTx, err := appDB.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin malicious transaction: %v", err)
	}
	defer func() { _ = maliciousTx.Rollback(ctx) }()

	// ApplyRLS must safely ignore the malicious claim key without crashing or executing injected SQL
	ApplyRLS(ctx, maliciousTx, maliciousAuthClaims)

	var tableCount int
	if queryErr := db.QueryRow(ctx, "SELECT count(*) FROM public.documents").Scan(&tableCount); queryErr != nil {
		t.Fatalf("table was compromised or query failed: %v", queryErr)
	}
	if tableCount != 2 {
		t.Fatalf("expected 2 documents to remain intact, got: %d", tableCount)
	}
}
