package common

import (
	"context"
	"log/slog"
	"os"
)

// LoggerKey is the context key for logger values.
type LoggerKey struct{}

// Fields represents structured logging fields.
type Fields map[string]any

// SetupLogger configures the global logger with appropriate settings.
func SetupLogger(level slog.Level, format string) error {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			// Customize attribute formatting if needed
			return a
		},
	}

	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	case "console":
		handler = slog.NewTextHandler(os.Stderr, opts)
	default:
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return nil
}

// LogError logs an error with additional context.
func LogError(err error, msg string, fields Fields) {
	attrs := make([]slog.Attr, 0, len(fields)+1)
	attrs = append(attrs, slog.String("error", err.Error()))

	for k, v := range fields {
		attrs = append(attrs, slog.Any(k, v))
	}

	slog.LogAttrs(context.Background(), slog.LevelError, msg, attrs...)
}

// LogInfo logs an info message with fields.
func LogInfo(msg string, fields Fields) {
	attrs := make([]slog.Attr, 0, len(fields))
	for k, v := range fields {
		attrs = append(attrs, slog.Any(k, v))
	}

	slog.LogAttrs(context.Background(), slog.LevelInfo, msg, attrs...)
}

// LogDebug logs a debug message with fields.
func LogDebug(msg string, fields Fields) {
	attrs := make([]slog.Attr, 0, len(fields))
	for k, v := range fields {
		attrs = append(attrs, slog.Any(k, v))
	}

	slog.LogAttrs(context.Background(), slog.LevelDebug, msg, attrs...)
}
