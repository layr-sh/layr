package data

import (
	"net/http"
	"strings"
)

// ControlPlaneHandler handles control plane endpoints under /v1/_/data/*.
type ControlPlaneHandler struct {
	*Service
}

// NewControlPlaneHandler creates an HTTP handler for data control plane management.
func NewControlPlaneHandler(service *Service) *ControlPlaneHandler {
	return &ControlPlaneHandler{
		Service: service,
	}
}

func (controlPlaneHandler *ControlPlaneHandler) extractSchemaAndTable(request *http.Request) (string, string) {
	schema := request.PathValue("schema_name")
	table := request.PathValue("table_name")
	if schema != "" && table != "" {
		return schema, table
	}
	path := strings.TrimPrefix(request.URL.Path, "/v1/_/data/tables")
	path = strings.TrimPrefix(path, "/v1/_/data")
	cleaned := strings.Trim(path, "/")
	if cleaned == "" {
		return schema, table
	}
	const (
		minSegments = 1
		twoSegments = 2
	)
	parts := strings.Split(cleaned, "/")
	if len(parts) >= minSegments && schema == "" {
		schema = parts[0]
	}
	if len(parts) >= twoSegments && table == "" {
		table = parts[1]
	}
	return schema, table
}

func (controlPlaneHandler *ControlPlaneHandler) extractColumnName(request *http.Request) string {
	columnName := request.PathValue("column_name")
	if columnName != "" {
		return columnName
	}
	path := strings.TrimPrefix(request.URL.Path, "/v1/_/data/tables")
	path = strings.TrimPrefix(path, "/v1/_/data")
	cleaned := strings.Trim(path, "/")
	if cleaned == "" {
		return ""
	}
	const (
		requiredSegments    = 4
		segmentActionIndex  = 2
		segmentElementIndex = 3
	)
	parts := strings.Split(cleaned, "/")
	if len(parts) >= requiredSegments && parts[segmentActionIndex] == "columns" {
		return parts[segmentElementIndex]
	}
	return ""
}

func (controlPlaneHandler *ControlPlaneHandler) extractIndexName(request *http.Request) string {
	indexName := request.PathValue("index_name")
	if indexName != "" {
		return indexName
	}
	path := strings.TrimPrefix(request.URL.Path, "/v1/_/data/tables")
	path = strings.TrimPrefix(path, "/v1/_/data")
	cleaned := strings.Trim(path, "/")
	if cleaned == "" {
		return ""
	}
	const (
		requiredSegments    = 4
		segmentActionIndex  = 2
		segmentElementIndex = 3
	)
	parts := strings.Split(cleaned, "/")
	if len(parts) >= requiredSegments && parts[segmentActionIndex] == "indexes" {
		return parts[segmentElementIndex]
	}
	return ""
}

func (controlPlaneHandler *ControlPlaneHandler) extractPolicyName(request *http.Request) string {
	policyName := request.PathValue("policy_name")
	if policyName != "" {
		return policyName
	}
	path := strings.TrimPrefix(request.URL.Path, "/v1/_/data/tables")
	path = strings.TrimPrefix(path, "/v1/_/data")
	cleaned := strings.Trim(path, "/")
	if cleaned == "" {
		return ""
	}
	const (
		requiredSegments    = 4
		segmentActionIndex  = 2
		segmentElementIndex = 3
	)
	parts := strings.Split(cleaned, "/")
	if len(parts) >= requiredSegments && parts[segmentActionIndex] == "policies" {
		return parts[segmentElementIndex]
	}
	return ""
}
