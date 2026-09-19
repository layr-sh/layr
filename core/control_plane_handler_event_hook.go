package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"uuid"
)

func (kernel *Kernel) handleListEventHooks(responseWriter http.ResponseWriter, request *http.Request) {
	queryValues := request.URL.Query()
	eventHookFilter := EventHookFilter{}
	if driverQueryParam := queryValues.Get("driver"); driverQueryParam != "" {
		eventHookFilter.Driver = &driverQueryParam
	}
	if isEnabledQueryParam := queryValues.Get("is_enabled"); isEnabledQueryParam != "" {
		parsedIsEnabled := isEnabledQueryParam == "true" || isEnabledQueryParam == "1"
		eventHookFilter.IsEnabled = &parsedIsEnabled
	}

	eventHooks, err := kernel.eventHookManager.List(request.Context(), eventHookFilter)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	kernel.writeJSON(responseWriter, eventHooks)
}

func (kernel *Kernel) handleCreateEventHook(responseWriter http.ResponseWriter, request *http.Request) {
	var createEventHookInput CreateEventHookInput
	if err := json.NewDecoder(request.Body).Decode(&createEventHookInput); err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	eventHook, err := kernel.eventHookManager.Create(request.Context(), createEventHookInput)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	if kernel.eventBus != nil {
		hookResourceID := eventHook.ID.String()
		kernel.eventBus.Publish(request.Context(), NewEventHookCreatedEvent(hookResourceID, EventHookCreatedEventData(*eventHook)))
	}
	kernel.writeJSONWithStatus(responseWriter, http.StatusCreated, eventHook)
}

func (kernel *Kernel) handleGetEventHook(responseWriter http.ResponseWriter, request *http.Request) {
	hookID, err := uuid.Parse(request.PathValue("event_hook_id"))
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	eventHook, err := kernel.eventHookManager.Get(request.Context(), hookID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	kernel.writeJSON(responseWriter, eventHook)
}

func (kernel *Kernel) handleUpdateEventHook(responseWriter http.ResponseWriter, request *http.Request) {
	hookID, err := uuid.Parse(request.PathValue("event_hook_id"))
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	var updateEventHookInput UpdateEventHookInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&updateEventHookInput); decodeErr != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, decodeErr.Error())
		return
	}
	eventHook, err := kernel.eventHookManager.Update(request.Context(), hookID, updateEventHookInput)
	if err != nil {
		if errors.Is(err, ErrEventHookNotFound) {
			WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
			return
		}
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	if kernel.eventBus != nil {
		hookResourceID := eventHook.ID.String()
		kernel.eventBus.Publish(request.Context(), NewEventHookUpdatedEvent(hookResourceID, EventHookUpdatedEventData(*eventHook)))
	}
	kernel.writeJSON(responseWriter, eventHook)
}

func (kernel *Kernel) handleDeleteEventHook(responseWriter http.ResponseWriter, request *http.Request) {
	hookID, err := uuid.Parse(request.PathValue("event_hook_id"))
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	eventHook, err := kernel.eventHookManager.Get(request.Context(), hookID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	_ = kernel.eventHookManager.Delete(request.Context(), hookID)
	if kernel.eventBus != nil {
		hookResourceID := hookID.String()
		kernel.eventBus.Publish(request.Context(), NewEventHookDeletedEvent(hookResourceID, EventHookDeletedEventData(*eventHook)))
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (kernel *Kernel) handleListEventHookDeliveries(responseWriter http.ResponseWriter, request *http.Request) {
	hookID, err := uuid.Parse(request.PathValue("event_hook_id"))
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	deliveries, err := kernel.eventHookManager.ListDeliveries(request.Context(), hookID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	kernel.writeJSON(responseWriter, deliveries)
}

func (kernel *Kernel) handleRetryEventHookDelivery(responseWriter http.ResponseWriter, request *http.Request) {
	deliveryID, err := uuid.Parse(request.PathValue("delivery_id"))
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	eventHookDelivery, err := kernel.eventHookManager.RetryDelivery(request.Context(), deliveryID)
	if err != nil {
		if errors.Is(err, ErrEventHookNotFound) || errors.Is(err, ErrEventHookDeliveryNotFound) {
			WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
			return
		}
		WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	kernel.writeJSON(responseWriter, eventHookDelivery)
}
