package filestorage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilestorageLoggerUnit(t *testing.T) {
	t.Parallel()

	require.NotNil(t, log)
	require.Equal(t, "filestorage", log.Scope())
}
