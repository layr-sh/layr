package core

import (
	"encoding/json"
	"errors"
	"net/http"
)

func (server *Server) handleListServiceAccounts(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list service accounts request")
	accounts, err := server.kernel.serviceAccountManager.List(request.Context())
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	log.Debugf("retrieved %d service account(s)", len(accounts))
	WriteJSONResponse(responseWriter, http.StatusOK, accounts)
}

func (server *Server) handleCreateServiceAccount(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling create service account request")
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
	log.Debugf("service account %s successfully created", serviceAccount.ID)
	WriteJSONResponse(responseWriter, http.StatusCreated, serviceAccount)
}

func (server *Server) handleGetServiceAccount(responseWriter http.ResponseWriter, request *http.Request) {
	serviceAccountID := request.PathValue("service_account_id")
	log.Tracef("handling get service account request: id=%s", serviceAccountID)
	serviceAccount, err := server.kernel.serviceAccountManager.Get(request.Context(), serviceAccountID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	log.Debugf("retrieved service account %s", serviceAccount.ID)
	WriteJSONResponse(responseWriter, http.StatusOK, serviceAccount)
}

func (server *Server) handleUpdateServiceAccount(responseWriter http.ResponseWriter, request *http.Request) {
	serviceAccountID := request.PathValue("service_account_id")
	log.Tracef("handling update service account request: id=%s", serviceAccountID)
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
	log.Debugf("service account %s successfully updated", serviceAccount.ID)
	WriteJSONResponse(responseWriter, http.StatusOK, serviceAccount)
}

func (server *Server) handleDeleteServiceAccount(responseWriter http.ResponseWriter, request *http.Request) {
	serviceAccountID := request.PathValue("service_account_id")
	log.Tracef("handling delete service account request: id=%s", serviceAccountID)
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
	log.Debugf("service account %s successfully deleted", serviceAccountID)
	responseWriter.WriteHeader(http.StatusNoContent)
}
