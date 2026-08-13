package custody

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
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

func TestCustodyEngineCryptographicDualApproval(t *testing.T) {
	ce := NewCustodyEngine(nil)
	ctx := context.Background()

	// 1. Generate Ed25519 keypairs for Alice (Security Officer) and Bob (Compliance Officer)
	alicePub, alicePriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate Alice keypair: %v", err)
	}
	bobPub, bobPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate Bob keypair: %v", err)
	}

	// Generate a third keypair with same role to test role duplication violation
	charliePub, charliePriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate Charlie keypair: %v", err)
	}

	alicePubHex := hex.EncodeToString(alicePub)
	bobPubHex := hex.EncodeToString(bobPub)
	charliePubHex := hex.EncodeToString(charliePub)

	// 2. Register officers into the CustodyEngine
	if err := ce.AddOfficer("Alice", "security:officer", alicePubHex); err != nil {
		t.Fatalf("failed to register Alice: %v", err)
	}
	if err := ce.AddOfficer("Bob", "compliance:officer", bobPubHex); err != nil {
		t.Fatalf("failed to register Bob: %v", err)
	}
	if err := ce.AddOfficer("Charlie", "security:officer", charliePubHex); err != nil {
		t.Fatalf("failed to register Charlie: %v", err)
	}

	// 3. Create large withdrawal approval request (e.g. 5.0 BTC)
	app, err := ce.CreateApprovalRequest(ctx, "wth_sec_100", "BTC", 5.0)
	if err != nil {
		t.Fatalf("failed to create approval request: %v", err)
	}

	// Message to sign is the withdrawal ID
	msg := []byte(app.WithdrawalID)

	// Sign with Alice (security:officer)
	aliceSig := ed25519.Sign(alicePriv, msg)
	aliceSigHex := hex.EncodeToString(aliceSig)

	// Submit Alice's cryptographic approval
	app, err = ce.SubmitCryptographicApproval(ctx, app.ID, alicePubHex, aliceSigHex)
	if err != nil {
		t.Fatalf("Alice failed to submit cryptographic signature: %v", err)
	}

	if app.FirstApprover != "Alice" || app.FirstApproverRole != "security:officer" || app.Status != "PENDING_SEC_APPROVAL" {
		t.Errorf("Unexpected status/approver after first signature: %+v", app)
	}

	// Test violation: Alice tries to sign again as the second approver
	_, err = ce.SubmitCryptographicApproval(ctx, app.ID, alicePubHex, aliceSigHex)
	if err == nil {
		t.Error("Expected 4-eyes violation when Alice tries to sign twice, but got nil error")
	}

	// Test violation: Charlie (another security:officer) tries to sign, causing a role collision
	charlieSig := ed25519.Sign(charliePriv, msg)
	charlieSigHex := hex.EncodeToString(charlieSig)
	_, err = ce.SubmitCryptographicApproval(ctx, app.ID, charliePubHex, charlieSigHex)
	if err == nil {
		t.Error("Expected role collision violation when Charlie (security:officer) signs after Alice (security:officer), but got nil error")
	}

	// Sign with Bob (compliance:officer)
	bobSig := ed25519.Sign(bobPriv, msg)
	bobSigHex := hex.EncodeToString(bobSig)

	// Submit Bob's cryptographic approval
	app, err = ce.SubmitCryptographicApproval(ctx, app.ID, bobPubHex, bobSigHex)
	if err != nil {
		t.Fatalf("Bob failed to submit cryptographic signature: %v", err)
	}

	if app.SecondApprover != "Bob" || app.SecondApproverRole != "compliance:officer" || app.Status != "APPROVED" {
		t.Errorf("Expected approval request to be APPROVED, got: %+v", app)
	}
}
