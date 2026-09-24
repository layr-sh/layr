package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// /healthz - Liveness probe
func (server *Server) handleGetHealth(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get liveness probe request")
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(GetHealthResponse{
		Status:        "healthy",
		UptimeSeconds: time.Since(server.uptime).Seconds(),
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	})
}

// /readyz - Readiness probe
func (server *Server) handleGetReadiness(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get readiness probe request")
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()

	databaseStatus := "ok"
	statusCode := http.StatusOK
	if err := server.kernel.DB().Ping(ctx); err != nil {
		databaseStatus = fmt.Sprintf("error: %v", err)
		statusCode = http.StatusServiceUnavailable
	}

	statusText := "ready"
	if statusCode != http.StatusOK {
		statusText = "unready"
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(GetReadinessResponse{
		Status:          statusText,
		Database:        databaseStatus,
		EnabledServices: GetConfig().GetEnabledServices(),
	})
}
