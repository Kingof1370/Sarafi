package main

import (
	"context"
	"testing"
	"time"

	"velyxora/packages/logger"
	"velyxora/packages/wallet"
	"velyxora/packages/deposits"
	"velyxora/packages/withdrawals"
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
	pws := wallet.NewPersistentWalletService(nil, nil, log)
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
	err = pws.GetWalletMgr().SetLimit("w_test_op_01", 1.0, 5.0)
	if err != nil {
		t.Fatalf("SetLimit failed: %v", err)
	}

	// Success transfer
	ok, err := pws.GetWalletMgr().InitiateTransfer("tx_pws_01", "w_test_op_01", "w_test_hot_01", "BTC", 0.5, "usr_test_01")
	if err != nil {
		t.Fatalf("InitiateTransfer failed: %v", err)
	}
	if !ok {
		t.Error("Transfer failed")
	}

	// Violation single limit transfer
	_, err = pws.GetWalletMgr().InitiateTransfer("tx_pws_02", "w_test_op_01", "w_test_hot_01", "BTC", 2.0, "usr_test_01")
	if err == nil {
		t.Error("Expected transaction single limit error, got nil")
	}
}

func TestPersistentWalletServiceDepositFlow(t *testing.T) {
	log := logger.NewLogger(logger.Config{Level: "INFO", Format: "JSON", ServiceName: "test-pws-deposit"})
	pws := wallet.NewPersistentWalletService(nil, nil, log)
	ctx := context.Background()

	_ = pws.Bootstrap(ctx)
	w, _ := pws.ProvisionWallet(ctx, "w_deposit_01", "usr_deposit", wallet.TypeHot)

	tx := &deposits.BlockchainTx{
		TxHash:      "0xdeposit_hash_99",
		Network:     "Ethereum",
		Asset:       "ETH",
		Amount:      10.5,
		Sender:      "0xSender",
		Receiver:    "0xReceiver",
		BlockNumber: 15000,
		Timestamp:   time.Now(),
	}

	// 1. Process raw blockchain transaction (Deposit Detected)
	rec, err := pws.ProcessBlockchainTransaction(ctx, tx, "usr_deposit", "w_deposit_01")
	if err != nil {
		t.Fatalf("ProcessBlockchainTransaction failed: %v", err)
	}

	if rec.Status != deposits.StatusPending {
		t.Errorf("Expected PENDING, got %s", rec.Status)
	}

	// 2. Increment confirmations under threshold (Deposit Confirmed)
	rec, err = pws.UpdateDepositConfirmation(ctx, tx.TxHash, 15005, "w_deposit_01") // 6 confs
	if err != nil {
		t.Fatalf("UpdateDepositConfirmation failed: %v", err)
	}

	if rec.Status != deposits.StatusConfirmed {
		t.Errorf("Expected status CONFIRMED at 6 confs, got %s", rec.Status)
	}

	// User shouldn't be credited yet
	if _, exists := w.Balances["ETH"]; exists {
		t.Error("User was credited ETH before required confirmations reached")
	}

	// 3. Finalize confirmations (Deposit Completed)
	rec, err = pws.UpdateDepositConfirmation(ctx, tx.TxHash, 15012, "w_deposit_01") // 13 confs (req: 12)
	if err != nil {
		t.Fatalf("UpdateDepositConfirmation failed: %v", err)
	}

	if rec.Status != deposits.StatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", rec.Status)
	}

	// User should now be credited!
	bal, exists := w.Balances["ETH"]
	if !exists {
		t.Fatal("Expected credited ETH balance segment, but none found")
	}

	if bal.Available != 10.5 {
		t.Errorf("Expected available balance 10.5, got %f", bal.Available)
	}
}

func TestPersistentWalletServiceWithdrawalFlow(t *testing.T) {
	log := logger.NewLogger(logger.Config{Level: "INFO", Format: "JSON", ServiceName: "test-pws-withdrawal"})
	pws := wallet.NewPersistentWalletService(nil, nil, log)
	ctx := context.Background()

	_ = pws.Bootstrap(ctx)
	w, _ := pws.ProvisionWallet(ctx, "w_withdraw_01", "usr_withdraw", wallet.TypeHot)
	_ = pws.UpdateBalance(ctx, "w_withdraw_01", "BTC", 5.0, 0, 0, 0) // Onboard 5.0 BTC available

	// Add recipient to address book
	pws.GetWithdrawalEngine().AddAddressToBook("usr_withdraw", "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", "Bitcoin", "My Safe Wallet", true)

	// 1. Process Request
	req, err := pws.ProcessWithdrawalRequest(ctx, "usr_withdraw", "w_withdraw_01", "BTC", 0.25, 0.0005, "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2")
	if err != nil {
		t.Fatalf("ProcessWithdrawalRequest failed: %v", err)
	}

	if req.Status != withdrawals.StatusRequested {
		t.Errorf("Expected status REQUESTED, got %s", req.Status)
	}

	// 2. Administrative Approvals
	req, err = pws.ProcessWithdrawalApproval(ctx, req.ID, "admin_user_99")
	if err != nil {
		t.Fatalf("ProcessWithdrawalApproval failed: %v", err)
	}

	if req.Status != withdrawals.StatusApproved {
		t.Errorf("Expected status APPROVED, got %s", req.Status)
	}

	// 3. Broadcast Hex Payload
	req, err = pws.ProcessWithdrawalBroadcast(ctx, req.ID, "010203040506070809")
	if err != nil {
		t.Fatalf("ProcessWithdrawalBroadcast failed: %v", err)
	}

	if req.Status != withdrawals.StatusBroadcast || req.TxHash == "" {
		t.Errorf("Expected status BROADCAST and valid tx hash, got %s", req.Status)
	}

	// 4. Finalize & Deduct User Balance
	req, err = pws.ProcessWithdrawalFinalize(ctx, req.ID, "w_withdraw_01")
	if err != nil {
		t.Fatalf("ProcessWithdrawalFinalize failed: %v", err)
	}

	if req.Status != withdrawals.StatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", req.Status)
	}

	// Available balance should be deducted by: amount (0.25) + fee (0.0005) = 0.2505
	bal := w.Balances["BTC"]
	expectedBalance := 5.0 - 0.2505
	if bal.Available != expectedBalance {
		t.Errorf("Expected available balance %f, got %f", expectedBalance, bal.Available)
	}
}
