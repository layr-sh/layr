package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"layr.sh/core"
)

func TestDataBaseHandlerKVIntegration(t *testing.T) {
	db, cleanup := setupTestDataDatabase(t)
	if db == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	service := NewService(db)
	databaseKVStore := core.NewDatabaseKVStore(ctx, db, 60*time.Second)
	defer func() { _ = databaseKVStore.Close() }()
	service.SetKVStore(databaseKVStore)

	_ = service.Start(ctx)
	defer func() { _ = service.Stop() }()

	baseHandler := service.BaseHandler()

	// 1. Set KV
	kvSetRequest := KVSetRequest{
		Value: "integrated-kv-value",
		TTL:   300,
	}
	setBytes, _ := json.Marshal(kvSetRequest)
	setRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/kv/user-pref", bytes.NewReader(setBytes))
	setRequest.SetPathValue("key", "user-pref")
	setResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleSetKV(setResponseRecorder, setRequest)
	assert.Equal(t, http.StatusOK, setResponseRecorder.Code)

	// 2. Get KV
	getRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/data/kv/user-pref", nil)
	getRequest.SetPathValue("key", "user-pref")
	getResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleGetKV(getResponseRecorder, getRequest)
	assert.Equal(t, http.StatusOK, getResponseRecorder.Code)
	assert.Contains(t, getResponseRecorder.Body.String(), "integrated-kv-value")

	// 3. Increment KV
	kvIncrementRequest := KVIncrementRequest{
		Key: "hits",
		TTL: 300,
	}
	incBytes, _ := json.Marshal(kvIncrementRequest)
	incrementRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/kv/increment", bytes.NewReader(incBytes))
	incrementResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleIncrementKV(incrementResponseRecorder, incrementRequest)
	assert.Equal(t, http.StatusOK, incrementResponseRecorder.Code)
	assert.Contains(t, incrementResponseRecorder.Body.String(), `"value":1`)

	// 4. MGet KV
	kvmGetRequest := KVMGetRequest{
		Keys: []string{"user-pref", "hits"},
	}
	mgetBytes, _ := json.Marshal(kvmGetRequest)
	mgetRequest := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/data/kv/mget", bytes.NewReader(mgetBytes))
	mgetResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleMGetKV(mgetResponseRecorder, mgetRequest)
	assert.Equal(t, http.StatusOK, mgetResponseRecorder.Code)

	// 5. Delete KV
	deleteRequest := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/v1/data/kv/user-pref", nil)
	deleteRequest.SetPathValue("key", "user-pref")
	deleteResponseRecorder := httptest.NewRecorder()
	baseHandler.HandleDeleteKV(deleteResponseRecorder, deleteRequest)
	assert.Equal(t, http.StatusNoContent, deleteResponseRecorder.Code)
}
