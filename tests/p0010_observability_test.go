package tests

import (
	"context"
	"fmt"
	"testing"

	"velyxora/packages/common"
	"velyxora/packages/logger"
)

func TestStructuredLoggingStandard(t *testing.T) {
	log := logger.NewLogger(logger.Config{
		Level:       "DEBUG",
		Format:      "JSON",
		ServiceName: "test-observability",
	})

	ctx := context.WithValue(context.Background(), "trace_id", "tr_correlation_p0010")
	ctx = context.WithValue(ctx, "user_id", "usr_jules")

	fields := map[string]interface{}{
		"price":       42000.0,
		"password":    "secret123", // sensitive should be redacted
		"api_secret":  "raw_key_secret", // sensitive should be redacted
		"wallet_id":   "wal_001",
		"event_type":  "settlement",
		"result":      "SUCCESS",
	}

	log.LogEvent(ctx, "INFO", "settlement-engine", "SETTLED", "SUCCESS", "", fields)

	if !logger.IsSensitiveField("password") {
		t.Errorf("expected 'password' to be identified as sensitive")
	}
	if !logger.IsSensitiveField("mfa_secret") {
		t.Errorf("expected 'mfa_secret' to be identified as sensitive")
	}
	if logger.IsSensitiveField("wallet_id") {
		t.Errorf("expected 'wallet_id' not to be classified as sensitive")
	}
}

func TestObservabilityManagerMetricsTrack(t *testing.T) {
	mgr := common.GetObservabilityManager()
	mgr.SetServiceName("matching-engine")

	// Verify metrics increments
	mgr.IncrementCounter("orders_accepted_total", map[string]string{"symbol": "BTC-USDT"})
	mgr.IncrementCounter("orders_accepted_total", map[string]string{"symbol": "BTC-USDT"})
	mgr.SetGauge("matching_latency_ns", 120.0, map[string]string{"symbol": "BTC-USDT"})
	mgr.ObserveHistogram("api_latency_ms", 15.5, map[string]string{"path": "/health"})

	snapshot := mgr.SnapshotMetrics()
	var foundCounter, foundGauge, foundHistogram bool
	for _, m := range snapshot {
		if m.Name == "orders_accepted_total" && m.Value == 2.0 {
			foundCounter = true
		}
		if m.Name == "matching_latency_ns" && m.Value == 120.0 {
			foundGauge = true
		}
		if m.Name == "api_latency_ms" && m.Value == 15.5 {
			foundHistogram = true
		}
	}

	if !foundCounter {
		t.Errorf("failed to retrieve expected incremented counter metric value")
	}
	if !foundGauge {
		t.Errorf("failed to retrieve expected matching latency gauge metric value")
	}
	if !foundHistogram {
		t.Errorf("failed to retrieve expected histogram latency metric value")
	}
}

func TestObservabilityManagerUptimeAndResourceUtilization(t *testing.T) {
	mgr := common.GetObservabilityManager()
	res := mgr.CollectSystemResources()

	if res["service"] != "matching-engine" {
		t.Errorf("expected service to be 'matching-engine', got %v", res["service"])
	}
}

func TestDependencyHealthAndReadinessAggregation(t *testing.T) {
	mgr := common.GetObservabilityManager()

	// 1. Success liveness checks
	status, details := mgr.CheckDependencyHealth(
		context.Background(),
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return nil },
		nil,
		nil,
	)

	if status != common.StatusReady {
		t.Errorf("expected READY status, got %s", status)
	}
	if details["database"] != "UP" || details["redis"] != "UP" {
		t.Errorf("expected dependencies to be healthy: %v", details)
	}

	// 2. Critical mandatory dependency unavailable
	status2, details2 := mgr.CheckDependencyHealth(
		context.Background(),
		func(ctx context.Context) error { return fmt.Errorf("postgres timeout") },
		func(ctx context.Context) error { return nil },
		nil,
		nil,
	)

	if status2 != common.StatusNotReady {
		t.Errorf("expected NOT_READY status when mandatory DB is unavailable, got %s", status2)
	}
	if details2["database"] != "DOWN: postgres timeout" {
		t.Errorf("expected details to report DB DOWN, got %v", details2["database"])
	}
}
