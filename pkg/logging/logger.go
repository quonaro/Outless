package logging

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"outless/pkg/config"

	"github.com/lmittmann/tint"
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
func NewFromConfig(service string, cfg config.LogsConfig) *slog.Logger {
	level := parseLevel(cfg.Level)
	logType := strings.ToLower(strings.TrimSpace(cfg.Type))

	var handler slog.Handler
	switch logType {
	case "pretty":
		if cfg.Colored {
			w := os.Stdout
			handler = tint.NewHandler(w, &tint.Options{
				Level:       level,
				TimeFormat:  time.RFC3339Nano,
				ReplaceAttr: replaceBuiltInAttrs,
			})
		} else {
			handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level:       level,
				ReplaceAttr: replaceBuiltInAttrs,
			})
		}
	case "text", "console":
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level:       level,
			ReplaceAttr: replaceBuiltInAttrs,
		})
	default:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:       level,
			ReplaceAttr: replaceBuiltInAttrs,
		})
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
	if attr.Key == slog.TimeKey {
		if value, ok := attr.Value.Any().(time.Time); ok {
			return slog.String(slog.TimeKey, value.UTC().Format(time.RFC3339Nano))
		}
	}
	return attr
}
