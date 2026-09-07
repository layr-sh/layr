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
	config := DefaultConfig()
	config.Data.Enabled = true
	config.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(config)
	defer UnloadConfig()

	cryptoKeyManager, err := NewCryptoKeyManager(config.Security.MasterEncryptionKey)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	server := NewServer(nil, cryptoKeyManager)

	// 1. Health Probe Flow
	healthRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	healthRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(healthRecorder, healthRequest)

	if healthRecorder.Code != http.StatusOK {
		t.Fatalf("health probe expected 200, got %d", healthRecorder.Code)
	}

	var healthPayload map[string]any
	if err := json.Unmarshal(healthRecorder.Body.Bytes(), &healthPayload); err != nil {
		t.Fatalf("failed to decode health JSON: %v", err)
	}
	if healthPayload["status"] != "healthy" {
		t.Fatalf("expected healthy status, got %v", healthPayload["status"])
	}

	// 2. Ready Probe Flow
	readyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	readyRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(readyRecorder, readyRequest)

	if readyRecorder.Code != http.StatusOK {
		t.Fatalf("ready probe expected 200, got %d", readyRecorder.Code)
	}

	// 3. Topology Discovery Flow
	topologyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/topology", nil)
	topologyRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(topologyRecorder, topologyRequest)

	if topologyRecorder.Code != http.StatusOK {
		t.Fatalf("topology expected 200, got %d", topologyRecorder.Code)
	}

	// 4. Metrics Probe Flow
	metricsRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	metricsRecorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(metricsRecorder, metricsRequest)

	if metricsRecorder.Code != http.StatusOK {
		t.Fatalf("metrics expected 200, got %d", metricsRecorder.Code)
	}
	if !strings.Contains(metricsRecorder.Body.String(), "uptime_seconds") {
		t.Fatalf("expected prometheus metrics in response: %s", metricsRecorder.Body.String())
	}

	// 5. Background Server Start and Clean Shutdown
	server.server.Addr = "127.0.0.1:0"
	serverErrorChannel := make(chan error, 1)
	go func() {
		serverErrorChannel <- server.Start()
	}()
	time.Sleep(50 * time.Millisecond)

	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownContext); err != nil {
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
