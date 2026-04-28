package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"outless/shared/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	envLogLevel  = "OUTLESS_LOG_LEVEL"
	envLogFormat = "OUTLESS_LOG_FORMAT"
)

// New creates a process logger with unified format across services.
// Deprecated: Use NewFromConfig for configuration-based logging.
func New(service string) *slog.Logger {
	level := parseLevel(os.Getenv(envLogLevel))
	opts := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceBuiltInAttrs,
	}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(os.Getenv(envLogFormat))) {
	case "text", "console":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	name := strings.TrimSpace(service)
	if name == "" {
		name = "unknown-service"
	}

	return slog.New(handler).With(
		slog.String("service", name),
		slog.Int("pid", os.Getpid()),
	)
}

// NewFromConfig creates a process logger with configuration-based settings.
func NewFromConfig(service string, cfg config.LogsConfig, module string) *slog.Logger {
	level := parseLevel(cfg.Level)
	logType := strings.ToLower(strings.TrimSpace(cfg.Type))

	name := strings.TrimSpace(service)
	if name == "" {
		name = "unknown-service"
	}

	moduleName := strings.TrimSpace(module)
	if moduleName == "" {
		moduleName = "unknown"
	}

	// Create console handler
	var consoleHandler slog.Handler
	switch logType {
	case "pretty":
		consoleHandler = &minimalHandler{
			w:       os.Stdout,
			level:   level,
			colored: cfg.Colored,
			module:  moduleName,
		}
	case "text", "console":
		consoleHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level:       level,
			ReplaceAttr: replaceBuiltInAttrs,
		})
	default:
		consoleHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:       level,
			ReplaceAttr: replaceBuiltInAttrs,
		})
	}

	// Determine handlers based on access/error config
	var handlers []slog.Handler

	// Access handler
	accessHandler := getOutputHandler(cfg.Access, level, logType, cfg.Colored, moduleName)
	if accessHandler != nil {
		handlers = append(handlers, accessHandler)
	}

	// Error handler (only for error level and above)
	if cfg.Error != "none" && cfg.Error != "" {
		errorHandler := getOutputHandler(cfg.Error, slog.LevelError, logType, cfg.Colored, moduleName)
		if errorHandler != nil {
			handlers = append(handlers, &errorLevelFilter{handler: errorHandler})
		}
	}

	// Fallback to console if no handlers configured
	if len(handlers) == 0 {
		handlers = append(handlers, consoleHandler)
	}

	var finalHandler slog.Handler
	if len(handlers) == 1 {
		finalHandler = handlers[0]
	} else {
		finalHandler = &multiHandler{handlers: handlers}
	}

	return slog.New(finalHandler).With(
		slog.String("service", name),
		slog.String("module", moduleName),
		slog.Int("pid", os.Getpid()),
	)
}

// getOutputHandler creates a handler for the given output destination
func getOutputHandler(output string, level slog.Level, logType string, colored bool, moduleName string) slog.Handler {
	if output == "" || output == "none" {
		return nil
	}

	var writer io.Writer
	switch strings.ToLower(strings.TrimSpace(output)) {
	case "stdout":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	default:
		// File path
		dir := filepath.Dir(output)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil
		}
		f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil
		}
		writer = f
	}

	switch logType {
	case "pretty":
		return &minimalHandler{
			w:       writer,
			level:   level,
			colored: colored,
			module:  moduleName,
		}
	case "text", "console":
		return slog.NewTextHandler(writer, &slog.HandlerOptions{
			Level:       level,
			ReplaceAttr: replaceBuiltInAttrs,
		})
	default:
		return slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level:       level,
			ReplaceAttr: replaceBuiltInAttrs,
		})
	}
}

// errorLevelFilter wraps a handler and only passes error level and above records
type errorLevelFilter struct {
	handler slog.Handler
}

func (f *errorLevelFilter) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= slog.LevelError && f.handler.Enabled(ctx, level)
}

func (f *errorLevelFilter) Handle(ctx context.Context, r slog.Record) error {
	return f.handler.Handle(ctx, r)
}

func (f *errorLevelFilter) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &errorLevelFilter{handler: f.handler.WithAttrs(attrs)}
}

func (f *errorLevelFilter) WithGroup(name string) slog.Handler {
	return &errorLevelFilter{handler: f.handler.WithGroup(name)}
}

// minimalHandler implements a minimal log format: [LEVEL] time module: message
type minimalHandler struct {
	w       io.Writer
	level   slog.Level
	colored bool
	module  string
}

func (h *minimalHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *minimalHandler) Handle(ctx context.Context, r slog.Record) error {
	// Extract worker from attrs and build attrs string
	var worker string
	var attrs []string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "worker" {
			worker = a.Value.String()
		} else if a.Key != slog.TimeKey && a.Key != slog.LevelKey && a.Key != slog.MessageKey {
			attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
		}
		return true
	})

	// Format level
	levelStr := r.Level.String()
	if len(levelStr) > 4 {
		levelStr = levelStr[:4]
	}

	// Build colored level
	var levelOutput string
	var reset string
	if h.colored {
		colors := map[slog.Level]string{
			slog.LevelDebug: "\033[36m", // cyan
			slog.LevelInfo:  "\033[32m", // green
			slog.LevelWarn:  "\033[33m", // yellow
			slog.LevelError: "\033[31m", // red
		}
		reset = "\033[0m"
		if color, ok := colors[r.Level]; ok {
			levelOutput = fmt.Sprintf("%s[%s]%s", color, levelStr, reset)
		} else {
			levelOutput = fmt.Sprintf("[%s]", levelStr)
		}
	} else {
		levelOutput = fmt.Sprintf("[%s]", levelStr)
	}

	// Build colored module
	var moduleOutput string
	if h.colored {
		if h.module != "" {
			moduleOutput = fmt.Sprintf("\033[35m(%s)%s", h.module, reset) // magenta
		} else {
			moduleOutput = ""
		}
	} else {
		if h.module != "" {
			moduleOutput = fmt.Sprintf("(%s)", h.module)
		} else {
			moduleOutput = ""
		}
	}

	// Build worker suffix
	var workerSuffix string
	if worker != "" {
		if h.colored {
			workerSuffix = fmt.Sprintf(" \033[90m[%s]%s", worker, reset) // gray
		} else {
			workerSuffix = fmt.Sprintf(" [%s]", worker)
		}
	}

	// Build attrs suffix
	var attrsSuffix string
	if len(attrs) > 0 {
		attrsSuffix = " | " + strings.Join(attrs, " | ")
	}

	// Build output with proper spacing
	var output string
	if moduleOutput != "" {
		output = fmt.Sprintf("%s %s%s: %s%s\n", levelOutput, moduleOutput, workerSuffix, r.Message, attrsSuffix)
	} else {
		output = fmt.Sprintf("%s%s: %s%s\n", levelOutput, workerSuffix, r.Message, attrsSuffix)
	}

	// Write output
	_, err := h.w.Write([]byte(output))
	return err
}

func (h *minimalHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *minimalHandler) WithGroup(name string) slog.Handler {
	return h
}

// createFileHandler creates a JSON handler for file logging with rotation support.
func createFileHandler(filePath string, level slog.Level, rotation config.RotationConfig) (slog.Handler, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// Use lumberjack for log rotation
	lumberjackLogger := &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    rotation.MaxSizeMB,
		MaxBackups: rotation.MaxBackups,
		MaxAge:     rotation.MaxAgeDays,
		Compress:   rotation.Compress,
	}

	return slog.NewJSONHandler(lumberjackLogger, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceBuiltInAttrs,
	}), nil
}

// multiHandler writes log records to multiple handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, handler := range h.handlers {
		if err := handler.Handle(ctx, r); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

func parseLevel(raw string) slog.Level {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return slog.LevelInfo
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(candidate))); err == nil {
		return level
	}

	switch strings.ToLower(candidate) {
	case "warning":
		return slog.LevelWarn
	case "fatal":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func replaceBuiltInAttrs(groups []string, attr slog.Attr) slog.Attr {
	// Skip service and pid for cleaner output
	if attr.Key == "service" || attr.Key == "pid" {
		return slog.Attr{}
	}

	// Format time compactly
	if attr.Key == slog.TimeKey {
		if value, ok := attr.Value.Any().(time.Time); ok {
			return slog.String(slog.TimeKey, value.UTC().Format("15:04:05"))
		}
	}

	return attr
}
