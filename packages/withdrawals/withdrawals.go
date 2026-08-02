package withdrawals

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// WithdrawalStatus defines the lifecycle states of a withdrawal transaction
type WithdrawalStatus string

const (
	StatusRequested WithdrawalStatus = "REQUESTED"
	StatusValidated WithdrawalStatus = "VALIDATED"
	StatusApproved  WithdrawalStatus = "APPROVED"
	StatusRejected  WithdrawalStatus = "REJECTED"
	StatusBroadcast WithdrawalStatus = "BROADCAST"
	StatusConfirmed WithdrawalStatus = "CONFIRMED"
	StatusCompleted WithdrawalStatus = "COMPLETED"
	StatusFailed    WithdrawalStatus = "FAILED"
)

// WithdrawalRequest represents a single user withdrawal session on the platform
type WithdrawalRequest struct {
	ID          string           `json:"id"`
	UserID      string           `json:"user_id"`
	Asset       string           `json:"asset"`
	Amount      float64          `json:"amount"`
	Fee         float64          `json:"fee"`
	Address     string           `json:"address"`
	TxHash      string           `json:"tx_hash,omitempty"`
	Status      WithdrawalStatus `json:"status"`
	RiskScore   float64          `json:"risk_score"`
	Message     string           `json:"message,omitempty"`
	Retries     int              `json:"retries"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// AddressBookRecord represents a verified destination key address whitelisted by the user
type AddressBookRecord struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Address       string    `json:"address"`
	Network       string    `json:"network"`
	Label         string    `json:"label"`
	IsWhitelisted bool      `json:"is_whitelisted"`
	TrustScore    float64   `json:"trust_score"`
	VerifiedAt    time.Time `json:"verified_at"`
}

// SecurityLimits holds threshold values for risk-based validation
type SecurityLimits struct {
	DailyLimit   float64
	SingleLimit  float64
	CooldownMins int
}

// WithdrawalEngine coordinates validation, risk assessments, whitelists, queues, and broadcasts
type WithdrawalEngine struct {
	mu           sync.RWMutex
	requests     map[string]*WithdrawalRequest // key: ID
	addressBook  map[string]*AddressBookRecord // key: UserID_Address
	limits       map[string]SecurityLimits     // key: Asset
	cooldowns    map[string]time.Time          // key: UserID_Asset
	dailySpent   map[string]float64            // key: UserID_Asset
	approvals    map[string][]string           // key: WithdrawalID, value: AdminIDs who approved
	auditLogs    []string
}

// NewWithdrawalEngine instantiates a clean Withdrawal subsystem
func NewWithdrawalEngine() *WithdrawalEngine {
	engine := &WithdrawalEngine{
		requests:    make(map[string]*WithdrawalRequest),
		addressBook: make(map[string]*AddressBookRecord),
		limits:      make(map[string]SecurityLimits),
		cooldowns:   make(map[string]time.Time),
		dailySpent:  make(map[string]float64),
		approvals:   make(map[string][]string),
		auditLogs:   make([]string, 0),
	}

	// Register Standard Safety Threshold Rules per Asset
	engine.limits["BTC"] = SecurityLimits{DailyLimit: 2.0, SingleLimit: 0.5, CooldownMins: 30}
	engine.limits["ETH"] = SecurityLimits{DailyLimit: 20.0, SingleLimit: 5.0, CooldownMins: 30}
	engine.limits["USDT"] = SecurityLimits{DailyLimit: 50000.0, SingleLimit: 10000.0, CooldownMins: 15}
	engine.limits["USDC"] = SecurityLimits{DailyLimit: 50000.0, SingleLimit: 10000.0, CooldownMins: 15}

	return engine
}

// CreateWithdrawal session validates security rules and queues standard withdrawal requests
func (we *WithdrawalEngine) CreateWithdrawal(ctx context.Context, userID, asset string, amount float64, fee float64, address string) (*WithdrawalRequest, error) {
	we.mu.Lock()
	defer we.mu.Unlock()

	// 1. Withdrawal Address Book & Whitelist Check
	addrKey := fmt.Sprintf("%s_%s", userID, address)
	bookRec, whitelisted := we.addressBook[addrKey]
	if !whitelisted || !bookRec.IsWhitelisted {
		return nil, fmt.Errorf("security violation: destination address %s is not whitelisted", address)
	}

	// 2. Cooldown Check
	cooldownKey := fmt.Sprintf("%s_%s", userID, asset)
	if exp, exists := we.cooldowns[cooldownKey]; exists && time.Now().Before(exp) {
		return nil, fmt.Errorf("security violation: withdrawal cooldown active. Try again after %v", exp.Format(time.RFC3339))
	}

	// 3. Asset Limit Checks
	limit, hasLimit := we.limits[asset]
	if hasLimit {
		if amount > limit.SingleLimit {
			return nil, fmt.Errorf("transaction limit exceeded: max single limit %.4f, requested %.4f", limit.SingleLimit, amount)
		}

		spentKey := fmt.Sprintf("%s_%s", userID, asset)
		todaySpent := we.dailySpent[spentKey]
		if todaySpent+amount > limit.DailyLimit {
			return nil, fmt.Errorf("daily velocity limit exceeded: max daily %.4f, spent today %.4f, requested %.4f", limit.DailyLimit, todaySpent, amount)
		}

		// Update Daily cumulative spent and set Cooldown
		we.dailySpent[spentKey] = todaySpent + amount
		we.cooldowns[cooldownKey] = time.Now().Add(time.Duration(limit.CooldownMins) * time.Minute)
	}

	// 4. Calculate Risk Score
	riskScore := 0.1 // Safe standard baseline
	if bookRec.TrustScore < 0.5 {
		riskScore = 0.8 // High Risk flag
	}

	reqID := fmt.Sprintf("wth_%d_%s", time.Now().UnixNano(), asset)
	req := &WithdrawalRequest{
		ID:        reqID,
		UserID:    userID,
		Asset:     asset,
		Amount:    amount,
		Fee:       fee,
		Address:   address,
		Status:    StatusRequested,
		RiskScore: riskScore,
		Retries:   0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	we.requests[reqID] = req
	we.auditLogs = append(we.auditLogs, fmt.Sprintf("[%s] WITHDRAWAL REQUESTED: ID %s, user %s, amount %.4f %s, status REQUESTED", time.Now().Format(time.RFC3339), reqID, userID, amount, asset))

	return req, nil
}

// AddAddressToBook verifies and inserts a trusted destination address to the user whitelists
func (we *WithdrawalEngine) AddAddressToBook(userID, address, network, label string, isWhitelisted bool) *AddressBookRecord {
	we.mu.Lock()
	defer we.mu.Unlock()

	key := fmt.Sprintf("%s_%s", userID, address)
	rec := &AddressBookRecord{
		ID:            fmt.Sprintf("adr_bk_%d", time.Now().UnixNano()),
		UserID:        userID,
		Address:       address,
		Network:       network,
		Label:         label,
		IsWhitelisted: isWhitelisted,
		TrustScore:    1.0,
		VerifiedAt:    time.Now(),
	}
	we.addressBook[key] = rec
	return rec
}

// ApproveWithdrawal registers administrative approvals, updating statuses on threshold limits
func (we *WithdrawalEngine) ApproveWithdrawal(ctx context.Context, reqID, adminID string) (*WithdrawalRequest, error) {
	we.mu.Lock()
	defer we.mu.Unlock()

	req, exists := we.requests[reqID]
	if !exists {
		return nil, fmt.Errorf("withdrawal request %s not found", reqID)
	}

	if req.Status != StatusRequested && req.Status != StatusValidated {
		return req, nil // Already approved/rejected/finalized
	}

	// Avoid duplicates
	approvalsList := we.approvals[reqID]
	alreadyApproved := false
	for _, app := range approvalsList {
		if app == adminID {
			alreadyApproved = true
			break
		}
	}

	if !alreadyApproved {
		approvalsList = append(approvalsList, adminID)
		we.approvals[reqID] = approvalsList
	}

	// If High Risk (RiskScore > 0.5), require 2 admin approvals
	requiredApprovals := 1
	if req.RiskScore > 0.5 {
		requiredApprovals = 2
	}

	if len(approvalsList) >= requiredApprovals {
		req.Status = StatusApproved
		req.UpdatedAt = time.Now()
		we.auditLogs = append(we.auditLogs, fmt.Sprintf("[%s] WITHDRAWAL APPROVED: ID %s, admin %s, threshold reached (%d/%d)", time.Now().Format(time.RFC3339), reqID, adminID, len(approvalsList), requiredApprovals))
	} else {
		req.Status = StatusValidated
		req.UpdatedAt = time.Now()
		we.auditLogs = append(we.auditLogs, fmt.Sprintf("[%s] WITHDRAWAL PARTIALLY APPROVED: ID %s, admin %s (%d/%d required)", time.Now().Format(time.RFC3339), reqID, adminID, len(approvalsList), requiredApprovals))
	}

	return req, nil
}

// CancelWithdrawal allows the user or support to abort requested sessions before broadcasting
func (we *WithdrawalEngine) CancelWithdrawal(ctx context.Context, reqID string) (*WithdrawalRequest, error) {
	we.mu.Lock()
	defer we.mu.Unlock()

	req, exists := we.requests[reqID]
	if !exists {
		return nil, fmt.Errorf("withdrawal request %s not found", reqID)
	}

	if req.Status != StatusRequested && req.Status != StatusValidated {
		return nil, errors.New("cannot cancel a finalized, approved, or broadcasting transaction")
	}

	req.Status = StatusRejected
	req.Message = "Canceled by user"
	req.UpdatedAt = time.Now()

	// Release daily spent limits
	spentKey := fmt.Sprintf("%s_%s", req.UserID, req.Asset)
	we.dailySpent[spentKey] -= req.Amount

	we.auditLogs = append(we.auditLogs, fmt.Sprintf("[%s] WITHDRAWAL CANCELED: ID %s, released limits", time.Now().Format(time.RFC3339), reqID))

	return req, nil
}

// BroadcastTransaction simulates broadcasting raw hex to nodes, assigning TxHash
func (we *WithdrawalEngine) BroadcastTransaction(ctx context.Context, reqID string, hexPayload string) (*WithdrawalRequest, error) {
	we.mu.Lock()
	defer we.mu.Unlock()

	req, exists := we.requests[reqID]
	if !exists {
		return nil, fmt.Errorf("withdrawal request %s not found", reqID)
	}

	if req.Status != StatusApproved {
		return nil, errors.New("only approved withdrawals can be broadcasted")
	}

	req.Status = StatusBroadcast
	req.TxHash = fmt.Sprintf("0xhash_broad_%d_%s", time.Now().UnixNano(), req.Asset)
	req.UpdatedAt = time.Now()

	we.auditLogs = append(we.auditLogs, fmt.Sprintf("[%s] TRANSACTION BROADCAST: ID %s, assigned tx %s", time.Now().Format(time.RFC3339), reqID, req.TxHash))

	return req, nil
}

// FinalizeWithdrawal sets status to COMPLETED upon successful network confirmations
func (we *WithdrawalEngine) FinalizeWithdrawal(ctx context.Context, reqID string) (*WithdrawalRequest, error) {
	we.mu.Lock()
	defer we.mu.Unlock()

	req, exists := we.requests[reqID]
	if !exists {
		return nil, fmt.Errorf("withdrawal request %s not found", reqID)
	}

	if req.Status != StatusBroadcast {
		return req, nil // Already finalized
	}

	req.Status = StatusCompleted
	req.UpdatedAt = time.Now()

	we.auditLogs = append(we.auditLogs, fmt.Sprintf("[%s] WITHDRAWAL COMPLETED: ID %s finalized on-chain", time.Now().Format(time.RFC3339), reqID))

	return req, nil
}

// GetRequest retrieves standard session details
func (we *WithdrawalEngine) GetRequest(id string) (*WithdrawalRequest, error) {
	we.mu.RLock()
	defer we.mu.RUnlock()

	req, exists := we.requests[id]
	if !exists {
		return nil, fmt.Errorf("withdrawal ID %s not found", id)
	}
	return req, nil
}

// ListAddressBook returns user address book whitelists
func (we *WithdrawalEngine) ListAddressBook(userID string) []*AddressBookRecord {
	we.mu.RLock()
	defer we.mu.RUnlock()

	var records []*AddressBookRecord
	for _, rec := range we.addressBook {
		if rec.UserID == userID {
			records = append(records, rec)
		}
	}
	return records
}

// ListRequests returns all active requests
func (we *WithdrawalEngine) ListRequests() []*WithdrawalRequest {
	we.mu.RLock()
	defer we.mu.RUnlock()

	var list []*WithdrawalRequest
	for _, r := range we.requests {
		list = append(list, r)
	}
	return list
}
