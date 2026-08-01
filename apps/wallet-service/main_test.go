package main

import (
	"context"
	"testing"
)

func TestBalanceEngineDoubleEntryReconciliation(t *testing.T) {
	be := NewBalanceEngine(nil)
	ctx := context.Background()

	// Settle debit and credit safely
	err := be.ProcessDoubleEntry(ctx, "tx_123", "usr_debit", "usr_credit", "USDT", 250.0, "Secure double entry ledger reconciliation")
	if err != nil {
		t.Fatalf("Reconciliation process failed: %v", err)
	}

	debitBal := be.balances["usr_debit_USDT"]
	creditBal := be.balances["usr_credit_USDT"]

	if debitBal.Available != 750.0 {
		t.Errorf("Expected debit user to have 750.0 left, got %f", debitBal.Available)
	}

	if creditBal.Available != 1250.0 {
		t.Errorf("Expected credit user to gain 250.0, got %f", creditBal.Available)
	}

	// Enforce negative limits
	err = be.ProcessDoubleEntry(ctx, "tx_124", "usr_debit", "usr_credit", "USDT", 900.0, "Excessive transfer")
	if err == nil {
		t.Error("Reconciliation process should block excessive debit operations exceeding total available limit")
	}
}
