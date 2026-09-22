package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	core.TestInMemoryKVDriver
	setErr    error
	setNXErr  error
	msetErr   error
	expireErr error
	incErr    error
	getErr    error
}

func newFailingKVDriver() *failingKVDriver {
	return &failingKVDriver{
		TestInMemoryKVDriver: *core.NewTestInMemoryKVDriver(),
	}
}

func (f *failingKVDriver) KVStore() *core.KVStore {
	return core.NewKVStoreFromDriver(f)
}

func (f *failingKVDriver) Get(ctx context.Context, key string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	value, err := f.TestInMemoryKVDriver.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("kv get: %w", err)
	}
	return value, nil
}

func (f *failingKVDriver) Set(ctx context.Context, key string, value string, expiry time.Duration) error {
	if f.setErr != nil {
		return f.setErr
	}
	if err := f.TestInMemoryKVDriver.Set(ctx, key, value, expiry); err != nil {
		return fmt.Errorf("kv set: %w", err)
	}
	return nil
}

func (f *failingKVDriver) SetNX(ctx context.Context, key string, value string, expiry time.Duration) (bool, error) {
	if f.setNXErr != nil {
		return false, f.setNXErr
	}
	ok, err := f.TestInMemoryKVDriver.SetNX(ctx, key, value, expiry)
	if err != nil {
		return false, fmt.Errorf("kv setnx: %w", err)
	}
	return ok, nil
}

func (f *failingKVDriver) MSet(ctx context.Context, entries map[string]string, expiry time.Duration) error {
	if f.msetErr != nil {
		return f.msetErr
	}
	if err := f.TestInMemoryKVDriver.MSet(ctx, entries, expiry); err != nil {
		return fmt.Errorf("kv mset: %w", err)
	}
	return nil
}

func (f *failingKVDriver) Expire(ctx context.Context, key string, expiry time.Duration) error {
	if f.expireErr != nil {
		return f.expireErr
	}
	if err := f.TestInMemoryKVDriver.Expire(ctx, key, expiry); err != nil {
		return fmt.Errorf("kv expire: %w", err)
	}
	return nil
}

func (f *failingKVDriver) Increment(ctx context.Context, key string, expiry time.Duration) (int64, error) {
	return f.IncrementBy(ctx, key, 1, expiry)
}

func (f *failingKVDriver) IncrementBy(ctx context.Context, key string, delta int64, expiry time.Duration) (int64, error) {
	if f.incErr != nil {
		return 0, f.incErr
	}
	incrementedValue, err := f.TestInMemoryKVDriver.IncrementBy(ctx, key, delta, expiry)
	if err != nil {
		return 0, fmt.Errorf("kv inc: %w", err)
	}
	return incrementedValue, nil
}

type failingBodyReader struct{}

func (f *failingBodyReader) Read(buffer []byte) (int, error) {
	return 0, errors.New("read stream failure")
}

func (f *failingBodyReader) Close() error { return nil }

func TestDataBaseHandlerKVUnit(t *testing.T) {
	kernel := core.NewTestKernel(nil)
	service := NewService(kernel)
	baseHandler := service.baseHandler

	t.Run("MissingOrInvalidKeyInPath", func(t *testing.T) {
		// Empty key
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/kv/", nil)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleGetKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.handleUpdateKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.handleTouchKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.handleDeleteKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Reserved key "mget"
		mgetRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/kv/mget", nil)
		mgetRequest.SetPathValue("key", "mget")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleGetKV(responseRecorder, mgetRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "reserved")

		responseRecorder = httptest.NewRecorder()
		baseHandler.handleUpdateKV(responseRecorder, mgetRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		responseRecorder = httptest.NewRecorder()
		baseHandler.handleTouchKV(responseRecorder, mgetRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Reserved key "increment"
		incRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/kv/increment", nil)
		incRequest.SetPathValue("key", "increment")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleGetKV(responseRecorder, incRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "reserved")

		// Overly long key (>512 bytes)
		longKey := strings.Repeat("a", 513)
		longKeyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/kv/"+longKey, nil)
		longKeyRequest.SetPathValue("key", longKey)
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleGetKV(responseRecorder, longKeyRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "exceeds maximum")
	})

	t.Run("GetSetDeleteLifecycle", func(t *testing.T) {
		// 1. Get non-existent -> 404
		getRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/kv/mykey", nil)
		getRequest.SetPathValue("key", "mykey")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleGetKV(responseRecorder, getRequest)
		assert.Equal(t, http.StatusNotFound, responseRecorder.Code)

		// 2. Set with empty body -> 400
		setEmptyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/mykey", nil)
		setEmptyRequest.SetPathValue("key", "mykey")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, setEmptyRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// 3. Set with JSON payload and custom TTL
		bodyReader := bytes.NewReader([]byte(`{"value":"hello world","ttl":60}`))
		setRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/mykey", bodyReader)
		setRequest.SetPathValue("key", "mykey")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, setRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"ttl":60`)

		// 4. Set via PUT with raw text payload without TTL (persists indefinitely)
		rawBodyReader := bytes.NewReader([]byte(`raw text value`))
		putRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/data/kv/rawkey", rawBodyReader)
		putRequest.SetPathValue("key", "rawkey")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleUpdateKV(responseRecorder, putRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		// 5. POST with arbitrary JSON without "value" wrapper -> 400 Bad Request
		arbitraryJSONReader := bytes.NewReader([]byte(`{"user_id":123,"name":"Alice"}`))
		arbitraryRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/user_profile", arbitraryJSONReader)
		arbitraryRequest.SetPathValue("key", "user_profile")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, arbitraryRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// 5b. PUT with arbitrary JSON without "value" wrapper -> 200 OK (stores verbatim)
		arbitraryPUTReader := bytes.NewReader([]byte(`{"user_id":123,"name":"Alice"}`))
		arbitraryPUTRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/data/kv/user_profile", arbitraryPUTReader)
		arbitraryPUTRequest.SetPathValue("key", "user_profile")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleUpdateKV(responseRecorder, arbitraryPUTRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		// 5c. PUT raw JSON array with URL query param ?ttl=300 -> 200 OK
		rawArrayReader := bytes.NewReader([]byte(`[{"id":1,"name":"Alice"}]`))
		rawArrayRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/data/kv/raw_array?ttl=300", rawArrayReader)
		rawArrayRequest.SetPathValue("key", "raw_array")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleUpdateKV(responseRecorder, rawArrayRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"ttl":300`)

		getProfileRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/kv/user_profile", nil)
		getProfileRequest.SetPathValue("key", "user_profile")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleGetKV(responseRecorder, getProfileRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		var profileGetKVResponse GetKVResponse
		_ = json.NewDecoder(responseRecorder.Body).Decode(&profileGetKVResponse)
		assert.Contains(t, profileGetKVResponse.Value, `"user_id":123`)

		// 6. Set and get legitimate empty string value
		emptyValueReader := bytes.NewReader([]byte(`{"value":""}`))
		emptyValueSetRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/empty_val_key", emptyValueReader)
		emptyValueSetRequest.SetPathValue("key", "empty_val_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, emptyValueSetRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		getEmptyValueRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/kv/empty_val_key", nil)
		getEmptyValueRequest.SetPathValue("key", "empty_val_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleGetKV(responseRecorder, getEmptyValueRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"value":""`)

		// 7. Get existing -> 200
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleGetKV(responseRecorder, getRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "hello world")

		// 8. Delete -> 204
		deleteRequest := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/v1/data/kv/mykey", nil)
		deleteRequest.SetPathValue("key", "mykey")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleDeleteKV(responseRecorder, deleteRequest)
		assert.Equal(t, http.StatusNoContent, responseRecorder.Code)

		// 9. Get deleted -> 404
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleGetKV(responseRecorder, getRequest)
		assert.Equal(t, http.StatusNotFound, responseRecorder.Code)
	})

	t.Run("PayloadSizeExceeded", func(t *testing.T) {
		largePayload := bytes.Repeat([]byte("a"), 2*1024*1024+10)
		largeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/large_key", bytes.NewReader(largePayload))
		largeRequest.SetPathValue("key", "large_key")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, largeRequest)
		assert.Equal(t, http.StatusRequestEntityTooLarge, responseRecorder.Code)
	})

	t.Run("SetFailureInternalServerError", func(t *testing.T) {
		failingKVDriver := newFailingKVDriver()
		failingKVDriver.setErr = errors.New("write failure")
		failKernel := core.NewTestKernel(nil, core.WithKVStore(failingKVDriver.KVStore()))
		failBaseHandler := NewService(failKernel).baseHandler

		bodyReader := bytes.NewReader([]byte(`{"value":"test"}`))
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/failkey", bodyReader)
		request.SetPathValue("key", "failkey")
		responseRecorder := httptest.NewRecorder()
		failBaseHandler.handleSetKV(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)

		// Get failure
		failGetDriver := newFailingKVDriver()
		failGetDriver.getErr = errors.New("read failure")
		failGetKernel := core.NewTestKernel(nil, core.WithKVStore(failGetDriver.KVStore()))
		failGetBaseHandler := NewService(failGetKernel).baseHandler
		getRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/kv/failkey", nil)
		getRequest.SetPathValue("key", "failkey")
		responseRecorder = httptest.NewRecorder()
		failGetBaseHandler.handleGetKV(responseRecorder, getRequest)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)

		// Read failure from body reader
		readFailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/readfail", &failingBodyReader{})
		readFailRequest.SetPathValue("key", "readfail")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, readFailRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Non-string value returns 400 Bad Request in POST
		nonStringValueReader := bytes.NewReader([]byte(`{"value":12345,"ttl":-5}`))
		nonStringRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/numeric_key", nonStringValueReader)
		nonStringRequest.SetPathValue("key", "numeric_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, nonStringRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Non-object JSON in POST -> 400
		nonObjectReader := bytes.NewReader([]byte(`"just_a_string"`))
		nonObjectRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/str_key", nonObjectReader)
		nonObjectRequest.SetPathValue("key", "str_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, nonObjectRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Negative TTL in POST -> clamped to 0
		negTTLReader := bytes.NewReader([]byte(`{"value":"valid_string","ttl":-5}`))
		negTTLRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/neg_ttl", negTTLReader)
		negTTLRequest.SetPathValue("key", "neg_ttl")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, negTTLRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		// Post SetNX error
		failingKVDriver.setNXErr = errors.New("post setnx fail")
		postNXFailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/post_nx?nx=true", bytes.NewReader([]byte(`{"value":"v"}`)))
		postNXFailRequest.SetPathValue("key", "post_nx")
		responseRecorder = httptest.NewRecorder()
		failBaseHandler.handleSetKV(responseRecorder, postNXFailRequest)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		failingKVDriver.setNXErr = nil

		// PUT payload size exceeded
		putLargeRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/data/kv/large_put", bytes.NewReader(bytes.Repeat([]byte("a"), 2*1024*1024+10)))
		putLargeRequest.SetPathValue("key", "large_put")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleUpdateKV(responseRecorder, putLargeRequest)
		assert.Equal(t, http.StatusRequestEntityTooLarge, responseRecorder.Code)

		// PUT read failure
		putReadFailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/data/kv/put_read_fail", &failingBodyReader{})
		putReadFailRequest.SetPathValue("key", "put_read_fail")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleUpdateKV(responseRecorder, putReadFailRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// PUT empty body
		putEmptyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/data/kv/put_empty", bytes.NewReader([]byte("")))
		putEmptyRequest.SetPathValue("key", "put_empty")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleUpdateKV(responseRecorder, putEmptyRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// PUT X-Layr-Cache-TTL header
		putHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/data/kv/put_header", bytes.NewReader([]byte("raw_val")))
		putHeaderRequest.Header.Set("X-Layr-Cache-TTL", "60")
		putHeaderRequest.SetPathValue("key", "put_header")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleUpdateKV(responseRecorder, putHeaderRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		// PUT SetNX error
		failingKVDriver.setNXErr = errors.New("put setnx fail")
		putNXFailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/data/kv/put_nx_fail?nx=true", bytes.NewReader([]byte("raw_val")))
		putNXFailRequest.SetPathValue("key", "put_nx_fail")
		responseRecorder = httptest.NewRecorder()
		failBaseHandler.handleUpdateKV(responseRecorder, putNXFailRequest)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		failingKVDriver.setNXErr = nil

		// PUT SetNX conflict
		firstPutNXConflictRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/data/kv/put_nx_conf?nx=true", bytes.NewReader([]byte("v1")))
		firstPutNXConflictRequest.SetPathValue("key", "put_nx_conf")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleUpdateKV(responseRecorder, firstPutNXConflictRequest)
		assert.Equal(t, http.StatusCreated, responseRecorder.Code)

		secondPutNXConflictRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/data/kv/put_nx_conf?nx=true", bytes.NewReader([]byte("v2")))
		secondPutNXConflictRequest.SetPathValue("key", "put_nx_conf")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleUpdateKV(responseRecorder, secondPutNXConflictRequest)
		assert.Equal(t, http.StatusConflict, responseRecorder.Code)

		// PUT Set error
		failingKVDriver.setErr = errors.New("put set fail")
		putSetFailRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/data/kv/put_set_fail", bytes.NewReader([]byte("raw_val")))
		putSetFailRequest.SetPathValue("key", "put_set_fail")
		responseRecorder = httptest.NewRecorder()
		failBaseHandler.handleUpdateKV(responseRecorder, putSetFailRequest)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		failingKVDriver.setErr = nil
	})

	t.Run("SetNX", func(t *testing.T) {
		// First write with nx=true -> 201 Created
		nxBodyReader := bytes.NewReader([]byte(`{"value":"lock_acquired","ttl":60}`))
		nxRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/lock_key?nx=true", nxBodyReader)
		nxRequest.SetPathValue("key", "lock_key")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, nxRequest)
		assert.Equal(t, http.StatusCreated, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"created":true`)

		// Second write with nx=true -> 409 Conflict
		nxSecondBodyReader := bytes.NewReader([]byte(`{"value":"another_lock","ttl":60}`))
		nxSecondRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/lock_key?nx=true", nxSecondBodyReader)
		nxSecondRequest.SetPathValue("key", "lock_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, nxSecondRequest)
		assert.Equal(t, http.StatusConflict, responseRecorder.Code)
	})

	t.Run("PatchKVTouch", func(t *testing.T) {
		// Set a key first
		setReader := bytes.NewReader([]byte(`{"value":"touch_me"}`))
		setRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/touch_key", setReader)
		setRequest.SetPathValue("key", "touch_key")
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, setRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		// Valid touch via PATCH
		touchReader := bytes.NewReader([]byte(`{"ttl":3600}`))
		patchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/v1/data/kv/touch_key", touchReader)
		patchRequest.SetPathValue("key", "touch_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleTouchKV(responseRecorder, patchRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"ttl":3600`)

		// Invalid touch body
		badTouchReader := bytes.NewReader([]byte(`{"ttl":-1}`))
		badPatchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/v1/data/kv/touch_key", badTouchReader)
		badPatchRequest.SetPathValue("key", "touch_key")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleTouchKV(responseRecorder, badPatchRequest)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Touch non-existent key -> 404
		nonExistentPatchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/v1/data/kv/non_existent", bytes.NewReader([]byte(`{"ttl":60}`)))
		nonExistentPatchRequest.SetPathValue("key", "non_existent")
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleTouchKV(responseRecorder, nonExistentPatchRequest)
		assert.Equal(t, http.StatusNotFound, responseRecorder.Code)

		// Touch expire error -> 500
		failTouchDriver := newFailingKVDriver()
		failTouchDriver.expireErr = errors.New("expire fail")
		failTouchKernel := core.NewTestKernel(nil, core.WithKVStore(failTouchDriver.KVStore()))
		failTouchBaseHandler := NewService(failTouchKernel).baseHandler
		failExpirePatchRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/v1/data/kv/any_key", bytes.NewReader([]byte(`{"ttl":60}`)))
		failExpirePatchRequest.SetPathValue("key", "any_key")
		responseRecorder = httptest.NewRecorder()
		failTouchBaseHandler.handleTouchKV(responseRecorder, failExpirePatchRequest)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})

	t.Run("MSetKV", func(t *testing.T) {
		// Invalid body
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/mset", bytes.NewReader([]byte(`invalid`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleSetMultipleKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Empty entries
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/mset", bytes.NewReader([]byte(`{"entries":{}}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleSetMultipleKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Reserved key inside entries
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/mset", bytes.NewReader([]byte(`{"entries":{"mget":"val"}}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleSetMultipleKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Valid mset
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/mset", bytes.NewReader([]byte(`{"entries":{"theme":"dark","notifications":"true"},"ttl":86400}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleSetMultipleKV(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"count":2`)

		// MSet store error -> 500
		failMSetDriver := newFailingKVDriver()
		failMSetDriver.msetErr = errors.New("mset fail")
		failMSetKernel := core.NewTestKernel(nil, core.WithKVStore(failMSetDriver.KVStore()))
		failMSetBaseHandler := NewService(failMSetKernel).baseHandler
		failMSetRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/mset", bytes.NewReader([]byte(`{"entries":{"a":"1"}}`)))
		responseRecorder = httptest.NewRecorder()
		failMSetBaseHandler.handleSetMultipleKV(responseRecorder, failMSetRequest)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})

	t.Run("HierarchicalKeysWithSlashes", func(t *testing.T) {
		slashKey := "users/123/profile/preferences"
		setReader := bytes.NewReader([]byte(`{"value":"active"}`))
		setRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/"+slashKey, setReader)
		setRequest.SetPathValue("key", slashKey)
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleSetKV(responseRecorder, setRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)

		getRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/data/kv/"+slashKey, nil)
		getRequest.SetPathValue("key", slashKey)
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleGetKV(responseRecorder, getRequest)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "active")
	})

	t.Run("MGetKV", func(t *testing.T) {
		// Invalid body
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/mget", bytes.NewReader([]byte(`invalid`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleGetMultipleKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Empty keys
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/mget", bytes.NewReader([]byte(`{"keys":[]}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleGetMultipleKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Valid keys
		authContext := datakv.ExtractAuthContext(request, "")
		_ = baseHandler.kernel.KVStore().Set(context.Background(), datakv.BuildInternalKey(authContext, "k1"), "v1", time.Hour)
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/mget", bytes.NewReader([]byte(`{"keys":["k1","k2","mget",""]}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleGetMultipleKV(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), "values")
	})

	t.Run("IncrementKV", func(t *testing.T) {
		// Invalid body
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/increment", bytes.NewReader([]byte(`invalid`)))
		responseRecorder := httptest.NewRecorder()
		baseHandler.handleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Empty key
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/increment", bytes.NewReader([]byte(`{"key":""}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Reserved key
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/increment", bytes.NewReader([]byte(`{"key":"mget"}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusBadRequest, responseRecorder.Code)

		// Valid increment with custom TTL
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/increment", bytes.NewReader([]byte(`{"key":"counter","ttl":300}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"value":1`)

		// Increment by custom step +5
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/increment", bytes.NewReader([]byte(`{"key":"counter","step":5,"ttl":300}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"value":6`)

		// Decrement by step -2
		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/increment", bytes.NewReader([]byte(`{"key":"counter","step":-2,"ttl":300}`)))
		responseRecorder = httptest.NewRecorder()
		baseHandler.handleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Contains(t, responseRecorder.Body.String(), `"value":4`)

		// Increment failure
		failDriver := newFailingKVDriver()
		failDriver.incErr = errors.New("inc failure")
		failKernel := core.NewTestKernel(nil, core.WithKVStore(failDriver.KVStore()))
		failBaseHandler := NewService(failKernel).baseHandler

		request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/data/kv/increment", bytes.NewReader([]byte(`{"key":"fail"}`)))
		responseRecorder = httptest.NewRecorder()
		failBaseHandler.handleIncrementKV(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	})
}
