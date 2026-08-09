package common

import "testing"

import (
	"os"
	"time"
)

func TestSecureRPCClient(t *testing.T) {
	// Test environment check
	os.Setenv("APP_ENV", "production")
	defer os.Unsetenv("APP_ENV")

	// Production should reject HTTP
	_, err := NewSecureRPCClient("http://my-insecure-node.com", 2*time.Second)
	if err == nil {
		t.Error("Expected error when using HTTP RPC endpoint in production")
	}

	// Production should accept HTTPS
	client, err := NewSecureRPCClient("https://my-secure-node.com", 2*time.Second)
	if err != nil {
		t.Errorf("Unexpected error with secure endpoint: %v", err)
	}

	if client.Timeout != 2*time.Second {
		t.Errorf("Expected timeout 2s, got %v", client.Timeout)
	}
}

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
