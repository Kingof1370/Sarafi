package custody

import (
	"context"
	"testing"
)

func TestVaultCustodyLifecycleAndTransfer(t *testing.T) {
	cm := NewCustodyManager()
	ctx := context.Background()

	// Initial bootstrap verification
	vaults := cm.ListVaults()
	if len(vaults) != 7 {
		t.Fatalf("Expected 7 defaulted vaults bootstrapped, got %d", len(vaults))
	}

	// Direct Deposit to hot vault
	err := cm.DepositToVault("v_hot", "BTC", 5.0)
	if err != nil {
		t.Fatalf("Failed to deposit to v_hot: %v", err)
	}

	// Verify balance
	vHot, _ := cm.LookupVault("v_hot")
	if vHot.Balances["BTC"] != 5.0 {
		t.Errorf("Expected balance 5.0, got %f", vHot.Balances["BTC"])
	}

	// Initiate transfer from Hot Vault to Warm Vault (Hot does not require 2 admins)
	tx, err := cm.CreateTransferRequest(ctx, "v_hot", "v_warm", "BTC", 2.0)
	if err != nil {
		t.Fatalf("Failed to create transfer request: %v", err)
	}

	if tx.RequiredApprovals != 1 {
		t.Errorf("Expected 1 required approval, got %d", tx.RequiredApprovals)
	}

	// Approve
	tx, err = cm.ApproveTransferRequest(ctx, tx.ID, "admin_alice")
	if err != nil {
		t.Fatalf("Failed to approve transfer: %v", err)
	}

	if tx.Status != StatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", tx.Status)
	}

	// Check post balances
	vHot, _ = cm.LookupVault("v_hot")
	vWarm, _ := cm.LookupVault("v_warm")

	if vHot.Balances["BTC"] != 3.0 {
		t.Errorf("Expected hot balance to decrease to 3.0, got %f", vHot.Balances["BTC"])
	}
	if vWarm.Balances["BTC"] != 2.0 {
		t.Errorf("Expected warm balance to increase to 2.0, got %f", vWarm.Balances["BTC"])
	}
}

func TestEmergencyLocksAndFreezing(t *testing.T) {
	cm := NewCustodyManager()
	ctx := context.Background()

	_ = cm.DepositToVault("v_hot", "BTC", 10.0)

	// Trigger emergency freeze
	isFrozen, err := cm.ToggleEmergencyFreeze(ctx, "FREEZE", "security_operator_01")
	if err != nil {
		t.Fatalf("Failed to trigger freeze: %v", err)
	}
	if !isFrozen {
		t.Fatal("Expected isFrozen flag to be true")
	}

	// Transfer during freeze should be blocked
	_, err = cm.CreateTransferRequest(ctx, "v_hot", "v_warm", "BTC", 1.0)
	if err == nil {
		t.Error("Expected error when requesting transfer under active emergency freeze, got nil")
	}

	// Deposit during freeze should be blocked
	err = cm.DepositToVault("v_hot", "BTC", 1.0)
	if err == nil {
		t.Error("Expected error depositing under active emergency freeze, got nil")
	}

	// Unfreeze
	isFrozen, err = cm.ToggleEmergencyFreeze(ctx, "UNFREEZE", "security_operator_01")
	if err != nil {
		t.Fatalf("Failed to trigger unfreeze: %v", err)
	}
	if isFrozen {
		t.Fatal("Expected isFrozen flag to be false")
	}

	// Transfers and deposits should work now
	err = cm.DepositToVault("v_hot", "BTC", 2.0)
	if err != nil {
		t.Fatalf("Failed to deposit after unfreezing: %v", err)
	}
}
