package data

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"time"

	"layr.sh/core"
)

// AuthorizeRequest applies Data API CORS and distributed request limits after
// the gateway has resolved the caller, before it dispatches a public route.
func (service *Service) AuthorizeRequest(responseWriter http.ResponseWriter, request *http.Request) bool {
	path := request.URL.Path
	if !strings.HasPrefix(path, "/v1/data/") && path != "/v1/graphql" && path != "/v1/realtime" {
		return true
	}
	config := service.configManager.Get()
	origin := request.Header.Get("Origin")
	responseWriter.Header().Add("Vary", "Origin")
	if origin != "" {
		allowed := false
		for _, candidate := range config.CORS.AllowedOrigins {
			if candidate == origin {
				responseWriter.Header().Set("Access-Control-Allow-Origin", origin)
				responseWriter.Header().Set("Access-Control-Allow-Credentials", "true")
				allowed = true
				break
			}
			if candidate == "*" {
				responseWriter.Header().Set("Access-Control-Allow-Origin", "*")
				allowed = true
			}
		}
		if !allowed {
			core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Origin is not allowed")
			return false
		}
		responseWriter.Header().Set("Access-Control-Expose-Headers", "Total-Count, Content-Range, Range-Unit, Cache-Key, Cache-Status, Retry-After")
		if request.Method == http.MethodOptions {
			method := request.Header.Get("Access-Control-Request-Method")
			switch method {
			case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
			default:
				core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Request method is not allowed")
				return false
			}
			var allowedHeaders []string
			for _, header := range strings.Split(request.Header.Get("Access-Control-Request-Headers"), ",") {
				header = strings.TrimSpace(header)
				if header == "" {
					continue
				}
				found := false
				for _, candidate := range config.CORS.AllowedHeaders {
					if candidate == "*" || strings.EqualFold(candidate, header) {
						found = true
						break
					}
				}
				allowedHeaders = append(allowedHeaders, header)
				if !found {
					core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Request header is not allowed")
					return false
				}
			}
			responseWriter.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			responseWriter.Header().Set("Access-Control-Allow-Headers", strings.Join(allowedHeaders, ", "))
			responseWriter.Header().Add("Vary", "Access-Control-Request-Headers")
			responseWriter.Header().Add("Vary", "Access-Control-Request-Method")
			responseWriter.WriteHeader(http.StatusNoContent)
			return false
		}
	}
	if !config.RateLimiting.Enabled {
		return true
	}
	authContext := core.GetAuthContext(request.Context())
	identity := authContext.ServiceAccountID + ":" + authContext.UserID
	if identity == ":" {
		identity = core.ExtractRequestClientIP(request)
	}
	fingerprint := sha256.Sum256([]byte(identity))
	now := time.Now().Unix()
	// Fixed minute quotas and one-second burst windows are shared across nodes.
	windows := []struct {
		name    string
		bucket  int64
		ttl     time.Duration
		maximum int
		retry   int64
	}{
		{"minute", now / 60, 2 * time.Minute, config.RateLimiting.RequestsPerMinute, 60 - now%60},
		{"burst", now, 2 * time.Second, config.RateLimiting.Burst, 1},
	}
	for _, window := range windows {
		count, err := service.kernel.KVStore().Increment(request.Context(), fmt.Sprintf("data:rate:%x:%s:%d", fingerprint, window.name, window.bucket), window.ttl)
		if err != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusServiceUnavailable, "Rate limiter is unavailable")
			return false
		}
		if count > int64(window.maximum) {
			responseWriter.Header().Set("Retry-After", fmt.Sprint(window.retry))
			core.WriteErrorResponse(responseWriter, request, http.StatusTooManyRequests, "Data request limit exceeded")
			return false
		}
	}
	return true
}
