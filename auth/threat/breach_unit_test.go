package threat

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestThreatBreachEmptyPasswordUnit(t *testing.T) {
	ctx := context.Background()
	isBreached, count, err := CheckPwnedPassword(ctx, nil, nil, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isBreached || count != 0 {
		t.Fatalf("expected (false, 0), got (%v, %d)", isBreached, count)
	}
}

func TestThreatBreachCacheHitUnit(t *testing.T) {
	ctx := context.Background()
	kvStore := newTestKVStore()

	// SHA-1 for "password" is 5BAA6 1E4C9B93F3F0682250B6CF8331B7EE68FD8
	cacheKey := "auth:threat:pwned:5BAA6"
	_ = kvStore.Set(ctx, cacheKey, "1E4C9B93F3F0682250B6CF8331B7EE68FD8:33000\n", 0)

	isBreached, count, err := CheckPwnedPassword(ctx, nil, kvStore, "password", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isBreached || count != 33000 {
		t.Fatalf("expected (true, 33000), got (%v, %d)", isBreached, count)
	}

	// Cache hit with clean password
	_ = kvStore.Set(ctx, cacheKey, "OTHER_SUFFIX:100\n", 0)
	isBreachedClean, countClean, cleanErr := CheckPwnedPassword(ctx, nil, kvStore, "password", false)
	if cleanErr != nil {
		t.Fatalf("unexpected error: %v", cleanErr)
	}
	if isBreachedClean || countClean != 0 {
		t.Fatalf("expected clean password, got (%v, %d)", isBreachedClean, countClean)
	}
}

func TestThreatBreachHTTPSuccessUnit(t *testing.T) {
	ctx := context.Background()
	kvStore := newTestKVStore()

	client := &mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			body := "  \n00000000000000000000000000000000000:1\n1E4C9B93F3F0682250B6CF8331B7EE68FD8:54200\n"
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		},
	}

	isBreached, count, err := CheckPwnedPassword(ctx, client, kvStore, "password", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isBreached || count != 54200 {
		t.Fatalf("expected (true, 54200), got (%v, %d)", isBreached, count)
	}

	// Verify cached in KVStore
	cachedContent, getErr := kvStore.Get(ctx, "auth:threat:pwned:5BAA6")
	if getErr != nil || cachedContent == "" {
		t.Fatalf("expected cache to be set, got: %s (err: %v)", cachedContent, getErr)
	}

	// Test non-numeric count fallback to 1
	nonNumericClient := &mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			body := "1E4C9B93F3F0682250B6CF8331B7EE68FD8:not_a_number\n"
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		},
	}
	isBreachedFallback, countFallback, fallbackErr := CheckPwnedPassword(ctx, nonNumericClient, nil, "password", false)
	if fallbackErr != nil || !isBreachedFallback || countFallback != 1 {
		t.Fatalf("expected count 1 fallback, got: (%v, %d, %v)", isBreachedFallback, countFallback, fallbackErr)
	}

	// Test unbreached password from API
	unbreachedClient := &mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			body := "00000000000000000000000000000000000:1\n"
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		},
	}
	isBreachedNone, countNone, noneErr := CheckPwnedPassword(ctx, unbreachedClient, nil, "password", false)
	if noneErr != nil || isBreachedNone || countNone != 0 {
		t.Fatalf("expected unbreached, got: (%v, %d, %v)", isBreachedNone, countNone, noneErr)
	}
}

func TestThreatBreachErrorsAndFailOpenUnit(t *testing.T) {
	ctx := context.Background()

	// 1. Request creation failure (nil context)
	//nolint:staticcheck
	if _, _, err := CheckPwnedPassword(nil, nil, nil, "password", false); err == nil {
		t.Fatal("expected error with nil context")
	}

	// 2. HTTP client error
	failingClient := &mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}

	if _, _, err := CheckPwnedPassword(ctx, failingClient, nil, "password", false); err == nil {
		t.Fatal("expected error when client fails and failOpen=false")
	}

	isBreachedOpen, countOpen, openErr := CheckPwnedPassword(ctx, failingClient, nil, "password", true)
	if openErr != nil || isBreachedOpen || countOpen != 0 {
		t.Fatalf("expected fail-open allowing password, got (%v, %d, %v)", isBreachedOpen, countOpen, openErr)
	}

	// 3. HTTP status not 200
	status500Client := &mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("server error")),
			}, nil
		},
	}

	if _, _, err := CheckPwnedPassword(ctx, status500Client, nil, "password", false); err == nil {
		t.Fatal("expected error on non-200 status and failOpen=false")
	}

	isBreached500Open, count500Open, open500Err := CheckPwnedPassword(ctx, status500Client, nil, "password", true)
	if open500Err != nil || isBreached500Open || count500Open != 0 {
		t.Fatalf("expected fail-open allowing password on 500, got (%v, %d, %v)", isBreached500Open, count500Open, open500Err)
	}

	// 4. Body read error
	bodyErrorClient := &mockHTTPClient{
		doFunc: func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       &failingBodyReader{},
			}, nil
		},
	}

	if _, _, err := CheckPwnedPassword(ctx, bodyErrorClient, nil, "password", false); err == nil {
		t.Fatal("expected error on body read failure and failOpen=false")
	}

	isBreachedReadOpen, countReadOpen, openReadErr := CheckPwnedPassword(ctx, bodyErrorClient, nil, "password", true)
	if openReadErr != nil || isBreachedReadOpen || countReadOpen != 0 {
		t.Fatalf("expected fail-open allowing password on read error, got (%v, %d, %v)", isBreachedReadOpen, countReadOpen, openReadErr)
	}
}

func TestThreatBreachDefaultClientUnit(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("1E4C9B93F3F0682250B6CF8331B7EE68FD8:10\n"))
	}))
	defer testServer.Close()

	originalBaseURL := pwnedPasswordsAPIBaseURL
	pwnedPasswordsAPIBaseURL = testServer.URL + "/"
	defer func() {
		pwnedPasswordsAPIBaseURL = originalBaseURL
	}()

	isBreached, count, err := CheckPwnedPassword(context.Background(), nil, nil, "password", false)
	if err != nil {
		t.Fatalf("unexpected error with default client: %v", err)
	}
	if !isBreached || count != 10 {
		t.Fatalf("expected (true, 10), got (%v, %d)", isBreached, count)
	}
}
