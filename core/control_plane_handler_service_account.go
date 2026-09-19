package core

import (
	"encoding/json"
	"errors"
	"net/http"
)

func (kernel *Kernel) handleListServiceAccounts(responseWriter http.ResponseWriter, request *http.Request) {
	accounts, err := kernel.serviceAccountManager.List(request.Context())
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	kernel.writeJSON(responseWriter, accounts)
}

func (kernel *Kernel) handleCreateServiceAccount(responseWriter http.ResponseWriter, request *http.Request) {
	var createServiceAccountInput CreateServiceAccountInput
	if err := json.NewDecoder(request.Body).Decode(&createServiceAccountInput); err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	serviceAccount, err := kernel.serviceAccountManager.Create(request.Context(), createServiceAccountInput)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	if kernel.eventBus != nil {
		kernel.eventBus.Publish(request.Context(), NewServiceAccountCreatedEvent(serviceAccount.ID, ServiceAccountCreatedEventData(serviceAccount.ServiceAccount)))
	}
	kernel.writeJSONWithStatus(responseWriter, http.StatusCreated, serviceAccount)
}

func (kernel *Kernel) handleGetServiceAccount(responseWriter http.ResponseWriter, request *http.Request) {
	serviceAccountID := request.PathValue("service_account_id")
	serviceAccount, err := kernel.serviceAccountManager.Get(request.Context(), serviceAccountID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	kernel.writeJSON(responseWriter, serviceAccount)
}

func (kernel *Kernel) handleUpdateServiceAccount(responseWriter http.ResponseWriter, request *http.Request) {
	serviceAccountID := request.PathValue("service_account_id")
	var updateServiceAccountInput UpdateServiceAccountInput
	if err := json.NewDecoder(request.Body).Decode(&updateServiceAccountInput); err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	serviceAccount, err := kernel.serviceAccountManager.Update(request.Context(), serviceAccountID, updateServiceAccountInput)
	if err != nil {
		if errors.Is(err, ErrServiceAccountNotFound) {
			WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
			return
		}
		WriteErrorResponse(responseWriter, request, http.StatusForbidden, err.Error())
		return
	}
	if kernel.eventBus != nil {
		kernel.eventBus.Publish(request.Context(), NewServiceAccountUpdatedEvent(serviceAccount.ID, ServiceAccountUpdatedEventData(*serviceAccount)))
	}
	kernel.writeJSON(responseWriter, serviceAccount)
}

func (kernel *Kernel) handleDeleteServiceAccount(responseWriter http.ResponseWriter, request *http.Request) {
	serviceAccountID := request.PathValue("service_account_id")
	serviceAccount, err := kernel.serviceAccountManager.Get(request.Context(), serviceAccountID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	err = kernel.serviceAccountManager.Delete(request.Context(), serviceAccountID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusForbidden, err.Error())
		return
	}
	if kernel.eventBus != nil {
		kernel.eventBus.Publish(request.Context(), NewServiceAccountDeletedEvent(serviceAccountID, ServiceAccountDeletedEventData(*serviceAccount)))
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}
