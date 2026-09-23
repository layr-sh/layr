package data

import (
	"encoding/json"
	"net/http"
	"strings"

	"layr.sh/core"
)

// handleExecuteSQL executes raw SQL from the console scratchpad.
func (controlPlaneHandler *ControlPlaneHandler) handleExecuteSQL(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handleExecuteSQL invoked")

	if request.Method != http.MethodPost {
		core.WriteErrorResponse(responseWriter, request, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeDataSchemaWrite) {
		return
	}

	var executeSQLInput ExecuteSQLInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&executeSQLInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or empty SQL query")
		return
	}

	rawSQL := strings.TrimSpace(executeSQLInput.SQL)
	if rawSQL == "" {
		rawSQL = strings.TrimSpace(executeSQLInput.Query)
	}
	if rawSQL == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or empty SQL query")
		return
	}

	executeSQLResponse, err := controlPlaneHandler.ddlEngine.ExecuteSQL(request.Context(), rawSQL)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}

	controlPlaneHandler.InvalidateCatalog(request.Context())

	controlPlaneHandler.kernel.EventBus().Publish(request.Context(), NewSQLExecutedEvent("raw_sql", SQLExecutedEventData{
		Query:        rawSQL,
		RowsAffected: executeSQLResponse.RowsAffected,
	}))

	log.Debugf("executed raw SQL query (%d row(s) returned, %d row(s) affected)", len(executeSQLResponse.Rows), executeSQLResponse.RowsAffected)
	core.WriteJSONResponse(responseWriter, http.StatusOK, executeSQLResponse)
}
