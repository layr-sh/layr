package filestorage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestFilestorageControlPlaneHandlerStatsUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	controlPlaneHandler := NewService(kernel).ControlPlaneHandler()
	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/_/file-storage/stats", nil)
	controlPlaneHandler.handleGetStats(responseRecorder, request)
	require.Equal(t, http.StatusForbidden, responseRecorder.Code)
}
