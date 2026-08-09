package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
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
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			keyLower := strings.ToLower(a.Key)
			if keyLower == "password" || keyLower == "jwt" || keyLower == "api_key" || keyLower == "secret" || keyLower == "private_key" || keyLower == "seed" || keyLower == "mfa" || keyLower == "token" {
				return slog.String(a.Key, "[REDACTED]")
			}
			if valStr, ok := a.Value.Any().(string); ok {
				return slog.String(a.Key, Redact(valStr))
			}
			return a
		},
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
	if ctx == nil {
		return l
	}

	attrs := []slog.Attr{}

	// 1. Extract standard trace_id from context if available
	if val, ok := ctx.Value("trace_id").(string); ok && val != "" {
		attrs = append(attrs, slog.String("trace_id", val))
	}

	// 2. Extract OpenTelemetry TraceID if available
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		attrs = append(attrs, slog.String("trace_id", spanContext.TraceID().String()))
		attrs = append(attrs, slog.String("span_id", spanContext.SpanID().String()))
	}

	return l.WithAttrs(attrs)
}

// WithAttrs returns a logger with custom attributes
func (l *Logger) WithAttrs(attrs []slog.Attr) *Logger {
	if len(attrs) == 0 {
		return l
	}
	args := make([]interface{}, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	return &Logger{
		Logger: l.Logger.With(args...),
	}
}

// Redact returns a redacted copy of a string if it contains sensitive tokens
func Redact(val string) string {
	sensitiveKeywords := []string{"password", "jwt", "api_key", "secret", "private_key", "seed", "mnemonic", "mfa", "passphrase", "token"}
	valLower := strings.ToLower(val)
	for _, kw := range sensitiveKeywords {
		if strings.Contains(valLower, kw) || isToken(val) {
			return "[REDACTED]"
		}
	}
	return val
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func isToken(s string) bool {
	if len(s) < 32 || strings.Contains(s, " ") {
		return false
	}
	// Check if s consists purely of typical base64/hex token characters without whitespace
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '=' || r == '+' || r == '/' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
