package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"velyxora/packages/monitoring"
)

// TestMonitoringComprehensive covers component health checks, automated escalations, and transaction fraud evaluations
func TestMonitoringComprehensive(t *testing.T) {
	ms := monitoring.NewMonitoringService()
	ctx := context.Background()

	// 1. Initial State checks
	health := ms.GetSystemHealth()
	if health["DATABASE"] != "HEALTHY" {
		t.Errorf("Expected DATABASE health status to be HEALTHY, got %s", health["DATABASE"])
	}

	// 2. Evaluate Normal Transaction (Low risk)
	fraud, err := ms.EvaluateTransactionFraud("usr_alice", "WITHDRAWAL", 150.0, "127.0.0.1", 20.0)
	if err != nil {
		t.Fatalf("Fraud evaluation failed: %v", err)
	}
	if fraud != nil {
		t.Error("Expected low-risk transaction to be clean, but was flagged")
	}

	// 3. Evaluate Anomalous High Risk Transaction (High risk geo distance, blacklisted IP, large amount)
	fraud, err = ms.EvaluateTransactionFraud("usr_fraudster", "DEPOSIT", 25000.0, "192.168.99.99", 2500.0)
	if err != nil {
		t.Fatalf("Fraud evaluation failed: %v", err)
	}

	if fraud == nil {
		t.Fatal("Expected high-risk transaction to be flagged as fraud, got nil")
	}

	if fraud.RiskScore != 1.0 {
		t.Errorf("Expected maximum risk score 1.0, got %.2f", fraud.RiskScore)
	}

	// High risk fraud must trigger urgent security alerts
	alerts := ms.ListAlerts()
	if len(alerts) != 1 {
		t.Fatalf("Expected exactly 1 urgent alert triggered, got %d", len(alerts))
	}

	if alerts[0].Severity != monitoring.SeverityHigh {
		t.Errorf("Expected alert severity HIGH, got %s", alerts[0].Severity)
	}

	// 4. Trigger Unhealthy Critical status and verify incident auto-escalation
	ms.UpdateComponentHealth("KAFKA", "UNHEALTHY")

	health = ms.GetSystemHealth()
	if health["KAFKA"] != "UNHEALTHY" {
		t.Errorf("Expected KAFKA health state to be UNHEALTHY, got %s", health["KAFKA"])
	}

	incidents := ms.ListIncidents()
	if len(incidents) != 1 {
		t.Fatalf("Expected 1 automated incident opened, got %d", len(incidents))
	}

	if incidents[0].Severity != monitoring.SeverityCritical {
		t.Errorf("Expected critical severity incident opened, got %s", incidents[0].Severity)
	}

	// 5. Automated Operational Recovery
	healed, err := ms.TriggerOperationalRecovery(ctx, "KAFKA")
	if err != nil {
		t.Fatalf("Operational recovery failed: %v", err)
	}
	if !healed {
		t.Error("Expected recovery process to heal component successfully")
	}

	health = ms.GetSystemHealth()
	if health["KAFKA"] != "HEALTHY" {
		t.Errorf("Expected KAFKA health state healed back to HEALTHY, got %s", health["KAFKA"])
	}
}

// TestMonitoringConcurrency verifies safety under massive concurrent health updates and evaluations
func TestMonitoringConcurrency(t *testing.T) {
	ms := monitoring.NewMonitoringService()

	var wg sync.WaitGroup
	workers := 100
	rounds := 30

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				// Concurrent health check reports
				ms.UpdateComponentHealth("DATABASE", "DEGRADED")
				ms.UpdateComponentHealth("DATABASE", "HEALTHY")

				// Concurrent transactions check
				_, _ = ms.EvaluateTransactionFraud(fmt.Sprintf("usr_%d", workerID), "WITHDRAWAL", 10.0, "127.0.0.1", 5.0)
			}
		}(i)
	}

	wg.Wait()

	// Verify consistent state
	health := ms.GetSystemHealth()
	if health["DATABASE"] != "HEALTHY" {
		t.Errorf("Expected DATABASE to resolve to final HEALTHY status, got %s", health["DATABASE"])
	}
}

// Benchmarks for performance validation
func BenchmarkFraudEngineEvaluation(b *testing.B) {
	ms := monitoring.NewMonitoringService()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ms.EvaluateTransactionFraud("usr_alice", "WITHDRAWAL", 150.0, "127.0.0.1", 20.0)
	}
}

func BenchmarkAlertProcessing(b *testing.B) {
	ms := monitoring.NewMonitoringService()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ms.ListAlerts()
	}
}

func BenchmarkServiceRecovery(b *testing.B) {
	ms := monitoring.NewMonitoringService()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ms.TriggerOperationalRecovery(ctx, "DATABASE")
	}
}
