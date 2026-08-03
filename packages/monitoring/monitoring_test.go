package monitoring

import (
	"context"
	"testing"
)

func TestMonitoringAndIncidentEscalation(t *testing.T) {
	ms := NewMonitoringService()

	// Initial health state should be HEALTHY
	health := ms.GetSystemHealth()
	if health["DATABASE"] != "HEALTHY" {
		t.Errorf("Expected DATABASE to be HEALTHY, got %s", health["DATABASE"])
	}

	// Trigger unhealthy status on critical Database
	ms.UpdateComponentHealth("DATABASE", "UNHEALTHY")

	// Critical database failure should auto-trigger a Critical System Incident!
	incidents := ms.ListIncidents()
	if len(incidents) != 1 {
		t.Fatalf("Expected 1 critical incident auto-opened, got %d", len(incidents))
	}

	inc := incidents[0]
	if inc.Severity != SeverityCritical || inc.Status != StatusOpen {
		t.Errorf("Mismatch in escalated incident attributes: severity %s status %s", inc.Severity, inc.Status)
	}

	// Automated recovery
	recovered, err := ms.TriggerOperationalRecovery(context.Background(), "DATABASE")
	if err != nil {
		t.Fatalf("Recovery failed: %v", err)
	}
	if !recovered {
		t.Error("Expected DATABASE to recover successfully")
	}

	health = ms.GetSystemHealth()
	if health["DATABASE"] != "HEALTHY" {
		t.Errorf("Expected DATABASE health restored, got %s", health["DATABASE"])
	}
}

func TestFraudDetectionRules(t *testing.T) {
	ms := NewMonitoringService()

	// Normal transaction should not trigger fraud
	fraud, err := ms.EvaluateTransactionFraud("usr_007", "WITHDRAWAL", 10.0, "127.0.0.1", 5.0)
	if err != nil {
		t.Fatalf("EvaluateTransactionFraud failed: %v", err)
	}
	if fraud != nil {
		t.Error("Expected no fraud flagging for normal transaction, got a flagged event")
	}

	// Abnormal Transaction (High volume + geographic anomaly + high IP risk pool)
	fraud, err = ms.EvaluateTransactionFraud("usr_fraudster", "WITHDRAWAL", 15000.0, "192.168.99.99", 1500.0)
	if err != nil {
		t.Fatalf("EvaluateTransactionFraud failed: %v", err)
	}

	if fraud == nil {
		t.Fatal("Expected fraud transaction to be flagged, got nil")
	}

	if fraud.RiskScore < 0.7 {
		t.Errorf("Expected risk score to be highly escalated, got %.2f", fraud.RiskScore)
	}

	alerts := ms.ListAlerts()
	if len(alerts) == 0 {
		t.Error("Expected a security alert generated for high-risk transaction fraud, got zero")
	}
}
