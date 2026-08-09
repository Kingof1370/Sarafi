package security

import (
	"testing"
)

func TestKeyManagerRotationAndFallback(t *testing.T) {
	legacyKey := "velyxora-apikeys-master-key-32b"
	secretPlaintext := "my-highly-confidential-api-secret"

	// 1. Setup KeyManager with legacy fallback configuration
	km, err := NewKeyManager(nil, "", legacyKey)
	if err != nil {
		t.Fatalf("Failed to instantiate KeyManager: %v", err)
	}

	// 2. Encrypt legacy way (should not have a version prefix)
	legacyCipher, err := km.Encrypt(secretPlaintext)
	if err != nil {
		t.Fatalf("Legacy encryption failed: %v", err)
	}

	if !testing_isHex(legacyCipher) {
		t.Errorf("Expected legacy cipher to be pure hex, got: %s", legacyCipher)
	}

	// 3. Decrypt legacy way
	decrypted, err := km.Decrypt(legacyCipher)
	if err != nil {
		t.Fatalf("Legacy decryption failed: %v", err)
	}

	if decrypted != secretPlaintext {
		t.Errorf("Expected decrypted plaintext to be '%s', got '%s'", secretPlaintext, decrypted)
	}

	// 4. Introduce key versioning and perform key rotation
	keysMap := map[string]string{
		"1": legacyKey,
		"2": "new-rotated-super-secure-key-32b",
	}

	// Rotate active key to version "2"
	kmRotated, err := NewKeyManager(keysMap, "2", legacyKey)
	if err != nil {
		t.Fatalf("Failed to instantiate rotated KeyManager: %v", err)
	}

	// 5. Encrypt with version 2 (should be prefixed with version)
	rotatedCipher, err := kmRotated.Encrypt(secretPlaintext)
	if err != nil {
		t.Fatalf("Rotated encryption failed: %v", err)
	}

	if !testing_hasPrefix(rotatedCipher, "v2:") {
		t.Errorf("Expected version 2 prefix, got: %s", rotatedCipher)
	}

	// 6. Decrypt rotated ciphertext
	decryptedRotated, err := kmRotated.Decrypt(rotatedCipher)
	if err != nil {
		t.Fatalf("Rotated decryption failed: %v", err)
	}

	if decryptedRotated != secretPlaintext {
		t.Errorf("Expected decrypted rotated plaintext to be '%s', got '%s'", secretPlaintext, decryptedRotated)
	}

	// 7. Critical Backwards-Compatibility Check:
	// Rotated KeyManager (knowing versions) must STILL be able to decrypt the legacy ciphertext!
	decryptedLegacyOnRotated, err := kmRotated.Decrypt(legacyCipher)
	if err != nil {
		t.Fatalf("Failed to decrypt legacy ciphertext on rotated KeyManager: %v", err)
	}

	if decryptedLegacyOnRotated != secretPlaintext {
		t.Errorf("Decrypted legacy plaintext on rotated KeyManager mismatch, got: %s", decryptedLegacyOnRotated)
	}

	// 8. Decrypting old versions explicitly
	// Suppose we decrypt version 1 ciphertext
	kmV1, _ := NewKeyManager(keysMap, "1", legacyKey)
	v1Cipher, err := kmV1.Encrypt(secretPlaintext)
	if err != nil {
		t.Fatalf("V1 encryption failed: %v", err)
	}

	if !testing_hasPrefix(v1Cipher, "v1:") {
		t.Errorf("Expected version 1 prefix, got: %s", v1Cipher)
	}

	decryptedV1, err := kmRotated.Decrypt(v1Cipher)
	if err != nil {
		t.Fatalf("Rotated KeyManager failed to decrypt V1 ciphertext: %v", err)
	}

	if decryptedV1 != secretPlaintext {
		t.Errorf("Decrypted V1 ciphertext mismatch, got: %s", decryptedV1)
	}
}

func testing_isHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func testing_hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
