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
	if ctx == nil {
		return l
	}

	attrs := []slog.Attr{}

	// Safely retrieve standard context fields
	if traceID, ok := ctx.Value("trace_id").(string); ok && traceID != "" {
		attrs = append(attrs, slog.String("correlation_id", traceID))
		attrs = append(attrs, slog.String("request_id", traceID))
	}
	if correlationID, ok := ctx.Value("correlation_id").(string); ok && correlationID != "" {
		attrs = append(attrs, slog.String("correlation_id", correlationID))
	}
	if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
		attrs = append(attrs, slog.String("user_id", userID))
	}
	if orderID, ok := ctx.Value("order_id").(string); ok && orderID != "" {
		attrs = append(attrs, slog.String("order_id", orderID))
	}
	if txID, ok := ctx.Value("transaction_id").(string); ok && txID != "" {
		attrs = append(attrs, slog.String("transaction_id", txID))
	}
	if symbol, ok := ctx.Value("symbol").(string); ok && symbol != "" {
		attrs = append(attrs, slog.String("symbol", symbol))
	}

	if len(attrs) > 0 {
		return &Logger{
			Logger: slog.New(l.Handler().WithAttrs(attrs)),
		}
	}

	return l
}

// LogEvent provides structured fields for structured audit trail compliance checks
func (l *Logger) LogEvent(ctx context.Context, level string, component string, eventType string, result string, errCode string, fields map[string]interface{}) {
	attrs := []slog.Attr{
		slog.String("component", component),
		slog.String("event_type", eventType),
		slog.String("result", result),
	}
	if errCode != "" {
		attrs = append(attrs, slog.String("error_code", errCode))
	}

	// Filter and sanitize sensitive metadata values before output logging
	for k, v := range fields {
		if IsSensitiveField(k) {
			attrs = append(attrs, slog.String(k, "[REDACTED_SENSITIVE_DATA]"))
		} else {
			attrs = append(attrs, slog.Any(k, v))
		}
	}

	logInstance := l.WithContext(ctx)

	switch level {
	case "DEBUG":
		logInstance.Logger.LogAttrs(ctx, slog.LevelDebug, "Event logged", attrs...)
	case "WARN":
		logInstance.Logger.LogAttrs(ctx, slog.LevelWarn, "Event logged", attrs...)
	case "ERROR":
		logInstance.Logger.LogAttrs(ctx, slog.LevelError, "Event logged", attrs...)
	default:
		logInstance.Logger.LogAttrs(ctx, slog.LevelInfo, "Event logged", attrs...)
	}
}

// IsSensitiveField returns true if the key refers to high security parameters
func IsSensitiveField(key string) bool {
	sensitive := map[string]bool{
		"password":      true,
		"private_key":   true,
		"api_secret":    true,
		"mfa_secret":    true,
		"refresh_token": true,
		"kyc_document":  true,
		"credential":    true,
		"access_token":  true,
		"passphrase":    true,
		"secret":        true,
	}
	return sensitive[key]
}
