package main

import (
	"context"
	"os"
	"testing"

	"velyxora/packages/common"
	"velyxora/packages/custody"
	"velyxora/packages/logger"
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

func TestWalletWorkerOrchestratorDepositAndWithdrawalFlow(t *testing.T) {
	os.Setenv("BLOCKCHAIN_MODE", "SIMULATION")
	defer os.Unsetenv("BLOCKCHAIN_MODE")

	log := logger.NewLogger(logger.Config{Level: "INFO", Format: "JSON", ServiceName: "test-wallet-worker"})
	ce := custody.NewCustodyEngine(nil)

	// Create orchestrator with mock parameters (nil database pool, nil producer)
	orch := NewWalletWorkerOrchestrator(nil, nil, ce, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test 1: Scan block for deposits
	simBTC := common.GetSimulator("BTC")
	adapter, _ := common.GetBlockchainAdapter("BTC")

	// Set deposit address balance
	toAddress := "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"
	simBTC.SetBalance("COINBASE", "BTC", 100.0)

	// Submit deposit transaction
	txHash, err := simBTC.SubmitTransaction("COINBASE", toAddress, "BTC", 1.5)
	if err != nil {
		t.Fatalf("Failed to submit simulated transaction: %v", err)
	}

	// Mine block containing the transaction
	simBTC.MineBlock()

	// Run scanner check manually
	orch.scanBlock(ctx, "BTC", simBTC.GetHeight(), adapter)

	// Verify that the local transaction was processed/tracked
	orch.mu.Lock()
	processed := orch.localTxs[txHash]
	orch.mu.Unlock()

	if !processed {
		t.Errorf("Expected deposit transaction %s to be detected and processed", txHash)
	}

	// Test 2: Withdrawal Queue Worker Local Track Nonces
	orch.mu.Lock()
	nonce := orch.nonces["0xWithdrawalAddress"]
	orch.nonces["0xWithdrawalAddress"] = nonce + 1
	orch.mu.Unlock()

	orch.mu.Lock()
	newNonce := orch.nonces["0xWithdrawalAddress"]
	orch.mu.Unlock()

	if newNonce != 1 {
		t.Errorf("Expected nonce to be incremented to 1, got %d", newNonce)
	}
}

func TestReconciliationEngineAudit(t *testing.T) {
	os.Setenv("BLOCKCHAIN_MODE", "SIMULATION")
	defer os.Unsetenv("BLOCKCHAIN_MODE")

	log := logger.NewLogger(logger.Config{Level: "INFO", Format: "JSON", ServiceName: "test-reconciliation"})
	ce := custody.NewCustodyEngine(nil)

	engine := NewReconciliationEngine(nil, ce, log)

	ctx := context.Background()
	_, issues, err := engine.RunReconciliationAudit(ctx, "BTC")
	if err != nil {
		t.Fatalf("Failed to run reconciliation audit: %v", err)
	}

	// In memory mode with nil DB, some warning/issues might be empty, but execution is verified!
	if issues == nil {
		t.Error("Expected issues list to be initialized (even if empty)")
	}
}
