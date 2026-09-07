package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestCoreServiceAccountFullLifecycleIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	pgContainer, containerErr := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("layr"),
		tcpostgres.WithUsername("layr"),
		tcpostgres.WithPassword("layr"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if containerErr != nil {
		t.Skip("docker not available")
		return
	}
	defer func() { _ = pgContainer.Terminate(ctx) }()

	databaseURL, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
	db, err := NewDatabasePool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to create db connection pool: %v", err)
	}
	defer db.Close()

	if migrationErr := db.RunMigrations(ctx, SystemDatabaseMigrations); migrationErr != nil {
		t.Fatalf("failed to run migrations: %v", migrationErr)
	}

	serviceAccountManager := NewServiceAccountManager(db)

	// 1. Create root account (omitting scopes to test default ScopeRoot branch)
	descriptionText := "Root Machine Key"
	rootServiceAccount, err := serviceAccountManager.Create(ctx, CreateServiceAccountInput{
		Name:        "Root Account",
		Description: &descriptionText,
	})
	if err != nil {
		t.Fatalf("failed to create root account: %v", err)
	}
	if rootServiceAccount.SecretKey == "" {
		t.Fatal("expected secret key in result")
	}
	if rootServiceAccount.Name != "Root Account" {
		t.Fatalf("expected name 'Root Account', got: %s", rootServiceAccount.Name)
	}

	// 2. Create second scoped account with IP whitelist and expiration
	future := time.Now().UTC().Add(1 * time.Hour)
	dataWorkerServiceAccount, err := serviceAccountManager.Create(ctx, CreateServiceAccountInput{
		Name:       "Data Worker",
		Scopes:     []string{"data:query.read", "data:cache.read"},
		AllowedIPs: []string{"192.168.1.100", "10.0.0.0/8"},
		ExpiresAt:  &future,
	})
	if err != nil {
		t.Fatalf("failed to create data worker: %v", err)
	}

	// 3. List
	list, err := serviceAccountManager.List(ctx)
	if err != nil {
		t.Fatalf("failed to list service accounts: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 accounts, got: %d", len(list))
	}

	// 4. Get
	retrievedServiceAccount, err := serviceAccountManager.Get(ctx, rootServiceAccount.ID)
	if err != nil {
		t.Fatalf("failed to get root service account: %v", err)
	}
	if retrievedServiceAccount.ID != rootServiceAccount.ID {
		t.Fatalf("expected root service account id %s, got %s", rootServiceAccount.ID, retrievedServiceAccount.ID)
	}

	// 5. Authenticate Root Account
	authenticatedRoot, err := serviceAccountManager.Authenticate(ctx, rootServiceAccount.SecretKey, "127.0.0.1")
	if err != nil {
		t.Fatalf("failed to authenticate root service account: %v", err)
	}
	if authenticatedRoot.ID != rootServiceAccount.ID {
		t.Fatalf("authenticated id mismatch: %s != %s", authenticatedRoot.ID, rootServiceAccount.ID)
	}

	// 6. Authenticate Data Worker with allowed IP
	authenticatedWorker, err := serviceAccountManager.Authenticate(ctx, dataWorkerServiceAccount.SecretKey, "10.1.2.3")
	if err != nil {
		t.Fatalf("failed to authenticate data worker service account with 10.1.2.3: %v", err)
	}
	if authenticatedWorker.ID != dataWorkerServiceAccount.ID {
		t.Fatalf("authenticated id mismatch: %s != %s", authenticatedWorker.ID, dataWorkerServiceAccount.ID)
	}

	// 7. Authenticate Data Worker with blocked IP -> ErrServiceAccountIPBlocked
	_, err = serviceAccountManager.Authenticate(ctx, dataWorkerServiceAccount.SecretKey, "8.8.8.8")
	if !errors.Is(err, ErrServiceAccountIPBlocked) {
		t.Fatalf("expected ErrServiceAccountIPBlocked, got: %v", err)
	}

	// 8. Authenticate Invalid Key -> ErrServiceAccountNotFound
	_, err = serviceAccountManager.Authenticate(ctx, "invalid_secret_key_123456789", "127.0.0.1")
	if !errors.Is(err, ErrServiceAccountNotFound) {
		t.Fatalf("expected ErrServiceAccountNotFound, got: %v", err)
	}

	// 9. Root Account Protection: Attempt deleting the ONLY root account -> ErrRootAccountProtected
	err = serviceAccountManager.Delete(ctx, rootServiceAccount.ID)
	if !errors.Is(err, ErrRootAccountProtected) {
		t.Fatalf("expected ErrRootAccountProtected when deleting only root account, got: %v", err)
	}

	// 10. Root Account Protection: Attempt disabling the ONLY root account -> ErrRootAccountProtected
	disabled := false
	_, err = serviceAccountManager.Update(ctx, rootServiceAccount.ID, UpdateServiceAccountInput{
		IsEnabled: &disabled,
	})
	if !errors.Is(err, ErrRootAccountProtected) {
		t.Fatalf("expected ErrRootAccountProtected when disabling only root account, got: %v", err)
	}

	// 11. Create a second root account
	secondRootServiceAccount, err := serviceAccountManager.Create(ctx, CreateServiceAccountInput{
		Name:   "Second Root",
		Scopes: []string{ScopeRoot},
	})
	if err != nil {
		t.Fatalf("failed to create second root account: %v", err)
	}

	// 12. Deleting rootServiceAccount now succeeds because secondRootServiceAccount is active root
	if deleteRootErr := serviceAccountManager.Delete(ctx, rootServiceAccount.ID); deleteRootErr != nil {
		t.Fatalf("expected delete to succeed with remaining root, got: %v", deleteRootErr)
	}

	// 13. Delete dataWorkerServiceAccount (non-root)
	if deleteWorkerErr := serviceAccountManager.Delete(ctx, dataWorkerServiceAccount.ID); deleteWorkerErr != nil {
		t.Fatalf("expected delete dataWorkerServiceAccount to succeed, got: %v", deleteWorkerErr)
	}

	// 13b. Attempt deleting service account owned by a console user -> ErrServiceAccountOwnedByConsoleUser
	consoleUserID := "01918342-9999-7000-8000-000000000001"
	_, _ = db.Exec(ctx, "INSERT INTO console.users (id, email, password_hash, is_enabled) VALUES ($1, $2, $3, true)", consoleUserID, "owned_service_account_test@layr.local", "dummyhash")
	userOwnedServiceAccount, err := serviceAccountManager.Create(ctx, CreateServiceAccountInput{
		ConsoleUserID: &consoleUserID,
		Name:          "User Owned Service Account",
		Scopes:        []string{"data:*"},
	})
	if err != nil {
		t.Fatalf("failed to create user owned service account: %v", err)
	}
	err = serviceAccountManager.Delete(ctx, userOwnedServiceAccount.ID)
	if !errors.Is(err, ErrServiceAccountOwnedByConsoleUser) {
		t.Fatalf("expected ErrServiceAccountOwnedByConsoleUser, got: %v", err)
	}
	// Deleting the console user cascades and removes userOwnedServiceAccount
	_, err = db.Exec(ctx, "DELETE FROM console.users WHERE id = $1", consoleUserID)
	if err != nil {
		t.Fatalf("failed to delete console user: %v", err)
	}
	if _, cascadeGetErr := serviceAccountManager.Get(ctx, userOwnedServiceAccount.ID); !errors.Is(cascadeGetErr, ErrServiceAccountNotFound) {
		t.Fatalf("expected userOwnedServiceAccount to be cascade deleted when user deleted, got: %v", cascadeGetErr)
	}

	// 13c. Get non-existent -> ErrServiceAccountNotFound
	if _, missingGetErr := serviceAccountManager.Get(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(missingGetErr, ErrServiceAccountNotFound) {
		t.Fatalf("expected ErrServiceAccountNotFound, got: %v", missingGetErr)
	}

	// 14. Authenticate with short key (<16 chars) -> ErrServiceAccountNotFound
	_, err = serviceAccountManager.Authenticate(ctx, "short_key", "127.0.0.1")
	if !errors.Is(err, ErrServiceAccountNotFound) {
		t.Fatalf("expected ErrServiceAccountNotFound for short key, got: %v", err)
	}

	// 15. Authenticate with Expired Service Account -> ErrServiceAccountExpired
	past := time.Now().UTC().Add(-1 * time.Hour)
	expiredServiceAccount, err := serviceAccountManager.Create(ctx, CreateServiceAccountInput{
		Name:      "Expired Service Account",
		Scopes:    []string{ScopeDataSchemaRead},
		ExpiresAt: &past,
	})
	if err != nil {
		t.Fatalf("failed to create expired service account: %v", err)
	}
	_, err = serviceAccountManager.Authenticate(ctx, expiredServiceAccount.SecretKey, "127.0.0.1")
	if !errors.Is(err, ErrServiceAccountExpired) {
		t.Fatalf("expected ErrServiceAccountExpired, got: %v", err)
	}

	// 16. Authenticate with Disabled Service Account -> ErrServiceAccountDisabled
	disabledFalse := false
	_, err = serviceAccountManager.Update(ctx, expiredServiceAccount.ID, UpdateServiceAccountInput{
		IsEnabled: &disabledFalse,
	})
	if err != nil {
		t.Fatalf("failed to update service account: %v", err)
	}
	_, err = serviceAccountManager.Authenticate(ctx, expiredServiceAccount.SecretKey, "127.0.0.1")
	if !errors.Is(err, ErrServiceAccountDisabled) {
		t.Fatalf("expected ErrServiceAccountDisabled, got: %v", err)
	}

	// 17. Nil db connection pool checks
	nilServiceAccountManager := NewServiceAccountManager(nil)
	if _, nilCreateErr := nilServiceAccountManager.Create(ctx, CreateServiceAccountInput{Name: "X"}); nilCreateErr == nil {
		t.Fatal("expected error on nil db connection pool Create")
	}
	if _, nilListErr := nilServiceAccountManager.List(ctx); nilListErr == nil {
		t.Fatal("expected error on nil db connection pool List")
	}
	if _, nilGetErr := nilServiceAccountManager.Get(ctx, "00000000-0000-0000-0000-000000000000"); nilGetErr == nil {
		t.Fatal("expected error on nil db connection pool Get")
	}
	if _, nilUpdateErr := nilServiceAccountManager.Update(ctx, "00000000-0000-0000-0000-000000000000", UpdateServiceAccountInput{}); nilUpdateErr == nil {
		t.Fatal("expected error on nil db connection pool Update")
	}
	if nilDeleteErr := nilServiceAccountManager.Delete(ctx, "00000000-0000-0000-0000-000000000000"); nilDeleteErr == nil {
		t.Fatal("expected error on nil db connection pool Delete")
	}
	if _, nilAuthErr := nilServiceAccountManager.Authenticate(ctx, "01234567890123456789", "127.0.0.1"); nilAuthErr == nil {
		t.Fatal("expected error on nil db connection pool Authenticate")
	}

	// 18. Empty name validation on Create
	if _, emptyCreateErr := serviceAccountManager.Create(ctx, CreateServiceAccountInput{Name: "   "}); emptyCreateErr == nil {
		t.Fatal("expected error on empty name Create")
	}

	// 19. Update all fields of service account
	newName := "Renamed Service Account"
	newDescription := "New description"
	newScopes := []string{ScopeDataQueryWrite}
	newIPs := []string{"10.0.0.1/32"}
	futureTime := time.Now().UTC().Add(24 * time.Hour)
	updatedServiceAccount, updateErr := serviceAccountManager.Update(ctx, expiredServiceAccount.ID, UpdateServiceAccountInput{
		Name:        &newName,
		Description: &newDescription,
		Scopes:      newScopes,
		AllowedIPs:  newIPs,
		ExpiresAt:   &futureTime,
	})
	if updateErr != nil {
		t.Fatalf("failed to update all fields of service account: %v", updateErr)
	}
	if updatedServiceAccount.Name != newName || *updatedServiceAccount.Description != newDescription {
		t.Fatalf("expected updated fields, got: %+v", updatedServiceAccount)
	}

	// 20. Removing Root Scope from only root account -> ErrRootAccountProtected
	_, err = serviceAccountManager.Update(ctx, secondRootServiceAccount.ID, UpdateServiceAccountInput{
		Scopes: []string{"data:query.read"},
	})
	if !errors.Is(err, ErrRootAccountProtected) {
		t.Fatalf("expected ErrRootAccountProtected when removing root scope, got: %v", err)
	}
}
