package common

import (
	"testing"
)

func TestBlockchainSimulator(t *testing.T) {
	sim := GetSimulator("ETH")

	addrFrom := "0xAddressFrom"
	addrTo := "0xAddressTo"

	sim.SetBalance(addrFrom, "ETH", 10.0)
	if sim.GetBalance(addrFrom, "ETH") != 10.0 {
		t.Errorf("Expected balance 10.0, got %f", sim.GetBalance(addrFrom, "ETH"))
	}

	txHash, err := sim.SubmitTransaction(addrFrom, addrTo, "ETH", 4.0)
	if err != nil {
		t.Fatalf("Failed to submit transaction: %v", err)
	}

	// Balance from should be debited immediately in pool
	if sim.GetBalance(addrFrom, "ETH") != 6.0 {
		t.Errorf("Expected balance 6.0 after submit, got %f", sim.GetBalance(addrFrom, "ETH"))
	}
	// Balance to should not be credited until block mined
	if sim.GetBalance(addrTo, "ETH") != 0.0 {
		t.Errorf("Expected balance 0.0 for receiver before mine, got %f", sim.GetBalance(addrTo, "ETH"))
	}

	// Mine block
	sim.MineBlock()

	// Receiver should now be credited
	if sim.GetBalance(addrTo, "ETH") != 4.0 {
		t.Errorf("Expected balance 4.0 for receiver after mine, got %f", sim.GetBalance(addrTo, "ETH"))
	}

	tx := sim.GetTransactionByHash(txHash)
	if tx == nil || tx.Status != "SUCCESS" {
		t.Errorf("Expected transaction status to be SUCCESS, got %v", tx)
	}

	// Test reorg
	sim.Reorg(1)
	// After reorg, block is rolled back. Receiver should be back to 0.0, sender back to 10.0 (or pending)
	if sim.GetBalance(addrTo, "ETH") != 0.0 {
		t.Errorf("Expected receiver balance to revert to 0.0 after reorg, got %f", sim.GetBalance(addrTo, "ETH"))
	}
}
