package logger_test

import (
	"context"
	"testing"
	"velyxora/packages/logger"
)

func TestLoggerWithContextAndLogEvent(t *testing.T) {
	l := logger.NewLogger(logger.Config{
		Level:       "DEBUG",
		Format:      "JSON",
		ServiceName: "test-service",
	})

	ctx := context.WithValue(context.Background(), "trace_id", "tr_123456")
	ctx = context.WithValue(ctx, "user_id", "usr_999")
	ctx = context.WithValue(ctx, "order_id", "ord_777")

	fields := map[string]interface{}{
		"price":       50000.0,
		"password":    "mysecretpassword", // Should be redacted
		"api_secret":  "apikey_secret",    // Should be redacted
		"mfa_secret":  "totp_secret",      // Should be redacted
		"kyc_document": "user_id_passport", // Should be redacted
	}

	l.LogEvent(ctx, "INFO", "matching_engine", "ORDER_EXECUTED", "SUCCESS", "", fields)

	if !logger.IsSensitiveField("password") {
		t.Errorf("expected 'password' to be identified as sensitive")
	}
	if !logger.IsSensitiveField("mfa_secret") {
		t.Errorf("expected 'mfa_secret' to be identified as sensitive")
	}
}
