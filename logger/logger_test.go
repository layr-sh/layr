package logger

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoggerLevelUnit(t *testing.T) {
	// 1. Verify String() representations
	if LevelDebug.String() != "DEBUG" {
		t.Errorf("expected DEBUG, got %q", LevelDebug.String())
	}
	if LevelInfo.String() != "INFO" {
		t.Errorf("expected INFO, got %q", LevelInfo.String())
	}
	if LevelWarn.String() != "WARN" {
		t.Errorf("expected WARN, got %q", LevelWarn.String())
	}
	if LevelError.String() != "ERROR" {
		t.Errorf("expected ERROR, got %q", LevelError.String())
	}

	// 2. Verify ParseLevel
	testCases := []struct {
		input         string
		expectedLevel Level
		expectError   bool
	}{
		{input: "TRACE", expectedLevel: LevelTrace, expectError: false},
		{input: "trace", expectedLevel: LevelTrace, expectError: false},
		{input: "  TrAcE  ", expectedLevel: LevelTrace, expectError: false},
		{input: "DEBUG", expectedLevel: LevelDebug, expectError: false},
		{input: "debug", expectedLevel: LevelDebug, expectError: false},
		{input: "  DeBuG  ", expectedLevel: LevelDebug, expectError: false},
		{input: "INFO", expectedLevel: LevelInfo, expectError: false},
		{input: "info", expectedLevel: LevelInfo, expectError: false},
		{input: "", expectedLevel: LevelInfo, expectError: false},
		{input: "   ", expectedLevel: LevelInfo, expectError: false},
		{input: "WARN", expectedLevel: LevelWarn, expectError: false},
		{input: "warn", expectedLevel: LevelWarn, expectError: false},
		{input: "WARNING", expectedLevel: LevelWarn, expectError: false},
		{input: "warning", expectedLevel: LevelWarn, expectError: false},
		{input: "ERROR", expectedLevel: LevelError, expectError: false},
		{input: "error", expectedLevel: LevelError, expectError: false},
		{input: "invalid_level", expectedLevel: LevelInfo, expectError: true},
	}

	for _, testCase := range testCases {
		parsedLevel, parseErr := ParseLevel(testCase.input)
		if testCase.expectError {
			if parseErr == nil {
				t.Errorf("ParseLevel(%q): expected error, got nil", testCase.input)
			}
		} else {
			if parseErr != nil {
				t.Errorf("ParseLevel(%q): unexpected error: %v", testCase.input, parseErr)
			}
			if parsedLevel != testCase.expectedLevel {
				t.Errorf("ParseLevel(%q): expected %v, got %v", testCase.input, testCase.expectedLevel, parsedLevel)
			}
		}
	}
}

func TestLoggerInstanceLoggingUnit(t *testing.T) {
	var buffer bytes.Buffer
	logger := New("inst-scope")
	logger.SetOutput(&buffer)
	logger.SetLevel(LevelTrace)

	if logger.GetLevel() != LevelTrace {
		t.Errorf("expected LevelTrace, got %v", logger.GetLevel())
	}
	if !logger.IsLevelEnabled(LevelTrace) {
		t.Error("expected LevelTrace to be enabled")
	}

	logger.Tracef("trace %s", "formatted")
	logger.Trace("trace direct")
	logger.Debugf("debug %s", "formatted")
	logger.Debug("debug direct")
	logger.Infof("info %s", "formatted")
	logger.Info("info direct")
	logger.Warnf("warn %s", "formatted")
	logger.Warn("warn direct")
	logger.Errorf("error %s", "formatted")
	logger.Error("error direct")
	logger.Logf(LevelInfo, "custom %s", "logf")
	logger.Log(LevelWarn, "custom log")

	loggedOutput := buffer.String()
	expectedSubstrings := []string{
		"[TRACE] [inst-scope] trace formatted",
		"[TRACE] [inst-scope] trace direct",
		"[DEBUG] [inst-scope] debug formatted",
		"[DEBUG] [inst-scope] debug direct",
		"[INFO] [inst-scope] info formatted",
		"[INFO] [inst-scope] info direct",
		"[WARN] [inst-scope] warn formatted",
		"[WARN] [inst-scope] warn direct",
		"[ERROR] [inst-scope] error formatted",
		"[ERROR] [inst-scope] error direct",
		"[INFO] [inst-scope] custom logf",
		"[WARN] [inst-scope] custom log",
	}

	for _, expectedSubstring := range expectedSubstrings {
		if !strings.Contains(loggedOutput, expectedSubstring) {
			t.Errorf("expected output to contain %q, but got:\n%s", expectedSubstring, loggedOutput)
		}
	}

	// Verify Slog() getter
	if logger.Slog() == nil {
		t.Error("expected non-nil slog.Logger from Slog()")
	}
}

func TestLoggerLevelFilteringUnit(t *testing.T) {
	var buffer bytes.Buffer
	logger := New()
	logger.SetOutput(&buffer)
	logger.SetLevel(LevelWarn)

	logger.Debugf("filtered debug")
	logger.Debug("filtered debug direct")
	logger.Infof("filtered info")
	logger.Info("filtered info direct")

	if buffer.Len() != 0 {
		t.Errorf("expected no output for lower log levels, got %q", buffer.String())
	}

	logger.Warnf("visible warning")
	logger.Errorf("visible error")

	output := buffer.String()
	if !strings.Contains(output, "[WARN] visible warning") {
		t.Errorf("expected output to contain '[WARN] visible warning', got %q", output)
	}
	if !strings.Contains(output, "[ERROR] visible error") {
		t.Errorf("expected output to contain '[ERROR] visible error', got %q", output)
	}

	// Dynamic level update
	logger.SetLevel(LevelDebug)
	if !logger.IsLevelEnabled(LevelDebug) {
		t.Error("expected LevelDebug to be enabled after SetLevel")
	}
	buffer.Reset()
	logger.Tracef("filtered trace")
	if buffer.Len() != 0 {
		t.Errorf("expected trace to be filtered when level is Debug, got %q", buffer.String())
	}
	logger.Debugf("now visible debug")
	if !strings.Contains(buffer.String(), "now visible debug") {
		t.Errorf("expected debug to be visible after setting level to DEBUG, got %q", buffer.String())
	}

	// Verbose and Debug modes
	logger.SetLevel(LevelWarn)
	logger.SetVerboseLogging(true)
	if !logger.IsVerboseLoggingEnabled() || logger.GetLevel() != LevelTrace {
		t.Errorf("expected verbose mode to lower level to Trace, got %v", logger.GetLevel())
	}
	logger.SetVerboseLogging(false)
	if logger.IsVerboseLoggingEnabled() {
		t.Error("expected verbose mode to be disabled")
	}

	logger.SetDebugMode(true)
	if !logger.IsDebugMode() || logger.GetLevel() != LevelDebug {
		t.Errorf("expected debug mode to lower level to Debug, got %v", logger.GetLevel())
	}
	logger.SetDebugMode(false)
	if logger.IsDebugMode() {
		t.Error("expected debug mode to be inactive")
	}
}

func TestLoggerScopeUnit(t *testing.T) {
	var buffer bytes.Buffer
	logger := New("custom-scope")
	logger.SetOutput(&buffer)

	if logger.Scope() != "custom-scope" {
		t.Errorf("expected scope 'custom-scope', got %q", logger.Scope())
	}

	logger.Infof("scoped message")
	if !strings.Contains(buffer.String(), "[INFO] [custom-scope] scoped message") {
		t.Errorf("expected output to contain '[INFO] [custom-scope] scoped message', got %q", buffer.String())
	}

	// Test SetScope and cleanScope
	logger.SetScope("[[bracket-scope]]")
	if logger.Scope() != "bracket-scope" {
		t.Errorf("expected cleaned scope 'bracket-scope', got %q", logger.Scope())
	}

	// Test WithScope inheritance
	childLogger := logger.WithScope("child-worker")
	if childLogger.Scope() != "child-worker" {
		t.Errorf("expected child scope 'child-worker', got %q", childLogger.Scope())
	}
	if childLogger.GetLevel() != logger.GetLevel() {
		t.Errorf("expected child logger to inherit log level %v, got %v", logger.GetLevel(), childLogger.GetLevel())
	}

	buffer.Reset()
	childLogger.Infof("child task execution")
	if !strings.Contains(buffer.String(), "[INFO] [child-worker] child task execution") {
		t.Errorf("expected child logger output to contain '[INFO] [child-worker] child task execution', got %q", buffer.String())
	}

	// Test ResetLevel
	logger.ResetLevel()
	if logger.hasExplicit {
		t.Error("expected hasExplicit to be false after reset")
	}

	// Test WithScope with explicit level on parent
	logger.SetLevel(LevelWarn)
	explicitScopeLogger := logger.WithScope("explicit-child")
	if explicitScopeLogger.GetLevel() != LevelWarn {
		t.Errorf("expected child to inherit explicit level LevelWarn, got %v", explicitScopeLogger.GetLevel())
	}
}

func TestLoggerSubsystemAndGlobalEnvironmentUnit(t *testing.T) {
	envKeys := []string{"LAYR_LOG_LEVEL", "LOG_LEVEL", "AUTH_LOG_LEVEL", "LAYR_AUTH_LOG_LEVEL"}
	savedEnv := make(map[string]string)
	for _, key := range envKeys {
		savedEnv[key] = os.Getenv(key)
	}
	defer func() {
		for key, value := range savedEnv {
			if value == "" {
				_ = os.Unsetenv(key)
			} else {
				_ = os.Setenv(key, value)
			}
		}
	}()

	clearAllEnv := func() {
		for _, key := range envKeys {
			_ = os.Unsetenv(key)
		}
	}

	// 1. Default threshold (no env set)
	clearAllEnv()
	logger := New()
	if logger.GetLevel() != LevelInfo {
		t.Errorf("expected default LevelInfo, got %v", logger.GetLevel())
	}

	// 2. Global LOG_LEVEL=debug
	clearAllEnv()
	_ = os.Setenv("LOG_LEVEL", "debug")
	globalDebugLogger := New()
	if globalDebugLogger.GetLevel() != LevelDebug {
		t.Errorf("expected LOG_LEVEL=debug to set LevelDebug, got %v", globalDebugLogger.GetLevel())
	}

	// 3. LAYR_LOG_LEVEL takes precedence over LOG_LEVEL
	clearAllEnv()
	_ = os.Setenv("LOG_LEVEL", "error")
	_ = os.Setenv("LAYR_LOG_LEVEL", "warn")
	precedenceLogger := New()
	if precedenceLogger.GetLevel() != LevelWarn {
		t.Errorf("expected LAYR_LOG_LEVEL=warn to take precedence, got %v", precedenceLogger.GetLevel())
	}

	// 4. Subsystem override: AUTH_LOG_LEVEL=debug while global LOG_LEVEL=warn
	clearAllEnv()
	_ = os.Setenv("LOG_LEVEL", "warn")
	_ = os.Setenv("AUTH_LOG_LEVEL", "debug")
	authLogger := New("auth")
	dbLogger := New("database")

	if authLogger.GetLevel() != LevelDebug {
		t.Errorf("expected authLogger to have LevelDebug via AUTH_LOG_LEVEL, got %v", authLogger.GetLevel())
	}
	if dbLogger.GetLevel() != LevelWarn {
		t.Errorf("expected dbLogger to have LevelWarn via global LOG_LEVEL, got %v", dbLogger.GetLevel())
	}

	// 5. LAYR_<SCOPE>_LOG_LEVEL precedence
	clearAllEnv()
	_ = os.Setenv("AUTH_LOG_LEVEL", "info")
	_ = os.Setenv("LAYR_AUTH_LOG_LEVEL", "error")
	precedenceScopeLogger := New("auth")
	if precedenceScopeLogger.GetLevel() != LevelError {
		t.Errorf("expected LAYR_AUTH_LOG_LEVEL=error to take precedence, got %v", precedenceScopeLogger.GetLevel())
	}

	// 6. Invalid level falls back to default
	clearAllEnv()
	_ = os.Setenv("LOG_LEVEL", "UNKNOWN_LEVEL")
	fallbackLogger := New()
	if fallbackLogger.GetLevel() != LevelInfo {
		t.Errorf("expected invalid level to fall back to LevelInfo, got %v", fallbackLogger.GetLevel())
	}
}

func TestLoggerSlogHandlerDirectUnit(t *testing.T) {
	var buffer bytes.Buffer
	levelVar := &slog.LevelVar{}
	levelVar.Set(LevelInfo)

	handler := &slogHandler{
		levelVar: levelVar,
		writer:   &buffer,
		scope:    "direct-handler",
	}

	ctx := context.Background()

	// 1. Enabled
	if handler.Enabled(ctx, LevelDebug) {
		t.Error("expected Debug to not be enabled")
	}
	if !handler.Enabled(ctx, LevelInfo) {
		t.Error("expected Info to be enabled")
	}

	// 2. Handle with zero time
	record := slog.Record{
		Time:    time.Time{},
		Level:   LevelInfo,
		Message: "zero time record",
	}
	if err := handler.Handle(ctx, record); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}
	if !strings.Contains(buffer.String(), "[INFO] [direct-handler] zero time record") {
		t.Errorf("expected output to contain message, got: %s", buffer.String())
	}

	// 3. Handle with nil writer defaults to os.Stderr
	nilWriterHandler := &slogHandler{
		levelVar: levelVar,
		writer:   nil,
		scope:    "",
	}
	if err := nilWriterHandler.Handle(ctx, record); err != nil {
		t.Fatalf("unexpected handle error with nil writer: %v", err)
	}

	// 4. WithAttrs and WithGroup return self
	if handler.WithAttrs(nil) != handler {
		t.Error("expected WithAttrs to return handler")
	}
	if handler.WithGroup("group") != handler {
		t.Error("expected WithGroup to return handler")
	}

	// 5. Handle with failing writer returns error
	failingHandler := &slogHandler{
		levelVar: levelVar,
		writer:   testErrorWriter{},
		scope:    "failing",
	}
	if err := failingHandler.Handle(ctx, record); err == nil {
		t.Error("expected error from failing writer, got nil")
	}
}

type testErrorWriter struct{}

func (testErrorWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failure")
}
