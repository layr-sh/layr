package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// /healthz - Liveness probe
func (server *Server) handleHealthz(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(HealthResponse{
		Status:        "healthy",
		UptimeSeconds: time.Since(server.uptime).Seconds(),
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	})
}

// /readyz - Readiness probe
func (server *Server) handleReadyz(responseWriter http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()

	databaseStatus := "ok"
	statusCode := http.StatusOK
	if server.db != nil {
		if err := server.db.Ping(ctx); err != nil {
			databaseStatus = fmt.Sprintf("error: %v", err)
			statusCode = http.StatusServiceUnavailable
		}
	}

	statusText := "ready"
	if statusCode != http.StatusOK {
		statusText = "unready"
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(ReadyResponse{
		Status:          statusText,
		Database:        databaseStatus,
		EnabledServices: GetConfig().GetEnabledServices(),
	})
}
