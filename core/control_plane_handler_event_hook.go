package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"uuid"
)

func (server *Server) handleListEventHooks(responseWriter http.ResponseWriter, request *http.Request) {
	queryValues := request.URL.Query()
	eventHookFilter := EventHookFilter{}
	if driverQueryParam := queryValues.Get("driver"); driverQueryParam != "" {
		eventHookFilter.Driver = &driverQueryParam
	}
	if isEnabledQueryParam := queryValues.Get("is_enabled"); isEnabledQueryParam != "" {
		parsedIsEnabled := isEnabledQueryParam == "true" || isEnabledQueryParam == "1"
		eventHookFilter.IsEnabled = &parsedIsEnabled
	}

	eventHooks, err := server.kernel.eventHookManager.List(request.Context(), eventHookFilter)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSONResponse(responseWriter, http.StatusOK, eventHooks)
}

func (server *Server) handleCreateEventHook(responseWriter http.ResponseWriter, request *http.Request) {
	var createEventHookInput CreateEventHookInput
	if err := json.NewDecoder(request.Body).Decode(&createEventHookInput); err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	eventHook, err := server.kernel.eventHookManager.Create(request.Context(), createEventHookInput)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	hookResourceID := eventHook.ID.String()
	server.kernel.eventBus.Publish(request.Context(), NewEventHookCreatedEvent(hookResourceID, EventHookCreatedEventData(*eventHook)))
	WriteJSONResponse(responseWriter, http.StatusCreated, eventHook)
}

func (server *Server) handleGetEventHook(responseWriter http.ResponseWriter, request *http.Request) {
	hookID, err := uuid.Parse(request.PathValue("event_hook_id"))
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	eventHook, err := server.kernel.eventHookManager.Get(request.Context(), hookID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	WriteJSONResponse(responseWriter, http.StatusOK, eventHook)
}

func (server *Server) handleUpdateEventHook(responseWriter http.ResponseWriter, request *http.Request) {
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
	eventHook, err := server.kernel.eventHookManager.Update(request.Context(), hookID, updateEventHookInput)
	if err != nil {
		if errors.Is(err, ErrEventHookNotFound) {
			WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
			return
		}
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	hookResourceID := eventHook.ID.String()
	server.kernel.eventBus.Publish(request.Context(), NewEventHookUpdatedEvent(hookResourceID, EventHookUpdatedEventData(*eventHook)))
	WriteJSONResponse(responseWriter, http.StatusOK, eventHook)
}

func (server *Server) handleDeleteEventHook(responseWriter http.ResponseWriter, request *http.Request) {
	hookID, err := uuid.Parse(request.PathValue("event_hook_id"))
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	eventHook, err := server.kernel.eventHookManager.Get(request.Context(), hookID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	_ = server.kernel.eventHookManager.Delete(request.Context(), hookID)
	hookResourceID := hookID.String()
	server.kernel.eventBus.Publish(request.Context(), NewEventHookDeletedEvent(hookResourceID, EventHookDeletedEventData(*eventHook)))
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (server *Server) handleListEventHookDeliveries(responseWriter http.ResponseWriter, request *http.Request) {
	hookID, err := uuid.Parse(request.PathValue("event_hook_id"))
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	deliveries, err := server.kernel.eventHookManager.ListDeliveries(request.Context(), hookID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSONResponse(responseWriter, http.StatusOK, deliveries)
}

func (server *Server) handleRetryEventHookDelivery(responseWriter http.ResponseWriter, request *http.Request) {
	deliveryID, err := uuid.Parse(request.PathValue("delivery_id"))
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	eventHookDelivery, err := server.kernel.eventHookManager.RetryDelivery(request.Context(), deliveryID)
	if err != nil {
		if errors.Is(err, ErrEventHookNotFound) || errors.Is(err, ErrEventHookDeliveryNotFound) {
			WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
			return
		}
		WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	hookResourceID := eventHookDelivery.EventHookID.String()
	server.kernel.eventBus.Publish(request.Context(), NewEventHookDeliveryRetriedEvent(hookResourceID, EventHookDeliveryRetriedEventData(*eventHookDelivery)))
	WriteJSONResponse(responseWriter, http.StatusOK, eventHookDelivery)
}
