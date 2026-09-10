// Package logger provides configuration, logging, and foundational services for Layr.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Level represents the severity level of a log message using slog.Level.
type Level = slog.Level

const (
	// LevelTrace logs fine-grained troubleshooting and tracing messages.
	LevelTrace = slog.LevelDebug - 4 //nolint:namingclarity
	// LevelDebug logs detailed diagnostic and troubleshooting messages.
	LevelDebug = slog.LevelDebug //nolint:namingclarity
	// LevelInfo logs general operational and informational messages.
	LevelInfo = slog.LevelInfo //nolint:namingclarity
	// LevelWarn logs warnings about unexpected but recoverable situations.
	LevelWarn = slog.LevelWarn //nolint:namingclarity
	// LevelError logs critical errors and operational failures.
	LevelError = slog.LevelError //nolint:namingclarity
)

// ParseLevel parses a string into a Level.
// Supported values (case-insensitive): "TRACE", "DEBUG", "INFO", "WARN", "WARNING", "ERROR".
// An empty or whitespace-only input defaults to LevelInfo.
func ParseLevel(levelText string) (Level, error) {
	switch strings.ToUpper(strings.TrimSpace(levelText)) {
	case "TRACE":
		return LevelTrace, nil
	case "DEBUG":
		return LevelDebug, nil
	case "INFO", "":
		return LevelInfo, nil
	case "WARN", "WARNING":
		return LevelWarn, nil
	case "ERROR":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level: %q", levelText)
	}
}

func formatLevel(level Level) string {
	switch {
	case level <= LevelTrace:
		return "TRACE"
	case level < LevelInfo:
		return "DEBUG"
	case level < LevelWarn:
		return "INFO"
	case level < LevelError:
		return "WARN"
	default:
		return "ERROR"
	}
}

type slogHandler struct {
	levelVar *slog.LevelVar
	writer   io.Writer
	scope    string
	mutex    sync.Mutex
	attrs    []slog.Attr
	groups   []string
}

func formatAttrs(attrs []slog.Attr) string {
	if len(attrs) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, attr := range attrs {
		if attr.Key == "" {
			continue
		}
		builder.WriteString(" ")
		builder.WriteString(attr.Key)
		builder.WriteString("=")
		stringValue := attr.Value.String()
		if strings.ContainsAny(stringValue, " \t\n\"") {
			builder.WriteString(strconv.Quote(stringValue))
		} else {
			builder.WriteString(stringValue)
		}
	}
	return builder.String()
}

func argsToAttrs(args []any) []slog.Attr {
	var attrs []slog.Attr
	for index := 0; index < len(args); index++ {
		switch value := args[index].(type) {
		case slog.Attr:
			attrs = append(attrs, value)
		case string:
			if index+1 < len(args) {
				attrs = append(attrs, slog.Any(value, args[index+1]))
				index++
			} else {
				attrs = append(attrs, slog.String("!BADKEY", value))
			}
		default:
			attrs = append(attrs, slog.Any("!EXTRA", value))
		}
	}
	return attrs
}

func (handler *slogHandler) writeMessage(recordTime time.Time, level slog.Level, message string, extraAttrs ...[]slog.Attr) error {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()

	targetWriter := handler.writer
	if targetWriter == nil {
		targetWriter = os.Stderr //nolint:namingclarity
	}

	if recordTime.IsZero() {
		recordTime = time.Now()
	}
	formattedTime := recordTime.Format("2006/01/02 15:04:05")
	levelText := formatLevel(level)

	var combinedAttrs []slog.Attr
	if len(handler.attrs) > 0 {
		combinedAttrs = append(combinedAttrs, handler.attrs...)
	}
	for _, attrList := range extraAttrs {
		combinedAttrs = append(combinedAttrs, attrList...)
	}
	attrsText := formatAttrs(combinedAttrs)

	var logLine string
	if handler.scope != "" {
		logLine = fmt.Sprintf("%s [%s] [%s] %s%s\n", formattedTime, levelText, handler.scope, message, attrsText)
	} else {
		logLine = fmt.Sprintf("%s [%s] %s%s\n", formattedTime, levelText, message, attrsText)
	}

	if _, err := io.WriteString(targetWriter, logLine); err != nil {
		return fmt.Errorf("failed to write log line: %w", err)
	}
	return nil
}

func (handler *slogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= handler.levelVar.Level()
}

func (handler *slogHandler) Handle(ctx context.Context, record slog.Record) error {
	var recordAttrs []slog.Attr
	prefix := ""
	if len(handler.groups) > 0 {
		prefix = strings.Join(handler.groups, ".") + "."
	}
	record.Attrs(func(attr slog.Attr) bool {
		if prefix != "" {
			recordAttrs = append(recordAttrs, slog.Attr{Key: prefix + attr.Key, Value: attr.Value})
		} else {
			recordAttrs = append(recordAttrs, attr)
		}
		return true
	})

	return handler.writeMessage(record.Time, record.Level, record.Message, recordAttrs)
}

func (handler *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return handler
	}
	handler.mutex.Lock()
	defer handler.mutex.Unlock()

	newAttrs := make([]slog.Attr, len(handler.attrs), len(handler.attrs)+len(attrs))
	copy(newAttrs, handler.attrs)

	prefix := ""
	if len(handler.groups) > 0 {
		prefix = strings.Join(handler.groups, ".") + "."
	}
	for _, attr := range attrs {
		if prefix != "" {
			newAttrs = append(newAttrs, slog.Attr{Key: prefix + attr.Key, Value: attr.Value})
		} else {
			newAttrs = append(newAttrs, attr)
		}
	}

	return &slogHandler{
		levelVar: handler.levelVar,
		writer:   handler.writer,
		scope:    handler.scope,
		attrs:    newAttrs,
		groups:   append([]string(nil), handler.groups...),
	}
}

func (handler *slogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return handler
	}
	handler.mutex.Lock()
	defer handler.mutex.Unlock()

	return &slogHandler{
		levelVar: handler.levelVar,
		writer:   handler.writer,
		scope:    handler.scope,
		attrs:    append([]slog.Attr(nil), handler.attrs...),
		groups:   append(append([]string(nil), handler.groups...), name),
	}
}

// Logger provides leveled, formatted logging capabilities backed by log/slog.
type Logger struct {
	loggerRWMutex sync.RWMutex
	scope         string
	levelVar      *slog.LevelVar
	handler       *slogHandler
	slogLogger    *slog.Logger
	hasExplicit   bool
}

// New creates a new Logger instance with an optional scope.
// When scope is provided, messages are tagged with [scope], and the logger
// respects scope-specific log level overrides (e.g. AUTH_LOG_LEVEL).
func New(optionalScope ...string) *Logger {
	var scope string
	if len(optionalScope) > 0 {
		scope = cleanScope(optionalScope[0])
	}

	levelVar := &slog.LevelVar{}
	handler := &slogHandler{
		levelVar: levelVar,
		writer:   os.Stderr,
		scope:    scope,
	}

	logger := &Logger{
		scope:      scope,
		levelVar:   levelVar,
		handler:    handler,
		slogLogger: slog.New(handler),
	}

	levelVar.Set(logger.resolveEnvLevel())
	return logger
}

// SetLevel sets an explicit minimum severity level on the logger instance.
func (logger *Logger) SetLevel(level Level) {
	logger.loggerRWMutex.Lock()
	defer logger.loggerRWMutex.Unlock()
	logger.hasExplicit = true
	logger.levelVar.Set(level)
}

// ResetLevel clears explicit log levels, reverting to environment and default thresholds.
func (logger *Logger) ResetLevel() {
	logger.loggerRWMutex.Lock()
	defer logger.loggerRWMutex.Unlock()
	logger.hasExplicit = false
	logger.levelVar.Set(logger.resolveEnvLevel())
}

// GetLevel resolves the active minimum log level threshold.
func (logger *Logger) GetLevel() Level {
	logger.loggerRWMutex.RLock()
	defer logger.loggerRWMutex.RUnlock()

	if logger.hasExplicit {
		return logger.levelVar.Level()
	}
	envLevel := logger.resolveEnvLevel()
	logger.levelVar.Set(envLevel)
	return envLevel
}

// Scope returns the configured scope of the logger.
func (logger *Logger) Scope() string {
	logger.loggerRWMutex.RLock()
	defer logger.loggerRWMutex.RUnlock()
	return logger.scope
}

// SetScope updates the scope of the logger.
func (logger *Logger) SetScope(scope string) {
	logger.loggerRWMutex.Lock()
	defer logger.loggerRWMutex.Unlock()
	logger.scope = cleanScope(scope)
	logger.handler.mutex.Lock()
	logger.handler.scope = logger.scope
	logger.handler.mutex.Unlock()
	if !logger.hasExplicit {
		logger.levelVar.Set(logger.resolveEnvLevel())
	}
}

// WithScope creates a child Logger with the specified scope,
// inheriting output and level settings from this logger.
func (logger *Logger) WithScope(scope string) *Logger {
	logger.loggerRWMutex.RLock()
	defer logger.loggerRWMutex.RUnlock()

	childLogger := New(scope)

	logger.handler.mutex.Lock()
	childWriter := logger.handler.writer
	logger.handler.mutex.Unlock()

	childLogger.SetOutput(childWriter)
	if logger.hasExplicit {
		childLogger.SetLevel(logger.levelVar.Level())
	}

	return childLogger
}

// SetVerboseLogging sets verbose logging mode, lowering the threshold to LevelTrace.
func (logger *Logger) SetVerboseLogging(enabled bool) {
	if enabled {
		logger.SetLevel(LevelTrace)
	} else {
		logger.ResetLevel()
	}
}

// IsVerboseLoggingEnabled reports whether trace-level logging is active.
func (logger *Logger) IsVerboseLoggingEnabled() bool {
	return logger.IsLevelEnabled(LevelTrace)
}

// SetDebugMode toggles debug logging mode, lowering the threshold to LevelDebug.
func (logger *Logger) SetDebugMode(enabled bool) {
	if enabled {
		logger.SetLevel(LevelDebug)
	} else {
		logger.ResetLevel()
	}
}

// IsDebugMode reports whether debug logging is active.
func (logger *Logger) IsDebugMode() bool {
	return logger.IsLevelEnabled(LevelDebug)
}

// IsLevelEnabled reports whether messages at the given level will be logged.
func (logger *Logger) IsLevelEnabled(level Level) bool {
	return level >= logger.GetLevel()
}

// SetOutput sets the destination writer for log output.
func (logger *Logger) SetOutput(writer io.Writer) {
	logger.handler.mutex.Lock()
	defer logger.handler.mutex.Unlock()
	logger.handler.writer = writer
}

// Slog returns the underlying *slog.Logger instance for structured logging.
func (logger *Logger) Slog() *slog.Logger {
	logger.loggerRWMutex.RLock()
	defer logger.loggerRWMutex.RUnlock()
	return logger.slogLogger
}

// Logf formats and writes a log message at the specified level if enabled.
func (logger *Logger) Logf(level Level, format string, args ...any) {
	if !logger.IsLevelEnabled(level) {
		return
	}
	_ = logger.handler.writeMessage(time.Now(), level, fmt.Sprintf(format, args...))
}

// Log writes a log message with default formatting at the specified level if enabled.
func (logger *Logger) Log(level Level, args ...any) {
	if !logger.IsLevelEnabled(level) {
		return
	}
	_ = logger.handler.writeMessage(time.Now(), level, fmt.Sprint(args...))
}

// Tracef logs a formatted message at LevelTrace.
func (logger *Logger) Tracef(format string, args ...any) {
	logger.Logf(LevelTrace, format, args...)
}

// Trace logs a message at LevelTrace.
func (logger *Logger) Trace(args ...any) {
	logger.Log(LevelTrace, args...)
}

// Debugf logs a formatted message at LevelDebug.
func (logger *Logger) Debugf(format string, args ...any) {
	logger.Logf(LevelDebug, format, args...)
}

// Debug logs a message at LevelDebug.
func (logger *Logger) Debug(args ...any) {
	logger.Log(LevelDebug, args...)
}

// Infof logs a formatted message at LevelInfo.
func (logger *Logger) Infof(format string, args ...any) {
	logger.Logf(LevelInfo, format, args...)
}

// Info logs a message at LevelInfo.
func (logger *Logger) Info(args ...any) {
	logger.Log(LevelInfo, args...)
}

// Warnf logs a formatted message at LevelWarn.
func (logger *Logger) Warnf(format string, args ...any) {
	logger.Logf(LevelWarn, format, args...)
}

// Warn logs a message at LevelWarn.
func (logger *Logger) Warn(args ...any) {
	logger.Log(LevelWarn, args...)
}

// Errorf logs a formatted message at LevelError.
func (logger *Logger) Errorf(format string, args ...any) {
	logger.Logf(LevelError, format, args...)
}

// Error logs a message at LevelError.
func (logger *Logger) Error(args ...any) {
	logger.Log(LevelError, args...)
}

// With creates a child Logger with the specified structured attributes pre-attached.
func (logger *Logger) With(args ...any) *Logger {
	logger.loggerRWMutex.RLock()
	defer logger.loggerRWMutex.RUnlock()

	attrs := argsToAttrs(args)
	newHandler := logger.handler.WithAttrs(attrs).(*slogHandler)
	childLogger := &Logger{
		scope:      logger.scope,
		levelVar:   logger.levelVar,
		handler:    newHandler,
		slogLogger: slog.New(newHandler),
	}
	if logger.hasExplicit {
		childLogger.hasExplicit = true
	}
	return childLogger
}

func (logger *Logger) resolveEnvLevel() Level {
	// 1. Check subsystem/scope-specific log level overrides
	if logger.scope != "" {
		normalizedScope := strings.ToUpper(strings.ReplaceAll(logger.scope, "-", "_"))
		for _, envKey := range []string{
			"LAYR_" + normalizedScope + "_LOG_LEVEL",
			normalizedScope + "_LOG_LEVEL",
		} {
			if value := strings.TrimSpace(os.Getenv(envKey)); value != "" {
				if parsedLevel, err := ParseLevel(value); err == nil {
					return parsedLevel
				}
			}
		}
	}

	// 2. Check global log level threshold
	for _, envKey := range []string{"LAYR_LOG_LEVEL", "LOG_LEVEL"} {
		if value := strings.TrimSpace(os.Getenv(envKey)); value != "" {
			if parsedLevel, err := ParseLevel(value); err == nil {
				return parsedLevel
			}
		}
	}

	return LevelInfo
}

func cleanScope(scope string) string {
	return strings.Trim(strings.TrimSpace(scope), "[]")
}
