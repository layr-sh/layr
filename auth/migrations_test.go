package auth

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthMigrationsDefinitionUnit(t *testing.T) {
	t.Parallel()

	require.Len(t, Migrations, 1)

	for _, databaseMigration := range Migrations {
		t.Run(fmt.Sprintf("Version_%d", databaseMigration.Version), func(t *testing.T) {
			require.Equal(t, "auth", databaseMigration.Service)
			require.Equal(t, 1, databaseMigration.Version)
			require.NotEmpty(t, databaseMigration.Description)
			require.NotEmpty(t, databaseMigration.UpSQL)
			require.NotEmpty(t, databaseMigration.DownSQL)
		})
	}
}
