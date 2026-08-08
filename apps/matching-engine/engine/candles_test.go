package engine

import (
	"context"
	"testing"
	"time"
)

func TestCandleEngineDeterministicAggregation(t *testing.T) {
	// Standalone/in-memory test without PG/Redis
	ce := NewCandleEngine(nil, nil)

	symbol := "BTC-USDT"
	now := time.Date(2025, 1, 1, 12, 0, 5, 0, time.UTC)

	// Process first trade
	err := ce.ProcessTrade(context.Background(), symbol, 50000.0, 1.5, now)
	if err != nil {
		t.Fatalf("Failed to process trade: %v", err)
	}

	// Boundary calculations
	o1, c1 := GetIntervalBoundary(now, "1m")
	if o1 != time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC) {
		t.Errorf("Expected 1m open time to be truncated, got %v", o1)
	}
	if c1 != time.Date(2025, 1, 1, 12, 1, 0, 0, time.UTC) {
		t.Errorf("Expected 1m close time to be 12:01:00, got %v", c1)
	}

	o5, c5 := GetIntervalBoundary(now, "5m")
	if o5 != time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC) {
		t.Errorf("Expected 5m open time to be 12:00:00, got %v", o5)
	}
	if c5 != time.Date(2025, 1, 1, 12, 5, 0, 0, time.UTC) {
		t.Errorf("Expected 5m close time to be 12:05:00, got %v", c5)
	}
}
