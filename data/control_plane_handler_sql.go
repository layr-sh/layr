package data

import (
	"encoding/json"
	"net/http"
	"strings"
)

// HandleExecuteSQL executes raw SQL from the console scratchpad.
func (controlPlaneHandler *ControlPlaneHandler) HandleExecuteSQL(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		controlPlaneHandler.writeForbidden(responseWriter, request)
		return
	}

	var executeSQLRequest ExecuteSQLRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&executeSQLRequest); decodeErr != nil {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid or empty SQL query")
		return
	}

	rawSQL := strings.TrimSpace(executeSQLRequest.SQL)
	if rawSQL == "" {
		rawSQL = strings.TrimSpace(executeSQLRequest.Query)
	}
	if rawSQL == "" {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid or empty SQL query")
		return
	}

	executeSQLResponse, err := controlPlaneHandler.ddlEngine.ExecuteSQL(request.Context(), rawSQL)
	if err != nil {
		controlPlaneHandler.writeError(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}

	controlPlaneHandler.invalidateCache(request.Context())
	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, executeSQLResponse)
}
