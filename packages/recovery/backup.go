package recovery

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// BackupType defines full vs incremental backup strategies
type BackupType string

const (
	BackupFull        BackupType = "FULL"
	BackupIncremental BackupType = "INCREMENTAL"
)

// BackupState encapsulates a snapshot state of the exchange
type BackupState struct {
	DatabaseTables map[string]int    `json:"database_tables"` // TableName -> RowCount
	WalletState    map[string]float64 `json:"wallet_state"`    // WalletID -> TotalBalance
	Configs        map[string]string  `json:"configs"`         // ConfigKey -> ConfigValue
	Secrets        map[string]string  `json:"secrets"`         // SecretID -> HashedSecret
}

// BackupMetadata represents a completed, encrypted backup block record
type BackupMetadata struct {
	ID         string     `json:"id"`
	Timestamp  time.Time  `json:"timestamp"`
	Type       BackupType `json:"type"`
	Size       int        `json:"size"`
	Checksum   string     `json:"checksum"`
	CipherKey  string     `json:"cipher_key"` // Hex-encoded encryption key
	Verified   bool       `json:"verified"`
}

// BackupManager orchestrates secure encrypted snapshot backups
type BackupManager struct {
	mu           sync.RWMutex
	backups      map[string]*BackupMetadata
	rawStorage   map[string][]byte // key: BackupID -> encrypted bytes
	encryptionKey []byte            // 32-byte master backup symmetric key
}

// NewBackupManager initializes the Enterprise Backup Manager
func NewBackupManager() (*BackupManager, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate secure backup key: %w", err)
	}
	return &BackupManager{
		backups:       make(map[string]*BackupMetadata),
		rawStorage:    make(map[string][]byte),
		encryptionKey: key,
	}, nil
}

// EncryptData performs secure 256-bit AES-GCM symmetric block encryption
func (bm *BackupManager) EncryptData(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(bm.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// DecryptData performs AES-GCM symmetric block decryption
func (bm *BackupManager) DecryptData(encryptedData []byte) ([]byte, error) {
	block, err := aes.NewCipher(bm.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failure: %w", err)
	}

	return plaintext, nil
}

// CreateBackup generates a fully encrypted backup of the exchange state
func (bm *BackupManager) CreateBackup(bType BackupType, state *BackupState) (*BackupMetadata, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if state == nil {
		return nil, fmt.Errorf("cannot backup empty state parameters")
	}

	// 1. Serialize backup state to JSON
	serialized, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize state: %w", err)
	}

	// 2. Encrypt serialized bytes securely via AES-GCM
	encrypted, err := bm.EncryptData(serialized)
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	backupID := fmt.Sprintf("bak_%d", time.Now().UnixNano())

	// 3. Compute SHA-256 integrity checksum on encrypted bytes
	hash := sha256.Sum256(encrypted)
	checksumHex := hex.EncodeToString(hash[:])

	meta := &BackupMetadata{
		ID:        backupID,
		Timestamp: time.Now(),
		Type:      bType,
		Size:      len(encrypted),
		Checksum:  checksumHex,
		CipherKey: hex.EncodeToString(bm.encryptionKey),
		Verified:  true, // Initialized as true since we just created it
	}

	// 4. Save to maps
	bm.backups[backupID] = meta
	bm.rawStorage[backupID] = encrypted

	return meta, nil
}

// ListBackups returns lists of completed metadata blocks
func (bm *BackupManager) ListBackups() []*BackupMetadata {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var list []*BackupMetadata
	for _, m := range bm.backups {
		list = append(list, m)
	}
	return list
}

// GetBackupData retrieves and decrypts a specific backup payload
func (bm *BackupManager) GetBackupData(backupID string) ([]byte, error) {
	bm.mu.RLock()
	encrypted, exists := bm.rawStorage[backupID]
	bm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("backup %s not found in storage", backupID)
	}

	// Verify Checksum before decrypting
	hash := sha256.Sum256(encrypted)
	computedChecksum := hex.EncodeToString(hash[:])

	meta := bm.backups[backupID]
	if meta.Checksum != computedChecksum {
		return nil, fmt.Errorf("security violation: backup %s checksum mismatch (tampered backup block)", backupID)
	}

	// Decrypt
	decrypted, err := bm.DecryptData(encrypted)
	if err != nil {
		return nil, err
	}

	return decrypted, nil
}
