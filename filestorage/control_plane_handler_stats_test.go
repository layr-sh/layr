package filestorage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
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

func TestFilestorageControlPlaneStatsInvalidDatabaseResultUnit(t *testing.T) {
	kernel := postgresFaultKernel(t.Context(), t, func(sql string) []pgproto3.BackendMessage {
		if strings.Contains(sql, "WITH bucket_usage AS") {
			return postgresResult([]uint32{pgtype.JSONBOID}, []string{`{"total_buckets":"not a count"}`})
		}
		return nil
	})
	request := storageTestRequest(http.MethodGet, "/v1/_/file-storage/stats", "", core.ScopeFileStorageBucketRead)
	responseRecorder := httptest.NewRecorder()
	NewService(kernel).ControlPlaneHandler().handleGetStats(responseRecorder, request)
	require.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	require.Contains(t, responseRecorder.Body.String(), "Failed to read file storage statistics")
}
