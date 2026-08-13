package main

import (
	"testing"
	"velyxora/packages/security"
)

func TestDynamicTOTPEnrollmentCryptographicProvisioning(t *testing.T) {
	// Call security GenerateTOTPSecret helper
	secret1, backupCodes1, err := security.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("Failed to generate TOTP secret: %v", err)
	}

	secret2, backupCodes2, err := security.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("Failed to generate second TOTP secret: %v", err)
	}

	// Assert that dynamic secrets are high-entropy, unique, and non-colliding
	if secret1 == secret2 {
		t.Error("Cryptographic key collision: generated identical secrets")
	}

	if len(secret1) != 32 {
		t.Errorf("Expected 32-character Base32 encoded secret, got size %d", len(secret1))
	}

	// Verify backup codes format and uniqueness
	if len(backupCodes1) != 3 {
		t.Errorf("Expected 3 unique backup codes, got %d", len(backupCodes1))
	}

	if backupCodes1[0] == backupCodes2[0] {
		t.Error("Backup codes collision detected")
	}
}
