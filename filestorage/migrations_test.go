package filestorage

import (
	"fmt"
	"testing"
)

func TestFilestorageMigrationsDefinitionUnit(t *testing.T) {
	if len(Migrations) == 0 {
		t.Fatal("expected at least one migration defined in Migrations")
	}

	for _, databaseMigration := range Migrations {
		t.Run(fmt.Sprintf("Version_%d", databaseMigration.Version), func(t *testing.T) {
			if databaseMigration.Service != "filestorage" {
				t.Fatalf("expected Service 'filestorage', got %q", databaseMigration.Service)
			}
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
		})
	}
}
