package ledger_common

import (
	"testing"
	"time"
)

func TestLedgerAuditTrail(t *testing.T) {
	t1 := time.Now()

	r1 := &WalletAuditRecord{
		ID:          "rec_01",
		UserID:      "usr_abc",
		WalletID:    "wal_01",
		Asset:       "BTC",
		Action:      ActionWalletCreated,
		Amount:      0.0,
		PrevBalance: 0.0,
		NewBalance:  0.0,
		Message:     "Wallet provisioned successfully",
		PrevHash:    "",
		Timestamp:   t1,
	}
	r1.Hash = r1.ComputeHash()

	ok, err := r1.ValidateIntegrity(nil)
	if err != nil {
		t.Fatalf("Validation of Genesis block failed: %v", err)
	}
	if !ok {
		t.Error("Genesis block is invalid")
	}

	// Chain child record
	t2 := t1.Add(time.Second)
	r2 := &WalletAuditRecord{
		ID:          "rec_02",
		UserID:      "usr_abc",
		WalletID:    "wal_01",
		Asset:       "BTC",
		Action:      ActionDepositReceived,
		Amount:      1.5,
		PrevBalance: 0.0,
		NewBalance:  1.5,
		Message:     "Bitcoin deposit of 1.5 BTC confirmed",
		PrevHash:    r1.Hash,
		Timestamp:   t2,
	}
	r2.Hash = r2.ComputeHash()

	ok, err = r2.ValidateIntegrity(r1)
	if err != nil {
		t.Fatalf("Validation of chained block failed: %v", err)
	}
	if !ok {
		t.Error("Chained block validation returned false")
	}

	// Corrupt child record to test validation failure
	r2.Amount = 2.5
	ok, err = r2.ValidateIntegrity(r1)
	if err == nil {
		t.Error("Expected error from validation of corrupted record, got nil")
	}
	if ok {
		t.Error("Corrupted block validated successfully, expected failure")
	}
}
