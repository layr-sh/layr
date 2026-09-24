package core

import (
	"net/http"
	"strconv"
	"uuid"
)

func (server *Server) handleListEvents(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list events request")
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

	events, err := server.kernel.eventManager.List(request.Context(), eventFilter)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	log.Debugf("retrieved %d event(s)", len(events))
	WriteJSONResponse(responseWriter, http.StatusOK, events)
}

func (server *Server) handleGetEvent(responseWriter http.ResponseWriter, request *http.Request) {
	eventParam := request.PathValue("event_id")
	log.Tracef("handling get event request: id=%s", eventParam)
	eventID, err := uuid.Parse(eventParam)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
		return
	}
	event, err := server.kernel.eventManager.Get(request.Context(), eventID)
	if err != nil {
		WriteErrorResponse(responseWriter, request, http.StatusNotFound, err.Error())
		return
	}
	log.Debugf("retrieved event %s", event.ID)
	WriteJSONResponse(responseWriter, http.StatusOK, event)
}
