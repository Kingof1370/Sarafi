package treasury

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestTreasuryLifecycleAndReconciliation(t *testing.T) {
	tm := NewTreasuryManager()
	ctx := context.Background()

	// Direct Deposit
	err := tm.DepositToPool(PoolTreasury, "BTC", 10.0)
	if err != nil {
		t.Fatalf("Failed to credit pool: %v", err)
	}

	bal := tm.GetPoolBalance(PoolTreasury, "BTC")
	if bal != 10.0 {
		t.Errorf("Expected balance 10.0, got %f", bal)
	}

	// Internal Transfer (Treasury -> Hot)
	tx, err := tm.CreateInternalTransfer(ctx, PoolTreasury, PoolHot, "BTC", 3.0)
	if err != nil {
		t.Fatalf("Failed to create internal transfer request: %v", err)
	}

	if tx.RequiredApprovals != 1 {
		t.Errorf("Expected 1 required approval, got %d", tx.RequiredApprovals)
	}

	// Approve
	tx, err = tm.ApproveInternalTransfer(ctx, tx.ID, "admin_alice")
	if err != nil {
		t.Fatalf("Failed to register approval: %v", err)
	}

	if tx.Status != StatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", tx.Status)
	}

	// Verify post balances
	tBal := tm.GetPoolBalance(PoolTreasury, "BTC")
	hBal := tm.GetPoolBalance(PoolHot, "BTC")

	if tBal != 7.0 {
		t.Errorf("Expected Treasury balance 7.0, got %f", tBal)
	}
	if hBal != 3.0 {
		t.Errorf("Expected Hot balance 3.0, got %f", hBal)
	}

	// Reconciliation
	report, err := tm.RunAutomaticReconciliation(ctx)
	if err != nil {
		t.Fatalf("Reconciliation execution failed: %v", err)
	}

	if !report.IsConsistent {
		t.Errorf("Expected reconciliation report to be consistent, got inconsistent")
	}
}

func TestTreasuryHighRiskDualApprovals(t *testing.T) {
	tm := NewTreasuryManager()
	ctx := context.Background()

	_ = tm.DepositToPool(PoolReserve, "ETH", 100.0)

	// Create request from Reserve (High security, requires 2 approvals)
	tx, err := tm.CreateInternalTransfer(ctx, PoolReserve, PoolHot, "ETH", 20.0)
	if err != nil {
		t.Fatalf("Failed to request reserve transfer: %v", err)
	}

	if tx.RequiredApprovals != 2 {
		t.Errorf("Expected 2 required approvals for Reserve pool, got %d", tx.RequiredApprovals)
	}

	// First admin approval
	tx, err = tm.ApproveInternalTransfer(ctx, tx.ID, "admin_alice")
	if err != nil {
		t.Fatalf("First approval failed: %v", err)
	}
	if tx.Status != StatusApproved {
		t.Errorf("Expected status APPROVED, got %s", tx.Status)
	}

	// Second admin approval (completes execution)
	tx, err = tm.ApproveInternalTransfer(ctx, tx.ID, "admin_bob")
	if err != nil {
		t.Fatalf("Second approval failed: %v", err)
	}
	if tx.Status != StatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", tx.Status)
	}

	// Check final balances
	rBal := tm.GetPoolBalance(PoolReserve, "ETH")
	hBal := tm.GetPoolBalance(PoolHot, "ETH")

	if rBal != 80.0 {
		t.Errorf("Expected Reserve balance 80.0, got %f", rBal)
	}
	if hBal != 20.0 {
		t.Errorf("Expected Hot balance 20.0, got %f", hBal)
	}
}

func TestTreasuryConcurrency(t *testing.T) {
	tm := NewTreasuryManager()
	ctx := context.Background()

	_ = tm.DepositToPool(PoolTreasury, "USDT", 10000.0)

	var wg sync.WaitGroup
	workers := 50
	rounds := 10

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				tx, err := tm.CreateInternalTransfer(ctx, PoolTreasury, PoolOperational, "USDT", 1.0)
				if err == nil {
					_, _ = tm.ApproveInternalTransfer(ctx, tx.ID, fmt.Sprintf("admin_w%d_r%d", workerID, r))
				}
			}
		}(i)
	}

	wg.Wait()

	tBal := tm.GetPoolBalance(PoolTreasury, "USDT")
	oBal := tm.GetPoolBalance(PoolOperational, "USDT")

	expectedMoved := float64(workers * rounds)
	expectedTreasury := 10000.0 - expectedMoved

	if oBal != expectedMoved {
		t.Errorf("Expected operational balance to be %f, got %f", expectedMoved, oBal)
	}
	if tBal != expectedTreasury {
		t.Errorf("Expected treasury balance to be %f, got %f", expectedTreasury, tBal)
	}
}
