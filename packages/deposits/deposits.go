package deposits

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// DepositStatus represents the lifecycle of a blockchain asset deposit
type DepositStatus string

const (
	StatusPending   DepositStatus = "PENDING"
	StatusConfirmed DepositStatus = "CONFIRMED"
	StatusCompleted DepositStatus = "COMPLETED"
	StatusFailed    DepositStatus = "FAILED"
)

// DepositRecord represents an inbound token transaction tracked on the platform
type DepositRecord struct {
	ID                   string        `json:"id"`
	UserID               string        `json:"user_id"`
	Asset                string        `json:"asset"`
	Amount               float64       `json:"amount"`
	Fee                  float64       `json:"fee"`
	Address              string        `json:"address"`
	TxHash               string        `json:"tx_hash"`
	BlockNumber          int64         `json:"block_number"`
	Confirmations        int           `json:"confirmations"`
	RequiredConfirmations int           `json:"required_confirmations"`
	Status               DepositStatus `json:"status"`
	Message              string        `json:"message,omitempty"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
}

// BlockchainTx represents a raw blockchain transaction parsed from block streams
type BlockchainTx struct {
	TxHash      string    `json:"tx_hash"`
	Network     string    `json:"network"`
	Asset       string    `json:"asset"`
	Amount      float64   `json:"amount"`
	Sender      string    `json:"sender"`
	Receiver    string    `json:"receiver"`
	BlockNumber int64     `json:"block_number"`
	GasUsed     int64     `json:"gas_used,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// NetworkRules holds confirmation threshold parameters
type NetworkRules struct {
	RequiredConf int
	MinDeposit   float64
}

// DepositEngine manages thread-safe deposit queues, processing loops, and duplicate protections
type DepositEngine struct {
	mu              sync.RWMutex
	processedHashes map[string]bool
	depositQueue    []*DepositRecord
	records         map[string]*DepositRecord // key: ID
	recordsByTx     map[string]*DepositRecord // key: TxHash
	rules           map[string]NetworkRules   // key: Asset/Network
	orphans         map[string]*BlockchainTx  // key: TxHash (orphan txs)
	auditLogs       []string
}

// NewDepositEngine instantiates the multi-blockchain Deposit Processing system
func NewDepositEngine() *DepositEngine {
	engine := &DepositEngine{
		processedHashes: make(map[string]bool),
		depositQueue:    make([]*DepositRecord, 0),
		records:         make(map[string]*DepositRecord),
		recordsByTx:     make(map[string]*DepositRecord),
		orphans:         make(map[string]*BlockchainTx),
		rules:           make(map[string]NetworkRules),
		auditLogs:       make([]string, 0),
	}

	// Register Standard Confirmation & Limit Rules
	engine.rules["BTC"] = NetworkRules{RequiredConf: 6, MinDeposit: 0.0001}
	engine.rules["ETH"] = NetworkRules{RequiredConf: 12, MinDeposit: 0.01}
	engine.rules["USDT"] = NetworkRules{RequiredConf: 12, MinDeposit: 1.0}
	engine.rules["USDC"] = NetworkRules{RequiredConf: 12, MinDeposit: 1.0}
	engine.rules["BNB"] = NetworkRules{RequiredConf: 12, MinDeposit: 0.01}
	engine.rules["SOL"] = NetworkRules{RequiredConf: 1, MinDeposit: 0.1}
	engine.rules["TRX"] = NetworkRules{RequiredConf: 15, MinDeposit: 10.0}
	engine.rules["LTC"] = NetworkRules{RequiredConf: 6, MinDeposit: 0.01}

	return engine
}

// ValidateAndDetectIngress processes raw block transactions, ensuring replay protection and duplicate transaction blocking
func (de *DepositEngine) ValidateAndDetectIngress(ctx context.Context, tx *BlockchainTx, associatedUserID string) (*DepositRecord, error) {
	if tx == nil || tx.TxHash == "" {
		return nil, errors.New("empty transaction payload")
	}

	de.mu.Lock()
	defer de.mu.Unlock()

	// 1. Duplicate & Replay Protection Check
	if de.processedHashes[tx.TxHash] {
		return nil, fmt.Errorf("security violation: transaction %s is a duplicate / replay attack", tx.TxHash)
	}

	// 2. Fetch Confirmation rules
	rule, exists := de.rules[tx.Asset]
	if !exists {
		rule = NetworkRules{RequiredConf: 12, MinDeposit: 0.0} // Safe default
	}

	// 3. Lower limit boundary check
	if tx.Amount < rule.MinDeposit {
		return nil, fmt.Errorf("transaction amount %.4f below minimum threshold limit %.4f", tx.Amount, rule.MinDeposit)
	}

	// 4. Provision Deposit Record
	depID := fmt.Sprintf("dep_%d_%s", time.Now().UnixNano(), tx.TxHash[:8])
	rec := &DepositRecord{
		ID:                   depID,
		UserID:               associatedUserID,
		Asset:                tx.Asset,
		Amount:               tx.Amount,
		Fee:                  0.0,
		Address:              tx.Receiver,
		TxHash:               tx.TxHash,
		BlockNumber:          tx.BlockNumber,
		Confirmations:        0,
		RequiredConfirmations: rule.RequiredConf,
		Status:               StatusPending,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	// Save status mappings
	de.processedHashes[tx.TxHash] = true
	de.records[depID] = rec
	de.recordsByTx[tx.TxHash] = rec
	de.depositQueue = append(de.depositQueue, rec)

	de.auditLogs = append(de.auditLogs, fmt.Sprintf("[%s] INGRESS DETECTED: tx %s, amount %.4f, status PENDING", time.Now().Format(time.RFC3339), tx.TxHash, tx.Amount))

	return rec, nil
}

// IncrementConfirmations pushes confirmations count up, finalizes completed records
func (de *DepositEngine) IncrementConfirmations(ctx context.Context, txHash string, currentBlockHeight int64) (*DepositRecord, error) {
	de.mu.Lock()
	defer de.mu.Unlock()

	rec, exists := de.recordsByTx[txHash]
	if !exists {
		return nil, fmt.Errorf("deposit matching transaction hash %s not found", txHash)
	}

	if rec.Status == StatusCompleted || rec.Status == StatusFailed {
		return rec, nil // Already finalized
	}

	// Calculate current depth
	confirmations := int(currentBlockHeight - rec.BlockNumber + 1)
	if confirmations < 0 {
		confirmations = 0
	}
	rec.Confirmations = confirmations
	rec.UpdatedAt = time.Now()

	de.auditLogs = append(de.auditLogs, fmt.Sprintf("[%s] CONFIRMATIONS UPDATE: tx %s, confs %d/%d", time.Now().Format(time.RFC3339), txHash, rec.Confirmations, rec.RequiredConfirmations))

	// State progression logic
	if rec.Confirmations >= rec.RequiredConfirmations {
		rec.Status = StatusCompleted
		de.auditLogs = append(de.auditLogs, fmt.Sprintf("[%s] DEPOSIT FINALIZED: tx %s, user %s credited", time.Now().Format(time.RFC3339), txHash, rec.UserID))
	} else if rec.Confirmations > 0 {
		rec.Status = StatusConfirmed
	}

	return rec, nil
}

// HandleChainReorganization rolls back records within affected block ranges to protect against double spends
func (de *DepositEngine) HandleChainReorganization(ctx context.Context, forkBlockNumber int64) ([]*DepositRecord, error) {
	de.mu.Lock()
	defer de.mu.Unlock()

	de.auditLogs = append(de.auditLogs, fmt.Sprintf("[%s] WARNING: CHAIN REORG DETECTED AT BLOCK %d! Rolling back confirmations...", time.Now().Format(time.RFC3339), forkBlockNumber))

	var affected []*DepositRecord
	for _, rec := range de.records {
		if rec.BlockNumber >= forkBlockNumber {
			// Roll back status to Pending and reset confirmations
			rec.Confirmations = 0
			if rec.Status == StatusCompleted {
				// Important: Funds credit should be reversed in ledger!
				rec.Status = StatusPending
				rec.Message = "Rolled back due to chain reorg"
				affected = append(affected, rec)
				de.auditLogs = append(de.auditLogs, fmt.Sprintf("[%s] REORG ROLLBACK: Completed deposit %s reverted to PENDING", time.Now().Format(time.RFC3339), rec.ID))
			} else if rec.Status == StatusConfirmed {
				rec.Status = StatusPending
				affected = append(affected, rec)
			}
			rec.UpdatedAt = time.Now()
		}
	}

	return affected, nil
}

// HandleOrphanTransaction marks a deposit as FAILED if its block transaction was excluded or dropped
func (de *DepositEngine) HandleOrphanTransaction(ctx context.Context, txHash string) (*DepositRecord, error) {
	de.mu.Lock()
	defer de.mu.Unlock()

	rec, exists := de.recordsByTx[txHash]
	if !exists {
		return nil, fmt.Errorf("transaction %s not found in deposit engine", txHash)
	}

	rec.Status = StatusFailed
	rec.Confirmations = 0
	rec.Message = "Orphan transaction: excluded from main chain"
	rec.UpdatedAt = time.Now()

	de.auditLogs = append(de.auditLogs, fmt.Sprintf("[%s] ORPHAN TRANSACTION EXCLUSION: tx %s set to FAILED", time.Now().Format(time.RFC3339), txHash))

	return rec, nil
}

// LookupDeposit retrieves a record by deposit ID
func (de *DepositEngine) LookupDeposit(id string) (*DepositRecord, error) {
	de.mu.RLock()
	defer de.mu.RUnlock()

	rec, exists := de.records[id]
	if !exists {
		return nil, fmt.Errorf("deposit ID %s not found", id)
	}
	return rec, nil
}

// LookupByTxHash retrieves a record by blockchain transaction hash
func (de *DepositEngine) LookupByTxHash(txHash string) (*DepositRecord, error) {
	de.mu.RLock()
	defer de.mu.RUnlock()

	rec, exists := de.recordsByTx[txHash]
	if !exists {
		return nil, fmt.Errorf("transaction hash %s not found", txHash)
	}
	return rec, nil
}

// ListDeposits returns all deposit records managed by the engine
func (de *DepositEngine) ListDeposits() []*DepositRecord {
	de.mu.RLock()
	defer de.mu.RUnlock()

	list := make([]*DepositRecord, 0, len(de.records))
	for _, rec := range de.records {
		list = append(list, rec)
	}
	return list
}
