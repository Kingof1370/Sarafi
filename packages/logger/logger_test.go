package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestLoggerJSON(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.NewJSONHandler(&buf, opts)
	slogLogger := slog.New(handler)

	l := &Logger{Logger: slogLogger}
	l.Info("test message", slog.String("key", "val"))

	var data map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if data["msg"] != "test message" {
		t.Errorf("Expected msg to be 'test message', got '%v'", data["msg"])
	}
	if data["key"] != "val" {
		t.Errorf("Expected key 'key' to be 'val', got '%v'", data["key"])
	}
}
