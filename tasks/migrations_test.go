package tasks

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTasksMigrationsDefinitionUnit(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, Migrations)

	for _, databaseMigration := range Migrations {
		require.Positive(t, databaseMigration.Version)
		require.NotEmpty(t, databaseMigration.Description)
		require.NotEmpty(t, databaseMigration.UpSQL)
		require.NotEmpty(t, databaseMigration.DownSQL)
	}
}
