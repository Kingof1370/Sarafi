package common

import (
	"os"
	"testing"
)

func TestBlockchainAdaptersModesAndValidations(t *testing.T) {
	// Set environment to SIMULATION for tests
	os.Setenv("BLOCKCHAIN_MODE", "SIMULATION")
	defer os.Unsetenv("BLOCKCHAIN_MODE")

	btc, err := GetBlockchainAdapter("BTC")
	if err != nil {
		t.Fatalf("Failed to resolve BTC adapter: %v", err)
	}

	if btc.GetMode() != ModeSimulation {
		t.Errorf("Expected SIMULATION mode, got %s", btc.GetMode())
	}

	// Address validation tests
	if !btc.ValidateAddress("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2") {
		t.Error("Valid BTC legacy address reported as invalid")
	}
	if btc.ValidateAddress("invalid_address") {
		t.Error("Invalid BTC address reported as valid")
	}

	eth, err := GetBlockchainAdapter("ETH")
	if err != nil {
		t.Fatalf("Failed to resolve ETH adapter: %v", err)
	}
	if !eth.ValidateAddress("0x71C7656EC7ab88b098defB751B7401B5f6d1476B") {
		t.Error("Valid ETH hex address reported as invalid")
	}
	if eth.ValidateAddress("0xInvalidHexLength") {
		t.Error("Invalid ETH address reported as valid")
	}

	sol, err := GetBlockchainAdapter("SOL")
	if err != nil {
		t.Fatalf("Failed to resolve SOL adapter: %v", err)
	}
	if !sol.ValidateAddress("Hxs86Xj38x8vMvVvE75A9XG9m9L9p9aBcDe") {
		t.Error("Valid SOL address reported as invalid")
	}
	if sol.ValidateAddress("invalid-short-sol") {
		t.Error("Invalid SOL address reported as valid")
	}

	// Simulated balance lookup tests
	simBTC := GetSimulator("BTC")
	simBTC.SetBalance("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", "BTC", 5.25)
	bal, err := btc.GetNativeBalance("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2")
	if err != nil {
		t.Fatalf("Failed to get simulated BTC balance: %v", err)
	}
	if bal != 5.25 {
		t.Errorf("Expected balance 5.25, got %f", bal)
	}

	// Simulated broadcast and confirmation tracking tests
	txHash, err := btc.BroadcastTransaction("raw_test_tx_payload")
	if err != nil {
		t.Fatalf("Failed to broadcast simulated transaction: %v", err)
	}
	if txHash == "" {
		t.Fatal("Broadcast transaction returned empty hash")
	}

	conf, err := btc.GetConfirmations(txHash)
	if err != nil {
		t.Fatalf("Failed to get confirmations: %v", err)
	}
	if conf != 0 {
		t.Errorf("Expected 0 confirmations before block is mined, got %d", conf)
	}

	// Mine simulated block
	simBTC.MineBlock()

	confAfter, err := btc.GetConfirmations(txHash)
	if err != nil {
		t.Fatalf("Failed to get confirmations: %v", err)
	}
	if confAfter != 1 {
		t.Errorf("Expected 1 confirmation after block is mined, got %d", confAfter)
	}
}

func TestBlockchainAdaptersProductionChecks(t *testing.T) {
	os.Setenv("BLOCKCHAIN_MODE", "SIMULATION")
	os.Setenv("ENV", "production")
	defer os.Unsetenv("BLOCKCHAIN_MODE")
	defer os.Unsetenv("ENV")

	_, err := GetBlockchainAdapter("BTC")
	if err == nil {
		t.Error("Expected error when attempting to run SIMULATION mode in production environment, got nil")
	}
}

func TestBlockchainAdaptersLiveModeConfigs(t *testing.T) {
	os.Setenv("BLOCKCHAIN_MODE", "LIVE")
	defer os.Unsetenv("BLOCKCHAIN_MODE")

	btc, err := GetBlockchainAdapter("BTC")
	if err != nil {
		t.Fatalf("Failed to get BTC adapter: %v", err)
	}

	_, err = btc.GetNativeBalance("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2")
	if err == nil || err.Error() != "BTC real node RPC URL is not configured" {
		t.Errorf("Expected RPC URL configuration error, got: %v", err)
	}
}
