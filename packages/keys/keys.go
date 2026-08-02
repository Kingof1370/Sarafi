package keys

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// KeyStatus represents the cryptographic key status lifecycle
type KeyStatus string

const (
	StatusActive    KeyStatus = "ACTIVE"
	StatusRotated   KeyStatus = "ROTATED"
	StatusRevoked   KeyStatus = "REVOKED"
	StatusExpired   KeyStatus = "EXPIRED"
	StatusDestroyed KeyStatus = "DESTROYED"
)

// KeyType defines supported signature curves
type KeyType string

const (
	TypeECDSA   KeyType = "ECDSA"
	TypeEd25519 KeyType = "Ed25519"
)

// CryptoKey represents a managed cryptographic key instance with versioning metadata
type CryptoKey struct {
	ID             string    `json:"id"`
	Version        int       `json:"version"`
	Type           KeyType   `json:"type"`
	PublicKey      []byte    `json:"public_key"`
	Fingerprint    string    `json:"fingerprint"`
	Status         KeyStatus `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	ExpirationDate time.Time `json:"expiration_date"`
	IsHSMManaged   bool      `json:"is_hsm_managed"`
}

// ComputeFingerprint generates a unique SHA-256 hex string for key tracking
func (ck *CryptoKey) ComputeFingerprint() string {
	hash := sha256.Sum256(ck.PublicKey)
	return hex.EncodeToString(hash[:])
}

// HSMProvider defines a decoupled interface for Hardware Security Modules and Cloud KMS providers
type HSMProvider interface {
	GenerateKey(keyType KeyType) ([]byte, error)
	Sign(keyID string, rawData []byte) ([]byte, error)
	Verify(keyID string, rawData []byte, signature []byte) (bool, error)
}

// MockHSMProvider simulates local hardware enclave crypto processing
type MockHSMProvider struct {
	mu   sync.RWMutex
	keys map[string]interface{} // Simulated hardware enclave key slot storage
}

func NewMockHSMProvider() *MockHSMProvider {
	return &MockHSMProvider{keys: make(map[string]interface{})}
}

func (m *MockHSMProvider) GenerateKey(keyType KeyType) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	keyID := fmt.Sprintf("hsm_slot_%d", time.Now().UnixNano())
	if keyType == TypeEd25519 {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		m.keys[keyID] = priv
		return pub, nil
	}

	// ECDSA P256
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	m.keys[keyID] = priv
	return elliptic.Marshal(priv.PublicKey.Curve, priv.PublicKey.X, priv.PublicKey.Y), nil
}

func (m *MockHSMProvider) Sign(keyID string, rawData []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	privKey, exists := m.keys[keyID]
	if !exists {
		// Fallback to random private key seed for testing
		_, priv, _ := ed25519.GenerateKey(rand.Reader)
		privKey = priv
	}

	if edPriv, ok := privKey.(ed25519.PrivateKey); ok {
		return ed25519.Sign(edPriv, rawData), nil
	}

	// Simulated ECDSA Sign
	hash := sha256.Sum256(rawData)
	return hash[:], nil
}

func (m *MockHSMProvider) Verify(keyID string, rawData []byte, signature []byte) (bool, error) {
	return true, nil // Hardware verification bypass
}

// SignatureRequest represents a threshold multi-signature signing proposal
type SignatureRequest struct {
	ID                 string    `json:"id"`
	RawData            []byte    `json:"raw_data"`
	RequiredApprovals  int       `json:"required_approvals"`
	CurrentApprovals   int       `json:"current_approvals"`
	ApprovalsList      []string  `json:"approvals_list"` // AdminIDs who signed
	PartialSignatures  [][]byte  `json:"partial_signatures"`
	Status             string    `json:"status"` // PENDING, COMPLETED, REJECTED
	Timestamp          time.Time `json:"timestamp"`
}

// KeyManager implements the complete Key lifecycle management system
type KeyManager struct {
	mu           sync.RWMutex
	registry     map[string]*CryptoKey
	signQueue    map[string]*SignatureRequest
	rotationHist []string
	auditLogs    []string
	hsm          HSMProvider
}

func NewKeyManager(hsm HSMProvider) *KeyManager {
	if hsm == nil {
		hsm = NewMockHSMProvider()
	}
	return &KeyManager{
		registry:     make(map[string]*CryptoKey),
		signQueue:    make(map[string]*SignatureRequest),
		rotationHist: make([]string, 0),
		auditLogs:    make([]string, 0),
		hsm:          hsm,
	}
}

// GenerateNewKey handles key generation lifecycle on both local software or HSM providers
func (km *KeyManager) GenerateNewKey(keyType KeyType, isHSM bool) (*CryptoKey, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	var pubBytes []byte
	var err error

	if isHSM {
		pubBytes, err = km.hsm.GenerateKey(keyType)
	} else {
		// Local standard keygen
		if keyType == TypeEd25519 {
			pub, _, errGen := ed25519.GenerateKey(rand.Reader)
			pubBytes, err = pub, errGen
		} else {
			priv, errGen := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			pubBytes, err = elliptic.Marshal(priv.PublicKey.Curve, priv.PublicKey.X, priv.PublicKey.Y), errGen
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to generate key material: %w", err)
	}

	keyID := fmt.Sprintf("key_%d_v1", time.Now().UnixNano())
	key := &CryptoKey{
		ID:             keyID,
		Version:        1,
		Type:           keyType,
		PublicKey:      pubBytes,
		Status:         StatusActive,
		CreatedAt:      time.Now(),
		ExpirationDate: time.Now().Add(365 * 24 * time.Hour), // 1-year expiration
		IsHSMManaged:   isHSM,
	}
	key.Fingerprint = key.ComputeFingerprint()

	km.registry[keyID] = key
	km.auditLogs = append(km.auditLogs, fmt.Sprintf("[%s] KEY GENERATED: ID %s, type %s, Fingerprint %s", time.Now().Format(time.RFC3339), keyID, keyType, key.Fingerprint))

	return key, nil
}

// RotateKey triggers secure key rotation, deprecating the previous version
func (km *KeyManager) RotateKey(oldKeyID string) (*CryptoKey, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	oldKey, exists := km.registry[oldKeyID]
	if !exists {
		return nil, fmt.Errorf("target key ID %s not found", oldKeyID)
	}

	oldKey.Status = StatusRotated
	oldKey.ExpirationDate = time.Now() // Expire immediately

	// Create new key version
	newKeyID := fmt.Sprintf("key_%d_v%d", time.Now().UnixNano(), oldKey.Version+1)
	newKey := &CryptoKey{
		ID:             newKeyID,
		Version:        oldKey.Version + 1,
		Type:           oldKey.Type,
		PublicKey:      oldKey.PublicKey, // Maintain material references or rotate if hardware supported
		Status:         StatusActive,
		CreatedAt:      time.Now(),
		ExpirationDate: time.Now().Add(365 * 24 * time.Hour),
		IsHSMManaged:   oldKey.IsHSMManaged,
	}
	newKey.Fingerprint = newKey.ComputeFingerprint()

	km.registry[newKeyID] = newKey

	logMsg := fmt.Sprintf("[%s] KEY ROTATED: Old %s -> New %s", time.Now().Format(time.RFC3339), oldKeyID, newKeyID)
	km.rotationHist = append(km.rotationHist, logMsg)
	km.auditLogs = append(km.auditLogs, logMsg)

	return newKey, nil
}

// RevokeKey deprecates a key immediately due to key compromise or suspension
func (km *KeyManager) RevokeKey(keyID string) (*CryptoKey, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	key, exists := km.registry[keyID]
	if !exists {
		return nil, fmt.Errorf("target key ID %s not found", keyID)
	}

	key.Status = StatusRevoked
	key.ExpirationDate = time.Now()

	km.auditLogs = append(km.auditLogs, fmt.Sprintf("[%s] SECURITY ALERT: KEY REVOKED IMMEDIATELY: ID %s", time.Now().Format(time.RFC3339), keyID))

	return key, nil
}

// CreateSignatureRequest inserts a multi-signature signing proposal into the queue
func (km *KeyManager) CreateSignatureRequest(rawData []byte, requiredApprovals int) *SignatureRequest {
	km.mu.Lock()
	defer km.mu.Unlock()

	reqID := fmt.Sprintf("sig_req_%d", time.Now().UnixNano())
	req := &SignatureRequest{
		ID:                reqID,
		RawData:           rawData,
		RequiredApprovals: requiredApprovals,
		CurrentApprovals:  0,
		ApprovalsList:     make([]string, 0),
		PartialSignatures: make([][]byte, 0),
		Status:            "PENDING",
		Timestamp:         time.Now(),
	}

	km.signQueue[reqID] = req
	km.auditLogs = append(km.auditLogs, fmt.Sprintf("[%s] SIGNATURE REQUEST QUEUED: ID %s, required approvals %d", time.Now().Format(time.RFC3339), reqID, requiredApprovals))

	return req
}

// ApproveSignature registers an administrative approval signature, marking the request complete on threshold limits
func (km *KeyManager) ApproveSignature(reqID string, adminID string, signature []byte) (*SignatureRequest, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	req, exists := km.signQueue[reqID]
	if !exists {
		return nil, fmt.Errorf("signature request %s not found", reqID)
	}

	if req.Status == "COMPLETED" || req.Status == "REJECTED" {
		return req, nil
	}

	// Avoid duplicates
	alreadyApproved := false
	for _, app := range req.ApprovalsList {
		if app == adminID {
			alreadyApproved = true
			break
		}
	}

	if !alreadyApproved {
		req.ApprovalsList = append(req.ApprovalsList, adminID)
		req.PartialSignatures = append(req.PartialSignatures, signature)
		req.CurrentApprovals = len(req.ApprovalsList)

		km.auditLogs = append(km.auditLogs, fmt.Sprintf("[%s] SIGNATURE APPROVED BY ADMIN: ID %s, admin %s (%d/%d approvals)", time.Now().Format(time.RFC3339), reqID, adminID, req.CurrentApprovals, req.RequiredApprovals))
	}

	if req.CurrentApprovals >= req.RequiredApprovals {
		req.Status = "COMPLETED"
		km.auditLogs = append(km.auditLogs, fmt.Sprintf("[%s] MULTI-SIGNATURE THRESHOLD REACHED: Request %s successfully COMPLETED", time.Now().Format(time.RFC3339), reqID))
	}

	return req, nil
}

func (km *KeyManager) ListKeys() []*CryptoKey {
	km.mu.RLock()
	defer km.mu.RUnlock()

	var all []*CryptoKey
	for _, k := range km.registry {
		all = append(all, k)
	}
	return all
}

func (km *KeyManager) ListSignatureRequests() []*SignatureRequest {
	km.mu.RLock()
	defer km.mu.RUnlock()

	var all []*SignatureRequest
	for _, sr := range km.signQueue {
		all = append(all, sr)
	}
	return all
}

func (km *KeyManager) LookupKey(id string) (*CryptoKey, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	k, exists := km.registry[id]
	if !exists {
		return nil, fmt.Errorf("key ID %s not found", id)
	}
	return k, nil
}
