package core

import (
	"fmt"
	"net/http"
	"runtime"
	"time"
)

// /metrics - Minimal Prometheus exposition
func (server *Server) handleGetMetrics(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get prometheus metrics request")
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	responseWriter.Header().Set("Content-Type", "text/plain; version=0.0.4")
	responseWriter.WriteHeader(http.StatusOK)
	output := fmt.Sprintf("# HELP uptime_seconds Process uptime\n# TYPE uptime_seconds gauge\nuptime_seconds %f\n# HELP http_requests_total Total HTTP requests handled\n# TYPE http_requests_total counter\nhttp_requests_total %d\n# HELP memory_alloc_bytes Allocated heap bytes\n# TYPE memory_alloc_bytes gauge\nmemory_alloc_bytes %d\n",
		time.Since(server.uptime).Seconds(),
		server.requestCount.Load(),
		memStats.Alloc,
	)
	_, _ = responseWriter.Write([]byte(output))
}
