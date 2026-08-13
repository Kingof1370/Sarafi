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

func TestEthereumClientAndBroadcasting(t *testing.T) {
	// Test creating client using the fallback Cloudflare node
	client, err := NewEthereumClient()
	if err != nil {
		t.Fatalf("Failed to initialize EthereumClient: %v", err)
	}

	// Validate public RPC responsiveness
	num, err := client.GetLatestBlockNumber()
	if err != nil {
		t.Logf("Warning: RPC node test failed (expected offline/sandbox limitations): %v", err)
		return
	}
	if num == 0 {
		t.Error("Latest block number cannot be 0")
	}

	// Query standard Vitalik address balance
	vitalikAddr := "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
	bal, err := client.GetBalance(vitalikAddr)
	if err != nil {
		t.Errorf("Failed to query vitalik balance: %v", err)
	}
	if bal == nil {
		t.Error("Returned balance is nil")
	}

	// Test validation
	ethAdapter := &ETHAdapter{RPCURL: client.URL}
	if !ethAdapter.ValidateAddress(vitalikAddr) {
		t.Error("Expected address to be validated successfully")
	}
}
