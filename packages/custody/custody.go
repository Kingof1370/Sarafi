package custody

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// VaultType defines the class of asset custody storage
type VaultType string

const (
	TypeHot           VaultType = "HOT"
	TypeWarm          VaultType = "WARM"
	TypeCold          VaultType = "COLD"
	TypeOffline       VaultType = "OFFLINE"
	TypeDeepCold      VaultType = "DEEP_COLD"
	TypeTreasuryVault VaultType = "TREASURY_VAULT"
	TypeReserveVault  VaultType = "RESERVE_VAULT"
	TypeRecoveryVault VaultType = "RECOVERY_VAULT"
)

// TransferStatus defines lifecycle stages for custody movements
type TransferStatus string

const (
	StatusRequested TransferStatus = "REQUESTED"
	StatusApproved  TransferStatus = "APPROVED"
	StatusCompleted TransferStatus = "COMPLETED"
	StatusRejected  TransferStatus = "REJECTED"
)

// Vault represents a secure institutional asset vault segment
type Vault struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Type      VaultType          `json:"type"`
	Balances  map[string]float64 `json:"balances"` // key: asset symbol
	IsLocked  bool               `json:"is_locked"`
	CreatedAt time.Time          `json:"created_at"`
}

// CustodyTransfer represents an internal asset movement between vaults
type CustodyTransfer struct {
	ID                string         `json:"id"`
	FromVaultID       string         `json:"from_vault_id"`
	ToVaultID         string         `json:"to_vault_id"`
	Asset             string         `json:"asset"`
	Amount            float64        `json:"amount"`
	Status            TransferStatus `json:"status"`
	RequiredApprovals int            `json:"required_approvals"`
	CurrentApprovals  int            `json:"current_approvals"`
	ApprovalsList     []string       `json:"approvals_list"` // AdminIDs who signed
	Timestamp         time.Time      `json:"timestamp"`
}

// CustodyManager orchestrates vaults, security rules, emergency actions, and 4-eyes workflows
type CustodyManager struct {
	mu                sync.RWMutex
	vaults            map[string]*Vault
	transfers         map[string]*CustodyTransfer
	isEmergencyFrozen bool
	auditLogs         []string
}

// NewCustodyManager initializes a clean Custody subsystem
func NewCustodyManager() *CustodyManager {
	manager := &CustodyManager{
		vaults:            make(map[string]*Vault),
		transfers:         make(map[string]*CustodyTransfer),
		isEmergencyFrozen: false,
		auditLogs:         make([]string, 0),
	}

	// Bootstrap default institutional vaults
	manager.bootstrapDefaultVaults()

	return manager
}

func (cm *CustodyManager) bootstrapDefaultVaults() {
	defaultVaults := []struct {
		ID   string
		Name string
		Type VaultType
	}{
		{"v_hot", "Hot Exchange Vault", TypeHot},
		{"v_warm", "Warm Operational Vault", TypeWarm},
		{"v_cold", "Cold Core Storage", TypeCold},
		{"v_deep_cold", "Deep Cold Air-Gapped Vault", TypeDeepCold},
		{"v_treasury", "Corporate Treasury Vault", TypeTreasuryVault},
		{"v_reserve", "Platform Emergency Reserve", TypeReserveVault},
		{"v_recovery", "Disaster Recovery Vault", TypeRecoveryVault},
	}

	for _, v := range defaultVaults {
		cm.vaults[v.ID] = &Vault{
			ID:        v.ID,
			Name:      v.Name,
			Type:      v.Type,
			Balances:  make(map[string]float64),
			IsLocked:  false,
			CreatedAt: time.Now(),
		}
	}
}

// DepositToVault directly credits an asset balance within a specified vault (used on block sync finalisations)
func (cm *CustodyManager) DepositToVault(vaultID, asset string, amount float64) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	v, exists := cm.vaults[vaultID]
	if !exists {
		return fmt.Errorf("vault %s not found", vaultID)
	}

	if v.IsLocked || cm.isEmergencyFrozen {
		return errors.New("vault operations suspended: emergency lock active")
	}

	v.Balances[asset] += amount
	return nil
}

// CreateTransferRequest initiates an asset movement, calculating 4-eyes approvals based on vault types
func (cm *CustodyManager) CreateTransferRequest(ctx context.Context, fromID, toID, asset string, amount float64) (*CustodyTransfer, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.isEmergencyFrozen {
		return nil, errors.New("security violation: all asset transfers suspended due to active EMERGENCY FREEZE")
	}

	src, exists := cm.vaults[fromID]
	if !exists {
		return nil, fmt.Errorf("source vault %s not found", fromID)
	}

	dst, exists := cm.vaults[toID]
	if !exists {
		return nil, fmt.Errorf("destination vault %s not found", toID)
	}

	if src.IsLocked || dst.IsLocked {
		return nil, errors.New("vault operations suspended: source or destination vault is locked")
	}

	// Balance check
	avail := src.Balances[asset]
	if avail < amount {
		return nil, fmt.Errorf("insufficient vault balance: available %.4f, requested %.4f", avail, amount)
	}

	// 4-Eyes Principle: approvals count based on priority tiers
	reqApprovals := 1
	if src.Type == TypeCold || src.Type == TypeDeepCold || src.Type == TypeTreasuryVault {
		reqApprovals = 2 // Require at least 2 independent admins for high-risk vaults
	}

	txID := fmt.Sprintf("cust_tx_%d_%s", time.Now().UnixNano(), asset)
	transfer := &CustodyTransfer{
		ID:                txID,
		FromVaultID:       fromID,
		ToVaultID:         toID,
		Asset:             asset,
		Amount:            amount,
		Status:            StatusRequested,
		RequiredApprovals: reqApprovals,
		CurrentApprovals:  0,
		ApprovalsList:     make([]string, 0),
		Timestamp:         time.Now(),
	}

	cm.transfers[txID] = transfer
	cm.auditLogs = append(cm.auditLogs, fmt.Sprintf("[%s] CUSTODY TRANSFER REQUESTED: ID %s, %s -> %s, amount %.4f %s", time.Now().Format(time.RFC3339), txID, fromID, toID, amount, asset))

	return transfer, nil
}

// ApproveTransferRequest registers an admin approval, executing the transfer immediately upon threshold achievement
func (cm *CustodyManager) ApproveTransferRequest(ctx context.Context, txID, adminID string) (*CustodyTransfer, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.isEmergencyFrozen {
		return nil, errors.New("all operations suspended: emergency lock active")
	}

	req, exists := cm.transfers[txID]
	if !exists {
		return nil, fmt.Errorf("custody transfer %s not found", txID)
	}

	if req.Status == StatusCompleted || req.Status == StatusRejected {
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
		req.CurrentApprovals = len(req.ApprovalsList)
		cm.auditLogs = append(cm.auditLogs, fmt.Sprintf("[%s] CUSTODY TRANSFER APPROVED BY ADMIN: ID %s, admin %s (%d/%d approvals)", time.Now().Format(time.RFC3339), txID, adminID, req.CurrentApprovals, req.RequiredApprovals))
	}

	if req.CurrentApprovals >= req.RequiredApprovals {
		// Execute double-entry transfer between vaults
		src := cm.vaults[req.FromVaultID]
		dst := cm.vaults[req.ToVaultID]

		if src.Balances[req.Asset] < req.Amount {
			req.Status = StatusRejected
			return nil, errors.New("execution failed: insufficient source vault balance at finalization")
		}

		src.Balances[req.Asset] -= req.Amount
		dst.Balances[req.Asset] += req.Amount
		req.Status = StatusCompleted

		cm.auditLogs = append(cm.auditLogs, fmt.Sprintf("[%s] CUSTODY TRANSFER EXECUTED SUCCESSFULLY: ID %s completed", time.Now().Format(time.RFC3339), txID))
	} else {
		req.Status = StatusApproved
	}

	return req, nil
}

// ToggleEmergencyFreeze triggers a system-wide platform halt, locking all vaults and transfers instantly
func (cm *CustodyManager) ToggleEmergencyFreeze(ctx context.Context, action string, operatorID string) (bool, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if action == "FREEZE" {
		cm.isEmergencyFrozen = true
		for _, v := range cm.vaults {
			v.IsLocked = true
		}
		cm.auditLogs = append(cm.auditLogs, fmt.Sprintf("[%s] !!! SECURITY EMERGENCY: SYSTEM-WIDE FREEZE TRIGGERED BY OPERATOR %s !!!", time.Now().Format(time.RFC3339), operatorID))
		return true, nil
	} else if action == "UNFREEZE" {
		cm.isEmergencyFrozen = false
		for _, v := range cm.vaults {
			v.IsLocked = false
		}
		cm.auditLogs = append(cm.auditLogs, fmt.Sprintf("[%s] EMERGENCY UNFREEZE: System operations successfully restored by Operator %s", time.Now().Format(time.RFC3339), operatorID))
		return false, nil
	}

	return false, fmt.Errorf("invalid emergency action: %s", action)
}

func (cm *CustodyManager) ListVaults() []*Vault {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var list []*Vault
	for _, v := range cm.vaults {
		list = append(list, v)
	}
	return list
}

func (cm *CustodyManager) ListTransfers() []*CustodyTransfer {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var list []*CustodyTransfer
	for _, t := range cm.transfers {
		list = append(list, t)
	}
	return list
}

func (cm *CustodyManager) LookupVault(id string) (*Vault, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	v, exists := cm.vaults[id]
	if !exists {
		return nil, fmt.Errorf("vault %s not found", id)
	}
	return v, nil
}

func (cm *CustodyManager) LookupTransfer(id string) (*CustodyTransfer, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	t, exists := cm.transfers[id]
	if !exists {
		return nil, fmt.Errorf("transfer %s not found", id)
	}
	return t, nil
}
