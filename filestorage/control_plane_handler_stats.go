package filestorage

import (
	"encoding/json"
	"net/http"

	"layr.sh/core"
)

// handleGetStats handles GET /v1/_/file-storage/stats.
func (controlPlaneHandler *ControlPlaneHandler) handleGetStats(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFileStorageBucketRead) {
		return
	}
	// Aggregate each relation separately to avoid multiplying object sizes by chunk or part counts.
	const querySQL = `
 WITH bucket_usage AS (
  SELECT b.name, b.backend, b.is_public, COUNT(o.id) AS object_count,
         COALESCE(SUM(o.size_bytes), 0) AS size_bytes, MAX(o.last_updated_at) AS last_updated_at
  FROM file_storage.buckets b LEFT JOIN file_storage.objects o ON o.bucket_id = b.id
  GROUP BY b.id
 ), content_types AS (
  SELECT content_type, COUNT(*) AS object_count, SUM(size_bytes) AS size_bytes
  FROM file_storage.objects GROUP BY content_type
 )
 SELECT jsonb_build_object(
  'total_buckets', COUNT(*),
  'public_buckets', COUNT(*) FILTER (WHERE is_public),
  'private_buckets', COUNT(*) FILTER (WHERE NOT is_public),
  'database_buckets', COUNT(*) FILTER (WHERE backend = 'database'),
  's3_buckets', COUNT(*) FILTER (WHERE backend = 's3'),
  'total_objects', COALESCE(SUM(object_count), 0),
  'total_size_bytes', COALESCE(SUM(size_bytes), 0),
  'database_objects', COALESCE(SUM(object_count) FILTER (WHERE backend = 'database'), 0),
  'database_size_bytes', COALESCE(SUM(size_bytes) FILTER (WHERE backend = 'database'), 0),
  's3_objects', COALESCE(SUM(object_count) FILTER (WHERE backend = 's3'), 0),
  's3_size_bytes', COALESCE(SUM(size_bytes) FILTER (WHERE backend = 's3'), 0),
  'last_updated_at', MAX(last_updated_at),
  'database_chunks', (SELECT COUNT(*) FROM file_storage.chunks),
  'database_chunk_bytes', (SELECT COALESCE(SUM(octet_length(chunk_data)), 0) FROM file_storage.chunks),
  'multipart_uploads', (SELECT COUNT(*) FROM file_storage.multipart_uploads),
  'multipart_parts', (SELECT COUNT(*) FROM file_storage.multipart_parts),
  'multipart_size_bytes', (SELECT COALESCE(SUM(size_bytes), 0) FROM file_storage.multipart_parts),
  'buckets', COALESCE(jsonb_agg(to_jsonb(bucket_usage) ORDER BY name), '[]'::jsonb),
  'content_types', (SELECT COALESCE(jsonb_agg(to_jsonb(content_types) ORDER BY size_bytes DESC, content_type), '[]'::jsonb) FROM content_types)
 ) FROM bucket_usage;
 `
	var rawStats []byte
	if err := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), querySQL).Scan(&rawStats); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to load file storage statistics", err.Error())
		return
	}
	var statsResponse StatsResponse
	if err := json.Unmarshal(rawStats, &statsResponse); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Failed to read file storage statistics", err.Error())
		return
	}
	core.WriteJSONResponse(responseWriter, http.StatusOK, statsResponse)
}
