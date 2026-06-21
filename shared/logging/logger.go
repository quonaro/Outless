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
)

const (
	envLogLevel  = "OUTLESS_LOG_LEVEL"
	envLogFormat = "OUTLESS_LOG_FORMAT"
)

// New creates a process logger with unified format across services.
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

	var consoleHandler slog.Handler
	switch logType {
	case "pretty":
		consoleHandler = &minimalHandler{w: os.Stdout, level: level, colored: cfg.Colored, module: moduleName}
	case "text", "console":
		consoleHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level, ReplaceAttr: replaceBuiltInAttrs})
	default:
		consoleHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level, ReplaceAttr: replaceBuiltInAttrs})
	}

	var finalHandler slog.Handler
	if output := getOutputHandler(cfg.Output, level, logType, cfg.Colored, moduleName); output != nil {
		finalHandler = output
	} else {
		finalHandler = consoleHandler
	}

	return slog.New(finalHandler).With(
		slog.String("service", name),
		slog.String("module", moduleName),
		slog.Int("pid", os.Getpid()),
	)
}

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
		dir := filepath.Dir(output)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil
		}
		f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil
		}
		writer = f
	}

	switch logType {
	case "pretty":
		return &minimalHandler{w: writer, level: level, colored: colored, module: moduleName}
	case "text", "console":
		return slog.NewTextHandler(writer, &slog.HandlerOptions{Level: level, ReplaceAttr: replaceBuiltInAttrs})
	default:
		return slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level, ReplaceAttr: replaceBuiltInAttrs})
	}
}

// minimalHandler implements a minimal log format: [LEVEL] (module): message
type minimalHandler struct {
	w       io.Writer
	level   slog.Level
	colored bool
	module  string
}

func (h *minimalHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *minimalHandler) Handle(_ context.Context, r slog.Record) error {
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

	levelStr := r.Level.String()
	if len(levelStr) > 4 {
		levelStr = levelStr[:4]
	}

	var levelOutput, reset string
	if h.colored {
		colors := map[slog.Level]string{
			slog.LevelDebug: "\033[36m",
			slog.LevelInfo:  "\033[32m",
			slog.LevelWarn:  "\033[33m",
			slog.LevelError: "\033[31m",
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

	var moduleOutput string
	if h.module != "" {
		if h.colored {
			moduleOutput = fmt.Sprintf("\033[35m(%s)%s", h.module, reset)
		} else {
			moduleOutput = fmt.Sprintf("(%s)", h.module)
		}
	}

	var workerSuffix string
	if worker != "" {
		if h.colored {
			workerSuffix = fmt.Sprintf(" \033[90m[%s]%s", worker, reset)
		} else {
			workerSuffix = fmt.Sprintf(" [%s]", worker)
		}
	}

	var attrsSuffix string
	if len(attrs) > 0 {
		attrsSuffix = " | " + strings.Join(attrs, " | ")
	}

	var output string
	if moduleOutput != "" {
		output = fmt.Sprintf("%s %s%s: %s%s\n", levelOutput, moduleOutput, workerSuffix, r.Message, attrsSuffix)
	} else {
		output = fmt.Sprintf("%s%s: %s%s\n", levelOutput, workerSuffix, r.Message, attrsSuffix)
	}

	_, err := h.w.Write([]byte(output))
	return err
}

func (h *minimalHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *minimalHandler) WithGroup(_ string) slog.Handler      { return h }

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

func replaceBuiltInAttrs(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key == "service" || attr.Key == "pid" {
		return slog.Attr{}
	}
	if attr.Key == slog.TimeKey {
		if value, ok := attr.Value.Any().(time.Time); ok {
			return slog.String(slog.TimeKey, value.UTC().Format("15:04:05"))
		}
	}
	return attr
}
