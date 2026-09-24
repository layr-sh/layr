package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"uuid"
)

func (server *Server) handleListEventHooks(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list event hooks request")
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
	log.Debugf("retrieved %d event hook(s)", len(eventHooks))
	WriteJSONResponse(responseWriter, http.StatusOK, eventHooks)
}

func (server *Server) handleCreateEventHook(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling create event hook request")
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
	log.Debugf("event hook %s successfully created", eventHook.ID)
	WriteJSONResponse(responseWriter, http.StatusCreated, eventHook)
}

func (server *Server) handleGetEventHook(responseWriter http.ResponseWriter, request *http.Request) {
	hookParam := request.PathValue("event_hook_id")
	log.Tracef("handling get event hook request: id=%s", hookParam)
	hookID, err := uuid.Parse(hookParam)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	eventHook, err := server.kernel.eventHookManager.Get(request.Context(), hookID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	log.Debugf("retrieved event hook %s", eventHook.ID)
	WriteJSONResponse(responseWriter, http.StatusOK, eventHook)
}

func (server *Server) handleUpdateEventHook(responseWriter http.ResponseWriter, request *http.Request) {
	hookParam := request.PathValue("event_hook_id")
	log.Tracef("handling update event hook request: id=%s", hookParam)
	hookID, err := uuid.Parse(hookParam)
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
	log.Debugf("event hook %s successfully updated", eventHook.ID)
	WriteJSONResponse(responseWriter, http.StatusOK, eventHook)
}

func (server *Server) handleDeleteEventHook(responseWriter http.ResponseWriter, request *http.Request) {
	hookParam := request.PathValue("event_hook_id")
	log.Tracef("handling delete event hook request: id=%s", hookParam)
	hookID, err := uuid.Parse(hookParam)
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
	log.Debugf("event hook %s successfully deleted", hookID)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (server *Server) handleListEventHookDeliveries(responseWriter http.ResponseWriter, request *http.Request) {
	hookParam := request.PathValue("event_hook_id")
	log.Tracef("handling list event hook deliveries request: hook_id=%s", hookParam)
	hookID, err := uuid.Parse(hookParam)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	deliveries, err := server.kernel.eventHookManager.ListDeliveries(request.Context(), hookID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	log.Debugf("retrieved %d delivery attempt(s)", len(deliveries))
	WriteJSONResponse(responseWriter, http.StatusOK, deliveries)
}

func (server *Server) handleRetryEventHookDelivery(responseWriter http.ResponseWriter, request *http.Request) {
	hookParam := request.PathValue("event_hook_id")
	deliveryParam := request.PathValue("delivery_id")
	log.Tracef("handling retry event hook delivery request: hook_id=%s delivery_id=%s", hookParam, deliveryParam)
	deliveryID, err := uuid.Parse(deliveryParam)
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
	log.Debugf("event hook delivery %s successfully retried", deliveryID)
	WriteJSONResponse(responseWriter, http.StatusOK, eventHookDelivery)
}
