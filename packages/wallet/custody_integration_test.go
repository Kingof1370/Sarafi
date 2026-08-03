package wallet

import (
	"context"
	"testing"
)

func TestPersistentWalletServiceCustodyIntegration(t *testing.T) {
	// Initialize PersistentWalletService without database (passing nil db and nil producer)
	pws := NewPersistentWalletService(nil, nil, nil)

	ctx := context.Background()
	err := pws.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}

	cm := pws.GetCustodyManager()
	if cm == nil {
		t.Fatal("Expected custody manager to be initialized, got nil")
	}

	// Verify standard institutional vaults are registered
	vaults := cm.ListVaults()
	if len(vaults) < 7 {
		t.Errorf("Expected at least 7 institutional vaults, got %d", len(vaults))
	}

	// Fetch specific vault
	vHot, err := cm.LookupVault("v_hot")
	if err != nil {
		t.Fatalf("Lookup v_hot failed: %v", err)
	}
	if vHot.ID != "v_hot" || vHot.Name != "Hot Exchange Vault" {
		t.Errorf("v_hot vault metadata mismatch")
	}

	// Perform a mock deposit in a vault
	err = cm.DepositToVault("v_hot", "BTC", 10.5)
	if err != nil {
		t.Fatalf("DepositToVault failed: %v", err)
	}

	vHot, _ = cm.LookupVault("v_hot")
	if vHot.Balances["BTC"] != 10.5 {
		t.Errorf("Expected BTC balance 10.5, got %f", vHot.Balances["BTC"])
	}

	// Initiate a 4-eyes transfer request from Hot Vault to Warm Vault
	req, err := cm.CreateTransferRequest(ctx, "v_hot", "v_warm", "BTC", 5.0)
	if err != nil {
		t.Fatalf("CreateTransferRequest failed: %v", err)
	}

	if req.RequiredApprovals != 1 {
		t.Errorf("Expected 1 required approval for Hot vault, got %d", req.RequiredApprovals)
	}

	// Execute transfer by approving it
	updatedReq, err := cm.ApproveTransferRequest(ctx, req.ID, "admin_alice")
	if err != nil {
		t.Fatalf("ApproveTransferRequest failed: %v", err)
	}

	if updatedReq.Status != "COMPLETED" {
		t.Errorf("Expected transfer status COMPLETED, got %s", updatedReq.Status)
	}

	// Verify target balances after transfer completes
	vWarm, _ := cm.LookupVault("v_warm")
	if vWarm.Balances["BTC"] != 5.0 {
		t.Errorf("Expected Warm Vault to have 5.0 BTC, got %f", vWarm.Balances["BTC"])
	}

	vHot, _ = cm.LookupVault("v_hot")
	if vHot.Balances["BTC"] != 5.5 {
		t.Errorf("Expected Hot Vault to have 5.5 BTC, got %f", vHot.Balances["BTC"])
	}

	// Trigger emergency freeze
	isFrozen, err := cm.ToggleEmergencyFreeze(ctx, "FREEZE", "admin_bob")
	if err != nil {
		t.Fatalf("ToggleEmergencyFreeze failed: %v", err)
	}
	if !isFrozen {
		t.Error("Expected system to be frozen")
	}

	// Create transfer request under frozen state should fail
	_, err = cm.CreateTransferRequest(ctx, "v_hot", "v_warm", "BTC", 1.0)
	if err == nil {
		t.Error("Expected error when requesting transfer under active emergency freeze, got nil")
	}

	// Unfreeze system
	isFrozen, err = cm.ToggleEmergencyFreeze(ctx, "UNFREEZE", "admin_bob")
	if err != nil {
		t.Fatalf("ToggleEmergencyFreeze unfreeze failed: %v", err)
	}
	if isFrozen {
		t.Error("Expected system to be unfrozen")
	}
}
