package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Logger wraps slog.Logger to provide structured logging capabilities
type Logger struct {
	*slog.Logger
}

// Config defines options for setting up the logger
type Config struct {
	Level       string // DEBUG, INFO, WARN, ERROR
	Format      string // JSON or TEXT
	ServiceName string // Name of the microservice
}

// NewLogger initializes a production-grade slog structured logger
func NewLogger(cfg Config) *Logger {
	var level slog.Level
	switch cfg.Level {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	var writer io.Writer = os.Stdout

	opts := &slog.HandlerOptions{
		Level: level,
	}

	if cfg.Format == "JSON" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	// Enrich with service name context
	if cfg.ServiceName != "" {
		handler = handler.WithAttrs([]slog.Attr{
			slog.String("service", cfg.ServiceName),
		})
	}

	return &Logger{
		Logger: slog.New(handler),
	}
}

// WithContext allows adding extra context variables to the log output
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Here you could extract trace IDs or span IDs if using OTEL/tracing
	return l
}
