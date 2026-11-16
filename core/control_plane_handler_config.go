package core

import (
	"encoding/json"
	"errors"
	"net/http"
)

// handleGetConfig handles GET /v1/_/core/config.
func (server *Server) handleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get core configuration request")
	if !server.kernel.serviceAccountManager.RequireScope(responseWriter, request, ScopeCoreConfigRead) {
		return
	}
	WriteJSONResponse(responseWriter, http.StatusOK, server.kernel.configManager.Get())
}

// handleUpdateConfig handles PUT /v1/_/core/config.
func (server *Server) handleUpdateConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling update core configuration request")
	if !server.kernel.serviceAccountManager.RequireScope(responseWriter, request, ScopeCoreConfigWrite) {
		return
	}
	runtimeConfig := server.kernel.configManager.Get()
	if err := json.NewDecoder(request.Body).Decode(&runtimeConfig); err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if err := server.kernel.configManager.Set(request.Context(), runtimeConfig); err != nil {
		if errors.Is(err, ErrInvalidConfig) {
			WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
			return
		}
		WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSONResponse(responseWriter, http.StatusOK, server.kernel.configManager.Get())
}
