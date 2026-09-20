package data

import (
	"encoding/json"
	"fmt"
	"net/http"

	"layr.sh/core"
)

// handleCreateColumn adds a new column to a table.
func (controlPlaneHandler *ControlPlaneHandler) handleCreateColumn(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
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
	controlPlaneHandler.invalidateCache(request.Context())
	controlPlaneHandler.invalidateTableCache(request.Context(), schema, table)

	if controlPlaneHandler.eventBus != nil {
		tableID := fmt.Sprintf("%s.%s", schema, table)
		controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
			Schema: schema,
			Table:  table,
			Action: "add_column",
			Detail: column.Name,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusCreated, CreateColumnResponse{
		Status: "created",
		Column: column.Name,
	})
}

// handleUpdateColumn alters a column definition.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateColumn(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
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
	controlPlaneHandler.invalidateCache(request.Context())
	controlPlaneHandler.invalidateTableCache(request.Context(), schema, table)

	if controlPlaneHandler.eventBus != nil {
		tableID := fmt.Sprintf("%s.%s", schema, table)
		controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
			Schema: schema,
			Table:  table,
			Action: "alter_column",
			Detail: columnName,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, UpdateColumnResponse{
		Status: "updated",
		Column: columnName,
	})
}

// handleDeleteColumn drops a column from a table.
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteColumn(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
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
	controlPlaneHandler.invalidateCache(request.Context())
	controlPlaneHandler.invalidateTableCache(request.Context(), schema, table)

	if controlPlaneHandler.eventBus != nil {
		tableID := fmt.Sprintf("%s.%s", schema, table)
		controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
			Schema: schema,
			Table:  table,
			Action: "drop_column",
			Detail: columnName,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, DeleteColumnResponse{
		Status: "deleted",
		Column: columnName,
	})
}
