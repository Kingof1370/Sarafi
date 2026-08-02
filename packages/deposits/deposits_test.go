package deposits

import (
	"context"
	"testing"
	"time"
)

func TestDepositIngressAndConfirmations(t *testing.T) {
	de := NewDepositEngine()
	ctx := context.Background()

	// Ingress transaction
	tx := &BlockchainTx{
		TxHash:      "0x123abc456def7890",
		Network:     "Ethereum",
		Asset:       "ETH",
		Amount:      5.5,
		Sender:      "0xSender",
		Receiver:    "0xReceiver",
		BlockNumber: 1000,
		Timestamp:   time.Now(),
	}

	rec, err := de.ValidateAndDetectIngress(ctx, tx, "usr_123")
	if err != nil {
		t.Fatalf("ValidateAndDetectIngress failed: %v", err)
	}

	if rec.Status != StatusPending {
		t.Errorf("Expected PENDING, got %s", rec.Status)
	}

	if rec.RequiredConfirmations != 12 {
		t.Errorf("Expected 12 required confirmations for ETH, got %d", rec.RequiredConfirmations)
	}

	// Double deposit violation (Replay protection)
	_, errDouble := de.ValidateAndDetectIngress(ctx, tx, "usr_123")
	if errDouble == nil {
		t.Error("Expected error when double-depositing same tx hash, got nil")
	}

	// Update confirmations below threshold
	rec, err = de.IncrementConfirmations(ctx, tx.TxHash, 1005) // 1005 - 1000 + 1 = 6 confirmations
	if err != nil {
		t.Fatalf("IncrementConfirmations failed: %v", err)
	}

	if rec.Confirmations != 6 {
		t.Errorf("Expected 6 confirmations, got %d", rec.Confirmations)
	}

	if rec.Status != StatusConfirmed {
		t.Errorf("Expected status CONFIRMED at 6 confs, got %s", rec.Status)
	}

	// Finalize completed deposit
	rec, err = de.IncrementConfirmations(ctx, tx.TxHash, 1012) // 1012 - 1000 + 1 = 13 confirmations (>= 12)
	if err != nil {
		t.Fatalf("IncrementConfirmations failed: %v", err)
	}

	if rec.Status != StatusCompleted {
		t.Errorf("Expected status COMPLETED at 13 confs, got %s", rec.Status)
	}
}

func TestChainReorganizationRollback(t *testing.T) {
	de := NewDepositEngine()
	ctx := context.Background()

	tx := &BlockchainTx{
		TxHash:      "0x789fork",
		Network:     "Bitcoin",
		Asset:       "BTC",
		Amount:      1.25,
		Sender:      "Sender",
		Receiver:    "Receiver",
		BlockNumber: 500,
		Timestamp:   time.Now(),
	}

	_, _ = de.ValidateAndDetectIngress(ctx, tx, "usr_456")
	_, _ = de.IncrementConfirmations(ctx, tx.TxHash, 505) // 6 confirmations -> StatusCompleted for BTC (req: 6)

	rec, _ := de.LookupByTxHash(tx.TxHash)
	if rec.Status != StatusCompleted {
		t.Fatalf("Expected COMPLETED, got %s", rec.Status)
	}

	// Trigger Chain Reorg at block 500
	affected, err := de.HandleChainReorganization(ctx, 500)
	if err != nil {
		t.Fatalf("HandleChainReorganization failed: %v", err)
	}

	if len(affected) != 1 {
		t.Errorf("Expected 1 affected record, got %d", len(affected))
	}

	if rec.Status != StatusPending {
		t.Errorf("Expected status rolled back to PENDING, got %s", rec.Status)
	}

	if rec.Confirmations != 0 {
		t.Errorf("Expected confirmations reset to 0, got %d", rec.Confirmations)
	}
}

func TestOrphanTransactions(t *testing.T) {
	de := NewDepositEngine()
	ctx := context.Background()

	tx := &BlockchainTx{
		TxHash:      "0xorphan",
		Network:     "Solana",
		Asset:       "SOL",
		Amount:      50.0,
		Sender:      "Sender",
		Receiver:    "Receiver",
		BlockNumber: 8000,
		Timestamp:   time.Now(),
	}

	_, _ = de.ValidateAndDetectIngress(ctx, tx, "usr_abc")

	rec, err := de.HandleOrphanTransaction(ctx, tx.TxHash)
	if err != nil {
		t.Fatalf("HandleOrphanTransaction failed: %v", err)
	}

	if rec.Status != StatusFailed {
		t.Errorf("Expected status FAILED for orphan transaction, got %s", rec.Status)
	}
}
