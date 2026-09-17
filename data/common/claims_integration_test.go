package common

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"layr.sh/core"
)

func TestCommonClaimsApplyRLSIntegration(t *testing.T) {
	ctx := context.Background()

	// 1. Boot Postgres testcontainer
	postgresContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("layr_claims_integration"),
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

	// 2. Begin transaction and apply claims
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	jwtClaims := core.JWTClaims{
		Subject:  "usr_integration_001",
		Role:     "editor",
		Email:    "editor@example.com",
		Issuer:   "layr-test",
		Audience: "layr-app:user",
		Claims: map[string]any{
			"tenant_id":     "tenant_xyz",
			"plan":          "enterprise",
			"unsafe.claim!": "ignored",
		},
	}

	ApplyRLS(ctx, tx, jwtClaims)

	// 3. Verify PostgreSQL current_setting values within transaction
	var currentSubject, currentRole, currentEmail, currentIssuer, currentAudience, currentTenant, currentPlan, currentUnsafe, currentFullJSON string
	if err := tx.QueryRow(ctx, "SELECT current_setting('request.jwt.sub', true)").Scan(&currentSubject); err != nil {
		t.Fatalf("failed to query request.jwt.sub: %v", err)
	}
	if currentSubject != "usr_integration_001" {
		t.Fatalf("expected sub usr_integration_001, got %q", currentSubject)
	}

	if err := tx.QueryRow(ctx, "SELECT current_setting('request.jwt.role', true)").Scan(&currentRole); err != nil {
		t.Fatalf("failed to query request.jwt.role: %v", err)
	}
	if currentRole != "editor" {
		t.Fatalf("expected role editor, got %q", currentRole)
	}

	if err := tx.QueryRow(ctx, "SELECT current_setting('request.jwt.email', true)").Scan(&currentEmail); err != nil {
		t.Fatalf("failed to query request.jwt.email: %v", err)
	}
	if currentEmail != "editor@example.com" {
		t.Fatalf("expected email editor@example.com, got %q", currentEmail)
	}

	if err := tx.QueryRow(ctx, "SELECT current_setting('request.jwt.iss', true)").Scan(&currentIssuer); err != nil {
		t.Fatalf("failed to query request.jwt.iss: %v", err)
	}
	if currentIssuer != "layr-test" {
		t.Fatalf("expected iss layr-test, got %q", currentIssuer)
	}

	if err := tx.QueryRow(ctx, "SELECT current_setting('request.jwt.aud', true)").Scan(&currentAudience); err != nil {
		t.Fatalf("failed to query request.jwt.aud: %v", err)
	}
	if currentAudience != "layr-app:user" {
		t.Fatalf("expected aud layr-app:user, got %q", currentAudience)
	}

	if err := tx.QueryRow(ctx, "SELECT current_setting('request.jwt.tenant_id', true)").Scan(&currentTenant); err != nil {
		t.Fatalf("failed to query request.jwt.tenant_id: %v", err)
	}
	if currentTenant != "tenant_xyz" {
		t.Fatalf("expected tenant tenant_xyz, got %q", currentTenant)
	}

	if err := tx.QueryRow(ctx, "SELECT current_setting('request.jwt.plan', true)").Scan(&currentPlan); err != nil {
		t.Fatalf("failed to query request.jwt.plan: %v", err)
	}
	if currentPlan != "enterprise" {
		t.Fatalf("expected plan enterprise, got %q", currentPlan)
	}

	if err := tx.QueryRow(ctx, "SELECT current_setting('request.jwt', true)").Scan(&currentFullJSON); err != nil {
		t.Fatalf("failed to query request.jwt: %v", err)
	}
	if !strings.Contains(currentFullJSON, "usr_integration_001") || !strings.Contains(currentFullJSON, "tenant_xyz") {
		t.Fatalf("expected full JSON in request.jwt, got %q", currentFullJSON)
	}

	// Unsafe claim must NOT be set
	_ = tx.QueryRow(ctx, "SELECT current_setting('request.jwt.unsafe.claim!', true)").Scan(&currentUnsafe)
	if currentUnsafe != "" {
		t.Fatalf("expected unsafe claim to be empty, got %q", currentUnsafe)
	}
}
