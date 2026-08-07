package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// HSMProvider defines abstract operations executed inside real or simulated Hardware Security Modules
type HSMProvider interface {
	GenerateHSMKey(keyID string) ([]byte, error)
	SignWithHSM(keyID string, data []byte) ([]byte, error)
	VerifyWithHSM(keyID string, data []byte, signature []byte) (bool, error)
}

// SimulatedHSM implements a thread-safe software-backed HSM provider
type SimulatedHSM struct {
	mu   sync.RWMutex
	keys map[string]*ecdsa.PrivateKey
}

// NewSimulatedHSM initializes the secure hardware simulation wrapper
func NewSimulatedHSM() *SimulatedHSM {
	return &SimulatedHSM{
		keys: make(map[string]*ecdsa.PrivateKey),
	}
}

// GenerateHSMKey provisions a new private key inside the simulated HSM enclave
func (s *SimulatedHSM) GenerateHSMKey(keyID string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key inside HSM: %w", err)
	}

	s.keys[keyID] = privKey

	// Return the serialized compressed public key (the private key never leaves the simulated HSM)
	pubBytes := elliptic.Marshal(elliptic.P256(), privKey.PublicKey.X, privKey.PublicKey.Y)
	return pubBytes, nil
}

// SignWithHSM performs high-security cryptographic signing inside the simulated HSM
func (s *SimulatedHSM) SignWithHSM(keyID string, data []byte) ([]byte, error) {
	s.mu.RLock()
	privKey, exists := s.keys[keyID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("key %s not found in HSM", keyID)
	}

	hash := sha256.Sum256(data)
	r, sigS, err := ecdsa.Sign(rand.Reader, privKey, hash[:])
	if err != nil {
		return nil, err
	}

	// Compact signature serialization
	signature := append(r.Bytes(), sigS.Bytes()...)
	return signature, nil
}

// VerifyWithHSM validates signatures against the derived public key in the HSM
func (s *SimulatedHSM) VerifyWithHSM(keyID string, data []byte, signature []byte) (bool, error) {
	s.mu.RLock()
	privKey, exists := s.keys[keyID]
	s.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("key %s not found in HSM", keyID)
	}

	if len(signature) < 64 {
		return false, errors.New("invalid signature format")
	}

	hash := sha256.Sum256(data)
	r := new(big.Int).SetBytes(signature[:32])
	sigS := new(big.Int).SetBytes(signature[32:])

	return ecdsa.Verify(&privKey.PublicKey, hash[:], r, sigS), nil
}

// APIKeyStatus represents API key operational states
type APIKeyStatus string

const (
	APIKeyActive  APIKeyStatus = "ACTIVE"
	APIKeyRevoked APIKeyStatus = "REVOKED"
	APIKeyExpired APIKeyStatus = "EXPIRED"
)

// APIKeyMetadata represents a secure programmatic access key lifecycle
type APIKeyMetadata struct {
	ID           string       `json:"id"`
	UserID       string       `json:"user_id"`
	PublicKey    string       `json:"public_key"`
	HashedSecret string       `json:"hashed_secret"`
	Status       APIKeyStatus `json:"status"`
	Permissions  []string     `json:"permissions"`
	ExpiresAt    time.Time    `json:"expires_at"`
	CreatedAt    time.Time    `json:"created_at"`
}

// KeyManager implements Enterprise Cryptographic Secret Rotation & Lifecycle Policies
type KeyManager struct {
	mu           sync.RWMutex
	hsm          HSMProvider
	apiKeys      map[string]*APIKeyMetadata
	secretsStore map[string]string // Key: secretID, Value: secretValue
}

// NewKeyManager initializes the Enterprise Cryptographic Key and Secret Orchestrator
func NewKeyManager(hsm HSMProvider) *KeyManager {
	if hsm == nil {
		hsm = NewSimulatedHSM()
	}
	return &KeyManager{
		hsm:          hsm,
		apiKeys:      make(map[string]*APIKeyMetadata),
		secretsStore: make(map[string]string),
	}
}

// RegisterAPIKey creates a secure API credentials key pair
func (km *KeyManager) RegisterAPIKey(userID string, permissions []string, secret string, duration time.Duration) (*APIKeyMetadata, string, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	keyID := fmt.Sprintf("key_%d", time.Now().UnixNano())

	// Create API secret token
	entropy := make([]byte, 32)
	_, _ = rand.Read(entropy)
	rawSecret := hex.EncodeToString(entropy)

	// Hash the secret securely using SHA-512
	hasher := sha512.New()
	hasher.Write([]byte(rawSecret))
	hashedSecret := hex.EncodeToString(hasher.Sum(nil))

	metadata := &APIKeyMetadata{
		ID:           keyID,
		UserID:       userID,
		PublicKey:    keyID,
		HashedSecret: hashedSecret,
		Status:       APIKeyActive,
		Permissions:  permissions,
		ExpiresAt:    time.Now().Add(duration),
		CreatedAt:    time.Now(),
	}

	km.apiKeys[keyID] = metadata
	return metadata, rawSecret, nil
}

// RevokeAPIKey flags programmatic credentials as permanently dead
func (km *KeyManager) RevokeAPIKey(keyID string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	key, exists := km.apiKeys[keyID]
	if !exists {
		return fmt.Errorf("API Key %s not found", keyID)
	}

	key.Status = APIKeyRevoked
	return nil
}

// RotateSecret updates encryption secrets securely
func (km *KeyManager) RotateSecret(secretID string, newValue string) {
	km.mu.Lock()
	defer km.mu.Unlock()
	km.secretsStore[secretID] = newValue
}

// RetrieveSecret retrieves live encryption parameters securely
func (km *KeyManager) RetrieveSecret(secretID string) (string, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	val, exists := km.secretsStore[secretID]
	if !exists {
		return "", fmt.Errorf("secret %s not registered", secretID)
	}
	return val, nil
}

// ValidateAPIKey validates programmatic credentials and permissions securely
func (km *KeyManager) ValidateAPIKey(keyID string, providedSecret string) (bool, error) {
	km.mu.RLock()
	key, exists := km.apiKeys[keyID]
	km.mu.RUnlock()

	if !exists {
		return false, errors.New("API Key not found")
	}

	if key.Status != APIKeyActive {
		return false, fmt.Errorf("API Key is %s", key.Status)
	}

	if time.Now().After(key.ExpiresAt) {
		key.Status = APIKeyExpired
		return false, errors.New("API Key has expired")
	}

	// Verify hashed secret matches
	hasher := sha512.New()
	hasher.Write([]byte(providedSecret))
	computedHash := hex.EncodeToString(hasher.Sum(nil))

	if key.HashedSecret != computedHash {
		return false, errors.New("invalid API secret credentials")
	}

	return true, nil
}

// EvaluateAdaptiveRisk implements Risk-Based Authentication
func EvaluateAdaptiveRisk(ip string, geo string, device string, ipHistory map[string]bool) float64 {
	score := 0.1 // Base background noise risk

	// Check for blacklisted IP / unverified IP subnet
	if ip == "192.168.99.99" || ip == "10.0.99.99" {
		score += 0.5
	}

	// Check if IP is in user's historical IP pool (unrecognized login location)
	if ipHistory != nil && !ipHistory[ip] {
		score += 0.25
	}

	// Geographical mismatch check
	if geo == "UNKNOWN" || geo == "" {
		score += 0.15
	}

	// Device fingerprint spoofing or empty checks
	if device == "UNKNOWN" || device == "" {
		score += 0.15
	}

	if score > 1.0 {
		score = 1.0
	}

	return score
}
