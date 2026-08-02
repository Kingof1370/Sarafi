package blockchain

import (
	"crypto/rand"
	"testing"
)

func TestHDWalletAndAdapters(t *testing.T) {
	mnemonic, err := GenerateMnemonic()
	if err != nil {
		t.Fatalf("GenerateMnemonic failed: %v", err)
	}

	wallet, err := NewHDWalletFromMnemonic(mnemonic)
	if err != nil {
		t.Fatalf("NewHDWalletFromMnemonic failed: %v", err)
	}

	masterKey, err := DeriveMasterKey(wallet.Seed)
	if err != nil {
		t.Fatalf("DeriveMasterKey failed: %v", err)
	}

	registry := NewAdapterRegistry()

	networks := []string{"Bitcoin", "Ethereum", "BNB Smart Chain", "Polygon", "Avalanche", "Solana", "Tron", "Litecoin"}

	for _, net := range networks {
		adapter, err := registry.Get(net)
		if err != nil {
			t.Fatalf("Failed to fetch adapter for %s: %v", net, err)
		}

		childKey, err := masterKey.DerivePath(adapter.DerivationPath())
		if err != nil {
			t.Fatalf("DerivePath failed for %s using path %s: %v", net, adapter.DerivationPath(), err)
		}

		address, err := adapter.GenerateAddress(childKey.Key)
		if err != nil {
			t.Fatalf("GenerateAddress failed for %s: %v", net, err)
		}

		if address == "" {
			t.Errorf("GenerateAddress returned empty address for %s", net)
		}

		if !adapter.ValidateAddress(address) {
			t.Errorf("ValidateAddress returned false for generated address %s on %s", address, net)
		}

		// Signature and verification test
		txData := []byte("v_tx_payload_test_string_99")
		sig, err := adapter.SignTransaction(childKey.Key, txData)
		if err != nil {
			t.Fatalf("SignTransaction failed on %s: %v", net, err)
		}

		pubKey := adapter.DerivePublicKey(childKey.Key)
		ok, err := adapter.VerifySignature(pubKey, txData, sig)
		if err != nil {
			t.Fatalf("VerifySignature error on %s: %v", net, err)
		}
		if !ok {
			t.Errorf("VerifySignature failed for %s", net)
		}
	}
}

func TestInvalidAddressValidation(t *testing.T) {
	registry := NewAdapterRegistry()
	adapters := []string{"Bitcoin", "Ethereum", "Solana", "Tron", "Litecoin"}

	for _, name := range adapters {
		adapter, err := registry.Get(name)
		if err != nil {
			t.Fatalf("Failed to get adapter %s: %v", name, err)
		}
		if adapter.ValidateAddress("invalid-address-format") {
			t.Errorf("ValidateAddress accepted invalid address on %s", name)
		}
	}
}

func TestRandomSignatures(t *testing.T) {
	registry := NewAdapterRegistry()
	adapter, _ := registry.Get("Ethereum")

	priv := make([]byte, 32)
	_, _ = rand.Read(priv)

	txData := []byte("some payload data")
	sig, err := adapter.SignTransaction(priv, txData)
	if err != nil {
		t.Fatalf("SignTransaction error: %v", err)
	}

	pub := adapter.DerivePublicKey(priv)
	ok, _ := adapter.VerifySignature(pub, txData, sig)
	if !ok {
		t.Error("Verification of fresh signature failed")
	}
}
