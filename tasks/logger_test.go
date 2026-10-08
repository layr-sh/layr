package tasks

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTasksLoggerUnit(t *testing.T) {
	t.Parallel()

	require.NotNil(t, log)
	require.Equal(t, "tasks", log.Scope())
}
