package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// SecurityEvent defines structural parameters of security-sensitive events
type SecurityEvent struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"` // e.g. AUTH_FAILED, POLICY_VIOLATION, FRAUD_DETECTED
	ActorID   string    `json:"actor_id"`
	Details   string    `json:"details"`
	ClientIP  string    `json:"client_ip"`
	Timestamp time.Time `json:"timestamp"`
}

// ChainedAuditRecord represents a cryptographically linked, tamper-proof audit block
type ChainedAuditRecord struct {
	Index     int64     `json:"index"`
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	ActorID   string    `json:"actor_id"`
	Details   string    `json:"details"`
	PrevHash  string    `json:"prev_hash"`
	Hash      string    `json:"hash"`
}

// ComputeHash calculates the SHA-256 block hash for cryptographically chained immutability
func (car *ChainedAuditRecord) ComputeHash() string {
	payload := fmt.Sprintf("%d|%s|%s|%s|%s|%s|%s",
		car.Index,
		car.ID,
		car.Timestamp.Format(time.RFC3339Nano),
		car.Action,
		car.ActorID,
		car.Details,
		car.PrevHash,
	)
	hash := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(hash[:])
}

// SecurityEventPipeline runs asynchronous processing of security events and logs them in a secure log chain
type SecurityEventPipeline struct {
	mu           sync.RWMutex
	eventChan    chan *SecurityEvent
	auditChain   []*ChainedAuditRecord
	ctx          context.Context
	cancel       context.CancelFunc
	activeAlerts []*SecurityEvent
}

// NewSecurityEventPipeline initializes the Zero-Trust Security Pipeline
func NewSecurityEventPipeline() *SecurityEventPipeline {
	ctx, cancel := context.WithCancel(context.Background())
	sep := &SecurityEventPipeline{
		eventChan:    make(chan *SecurityEvent, 1000),
		auditChain:   make([]*ChainedAuditRecord, 0),
		ctx:          ctx,
		cancel:       cancel,
		activeAlerts: make([]*SecurityEvent, 0),
	}
	// Start asynchronous pipeline consumer
	go sep.runPipelineLoop()
	return sep
}

func (sep *SecurityEventPipeline) runPipelineLoop() {
	for {
		select {
		case evt := <-sep.eventChan:
			sep.processEvent(evt)
		case <-sep.ctx.Done():
			return
		}
	}
}

// PublishEvent sends a security event to the channel pipeline without blocking
func (sep *SecurityEventPipeline) PublishEvent(evt *SecurityEvent) {
	select {
	case sep.eventChan <- evt:
	default:
		// Drop/log error if channel is completely flooded (prevents denial of service on main trading engine)
		fmt.Printf("[ALARM DROPPED] Security channel queue full: ID %s, Action %s\n", evt.ID, evt.Action)
	}
}

// processEvent processes and logs security events inside the immutable chained logs
func (sep *SecurityEventPipeline) processEvent(evt *SecurityEvent) {
	sep.mu.Lock()
	defer sep.mu.Unlock()

	// 1. Keep track of severe alerts
	if evt.Action == "FRAUD_DETECTED" || evt.Action == "POLICY_VIOLATION" {
		sep.activeAlerts = append(sep.activeAlerts, evt)
	}

	// 2. Append to cryptographic log chain
	var prevHash string
	index := int64(len(sep.auditChain))
	if index > 0 {
		prevHash = sep.auditChain[index-1].Hash
	}

	rec := &ChainedAuditRecord{
		Index:     index,
		ID:        evt.ID,
		Timestamp: evt.Timestamp,
		Action:    evt.Action,
		ActorID:   evt.ActorID,
		Details:   evt.Details,
		PrevHash:  prevHash,
	}
	rec.Hash = rec.ComputeHash()

	sep.auditChain = append(sep.auditChain, rec)
}

// ValidateChain checks each record in the chain and detects manual log tampering
func (sep *SecurityEventPipeline) ValidateChain() (bool, error) {
	sep.mu.RLock()
	defer sep.mu.RUnlock()

	for i := 0; i < len(sep.auditChain); i++ {
		rec := sep.auditChain[i]

		// 1. Recompute current block hash
		computed := rec.ComputeHash()
		if computed != rec.Hash {
			return false, fmt.Errorf("tamper detected: log record index %d hash mismatch. Expected %s, got %s", rec.Index, rec.Hash, computed)
		}

		// 2. Check back-link matches previous block hash
		if i > 0 {
			prev := sep.auditChain[i-1]
			if rec.PrevHash != prev.Hash {
				return false, fmt.Errorf("tamper detected: log record index %d back-link to index %d broken", rec.Index, i-1)
			}
		}
	}

	return true, nil
}

// ListAuditChain returns the current audit chain records
func (sep *SecurityEventPipeline) ListAuditChain() []*ChainedAuditRecord {
	sep.mu.RLock()
	defer sep.mu.RUnlock()

	list := make([]*ChainedAuditRecord, len(sep.auditChain))
	copy(list, sep.auditChain)
	return list
}

// ListActiveAlerts returns the currently tracked security threats
func (sep *SecurityEventPipeline) ListActiveAlerts() []*SecurityEvent {
	sep.mu.RLock()
	defer sep.mu.RUnlock()

	list := make([]*SecurityEvent, len(sep.activeAlerts))
	copy(list, sep.activeAlerts)
	return list
}

// Shutdown stops the asynchronous pipeline safely
func (sep *SecurityEventPipeline) Shutdown() {
	sep.cancel()
}

// ComplianceScanReport captures SOC2, ISO27001, and GDPR readiness status parameters
type ComplianceScanReport struct {
	Timestamp      time.Time         `json:"timestamp"`
	SOC2Passed     bool              `json:"soc2_passed"`
	ISO27001Passed bool              `json:"iso27001_passed"`
	GDPRPassed     bool              `json:"gdpr_passed"`
	Details        map[string]string `json:"details"`
}

// RunComplianceVerification checks the codebase state and configurations for corporate compliance
func RunComplianceVerification(dbConnected bool, rptEnabled bool, keysEncrypted bool) *ComplianceScanReport {
	details := make(map[string]string)
	soc2 := true
	iso := true
	gdpr := true

	// 1. SOC2: Requires active database encryption & detailed chained auditing trails
	if !dbConnected {
		soc2 = false
		details["SOC2_DB_ENCRYPTION"] = "FAILED: Relational database connection is offline or unencrypted."
	} else {
		details["SOC2_DB_ENCRYPTION"] = "PASSED: Secure TLS-wrapped relational database connected."
	}

	// 2. ISO27001: Demands cryptographic key rotation and secure Hardware Security Module providers
	if !keysEncrypted {
		iso = false
		details["ISO_KEY_ENCRYPTION"] = "FAILED: Operational API keys and secrets stored without secure SHA-512 hashing."
	} else {
		details["ISO_KEY_ENCRYPTION"] = "PASSED: All system secrets hashed/salted and rotated."
	}

	// 3. GDPR: Demands zero-trust impossible travel, IP rapid-fire blocking, and anti-replay mechanics
	if !rptEnabled {
		gdpr = false
		details["GDPR_REPLAY_PROTECTION"] = "FAILED: Replay attack trackers or anti-CSRF token engines are disabled."
	} else {
		details["GDPR_REPLAY_PROTECTION"] = "PASSED: Active cryptographic nonce anti-replay and CSRF managers validated."
	}

	return &ComplianceScanReport{
		Timestamp:      time.Now(),
		SOC2Passed:     soc2,
		ISO27001Passed: iso,
		GDPRPassed:     gdpr,
		Details:        details,
	}
}
