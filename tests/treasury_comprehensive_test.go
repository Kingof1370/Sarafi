package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"velyxora/packages/treasury"
)

// TestTreasuryComprehensive covers institutional transfers, risk and dual approvals workflows, and automatic reconciliation
func TestTreasuryComprehensive(t *testing.T) {
	tm := treasury.NewTreasuryManager()
	ctx := context.Background()

	// 1. Initial State checks
	transfers := tm.ListTransfers()
	if len(transfers) != 0 {
		t.Fatalf("Expected zero initial internal transfers, got %d", len(transfers))
	}

	// 2. Fund Pools
	err := tm.DepositToPool(treasury.PoolTreasury, "BTC", 5000.0)
	if err != nil {
		t.Fatalf("Failed to credit treasury pool: %v", err)
	}

	err = tm.DepositToPool(treasury.PoolReserve, "BTC", 2000.0)
	if err != nil {
		t.Fatalf("Failed to credit reserve pool: %v", err)
	}

	// 3. Request Standard Transfer (Treasury -> Hot)
	tx, err := tm.CreateInternalTransfer(ctx, treasury.PoolTreasury, treasury.PoolHot, "BTC", 500.0)
	if err != nil {
		t.Fatalf("Failed to create transfer: %v", err)
	}

	if tx.RequiredApprovals != 1 {
		t.Errorf("Expected 1 required approval, got %d", tx.RequiredApprovals)
	}

	// Admin 1 approves (completes the transfer)
	tx, err = tm.ApproveInternalTransfer(ctx, tx.ID, "admin_alice")
	if err != nil {
		t.Fatalf("Failed to register first approval: %v", err)
	}

	if tx.Status != treasury.StatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", tx.Status)
	}

	// Verify post-balances
	tBal := tm.GetPoolBalance(treasury.PoolTreasury, "BTC")
	hBal := tm.GetPoolBalance(treasury.PoolHot, "BTC")

	if tBal != 4500.0 {
		t.Errorf("Expected Treasury balance 4500.0, got %f", tBal)
	}
	if hBal != 500.0 {
		t.Errorf("Expected Hot balance 500.0, got %f", hBal)
	}

	// 4. Request High Risk / Multi-Party Transfer (Reserve -> Hot)
	txHigh, err := tm.CreateInternalTransfer(ctx, treasury.PoolReserve, treasury.PoolHot, "BTC", 100.0)
	if err != nil {
		t.Fatalf("Failed to create high risk transfer: %v", err)
	}

	if txHigh.RequiredApprovals != 2 {
		t.Errorf("Expected 2 required approvals for Reserve pool, got %d", txHigh.RequiredApprovals)
	}

	// First admin approval
	txHigh, err = tm.ApproveInternalTransfer(ctx, txHigh.ID, "admin_alice")
	if err != nil {
		t.Fatalf("First approval failed: %v", err)
	}
	if txHigh.Status != treasury.StatusApproved {
		t.Errorf("Expected status APPROVED, got %s", txHigh.Status)
	}

	// Second admin approval (finalizes transfer)
	txHigh, err = tm.ApproveInternalTransfer(ctx, txHigh.ID, "admin_bob")
	if err != nil {
		t.Fatalf("Second approval failed: %v", err)
	}
	if txHigh.Status != treasury.StatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", txHigh.Status)
	}

	// Verify post-balances
	rBal := tm.GetPoolBalance(treasury.PoolReserve, "BTC")
	hBal = tm.GetPoolBalance(treasury.PoolHot, "BTC")

	if rBal != 1900.0 {
		t.Errorf("Expected Reserve balance 1900.0, got %f", rBal)
	}
	if hBal != 600.0 {
		t.Errorf("Expected Hot balance 600.0, got %f", hBal)
	}

	// 5. Automatic Reconciliation Execution
	report, err := tm.RunAutomaticReconciliation(ctx)
	if err != nil {
		t.Fatalf("Reconciliation execution failed: %v", err)
	}

	if !report.IsConsistent {
		t.Errorf("Expected reconciliation report to be consistent, got inconsistent: %s", report.Details)
	}
}

// TestTreasuryConcurrency verifies thread-safety under extremely heavy concurrent transfers
func TestTreasuryConcurrency(t *testing.T) {
	tm := treasury.NewTreasuryManager()
	ctx := context.Background()

	_ = tm.DepositToPool(treasury.PoolTreasury, "USDT", 100000.0)

	var wg sync.WaitGroup
	workers := 100
	rounds := 50

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				tx, err := tm.CreateInternalTransfer(ctx, treasury.PoolTreasury, treasury.PoolOperational, "USDT", 1.0)
				if err == nil {
					_, _ = tm.ApproveInternalTransfer(ctx, tx.ID, fmt.Sprintf("admin_w%d_r%d", workerID, r))
				}
			}
		}(i)
	}

	wg.Wait()

	tBal := tm.GetPoolBalance(treasury.PoolTreasury, "USDT")
	oBal := tm.GetPoolBalance(treasury.PoolOperational, "USDT")

	expectedMoved := float64(workers * rounds)
	expectedTreasury := 100000.0 - expectedMoved

	if oBal != expectedMoved {
		t.Errorf("Expected operational balance to be %f, got %f", expectedMoved, oBal)
	}
	if tBal != expectedTreasury {
		t.Errorf("Expected treasury balance to be %f, got %f", expectedTreasury, tBal)
	}
}

// Benchmarks for performance validation
func BenchmarkTreasuryLookup(b *testing.B) {
	tm := treasury.NewTreasuryManager()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tm.GetPoolBalance(treasury.PoolTreasury, "BTC")
	}
}

func BenchmarkTreasuryTransferLifecycle(b *testing.B) {
	tm := treasury.NewTreasuryManager()
	_ = tm.DepositToPool(treasury.PoolTreasury, "BTC", 100000000.0)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, _ := tm.CreateInternalTransfer(ctx, treasury.PoolTreasury, treasury.PoolHot, "BTC", 1.0)
		_, _ = tm.ApproveInternalTransfer(ctx, tx.ID, "admin_alice")
	}
}

func BenchmarkAutomaticReconciliation(b *testing.B) {
	tm := treasury.NewTreasuryManager()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tm.RunAutomaticReconciliation(ctx)
	}
}
