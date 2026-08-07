package tests

import (
	"fmt"
	"testing"
	"time"

	"velyxora/packages/security"
)

func TestComprehensiveZeroTrustAndSecurityHardening(t *testing.T) {
	// 1. Initialize policy and key managers
	pe := security.NewPolicyEngine()
	km := security.NewKeyManager(nil)
	sep := security.NewSecurityEventPipeline()
	defer sep.Shutdown()

	ctxNormal := &security.ContextAttributes{
		ClientIP:          "127.0.0.1",
		GeoLocation:       "US",
		DeviceFingerprint: "secure_chrome_v124",
		RequestTime:       time.Now(),
		RiskScore:         0.1,
		SessionAgeSeconds: 15,
	}

	// 2. Register API Key credentials
	meta, rawSecret, err := km.RegisterAPIKey("usr_compliance", []string{"audit_read"}, "auditor_pass", 24*time.Hour)
	if err != nil {
		t.Fatalf("RegisterAPIKey failed: %v", err)
	}

	ok, err := km.ValidateAPIKey(meta.ID, rawSecret)
	if !ok || err != nil {
		t.Fatalf("ValidateAPIKey failed on valid API key: %v", err)
	}

	// 3. Evaluate dynamic ABAC rules on Auditor action
	allowed, err := pe.Evaluate(security.RoleComplianceAuditor, security.ResourceAudit, security.ActionRead, ctxNormal)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !allowed {
		t.Error("expected Compliance Auditor to be authorized for Audit Resource Read operations")
	}

	// 4. Log successful audit action in the cryptographic immutable chain
	sep.PublishEvent(&security.SecurityEvent{
		ID:        fmt.Sprintf("evt_aud_%d", time.Now().UnixNano()),
		Action:    "COMPLIANCE_AUDIT_EXEC",
		ActorID:   "usr_compliance",
		Details:   "Compliance officer retrieved active policy rules.",
		ClientIP:  "127.0.0.1",
		Timestamp: time.Now(),
	})

	// Wait briefly for asynchronous processing loop
	time.Sleep(50 * time.Millisecond)

	chain := sep.ListAuditChain()
	if len(chain) == 0 {
		t.Fatal("expected logged compliance audit event to be present in secure chain")
	}

	valid, err := sep.ValidateChain()
	if !valid || err != nil {
		t.Fatalf("expected cryptographic audit chain to be perfectly valid: %v", err)
	}

	// 5. Test Compliance scanner verification
	report := security.RunComplianceVerification(true, true, true)
	if !report.SOC2Passed || !report.ISO27001Passed || !report.GDPRPassed {
		t.Error("expected 100% compliance checks to pass")
	}
}
