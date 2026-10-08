package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthLoggerUnit(t *testing.T) {
	t.Parallel()

	require.NotNil(t, log)
	require.Equal(t, "auth", log.Scope())
}
