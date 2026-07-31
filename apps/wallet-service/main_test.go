package main

import (
	"testing"
	"velyxora/packages/types"
)

func TestMockBalanceState(t *testing.T) {
	mockBalances := make(map[string]*types.Balance)
	bal := getOrInitMockBalance(mockBalances, "usr1", "BTC")

	if bal.Free != 10.0 {
		t.Errorf("Expected initialized mock free balance to be 10.0, got %f", bal.Free)
	}

	bal.Free += 2.5
	if getOrInitMockBalance(mockBalances, "usr1", "BTC").Free != 12.5 {
		t.Errorf("Balance adjustments were not reflected correctly")
	}
}
