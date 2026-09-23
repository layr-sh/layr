package image

import (
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageBaseHandlerUnit(t *testing.T) {
	t.Parallel()

	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	baseHandler := NewBaseHandler(service)

	require.NotNil(t, baseHandler)
	require.Equal(t, service, baseHandler.Service)
}
