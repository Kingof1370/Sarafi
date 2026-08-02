package address

import (
	"testing"
	"time"

	"velyxora/packages/blockchain"
)

func TestAddressRegistryAndOwnership(t *testing.T) {
	ar := NewAddressRegistry()
	walletRegistry := blockchain.NewAdapterRegistry()

	ethAdapter, err := walletRegistry.Get("Ethereum")
	if err != nil {
		t.Fatalf("Failed to fetch Ethereum adapter: %v", err)
	}

	privKey := []byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16,
		0x17, 0x18, 0x19, 0x20, 0x21, 0x22, 0x23, 0x24,
		0x25, 0x26, 0x27, 0x28, 0x29, 0x30, 0x31, 0x32,
	}

	pubKey := ethAdapter.DerivePublicKey(privKey)
	addressString, err := ethAdapter.GenerateAddress(privKey)
	if err != nil {
		t.Fatalf("GenerateAddress failed: %v", err)
	}

	rec := &AddressRecord{
		UserID:         "usr_01",
		Network:        "Ethereum",
		Address:        addressString,
		PublicKey:      pubKey,
		DerivationPath: "m/44'/60'/0'/0/0",
		Status:         StatusAllocated,
		CreatedAt:      time.Now(),
	}

	meta := &AddressMetadata{
		Address:          addressString,
		KeyIndex:         0,
		TransactionCount: 0,
	}

	err = ar.RegisterAddress(rec, meta)
	if err != nil {
		t.Fatalf("RegisterAddress failed: %v", err)
	}

	// Retrieve validations
	lookRecord, err := ar.LookupAddress("usr_01", "Ethereum")
	if err != nil {
		t.Errorf("LookupAddress failed: %v", err)
	} else if lookRecord.Address != addressString {
		t.Errorf("Expected address %s, got %s", addressString, lookRecord.Address)
	}

	lookByAdd, err := ar.LookupByAddress(addressString)
	if err != nil {
		t.Errorf("LookupByAddress failed: %v", err)
	} else if lookByAdd.UserID != "usr_01" {
		t.Errorf("Expected UserID usr_01, got %s", lookByAdd.UserID)
	}

	// Verify cryptographic ownership signature
	challenge := []byte("challenge_verification_99")
	sig, err := ethAdapter.SignTransaction(privKey, challenge)
	if err != nil {
		t.Fatalf("SignTransaction failed: %v", err)
	}

	ok, err := ar.VerifyOwnership(ethAdapter, addressString, challenge, sig)
	if err != nil {
		t.Fatalf("VerifyOwnership failed with error: %v", err)
	}
	if !ok {
		t.Errorf("VerifyOwnership returned false, expected true signature match")
	}

	// Check status updated to StatusVerified
	if rec.Status != StatusVerified {
		t.Errorf("Expected status to be updated to VERIFIED, got %s", rec.Status)
	}
}
