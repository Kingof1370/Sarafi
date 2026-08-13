package security

import (
	"context"
	"crypto/rand"
	"io"
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

func TestEnvelopeEncryptionAndMemoryZeroing(t *testing.T) {
	km, err := NewKeyManager(nil, "", "velyxora-apikeys-master-key-32b")
	if err != nil {
		t.Fatalf("Failed to instantiate KeyManager: %v", err)
	}

	ctx := context.Background()

	// 1. Generate some mock private key bytes
	rawKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, rawKey); err != nil {
		t.Fatalf("Failed to generate raw key: %v", err)
	}

	// Keep a copy of the rawKey because EncryptPrivateKey is going to zero out the slice passed to it
	rawKeyCopy := make([]byte, len(rawKey))
	copy(rawKeyCopy, rawKey)

	// 2. Perform Envelope Encryption
	encryptedHex, err := km.EncryptPrivateKey(ctx, rawKey)
	if err != nil {
		t.Fatalf("Envelope encryption failed: %v", err)
	}

	// Verify that rawKey was successfully zeroed out in memory!
	for i, b := range rawKey {
		if b != 0 {
			t.Errorf("Expected rawKey slice to be zeroed out, but index %d is %d", i, b)
		}
	}

	// 3. Decrypt the envelope encrypted key
	decryptedKey, err := km.DecryptPrivateKey(ctx, encryptedHex)
	if err != nil {
		t.Fatalf("Envelope decryption failed: %v", err)
	}
	defer ZeroMemory(decryptedKey)

	// Verify decrypted content matches original raw key copy
	if len(decryptedKey) != len(rawKeyCopy) {
		t.Fatalf("Decrypted key length mismatch: got %d, expected %d", len(decryptedKey), len(rawKeyCopy))
	}

	for i := range decryptedKey {
		if decryptedKey[i] != rawKeyCopy[i] {
			t.Errorf("Decrypted key content mismatch at index %d", i)
		}
	}
}

func TestKMSClientsMockFallback(t *testing.T) {
	ctx := context.Background()

	// Test AWS KMS wrapper with mock fallback
	awsKMS := &AWSKMSClient{Client: nil, KeyID: "arn:aws:kms:us-east-1:123456789012:key/test"}
	plaintext := []byte("highly-confidential-blockchain-private-key-data")
	ciphertext, err := awsKMS.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("AWS KMS client encrypt failed: %v", err)
	}

	decrypted, err := awsKMS.Decrypt(ctx, ciphertext)
	if err != nil {
		t.Fatalf("AWS KMS client decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("AWS KMS client decrypt mismatch, expected %s, got %s", string(plaintext), string(decrypted))
	}

	// Test Vault Transit client wrapper with mock fallback
	vaultKMS := &VaultKMSClient{Client: nil, KeyName: "velyxora-transit-key"}
	ciphertextVault, err := vaultKMS.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("Vault KMS client encrypt failed: %v", err)
	}

	decryptedVault, err := vaultKMS.Decrypt(ctx, ciphertextVault)
	if err != nil {
		t.Fatalf("Vault KMS client decrypt failed: %v", err)
	}

	if string(decryptedVault) != string(plaintext) {
		t.Errorf("Vault KMS client decrypt mismatch, expected %s, got %s", string(plaintext), string(decryptedVault))
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
