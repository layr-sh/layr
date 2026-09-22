package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCoreServerLifecycleAndProbeFlowE2E(t *testing.T) {
	kernel, cleanup := SetupTestKernel(t, nil)
	defer cleanup()

	server := kernel.Server()

	// 1. Health Probe Flow
	healthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	healthResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(healthResponseRecorder, healthRequest)

	if healthResponseRecorder.Code != http.StatusOK {
		t.Fatalf("health probe expected 200, got %d", healthResponseRecorder.Code)
	}

	var healthPayload map[string]any
	if err := json.Unmarshal(healthResponseRecorder.Body.Bytes(), &healthPayload); err != nil {
		t.Fatalf("failed to decode health JSON: %v", err)
	}
	if healthPayload["status"] != "healthy" {
		t.Fatalf("expected healthy status, got %v", healthPayload["status"])
	}

	// 2. Ready Probe Flow
	readyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	readyResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(readyResponseRecorder, readyRequest)

	if readyResponseRecorder.Code != http.StatusOK {
		t.Fatalf("ready probe expected 200, got %d", readyResponseRecorder.Code)
	}

	// 3. Manifest Discovery Flow
	manifestRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/manifest", nil)
	manifestResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(manifestResponseRecorder, manifestRequest)

	if manifestResponseRecorder.Code != http.StatusOK {
		t.Fatalf("manifest expected 200, got %d", manifestResponseRecorder.Code)
	}

	// 4. Metrics Probe Flow
	metricsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	metricsResponseRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(metricsResponseRecorder, metricsRequest)

	if metricsResponseRecorder.Code != http.StatusOK {
		t.Fatalf("metrics expected 200, got %d", metricsResponseRecorder.Code)
	}
	if !strings.Contains(metricsResponseRecorder.Body.String(), "uptime_seconds") {
		t.Fatalf("expected prometheus metrics in response: %s", metricsResponseRecorder.Body.String())
	}

	// 5. Background Server Start and Clean Shutdown
	server.server.Addr = "127.0.0.1:0"
	serverErrorChannel := make(chan error, 1)
	go func() {
		serverErrorChannel <- server.Start()
	}()
	time.Sleep(50 * time.Millisecond)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("expected clean shutdown, got %v", err)
	}

	select {
	case err := <-serverErrorChannel:
		if err != nil {
			t.Fatalf("server exited with unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not terminate within 2 seconds")
	}
}
