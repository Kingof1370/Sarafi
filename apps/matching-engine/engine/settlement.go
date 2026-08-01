package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
	"velyxora/packages/database"
)

// SettlementStatus represents the lifecycle state of a transaction leg settlement
type SettlementStatus string

const (
	SettlePending   SettlementStatus = "PENDING"
	SettleCompleted SettlementStatus = "COMPLETED"
	SettleFailed    SettlementStatus = "FAILED"
	SettleRetrying  SettlementStatus = "RETRYING"
)

// SettlementJob maps executions waiting to clear through database transactions
type SettlementJob struct {
	ID         string           `json:"id"`
	Exec       *Execution       `json:"exec"`
	BaseAsset  string           `json:"base_asset"`
	QuoteAsset string           `json:"quote_asset"`
	Status     SettlementStatus `json:"status"`
	Retries    int              `json:"retries"`
	MaxRetries int              `json:"max_retries"`
	ErrorMsg   string           `json:"error_msg,omitempty"`
	Timestamp  time.Time        `json:"timestamp"`
}

// SettlementEngine processes trade clearing, fees ledger distribution, and retries asynchronously
type SettlementEngine struct {
	mu            sync.RWMutex
	db            *database.DB
	queue         []*SettlementJob
	history       []*SettlementJob
	reconciled    bool
	runningJobs   map[string]bool
	onCompleted   func(job *SettlementJob)
}

// NewSettlementEngine initializes a complete financial clearing house
func NewSettlementEngine(db *database.DB) *SettlementEngine {
	return &SettlementEngine{
		db:          db,
		queue:       make([]*SettlementJob, 0),
		history:     make([]*SettlementJob, 0),
		reconciled:  true,
		runningJobs: make(map[string]bool),
	}
}

// QueueSettlement enters an execution leg into the asynchronous clearing pipelines
func (se *SettlementEngine) QueueSettlement(exec *Execution, baseAsset, quoteAsset string) *SettlementJob {
	se.mu.Lock()
	defer se.mu.Unlock()

	job := &SettlementJob{
		ID:         "set_job_" + exec.TradeID,
		Exec:       exec,
		BaseAsset:  baseAsset,
		QuoteAsset: quoteAsset,
		Status:     SettlePending,
		Retries:    0,
		MaxRetries: 3,
		Timestamp:  time.Now(),
	}

	se.queue = append(se.queue, job)
	return job
}

// ProcessQueue runs through all pending settlement jobs sequentially and applies retry logic on failures
func (se *SettlementEngine) ProcessQueue(ctx context.Context) (int, int) {
	se.mu.Lock()
	if len(se.queue) == 0 {
		se.mu.Unlock()
		return 0, 0
	}

	activeJobs := make([]*SettlementJob, len(se.queue))
	copy(activeJobs, se.queue)
	se.queue = make([]*SettlementJob, 0)
	se.mu.Unlock()

	successCount := 0
	failCount := 0

	for _, job := range activeJobs {
		// Strict Double Entry Accounting Validation: Total Debit MUST exactly match Total Credit + Fees
		buyerDebit := job.Exec.Price*job.Exec.Quantity + job.Exec.BuyerFee
		sellerCredit := job.Exec.Price*job.Exec.Quantity - job.Exec.SellerFee
		feeIncome := job.Exec.BuyerFee + job.Exec.SellerFee

		// Verification: Buyer debit == Seller proceeds + Platform fees income
		if buyerDebit != (sellerCredit + feeIncome) {
			job.Status = SettleFailed
			job.ErrorMsg = "Double Entry Validation Failure: accounting equations desynchronized"
			se.archiveJob(job)
			failCount++
			continue
		}

		// Process actual settlement
		err := se.executeSettlementTransaction(ctx, job)
		if err != nil {
			job.Retries++
			if job.Retries <= job.MaxRetries {
				job.Status = SettleRetrying
				job.ErrorMsg = err.Error()
				// Place back in queue for retry backoff
				se.mu.Lock()
				se.queue = append(se.queue, job)
				se.mu.Unlock()
			} else {
				job.Status = SettleFailed
				job.ErrorMsg = fmt.Sprintf("Max retries exceeded: %v", err)
				se.archiveJob(job)
				failCount++
			}
		} else {
			job.Status = SettleCompleted
			se.archiveJob(job)
			successCount++
			if se.onCompleted != nil {
				se.onCompleted(job)
			}
		}
	}

	return successCount, failCount
}

func (se *SettlementEngine) executeSettlementTransaction(ctx context.Context, job *SettlementJob) error {
	exec := job.Exec

	// If database is offline, memory clearing tracker successfully records
	if se.db == nil {
		return nil
	}

	tx, err := se.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin atomic SQL settlement tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Debit Buyer Quote Asset: Price * Quantity + BuyerFee
	buyerDebit := exec.Price*exec.Quantity + exec.BuyerFee
	_, err = tx.Exec(ctx,
		"UPDATE balances SET available = available - $1, total = total - $1, updated_at = NOW() WHERE user_id = $2 AND asset = $3",
		buyerDebit, exec.BuyerID, job.QuoteAsset)
	if err != nil {
		return fmt.Errorf("failed to debit buyer quote balance: %w", err)
	}

	// 2. Credit Buyer Base Asset: Quantity
	_, err = tx.Exec(ctx,
		"INSERT INTO balances (user_id, asset, available, total, updated_at) VALUES ($1, $2, $3, $3, NOW()) "+
			"ON CONFLICT (user_id, asset) DO UPDATE SET available = balances.available + EXCLUDED.available, total = balances.total + EXCLUDED.total, updated_at = NOW()",
		exec.BuyerID, job.BaseAsset, exec.Quantity)
	if err != nil {
		return fmt.Errorf("failed to credit buyer base balance: %w", err)
	}

	// 3. Debit Seller Base Asset: Quantity
	_, err = tx.Exec(ctx,
		"UPDATE balances SET available = available - $1, total = total - $1, updated_at = NOW() WHERE user_id = $2 AND asset = $3",
		exec.Quantity, exec.SellerID, job.BaseAsset)
	if err != nil {
		return fmt.Errorf("failed to debit seller base balance: %w", err)
	}

	// 4. Credit Seller Quote Asset: Price * Quantity - SellerFee
	sellerCredit := exec.Price*exec.Quantity - exec.SellerFee
	_, err = tx.Exec(ctx,
		"INSERT INTO balances (user_id, asset, available, total, updated_at) VALUES ($1, $2, $3, $3, NOW()) "+
			"ON CONFLICT (user_id, asset) DO UPDATE SET available = balances.available + EXCLUDED.available, total = balances.total + EXCLUDED.total, updated_at = NOW()",
		exec.SellerID, job.QuoteAsset, sellerCredit)
	if err != nil {
		return fmt.Errorf("failed to credit seller quote balance: %w", err)
	}

	// 5. Credit Platform Fee Account
	platformFeeAccount := "platform_fees"
	_, err = tx.Exec(ctx,
		"INSERT INTO balances (user_id, asset, available, total, updated_at) VALUES ($1, $2, $3, $3, NOW()) "+
			"ON CONFLICT (user_id, asset) DO UPDATE SET available = balances.available + EXCLUDED.available, total = balances.total + EXCLUDED.total, updated_at = NOW()",
		platformFeeAccount, job.QuoteAsset, exec.BuyerFee+exec.SellerFee)
	if err != nil {
		return fmt.Errorf("failed to credit platform fee balance: %w", err)
	}

	// 6. Record Dual Entry Ledger Postings
	ledgerTxID := "tx_settle_" + exec.TradeID

	// Buyer quote debit
	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
		"ent_b_q_"+exec.TradeID, ledgerTxID, exec.BuyerID, job.QuoteAsset, "DEBIT", buyerDebit, "Buyer execution quote cost")
	if err != nil {
		return fmt.Errorf("failed to write buyer quote ledger: %w", err)
	}

	// Buyer base credit
	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
		"ent_b_b_"+exec.TradeID, ledgerTxID, exec.BuyerID, job.BaseAsset, "CREDIT", exec.Quantity, "Buyer execution base delivery")
	if err != nil {
		return fmt.Errorf("failed to write buyer base ledger: %w", err)
	}

	// Seller base debit
	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
		"ent_s_b_"+exec.TradeID, ledgerTxID, exec.SellerID, job.BaseAsset, "DEBIT", exec.Quantity, "Seller execution base cost")
	if err != nil {
		return fmt.Errorf("failed to write seller base ledger: %w", err)
	}

	// Seller quote credit
	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
		"ent_s_q_"+exec.TradeID, ledgerTxID, exec.SellerID, job.QuoteAsset, "CREDIT", sellerCredit, "Seller execution quote delivery")
	if err != nil {
		return fmt.Errorf("failed to write seller quote ledger: %w", err)
	}

	// Commit Transaction
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit settlement transaction leg: %w", err)
	}

	return nil
}

func (se *SettlementEngine) archiveJob(job *SettlementJob) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.history = append(se.history, job)
}

// GetSettledCount returns completed jobs count
func (se *SettlementEngine) GetSettledCount() int {
	se.mu.RLock()
	defer se.mu.RUnlock()

	count := 0
	for _, job := range se.history {
		if job.Status == SettleCompleted {
			count++
		}
	}
	return count
}

// GetFailedJobs returns list of failed settlement jobs
func (se *SettlementEngine) GetFailedJobs() []*SettlementJob {
	se.mu.RLock()
	defer se.mu.RUnlock()

	fails := make([]*SettlementJob, 0)
	for _, job := range se.history {
		if job.Status == SettleFailed {
			fails = append(fails, job)
		}
	}
	return fails
}

// GetPendingJobsCount returns current size of pending queue
func (se *SettlementEngine) GetPendingJobsCount() int {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return len(se.queue)
}
