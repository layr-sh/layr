package data

import (
	"testing"

	"layr.sh/core"
)

func TestDataMigrationsDefinitionUnit(t *testing.T) {
	if len(Migrations) == 0 {
		t.Fatal("expected at least one migration defined in Migrations")
	}

	for _, databaseMigration := range Migrations {
		if databaseMigration.Version <= 0 {
			t.Fatalf("expected positive migration version, got %d", databaseMigration.Version)
		}
		if databaseMigration.Description == "" {
			t.Fatalf("expected non-empty description for migration %d", databaseMigration.Version)
		}
		if databaseMigration.UpSQL == "" {
			t.Fatalf("expected non-empty UpSQL for migration version %d", databaseMigration.Version)
		}
		if databaseMigration.DownSQL == "" {
			t.Fatalf("expected non-empty DownSQL for migration version %d", databaseMigration.Version)
		}
	}
}

func TestDataServiceFactoryUnit(t *testing.T) {
	serviceFactory, exists := core.GetServiceFactory("data")
	if !exists || serviceFactory == nil {
		t.Fatal("expected 'data' service factory to be registered in core")
	}

	kernel := &core.Kernel{}
	serviceRunner, err := serviceFactory(kernel)
	if err != nil {
		t.Fatalf("unexpected error creating data service from kernel: %v", err)
	}
	if serviceRunner == nil {
		t.Fatal("expected non-nil service runner from factory")
	}
	if _, ok := serviceRunner.(*Service); !ok {
		t.Fatalf("expected *Service runner, got %T", serviceRunner)
	}

	dependenciesKernel := core.NewTestKernel(nil)
	dependenciesServiceRunner, err := serviceFactory(dependenciesKernel)
	if err != nil {
		t.Fatalf("unexpected error creating data service from kernel with deps: %v", err)
	}
	if dependenciesServiceRunner == nil {
		t.Fatal("expected non-nil service runner from factory with deps")
	}
}
