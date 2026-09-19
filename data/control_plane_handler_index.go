package data

import (
	"encoding/json"
	"fmt"
	"net/http"

	"layr.sh/core"
)

// HandleListIndexes lists all indexes on a table.
func (controlPlaneHandler *ControlPlaneHandler) HandleListIndexes(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.read") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/indexes")
		return
	}
	indexes, err := controlPlaneHandler.ddlEngine.ListIndexes(request.Context(), schema, table)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, ListIndexesResponse{
		Indexes: indexes,
		Count:   len(indexes),
	})
}

// HandleCreateIndex creates a new index on a table.
func (controlPlaneHandler *ControlPlaneHandler) HandleCreateIndex(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/indexes")
		return
	}
	var createIndexRequest CreateIndexRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&createIndexRequest); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if createErr := controlPlaneHandler.ddlEngine.CreateIndex(request.Context(), schema, table, createIndexRequest); createErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, createErr.Error())
		return
	}
	controlPlaneHandler.invalidateCache(request.Context())
	controlPlaneHandler.invalidateTableCache(request.Context(), schema, table)

	if controlPlaneHandler.eventBus != nil {
		tableID := fmt.Sprintf("%s.%s", schema, table)
		controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
			Schema: schema,
			Table:  table,
			Action: "create_index",
			Detail: createIndexRequest.IndexName,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusCreated, CreateIndexResponse{
		Status: "created",
		Index:  createIndexRequest.IndexName,
	})
}

// HandleDropIndex drops an index from a table.
func (controlPlaneHandler *ControlPlaneHandler) HandleDropIndex(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	indexName := controlPlaneHandler.extractIndexName(request)
	if schema == "" || indexName == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/indexes/{index}")
		return
	}
	if dropErr := controlPlaneHandler.ddlEngine.DropIndex(request.Context(), schema, indexName); dropErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, dropErr.Error())
		return
	}
	controlPlaneHandler.invalidateCache(request.Context())
	if table != "" {
		controlPlaneHandler.invalidateTableCache(request.Context(), schema, table)
	}

	if controlPlaneHandler.eventBus != nil {
		tableID := fmt.Sprintf("%s.%s", schema, table)
		controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
			Schema: schema,
			Table:  table,
			Action: "drop_index",
			Detail: indexName,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, DropIndexResponse{
		Status: "deleted",
		Index:  indexName,
	})
}
