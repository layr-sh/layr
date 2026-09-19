package data

import (
	"encoding/json"
	"fmt"
	"net/http"

	"layr.sh/core"
)

// HandleListTables lists all tables in the allowed schemas.
func (controlPlaneHandler *ControlPlaneHandler) HandleListTables(responseWriter http.ResponseWriter, request *http.Request) {
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

// HandleCreateTable creates a new database table.
func (controlPlaneHandler *ControlPlaneHandler) HandleCreateTable(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	var createTableRequest CreateTableRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&createTableRequest); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON request body")
		return
	}
	if createErr := controlPlaneHandler.ddlEngine.CreateTable(request.Context(), createTableRequest); createErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, createErr.Error())
		return
	}
	controlPlaneHandler.invalidateCache(request.Context())
	controlPlaneHandler.invalidateTableCache(request.Context(), createTableRequest.Schema, createTableRequest.Name)

	if controlPlaneHandler.eventBus != nil {
		tableID := fmt.Sprintf("%s.%s", createTableRequest.Schema, createTableRequest.Name)
		controlPlaneHandler.eventBus.Publish(request.Context(), NewTableCreatedEvent(tableID, TableCreatedEventData{
			Schema:      createTableRequest.Schema,
			Table:       createTableRequest.Name,
			Columns:     createTableRequest.Columns,
			ForeignKeys: createTableRequest.ForeignKeys,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusCreated, CreateTableResponse{
		Status:  "created",
		Schema:  createTableRequest.Schema,
		Table:   createTableRequest.Name,
		Message: fmt.Sprintf("Table %s.%s successfully created", createTableRequest.Schema, createTableRequest.Name),
	})
}

// HandleGetTable returns the schema summary for a specific table.
func (controlPlaneHandler *ControlPlaneHandler) HandleGetTable(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.read") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}")
		return
	}
	tableSummary, err := controlPlaneHandler.ddlEngine.GetTable(request.Context(), schema, table)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, tableSummary)
}

// HandleDropTable drops a table.
func (controlPlaneHandler *ControlPlaneHandler) HandleDropTable(responseWriter http.ResponseWriter, request *http.Request) {
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

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, DropTableResponse{
		Status:  "deleted",
		Schema:  schema,
		Table:   table,
		Message: fmt.Sprintf("Table %s.%s dropped", schema, table),
	})
}

// HandleTruncateTable truncates all rows in a table.
func (controlPlaneHandler *ControlPlaneHandler) HandleTruncateTable(responseWriter http.ResponseWriter, request *http.Request) {
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
