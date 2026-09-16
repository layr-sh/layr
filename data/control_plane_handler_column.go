package data

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// HandleAddColumn adds a new column to a table.
func (controlPlaneHandler *ControlPlaneHandler) HandleAddColumn(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		controlPlaneHandler.writeForbidden(responseWriter, request)
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/columns")
		return
	}
	var columnDefinition ColumnDefinition
	if decodeErr := json.NewDecoder(request.Body).Decode(&columnDefinition); decodeErr != nil {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if addErr := controlPlaneHandler.ddlEngine.AddColumn(request.Context(), schema, table, columnDefinition); addErr != nil {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, addErr.Error())
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
			Detail: columnDefinition.Name,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusCreated, AddColumnResponse{
		Status: "created",
		Column: columnDefinition.Name,
	})
}

// HandleAlterColumn alters a column definition.
func (controlPlaneHandler *ControlPlaneHandler) HandleAlterColumn(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		controlPlaneHandler.writeForbidden(responseWriter, request)
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	columnName := controlPlaneHandler.extractColumnName(request)
	if schema == "" || table == "" || columnName == "" {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/columns/{column}")
		return
	}
	var alterColumnRequest AlterColumnRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&alterColumnRequest); decodeErr != nil {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if alterErr := controlPlaneHandler.ddlEngine.AlterColumn(request.Context(), schema, table, columnName, alterColumnRequest); alterErr != nil {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, alterErr.Error())
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

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, AlterColumnResponse{
		Status: "updated",
		Column: columnName,
	})
}

// HandleDropColumn drops a column from a table.
func (controlPlaneHandler *ControlPlaneHandler) HandleDropColumn(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		controlPlaneHandler.writeForbidden(responseWriter, request)
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	columnName := controlPlaneHandler.extractColumnName(request)
	if schema == "" || table == "" || columnName == "" {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/columns/{column}")
		return
	}
	cascade := request.URL.Query().Get("cascade") == "true"
	if dropErr := controlPlaneHandler.ddlEngine.DropColumn(request.Context(), schema, table, columnName, cascade); dropErr != nil {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, dropErr.Error())
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

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, DropColumnResponse{
		Status: "deleted",
		Column: columnName,
	})
}
