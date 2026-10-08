package function

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFunctionLoggerUnit(t *testing.T) {
	t.Parallel()

	require.NotNil(t, log)
	require.Equal(t, "function", log.Scope())
}
