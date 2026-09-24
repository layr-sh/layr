package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"layr.sh/core"
	datakv "layr.sh/data/kv"
)

const (
	maxKVKeyLength    = 512
	maxKVPayloadBytes = 2 * 1024 * 1024 // 2 MB
)

func validateKVKey(cacheKey string) (bool, string) {
	if cacheKey == "" {
		return false, "Missing key in path"
	}
	if len(cacheKey) > maxKVKeyLength {
		return false, "Key length exceeds maximum of 512 bytes"
	}
	if cacheKey == "mget" || cacheKey == "mset" || cacheKey == "increment" {
		return false, "'mget', 'mset', and 'increment' are reserved key names"
	}
	return true, ""
}

// handleGetKV handles GET /v1/data/kv/{key...}.
func (handler *BaseHandler) handleGetKV(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	cacheKey := handler.extractKVKey(request)
	log.Tracef("handling get KV key request: %s", cacheKey)
	if valid, errorMsg := validateKVKey(cacheKey); !valid {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, errorMsg)
		return
	}
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.executeGetKV(responseWriter, request, authContext, cacheKey)
}

// handleSetKV handles POST /v1/data/kv/{key...}.
func (handler *BaseHandler) handleSetKV(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	cacheKey := handler.extractKVKey(request)
	log.Tracef("handling set KV key request: %s", cacheKey)
	if valid, errorMsg := validateKVKey(cacheKey); !valid {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, errorMsg)
		return
	}
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.executePostKV(responseWriter, request, authContext, cacheKey)
}

// handleUpdateKV handles PUT /v1/data/kv/{key...}.
func (handler *BaseHandler) handleUpdateKV(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	cacheKey := handler.extractKVKey(request)
	log.Tracef("handling update KV key request: %s", cacheKey)
	if valid, errorMsg := validateKVKey(cacheKey); !valid {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, errorMsg)
		return
	}
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.executePutKV(responseWriter, request, authContext, cacheKey)
}

// handleTouchKV handles PATCH /v1/data/kv/{key...}.
func (handler *BaseHandler) handleTouchKV(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	cacheKey := handler.extractKVKey(request)
	log.Tracef("handling touch KV key request: %s", cacheKey)
	if valid, errorMsg := validateKVKey(cacheKey); !valid {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, errorMsg)
		return
	}
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.executePatchKV(responseWriter, request, authContext, cacheKey)
}

// handleDeleteKV handles DELETE /v1/data/kv/{key...}.
func (handler *BaseHandler) handleDeleteKV(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	cacheKey := handler.extractKVKey(request)
	log.Tracef("handling delete KV key request: %s", cacheKey)
	if valid, errorMsg := validateKVKey(cacheKey); !valid {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, errorMsg)
		return
	}
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.executeDeleteKV(responseWriter, request, authContext, cacheKey)
}

// handleGetMultipleKV handles POST /v1/data/kv/mget.
func (handler *BaseHandler) handleGetMultipleKV(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get multiple KV keys request")
	responseWriter.Header().Set("Content-Type", "application/json")
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.executeMGetKV(responseWriter, request, authContext)
}

// handleSetMultipleKV handles POST /v1/data/kv/mset.
func (handler *BaseHandler) handleSetMultipleKV(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling set multiple KV keys request")
	responseWriter.Header().Set("Content-Type", "application/json")
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.executeMSetKV(responseWriter, request, authContext)
}

// handleIncrementKV handles POST /v1/data/kv/increment.
func (handler *BaseHandler) handleIncrementKV(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling increment KV key request")
	responseWriter.Header().Set("Content-Type", "application/json")
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.executeIncrementKV(responseWriter, request, authContext)
}

func (handler *BaseHandler) extractKVKey(request *http.Request) string {
	cacheKey := request.PathValue("key")
	if cacheKey == "" {
		cacheKey = request.PathValue("cache_key")
	}
	if cacheKey != "" {
		return strings.Trim(cacheKey, "/")
	}
	trimmed := strings.TrimPrefix(request.URL.Path, "/v1/data/kv")
	return strings.Trim(trimmed, "/")
}

func (handler *BaseHandler) executeGetKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext, cacheKey string) {
	internalKey := datakv.BuildInternalKey(authContext, cacheKey)
	storedValue, err := handler.kernel.KVStore().Get(request.Context(), internalKey)
	if err != nil {
		if errors.Is(err, core.ErrKVStoreKeyNotFound) || err.Error() == "key not found" {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Key not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("kv get failed for key %q: %v", cacheKey, err))
		return
	}

	log.Debugf("retrieved kv key %s", cacheKey)
	core.WriteJSONResponse(responseWriter, http.StatusOK, GetKVResponse{
		Key:   cacheKey,
		Value: storedValue,
	})
}

func (handler *BaseHandler) executePostKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext, cacheKey string) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxKVPayloadBytes)
	bodyBytes, readErr := io.ReadAll(request.Body)
	if readErr != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(readErr, &maxBytesError) {
			core.WriteErrorResponse(responseWriter, request, http.StatusRequestEntityTooLarge, "Request body exceeds size limit")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or empty request body")
		return
	}
	if len(bodyBytes) == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or empty request body")
		return
	}

	var rawMap map[string]json.RawMessage
	if unmarshalMapErr := json.Unmarshal(bodyBytes, &rawMap); unmarshalMapErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}
	if _, hasValue := rawMap["value"]; !hasValue {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Missing required 'value' property")
		return
	}

	var setKVInput SetKVInput
	if decodeErr := json.Unmarshal(bodyBytes, &setKVInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	ttlSeconds := setKVInput.TTL
	if ttlSeconds < 0 {
		ttlSeconds = 0
	}

	internalKey := datakv.BuildInternalKey(authContext, cacheKey)
	isNX := request.URL.Query().Get("nx") == "true" || request.Header.Get("If-None-Match") == "*"
	if isNX {
		created, setNXErr := handler.kernel.KVStore().SetNX(request.Context(), internalKey, setKVInput.Value, time.Duration(ttlSeconds)*time.Second)
		if setNXErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("kv post (nx) failed for key %q: %v", cacheKey, setNXErr))
			return
		}
		if !created {
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Key already exists")
			return
		}
		handler.kernel.EventBus().Publish(request.Context(), NewKVSetEvent(cacheKey, KVSetEventData{
			Key: cacheKey,
			TTL: ttlSeconds,
		}))
		log.Debugf("set kv key %s (created=true, ttl=%d)", cacheKey, ttlSeconds)
		core.WriteJSONResponse(responseWriter, http.StatusCreated, SetKVResponse{
			Key:     cacheKey,
			Status:  "ok",
			Created: true,
			TTL:     ttlSeconds,
		})
		return
	}

	setErr := handler.kernel.KVStore().Set(request.Context(), internalKey, setKVInput.Value, time.Duration(ttlSeconds)*time.Second)
	if setErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("kv post failed for key %q: %v", cacheKey, setErr))
		return
	}

	handler.kernel.EventBus().Publish(request.Context(), NewKVSetEvent(cacheKey, KVSetEventData{
		Key: cacheKey,
		TTL: ttlSeconds,
	}))
	log.Debugf("set kv key %s (ttl=%d)", cacheKey, ttlSeconds)
	core.WriteJSONResponse(responseWriter, http.StatusOK, SetKVResponse{
		Key:    cacheKey,
		Status: "ok",
		TTL:    ttlSeconds,
	})
}

func (handler *BaseHandler) executePutKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext, cacheKey string) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxKVPayloadBytes)
	bodyBytes, readErr := io.ReadAll(request.Body)
	if readErr != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(readErr, &maxBytesError) {
			core.WriteErrorResponse(responseWriter, request, http.StatusRequestEntityTooLarge, "Request body exceeds size limit")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or empty request body")
		return
	}
	if len(bodyBytes) == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid or empty request body")
		return
	}

	valueToStore := string(bodyBytes)
	ttlSeconds := 0
	if rawTTL := request.URL.Query().Get("ttl"); rawTTL != "" {
		if parsedTTL, parseErr := strconv.Atoi(rawTTL); parseErr == nil && parsedTTL > 0 {
			ttlSeconds = parsedTTL
		}
	} else if ttlHeader := request.Header.Get("X-Layr-Cache-TTL"); ttlHeader != "" {
		if parsedTTL, parseErr := strconv.Atoi(ttlHeader); parseErr == nil && parsedTTL > 0 {
			ttlSeconds = parsedTTL
		}
	}

	internalKey := datakv.BuildInternalKey(authContext, cacheKey)
	isNX := request.URL.Query().Get("nx") == "true" || request.Header.Get("If-None-Match") == "*"
	if isNX {
		created, setNXErr := handler.kernel.KVStore().SetNX(request.Context(), internalKey, valueToStore, time.Duration(ttlSeconds)*time.Second)
		if setNXErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("kv put (nx) failed for key %q: %v", cacheKey, setNXErr))
			return
		}
		if !created {
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, "Key already exists")
			return
		}
		handler.kernel.EventBus().Publish(request.Context(), NewKVSetEvent(cacheKey, KVSetEventData{
			Key: cacheKey,
			TTL: ttlSeconds,
		}))
		log.Debugf("put kv key %s (created=true, ttl=%d)", cacheKey, ttlSeconds)
		core.WriteJSONResponse(responseWriter, http.StatusCreated, SetKVResponse{
			Key:     cacheKey,
			Status:  "ok",
			Created: true,
			TTL:     ttlSeconds,
		})
		return
	}

	setErr := handler.kernel.KVStore().Set(request.Context(), internalKey, valueToStore, time.Duration(ttlSeconds)*time.Second)
	if setErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("kv put failed for key %q: %v", cacheKey, setErr))
		return
	}

	handler.kernel.EventBus().Publish(request.Context(), NewKVSetEvent(cacheKey, KVSetEventData{
		Key: cacheKey,
		TTL: ttlSeconds,
	}))
	log.Debugf("put kv key %s (ttl=%d)", cacheKey, ttlSeconds)
	core.WriteJSONResponse(responseWriter, http.StatusOK, SetKVResponse{
		Key:    cacheKey,
		Status: "ok",
		TTL:    ttlSeconds,
	})
}

func (handler *BaseHandler) executePatchKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext, cacheKey string) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxKVPayloadBytes)
	var touchKVInput TouchKVInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&touchKVInput); decodeErr != nil || touchKVInput.TTL <= 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid expiration value")
		return
	}

	internalKey := datakv.BuildInternalKey(authContext, cacheKey)
	expireErr := handler.kernel.KVStore().Expire(request.Context(), internalKey, time.Duration(touchKVInput.TTL)*time.Second)
	if expireErr != nil {
		if errors.Is(expireErr, core.ErrKVStoreKeyNotFound) || expireErr.Error() == "key not found" {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, "Key not found")
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("kv patch failed for key %q: %v", cacheKey, expireErr))
		return
	}

	handler.kernel.EventBus().Publish(request.Context(), NewKVTouchedEvent(cacheKey, KVTouchedEventData{
		Key: cacheKey,
		TTL: touchKVInput.TTL,
	}))
	log.Debugf("touched kv key %s (ttl=%d)", cacheKey, touchKVInput.TTL)
	core.WriteJSONResponse(responseWriter, http.StatusOK, TouchKVResponse{
		Key:    cacheKey,
		Status: "ok",
		TTL:    touchKVInput.TTL,
	})
}

func (handler *BaseHandler) executeDeleteKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext, cacheKey string) {
	internalKey := datakv.BuildInternalKey(authContext, cacheKey)
	_ = handler.kernel.KVStore().Delete(request.Context(), internalKey)
	handler.kernel.EventBus().Publish(request.Context(), NewKVDeletedEvent(cacheKey, KVDeletedEventData{
		Key: cacheKey,
	}))
	log.Debugf("deleted kv key %s", cacheKey)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *BaseHandler) executeMGetKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxKVPayloadBytes)
	var getMultipleKVInput GetMultipleKVInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&getMultipleKVInput); decodeErr != nil || len(getMultipleKVInput.Keys) == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Keys are required")
		return
	}

	internalKeys := make([]string, 0, len(getMultipleKVInput.Keys))
	keyLookup := make(map[string]string, len(getMultipleKVInput.Keys))
	for _, key := range getMultipleKVInput.Keys {
		if key == "" || len(key) > maxKVKeyLength || key == "mget" || key == "mset" || key == "increment" {
			continue
		}
		internalKey := datakv.BuildInternalKey(authContext, key)
		internalKeys = append(internalKeys, internalKey)
		keyLookup[internalKey] = key
	}

	result := make(map[string]string)
	if len(internalKeys) > 0 {
		mgetResults, mgetErr := handler.kernel.KVStore().MGet(request.Context(), internalKeys)
		if mgetErr == nil {
			for internalKey, storedValue := range mgetResults {
				if userKey, ok := keyLookup[internalKey]; ok {
					result[userKey] = storedValue
				}
			}
		}
	}

	log.Debugf("retrieved %d kv key(s)", len(result))
	core.WriteJSONResponse(responseWriter, http.StatusOK, GetMultipleKVResponse{
		Values: result,
	})
}

func (handler *BaseHandler) executeMSetKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxKVPayloadBytes)
	var setMultipleKVInput SetMultipleKVInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&setMultipleKVInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}
	if len(setMultipleKVInput.Entries) == 0 {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Entries cannot be empty")
		return
	}

	ttlSeconds := 0
	if setMultipleKVInput.TTL > 0 {
		ttlSeconds = setMultipleKVInput.TTL
	}

	internalEntries := make(map[string]string, len(setMultipleKVInput.Entries))
	for userKey, userValue := range setMultipleKVInput.Entries {
		if valid, errorMsg := validateKVKey(userKey); !valid {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Invalid key %q: %s", userKey, errorMsg))
			return
		}
		internalKey := datakv.BuildInternalKey(authContext, userKey)
		internalEntries[internalKey] = userValue
	}

	msetErr := handler.kernel.KVStore().MSet(request.Context(), internalEntries, time.Duration(ttlSeconds)*time.Second)
	if msetErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("kv mset failed: %v", msetErr))
		return
	}

	for userKey := range setMultipleKVInput.Entries {
		handler.kernel.EventBus().Publish(request.Context(), NewKVSetEvent(userKey, KVSetEventData{
			Key: userKey,
			TTL: ttlSeconds,
		}))
	}
	log.Debugf("set %d kv key(s)", len(internalEntries))
	core.WriteJSONResponse(responseWriter, http.StatusOK, SetMultipleKVResponse{
		Status: "ok",
		Count:  len(internalEntries),
	})
}

func (handler *BaseHandler) executeIncrementKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxKVPayloadBytes)
	var incrementKVInput IncrementKVInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&incrementKVInput); decodeErr != nil || incrementKVInput.Key == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Key is required")
		return
	}

	if valid, errorMsg := validateKVKey(incrementKVInput.Key); !valid {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, errorMsg)
		return
	}

	ttlSeconds := 0
	if incrementKVInput.TTL > 0 {
		ttlSeconds = incrementKVInput.TTL
	}

	delta := int64(1)
	if incrementKVInput.Step != nil {
		delta = *incrementKVInput.Step
	}

	internalKey := datakv.BuildInternalKey(authContext, incrementKVInput.Key)
	counterValue, incrementErr := handler.kernel.KVStore().IncrementBy(request.Context(), internalKey, delta, time.Duration(ttlSeconds)*time.Second)
	if incrementErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("kv increment failed for key %q: %v", incrementKVInput.Key, incrementErr))
		return
	}

	handler.kernel.EventBus().Publish(request.Context(), NewKVSetEvent(incrementKVInput.Key, KVSetEventData{
		Key: incrementKVInput.Key,
		TTL: ttlSeconds,
	}))
	log.Debugf("incremented kv key %s to %d", incrementKVInput.Key, counterValue)
	core.WriteJSONResponse(responseWriter, http.StatusOK, IncrementKVResponse{
		Key:   incrementKVInput.Key,
		Value: counterValue,
	})
}
