package core

import (
	"encoding/json"
	"errors"
	"net/http"
)

func (server *Server) handleListServiceAccounts(responseWriter http.ResponseWriter, request *http.Request) {
	accounts, err := server.kernel.serviceAccountManager.List(request.Context())
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSONResponse(responseWriter, http.StatusOK, accounts)
}

func (server *Server) handleCreateServiceAccount(responseWriter http.ResponseWriter, request *http.Request) {
	var createServiceAccountInput CreateServiceAccountInput
	if err := json.NewDecoder(request.Body).Decode(&createServiceAccountInput); err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	serviceAccount, err := server.kernel.serviceAccountManager.Create(request.Context(), createServiceAccountInput)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	server.kernel.eventBus.Publish(request.Context(), NewServiceAccountCreatedEvent(serviceAccount.ID, ServiceAccountCreatedEventData(serviceAccount.ServiceAccount)))
	WriteJSONResponse(responseWriter, http.StatusCreated, serviceAccount)
}

func (server *Server) handleGetServiceAccount(responseWriter http.ResponseWriter, request *http.Request) {
	serviceAccountID := request.PathValue("service_account_id")
	serviceAccount, err := server.kernel.serviceAccountManager.Get(request.Context(), serviceAccountID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	WriteJSONResponse(responseWriter, http.StatusOK, serviceAccount)
}

func (server *Server) handleUpdateServiceAccount(responseWriter http.ResponseWriter, request *http.Request) {
	serviceAccountID := request.PathValue("service_account_id")
	var updateServiceAccountInput UpdateServiceAccountInput
	if err := json.NewDecoder(request.Body).Decode(&updateServiceAccountInput); err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	serviceAccount, err := server.kernel.serviceAccountManager.Update(request.Context(), serviceAccountID, updateServiceAccountInput)
	if err != nil {
		if errors.Is(err, ErrServiceAccountNotFound) {
			WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
			return
		}
		WriteErrorResponse(responseWriter, request, http.StatusForbidden, err.Error())
		return
	}
	server.kernel.eventBus.Publish(request.Context(), NewServiceAccountUpdatedEvent(serviceAccount.ID, ServiceAccountUpdatedEventData(*serviceAccount)))
	WriteJSONResponse(responseWriter, http.StatusOK, serviceAccount)
}

func (server *Server) handleDeleteServiceAccount(responseWriter http.ResponseWriter, request *http.Request) {
	serviceAccountID := request.PathValue("service_account_id")
	serviceAccount, err := server.kernel.serviceAccountManager.Get(request.Context(), serviceAccountID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	err = server.kernel.serviceAccountManager.Delete(request.Context(), serviceAccountID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusForbidden, err.Error())
		return
	}
	server.kernel.eventBus.Publish(request.Context(), NewServiceAccountDeletedEvent(serviceAccountID, ServiceAccountDeletedEventData(*serviceAccount)))
	responseWriter.WriteHeader(http.StatusNoContent)
}
