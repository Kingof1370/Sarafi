package custody

import (
	"context"
	"testing"
)

func TestCustodyEngineDualApprovalAndFreeze(t *testing.T) {
	ce := NewCustodyEngine(nil)

	// Test 1: Global Emergency Freeze Toggle
	ce.SetGlobalFreeze(true)
	if !ce.IsFrozen() {
		t.Error("Expected engine to report globally frozen")
	}

	_, err := ce.ValidateWithdrawal("BTC", 0.5)
	if err == nil || err.Error() != "emergency freeze is currently ACTIVE. All withdrawal transfers are blocked" {
		t.Errorf("Expected emergency freeze validation error, got: %v", err)
	}

	// Disable freeze
	ce.SetGlobalFreeze(false)
	if ce.IsFrozen() {
		t.Error("Expected engine to report unfrozen")
	}

	// Test 2: Policy Checks (dual-approval requirements)
	needsApproval, err := ce.ValidateWithdrawal("BTC", 0.5)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if needsApproval {
		t.Error("Expected BTC 0.5 withdrawal to NOT require dual approval")
	}

	needsApproval, err = ce.ValidateWithdrawal("BTC", 2.5)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !needsApproval {
		t.Error("Expected BTC 2.5 withdrawal to require dual approval")
	}

	_, err = ce.ValidateWithdrawal("BTC", 25.0)
	if err == nil {
		t.Error("Expected maximum single withdrawal limit error, got nil")
	}

	// Test 3: Multi-Signature Approval Logic & 4-Eyes Principle
	app, err := ce.CreateApprovalRequest(context.Background(), "wth_1", "BTC", 2.5)
	if err != nil {
		t.Fatalf("Failed to create approval request: %v", err)
	}

	if app.Status != "PENDING" {
		t.Errorf("Expected status PENDING, got %s", app.Status)
	}

	// First approval signature
	app, err = ce.SubmitApproval(context.Background(), app.ID, "admin_alice")
	if err != nil {
		t.Fatalf("Alice failed to approve: %v", err)
	}
	if app.FirstApprover != "admin_alice" || app.Status != "PENDING" {
		t.Errorf("Expected Alice to be recorded as first approver, got: %+v", app)
	}

	// Second approval signature by same person (must fail)
	_, err = ce.SubmitApproval(context.Background(), app.ID, "admin_alice")
	if err == nil || err.Error() != "multi-sig validation violation: second approver must be different from first approver (4-eyes principle)" {
		t.Errorf("Expected 4-eyes security violation, got: %v", err)
	}

	// Second approval signature by different person (must pass)
	app, err = ce.SubmitApproval(context.Background(), app.ID, "admin_bob")
	if err != nil {
		t.Fatalf("Bob failed to approve: %v", err)
	}
	if app.SecondApprover != "admin_bob" || app.Status != "APPROVED" {
		t.Errorf("Expected approval request to be completed and APPROVED, got: %+v", app)
	}
}
