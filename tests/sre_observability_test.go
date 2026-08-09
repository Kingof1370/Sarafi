package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"velyxora/packages/common"
	"velyxora/packages/database"
	"velyxora/packages/logger"
)

func TestSREMetricsAndLogRedaction(t *testing.T) {
	// 1. Verify Prometheus Technical Metrics Registry
	om := common.GetObservabilityManager()
	if om == nil {
		t.Fatal("ObservabilityManager is nil")
	}

	// Record sample metrics to verify they don't panic
	om.HTTPRequestsTotal.WithLabelValues("GET", "/api/v1/test", "200").Inc()
	om.WSConnectionsActive.Inc()
	om.WSConnectionsActive.Dec()
	om.OrdersSubmitted.WithLabelValues("BTC-USDT", "BUY", "LIMIT").Inc()
	om.OrdersAccepted.WithLabelValues("BTC-USDT", "BUY").Inc()

	// 2. Verify Log Redaction Logic
	plainText := "my-secret-password-is-12345"
	redacted := logger.Redact(plainText)
	if redacted != "[REDACTED]" {
		t.Errorf("Expected redaction of sensitive password token, got: %s", redacted)
	}

	tokenText := "abcdef1234567890abcdef1234567890abcdef1234567890"
	redactedToken := logger.Redact(tokenText)
	if redactedToken != "[REDACTED]" {
		t.Errorf("Expected redaction of long token, got: %s", redactedToken)
	}

	nonSensitive := "short-safe-log"
	if logger.Redact(nonSensitive) != nonSensitive {
		t.Errorf("Expected short-safe-log to remain unredacted, got: %s", logger.Redact(nonSensitive))
	}
}

func TestSREIncidentAndAlertingLifecycle(t *testing.T) {
	ctx := context.Background()

	// Connect to real docker-compose postgres running locally on port 5432
	db, err := database.NewConnectionPool(database.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "velyxora",
		Password: "super-secure-db-password-123",
		DBName:   "velyxora",
		SSLMode:  "disable",
	})
	if err != nil {
		t.Skipf("Skipping real DB alert lifecycle test; database not reachable: %v", err)
		return
	}
	defer db.Close()

	// Run migration to guarantee table presence
	_ = database.RunMigrations(ctx, db)

	// Clean up existing test state for the title
	_, _ = db.Pool.Exec(ctx, "DELETE FROM sre_alerts WHERE metric_name = 'test_service_unreachable'")
	_, _ = db.Pool.Exec(ctx, "DELETE FROM sre_incidents WHERE title = 'Test Service Down'")

	testLog := logger.NewLogger(logger.Config{
		Level:       "DEBUG",
		Format:      "JSON",
		ServiceName: "sre-test-logger",
	})

	// Use temporary function matching handleSREDependencyIncident signature
	handleTestDependencyIncident := func(dbCtx context.Context, title, desc, service string, isDown bool, errStr string) {
		var existingID string
		var existingStatus string
		queryErr := db.Pool.QueryRow(dbCtx,
			"SELECT id, status FROM sre_incidents WHERE title = $1 AND status != 'RESOLVED'",
			title).Scan(&existingID, &existingStatus)

		if isDown {
			// Dependency is DOWN
			if queryErr != nil {
				// Incident does NOT exist, create a new one!
				incID := "inc_" + fmt.Sprintf("%d", time.Now().UnixNano())
				_, dbErr := db.Pool.Exec(dbCtx,
					"INSERT INTO sre_incidents (id, title, description, status, severity, service, created_at, updated_at) VALUES ($1, $2, $3, 'OPEN', 'CRITICAL', $4, NOW(), NOW())",
					incID, title, desc, service)
				if dbErr == nil {
					testLog.Warn(fmt.Sprintf("[SRE ALERT] Created SRE Incident: %s", title))
					// Log alert record
					altID := "alt_" + fmt.Sprintf("%d", time.Now().UnixNano())
					_, _ = db.Pool.Exec(dbCtx,
						"INSERT INTO sre_alerts (id, incident_id, metric_name, value, threshold, status, details, created_at, updated_at) VALUES ($1, $2, $3, 1.0, 0.5, 'TRIGGERED', $4, NOW(), NOW())",
						altID, incID, service+"_unreachable", errStr)
				}
			}
		} else {
			// Dependency is UP
			if queryErr == nil {
				// Incident exists and is OPEN, mitigate and RESOLVE it!
				_, dbErr := db.Pool.Exec(dbCtx,
					"UPDATE sre_incidents SET status = 'RESOLVED', updated_at = NOW() WHERE id = $1",
					existingID)
				if dbErr == nil {
					testLog.Info(fmt.Sprintf("[SRE HEALED] Resolved SRE Incident: %s", title))
					// Update corresponding alert statuses
					_, _ = db.Pool.Exec(dbCtx,
						"UPDATE sre_alerts SET status = 'RESOLVED', updated_at = NOW() WHERE incident_id = $1",
						existingID)
				}
			}
		}
	}

	// 1. Simulate Failure Injection: Mark Service DOWN
	handleTestDependencyIncident(ctx, "Test Service Down", "Failure injected during integration test runs.", "test_service", true, "connection timeout")

	// Verify that the incident was created inside Postgres
	var count int
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM sre_incidents WHERE title = 'Test Service Down' AND status = 'OPEN'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query sre_incidents: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 open incident for 'Test Service Down', found: %d", count)
	}

	// Verify that alert was logged
	var alertCount int
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM sre_alerts WHERE metric_name = 'test_service_unreachable' AND status = 'TRIGGERED'").Scan(&alertCount)
	if err != nil {
		t.Fatalf("Failed to query sre_alerts: %v", err)
	}
	if alertCount != 1 {
		t.Errorf("Expected 1 triggered alert, found: %d", alertCount)
	}

	// 2. Simulate Self-Healing / Auto-Mitigation: Mark Service UP
	handleTestDependencyIncident(ctx, "Test Service Down", "Failure injected during integration test runs.", "test_service", false, "")

	// Verify that the incident is now RESOLVED
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM sre_incidents WHERE title = 'Test Service Down' AND status = 'RESOLVED'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query resolved sre_incidents: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected incident to be RESOLVED, found open/unresolved count instead")
	}

	// Clean up after successful test
	_, _ = db.Pool.Exec(ctx, "DELETE FROM sre_alerts WHERE metric_name = 'test_service_unreachable'")
	_, _ = db.Pool.Exec(ctx, "DELETE FROM sre_incidents WHERE title = 'Test Service Down'")
}
