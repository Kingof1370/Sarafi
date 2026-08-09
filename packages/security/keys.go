package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// KeyManager manages cryptographic keys, versions, and provides backward-compatible key rotation support.
type KeyManager struct {
	keys             map[string][]byte
	activeVersion    string
	legacyDefaultKey []byte
}

// NewKeyManager initializes a KeyManager with versioned keys and a designated active version.
func NewKeyManager(keys map[string]string, activeVersion string, legacyDefault string) (*KeyManager, error) {
	parsedKeys := make(map[string][]byte)
	for ver, keyStr := range keys {
		// Ensure each key is safe (force 32 bytes for AES-256)
		padded := make([]byte, 32)
		copy(padded, []byte(keyStr))
		parsedKeys[ver] = padded
	}

	var legacyKeyBytes []byte
	if legacyDefault != "" {
		legacyKeyBytes = make([]byte, 32)
		copy(legacyKeyBytes, []byte(legacyDefault))
	}

	if activeVersion != "" {
		if _, ok := parsedKeys[activeVersion]; !ok {
			return nil, fmt.Errorf("active version %s not found in key map", activeVersion)
		}
	}

	return &KeyManager{
		keys:             parsedKeys,
		activeVersion:    activeVersion,
		legacyDefaultKey: legacyKeyBytes,
	}, nil
}

// Encrypt encrypts plain text using the current active key version, prefixing the output with version metadata.
func (km *KeyManager) Encrypt(plaintext string) (string, error) {
	if km.activeVersion == "" || len(km.keys) == 0 {
		// Fallback to legacy encryption if no versioned keys defined
		if len(km.legacyDefaultKey) == 0 {
			return "", errors.New("no active cryptographic key defined")
		}
		ciphertext, err := aesGCMEncrypt(km.legacyDefaultKey, []byte(plaintext))
		if err != nil {
			return "", err
		}
		return hex.EncodeToString(ciphertext), nil
	}

	key := km.keys[km.activeVersion]
	ciphertext, err := aesGCMEncrypt(key, []byte(plaintext))
	if err != nil {
		return "", err
	}

	// Format output with version prefix: "v[version]:[hex_ciphertext]"
	formatted := fmt.Sprintf("v%s:%s", km.activeVersion, hex.EncodeToString(ciphertext))
	return formatted, nil
}

// Decrypt decrypts ciphertext, parsing version metadata if present, falling back gracefully to legacy defaults.
func (km *KeyManager) Decrypt(encryptedStr string) (string, error) {
	if strings.HasPrefix(encryptedStr, "v") && strings.Contains(encryptedStr, ":") {
		parts := strings.SplitN(encryptedStr, ":", 2)
		versionWithV := parts[0]
		ciphertextHex := parts[1]

		version := strings.TrimPrefix(versionWithV, "v")
		key, ok := km.keys[version]
		if !ok {
			return "", fmt.Errorf("cryptographic error: key version '%s' not configured or unavailable", version)
		}

		ciphertext, err := hex.DecodeString(ciphertextHex)
		if err != nil {
			return "", fmt.Errorf("failed to decode ciphertext hex: %w", err)
		}

		plaintext, err := aesGCMDecrypt(key, ciphertext)
		if err != nil {
			return "", fmt.Errorf("failed GCM decryption for version %s: %w", version, err)
		}

		return string(plaintext), nil
	}

	// Legacy unversioned decryption fallback
	if len(km.legacyDefaultKey) == 0 {
		return "", errors.New("ciphertext missing version and no legacy key configured")
	}

	ciphertext, err := hex.DecodeString(encryptedStr)
	if err != nil {
		return "", fmt.Errorf("failed to decode legacy ciphertext hex: %w", err)
	}

	plaintext, err := aesGCMDecrypt(km.legacyDefaultKey, ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed GCM decryption with legacy key: %w", err)
	}

	return string(plaintext), nil
}

// aesGCMEncrypt helper performs basic AES-GCM-256 seal operation
func aesGCMEncrypt(key []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// aesGCMDecrypt helper performs basic AES-GCM-256 open operation
func aesGCMDecrypt(key []byte, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, actualCipher := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, actualCipher, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
