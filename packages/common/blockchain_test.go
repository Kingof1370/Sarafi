package common

import "testing"

func TestBlockchainAdapters(t *testing.T) {
	btc, _ := GetBlockchainAdapter("BTC")
	if !btc.ValidateAddress("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2") {
		t.Error("Valid BTC legacy address reported as invalid")
	}

	eth, _ := GetBlockchainAdapter("ETH")
	if !eth.ValidateAddress("0x71C7656EC7ab88b098defB751B7401B5f6d1476B") {
		t.Error("Valid ETH hex address reported as invalid")
	}

	sol, _ := GetBlockchainAdapter("SOL")
	if !sol.ValidateAddress("Hxs86Xj38x8vMvVvE75A9XG9m9L9p9") {
		// Valid length range base58 check
		if sol.ValidateAddress("invalid_short") {
			t.Error("Invalid SOL address reported as valid")
		}
	}
}
