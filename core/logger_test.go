package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoreLoggerUnit(t *testing.T) {
	t.Parallel()

	require.NotNil(t, log)
	require.Equal(t, "core", log.Scope())
}
