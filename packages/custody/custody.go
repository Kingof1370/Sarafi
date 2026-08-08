package custody

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"velyxora/packages/common"
	"velyxora/packages/database"
)

// VaultRole represents institutional vault categories
type VaultRole string

const (
	RoleHot      VaultRole = "HOT"
	RoleWarm     VaultRole = "WARM"
	RoleCold     VaultRole = "COLD"
	RoleTreasury VaultRole = "TREASURY"
)

// Vault represents segregated custody compartments
type Vault struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Role      VaultRole `json:"role" db:"role"`
	Asset     string    `json:"asset" db:"asset"`
	Address   string    `json:"address" db:"address"`
	Limit     float64   `json:"limit" db:"vault_limit"`
	Balance   float64   `json:"balance" db:"balance"`
	IsFrozen  bool      `json:"is_frozen" db:"is_frozen"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// CustodyPolicy defines the governance rules for withdrawals
type CustodyPolicy struct {
	Asset                 string  `json:"asset"`
	DualApprovalThreshold float64 `json:"dual_approval_threshold"`
	MaxSingleWithdrawal   float64 `json:"max_single_withdrawal"`
}

// CustodyApproval represents a multi-sig / dual approval request
type CustodyApproval struct {
	ID             string    `json:"id" db:"id"`
	WithdrawalID   string    `json:"withdrawal_id" db:"withdrawal_id"`
	Asset          string    `json:"asset" db:"asset"`
	Amount         float64   `json:"amount" db:"amount"`
	Status         string    `json:"status" db:"status"` // "PENDING", "APPROVED", "REJECTED"
	FirstApprover  string    `json:"first_approver" db:"first_approver"`
	SecondApprover string    `json:"second_approver" db:"second_approver"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// CustodyEngine coordinates dynamic institutional policy controls and approvals
type CustodyEngine struct {
	mu             sync.RWMutex
	db             *database.DB
	globalFreeze   bool
	policies       map[string]*CustodyPolicy
	vaults         map[string]*Vault
	localApprovals map[string]*CustodyApproval
}

// NewCustodyEngine instantiates the custody management orchestrator
func NewCustodyEngine(db *database.DB) *CustodyEngine {
	ce := &CustodyEngine{
		db:             db,
		globalFreeze:   false,
		policies:       make(map[string]*CustodyPolicy),
		vaults:         make(map[string]*Vault),
		localApprovals: make(map[string]*CustodyApproval),
	}

	// Seed default policies (e.g. BTC dual-approval threshold 1.0 BTC, ETH threshold 10.0 ETH)
	ce.policies["BTC"] = &CustodyPolicy{Asset: "BTC", DualApprovalThreshold: 1.0, MaxSingleWithdrawal: 10.0}
	ce.policies["ETH"] = &CustodyPolicy{Asset: "ETH", DualApprovalThreshold: 10.0, MaxSingleWithdrawal: 100.0}
	ce.policies["USDT"] = &CustodyPolicy{Asset: "USDT", DualApprovalThreshold: 5000.0, MaxSingleWithdrawal: 50000.0}
	ce.policies["SOL"] = &CustodyPolicy{Asset: "SOL", DualApprovalThreshold: 100.0, MaxSingleWithdrawal: 1000.0}

	return ce
}

// SetGlobalFreeze sets the global emergency freeze toggle state safely
func (ce *CustodyEngine) SetGlobalFreeze(frozen bool) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.globalFreeze = frozen

	if ce.db != nil {
		_, _ = ce.db.Pool.Exec(context.Background(),
			"INSERT INTO custody_emergency_logs (id, action, initiated_at) VALUES ($1, $2, NOW())",
			fmt.Sprintf("em_%d", time.Now().UnixNano()), fmt.Sprintf("FREEZE_%t", frozen))
	}
}

// IsFrozen returns whether the global freeze control is active
func (ce *CustodyEngine) IsFrozen() bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.globalFreeze
}

// ValidateWithdrawal checks policies to see if dual approval is required or if it exceeds max limits
func (ce *CustodyEngine) ValidateWithdrawal(asset string, amount float64) (bool, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	if ce.globalFreeze {
		return false, errors.New("emergency freeze is currently ACTIVE. All withdrawal transfers are blocked")
	}

	policy, ok := ce.policies[asset]
	if !ok {
		// Default strict policy for unregistered assets
		policy = &CustodyPolicy{Asset: asset, DualApprovalThreshold: 0.0, MaxSingleWithdrawal: 1000.0}
	}

	if amount > policy.MaxSingleWithdrawal {
		return false, fmt.Errorf("withdrawal amount %f exceeds maximum single withdrawal limit %f for %s", amount, policy.MaxSingleWithdrawal, asset)
	}

	// Returns true if dual approval is required
	return amount >= policy.DualApprovalThreshold, nil
}

// CreateApprovalRequest logs a pending custody approval
func (ce *CustodyEngine) CreateApprovalRequest(ctx context.Context, withdrawalID, asset string, amount float64) (*CustodyApproval, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	appID := fmt.Sprintf("app_%d", time.Now().UnixNano())
	approval := &CustodyApproval{
		ID:           appID,
		WithdrawalID: withdrawalID,
		Asset:        asset,
		Amount:       amount,
		Status:       "PENDING",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if ce.db != nil {
		_, err := ce.db.Pool.Exec(ctx,
			"INSERT INTO custody_approvals (id, withdrawal_id, asset, amount, status, first_approver, second_approver, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())",
			approval.ID, approval.WithdrawalID, approval.Asset, approval.Amount, approval.Status, "", "")
		if err != nil {
			return nil, fmt.Errorf("failed to persist custody approval: %w", err)
		}
	} else {
		ce.localApprovals[approval.ID] = approval
	}

	return approval, nil
}

// SubmitApproval processes an approval signature from an authorized administrator
func (ce *CustodyEngine) SubmitApproval(ctx context.Context, approvalID, approver string) (*CustodyApproval, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	var app *CustodyApproval

	if ce.db != nil {
		app = &CustodyApproval{}
		err := ce.db.Pool.QueryRow(ctx,
			"SELECT id, withdrawal_id, asset, amount, status, first_approver, second_approver, created_at, updated_at FROM custody_approvals WHERE id = $1",
			approvalID).Scan(&app.ID, &app.WithdrawalID, &app.Asset, &app.Amount, &app.Status, &app.FirstApprover, &app.SecondApprover, &app.CreatedAt, &app.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("custody approval request not found: %w", err)
		}
	} else {
		var ok bool
		app, ok = ce.localApprovals[approvalID]
		if !ok {
			return nil, errors.New("custody approval request not found in memory")
		}
	}

	if app.Status != "PENDING" {
		return nil, fmt.Errorf("approval request is already %s", app.Status)
	}

	// 4-Eyes Principle: Ensure the same person cannot approve twice
	if app.FirstApprover == "" {
		app.FirstApprover = approver
		app.UpdatedAt = time.Now()

		if ce.db != nil {
			_, err := ce.db.Pool.Exec(ctx,
				"UPDATE custody_approvals SET first_approver = $1, updated_at = NOW() WHERE id = $2",
				approver, approvalID)
			if err != nil {
				return nil, err
			}
		}
	} else {
		if app.FirstApprover == approver {
			return nil, errors.New("multi-sig validation violation: second approver must be different from first approver (4-eyes principle)")
		}

		app.SecondApprover = approver
		app.Status = "APPROVED"
		app.UpdatedAt = time.Now()

		if ce.db != nil {
			_, err := ce.db.Pool.Exec(ctx,
				"UPDATE custody_approvals SET second_approver = $1, status = 'APPROVED', updated_at = NOW() WHERE id = $2",
				approver, approvalID)
			if err != nil {
				return nil, err
			}
		}
	}

	return app, nil
}

// GetVaultBalance returns balance in individual segregated vault compartment
func (ce *CustodyEngine) GetVaultBalance(role VaultRole, asset string) float64 {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	if ce.db != nil {
		var bal float64
		err := ce.db.Pool.QueryRow(context.Background(),
			"SELECT COALESCE(SUM(balance), 0.0) FROM custody_vaults WHERE role = $1 AND asset = $2",
			string(role), asset).Scan(&bal)
		if err == nil {
			return common.RoundToPrecision(bal, 8)
		}
	}

	// Fallback mock balance sheet per vault
	switch role {
	case RoleHot:
		return 100.0
	case RoleWarm:
		return 500.0
	case RoleCold:
		return 5000.0
	case RoleTreasury:
		return 20000.0
	}
	return 0.0
}
