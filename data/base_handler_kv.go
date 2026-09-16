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

// HandleGetKV handles GET /api/v1/data/kv/{key...}.
func (handler *BaseHandler) HandleGetKV(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	if handler.kvStore == nil {
		handler.writeError(responseWriter, request, http.StatusServiceUnavailable, "KV store is not available", "LAYR_DATA_005")
		return
	}
	cacheKey := handler.extractKVKey(request)
	if valid, errorMsg := validateKVKey(cacheKey); !valid {
		handler.writeError(responseWriter, request, http.StatusBadRequest, errorMsg, "LAYR_DATA_001")
		return
	}
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.handleGetKV(responseWriter, request, authContext, cacheKey)
}

// HandlePostKV handles POST /api/v1/data/kv/{key...}.
func (handler *BaseHandler) HandlePostKV(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	if handler.kvStore == nil {
		handler.writeError(responseWriter, request, http.StatusServiceUnavailable, "KV store is not available", "LAYR_DATA_005")
		return
	}
	cacheKey := handler.extractKVKey(request)
	if valid, errorMsg := validateKVKey(cacheKey); !valid {
		handler.writeError(responseWriter, request, http.StatusBadRequest, errorMsg, "LAYR_DATA_001")
		return
	}
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.handlePostKV(responseWriter, request, authContext, cacheKey)
}

// HandlePutKV handles PUT /api/v1/data/kv/{key...}.
func (handler *BaseHandler) HandlePutKV(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	if handler.kvStore == nil {
		handler.writeError(responseWriter, request, http.StatusServiceUnavailable, "KV store is not available", "LAYR_DATA_005")
		return
	}
	cacheKey := handler.extractKVKey(request)
	if valid, errorMsg := validateKVKey(cacheKey); !valid {
		handler.writeError(responseWriter, request, http.StatusBadRequest, errorMsg, "LAYR_DATA_001")
		return
	}
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.handlePutKV(responseWriter, request, authContext, cacheKey)
}

// HandleSetKV handles legacy or combined POST/PUT /api/v1/data/kv/{key...}.
func (handler *BaseHandler) HandleSetKV(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPut {
		handler.HandlePutKV(responseWriter, request)
		return
	}
	handler.HandlePostKV(responseWriter, request)
}

// HandlePatchKV handles PATCH /api/v1/data/kv/{key...}.
func (handler *BaseHandler) HandlePatchKV(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	if handler.kvStore == nil {
		handler.writeError(responseWriter, request, http.StatusServiceUnavailable, "KV store is not available", "LAYR_DATA_005")
		return
	}
	cacheKey := handler.extractKVKey(request)
	if valid, errorMsg := validateKVKey(cacheKey); !valid {
		handler.writeError(responseWriter, request, http.StatusBadRequest, errorMsg, "LAYR_DATA_001")
		return
	}
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.handlePatchKV(responseWriter, request, authContext, cacheKey)
}

// HandleDeleteKV handles DELETE /api/v1/data/kv/{key...}.
func (handler *BaseHandler) HandleDeleteKV(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	if handler.kvStore == nil {
		handler.writeError(responseWriter, request, http.StatusServiceUnavailable, "KV store is not available", "LAYR_DATA_005")
		return
	}
	cacheKey := handler.extractKVKey(request)
	if valid, errorMsg := validateKVKey(cacheKey); !valid {
		handler.writeError(responseWriter, request, http.StatusBadRequest, errorMsg, "LAYR_DATA_001")
		return
	}
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.handleDeleteKV(responseWriter, request, authContext, cacheKey)
}

// HandleMGetKV handles POST /api/v1/data/kv/mget.
func (handler *BaseHandler) HandleMGetKV(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	if handler.kvStore == nil {
		handler.writeError(responseWriter, request, http.StatusServiceUnavailable, "KV store is not available", "LAYR_DATA_005")
		return
	}
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.handleMGetKV(responseWriter, request, authContext)
}

// HandleMSetKV handles POST /api/v1/data/kv/mset.
func (handler *BaseHandler) HandleMSetKV(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	if handler.kvStore == nil {
		handler.writeError(responseWriter, request, http.StatusServiceUnavailable, "KV store is not available", "LAYR_DATA_005")
		return
	}
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.handleMSetKV(responseWriter, request, authContext)
}

// HandleIncrementKV handles POST /api/v1/data/kv/increment.
func (handler *BaseHandler) HandleIncrementKV(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	if handler.kvStore == nil {
		handler.writeError(responseWriter, request, http.StatusServiceUnavailable, "KV store is not available", "LAYR_DATA_005")
		return
	}
	authContext := datakv.ExtractAuthContext(request, handler.saltSecret)
	handler.handleIncrementKV(responseWriter, request, authContext)
}

func (handler *BaseHandler) extractKVKey(request *http.Request) string {
	cacheKey := request.PathValue("key")
	if cacheKey == "" {
		cacheKey = request.PathValue("cache_key")
	}
	if cacheKey != "" {
		return strings.Trim(cacheKey, "/")
	}
	trimmed := strings.TrimPrefix(request.URL.Path, "/api/v1/data/kv")
	return strings.Trim(trimmed, "/")
}

func (handler *BaseHandler) handleGetKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext, cacheKey string) {
	internalKey := datakv.BuildInternalKey(authContext, cacheKey)
	storedValue, err := handler.kvStore.Get(request.Context(), internalKey)
	if err != nil {
		if errors.Is(err, core.ErrKVStoreKeyNotFound) || err.Error() == "key not found" {
			handler.writeError(responseWriter, request, http.StatusNotFound, "Key not found", "LAYR_DATA_002")
			return
		}
		handler.writeError(responseWriter, request, http.StatusInternalServerError, err.Error(), "LAYR_DATA_005")
		return
	}

	handler.writeJSON(responseWriter, http.StatusOK, KVGetResponse{
		Key:   cacheKey,
		Value: storedValue,
	})
}

func (handler *BaseHandler) handlePostKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext, cacheKey string) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxKVPayloadBytes)
	bodyBytes, readErr := io.ReadAll(request.Body)
	if readErr != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(readErr, &maxBytesError) {
			handler.writeError(responseWriter, request, http.StatusRequestEntityTooLarge, "Request body exceeds 2MB limit", "LAYR_DATA_001")
			return
		}
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid or empty request body", "LAYR_DATA_001")
		return
	}
	if len(bodyBytes) == 0 {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid or empty request body", "LAYR_DATA_001")
		return
	}

	var rawMap map[string]json.RawMessage
	if unmarshalMapErr := json.Unmarshal(bodyBytes, &rawMap); unmarshalMapErr != nil {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON body: expected JSON object", "LAYR_DATA_001")
		return
	}
	if _, hasValue := rawMap["value"]; !hasValue {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Missing required 'value' property in JSON body", "LAYR_DATA_001")
		return
	}

	var kvSetRequest KVSetRequest
	if decodeErr := json.Unmarshal(bodyBytes, &kvSetRequest); decodeErr != nil {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON body: expected { value: string, ttl?: number }", "LAYR_DATA_001")
		return
	}

	ttlSeconds := kvSetRequest.TTL
	if ttlSeconds < 0 {
		ttlSeconds = 0
	}

	internalKey := datakv.BuildInternalKey(authContext, cacheKey)
	isNX := request.URL.Query().Get("nx") == "true" || request.Header.Get("If-None-Match") == "*"
	if isNX {
		created, setNXErr := handler.kvStore.SetNX(request.Context(), internalKey, kvSetRequest.Value, time.Duration(ttlSeconds)*time.Second)
		if setNXErr != nil {
			handler.writeError(responseWriter, request, http.StatusInternalServerError, setNXErr.Error(), "LAYR_DATA_005")
			return
		}
		if !created {
			handler.writeError(responseWriter, request, http.StatusConflict, "Key already exists", "LAYR_DATA_002")
			return
		}
		handler.writeJSON(responseWriter, http.StatusCreated, KVSetResponse{
			Key:     cacheKey,
			Status:  "ok",
			Created: true,
			TTL:     ttlSeconds,
		})
		return
	}

	setErr := handler.kvStore.Set(request.Context(), internalKey, kvSetRequest.Value, time.Duration(ttlSeconds)*time.Second)
	if setErr != nil {
		handler.writeError(responseWriter, request, http.StatusInternalServerError, setErr.Error(), "LAYR_DATA_005")
		return
	}

	handler.writeJSON(responseWriter, http.StatusOK, KVSetResponse{
		Key:    cacheKey,
		Status: "ok",
		TTL:    ttlSeconds,
	})
}

func (handler *BaseHandler) handlePutKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext, cacheKey string) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxKVPayloadBytes)
	bodyBytes, readErr := io.ReadAll(request.Body)
	if readErr != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(readErr, &maxBytesError) {
			handler.writeError(responseWriter, request, http.StatusRequestEntityTooLarge, "Request body exceeds 2MB limit", "LAYR_DATA_001")
			return
		}
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid or empty request body", "LAYR_DATA_001")
		return
	}
	if len(bodyBytes) == 0 {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid or empty request body", "LAYR_DATA_001")
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
		created, setNXErr := handler.kvStore.SetNX(request.Context(), internalKey, valueToStore, time.Duration(ttlSeconds)*time.Second)
		if setNXErr != nil {
			handler.writeError(responseWriter, request, http.StatusInternalServerError, setNXErr.Error(), "LAYR_DATA_005")
			return
		}
		if !created {
			handler.writeError(responseWriter, request, http.StatusConflict, "Key already exists", "LAYR_DATA_002")
			return
		}
		handler.writeJSON(responseWriter, http.StatusCreated, KVSetResponse{
			Key:     cacheKey,
			Status:  "ok",
			Created: true,
			TTL:     ttlSeconds,
		})
		return
	}

	setErr := handler.kvStore.Set(request.Context(), internalKey, valueToStore, time.Duration(ttlSeconds)*time.Second)
	if setErr != nil {
		handler.writeError(responseWriter, request, http.StatusInternalServerError, setErr.Error(), "LAYR_DATA_005")
		return
	}

	handler.writeJSON(responseWriter, http.StatusOK, KVSetResponse{
		Key:    cacheKey,
		Status: "ok",
		TTL:    ttlSeconds,
	})
}

func (handler *BaseHandler) handlePatchKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext, cacheKey string) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxKVPayloadBytes)
	var kvTouchRequest KVTouchRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&kvTouchRequest); decodeErr != nil || kvTouchRequest.TTL <= 0 {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON body: positive 'ttl' required", "LAYR_DATA_001")
		return
	}

	internalKey := datakv.BuildInternalKey(authContext, cacheKey)
	expireErr := handler.kvStore.Expire(request.Context(), internalKey, time.Duration(kvTouchRequest.TTL)*time.Second)
	if expireErr != nil {
		if errors.Is(expireErr, core.ErrKVStoreKeyNotFound) || expireErr.Error() == "key not found" {
			handler.writeError(responseWriter, request, http.StatusNotFound, "Key not found", "LAYR_DATA_002")
			return
		}
		handler.writeError(responseWriter, request, http.StatusInternalServerError, expireErr.Error(), "LAYR_DATA_005")
		return
	}

	handler.writeJSON(responseWriter, http.StatusOK, KVTouchResponse{
		Key:    cacheKey,
		Status: "ok",
		TTL:    kvTouchRequest.TTL,
	})
}

func (handler *BaseHandler) handleDeleteKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext, cacheKey string) {
	internalKey := datakv.BuildInternalKey(authContext, cacheKey)
	_ = handler.kvStore.Delete(request.Context(), internalKey)
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (handler *BaseHandler) handleMGetKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxKVPayloadBytes)
	var kvmGetRequest KVMGetRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&kvmGetRequest); decodeErr != nil || len(kvmGetRequest.Keys) == 0 {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON body: 'keys' array required", "LAYR_DATA_001")
		return
	}

	internalKeys := make([]string, 0, len(kvmGetRequest.Keys))
	keyLookup := make(map[string]string, len(kvmGetRequest.Keys))
	for _, key := range kvmGetRequest.Keys {
		if key == "" || len(key) > maxKVKeyLength || key == "mget" || key == "mset" || key == "increment" {
			continue
		}
		internalKey := datakv.BuildInternalKey(authContext, key)
		internalKeys = append(internalKeys, internalKey)
		keyLookup[internalKey] = key
	}

	result := make(map[string]string)
	if len(internalKeys) > 0 {
		mgetResults, mgetErr := handler.kvStore.MGet(request.Context(), internalKeys)
		if mgetErr == nil {
			for internalKey, storedValue := range mgetResults {
				if userKey, ok := keyLookup[internalKey]; ok {
					result[userKey] = storedValue
				}
			}
		}
	}

	handler.writeJSON(responseWriter, http.StatusOK, KVMGetResponse{
		Values: result,
	})
}

func (handler *BaseHandler) handleMSetKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxKVPayloadBytes)
	var kvMSetRequest KVMSetRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&kvMSetRequest); decodeErr != nil {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON body: 'entries' map required", "LAYR_DATA_001")
		return
	}
	if len(kvMSetRequest.Entries) == 0 {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "'entries' must not be empty", "LAYR_DATA_001")
		return
	}

	ttlSeconds := 0
	if kvMSetRequest.TTL > 0 {
		ttlSeconds = kvMSetRequest.TTL
	}

	internalEntries := make(map[string]string, len(kvMSetRequest.Entries))
	for userKey, userValue := range kvMSetRequest.Entries {
		if valid, errorMsg := validateKVKey(userKey); !valid {
			handler.writeError(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Invalid key %q: %s", userKey, errorMsg), "LAYR_DATA_001")
			return
		}
		internalKey := datakv.BuildInternalKey(authContext, userKey)
		internalEntries[internalKey] = userValue
	}

	msetErr := handler.kvStore.MSet(request.Context(), internalEntries, time.Duration(ttlSeconds)*time.Second)
	if msetErr != nil {
		handler.writeError(responseWriter, request, http.StatusInternalServerError, msetErr.Error(), "LAYR_DATA_005")
		return
	}

	handler.writeJSON(responseWriter, http.StatusOK, KVMSetResponse{
		Status: "ok",
		Count:  len(internalEntries),
	})
}

func (handler *BaseHandler) handleIncrementKV(responseWriter http.ResponseWriter, request *http.Request, authContext datakv.AuthContext) {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxKVPayloadBytes)
	var kvIncrementRequest KVIncrementRequest
	if decodeErr := json.NewDecoder(request.Body).Decode(&kvIncrementRequest); decodeErr != nil || kvIncrementRequest.Key == "" {
		handler.writeError(responseWriter, request, http.StatusBadRequest, "Invalid JSON body: 'key' required", "LAYR_DATA_001")
		return
	}

	if valid, errorMsg := validateKVKey(kvIncrementRequest.Key); !valid {
		handler.writeError(responseWriter, request, http.StatusBadRequest, errorMsg, "LAYR_DATA_001")
		return
	}

	ttlSeconds := 0
	if kvIncrementRequest.TTL > 0 {
		ttlSeconds = kvIncrementRequest.TTL
	}

	delta := int64(1)
	if kvIncrementRequest.Step != nil {
		delta = *kvIncrementRequest.Step
	}

	internalKey := datakv.BuildInternalKey(authContext, kvIncrementRequest.Key)
	counterValue, incrementErr := handler.kvStore.IncrementBy(request.Context(), internalKey, delta, time.Duration(ttlSeconds)*time.Second)
	if incrementErr != nil {
		handler.writeError(responseWriter, request, http.StatusInternalServerError, incrementErr.Error(), "LAYR_DATA_005")
		return
	}

	handler.writeJSON(responseWriter, http.StatusOK, KVIncrementResponse{
		Key:   kvIncrementRequest.Key,
		Value: counterValue,
	})
}
