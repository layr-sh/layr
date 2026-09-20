package data

import (
	"encoding/json"
	"net/http"
	"strings"

	"layr.sh/core"
)

// handleExecuteSQL executes raw SQL from the console scratchpad.
func (controlPlaneHandler *ControlPlaneHandler) handleExecuteSQL(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		core.WriteErrorResponse(responseWriter, request, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
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

	controlPlaneHandler.invalidateCache(request.Context())
	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, executeSQLResponse)
}
