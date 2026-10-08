package data

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDataLoggerUnit(t *testing.T) {
	t.Parallel()

	require.NotNil(t, log)
	require.Equal(t, "data", log.Scope())
}
