package main

import (
	"context"
	"testing"

	"velyxora/packages/logger"
	"velyxora/packages/wallet"
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

func TestPersistentWalletServiceAllOperations(t *testing.T) {
	log := logger.NewLogger(logger.Config{Level: "INFO", Format: "JSON", ServiceName: "test-pws"})
	pws := NewPersistentWalletService(nil, nil, log)
	ctx := context.Background()

	err := pws.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap failed: %v", err)
	}

	// 1. Provision a hot wallet and operational wallet
	wHot, err := pws.ProvisionWallet(ctx, "w_test_hot_01", "usr_test_01", wallet.TypeHot)
	if err != nil {
		t.Fatalf("ProvisionWallet failed: %v", err)
	}

	wOp, err := pws.ProvisionWallet(ctx, "w_test_op_01", "usr_test_01", wallet.TypeOperational)
	if err != nil {
		t.Fatalf("ProvisionWallet failed: %v", err)
	}
	if wOp.ID != "w_test_op_01" {
		t.Errorf("Expected w_test_op_01, got %s", wOp.ID)
	}

	// 2. Allocate addresses
	addrHot, err := pws.AllocateAddress(ctx, "usr_test_01", "w_test_hot_01", "Ethereum")
	if err != nil {
		t.Fatalf("AllocateAddress failed: %v", err)
	}
	if addrHot.Address == "" {
		t.Error("Allocated empty hot address")
	}

	// 3. Update Balance
	err = pws.UpdateBalance(ctx, "w_test_hot_01", "BTC", 5.0, 0, 0, 0)
	if err != nil {
		t.Fatalf("UpdateBalance failed: %v", err)
	}

	if wHot.Balances["BTC"].Available != 5.0 {
		t.Errorf("Expected 5.0 BTC available, got %f", wHot.Balances["BTC"].Available)
	}

	// 4. Try transfers within limits
	_ = pws.UpdateBalance(ctx, "w_test_op_01", "BTC", 10.0, 0, 0, 0)
	err = pws.walletMgr.SetLimit("w_test_op_01", 1.0, 5.0)
	if err != nil {
		t.Fatalf("SetLimit failed: %v", err)
	}

	// Success transfer
	ok, err := pws.walletMgr.InitiateTransfer("tx_pws_01", "w_test_op_01", "w_test_hot_01", "BTC", 0.5, "usr_test_01")
	if err != nil {
		t.Fatalf("InitiateTransfer failed: %v", err)
	}
	if !ok {
		t.Error("Transfer failed")
	}

	// Violation single limit transfer
	_, err = pws.walletMgr.InitiateTransfer("tx_pws_02", "w_test_op_01", "w_test_hot_01", "BTC", 2.0, "usr_test_01")
	if err == nil {
		t.Error("Expected transaction single limit error, got nil")
	}
}
