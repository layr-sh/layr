package function

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFunctionMigrationsDefinitionUnit(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, Migrations)

	for _, databaseMigration := range Migrations {
		t.Run(fmt.Sprintf("Version_%d", databaseMigration.Version), func(t *testing.T) {
			require.Equal(t, "function", databaseMigration.Service)
			require.Positive(t, databaseMigration.Version)
			require.NotEmpty(t, databaseMigration.Description)
			require.NotEmpty(t, databaseMigration.UpSQL)
			require.NotEmpty(t, databaseMigration.DownSQL)
		})
	}
}
