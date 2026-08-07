package security

import (
	"testing"
	"time"
)

func TestSecurityEventPipelineAndCompliance(t *testing.T) {
	sep := NewSecurityEventPipeline()
	defer sep.Shutdown()

	// 1. Publish some events to the pipeline
	evt1 := &SecurityEvent{
		ID:        "evt_1",
		Action:    "LOGIN_SUCCESS",
		ActorID:   "usr_admin",
		Details:   "Admin authenticated successfully",
		ClientIP:  "127.0.0.1",
		Timestamp: time.Now(),
	}

	evt2 := &SecurityEvent{
		ID:        "evt_2",
		Action:    "POLICY_VIOLATION",
		ActorID:   "usr_malicious",
		Details:   "Unauthorized write attempt on keys",
		ClientIP:  "203.0.113.12",
		Timestamp: time.Now(),
	}

	sep.PublishEvent(evt1)
	sep.PublishEvent(evt2)

	// Wait briefly for asynchronous processing loop
	time.Sleep(50 * time.Millisecond)

	// Verify events are logged in the cryptographic chain
	chain := sep.ListAuditChain()
	if len(chain) != 2 {
		t.Fatalf("expected 2 records in secure audit chain, got %d", len(chain))
	}

	// 2. Validate chain integrity
	valid, err := sep.ValidateChain()
	if !valid || err != nil {
		t.Fatalf("expected cryptographic log chain to be perfectly valid: %v", err)
	}

	// 3. Simulate Tampering
	chain[0].Details = "Tampered Action Log Details" // modify memory record
	valid, err = sep.ValidateChain()
	if !valid && err != nil {
		t.Logf("PASSED: successfully detected manual log tampering in secure chain: %v", err)
	} else {
		t.Error("expected ValidateChain to catch record tampering")
	}

	// Restore and make sure it checks out again
	chain[0].Details = "Admin authenticated successfully"
	chain[0].Hash = chain[0].ComputeHash() // re-compute hash
	valid, err = sep.ValidateChain()
	if !valid || err != nil {
		t.Fatalf("expected restored chain to be valid again: %v", err)
	}

	// 4. Test Compliance Verification Scans
	rep := RunComplianceVerification(true, true, true)
	if !rep.SOC2Passed || !rep.ISO27001Passed || !rep.GDPRPassed {
		t.Error("expected perfect compliance checks given optimal parameters")
	}

	repFailed := RunComplianceVerification(false, false, false)
	if repFailed.SOC2Passed || repFailed.ISO27001Passed || repFailed.GDPRPassed {
		t.Error("expected failing compliance checks given offline parameters")
	}
}
