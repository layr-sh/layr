package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockServiceRunner struct {
	started           bool
	stopped           bool
	registered        bool
	openAPIRegistered bool
	startErr          error
}

func (mock *mockServiceRunner) Start(ctx context.Context) error {
	mock.started = true
	return mock.startErr
}

func (mock *mockServiceRunner) RegisterRoutes(router *Router, controlPlaneRouter *Router) {
	mock.registered = true
	mock.openAPIRegistered = true
}

func (mock *mockServiceRunner) Stop() error {
	mock.stopped = true
	return nil
}

func TestCoreKernelValidationUnit(t *testing.T) {
	// 1. Invalid config with zero functional services -> must fail
	invalidConfig := DefaultConfig()
	invalidConfig.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(invalidConfig)
	defer UnloadConfig()

	_, err := NewKernel()
	if err == nil {
		t.Fatal("expected NewKernel to fail on invalid config (zero functional services)")
	}

	// 2. Invalid master encryption key
	invalidMasterEncryptionKeyConfig := DefaultConfig()
	invalidMasterEncryptionKeyConfig.Data.Enabled = true
	invalidMasterEncryptionKeyConfig.Security.MasterEncryptionKey = "short"
	SetLoadedConfig(invalidMasterEncryptionKeyConfig)

	_, err = NewKernel()
	if err == nil {
		t.Fatal("expected NewKernel to fail on invalid master encryption key")
	}

	// 3. Valid config
	validConfig := DefaultConfig()
	validConfig.Data.Enabled = true
	validConfig.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(validConfig)

	kernel, err := NewKernel()
	if err != nil {
		t.Fatalf("unexpected NewKernel error: %v", err)
	}
	if kernel == nil {
		t.Fatal("expected non-nil Kernel instance")
	}

	if clientPublishableKey := kernel.ClientPublishableKey(); len(clientPublishableKey) != 64 {
		t.Fatalf("expected 64-char hex client publishable key, got %s", clientPublishableKey)
	}

	nilKeyKernel := &Kernel{}
	if clientPublishableKey := nilKeyKernel.ClientPublishableKey(); clientPublishableKey != "" {
		t.Fatalf("expected empty key for nil cryptoCryptoKeyManager, got %s", clientPublishableKey)
	}

	// Clean stop on unstarted kernel (all nil subsystems)
	if err := kernel.Stop(context.Background()); err != nil {
		t.Fatalf("expected clean stop, got: %v", err)
	}
}

func TestCoreKernelServiceFactoryAndGettersUnit(t *testing.T) {
	// Test RegisterServiceFactory and GetServiceFactory
	RegisterServiceFactory("test_custom_service", func(kernel *Kernel) (ServiceRunner, error) {
		return &mockServiceRunner{}, nil
	})
	serviceFactory, ok := GetServiceFactory("test_custom_service")
	if !ok || serviceFactory == nil {
		t.Fatal("expected to retrieve registered service factory")
	}
	_, notFound := GetServiceFactory("nonexistent_service_name_xyz")
	if notFound {
		t.Fatal("expected false for nonexistent service factory")
	}

	validConfig := DefaultConfig()
	validConfig.Data.Enabled = true
	validConfig.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(validConfig)
	defer UnloadConfig()

	kernel, err := NewKernel()
	if err != nil {
		t.Fatalf("unexpected NewKernel error: %v", err)
	}

	// Test Getters before Start
	if kernel.CryptoKeyManager() == nil {
		t.Error("expected non-nil CryptoKeyManager")
	}
	if kernel.DB() != nil {
		t.Error("expected nil DB before start")
	}
	if kernel.KVStore() != nil {
		t.Error("expected nil KVStore before start")
	}
	if kernel.WebhookEventBus() != nil {
		t.Error("expected nil WebhookEventBus before start")
	}
	if kernel.ServiceAccountManager() != nil {
		t.Error("expected nil ServiceAccountManager before start")
	}
	if kernel.WebhookManager() != nil {
		t.Error("expected nil WebhookManager before start")
	}

	// Test nil kernel accessor coverage
	var nilKernel *Kernel
	if nilKernel.DB() != nil {
		t.Error("expected nil pool from nil kernel")
	}
	if nilKernel.CryptoKeyManager() != nil {
		t.Error("expected nil cryptoCryptoKeyManager from nil kernel")
	}
	if nilKernel.KVStore() != nil {
		t.Error("expected nil kvStore from nil kernel")
	}
	if nilKernel.WebhookEventBus() != nil {
		t.Error("expected nil eventBus from nil kernel")
	}
	if nilKernel.ServiceAccountManager() != nil {
		t.Error("expected nil serviceAccountManager from nil kernel")
	}
	if nilKernel.WebhookManager() != nil {
		t.Error("expected nil webhookManager from nil kernel")
	}

	// Test Setters with nil and non-nil kernels
	var nilDB *DatabasePool
	nilKernel.SetDB(nilDB)
	nilKernel.SetKVStore(nil)
	nilKernel.SetWebhookEventBus(nil)
	nilKernel.SetServiceAccountManager(nil)

	kernel.SetDB(nilDB)
	kernel.SetKVStore(nil)
	kernel.SetWebhookEventBus(nil)
	kernel.SetServiceAccountManager(nil)

	if kernel.DB() != nil {
		t.Error("expected nil pool after SetDB(nil)")
	}
}

func TestCoreKernelServiceAccountHelpersUnit(t *testing.T) {
	keyHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	keyHeaderRequest.Header.Set("X-Layr-Service-Account-Key", "12345")
	if key := ExtractRequestServiceAccountKey(keyHeaderRequest); key != "12345" {
		t.Fatalf("expected 12345, got %s", key)
	}

	bearerHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	bearerHeaderRequest.Header.Set("Authorization", "Bearer 67890")
	if key := ExtractRequestServiceAccountKey(bearerHeaderRequest); key != "67890" {
		t.Fatalf("expected 67890, got %s", key)
	}

	noHeaderRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if key := ExtractRequestServiceAccountKey(noHeaderRequest); key != "" {
		t.Fatalf("expected empty key, got %s", key)
	}

	if isIPAllowed("invalid-ip", []string{"192.168.1.1"}) {
		t.Fatal("expected invalid-ip to be blocked")
	}
	if !isIPAllowed("192.168.1.1", []string{"192.168.1.1"}) {
		t.Fatal("expected exact IP match to be allowed")
	}
	if isIPAllowed("192.168.1.2", []string{"192.168.1.1"}) {
		t.Fatal("expected different IP to be blocked")
	}
	if !isIPAllowed("10.0.5.20", []string{"10.0.0.0/16"}) {
		t.Fatal("expected CIDR match to be allowed")
	}
	if isIPAllowed("11.0.5.20", []string{"10.0.0.0/16"}) {
		t.Fatal("expected non-matching CIDR to be blocked")
	}
}

func TestCoreKernelWebhookHelpersUnit(t *testing.T) {
	signature := ComputeWebhookSignature("secret", "1776484800", []byte(`{"test":true}`))
	if !strings.HasPrefix(signature, "sha256=") {
		t.Fatalf("expected sha256= prefix, got: %s", signature)
	}

	if !matchWebhookEventPattern("*", "data.table.created") {
		t.Fatal("expected wildcard match")
	}
	if !matchWebhookEventPattern("data.*", "data.table.created") {
		t.Fatal("expected prefix match")
	}
	if matchWebhookEventPattern("data.*", "auth.user.created") {
		t.Fatal("expected prefix mismatch")
	}
}
