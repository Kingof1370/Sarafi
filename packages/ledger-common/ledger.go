package ledger_common

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// AuditAction represents high-level wallet operations to track in audit logs
type AuditAction string

const (
	ActionWalletCreated     AuditAction = "WALLET_CREATED"
	ActionAddressAllocated  AuditAction = "ADDRESS_ALLOCATED"
	ActionDepositReceived   AuditAction = "DEPOSIT_RECEIVED"
	ActionWithdrawalCreated AuditAction = "WITHDRAWAL_CREATED"
	ActionWithdrawalApproved AuditAction = "WITHDRAWAL_APPROVED"
	ActionWithdrawalSent    AuditAction = "WITHDRAWAL_SENT"
	ActionBalanceReserved   AuditAction = "BALANCE_RESERVED"
	ActionBalanceReleased   AuditAction = "BALANCE_RELEASED"
	ActionBalanceAdjusted   AuditAction = "BALANCE_ADJUSTED"
	ActionIntegrityFailed   AuditAction = "INTEGRITY_FAILED"
)

// WalletAuditRecord defines standard log records for secure off-chain and on-chain compliance validation
type WalletAuditRecord struct {
	ID          string      `json:"id"`
	UserID      string      `json:"user_id"`
	WalletID    string      `json:"wallet_id"`
	Asset       string      `json:"asset"`
	Action      AuditAction `json:"action"`
	Amount      float64     `json:"amount"`
	PrevBalance float64     `json:"prev_balance"`
	NewBalance  float64     `json:"new_balance"`
	Message     string      `json:"message"`
	PrevHash    string      `json:"prev_hash"`
	Hash        string      `json:"hash"`
	Timestamp   time.Time   `json:"timestamp"`
}

// ComputeHash generates a secure cryptographically chained hash for the record to prevent tampering
func (ar *WalletAuditRecord) ComputeHash() string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%.18f|%.18f|%.18f|%s|%s|%d",
		ar.ID, ar.UserID, ar.WalletID, ar.Asset, string(ar.Action), ar.Amount, ar.PrevBalance, ar.NewBalance, ar.Message, ar.PrevHash, ar.Timestamp.UnixNano())
	hash := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(hash[:])
}

// ValidateIntegrity confirms the record hash matches and chains correctly with the previous record
func (ar *WalletAuditRecord) ValidateIntegrity(prevRecord *WalletAuditRecord) (bool, error) {
	computed := ar.ComputeHash()
	if computed != ar.Hash {
		return false, fmt.Errorf("hash verification failed: record has %s, computed %s", ar.Hash, computed)
	}

	if prevRecord != nil {
		if ar.PrevHash != prevRecord.Hash {
			return false, errors.New("hash chain integrity broken with previous block")
		}
	} else if ar.PrevHash != "" {
		return false, errors.New("genesis block should have empty prev_hash")
	}

	return true, nil
}
