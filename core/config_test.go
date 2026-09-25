package core

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreConfigDefaultsUnit(t *testing.T) {
	config := DefaultConfig()

	// Verify all root properties and project metadata
	if config.Version != "1" {
		t.Fatalf("expected version '1', got '%s'", config.Version)
	}
	if config.Project.Name != "layr-app" {
		t.Fatalf("expected project name 'layr-app', got '%s'", config.Project.Name)
	}
	if config.Project.Description != "Layr Application" {
		t.Fatalf("expected project description 'Layr Application', got '%s'", config.Project.Description)
	}

	// Verify server defaults
	if config.Server.ListenAddr != ":8080" {
		t.Fatalf("expected listen addr ':8080', got '%s'", config.Server.ListenAddr)
	}
	if config.Server.BaseURL != "http://localhost:8080" {
		t.Fatalf("expected base url 'http://localhost:8080', got '%s'", config.Server.BaseURL)
	}

	// Verify database defaults
	if config.Database.URL != ".layr/data" {
		t.Fatalf("expected database url '.layr/data', got '%s'", config.Database.URL)
	}
	if config.Database.MaxConnections != 25 {
		t.Fatalf("expected max_connections 25, got %d", config.Database.MaxConnections)
	}
	if config.Database.MinConnections != 2 {
		t.Fatalf("expected min_connections 2, got %d", config.Database.MinConnections)
	}
	if config.Database.ConnectionTimeoutMs != 5000 {
		t.Fatalf("expected connection_timeout_ms 5000, got %d", config.Database.ConnectionTimeoutMs)
	}
	if config.Database.IdleTimeoutMs != 300000 {
		t.Fatalf("expected idle_timeout_ms 300000, got %d", config.Database.IdleTimeoutMs)
	}
	if config.Database.MaxLifetimeMs != 1800000 {
		t.Fatalf("expected max_lifetime_ms 1800000, got %d", config.Database.MaxLifetimeMs)
	}
	if config.Database.HealthCheckPeriodMs != 15000 {
		t.Fatalf("expected health_check_period_ms 15000, got %d", config.Database.HealthCheckPeriodMs)
	}
	if config.Database.SSLMode != "" {
		t.Fatalf("expected empty default ssl_mode, got '%s'", config.Database.SSLMode)
	}

	// Verify KV store default
	if config.KVStore.Backend != "database" {
		t.Fatalf("expected kv_store backend 'database', got '%s'", config.KVStore.Backend)
	}

	// Verify Console, Data, Auth, and FileStorage default to true, other functional services default to false
	if !config.Console.Enabled {
		t.Fatal("expected console.enabled to default to true")
	}
	if !config.Data.Enabled {
		t.Fatal("expected data.enabled to default to true")
	}
	if !config.Auth.Enabled {
		t.Fatal("expected auth.enabled to default to true")
	}
	if !config.FileStorage.Enabled {
		t.Fatal("expected file_storage.enabled to default to true")
	}
	if config.Tasks.Enabled || config.Image.Enabled || config.Function.Enabled {
		t.Fatal("expected tasks, image, and function to default to false")
	}
}

func TestCoreConfigEnableServiceEdgeCasesUnit(t *testing.T) {
	// Case-insensitive service enablement
	caseVariations := []struct {
		inputName      string
		canonicalField string
	}{
		{inputName: "DATA", canonicalField: "data"},
		{inputName: "Data", canonicalField: "data"},
		{inputName: "dAtA", canonicalField: "data"},
		{inputName: "AUTH", canonicalField: "auth"},
		{inputName: "Auth", canonicalField: "auth"},
		{inputName: "TASKS", canonicalField: "tasks"},
		{inputName: "Tasks", canonicalField: "tasks"},
		{inputName: "FILE_STORAGE", canonicalField: "file_storage"},
		{inputName: "FileStorage", canonicalField: "file_storage"},
		{inputName: "IMAGE", canonicalField: "image"},
		{inputName: "Image", canonicalField: "image"},
		{inputName: "FUNCTION", canonicalField: "function"},
		{inputName: "Function", canonicalField: "function"},
		{inputName: "CONSOLE", canonicalField: "console"},
		{inputName: "Console", canonicalField: "console"},
	}

	for _, tableTest := range caseVariations {
		t.Run(tableTest.inputName, func(t *testing.T) {
			config := DefaultConfig()
			if err := config.EnableService(tableTest.inputName); err != nil {
				t.Fatalf("expected success enabling service '%s', got: %v", tableTest.inputName, err)
			}
			if !config.IsServiceEnabled(tableTest.inputName) {
				t.Fatalf("expected service '%s' to be reported as enabled", tableTest.inputName)
			}
			if !config.IsServiceEnabled(tableTest.canonicalField) {
				t.Fatalf("expected canonical service '%s' to be reported as enabled", tableTest.canonicalField)
			}
		})
	}

	// Invalid / unknown service names
	invalidServiceNames := []string{
		"", "   ", "unknown", "invalid", "redis", "postgres", "studio", "unsupported", "custom", "123", "!@#$%", "auth_service",
	}
	for _, invalidName := range invalidServiceNames {
		t.Run(fmt.Sprintf("%q", invalidName), func(t *testing.T) {
			config := DefaultConfig()
			if err := config.EnableService(invalidName); err == nil {
				t.Fatalf("expected error enabling invalid service name '%s'", invalidName)
			}
			if config.IsServiceEnabled(invalidName) {
				t.Fatalf("expected invalid service '%s' to not be enabled", invalidName)
			}
		})
	}
}

func TestCoreConfigServiceQueriesAndAggregationUnit(t *testing.T) {
	// 1. Zero functional services enabled
	config := DefaultConfig()
	config.Data.Enabled = false
	config.Auth.Enabled = false
	config.FileStorage.Enabled = false
	if config.HasAnyFunctionalServiceEnabled() {
		t.Fatal("expected HasAnyFunctionalServiceEnabled to be false on fresh default config")
	}
	if len(config.GetFunctionalServices()) != 0 {
		t.Fatalf("expected 0 functional services, got %d", len(config.GetFunctionalServices()))
	}
	enabledServicesWithConsole := config.GetEnabledServices()
	if len(enabledServicesWithConsole) != 1 || enabledServicesWithConsole[0] != "console" {
		t.Fatalf("expected [console], got %v", enabledServicesWithConsole)
	}

	// 2. Each of the 6 functional services enabled individually
	allFunctionalServices := []string{"data", "auth", "tasks", "file_storage", "image", "function"}
	for _, serviceName := range allFunctionalServices {
		t.Run(serviceName, func(t *testing.T) {
			singleServiceConfig := DefaultConfig()
			singleServiceConfig.Data.Enabled = false
			singleServiceConfig.Auth.Enabled = false
			singleServiceConfig.FileStorage.Enabled = false
			singleServiceConfig.Console.Enabled = false
			if err := singleServiceConfig.EnableService(serviceName); err != nil {
				t.Fatalf("failed to enable service '%s': %v", serviceName, err)
			}
			if !singleServiceConfig.HasAnyFunctionalServiceEnabled() {
				t.Fatalf("expected HasAnyFunctionalServiceEnabled true for '%s'", serviceName)
			}
			functionalList := singleServiceConfig.GetFunctionalServices()
			if len(functionalList) != 1 || functionalList[0] != serviceName {
				t.Fatalf("expected [%s], got %v", serviceName, functionalList)
			}
			enabledList := singleServiceConfig.GetEnabledServices()
			if len(enabledList) != 1 || enabledList[0] != serviceName {
				t.Fatalf("expected [%s], got %v", serviceName, enabledList)
			}
		})
	}

	// 3. All 6 functional services enabled with console enabled
	allEnabledConfig := DefaultConfig()
	for _, serviceName := range allFunctionalServices {
		_ = allEnabledConfig.EnableService(serviceName)
	}
	if !allEnabledConfig.HasAnyFunctionalServiceEnabled() {
		t.Fatal("expected HasAnyFunctionalServiceEnabled true when all services are enabled")
	}
	if len(allEnabledConfig.GetFunctionalServices()) != 6 {
		t.Fatalf("expected 6 functional services, got %d", len(allEnabledConfig.GetFunctionalServices()))
	}
	if len(allEnabledConfig.GetEnabledServices()) != 7 {
		t.Fatalf("expected 7 total enabled services, got %d", len(allEnabledConfig.GetEnabledServices()))
	}

	// 4. All 6 functional services enabled with console disabled
	allEnabledConfig.Console.Enabled = false
	if len(allEnabledConfig.GetEnabledServices()) != 6 {
		t.Fatalf("expected 6 total enabled services when console is disabled, got %d", len(allEnabledConfig.GetEnabledServices()))
	}
}

func TestCoreConfigValidationVersionEdgeCasesUnit(t *testing.T) {
	validHexKey := strings.Repeat("a", 64)

	invalidVersions := []string{"", "0", "2", "1.0", "v1", "alpha", "-1", "   ", "null"}
	for _, invalidVersion := range invalidVersions {
		t.Run(fmt.Sprintf("%q", invalidVersion), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Version = invalidVersion

			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error for invalid version '%s'", invalidVersion)
			}
		})
	}
}

func TestCoreConfigValidationDatabaseURLEdgeCasesUnit(t *testing.T) {
	validHexKey := strings.Repeat("b", 64)

	invalidURLs := []string{"", "   ", "\t", "\n", " \t\n "}
	for _, invalidURL := range invalidURLs {
		t.Run(fmt.Sprintf("%q", invalidURL), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.URL = invalidURL

			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error for empty/whitespace database url '%s'", invalidURL)
			}
		})
	}
}

func TestCoreConfigValidationDatabasePoolBoundariesUnit(t *testing.T) {
	validHexKey := strings.Repeat("c", 64)

	// MaxConnections: [1, 1000]
	invalidMaxConnections := []int{-100, -1, 0, 1001, 2000, 50000}
	for _, invalidMax := range invalidMaxConnections {
		t.Run(fmt.Sprintf("InvalidMaxConn_%d", invalidMax), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.MaxConnections = invalidMax
			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error for max_connections %d", invalidMax)
			}
		})
	}

	// Boundary values for MaxConnections: 1 and 1000
	for _, validMax := range []int{1, 100, 1000} {
		t.Run(fmt.Sprintf("ValidMaxConn_%d", validMax), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.MaxConnections = validMax
			config.Database.MinConnections = 1
			if err := config.Validate(); err != nil {
				t.Fatalf("expected valid configuration for max_connections %d, got: %v", validMax, err)
			}
		})
	}

	// MinConnections: [0, max_connections]
	invalidMinConnections := []struct {
		min int
		max int
	}{
		{min: -1, max: 25},
		{min: -100, max: 25},
		{min: 26, max: 25},
		{min: 100, max: 10},
	}
	for _, tableTest := range invalidMinConnections {
		t.Run(fmt.Sprintf("InvalidMinConn_%d_Max_%d", tableTest.min, tableTest.max), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.MaxConnections = tableTest.max
			config.Database.MinConnections = tableTest.min
			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error for min_connections %d (max: %d)", tableTest.min, tableTest.max)
			}
		})
	}

	// Boundary values for MinConnections: 0 and equal to max_connections
	for _, validMin := range []int{0, 25} {
		t.Run(fmt.Sprintf("ValidMinConn_%d", validMin), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.MaxConnections = 25
			config.Database.MinConnections = validMin
			if err := config.Validate(); err != nil {
				t.Fatalf("expected valid configuration for min_connections %d, got: %v", validMin, err)
			}
		})
	}

	// ConnectionTimeoutMs: [100, 60000]
	invalidConnectionTimeouts := []int{-1, 0, 50, 99, 60001, 100000}
	for _, timeout := range invalidConnectionTimeouts {
		t.Run(fmt.Sprintf("InvalidConnTimeout_%d", timeout), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.ConnectionTimeoutMs = timeout
			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error for connection_timeout_ms %d", timeout)
			}
		})
	}
	for _, validTimeout := range []int{100, 5000, 60000} {
		t.Run(fmt.Sprintf("ValidConnTimeout_%d", validTimeout), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.ConnectionTimeoutMs = validTimeout
			if err := config.Validate(); err != nil {
				t.Fatalf("expected valid configuration for connection_timeout_ms %d, got: %v", validTimeout, err)
			}
		})
	}

	// IdleTimeoutMs: [1000, 86400000]
	invalidIdleTimeouts := []int{-1, 0, 500, 999, 86400001, 100000000}
	for _, timeout := range invalidIdleTimeouts {
		t.Run(fmt.Sprintf("InvalidIdleTimeout_%d", timeout), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.IdleTimeoutMs = timeout
			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error for idle_timeout_ms %d", timeout)
			}
		})
	}
	for _, validTimeout := range []int{1000, 300000, 86400000} {
		t.Run(fmt.Sprintf("ValidIdleTimeout_%d", validTimeout), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.IdleTimeoutMs = validTimeout
			if err := config.Validate(); err != nil {
				t.Fatalf("expected valid configuration for idle_timeout_ms %d, got: %v", validTimeout, err)
			}
		})
	}

	// MaxLifetimeMs: [1000, 86400000]
	invalidMaxLifetimes := []int{-1, 0, 999, 86400001, 200000000}
	for _, timeout := range invalidMaxLifetimes {
		t.Run(fmt.Sprintf("InvalidMaxLifetime_%d", timeout), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.MaxLifetimeMs = timeout
			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error for max_lifetime_ms %d", timeout)
			}
		})
	}
	for _, validTimeout := range []int{1000, 1800000, 86400000} {
		t.Run(fmt.Sprintf("ValidMaxLifetime_%d", validTimeout), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.MaxLifetimeMs = validTimeout
			if err := config.Validate(); err != nil {
				t.Fatalf("expected valid configuration for max_lifetime_ms %d, got: %v", validTimeout, err)
			}
		})
	}

	// HealthCheckPeriodMs: [1000, 3600000]
	invalidHealthChecks := []int{-1, 0, 999, 3600001, 10000000}
	for _, period := range invalidHealthChecks {
		t.Run(fmt.Sprintf("InvalidHealthCheck_%d", period), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.HealthCheckPeriodMs = period
			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error for health_check_period_ms %d", period)
			}
		})
	}
	for _, validPeriod := range []int{1000, 15000, 3600000} {
		t.Run(fmt.Sprintf("ValidHealthCheck_%d", validPeriod), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.HealthCheckPeriodMs = validPeriod
			if err := config.Validate(); err != nil {
				t.Fatalf("expected valid configuration for health_check_period_ms %d, got: %v", validPeriod, err)
			}
		})
	}
}

func TestCoreConfigValidationSSLModeEdgeCasesUnit(t *testing.T) {
	validHexKey := strings.Repeat("d", 64)

	// Valid SSL modes
	validSSLModes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}
	for _, sslMode := range validSSLModes {
		t.Run(fmt.Sprintf("Valid_%s", sslMode), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.SSLMode = sslMode

			if err := config.Validate(); err != nil {
				t.Fatalf("expected valid configuration for ssl_mode '%s', got: %v", sslMode, err)
			}
		})
	}

	// Invalid SSL modes
	invalidSSLModes := []string{"disabled", "enabled", "strict", "verify", "1", "false", "REQUIRE"}
	for _, sslMode := range invalidSSLModes {
		t.Run(fmt.Sprintf("Invalid_%s", sslMode), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.Database.SSLMode = sslMode

			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error for invalid ssl_mode '%s'", sslMode)
			}
		})
	}
}

func TestCoreConfigValidationMasterEncryptionKeyEdgeCasesUnit(t *testing.T) {
	// Empty / whitespace key
	for _, emptyKey := range []string{"", "   ", "\t\n"} {
		t.Run(fmt.Sprintf("%q", emptyKey), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = emptyKey
			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error for empty master key '%s'", emptyKey)
			}
		})
	}

	// Invalid length keys
	invalidLengthKeys := []string{
		"1",
		"1234567890",
		strings.Repeat("a", 32),
		strings.Repeat("a", 63),
		strings.Repeat("a", 65),
		strings.Repeat("a", 128),
	}
	for _, invalidLengthKey := range invalidLengthKeys {
		t.Run(fmt.Sprintf("Len_%d", len(invalidLengthKey)), func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = invalidLengthKey
			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error for master key with length %d", len(invalidLengthKey))
			}
		})
	}

	// Length 64 but invalid hex characters
	invalidHexKeys := []string{
		strings.Repeat("z", 64),
		strings.Repeat("g", 64),
		strings.Repeat("a", 63) + "!",
		strings.Repeat("a", 63) + " ",
		strings.Repeat("a", 30) + "xx" + strings.Repeat("a", 32),
	}
	for _, invalidHexKey := range invalidHexKeys {
		t.Run(invalidHexKey, func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = invalidHexKey
			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error for non-hex master key: %s", invalidHexKey)
			}
		})
	}

	// Valid 64-character hex keys
	validHexKeys := []string{
		strings.Repeat("0", 64),
		strings.Repeat("f", 64),
		strings.Repeat("A", 64),
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	for _, validHexKey := range validHexKeys {
		t.Run(validHexKey, func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			if err := config.Validate(); err != nil {
				t.Fatalf("expected valid configuration for hex key, got: %v", err)
			}
		})
	}
}

func TestCoreConfigValidationKVStoreEdgeCasesUnit(t *testing.T) {
	validHexKey := strings.Repeat("e", 64)

	// Invalid backend
	for _, invalidBackend := range []string{"memcached", "memory", "dynamodb", "bad_backend", "123"} {
		t.Run(invalidBackend, func(t *testing.T) {
			config := DefaultConfig()
			config.Data.Enabled = true
			config.Security.MasterEncryptionKey = validHexKey
			config.KVStore.Backend = invalidBackend
			if err := config.Validate(); err == nil {
				t.Fatalf("expected validation error for invalid kv backend '%s'", invalidBackend)
			}
		})
	}

	// Redis backend with missing URL and missing ClusterURLs
	config := DefaultConfig()
	config.Data.Enabled = true
	config.Security.MasterEncryptionKey = validHexKey
	config.KVStore.Backend = "redis"
	config.KVStore.URL = ""
	config.KVStore.ClusterURLs = nil
	if err := config.Validate(); err == nil {
		t.Fatal("expected error on redis backend with empty URL and empty ClusterURLs")
	}

	// Redis backend with empty ClusterURLs slice
	config.KVStore.ClusterURLs = []string{}
	if err := config.Validate(); err == nil {
		t.Fatal("expected error on redis backend with empty ClusterURLs slice")
	}

	// Valid Redis backend with URL
	config.KVStore.URL = "redis://localhost:6379"
	config.KVStore.ClusterURLs = nil
	if err := config.Validate(); err != nil {
		t.Fatalf("expected valid redis config with URL, got: %v", err)
	}

	// Valid Redis backend with ClusterURLs
	config.KVStore.URL = ""
	config.KVStore.ClusterURLs = []string{"redis://node1:6379", "redis://node2:6379"}
	if err := config.Validate(); err != nil {
		t.Fatalf("expected valid redis config with ClusterURLs, got: %v", err)
	}

	// Valid Database backend (URL not required)
	config.KVStore.Backend = "database"
	config.KVStore.URL = ""
	config.KVStore.ClusterURLs = nil
	if err := config.Validate(); err != nil {
		t.Fatalf("expected valid database backend config, got: %v", err)
	}

	// Valid empty backend (defaults to database)
	config.KVStore.Backend = ""
	if err := config.Validate(); err != nil {
		t.Fatalf("expected valid empty backend config, got: %v", err)
	}
}

func TestCoreConfigValidationMinimumFunctionalServiceInvariantUnit(t *testing.T) {
	validHexKey := strings.Repeat("f", 64)

	// Zero functional services enabled (even if Console is enabled)
	zeroServiceConfig := DefaultConfig()
	zeroServiceConfig.Data.Enabled = false
	zeroServiceConfig.Auth.Enabled = false
	zeroServiceConfig.FileStorage.Enabled = false
	zeroServiceConfig.Security.MasterEncryptionKey = validHexKey
	zeroServiceConfig.Console.Enabled = true
	if err := zeroServiceConfig.Validate(); err == nil {
		t.Fatal("expected Minimum Functional Service Invariant error when zero functional services enabled")
	}

	// Each functional service individually satisfies the invariant
	allFunctionalServices := []string{"data", "auth", "tasks", "file_storage", "image", "function"}
	for _, serviceName := range allFunctionalServices {
		t.Run(serviceName, func(t *testing.T) {
			validServiceConfig := DefaultConfig()
			validServiceConfig.Security.MasterEncryptionKey = validHexKey
			_ = validServiceConfig.EnableService(serviceName)
			if err := validServiceConfig.Validate(); err != nil {
				t.Fatalf("expected service '%s' to satisfy minimum functional service invariant, got: %v", serviceName, err)
			}
		})
	}
}

func TestCoreConfigParseBooleanExhaustiveEdgeCasesUnit(t *testing.T) {
	truthyCases := []string{
		"true", "TRUE", "True", "tRuE", "1", "yes", "YES", "Yes", "yEs", "on", "ON", "On",
		"  true  ", "\t1\t", " yes\n", " ON ",
	}
	for _, truthyCase := range truthyCases {
		t.Run(fmt.Sprintf("%q", truthyCase), func(t *testing.T) {
			if !parseFlag(truthyCase) {
				t.Fatalf("expected truthy result for input '%s'", truthyCase)
			}
		})
	}

	falsyCases := []string{
		"false", "FALSE", "False", "0", "no", "NO", "No", "off", "OFF", "Off",
		"", "   ", "\t\n", "none", "null", "undefined", "invalid", "-1", "2", "truee", "yess", "00",
	}
	for _, falsyCase := range falsyCases {
		t.Run(fmt.Sprintf("%q", falsyCase), func(t *testing.T) {
			if parseFlag(falsyCase) {
				t.Fatalf("expected falsy result for input '%s'", falsyCase)
			}
		})
	}
}

func TestCoreConfigApplyEnvConfigOverridesExhaustiveFieldsUnit(t *testing.T) {
	validHexKey := strings.Repeat("a", 64)

	environmentMap := map[string]string{
		// Project
		"LAYR__PROJECT__NAME":        "Overridden Name",
		"LAYR__PROJECT__DESCRIPTION": "Overridden Description",
		// Server
		"LAYR__SERVER__LISTEN_ADDR": ":9999",
		"LAYR__SERVER__BASE_URL":    "http://192.168.1.1:9999",
		// Database
		"LAYR__DATABASE__URL":                    "postgres://user:secret@db.internal:5432/proddb",
		"LAYR__DATABASE__MAX_CONNECTIONS":        "75",
		"LAYR__DATABASE__MIN_CONNECTIONS":        "8",
		"LAYR__DATABASE__CONNECTION_TIMEOUT_MS":  "12000",
		"LAYR__DATABASE__IDLE_TIMEOUT_MS":        "450000",
		"LAYR__DATABASE__MAX_LIFETIME_MS":        "2400000",
		"LAYR__DATABASE__HEALTH_CHECK_PERIOD_MS": "20000",
		"LAYR__DATABASE__SSL_MODE":               "VERIFY-FULL",
		"LAYR__DATABASE__SSL_ROOT_CERT":          "/certs/root.pem",
		"LAYR__DATABASE__SSL_CERT":               "/certs/client.pem",
		"LAYR__DATABASE__SSL_KEY":                "/certs/client-key.pem",
		// Security
		"LAYR__SECURITY__MASTER_ENCRYPTION_KEY": validHexKey,
		// KV Store
		"LAYR__KV_STORE__BACKEND":      "REDIS",
		"LAYR__KV_STORE__URL":          "redis://custom:6379",
		"LAYR__KV_STORE__CLUSTER_URLS": "  redis://cluster-1:6379 , , redis://cluster-2:6379  , ",
		// Services
		"LAYR__DATA__ENABLED":                  "true",
		"LAYR__AUTH__ENABLED":                  "1",
		"LAYR__TASKS__ENABLED":                 "yes",
		"LAYR__FILE_STORAGE__ENABLED":          "on",
		"LAYR__IMAGE__ENABLED":                 "YES",
		"LAYR__FUNCTION__ENABLED":              "true",
		"LAYR__CONSOLE__ENABLED":               "true",
		"LAYR__CONSOLE__INITIAL_USER_EMAIL":    "console_user@layr.sh",
		"LAYR__CONSOLE__INITIAL_USER_PASSWORD": "supersecretconsolepassword",
	}

	for environmentKey, environmentValue := range environmentMap {
		t.Setenv(environmentKey, environmentValue)
	}

	config := DefaultConfig()
	ApplyEnvConfigOverrides(config)

	// Validate Project fields
	if config.Project.Name != "Overridden Name" ||
		config.Project.Description != "Overridden Description" {
		t.Fatalf("project overrides failed: %+v", config.Project)
	}

	// Validate Server fields
	if config.Server.ListenAddr != ":9999" || config.Server.BaseURL != "http://192.168.1.1:9999" {
		t.Fatalf("server overrides failed: %+v", config.Server)
	}

	// Validate Database fields
	if config.Database.URL != "postgres://user:secret@db.internal:5432/proddb" ||
		config.Database.MaxConnections != 75 ||
		config.Database.MinConnections != 8 ||
		config.Database.ConnectionTimeoutMs != 12000 ||
		config.Database.IdleTimeoutMs != 450000 ||
		config.Database.MaxLifetimeMs != 2400000 ||
		config.Database.HealthCheckPeriodMs != 20000 ||
		config.Database.SSLMode != "verify-full" ||
		config.Database.SSLRootCert != "/certs/root.pem" ||
		config.Database.SSLCert != "/certs/client.pem" ||
		config.Database.SSLKey != "/certs/client-key.pem" {
		t.Fatalf("database overrides failed: %+v", config.Database)
	}

	// Validate Security fields
	if config.Security.MasterEncryptionKey != validHexKey {
		t.Fatalf("security override failed: %s", config.Security.MasterEncryptionKey)
	}

	// Validate KV Store fields
	if config.KVStore.Backend != "redis" ||
		config.KVStore.URL != "redis://custom:6379" ||
		len(config.KVStore.ClusterURLs) != 2 ||
		config.KVStore.ClusterURLs[0] != "redis://cluster-1:6379" ||
		config.KVStore.ClusterURLs[1] != "redis://cluster-2:6379" {
		t.Fatalf("kv store overrides failed: %+v", config.KVStore)
	}

	// Validate Services enablement
	if !config.Data.Enabled || !config.Auth.Enabled || !config.Tasks.Enabled ||
		!config.FileStorage.Enabled || !config.Image.Enabled || !config.Function.Enabled {
		t.Fatal("service boolean overrides failed")
	}

	// Validate Console fields
	if !config.Console.Enabled ||
		config.Console.InitialUserEmail != "console_user@layr.sh" ||
		config.Console.InitialUserPassword != "supersecretconsolepassword" {
		t.Fatalf("console overrides failed: %+v", config.Console)
	}
}

func TestCoreConfigApplyEnvConfigOverridesAliasesAndMalformedInputsUnit(t *testing.T) {
	validHexKey := strings.Repeat("9", 64)

	// Test Top-level aliases: DATABASE_URL, MASTER_ENCRYPTION_KEY, PORT, BASE_URL
	t.Setenv("DATABASE_URL", "postgres://alias@localhost:5432/aliasdb")
	t.Setenv("MASTER_ENCRYPTION_KEY", validHexKey)
	t.Setenv("PORT", "7777")
	t.Setenv("BASE_URL", "https://layr.example.com")

	config := DefaultConfig()
	ApplyEnvConfigOverrides(config)

	if config.Database.URL != "postgres://alias@localhost:5432/aliasdb" {
		t.Fatalf("expected DATABASE_URL alias override, got '%s'", config.Database.URL)
	}
	if config.Security.MasterEncryptionKey != validHexKey {
		t.Fatalf("expected MASTER_ENCRYPTION_KEY alias override, got '%s'", config.Security.MasterEncryptionKey)
	}
	if config.Server.ListenAddr != ":7777" {
		t.Fatalf("expected PORT alias override :7777, got %s", config.Server.ListenAddr)
	}
	if config.Server.BaseURL != "https://layr.example.com" {
		t.Fatalf("expected BASE_URL alias override, got %s", config.Server.BaseURL)
	}

	// Test PORT alias when ListenAddr does not contain a colon
	noColonListenAddrConfig := DefaultConfig()
	noColonListenAddrConfig.Server.ListenAddr = "rawaddr"
	ApplyEnvConfigOverrides(noColonListenAddrConfig)
	if noColonListenAddrConfig.Server.ListenAddr != ":7777" {
		t.Fatalf("expected PORT alias override without colon, got %s", noColonListenAddrConfig.Server.ListenAddr)
	}

	// Test Invalid PORT alias (should not crash or overwrite valid port)
	t.Setenv("PORT", "invalid_non_numeric_port")
	ApplyEnvConfigOverrides(config)
	if config.Server.ListenAddr != ":7777" {
		t.Fatalf("expected port to remain :7777 after invalid PORT string, got %s", config.Server.ListenAddr)
	}

	// Test applyEnvConfigField directly with server fields
	applyEnvConfigField(config, "server", "listen_addr", ":8000")
	applyEnvConfigField(config, "server", "base_url", "http://localhost:8000")
	if config.Server.ListenAddr != ":8000" || config.Server.BaseURL != "http://localhost:8000" {
		t.Fatalf("expected server listen_addr and base_url overrides, got %+v", config.Server)
	}
	applyEnvConfigField(config, "database", "max_connections", "bad_int")
	applyEnvConfigField(config, "database", "min_connections", "bad_int")
	applyEnvConfigField(config, "database", "connection_timeout_ms", "bad_int")
	applyEnvConfigField(config, "database", "idle_timeout_ms", "bad_int")
	applyEnvConfigField(config, "database", "max_lifetime_ms", "bad_int")
	applyEnvConfigField(config, "database", "health_check_period_ms", "bad_int")
	applyEnvConfigField(config, "kv_store", "cluster_urls", "") // empty string should do nothing

	// Test applyEnvConfigField with KV aliases
	applyEnvConfigField(config, "kvstore", "backend", "DATABASE")
	if config.KVStore.Backend != "database" {
		t.Fatalf("expected kvstore backend database, got %s", config.KVStore.Backend)
	}
	applyEnvConfigField(config, "kv", "backend", "REDIS")
	if config.KVStore.Backend != "redis" {
		t.Fatalf("expected kv backend redis, got %s", config.KVStore.Backend)
	}

	// Test unknown sections and fields
	applyEnvConfigField(config, "unknown_section", "field", "value")
	applyEnvConfigField(config, "project", "unknown_field", "value")
	applyEnvConfigField(config, "security", "unknown_field", "value")
	applyEnvConfigField(config, "console", "unknown_field", "value")
}

func TestCoreConfigGetAndLifecycleUnit(t *testing.T) {
	UnloadConfig()
	defer UnloadConfig()

	// Default fallback when not loaded
	defaultConfig := GetConfig()
	if defaultConfig == nil || defaultConfig.Project.Name != "layr-app" {
		t.Fatalf("expected Default config when not loaded, got: %v", defaultConfig)
	}

	// ServerBaseURL verification on default
	if defaultConfig.ServerBaseURL() != "http://localhost:8080" {
		t.Fatalf("expected http://localhost:8080, got: %s", defaultConfig.ServerBaseURL())
	}

	// SetLoadedConfig custom config
	customConfig := DefaultConfig()
	customConfig.Project.Name = "custom-project"
	customConfig.Server.ListenAddr = "127.0.0.1:9090"
	customConfig.Server.BaseURL = "http://127.0.0.1:9090"
	SetLoadedConfig(customConfig)

	activeConfig := GetConfig()
	if activeConfig.Project.Name != "custom-project" {
		t.Fatalf("expected custom-project, got: %s", activeConfig.Project.Name)
	}
	if activeConfig.ServerBaseURL() != "http://127.0.0.1:9090" {
		t.Fatalf("expected http://127.0.0.1:9090, got: %s", activeConfig.ServerBaseURL())
	}

	// Nil receiver or empty base url ServerBaseURL
	var nilConfig *Config
	if nilConfig.ServerBaseURL() != "http://localhost:8080" {
		t.Fatalf("expected fallback for nil config, got: %s", nilConfig.ServerBaseURL())
	}

	emptyBaseURLConfig := &Config{}
	if emptyBaseURLConfig.ServerBaseURL() != "http://localhost:8080" {
		t.Fatalf("expected fallback for empty base url, got: %s", emptyBaseURLConfig.ServerBaseURL())
	}

	// UnloadConfig clears back to nil
	UnloadConfig()
	afterUnloadConfigConfig := GetConfig()
	if afterUnloadConfigConfig.Project.Name != "layr-app" {
		t.Fatalf("expected Default after UnloadConfig, got: %s", afterUnloadConfigConfig.Project.Name)
	}
}

func TestCoreConfigWriteConfigFileDefaultUnit(t *testing.T) {
	t.Cleanup(UnloadConfig)
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "sub", "config.yaml")

	t.Setenv("PORT", "9123")
	t.Setenv("LAYR__PROJECT__NAME", "Env Written App")

	if writeErr := WriteConfigFile(targetPath); writeErr != nil {
		t.Fatalf("unexpected write config file error: %v", writeErr)
	}

	loadedConfig, loadErr := LoadConfig(targetPath)
	if loadErr != nil {
		t.Fatalf("unexpected load config file error: %v", loadErr)
	}

	if loadedConfig.Server.ListenAddr != ":9123" {
		t.Errorf("expected server listen addr ':9123', got %q", loadedConfig.Server.ListenAddr)
	}
	if loadedConfig.Project.Name != "Env Written App" {
		t.Errorf("expected project name 'Env Written App', got %q", loadedConfig.Project.Name)
	}
	const expectedKeyLength = 64
	if len(loadedConfig.Security.MasterEncryptionKey) != expectedKeyLength {
		t.Errorf("expected 64-char hex master encryption key, got %q", loadedConfig.Security.MasterEncryptionKey)
	}
}

func TestCoreConfigWriteConfigFileCustomUnit(t *testing.T) {
	t.Cleanup(UnloadConfig)
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "custom.yaml")

	customConfig := *DefaultConfig()
	customConfig.Project.Name = "Explicit Custom App"
	customConfig.Server.ListenAddr = ":443"
	customConfig.Security.MasterEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	if writeErr := WriteConfigFile(targetPath, customConfig); writeErr != nil {
		t.Fatalf("unexpected write error: %v", writeErr)
	}

	loadedConfig, loadErr := LoadConfig(targetPath)
	if loadErr != nil {
		t.Fatalf("unexpected load error: %v", loadErr)
	}

	if loadedConfig.Project.Name != "Explicit Custom App" {
		t.Errorf("expected 'Explicit Custom App', got %q", loadedConfig.Project.Name)
	}
	if loadedConfig.Server.ListenAddr != ":443" {
		t.Errorf("expected ':443', got %q", loadedConfig.Server.ListenAddr)
	}
	if loadedConfig.Security.MasterEncryptionKey != "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Errorf("expected preserved master encryption key, got %q", loadedConfig.Security.MasterEncryptionKey)
	}

	emptyKeyPath := filepath.Join(tempDir, "custom_empty_key.yaml")
	customEmptyMasterEncryptionKeyConfig := *DefaultConfig()
	customEmptyMasterEncryptionKeyConfig.Security.MasterEncryptionKey = ""
	if writeEmptyErr := WriteConfigFile(emptyKeyPath, customEmptyMasterEncryptionKeyConfig); writeEmptyErr != nil {
		t.Fatalf("unexpected write error: %v", writeEmptyErr)
	}
	loadedEmptyMasterEncryptionKeyConfig, loadEmptyErr := LoadConfig(emptyKeyPath)
	if loadEmptyErr != nil {
		t.Fatalf("unexpected load error: %v", loadEmptyErr)
	}
	const expectedKeyLength = 64
	if len(loadedEmptyMasterEncryptionKeyConfig.Security.MasterEncryptionKey) != expectedKeyLength {
		t.Errorf("expected generated 64-char hex key, got %q", loadedEmptyMasterEncryptionKeyConfig.Security.MasterEncryptionKey)
	}
}

func TestCoreConfigWriteConfigFileEnvSecurityKeyUnit(t *testing.T) {
	t.Cleanup(UnloadConfig)
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "env_security.yaml")

	expectedKey := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	t.Setenv("LAYR__SECURITY__MASTER_ENCRYPTION_KEY", expectedKey)

	if writeErr := WriteConfigFile(targetPath); writeErr != nil {
		t.Fatalf("unexpected write error: %v", writeErr)
	}

	loadedConfig, loadErr := LoadConfig(targetPath)
	if loadErr != nil {
		t.Fatalf("unexpected load error: %v", loadErr)
	}

	if loadedConfig.Security.MasterEncryptionKey != expectedKey {
		t.Errorf("expected master encryption key from env %q, got %q", expectedKey, loadedConfig.Security.MasterEncryptionKey)
	}
}

func TestCoreConfigCreateConfigFileAliasUnit(t *testing.T) {
	t.Cleanup(UnloadConfig)
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "alias.yaml")

	if createErr := CreateConfigFile(targetPath); createErr != nil {
		t.Fatalf("unexpected create error: %v", createErr)
	}

	loadedConfig, loadErr := LoadConfig(targetPath)
	if loadErr != nil {
		t.Fatalf("unexpected load error: %v", loadErr)
	}

	if loadedConfig.Project.Name != "layr-app" {
		t.Errorf("expected 'layr-app', got %q", loadedConfig.Project.Name)
	}
}

func TestCoreConfigWriteConfigFileErrorsUnit(t *testing.T) {
	// Empty path error
	if emptyPathErr := WriteConfigFile(""); emptyPathErr == nil {
		t.Fatal("expected error for empty path, got nil")
	}

	// Directory creation error
	invalidDirPath := filepath.Join("/dev/null", "cannot_mkdir", "config.yaml")
	if dirCreationErr := WriteConfigFile(invalidDirPath); dirCreationErr == nil {
		t.Fatal("expected error creating directory under /dev/null, got nil")
	}

	// Write file error (target path is an existing directory)
	tempDir := t.TempDir()
	if writeDirErr := WriteConfigFile(tempDir); writeDirErr == nil {
		t.Fatal("expected error writing to directory path, got nil")
	}

	// YAML marshal error
	originalMarshal := yamlMarshal
	defer func() { yamlMarshal = originalMarshal }()
	yamlMarshal = func(_ any) ([]byte, error) {
		return nil, errors.New("simulated marshal error")
	}
	marshalErrorPath := filepath.Join(tempDir, "marshal_fail.yaml")
	if marshalErr := WriteConfigFile(marshalErrorPath); marshalErr == nil {
		t.Fatal("expected marshal error, got nil")
	}
}

func TestCoreConfigVerboseLoggingUnit(t *testing.T) {
	t.Cleanup(UnloadConfig)
	var buffer bytes.Buffer
	log.SetOutput(&buffer)
	defer log.SetOutput(nil)

	tempDir := t.TempDir()
	targetConfigFile := filepath.Join(tempDir, "verbose_test.yaml")

	// Test programmatic SetVerboseLogging
	log.SetVerboseLogging(true)
	defer log.SetVerboseLogging(false)

	t.Setenv("PORT", "8888")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("MASTER_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	t.Setenv("BASE_URL", "https://api.example.com")
	t.Setenv("LAYR__PROJECT__NAME", "Verbose App")
	t.Setenv("LAYR__SERVER__LISTEN_ADDR", ":8888")

	// 1. WriteConfigFile must NEVER print "Overriding %s from ENV" messages
	if writeErr := WriteConfigFile(targetConfigFile); writeErr != nil {
		t.Fatalf("unexpected write error: %v", writeErr)
	}
	if buffer.Len() != 0 {
		t.Errorf("expected WriteConfigFile NOT to log any override messages, but got:\n%s", buffer.String())
	}
	buffer.Reset()

	// 2. LoadConfig MUST print "Overriding %s from ENV" messages when verbose logging is enabled
	if _, loadErr := LoadConfig(targetConfigFile); loadErr != nil {
		t.Fatalf("unexpected load error: %v", loadErr)
	}

	loggedContent := buffer.String()
	expectedSubstrings := []string{
		"Overriding server.port from ENV",
		"Overriding database.url from ENV",
		"Overriding security.master_encryption_key from ENV",
		"Overriding server.base_url from ENV",
		"Overriding project.name from ENV",
		"Overriding server.listen_addr from ENV",
	}
	for _, expectedSubstring := range expectedSubstrings {
		t.Run(expectedSubstring, func(t *testing.T) {
			if !strings.Contains(loggedContent, expectedSubstring) {
				t.Errorf("expected log to contain %q, got output:\n%s", expectedSubstring, loggedContent)
			}
		})
	}

	// 3. Test disabled logging: LoadConfig must not log
	log.SetVerboseLogging(false)
	buffer.Reset()
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LAYR_LOG_LEVEL", "")
	if _, loadErr := LoadConfig(targetConfigFile); loadErr != nil {
		t.Fatalf("unexpected load error: %v", loadErr)
	}
	if buffer.Len() != 0 {
		t.Errorf("expected no logs when verbose logging is disabled, got %q", buffer.String())
	}

	// 4. Test enabled via LOG_LEVEL=debug environment variable: LoadConfig must log
	t.Setenv("LOG_LEVEL", "debug")
	buffer.Reset()
	if _, loadErr := LoadConfig(targetConfigFile); loadErr != nil {
		t.Fatalf("unexpected load error: %v", loadErr)
	}
	if !strings.Contains(buffer.String(), "Overriding server.port from ENV") {
		t.Errorf("expected log output when LOG_LEVEL=debug, got %q", buffer.String())
	}
}
