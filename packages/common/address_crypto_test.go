package common

import (
	"crypto/sha256"
	"testing"
)

func TestSecureHDWalletDerivation(t *testing.T) {
	seed := sha256.Sum256([]byte("velyxora-production-remediation-seed-key-777"))

	// 1. Generate addresses for multiple networks at index 1
	ethAddress, ethPath, err := GenerateCryptographicAddress(seed[:], "ETH", 1)
	if err != nil {
		t.Fatalf("Failed to generate ETH address: %v", err)
	}
	if ethPath != "m/44'/60'/0'/0/1" {
		t.Errorf("Unexpected ETH path: %s", ethPath)
	}

	btcAddress, btcPath, err := GenerateCryptographicAddress(seed[:], "BTC", 1)
	if err != nil {
		t.Fatalf("Failed to generate BTC address: %v", err)
	}
	if btcPath != "m/44'/0'/0'/0/1" {
		t.Errorf("Unexpected BTC path: %s", btcPath)
	}

	solAddress, solPath, err := GenerateCryptographicAddress(seed[:], "SOL", 1)
	if err != nil {
		t.Fatalf("Failed to generate SOL address: %v", err)
	}
	if solPath != "m/44'/501'/0'/0/1" {
		t.Errorf("Unexpected SOL path: %s", solPath)
	}

	// 2. Assert determinism: regenerating at same index matches exactly
	ethAddressDup, _, _ := GenerateCryptographicAddress(seed[:], "ETH", 1)
	if ethAddress != ethAddressDup {
		t.Error("Address generation is non-deterministic")
	}

	// 3. Assert index isolation: different indices produce completely different addresses
	ethAddressIndex2, _, _ := GenerateCryptographicAddress(seed[:], "ETH", 2)
	if ethAddress == ethAddressIndex2 {
		t.Error("Index collision: different indices produced the same address")
	}

	// 4. Verify Cryptographic validations
	if !ValidateCryptographicAddress(ethAddress, "ETH") {
		t.Errorf("Generated ETH address %s failed validation", ethAddress)
	}
	if !ValidateCryptographicAddress(btcAddress, "BTC") {
		t.Errorf("Generated BTC address %s failed validation", btcAddress)
	}
	if !ValidateCryptographicAddress(solAddress, "SOL") {
		t.Errorf("Generated SOL address %s failed validation", solAddress)
	}

	// 5. Test invalid validations
	if ValidateCryptographicAddress("0xInvalidEthAddressFormatThatIsTooLongAndContainsSpam", "ETH") {
		t.Error("Failed to reject corrupt ETH address format")
	}
	if ValidateCryptographicAddress("1InvalidBtcAddressFormat_Short", "BTC") {
		t.Error("Failed to reject corrupt BTC address format")
	}
}

func TestEIP55ChecksumEncoding(t *testing.T) {
	// Mixed-case EIP-55 checksum encoding assertion
	address := "0xfb6916095ca1df60bb79ce92ce3ea74c37c5d359"
	checksummed := EIP55ChecksumEncode(address)

	expected := "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359"
	if checksummed != expected {
		t.Errorf("EIP-55 checksum mismatch: expected %s, got %s", expected, checksummed)
	}
}
