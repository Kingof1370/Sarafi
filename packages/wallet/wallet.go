package wallet

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"velyxora/packages/address"
	"velyxora/packages/ledger-common"
)

// WalletType defines the specific operational role and accessibility profile of a wallet
type WalletType string

const (
	TypeHot         WalletType = "HOT"
	TypeWarm        WalletType = "WARM"
	TypeCold        WalletType = "COLD"
	TypeTreasury    WalletType = "TREASURY"
	TypeOperational WalletType = "OPERATIONAL"
	TypeFee         WalletType = "FEE"
	TypeReserve     WalletType = "RESERVE"
	TypeRecovery    WalletType = "RECOVERY"
)

// Balance represents the multi-dimensional financial segments held by a wallet for a specific asset
type Balance struct {
	Asset     string    `json:"asset"`
	Available float64   `json:"available"`
	Locked    float64   `json:"locked"`
	Reserved  float64   `json:"reserved"`
	Pending   float64   `json:"pending"`
	Total     float64   `json:"total"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BalanceHistory represents a single snapshot of a balance segment at a specific point in time
type BalanceHistory struct {
	ID        string    `json:"id"`
	WalletID  string    `json:"wallet_id"`
	Asset     string    `json:"asset"`
	Available float64   `json:"available"`
	Locked    float64   `json:"locked"`
	Reserved  float64   `json:"reserved"`
	Pending   float64   `json:"pending"`
	Total     float64   `json:"total"`
	Timestamp time.Time `json:"timestamp"`
}

// Wallet defines the core domain model of an Enterprise Wallet
type Wallet struct {
	ID        string           `json:"id"`
	UserID    string           `json:"user_id"`
	Type      WalletType       `json:"type"`
	Addresses []*address.AddressRecord `json:"addresses"`
	Balances  map[string]*Balance `json:"balances"` // key: asset symbol
	IsLocked  bool             `json:"is_locked"`
	CreatedAt time.Time        `json:"created_at"`
}

// OperationalLimit defines safety boundaries for transactional processing
type OperationalLimit struct {
	WalletID      string  `json:"wallet_id"`
	MaxTxLimit    float64 `json:"max_tx_limit"`
	DailyTxLimit  float64 `json:"daily_tx_limit"`
	DailySpent    float64 `json:"daily_spent"`
	ResetInterval time.Time `json:"reset_interval"`
}

// WalletManager implements thread-safe enterprise business operations and access-control rules
type WalletManager struct {
	mu            sync.RWMutex
	wallets       map[string]*Wallet
	limits        map[string]*OperationalLimit
	auditTrail    []*ledger_common.WalletAuditRecord
	approvalsQueue map[string][]string // key: transferID, value: list of admins who approved
}

// NewWalletManager initializes the primary wallet orchestration controller
func NewWalletManager() *WalletManager {
	return &WalletManager{
		wallets:       make(map[string]*Wallet),
		limits:        make(map[string]*OperationalLimit),
		auditTrail:    make([]*ledger_common.WalletAuditRecord, 0),
		approvalsQueue: make(map[string][]string),
	}
}

// ProvisionWallet creates a new managed wallet profile
func (wm *WalletManager) ProvisionWallet(walletID, userID string, wType WalletType) (*Wallet, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if _, exists := wm.wallets[walletID]; exists {
		return nil, fmt.Errorf("wallet %s already exists", walletID)
	}

	w := &Wallet{
		ID:        walletID,
		UserID:    userID,
		Type:      wType,
		Addresses: make([]*address.AddressRecord, 0),
		Balances:  make(map[string]*Balance),
		IsLocked:  false,
		CreatedAt: time.Now(),
	}

	wm.wallets[walletID] = w

	// Track in audit logs
	wm.recordAudit(userID, walletID, "", ledger_common.ActionWalletCreated, 0, 0, 0, "Wallet provisioned successfully")

	return w, nil
}

// GetWallet retrieves a wallet thread-safely
func (wm *WalletManager) GetWallet(walletID string) (*Wallet, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	w, exists := wm.wallets[walletID]
	if !exists {
		return nil, fmt.Errorf("wallet %s not found", walletID)
	}
	return w, nil
}

// SetLimit configures transaction thresholds for Operational wallets
func (wm *WalletManager) SetLimit(walletID string, maxLimit, dailyLimit float64) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	w, exists := wm.wallets[walletID]
	if !exists {
		return fmt.Errorf("wallet %s not found", walletID)
	}

	if w.Type != TypeOperational {
		return errors.New("limits can only be set on OPERATIONAL wallets")
	}

	wm.limits[walletID] = &OperationalLimit{
		WalletID:      walletID,
		MaxTxLimit:    maxLimit,
		DailyTxLimit:  dailyLimit,
		DailySpent:    0,
		ResetInterval: time.Now().Add(24 * time.Hour),
	}

	return nil
}

// InitiateTransfer routes funds securely using multi-stage checks based on wallet roles
func (wm *WalletManager) InitiateTransfer(txID string, sourceWalletID, destWalletID string, asset string, amount float64, requesterID string) (bool, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	src, exists := wm.wallets[sourceWalletID]
	if !exists {
		return false, fmt.Errorf("source wallet %s not found", sourceWalletID)
	}

	dst, exists := wm.wallets[destWalletID]
	if !exists {
		return false, fmt.Errorf("destination wallet %s not found", destWalletID)
	}

	if src.IsLocked || dst.IsLocked {
		return false, errors.New("transaction rejected: one or both wallets are currently locked")
	}

	// 1. Fetch source balance
	bal, hasBal := src.Balances[asset]
	if !hasBal || bal.Available < amount {
		return false, errors.New("insufficient available balance")
	}

	// 2. Cold Wallet rules: Must never be directly accessed/moved without multi-stage authorization
	if src.Type == TypeCold {
		return false, fmt.Errorf("transfer from COLD wallet %s is pending multi-stage authorization. Transfer ID: %s", sourceWalletID, txID)
	}

	// 3. Treasury Wallet rules: Elevated authorizations required (simulated by multi-signature/approvals queue)
	if src.Type == TypeTreasury {
		approvals := wm.approvalsQueue[txID]
		if len(approvals) < 2 {
			return false, fmt.Errorf("transfer from TREASURY wallet %s requires at least 2 administrative approvals. Current approvals: %d", sourceWalletID, len(approvals))
		}
	}

	// 4. Operational limits checks
	if src.Type == TypeOperational {
		limit, hasLimit := wm.limits[sourceWalletID]
		if hasLimit {
			// Daily limit reset check
			if time.Now().After(limit.ResetInterval) {
				limit.DailySpent = 0
				limit.ResetInterval = time.Now().Add(24 * time.Hour)
			}

			if amount > limit.MaxTxLimit {
				return false, fmt.Errorf("transaction amount %.4f exceeds maximum single limit of %.4f", amount, limit.MaxTxLimit)
			}
			if limit.DailySpent+amount > limit.DailyTxLimit {
				return false, fmt.Errorf("transaction exceeds daily spending limit of %.4f. Spent today: %.4f", limit.DailyTxLimit, limit.DailySpent)
			}

			limit.DailySpent += amount
		}
	}

	// 5. Execute Double Entry Transfer
	prevSrc := bal.Available
	bal.Available -= amount
	bal.Total -= amount
	bal.UpdatedAt = time.Now()

	dstBal, hasDstBal := dst.Balances[asset]
	if !hasDstBal {
		dstBal = &Balance{
			Asset:     asset,
			Available: 0,
			Total:     0,
			UpdatedAt: time.Now(),
		}
		dst.Balances[asset] = dstBal
	}

	prevDst := dstBal.Available
	dstBal.Available += amount
	dstBal.Total += amount
	dstBal.UpdatedAt = time.Now()

	// 6. Record Chain-hashes inside audits
	wm.recordAudit(src.UserID, sourceWalletID, asset, ledger_common.ActionBalanceAdjusted, amount, prevSrc, bal.Available, fmt.Sprintf("Transferred to %s", destWalletID))
	wm.recordAudit(dst.UserID, destWalletID, asset, ledger_common.ActionDepositReceived, amount, prevDst, dstBal.Available, fmt.Sprintf("Received from %s", sourceWalletID))

	return true, nil
}

// ApproveTransfer registers administrative approvals for high-security transfers (e.g. Treasury and Cold)
func (wm *WalletManager) ApproveTransfer(txID string, adminID string) int {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	approvals := wm.approvalsQueue[txID]
	for _, app := range approvals {
		if app == adminID {
			return len(approvals) // Already approved by this admin
		}
	}

	approvals = append(approvals, adminID)
	wm.approvalsQueue[txID] = approvals
	return len(approvals)
}

// UpdateBalance directly modifies balance segments (e.g., locking/reserving funds during trading operations)
func (wm *WalletManager) UpdateBalance(walletID string, asset string, availableDelta, lockedDelta, reservedDelta, pendingDelta float64) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	w, exists := wm.wallets[walletID]
	if !exists {
		return fmt.Errorf("wallet %s not found", walletID)
	}

	bal, exists := w.Balances[asset]
	if !exists {
		bal = &Balance{
			Asset:     asset,
			Available: 0,
			Locked:    0,
			Reserved:  0,
			Pending:   0,
			Total:     0,
			UpdatedAt: time.Now(),
		}
		w.Balances[asset] = bal
	}

	prevAvail := bal.Available

	bal.Available += availableDelta
	bal.Locked += lockedDelta
	bal.Reserved += reservedDelta
	bal.Pending += pendingDelta

	bal.Total = bal.Available + bal.Locked + bal.Reserved + bal.Pending
	bal.UpdatedAt = time.Now()

	if bal.Available < 0 || bal.Locked < 0 || bal.Reserved < 0 || bal.Pending < 0 {
		return errors.New("safety violation: negative balance segments are forbidden")
	}

	wm.recordAudit(w.UserID, walletID, asset, ledger_common.ActionBalanceAdjusted, availableDelta, prevAvail, bal.Available, "Balance update processed")
	return nil
}

// recordAudit appends and cryptographically chains a new record into audit logs
func (wm *WalletManager) recordAudit(userID, walletID, asset string, action ledger_common.AuditAction, amount, prev, newB float64, msg string) {
	var prevHash string
	if len(wm.auditTrail) > 0 {
		prevHash = wm.auditTrail[len(wm.auditTrail)-1].Hash
	}

	rec := &ledger_common.WalletAuditRecord{
		ID:          fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		UserID:      userID,
		WalletID:    walletID,
		Asset:       asset,
		Action:      action,
		Amount:      amount,
		PrevBalance: prev,
		NewBalance:  newB,
		Message:     msg,
		PrevHash:    prevHash,
		Timestamp:   time.Now(),
	}
	rec.Hash = rec.ComputeHash()
	wm.auditTrail = append(wm.auditTrail, rec)
}

// ListAuditLogs returns the platform wallet logs
func (wm *WalletManager) ListAuditLogs() []*ledger_common.WalletAuditRecord {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	list := make([]*ledger_common.WalletAuditRecord, len(wm.auditTrail))
	copy(list, wm.auditTrail)
	return list
}
