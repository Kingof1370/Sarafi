package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// IncidentSeverity defines classification rankings for system anomalies
type IncidentSeverity string

const (
	SeverityLow      IncidentSeverity = "LOW"
	SeverityMedium   IncidentSeverity = "MEDIUM"
	SeverityHigh     IncidentSeverity = "HIGH"
	SeverityCritical IncidentSeverity = "CRITICAL"
)

// IncidentStatus represents the operational status of an incident
type IncidentStatus string

const (
	StatusOpen     IncidentStatus = "OPEN"
	StatusResolved IncidentStatus = "RESOLVED"
	StatusClosed   IncidentStatus = "CLOSED"
)

// FraudEvent captures detailed behavioral metrics flag of anomalous actions
type FraudEvent struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	ActionType string    `json:"action_type"` // e.g. DEPOSIT, WITHDRAWAL
	RiskScore  float64   `json:"risk_score"`
	Details    string    `json:"details"`
	Timestamp  time.Time `json:"timestamp"`
}

// SystemIncident represents an operational threat event or infrastructure failure
type SystemIncident struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Severity  IncidentSeverity `json:"severity"`
	Status    IncidentStatus   `json:"status"`
	Details   string           `json:"details"`
	Timestamp time.Time        `json:"timestamp"`
}

// Alert represents a generated notification
type Alert struct {
	ID        string           `json:"id"`
	Source    string           `json:"source"`
	Message   string           `json:"message"`
	Severity  IncidentSeverity `json:"severity"`
	Timestamp time.Time        `json:"timestamp"`
}

// MonitoringService orchestrates real-time health checks, fraud rules, alerts, and automated escalations
type MonitoringService struct {
	mu           sync.RWMutex
	incidents    map[string]*SystemIncident
	fraudEvents  map[string]*FraudEvent
	alerts       []*Alert
	healthStatus map[string]string // key: componentName -> status ("HEALTHY", "DEGRADED", "UNHEALTHY")
	recoveryLogs []string
}

// NewMonitoringService initializes the enterprise supervisor platform
func NewMonitoringService() *MonitoringService {
	ms := &MonitoringService{
		incidents:    make(map[string]*SystemIncident),
		fraudEvents:  make(map[string]*FraudEvent),
		alerts:       make([]*Alert, 0),
		healthStatus: make(map[string]string),
		recoveryLogs: make([]string, 0),
	}

	// Bootstrap components health statuses
	components := []string{"DATABASE", "KAFKA", "REDIS", "WALLET_CORE", "CUSTODY_VAULTS", "TREASURY_POOLS", "API_GATEWAY", "BLOCKCHAIN_BAL"}
	for _, c := range components {
		ms.healthStatus[c] = "HEALTHY"
	}

	return ms
}

// UpdateComponentHealth sets the live operational state of an exchange component
func (ms *MonitoringService) UpdateComponentHealth(component string, status string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.healthStatus[component] = status

	// Auto-escalation: If Database or Kafka becomes UNHEALTHY, trigger immediate critical incident
	if status == "UNHEALTHY" && (component == "DATABASE" || component == "KAFKA") {
		ms.triggerCriticalIncident(component + " microservice failure detected!")
	}
}

func (ms *MonitoringService) triggerCriticalIncident(message string) {
	incID := fmt.Sprintf("inc_%d", time.Now().UnixNano())
	ms.incidents[incID] = &SystemIncident{
		ID:        incID,
		Title:     "AUTOMATED EMERGENCY ESCALATION",
		Severity:  SeverityCritical,
		Status:    StatusOpen,
		Details:   message,
		Timestamp: time.Now(),
	}
	ms.alerts = append(ms.alerts, &Alert{
		ID:        fmt.Sprintf("alt_%d", time.Now().UnixNano()),
		Source:    "HEALTH_MONITOR",
		Message:   "Critical incident opened: " + message,
		Severity:  SeverityCritical,
		Timestamp: time.Now(),
	})
}

// EvaluateTransactionFraud validates behavioral and environmental vectors to calculate risk scores
func (ms *MonitoringService) EvaluateTransactionFraud(userID, action string, amount float64, ip string, geoDistance float64) (*FraudEvent, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	riskScore := 0.1
	details := "Normal user transaction behavior."

	// 1. Large Volume check
	if amount >= 10000.0 {
		riskScore += 0.4
		details = "Anomalously large volume detected."
	}

	// 2. High geographical distance check (indicating impossible travel anomaly)
	if geoDistance >= 1000.0 {
		riskScore += 0.35
		details = details + " Impossible travel geographical anomaly detected."
	}

	// 3. Known high-risk IP profiles
	if ip == "192.168.99.99" || ip == "10.0.99.99" {
		riskScore += 0.5
		details = details + " Blacklisted network IP pool detected."
	}

	if riskScore > 1.0 {
		riskScore = 1.0
	}

	// Automatic fraud flagging
	var fraud *FraudEvent
	if riskScore >= 0.7 {
		fID := fmt.Sprintf("frd_%d", time.Now().UnixNano())
		fraud = &FraudEvent{
			ID:         fID,
			UserID:     userID,
			ActionType: action,
			RiskScore:  riskScore,
			Details:    details,
			Timestamp:  time.Now(),
		}
		ms.fraudEvents[fID] = fraud

		// Generate urgent security alert
		ms.alerts = append(ms.alerts, &Alert{
			ID:        fmt.Sprintf("alt_%d", time.Now().UnixNano()),
			Source:    "FRAUD_ENGINE",
			Message:   fmt.Sprintf("Fraud detected for User %s. Risk: %.2f. Details: %s", userID, riskScore, details),
			Severity:  SeverityHigh,
			Timestamp: time.Now(),
		})
	}

	return fraud, nil
}

// TriggerOperationalRecovery performs automated service healing and validations
func (ms *MonitoringService) TriggerOperationalRecovery(ctx context.Context, component string) (bool, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	status, exists := ms.healthStatus[component]
	if !exists {
		return false, fmt.Errorf("component %s not tracked", component)
	}

	if status == "HEALTHY" {
		return true, nil // No recovery needed
	}

	ms.recoveryLogs = append(ms.recoveryLogs, fmt.Sprintf("[%s] INITIATING RECOVERY: component %s", time.Now().Format(time.RFC3339), component))

	// Simulate automatic service healing
	ms.healthStatus[component] = "HEALTHY"
	ms.recoveryLogs = append(ms.recoveryLogs, fmt.Sprintf("[%s] RECOVERY SUCCESSFUL: component %s health restored", time.Now().Format(time.RFC3339), component))

	return true, nil
}

// GetSystemHealth returns current health statuses
func (ms *MonitoringService) GetSystemHealth() map[string]string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	health := make(map[string]string)
	for k, v := range ms.healthStatus {
		health[k] = v
	}
	return health
}

// ListIncidents returns all tracked incidents
func (ms *MonitoringService) ListIncidents() []*SystemIncident {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var list []*SystemIncident
	for _, inc := range ms.incidents {
		list = append(list, inc)
	}
	return list
}

// ListFraudAlerts returns flagged fraud logs
func (ms *MonitoringService) ListFraudAlerts() []*FraudEvent {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var list []*FraudEvent
	for _, f := range ms.fraudEvents {
		list = append(list, f)
	}
	return list
}

// ListAlerts returns tracked alerts notifications
func (ms *MonitoringService) ListAlerts() []*Alert {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.alerts
}

// ListRecoveryHistory returns recovery events
func (ms *MonitoringService) ListRecoveryHistory() []string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.recoveryLogs
}
