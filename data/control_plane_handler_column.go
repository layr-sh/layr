package data

import (
	"encoding/json"
	"fmt"
	"net/http"

	"layr.sh/core"
)

// handleCreateColumn adds a new column to a table.
func (controlPlaneHandler *ControlPlaneHandler) handleCreateColumn(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleCreateColumn invoked")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeDataSchemaWrite) {
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /v1/_/data/tables/{schema}/{table}/columns")
		return
	}
	var column Column
	if decodeErr := json.NewDecoder(request.Body).Decode(&column); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if addErr := controlPlaneHandler.ddlEngine.AddColumn(request.Context(), schema, table, column); addErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, addErr.Error())
		return
	}
	controlPlaneHandler.InvalidateCatalog(request.Context())
	controlPlaneHandler.InvalidateTableCache(request.Context(), schema, table)

	tableID := fmt.Sprintf("%s.%s", schema, table)
	controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
		Schema: schema,
		Table:  table,
		Action: "add_column",
		Detail: column.Name,
	}))

	log.Debugf("column %s successfully added to %s.%s", column.Name, schema, table)
	core.WriteJSONResponse(responseWriter, http.StatusCreated, CreateColumnResponse{
		Status: "created",
		Column: column.Name,
	})
}

// handleUpdateColumn alters a column definition.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateColumn(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleUpdateColumn invoked")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeDataSchemaWrite) {
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	columnName := controlPlaneHandler.extractColumnName(request)
	if schema == "" || table == "" || columnName == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /v1/_/data/tables/{schema}/{table}/columns/{column}")
		return
	}
	var updateColumnInput UpdateColumnInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&updateColumnInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if alterErr := controlPlaneHandler.ddlEngine.AlterColumn(request.Context(), schema, table, columnName, updateColumnInput); alterErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, alterErr.Error())
		return
	}
	controlPlaneHandler.InvalidateCatalog(request.Context())
	controlPlaneHandler.InvalidateTableCache(request.Context(), schema, table)

	tableID := fmt.Sprintf("%s.%s", schema, table)
	controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
		Schema: schema,
		Table:  table,
		Action: "alter_column",
		Detail: columnName,
	}))

	log.Debugf("column %s successfully altered in %s.%s", columnName, schema, table)
	core.WriteJSONResponse(responseWriter, http.StatusOK, UpdateColumnResponse{
		Status: "updated",
		Column: columnName,
	})
}

// handleDeleteColumn drops a column from a table.
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteColumn(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleDeleteColumn invoked")

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeDataSchemaWrite) {
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	columnName := controlPlaneHandler.extractColumnName(request)
	if schema == "" || table == "" || columnName == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /v1/_/data/tables/{schema}/{table}/columns/{column}")
		return
	}
	cascade := request.URL.Query().Get("cascade") == "true"
	if dropErr := controlPlaneHandler.ddlEngine.DropColumn(request.Context(), schema, table, columnName, cascade); dropErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, dropErr.Error())
		return
	}
	controlPlaneHandler.InvalidateCatalog(request.Context())
	controlPlaneHandler.InvalidateTableCache(request.Context(), schema, table)

	tableID := fmt.Sprintf("%s.%s", schema, table)
	controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
		Schema: schema,
		Table:  table,
		Action: "drop_column",
		Detail: columnName,
	}))

	log.Debugf("column %s successfully dropped from %s.%s", columnName, schema, table)
	core.WriteJSONResponse(responseWriter, http.StatusOK, DeleteColumnResponse{
		Status: "deleted",
		Column: columnName,
	})
}
