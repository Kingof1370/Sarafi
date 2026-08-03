package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"velyxora/packages/custody"
)

// TestCustodyComprehensive is a detailed integration suite verifying institutional digital custody safeguards
func TestCustodyComprehensive(t *testing.T) {
	cm := custody.NewCustodyManager()
	ctx := context.Background()

	// 1. Initial State Checks
	vaults := cm.ListVaults()
	if len(vaults) != 7 {
		t.Fatalf("Expected exactly 7 institutional vaults bootstraped, got %d", len(vaults))
	}

	// 2. Fund Vaults
	err := cm.DepositToVault("v_cold", "BTC", 1000.0)
	if err != nil {
		t.Fatalf("Failed to credit cold vault: %v", err)
	}

	err = cm.DepositToVault("v_hot", "BTC", 50.0)
	if err != nil {
		t.Fatalf("Failed to credit hot vault: %v", err)
	}

	// 3. 4-Eyes Rule Enforcement (Multi-signature logic)
	// High risk vault (Cold) requires 2 admins
	tx, err := cm.CreateTransferRequest(ctx, "v_cold", "v_hot", "BTC", 100.0)
	if err != nil {
		t.Fatalf("Failed to create high-risk transfer: %v", err)
	}

	if tx.RequiredApprovals != 2 {
		t.Errorf("Expected 2 approvals required for cold vault transfer, got %d", tx.RequiredApprovals)
	}

	// Admin 1 approves
	tx, err = cm.ApproveTransferRequest(ctx, tx.ID, "admin_1")
	if err != nil {
		t.Fatalf("Admin 1 approval failed: %v", err)
	}
	if tx.Status != custody.StatusApproved {
		t.Errorf("Expected status APPROVED after 1st admin approval, got %s", tx.Status)
	}

	// Double check balances before final signature
	coldV, _ := cm.LookupVault("v_cold")
	if coldV.Balances["BTC"] != 1000.0 {
		t.Errorf("Expected cold balance to remain 1000.0 before second signature, got %f", coldV.Balances["BTC"])
	}

	// Admin 2 approves
	tx, err = cm.ApproveTransferRequest(ctx, tx.ID, "admin_2")
	if err != nil {
		t.Fatalf("Admin 2 approval failed: %v", err)
	}
	if tx.Status != custody.StatusCompleted {
		t.Errorf("Expected status COMPLETED after 2nd admin approval, got %s", tx.Status)
	}

	// Verify post-transfer balances
	coldV, _ = cm.LookupVault("v_cold")
	hotV, _ := cm.LookupVault("v_hot")
	if coldV.Balances["BTC"] != 900.0 {
		t.Errorf("Expected cold balance to be 900.0, got %f", coldV.Balances["BTC"])
	}
	if hotV.Balances["BTC"] != 150.0 {
		t.Errorf("Expected hot balance to be 150.0, got %f", hotV.Balances["BTC"])
	}
}

// TestCustodyEmergencySystem Homing and Freezing validation
func TestCustodyEmergencySystem(t *testing.T) {
	cm := custody.NewCustodyManager()
	ctx := context.Background()

	_ = cm.DepositToVault("v_hot", "BTC", 100.0)

	// Trigger emergency freeze
	frozen, err := cm.ToggleEmergencyFreeze(ctx, "FREEZE", "sys_operator_99")
	if err != nil {
		t.Fatalf("Emergency freeze failed: %v", err)
	}
	if !frozen {
		t.Fatal("System was not flagged as frozen")
	}

	// Operations should be suspended immediately
	_, err = cm.CreateTransferRequest(ctx, "v_hot", "v_warm", "BTC", 10.0)
	if err == nil {
		t.Error("Expected error when requesting transfer under active emergency freeze, got nil")
	}

	err = cm.DepositToVault("v_hot", "BTC", 10.0)
	if err == nil {
		t.Error("Expected error depositing under active emergency freeze, got nil")
	}

	// Restore operations
	frozen, err = cm.ToggleEmergencyFreeze(ctx, "UNFREEZE", "sys_operator_99")
	if err != nil {
		t.Fatalf("Emergency unfreeze failed: %v", err)
	}
	if frozen {
		t.Fatal("System was still flagged as frozen after unfreeze")
	}

	// Operations should succeed now
	err = cm.DepositToVault("v_hot", "BTC", 10.0)
	if err != nil {
		t.Fatalf("Failed to deposit after unfreezing: %v", err)
	}
}

// TestCustodyConcurrency verifies thread safety under intense operations
func TestCustodyConcurrency(t *testing.T) {
	cm := custody.NewCustodyManager()
	ctx := context.Background()

	_ = cm.DepositToVault("v_hot", "BTC", 10000.0)

	var wg sync.WaitGroup
	workers := 50
	rounds := 20

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				// Perform concurrent transfers from Hot to Warm
				tx, err := cm.CreateTransferRequest(ctx, "v_hot", "v_warm", "BTC", 1.0)
				if err == nil {
					_, _ = cm.ApproveTransferRequest(ctx, tx.ID, fmt.Sprintf("admin_w%d_r%d", workerID, r))
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify final state consistency
	vHot, _ := cm.LookupVault("v_hot")
	vWarm, _ := cm.LookupVault("v_warm")

	expectedTransferred := float64(workers * rounds)
	expectedHot := 10000.0 - expectedTransferred

	if vWarm.Balances["BTC"] != expectedTransferred {
		t.Errorf("Expected warm balance to be %f, got %f", expectedTransferred, vWarm.Balances["BTC"])
	}
	if vHot.Balances["BTC"] != expectedHot {
		t.Errorf("Expected hot balance to be %f, got %f", expectedHot, vHot.Balances["BTC"])
	}
}

// Benchmarks for performance validation
func BenchmarkVaultLookup(b *testing.B) {
	cm := custody.NewCustodyManager()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cm.LookupVault("v_hot")
	}
}

func BenchmarkCustodyTransferLifecycle(b *testing.B) {
	cm := custody.NewCustodyManager()
	_ = cm.DepositToVault("v_hot", "BTC", 100000000.0)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, _ := cm.CreateTransferRequest(ctx, "v_hot", "v_warm", "BTC", 1.0)
		_, _ = cm.ApproveTransferRequest(ctx, tx.ID, "admin_runner")
	}
}
