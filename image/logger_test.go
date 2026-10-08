package image

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageLoggerUnit(t *testing.T) {
	t.Parallel()

	require.NotNil(t, log)
	require.Equal(t, "image", log.Scope())
}
