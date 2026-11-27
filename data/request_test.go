package data

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"layr.sh/core"
)

type failedRateDriver struct{ *core.TestInMemoryKVDriver }

func (driver *failedRateDriver) Increment(context.Context, string, time.Duration) (int64, error) {
	return 0, errors.New("rate store offline")
}

func TestDataRequestCORSAndLimitsUnit(t *testing.T) {
	service := NewService(core.NewTestKernel(nil))
	call := func(ctx context.Context, method, path, origin, headers string) *httptest.ResponseRecorder {
		request := httptest.NewRequestWithContext(ctx, method, path, nil)
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		request.Header.Set("Access-Control-Request-Headers", headers)
		request.Header.Set("Access-Control-Request-Method", http.MethodGet)
		responseRecorder := httptest.NewRecorder()
		service.AuthorizeRequest(responseRecorder, request)
		return responseRecorder
	}
	ctx := context.Background()
	for _, path := range []string{"/healthz", "/v1/_/data/config", "/unrelated"} {
		t.Run("case", func(t *testing.T) {
			if call(ctx, http.MethodGet, path, "https://untrusted.test", "").Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("Data middleware affected another route")
			}
		})
	}
	responseRecorder := call(ctx, http.MethodOptions, "/v1/data/public/items", "https://app.test", "Publishable-Key, Content-Type")
	if responseRecorder.Code != http.StatusNoContent || responseRecorder.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("invalid public preflight: %d %v", responseRecorder.Code, responseRecorder.Header())
	}
	config := service.configManager.Get()
	config.CORS.AllowedOrigins = []string{"https://app.test"}
	config.CORS.AllowedHeaders = []string{"Authorization"}
	service.configManager.SetMemoryConfig(config)
	responseRecorder = call(ctx, http.MethodGet, "/v1/graphql", "https://app.test", "")
	if responseRecorder.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("explicit trusted origin did not allow credentials")
	}
	if call(ctx, http.MethodGet, "/v1/realtime", "https://other.test", "").Code != http.StatusForbidden {
		t.Fatal("untrusted origin was accepted")
	}
	if call(ctx, http.MethodOptions, "/v1/data/public/items", "https://app.test", "X-Denied").Code != http.StatusForbidden {
		t.Fatal("unconfigured request header accepted")
	}
	if call(ctx, http.MethodOptions, "/v1/graphql", "https://app.test", "Authorization, ,authorization").Code != http.StatusNoContent {
		t.Fatal("configured header was rejected")
	}
	config.CORS.AllowedHeaders = []string{"*"}
	service.configManager.SetMemoryConfig(config)
	responseRecorder = call(ctx, http.MethodOptions, "/v1/graphql", "https://app.test", "X-Custom")
	if responseRecorder.Code != http.StatusNoContent || responseRecorder.Header().Get("Access-Control-Allow-Headers") != "X-Custom" {
		t.Fatal("credentialed wildcard preflight did not echo requested headers")
	}
	invalidMethodRequest := httptest.NewRequestWithContext(ctx, http.MethodOptions, "/v1/graphql", nil)
	invalidMethodRequest.Header.Set("Origin", "https://app.test")
	invalidMethodRequest.Header.Set("Access-Control-Request-Method", http.MethodTrace)
	invalidMethodResponseRecorder := httptest.NewRecorder()
	if service.AuthorizeRequest(invalidMethodResponseRecorder, invalidMethodRequest) || invalidMethodResponseRecorder.Code != http.StatusForbidden {
		t.Fatal("unsupported preflight method was accepted")
	}
	config.RateLimiting = RateLimitingConfig{Enabled: true, RequestsPerMinute: 1, Burst: 5}
	service.configManager.SetMemoryConfig(config)
	aliceCtx := core.WithAuthContext(ctx, core.AuthContext{UserID: "aliceCtx"})
	if call(aliceCtx, http.MethodGet, "/v1/data/public/items", "", "").Code != http.StatusOK {
		t.Fatal("first request rejected")
	}
	responseRecorder = call(aliceCtx, http.MethodGet, "/v1/graphql", "", "")
	if responseRecorder.Code != http.StatusTooManyRequests || responseRecorder.Header().Get("Retry-After") == "" {
		t.Fatal("minute quota not enforced across endpoints")
	}
	bobCtx := core.WithAuthContext(ctx, core.AuthContext{ServiceAccountID: "bobCtx"})
	if call(bobCtx, http.MethodGet, "/v1/graphql", "", "").Code != http.StatusOK {
		t.Fatal("another caller inherited a quota")
	}
	config.RateLimiting = RateLimitingConfig{Enabled: true, RequestsPerMinute: 100, Burst: 1}
	service.configManager.SetMemoryConfig(config)
	denied := false
	for index := 0; index < 3; index++ {
		if call(ctx, http.MethodGet, "/v1/graphql", "", "").Code == http.StatusTooManyRequests {
			denied = true
		}
	}
	if !denied {
		t.Fatal("burst limit not enforced")
	}
	driver := &failedRateDriver{core.NewTestInMemoryKVDriver()}
	service = NewService(core.NewTestKernel(nil, core.WithKVStore(core.NewKVStoreFromDriver(driver))))
	service.configManager.SetMemoryConfig(config)
	if call(ctx, http.MethodGet, "/v1/graphql", "", "").Code != http.StatusServiceUnavailable {
		t.Fatal("rate-store failure did not fail closed")
	}
}
