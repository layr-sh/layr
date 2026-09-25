package core

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func (mock *mockServiceRunner) RegisterRoutes(baseRouter *Router, controlPlaneRouter *Router) {
	mock.registered = true
	mock.openAPIRegistered = true
}

func (mock *mockServiceRunner) RegisterHostRoutes(server *Server) {
}

func (mock *mockServiceRunner) Stop() {
	mock.stopped = true
}

func TestCoreKernelValidationUnit(t *testing.T) {
	// 1. Invalid config with zero functional services -> must fail
	invalidConfig := DefaultConfig()
	invalidConfig.Data.Enabled = false
	invalidConfig.Auth.Enabled = false
	invalidConfig.FileStorage.Enabled = false
	invalidConfig.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(invalidConfig)
	defer UnloadConfig()

	_, err := NewKernel(nil)
	if err == nil {
		t.Fatal("expected NewKernel to fail on invalid config (zero functional services)")
	}

	// 2. Invalid master encryption key
	invalidMasterEncryptionKeyConfig := DefaultConfig()
	invalidMasterEncryptionKeyConfig.Data.Enabled = true
	invalidMasterEncryptionKeyConfig.Security.MasterEncryptionKey = "short"
	SetLoadedConfig(invalidMasterEncryptionKeyConfig)

	_, err = NewKernel(nil)
	if err == nil {
		t.Fatal("expected NewKernel to fail on invalid master encryption key")
	}

	// 3. Valid config
	validConfig := DefaultConfig()
	validConfig.Data.Enabled = true
	validConfig.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(validConfig)

	kernel, err := NewKernel(nil)
	if err != nil {
		t.Fatalf("unexpected NewKernel error: %v", err)
	}

	if publishableKey := kernel.PublishableKey(); len(publishableKey) != 64 {
		t.Fatalf("expected 64-char hex publishable key, got %s", publishableKey)
	}

	// Clean stop on unstarted kernel (all nil subsystems)
	kernel.Stop(context.Background())
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

	kernel, err := NewKernel(nil)
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
	if kernel.EventBus() != nil {
		t.Error("expected nil EventBus before start")
	}
	if kernel.EventManager() != nil {
		t.Error("expected nil EventManager before start")
	}
	if kernel.EventHookManager() != nil {
		t.Error("expected nil EventHookManager before start")
	}
	if kernel.ServiceAccountManager() != nil {
		t.Error("expected nil ServiceAccountManager before start")
	}
	if kernel.JWTSigner() == nil {
		t.Error("expected non-nil JWTSigner before start")
	}
	if kernel.Server() != nil {
		t.Error("expected nil Server before start")
	}

	// Test SetServer on kernel
	kernel.SetServer(nil)
	if kernel.Server() != nil {
		t.Error("expected nil server after SetServer(nil)")
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

func TestCoreKernelEventHookHelpersUnit(t *testing.T) {
	signature := ComputeHookSignature("secret", "1776484800", []byte(`{"test":true}`))
	if len(signature) == 0 {
		t.Fatal("expected non-empty signature")
	}

	if !MatchEventPattern("*", "data.table.created") {
		t.Fatal("expected wildcard match")
	}
	if !MatchEventPattern("data.*", "data.table.created") {
		t.Fatal("expected prefix match")
	}
	if MatchEventPattern("data.*", "auth.user.created") {
		t.Fatal("expected prefix mismatch")
	}
}

func TestCoreKernelValidateSubsystemsUnit(t *testing.T) {
	var nilKernel *Kernel
	if nilErr := nilKernel.ValidateSubsystems(); nilErr == nil {
		t.Fatal("expected error on nil kernel")
	}

	kernel := &Kernel{}
	if err := kernel.ValidateSubsystems(); err == nil {
		t.Fatal("expected error on uninitialized kernel db")
	}

	kernel.db = &DatabasePool{}
	if err := kernel.ValidateSubsystems(); err == nil {
		t.Fatal("expected error on uninitialized crypto key manager")
	}

	cryptoKeyManager, _ := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	kernel.cryptoKeyManager = cryptoKeyManager
	if err := kernel.ValidateSubsystems(); err == nil {
		t.Fatal("expected error on uninitialized node registry")
	}

	kernel.nodeManager = &NodeManager{}
	if err := kernel.ValidateSubsystems(); err == nil {
		t.Fatal("expected error on uninitialized kv store")
	}

	kernel.kvStore = &KVStore{}
	if err := kernel.ValidateSubsystems(); err == nil {
		t.Fatal("expected error on uninitialized event bus")
	}

	kernel.eventBus = &EventBus{}
	if err := kernel.ValidateSubsystems(); err == nil {
		t.Fatal("expected error on uninitialized event manager")
	}

	kernel.eventManager = &EventManager{}
	if err := kernel.ValidateSubsystems(); err == nil {
		t.Fatal("expected error on uninitialized event hook manager")
	}

	kernel.eventHookManager = &EventHookManager{}
	if err := kernel.ValidateSubsystems(); err == nil {
		t.Fatal("expected error on uninitialized service account manager")
	}

	kernel.serviceAccountManager = &ServiceAccountManager{}
	if err := kernel.ValidateSubsystems(); err == nil {
		t.Fatal("expected error on uninitialized jwt signer")
	}

	kernel.jwtSigner = &JWTSigner{}
	if err := kernel.ValidateSubsystems(); err == nil {
		t.Fatal("expected error on uninitialized server")
	}

	kernel.server = &Server{}
	if err := kernel.ValidateSubsystems(); err != nil {
		t.Fatalf("expected nil error on fully initialized kernel, got: %v", err)
	}
}

func TestCoreKernelWithDBUnit(t *testing.T) {
	validConfig := DefaultConfig()
	validConfig.Data.Enabled = true
	validConfig.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	SetLoadedConfig(validConfig)
	defer UnloadConfig()

	mockDB := &DatabasePool{}
	kernel, err := NewKernel(mockDB)
	if err != nil {
		t.Fatalf("unexpected NewKernel error: %v", err)
	}
	defer kernel.eventBus.Close()

	if kernel.DB() != mockDB {
		t.Errorf("expected DB %v, got %v", mockDB, kernel.DB())
	}
	if kernel.ServiceAccountManager() == nil {
		t.Error("expected non-nil ServiceAccountManager")
	}
	if kernel.EventBus() == nil {
		t.Error("expected non-nil EventBus")
	}
	if kernel.EventManager() == nil {
		t.Error("expected non-nil EventManager")
	}
	if kernel.EventHookManager() == nil {
		t.Error("expected non-nil EventHookManager")
	}

	// Test nil node
	if kernel.Node().NodeName != "" {
		t.Errorf("expected empty Node, got %v", kernel.Node())
	}
	if kernel.NodeManager() != nil {
		t.Error("expected nil NodeManager")
	}

	// Test non-nil node
	dummyNodeManager := &NodeManager{node: Node{NodeName: "test-node"}}
	kernel.nodeManager = dummyNodeManager
	if kernel.NodeManager() != dummyNodeManager {
		t.Errorf("expected %v, got %v", dummyNodeManager, kernel.NodeManager())
	}
	if kernel.Node().NodeName != "test-node" {
		t.Errorf("expected test-node, got %s", kernel.Node().NodeName)
	}
}
