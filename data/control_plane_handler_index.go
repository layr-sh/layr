package data

import (
	"encoding/json"
	"fmt"
	"net/http"

	"layr.sh/core"
)

// handleListIndexes lists all indexes on a table.
func (controlPlaneHandler *ControlPlaneHandler) handleListIndexes(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleListIndexes invoked")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeDataSchemaRead) {
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /v1/_/data/tables/{schema}/{table}/indexes")
		return
	}
	indexes, err := controlPlaneHandler.ddlEngine.ListIndexes(request.Context(), schema, table)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	log.Debugf("retrieved %d index(es) on %s.%s", len(indexes), schema, table)
	core.WriteJSONResponse(responseWriter, http.StatusOK, ListIndexesResponse{
		Indexes: indexes,
		Count:   len(indexes),
	})
}

// handleCreateIndex creates a new index on a table.
func (controlPlaneHandler *ControlPlaneHandler) handleCreateIndex(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleCreateIndex invoked")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeDataSchemaWrite) {
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /v1/_/data/tables/{schema}/{table}/indexes")
		return
	}
	var createIndexInput CreateIndexInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createIndexInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if createErr := controlPlaneHandler.ddlEngine.CreateIndex(request.Context(), schema, table, createIndexInput); createErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, createErr.Error())
		return
	}
	controlPlaneHandler.InvalidateCatalog(request.Context())
	controlPlaneHandler.InvalidateTableCache(request.Context(), schema, table)

	tableID := fmt.Sprintf("%s.%s", schema, table)
	controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
		Schema: schema,
		Table:  table,
		Action: "create_index",
		Detail: createIndexInput.IndexName,
	}))

	log.Debugf("index %s successfully created on %s.%s", createIndexInput.IndexName, schema, table)
	core.WriteJSONResponse(responseWriter, http.StatusCreated, CreateIndexResponse{
		Status: "created",
		Index:  createIndexInput.IndexName,
	})
}

// handleDeleteIndex drops an index from a table.
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteIndex(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleDeleteIndex invoked")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeDataSchemaWrite) {
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	indexName := controlPlaneHandler.extractIndexName(request)
	if schema == "" || indexName == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /v1/_/data/tables/{schema}/{table}/indexes/{index}")
		return
	}
	if dropErr := controlPlaneHandler.ddlEngine.DropIndex(request.Context(), schema, indexName); dropErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, dropErr.Error())
		return
	}
	controlPlaneHandler.InvalidateCatalog(request.Context())
	if table != "" {
		controlPlaneHandler.InvalidateTableCache(request.Context(), schema, table)
	}

	tableID := fmt.Sprintf("%s.%s", schema, table)
	controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
		Schema: schema,
		Table:  table,
		Action: "drop_index",
		Detail: indexName,
	}))

	log.Debugf("index %s successfully dropped from %s.%s", indexName, schema, table)
	core.WriteJSONResponse(responseWriter, http.StatusOK, DeleteIndexResponse{
		Status: "deleted",
		Index:  indexName,
	})
}
