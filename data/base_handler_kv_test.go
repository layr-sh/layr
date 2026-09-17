package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"layr.sh/core"
	datakv "layr.sh/data/kv"
)

type failingKVDriver struct {
	inMemoryKVDriver
	setErr    error
	setNXErr  error
	msetErr   error
	expireErr error
	incErr    error
	getErr    error
}

func newFailingKVDriver() *failingKVDriver {
	return &failingKVDriver{
		inMemoryKVDriver: *newInMemoryKVDriver(),
	}
}

func (f *failingKVDriver) KVStore() *core.KVStore {
	return core.NewKVStoreFromDriver(f)
}

func (f *failingKVDriver) Get(ctx context.Context, key string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.inMemoryKVDriver.Get(ctx, key)
}

func (f *failingKVDriver) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	if f.setErr != nil {
		return f.setErr
	}
	return f.inMemoryKVDriver.Set(ctx, key, value, expiry)
}

func (f *failingKVDriver) SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error) {
	if f.setNXErr != nil {
		return false, f.setNXErr
	}
	return f.inMemoryKVDriver.SetNX(ctx, key, value, expiry)
}

func (f *failingKVDriver) MSet(ctx context.Context, entries map[string]string, expiry time.Duration) error {
	if f.msetErr != nil {
		return f.msetErr
	}
	return f.inMemoryKVDriver.MSet(ctx, entries, expiry)
}

func (f *failingKVDriver) Expire(ctx context.Context, key string, expiry time.Duration) error {
	if f.expireErr != nil {
		return f.expireErr
	}
	return f.inMemoryKVDriver.Expire(ctx, key, expiry)
}

func (f *failingKVDriver) Increment(ctx context.Context, key string, expiry time.Duration) (int64, error) {
	return f.IncrementBy(ctx, key, 1, expiry)
}

func (f *failingKVDriver) IncrementBy(ctx context.Context, key string, delta int64, expiry time.Duration) (int64, error) {
	if f.incErr != nil {
		return 0, f.incErr
	}
	return f.inMemoryKVDriver.IncrementBy(ctx, key, delta, expiry)
}

type failingBodyReader struct{}

func (f *failingBodyReader) Read(buffer []byte) (int, error) {
	return 0, errors.New("read stream failure")
}

func (f *failingBodyReader) Close() error { return nil }

func TestDataBaseHandlerKVUnit(t *testing.T) {
	configManager := NewConfigManager(nil)
	baseHandler := NewBaseHandler(nil, configManager)
	inMemoryKVStore := newInMemoryKVStore()
	baseHandler.SetKVStore(inMemoryKVStore)

	t.Run("KVStoreNilChecks", func(t *testing.T) {
		nilKVBaseHandler := NewBaseHandler(nil, configManager)
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/kv/test", nil)
		request.SetPathValue("key", "test")
		responseRecorder := httptest.NewRecorder()
		nilKVBaseHandler.HandleGetKV(responseRecorder, request)
		assert.Equal(t, http.StatusServiceUnavailable, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		nilKVBaseHandler.HandleSetKV(responseRecorder, request)
		assert.Equal(t, http.StatusServiceUnavailable, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		nilKVBaseHandler.HandleDeleteKV(responseRecorder, request)
		assert.Equal(t, http.StatusServiceUnavailable, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		nilKVBaseHandler.HandleMGetKV(responseRecorder, request)
		assert.Equal(t, http.StatusServiceUnavailable, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		nilKVBaseHandler.HandleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusServiceUnavailable, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		nilKVBaseHandler.HandlePutKV(responseRecorder, request)
		assert.Equal(t, http.StatusServiceUnavailable, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		nilKVBaseHandler.HandlePatchKV(responseRecorder, request)
		assert.Equal(t, http.StatusServiceUnavailable, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		nilKVBaseHandler.HandleMSetKV(responseRecorder, request)
		assert.Equal(t, http.StatusServiceUnavailable, responseRecorder.Code)
	})

	t.Run("MissingOrInvalidKeyInPath", func(t *testing.T) {
		// Empty key
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/kv/", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.HandleGetKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleSetKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePutKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePatchKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleDeleteKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Reserved key "mget"
		mgetRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/kv/mget", nil)
		mgetRequest.SetPathValue("key", "mget")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleGetKV(responseRecorder, mgetRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "reserved")

		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePutKV(responseRecorder, mgetRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePatchKV(responseRecorder, mgetRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Reserved key "increment"
		incRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/kv/increment", nil)
		incRequest.SetPathValue("key", "increment")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleGetKV(responseRecorder, incRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "reserved")

		// Overly long key (>512 bytes)
		longKey := strings.Repeat("a", 513)
		longKeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/kv/"+longKey, nil)
		longKeyRequest.SetPathValue("key", longKey)
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleGetKV(responseRecorder, longKeyRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "exceeds maximum")
	})

	t.Run("GetSetDeleteLifecycle", func(t *testing.T) {
		// 1. Get non-existent -> 404
		getRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/kv/mykey", nil)
		getRequest.SetPathValue("key", "mykey")
		responseRecorder := httptest.NewRecorder()
		baseHandler.HandleGetKV(responseRecorder, getRequest)
		assert.Equal(t, http.StatusNotFound, responseRecorder.Code)

		// 2. Set with empty body -> 400
		setEmptyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/mykey", nil)
		setEmptyRequest.SetPathValue("key", "mykey")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleSetKV(responseRecorder, setEmptyRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// 3. Set with JSON payload and custom TTL
		bodyReader := bytes.NewReader([]byte(`{"value":"hello world","ttl":60}`))
		setRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/mykey", bodyReader)
		setRequest.SetPathValue("key", "mykey")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleSetKV(responseRecorder, setRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"ttl":60`)

		// 4. Set via PUT with raw text payload without TTL (persists indefinitely)
		rawBodyReader := bytes.NewReader([]byte(`raw text value`))
		putRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/data/kv/rawkey", rawBodyReader)
		putRequest.SetPathValue("key", "rawkey")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleSetKV(responseRecorder, putRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		// 5. POST with arbitrary JSON without "value" wrapper -> 400 Bad Request
		arbitraryJSONReader := bytes.NewReader([]byte(`{"user_id":123,"name":"Alice"}`))
		arbitraryRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/user_profile", arbitraryJSONReader)
		arbitraryRequest.SetPathValue("key", "user_profile")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePostKV(responseRecorder, arbitraryRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// 5b. PUT with arbitrary JSON without "value" wrapper -> 200 OK (stores verbatim)
		arbitraryPUTReader := bytes.NewReader([]byte(`{"user_id":123,"name":"Alice"}`))
		arbitraryPUTRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/data/kv/user_profile", arbitraryPUTReader)
		arbitraryPUTRequest.SetPathValue("key", "user_profile")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePutKV(responseRecorder, arbitraryPUTRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		// 5c. PUT raw JSON array with URL query param ?ttl=300 -> 200 OK
		rawArrayReader := bytes.NewReader([]byte(`[{"id":1,"name":"Alice"}]`))
		rawArrayRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/data/kv/raw_array?ttl=300", rawArrayReader)
		rawArrayRequest.SetPathValue("key", "raw_array")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePutKV(responseRecorder, rawArrayRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"ttl":300`)

		getProfileRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/kv/user_profile", nil)
		getProfileRequest.SetPathValue("key", "user_profile")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleGetKV(responseRecorder, getProfileRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		var profileKVGetResponse KVGetResponse
		_ = json.NewDecoder(responseRecorder.Body).Decode(&profileKVGetResponse)
		assert.Contains(t, profileKVGetResponse.Value, `"user_id":123`)

		// 6. Set and get legitimate empty string value
		emptyValueReader := bytes.NewReader([]byte(`{"value":""}`))
		emptyValueSetRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/empty_val_key", emptyValueReader)
		emptyValueSetRequest.SetPathValue("key", "empty_val_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePostKV(responseRecorder, emptyValueSetRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		getEmptyValueRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/kv/empty_val_key", nil)
		getEmptyValueRequest.SetPathValue("key", "empty_val_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleGetKV(responseRecorder, getEmptyValueRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"value":""`)

		// 7. Get existing -> 200
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleGetKV(responseRecorder, getRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "hello world")

		// 8. Delete -> 204
		deleteRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/data/kv/mykey", nil)
		deleteRequest.SetPathValue("key", "mykey")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleDeleteKV(responseRecorder, deleteRequest)
		assert.Equal(t, http.StatusNoContent, responseRecorder.Code)

		// 9. Get deleted -> 404
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleGetKV(responseRecorder, getRequest)
		assert.Equal(t, http.StatusNotFound, responseRecorder.Code)
	})

	t.Run("PayloadSizeExceeded", func(t *testing.T) {
		largePayload := bytes.Repeat([]byte("a"), 2*1024*1024+10)
		largeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/large_key", bytes.NewReader(largePayload))
		largeRequest.SetPathValue("key", "large_key")
		responseRecorder := httptest.NewRecorder()
		baseHandler.HandlePostKV(responseRecorder, largeRequest)
		assert.Equal(t, http.StatusRequestEntityTooLarge, responseRecorder.Code)
	})

	t.Run("SetFailureInternalServerError", func(t *testing.T) {
		failingKVDriver := newFailingKVDriver()
		failingKVDriver.setErr = errors.New("write failure")
		failBaseHandler := NewBaseHandler(nil, configManager)
		failBaseHandler.SetKVStore(failingKVDriver.KVStore())

		bodyReader := bytes.NewReader([]byte(`{"value":"test"}`))
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/failkey", bodyReader)
		request.SetPathValue("key", "failkey")
		responseRecorder := httptest.NewRecorder()
		failBaseHandler.HandlePostKV(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)

		// Get failure
		failGetDriver := newFailingKVDriver()
		failGetDriver.getErr = errors.New("read failure")
		failGetBaseHandler := NewBaseHandler(nil, configManager)
		failGetBaseHandler.SetKVStore(failGetDriver.KVStore())
		getRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/kv/failkey", nil)
		getRequest.SetPathValue("key", "failkey")
		responseRecorder = httptest.NewRecorder()
		failGetBaseHandler.HandleGetKV(responseRecorder, getRequest)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)

		// Read failure from body reader
		readFailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/readfail", &failingBodyReader{})
		readFailRequest.SetPathValue("key", "readfail")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePostKV(responseRecorder, readFailRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Non-string value returns 400 Bad Request in POST
		nonStringValueReader := bytes.NewReader([]byte(`{"value":12345,"ttl":-5}`))
		nonStringRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/numeric_key", nonStringValueReader)
		nonStringRequest.SetPathValue("key", "numeric_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePostKV(responseRecorder, nonStringRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Non-object JSON in POST -> 400
		nonObjectReader := bytes.NewReader([]byte(`"just_a_string"`))
		nonObjectRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/str_key", nonObjectReader)
		nonObjectRequest.SetPathValue("key", "str_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePostKV(responseRecorder, nonObjectRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Negative TTL in POST -> clamped to 0
		negTTLReader := bytes.NewReader([]byte(`{"value":"valid_string","ttl":-5}`))
		negTTLRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/neg_ttl", negTTLReader)
		negTTLRequest.SetPathValue("key", "neg_ttl")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePostKV(responseRecorder, negTTLRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		// Post SetNX error
		failingKVDriver.setNXErr = errors.New("post setnx fail")
		postNXFailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/post_nx?nx=true", bytes.NewReader([]byte(`{"value":"v"}`)))
		postNXFailRequest.SetPathValue("key", "post_nx")
		responseRecorder = httptest.NewRecorder()
		failBaseHandler.HandlePostKV(responseRecorder, postNXFailRequest)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		failingKVDriver.setNXErr = nil

		// PUT payload size exceeded
		putLargeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/data/kv/large_put", bytes.NewReader(bytes.Repeat([]byte("a"), 2*1024*1024+10)))
		putLargeRequest.SetPathValue("key", "large_put")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePutKV(responseRecorder, putLargeRequest)
		assert.Equal(t, http.StatusRequestEntityTooLarge, responseRecorder.Code)

		// PUT read failure
		putReadFailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/data/kv/put_read_fail", &failingBodyReader{})
		putReadFailRequest.SetPathValue("key", "put_read_fail")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePutKV(responseRecorder, putReadFailRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// PUT empty body
		putEmptyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/data/kv/put_empty", bytes.NewReader([]byte("")))
		putEmptyRequest.SetPathValue("key", "put_empty")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePutKV(responseRecorder, putEmptyRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// PUT X-Layr-Cache-TTL header
		putHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/data/kv/put_header", bytes.NewReader([]byte("raw_val")))
		putHeaderRequest.Header.Set("X-Layr-Cache-TTL", "60")
		putHeaderRequest.SetPathValue("key", "put_header")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePutKV(responseRecorder, putHeaderRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		// PUT SetNX error
		failingKVDriver.setNXErr = errors.New("put setnx fail")
		putNXFailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/data/kv/put_nx_fail?nx=true", bytes.NewReader([]byte("raw_val")))
		putNXFailRequest.SetPathValue("key", "put_nx_fail")
		responseRecorder = httptest.NewRecorder()
		failBaseHandler.HandlePutKV(responseRecorder, putNXFailRequest)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		failingKVDriver.setNXErr = nil

		// PUT SetNX conflict
		firstPutNXConflictRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/data/kv/put_nx_conf?nx=true", bytes.NewReader([]byte("v1")))
		firstPutNXConflictRequest.SetPathValue("key", "put_nx_conf")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePutKV(responseRecorder, firstPutNXConflictRequest)
		assert.Equal(t, http.StatusCreated, responseRecorder.Code)

		secondPutNXConflictRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/data/kv/put_nx_conf?nx=true", bytes.NewReader([]byte("v2")))
		secondPutNXConflictRequest.SetPathValue("key", "put_nx_conf")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePutKV(responseRecorder, secondPutNXConflictRequest)
		assert.Equal(t, http.StatusConflict, responseRecorder.Code)

		// PUT Set error
		failingKVDriver.setErr = errors.New("put set fail")
		putSetFailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/data/kv/put_set_fail", bytes.NewReader([]byte("raw_val")))
		putSetFailRequest.SetPathValue("key", "put_set_fail")
		responseRecorder = httptest.NewRecorder()
		failBaseHandler.HandlePutKV(responseRecorder, putSetFailRequest)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		failingKVDriver.setErr = nil
	})

	t.Run("SetNX", func(t *testing.T) {
		// First write with nx=true -> 201 Created
		nxBodyReader := bytes.NewReader([]byte(`{"value":"lock_acquired","ttl":60}`))
		nxRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/lock_key?nx=true", nxBodyReader)
		nxRequest.SetPathValue("key", "lock_key")
		responseRecorder := httptest.NewRecorder()
		baseHandler.HandlePostKV(responseRecorder, nxRequest)
		assert.Equal(t, http.StatusCreated, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"created":true`)

		// Second write with nx=true -> 409 Conflict
		nxSecondBodyReader := bytes.NewReader([]byte(`{"value":"another_lock","ttl":60}`))
		nxSecondRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/lock_key?nx=true", nxSecondBodyReader)
		nxSecondRequest.SetPathValue("key", "lock_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePostKV(responseRecorder, nxSecondRequest)
		assert.Equal(t, http.StatusConflict, responseRecorder.Code)
	})

	t.Run("PatchKVTouch", func(t *testing.T) {
		// Set a key first
		setReader := bytes.NewReader([]byte(`{"value":"touch_me"}`))
		setRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/touch_key", setReader)
		setRequest.SetPathValue("key", "touch_key")
		responseRecorder := httptest.NewRecorder()
		baseHandler.HandlePostKV(responseRecorder, setRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		// Valid touch via PATCH
		touchReader := bytes.NewReader([]byte(`{"ttl":3600}`))
		patchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/data/kv/touch_key", touchReader)
		patchRequest.SetPathValue("key", "touch_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePatchKV(responseRecorder, patchRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"ttl":3600`)

		// Invalid touch body
		badTouchReader := bytes.NewReader([]byte(`{"ttl":-1}`))
		badPatchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/data/kv/touch_key", badTouchReader)
		badPatchRequest.SetPathValue("key", "touch_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePatchKV(responseRecorder, badPatchRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Touch non-existent key -> 404
		nonExistentPatchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/data/kv/non_existent", bytes.NewReader([]byte(`{"ttl":60}`)))
		nonExistentPatchRequest.SetPathValue("key", "non_existent")
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandlePatchKV(responseRecorder, nonExistentPatchRequest)
		assert.Equal(t, http.StatusNotFound, responseRecorder.Code)

		// Touch expire error -> 500
		failTouchDriver := newFailingKVDriver()
		failTouchDriver.expireErr = errors.New("expire fail")
		failTouchBaseHandler := NewBaseHandler(nil, configManager)
		failTouchBaseHandler.SetKVStore(failTouchDriver.KVStore())
		failExpirePatchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/v1/data/kv/any_key", bytes.NewReader([]byte(`{"ttl":60}`)))
		failExpirePatchRequest.SetPathValue("key", "any_key")
		responseRecorder = httptest.NewRecorder()
		failTouchBaseHandler.HandlePatchKV(responseRecorder, failExpirePatchRequest)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})

	t.Run("MSetKV", func(t *testing.T) {
		// Invalid body
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/mset", bytes.NewReader([]byte(`invalid`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.HandleMSetKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Empty entries
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/mset", bytes.NewReader([]byte(`{"entries":{}}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleMSetKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Reserved key inside entries
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/mset", bytes.NewReader([]byte(`{"entries":{"mget":"val"}}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleMSetKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Valid mset
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/mset", bytes.NewReader([]byte(`{"entries":{"theme":"dark","notifications":"true"},"ttl":86400}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleMSetKV(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"count":2`)

		// MSet store error -> 500
		failMSetDriver := newFailingKVDriver()
		failMSetDriver.msetErr = errors.New("mset fail")
		failMSetBaseHandler := NewBaseHandler(nil, configManager)
		failMSetBaseHandler.SetKVStore(failMSetDriver.KVStore())
		failMSetRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/mset", bytes.NewReader([]byte(`{"entries":{"a":"1"}}`)))
		responseRecorder = httptest.NewRecorder()
		failMSetBaseHandler.HandleMSetKV(responseRecorder, failMSetRequest)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})

	t.Run("HierarchicalKeysWithSlashes", func(t *testing.T) {
		slashKey := "users/123/profile/preferences"
		setReader := bytes.NewReader([]byte(`{"value":"active"}`))
		setRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/"+slashKey, setReader)
		setRequest.SetPathValue("key", slashKey)
		responseRecorder := httptest.NewRecorder()
		baseHandler.HandlePostKV(responseRecorder, setRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		getRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/data/kv/"+slashKey, nil)
		getRequest.SetPathValue("key", slashKey)
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleGetKV(responseRecorder, getRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "active")
	})

	t.Run("MGetKV", func(t *testing.T) {
		// Invalid body
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/mget", bytes.NewReader([]byte(`invalid`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.HandleMGetKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Empty keys
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/mget", bytes.NewReader([]byte(`{"keys":[]}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleMGetKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Valid keys
		authContext := datakv.ExtractAuthContext(request, "")
		_ = inMemoryKVStore.Set(context.Background(), datakv.BuildInternalKey(authContext, "k1"), "v1", time.Hour)
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/mget", bytes.NewReader([]byte(`{"keys":["k1","k2","mget",""]}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleMGetKV(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "values")
	})

	t.Run("IncrementKV", func(t *testing.T) {
		// Invalid body
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/increment", bytes.NewReader([]byte(`invalid`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.HandleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Empty key
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/increment", bytes.NewReader([]byte(`{"key":""}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Reserved key
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/increment", bytes.NewReader([]byte(`{"key":"mget"}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Valid increment with custom TTL
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/increment", bytes.NewReader([]byte(`{"key":"counter","ttl":300}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"value":1`)

		// Increment by custom step +5
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/increment", bytes.NewReader([]byte(`{"key":"counter","step":5,"ttl":300}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"value":6`)

		// Decrement by step -2
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/increment", bytes.NewReader([]byte(`{"key":"counter","step":-2,"ttl":300}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.HandleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"value":4`)

		// Increment failure
		failDriver := newFailingKVDriver()
		failDriver.incErr = errors.New("inc failure")
		failBaseHandler := NewBaseHandler(nil, configManager)
		failBaseHandler.SetKVStore(failDriver.KVStore())

		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/data/kv/increment", bytes.NewReader([]byte(`{"key":"fail"}`)))
		responseRecorder = httptest.NewRecorder()
		failBaseHandler.HandleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})
}
