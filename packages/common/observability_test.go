package common_test

import (
	"context"
	"errors"
	"testing"
	"velyxora/packages/common"
)

func TestObservabilityManagerMetrics(t *testing.T) {
	mgr := common.GetObservabilityManager()
	mgr.SetServiceName("test-service")

	mgr.IncrementCounter("api_requests_total", map[string]string{"path": "/health"})
	mgr.SetGauge("cpu_usage_pct", 12.5, map[string]string{"node": "node-1"})
	mgr.ObserveHistogram("api_latency_ms", 45.2, map[string]string{"path": "/health"})

	metrics := mgr.SnapshotMetrics()
	if len(metrics) < 3 {
		t.Errorf("expected at least 3 snapshot metrics, got %d", len(metrics))
	}

	resources := mgr.CollectSystemResources()
	if resources["service"] != "test-service" {
		t.Errorf("expected service to be 'test-service', got %v", resources["service"])
	}
}

func TestDependencyHealthCheck(t *testing.T) {
	mgr := common.GetObservabilityManager()

	// 1. Success case
	status, details := mgr.CheckDependencyHealth(
		context.Background(),
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return nil },
	)

	if status != common.StatusReady {
		t.Errorf("expected READY status, got %s", status)
	}
	if details["database"] != "UP" || details["redis"] != "UP" || details["kafka"] != "UP" || details["blockchain"] != "UP" {
		t.Errorf("expected all dependencies UP, got %v", details)
	}

	// 2. Degraded case (Kafka or Blockchain down)
	status2, details2 := mgr.CheckDependencyHealth(
		context.Background(),
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return errors.New("kafka connection lost") },
		func(ctx context.Context) error { return nil },
	)

	if status2 != common.StatusDegraded {
		t.Errorf("expected DEGRADED status, got %s", status2)
	}
	if details2["kafka"] != "DOWN: kafka connection lost" {
		t.Errorf("expected kafka to be DOWN with message, got %s", details2["kafka"])
	}

	// 3. Not Ready case (DB or Redis down)
	status3, details3 := mgr.CheckDependencyHealth(
		context.Background(),
		func(ctx context.Context) error { return errors.New("database connection timeout") },
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return nil },
	)

	if status3 != common.StatusNotReady {
		t.Errorf("expected NOT_READY status, got %s", status3)
	}
	if details3["database"] != "DOWN: database connection timeout" {
		t.Errorf("expected database to be DOWN with message, got %s", details3["database"])
	}
}

func TestCategorizedErrors(t *testing.T) {
	err := common.NewCategorizedError(common.ErrorValidation, "VAL_001", "invalid email address", nil)
	if err.Type != common.ErrorValidation {
		t.Errorf("expected validation error, got %s", err.Type)
	}
	if err.Code != "VAL_001" {
		t.Errorf("expected code VAL_001, got %s", err.Code)
	}
	if err.Error() != "[VALIDATION] invalid email address" {
		t.Errorf("expected formatted string error, got %s", err.Error())
	}
}
