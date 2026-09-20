package data

import (
	"encoding/json"
	"fmt"
	"net/http"

	"layr.sh/core"
)

// handleListTables lists all tables in the allowed schemas.
func (controlPlaneHandler *ControlPlaneHandler) handleListTables(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.read") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	schemas := []string{"public"}
	if controlPlaneHandler.configManager != nil {
		schemas = controlPlaneHandler.configManager.Get().Schemas
	}
	tables, err := controlPlaneHandler.ddlEngine.ListTables(request.Context(), schemas)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, ListTablesResponse{
		Tables: tables,
		Count:  len(tables),
	})
}

// handleCreateTable creates a new database table.
func (controlPlaneHandler *ControlPlaneHandler) handleCreateTable(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	var createTableInput CreateTableInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createTableInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON request body")
		return
	}
	if createErr := controlPlaneHandler.ddlEngine.CreateTable(request.Context(), createTableInput); createErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, createErr.Error())
		return
	}
	controlPlaneHandler.invalidateCache(request.Context())
	controlPlaneHandler.invalidateTableCache(request.Context(), createTableInput.Schema, createTableInput.Name)

	if controlPlaneHandler.eventBus != nil {
		tableID := fmt.Sprintf("%s.%s", createTableInput.Schema, createTableInput.Name)
		controlPlaneHandler.eventBus.Publish(request.Context(), NewTableCreatedEvent(tableID, TableCreatedEventData{
			Schema:      createTableInput.Schema,
			Table:       createTableInput.Name,
			Columns:     createTableInput.Columns,
			ForeignKeys: createTableInput.ForeignKeys,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusCreated, CreateTableResponse{
		Status:  "created",
		Schema:  createTableInput.Schema,
		Table:   createTableInput.Name,
		Message: fmt.Sprintf("Table %s.%s successfully created", createTableInput.Schema, createTableInput.Name),
	})
}

// handleGetTable returns the schema summary for a specific table.
func (controlPlaneHandler *ControlPlaneHandler) handleGetTable(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.read") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	schema, tableName := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || tableName == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}")
		return
	}
	table, err := controlPlaneHandler.ddlEngine.GetTable(request.Context(), schema, tableName)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, table)
}

// handleDeleteTable drops a table.
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteTable(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}")
		return
	}
	cascade := request.URL.Query().Get("cascade") == "true"
	if dropErr := controlPlaneHandler.ddlEngine.DropTable(request.Context(), schema, table, cascade); dropErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, dropErr.Error())
		return
	}
	controlPlaneHandler.invalidateCache(request.Context())
	controlPlaneHandler.invalidateTableCache(request.Context(), schema, table)

	if controlPlaneHandler.eventBus != nil {
		tableID := fmt.Sprintf("%s.%s", schema, table)
		controlPlaneHandler.eventBus.Publish(request.Context(), NewTableDeletedEvent(tableID, TableDeletedEventData{
			Schema: schema,
			Table:  table,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, DeleteTableResponse{
		Status:  "deleted",
		Schema:  schema,
		Table:   table,
		Message: fmt.Sprintf("Table %s.%s dropped", schema, table),
	})
}

// handleTruncateTable truncates all rows in a table.
func (controlPlaneHandler *ControlPlaneHandler) handleTruncateTable(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}")
		return
	}
	cascade := request.URL.Query().Get("cascade") == "true"
	if truncateErr := controlPlaneHandler.ddlEngine.TruncateTable(request.Context(), schema, table, cascade); truncateErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, truncateErr.Error())
		return
	}
	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, map[string]any{
		"status":  "truncated",
		"message": fmt.Sprintf("Table %s.%s truncated", schema, table),
	})
}
