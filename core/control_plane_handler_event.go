package core

import (
	"net/http"
	"strconv"
	"uuid"
)

func (kernel *Kernel) handleListEvents(responseWriter http.ResponseWriter, request *http.Request) {
	queryValues := request.URL.Query()
	eventFilter := EventFilter{}
	if typeQueryParam := queryValues.Get("type"); typeQueryParam != "" {
		eventFilter.Type = &typeQueryParam
	}
	if actorTypeQueryParam := queryValues.Get("actor_type"); actorTypeQueryParam != "" {
		eventFilter.ActorType = &actorTypeQueryParam
	}
	if actorIDQueryParam := queryValues.Get("actor_id"); actorIDQueryParam != "" {
		if parsedActorID, parseErr := uuid.Parse(actorIDQueryParam); parseErr == nil {
			eventFilter.ActorID = &parsedActorID
		}
	}
	if resourceTypeQueryParam := queryValues.Get("resource_type"); resourceTypeQueryParam != "" {
		eventFilter.ResourceType = &resourceTypeQueryParam
	}
	if resourceIDQueryParam := queryValues.Get("resource_id"); resourceIDQueryParam != "" {
		eventFilter.ResourceID = &resourceIDQueryParam
	}
	if limitQueryParam := queryValues.Get("limit"); limitQueryParam != "" {
		if parsedLimit, parseErr := strconv.Atoi(limitQueryParam); parseErr == nil {
			eventFilter.Limit = parsedLimit
		}
	}
	if offsetQueryParam := queryValues.Get("offset"); offsetQueryParam != "" {
		if parsedOffset, parseErr := strconv.Atoi(offsetQueryParam); parseErr == nil {
			eventFilter.Offset = parsedOffset
		}
	}

	events, err := kernel.eventManager.List(request.Context(), eventFilter)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	kernel.writeJSON(responseWriter, events)
}

func (kernel *Kernel) handleGetEvent(responseWriter http.ResponseWriter, request *http.Request) {
	eventID, err := uuid.Parse(request.PathValue("event_id"))
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	event, err := kernel.eventManager.Get(request.Context(), eventID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	kernel.writeJSON(responseWriter, event)
}
