package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
)

type EncryptionEngine struct {
	encryptionKey []byte
	algorithm     string
	mutex         *sync.RWMutex
}

func NewEncryptionEngine(keyHex string, algorithm string) (*EncryptionEngine, error) {
	if algorithm != "AES-256-GCM" && algorithm != "AES-256-CBC" {
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encryption key: %w", err)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 256 bits (64 hex chars), got %d bits", len(key)*8)
	}

	return &EncryptionEngine{
		encryptionKey: key,
		algorithm:     algorithm,
		mutex:         &sync.RWMutex{},
	}, nil
}

func (ee *EncryptionEngine) EncryptAESGCM(plaintext string) (string, error) {
	ee.mutex.RLock()
	defer ee.mutex.RUnlock()

	block, err := aes.NewCipher(ee.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM mode: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (ee *EncryptionEngine) DecryptAESGCM(ciphertext string) (string, error) {
	ee.mutex.RLock()
	defer ee.mutex.RUnlock()

	block, err := aes.NewCipher(ee.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM mode: %w", err)
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

func (ee *EncryptionEngine) HashSHA256(plaintext string) string {
	hash := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(hash[:])
}

func (ee *EncryptionEngine) GenerateEncryptionKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}
	return hex.EncodeToString(key), nil
}

func (ee *EncryptionEngine) RotateKey(newKeyHex string) error {
	key, err := hex.DecodeString(newKeyHex)
	if err != nil {
		return fmt.Errorf("failed to decode new key: %w", err)
	}

	if len(key) != 32 {
		return fmt.Errorf("new key must be 256 bits")
	}

	ee.mutex.Lock()
	ee.encryptionKey = key
	ee.mutex.Unlock()

	return nil
}

func (ee *EncryptionEngine) DeriveKeyFromPassword(password string, salt string) string {
	hash := sha256.Sum256([]byte(password + salt))
	return hex.EncodeToString(hash[:])
}
