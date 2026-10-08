package console

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsoleLoggerUnit(t *testing.T) {
	t.Parallel()

	require.NotNil(t, log)
	require.Equal(t, "console", log.Scope())
}
