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
	t.Run("verify string representations", func(t *testing.T) {
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
	})

	t.Run("verify parse level", func(t *testing.T) {
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
			t.Run(testCase.input, func(t *testing.T) {
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
			})
		}
	})
}

func TestLoggerInstanceLoggingUnit(t *testing.T) {
	var buffer bytes.Buffer
	logger := New("inst-scope")
	logger.SetOutput(&buffer)
	logger.SetLevel(LevelTrace)

	t.Run("level getters and predicates", func(t *testing.T) {
		if logger.GetLevel() != LevelTrace {
			t.Errorf("expected LevelTrace, got %v", logger.GetLevel())
		}
		if !logger.IsLevelEnabled(LevelTrace) {
			t.Error("expected LevelTrace to be enabled")
		}
	})

	t.Run("level methods and formatting", func(t *testing.T) {
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
			t.Run(expectedSubstring, func(t *testing.T) {
				if !strings.Contains(loggedOutput, expectedSubstring) {
					t.Errorf("expected output to contain %q, but got:\n%s", expectedSubstring, loggedOutput)
				}
			})
		}
	})

	t.Run("slog getter", func(t *testing.T) {
		if logger.Slog() == nil {
			t.Error("expected non-nil slog.Logger from Slog()")
		}
	})
}

func TestLoggerLevelFilteringUnit(t *testing.T) {
	t.Run("filtering lower levels", func(t *testing.T) {
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
	})

	t.Run("visible higher levels", func(t *testing.T) {
		var buffer bytes.Buffer
		logger := New()
		logger.SetOutput(&buffer)
		logger.SetLevel(LevelWarn)

		logger.Warnf("visible warning")
		logger.Errorf("visible error")

		output := buffer.String()
		if !strings.Contains(output, "[WARN] visible warning") {
			t.Errorf("expected output to contain '[WARN] visible warning', got %q", output)
		}
		if !strings.Contains(output, "[ERROR] visible error") {
			t.Errorf("expected output to contain '[ERROR] visible error', got %q", output)
		}
	})

	t.Run("dynamic level update", func(t *testing.T) {
		var buffer bytes.Buffer
		logger := New()
		logger.SetOutput(&buffer)
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
	})

	t.Run("verbose mode toggling", func(t *testing.T) {
		logger := New()
		logger.SetLevel(LevelWarn)
		logger.SetVerboseLogging(true)
		if !logger.IsVerboseLoggingEnabled() || logger.GetLevel() != LevelTrace {
			t.Errorf("expected verbose mode to lower level to Trace, got %v", logger.GetLevel())
		}
		logger.SetVerboseLogging(false)
		if logger.IsVerboseLoggingEnabled() {
			t.Error("expected verbose mode to be disabled")
		}
	})

	t.Run("debug mode toggling", func(t *testing.T) {
		logger := New()
		logger.SetDebugMode(true)
		if !logger.IsDebugMode() || logger.GetLevel() != LevelDebug {
			t.Errorf("expected debug mode to lower level to Debug, got %v", logger.GetLevel())
		}
		logger.SetDebugMode(false)
		if logger.IsDebugMode() {
			t.Error("expected debug mode to be inactive")
		}
	})
}

func TestLoggerScopeUnit(t *testing.T) {
	t.Run("scope formatting", func(t *testing.T) {
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
	})

	t.Run("clean scope with brackets", func(t *testing.T) {
		logger := New()
		logger.SetScope("[[bracket-scope]]")
		if logger.Scope() != "bracket-scope" {
			t.Errorf("expected cleaned scope 'bracket-scope', got %q", logger.Scope())
		}
	})

	t.Run("with scope inheritance", func(t *testing.T) {
		var buffer bytes.Buffer
		logger := New()
		logger.SetOutput(&buffer)
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
	})

	t.Run("reset level", func(t *testing.T) {
		logger := New()
		logger.ResetLevel()
		if logger.hasExplicit {
			t.Error("expected hasExplicit to be false after reset")
		}
	})

	t.Run("with scope explicit level inheritance", func(t *testing.T) {
		logger := New()
		logger.SetLevel(LevelWarn)
		explicitScopeLogger := logger.WithScope("explicit-child")
		if explicitScopeLogger.GetLevel() != LevelWarn {
			t.Errorf("expected child to inherit explicit level LevelWarn, got %v", explicitScopeLogger.GetLevel())
		}
	})
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

	t.Run("default threshold", func(t *testing.T) {
		clearAllEnv()
		logger := New()
		if logger.GetLevel() != LevelInfo {
			t.Errorf("expected default LevelInfo, got %v", logger.GetLevel())
		}
	})

	t.Run("global LOG_LEVEL", func(t *testing.T) {
		clearAllEnv()
		_ = os.Setenv("LOG_LEVEL", "debug")
		globalDebugLogger := New()
		if globalDebugLogger.GetLevel() != LevelDebug {
			t.Errorf("expected LOG_LEVEL=debug to set LevelDebug, got %v", globalDebugLogger.GetLevel())
		}
	})

	t.Run("LAYR_LOG_LEVEL takes precedence over LOG_LEVEL", func(t *testing.T) {
		clearAllEnv()
		_ = os.Setenv("LOG_LEVEL", "error")
		_ = os.Setenv("LAYR_LOG_LEVEL", "warn")
		precedenceLogger := New()
		if precedenceLogger.GetLevel() != LevelWarn {
			t.Errorf("expected LAYR_LOG_LEVEL=warn to take precedence, got %v", precedenceLogger.GetLevel())
		}
	})

	t.Run("subsystem override", func(t *testing.T) {
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
	})

	t.Run("LAYR_SCOPE_LOG_LEVEL precedence", func(t *testing.T) {
		clearAllEnv()
		_ = os.Setenv("AUTH_LOG_LEVEL", "info")
		_ = os.Setenv("LAYR_AUTH_LOG_LEVEL", "error")
		precedenceScopeLogger := New("auth")
		if precedenceScopeLogger.GetLevel() != LevelError {
			t.Errorf("expected LAYR_AUTH_LOG_LEVEL=error to take precedence, got %v", precedenceScopeLogger.GetLevel())
		}
	})

	t.Run("invalid level falls back to default", func(t *testing.T) {
		clearAllEnv()
		_ = os.Setenv("LOG_LEVEL", "UNKNOWN_LEVEL")
		fallbackLogger := New()
		if fallbackLogger.GetLevel() != LevelInfo {
			t.Errorf("expected invalid level to fall back to LevelInfo, got %v", fallbackLogger.GetLevel())
		}
	})
}

func TestLoggerSlogHandlerDirectUnit(t *testing.T) {
	levelVar := &slog.LevelVar{}
	levelVar.Set(LevelInfo)

	ctx := context.Background()

	t.Run("enabled checks", func(t *testing.T) {
		var buffer bytes.Buffer
		handler := &slogHandler{
			levelVar: levelVar,
			writer:   &buffer,
			scope:    "direct-handler",
		}
		if handler.Enabled(ctx, LevelDebug) {
			t.Error("expected Debug to not be enabled")
		}
		if !handler.Enabled(ctx, LevelInfo) {
			t.Error("expected Info to be enabled")
		}
	})

	t.Run("handle with zero time", func(t *testing.T) {
		var buffer bytes.Buffer
		handler := &slogHandler{
			levelVar: levelVar,
			writer:   &buffer,
			scope:    "direct-handler",
		}
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
	})

	t.Run("handle with nil writer defaults to stderr", func(t *testing.T) {
		record := slog.Record{
			Time:    time.Time{},
			Level:   LevelInfo,
			Message: "zero time record",
		}
		nilWriterHandler := &slogHandler{
			levelVar: levelVar,
			writer:   nil,
			scope:    "",
		}
		if err := nilWriterHandler.Handle(ctx, record); err != nil {
			t.Fatalf("unexpected handle error with nil writer: %v", err)
		}
	})

	t.Run("with attrs and with group empty inputs", func(t *testing.T) {
		var buffer bytes.Buffer
		handler := &slogHandler{
			levelVar: levelVar,
			writer:   &buffer,
			scope:    "direct-handler",
		}
		if handler.WithAttrs(nil) != handler {
			t.Error("expected WithAttrs(nil) to return handler")
		}
		if handler.WithGroup("") != handler {
			t.Error("expected WithGroup(\"\") to return handler")
		}
	})

	t.Run("with attrs and with group formatting", func(t *testing.T) {
		var buffer bytes.Buffer
		handler := &slogHandler{
			levelVar: levelVar,
			writer:   &buffer,
			scope:    "direct-handler",
		}
		groupedHandler := handler.WithGroup("testgroup")
		if groupedHandler == handler {
			t.Error("expected WithGroup to return new handler instance")
		}
		attrHandler := groupedHandler.WithAttrs([]slog.Attr{slog.String("sub_key", "sub_val")})
		if attrHandler == groupedHandler {
			t.Error("expected WithAttrs to return new handler instance")
		}

		buffer.Reset()
		attrRecord := slog.Record{
			Time:    time.Now(),
			Level:   LevelInfo,
			Message: "attr message",
		}
		attrRecord.AddAttrs(slog.String("extra", "extra val with space"))
		if err := attrHandler.Handle(ctx, attrRecord); err != nil {
			t.Fatalf("unexpected handle error with attrs: %v", err)
		}
		if !strings.Contains(buffer.String(), `testgroup.sub_key=sub_val`) || !strings.Contains(buffer.String(), `testgroup.extra="extra val with space"`) {
			t.Errorf("expected output to contain grouped and quoted attrs, got: %s", buffer.String())
		}

		buffer.Reset()
		ungroupedAttrHandler := handler.WithAttrs([]slog.Attr{slog.String("direct_key", "direct_val")})
		if err := ungroupedAttrHandler.Handle(ctx, attrRecord); err != nil {
			t.Fatalf("unexpected handle error with ungrouped attrs: %v", err)
		}
		if !strings.Contains(buffer.String(), "direct_key=direct_val") || !strings.Contains(buffer.String(), `extra="extra val with space"`) {
			t.Errorf("expected ungrouped attrs in output, got: %s", buffer.String())
		}
	})

	t.Run("handle with failing writer returns error", func(t *testing.T) {
		failingHandler := &slogHandler{
			levelVar: levelVar,
			writer:   testErrorWriter{},
			scope:    "failing",
		}
		record := slog.Record{
			Time:    time.Time{},
			Level:   LevelInfo,
			Message: "zero time record",
		}
		if err := failingHandler.Handle(ctx, record); err == nil {
			t.Error("expected error from failing writer, got nil")
		}
	})
}

type testErrorWriter struct{}

func (testErrorWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failure")
}

func TestLoggerWithUnit(t *testing.T) {
	var buffer bytes.Buffer
	testLogger := New("with-scope")
	testLogger.SetOutput(&buffer)
	testLogger.SetLevel(LevelTrace)

	t.Run("with structured attributes", func(t *testing.T) {
		childWithLogger := testLogger.With("custom_attr", "custom_val", slog.Int("count", 42), slog.String("", "ignored"), 123, "odd_key_only")
		childWithLogger.SetOutput(&buffer)
		childWithLogger.Info("with child message")
		if !strings.Contains(buffer.String(), "custom_attr=custom_val") || !strings.Contains(buffer.String(), "count=42") || !strings.Contains(buffer.String(), "!BADKEY=odd_key_only") || !strings.Contains(buffer.String(), "!EXTRA=123") {
			t.Errorf("expected structured attrs in output, got: %s", buffer.String())
		}
	})

	t.Run("with explicit level preservation", func(t *testing.T) {
		explicitLogger := New("exp")
		explicitLogger.SetLevel(LevelWarn)
		explicitChildLogger := explicitLogger.With("key", "val")
		if explicitChildLogger.GetLevel() != LevelWarn {
			t.Errorf("expected child logger to retain explicit LevelWarn, got %v", explicitChildLogger.GetLevel())
		}
	})
}
