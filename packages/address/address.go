package address

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"velyxora/packages/blockchain"
)

// AddressStatus defines ownership and lifecycle states
type AddressStatus string

const (
	StatusAllocated AddressStatus = "ALLOCATED"
	StatusVerified  AddressStatus = "VERIFIED"
	StatusInactive  AddressStatus = "INACTIVE"
)

// AddressRecord represents a generated and assigned deposit address inside the exchange
type AddressRecord struct {
	UserID    string        `json:"user_id"`
	Network   string        `json:"network"`
	Address   string        `json:"address"`
	PublicKey []byte        `json:"public_key"`
	DerivationPath string   `json:"derivation_path"`
	Memo      string        `json:"memo,omitempty"`
	Status    AddressStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
}

// AddressMetadata stores tracking and HD wallet attributes
type AddressMetadata struct {
	Address        string `json:"address"`
	KeyIndex       uint32 `json:"key_index"`
	IsChange       bool   `json:"is_change"`
	TransactionCount int   `json:"transaction_count"`
}

// AddressRegistry handles thread-safe allocation and quick lookup of generated user deposit addresses
type AddressRegistry struct {
	mu        sync.RWMutex
	records   map[string]*AddressRecord // key: "userID_network" or "address"
	byAddress map[string]*AddressRecord
	metadata  map[string]*AddressMetadata
}

// NewAddressRegistry instantiates an empty addresses subsystem
func NewAddressRegistry() *AddressRegistry {
	return &AddressRegistry{
		records:   make(map[string]*AddressRecord),
		byAddress: make(map[string]*AddressRecord),
		metadata:  make(map[string]*AddressMetadata),
	}
}

// RegisterAddress inserts a new generated address record into the thread-safe registries
func (ar *AddressRegistry) RegisterAddress(record *AddressRecord, meta *AddressMetadata) error {
	if record == nil || record.Address == "" || record.UserID == "" {
		return errors.New("invalid address record parameters")
	}

	ar.mu.Lock()
	defer ar.mu.Unlock()

	key := fmt.Sprintf("%s_%s", record.UserID, record.Network)
	ar.records[key] = record
	ar.byAddress[record.Address] = record

	if meta != nil {
		ar.metadata[record.Address] = meta
	}

	return nil
}

// LookupAddress retrieves a record by user and network
func (ar *AddressRegistry) LookupAddress(userID, network string) (*AddressRecord, error) {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	key := fmt.Sprintf("%s_%s", userID, network)
	rec, exists := ar.records[key]
	if !exists {
		return nil, errors.New("address not allocated for this user on network")
	}
	return rec, nil
}

// LookupByAddress retrieves a record directly by address string
func (ar *AddressRegistry) LookupByAddress(address string) (*AddressRecord, error) {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	rec, exists := ar.byAddress[address]
	if !exists {
		return nil, errors.New("address not found in deposit registries")
	}
	return rec, nil
}

// VerifyOwnership validates that a specific address belongs to the user and verifies a cryptographic message signature
func (ar *AddressRegistry) VerifyOwnership(adapter blockchain.BlockchainAdapter, address string, challenge []byte, signature []byte) (bool, error) {
	rec, err := ar.LookupByAddress(address)
	if err != nil {
		return false, fmt.Errorf("failed to locate registered address records: %w", err)
	}

	if len(rec.PublicKey) == 0 {
		return false, errors.New("no public key registered for address verification")
	}

	// Verify using the standard blockchain adapter
	verified, err := adapter.VerifySignature(rec.PublicKey, challenge, signature)
	if err != nil {
		return false, fmt.Errorf("cryptographic signature verification failed: %w", err)
	}

	if verified {
		ar.mu.Lock()
		rec.Status = StatusVerified
		ar.mu.Unlock()
	}

	return verified, nil
}
