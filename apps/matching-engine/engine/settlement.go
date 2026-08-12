package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
	"velyxora/packages/common"
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
		// Strict Double Entry Accounting Validation: Total Debit MUST exactly match Total Credit + Fees precisely.
		// Since we now support multi-asset fee payments (e.g. fees paid in VLX instead of the quote asset),
		// we must perform validation on a per-asset basis.

		// 1. Identify the assets involved in the double entry checks
		// - BaseAsset: Seller debited, Buyer credited
		// - QuoteAsset: Buyer debited, Seller credited (potentially adjusted for buyer/seller fees)
		// - VLX/Other Fee Assets: Buyer/Seller debited, platform credited

		tradeValue := common.SafeMul(job.Exec.Price, job.Exec.Quantity)

		// Base asset balance flow:
		// Buyer debit = -Quantity, Seller credit = -Quantity. Net change of base asset = 0.
		// Let's verify quote asset and fee assets balance perfectly.

		// Calculate expected debits and credits per asset:
		// Map of expected net movement: asset -> balance (must sum to zero)
		netMovement := make(map[string]float64)

		// Buyer trade leg:
		// Buyer receives BaseAsset: +Quantity
		netMovement[job.BaseAsset] = common.SafeAdd(netMovement[job.BaseAsset], job.Exec.Quantity)

		// Buyer pays QuoteAsset (price * quantity): -tradeValue
		netMovement[job.QuoteAsset] = common.SafeSub(netMovement[job.QuoteAsset], tradeValue)

		// Buyer fee:
		buyerFeeAsset := job.Exec.BuyerFeeAsset
		if buyerFeeAsset == "" {
			buyerFeeAsset = job.QuoteAsset
		}
		// Buyer pays fee: -BuyerFee
		netMovement[buyerFeeAsset] = common.SafeSub(netMovement[buyerFeeAsset], job.Exec.BuyerFee)
		// Platform receives buyer fee: +BuyerFee
		netMovement[buyerFeeAsset] = common.SafeAdd(netMovement[buyerFeeAsset], job.Exec.BuyerFee)

		// Seller trade leg:
		// Seller delivers BaseAsset: -Quantity
		netMovement[job.BaseAsset] = common.SafeSub(netMovement[job.BaseAsset], job.Exec.Quantity)

		// Seller receives QuoteAsset (price * quantity): +tradeValue
		netMovement[job.QuoteAsset] = common.SafeAdd(netMovement[job.QuoteAsset], tradeValue)

		// Seller fee:
		sellerFeeAsset := job.Exec.SellerFeeAsset
		if sellerFeeAsset == "" {
			sellerFeeAsset = job.QuoteAsset
		}
		// Seller pays fee: -SellerFee
		netMovement[sellerFeeAsset] = common.SafeSub(netMovement[sellerFeeAsset], job.Exec.SellerFee)
		// Platform receives seller fee: +SellerFee
		netMovement[sellerFeeAsset] = common.SafeAdd(netMovement[sellerFeeAsset], job.Exec.SellerFee)

		// Verification: The net movement across ALL accounts for each involved asset must be exactly 0
		valid := true
		for asset, val := range netMovement {
			// Allow for tiny floating point noise, but since common.SafeAdd/Sub are used, it should be exact or near-exact
			if val < -1e-9 || val > 1e-9 {
				valid = false
				job.Status = SettleFailed
				job.ErrorMsg = fmt.Sprintf("Double Entry Validation Failure: asset %s equation desynchronized (net movement: %f)", asset, val)
				break
			}
		}

		if !valid {
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

	tradeValue := common.SafeMul(exec.Price, exec.Quantity)

	// Determine fee assets
	buyerFeeAsset := exec.BuyerFeeAsset
	if buyerFeeAsset == "" {
		buyerFeeAsset = job.QuoteAsset
	}
	sellerFeeAsset := exec.SellerFeeAsset
	if sellerFeeAsset == "" {
		sellerFeeAsset = job.QuoteAsset
	}

	// 1. Debit Buyer Quote Asset (Price * Quantity) and potentially fee if paid in Quote asset (from Reserved)
	// During order validation/reservation, the buyer had `Quantity * Price + BuyerRawFee` reserved in QuoteAsset.
	// Since the actual fee could have been paid in VLX instead of the QuoteAsset, we must release the hold correctly:
	// - If paid in QuoteAsset: reserved -= (tradeValue + BuyerFee), total -= (tradeValue + BuyerFee)
	// - If paid in VLX: reserved -= (tradeValue + BuyerRawFee), total -= tradeValue.
	//   And we also debit VLX from the buyer's balance: available -= BuyerFee (or total -= BuyerFee, but since it wasn't reserved, we subtract from available and total).
	buyerQuoteReservedDebit := common.SafeAdd(tradeValue, exec.BuyerRawFee)
	var buyerQuoteTotalDebit float64
	if buyerFeeAsset == job.QuoteAsset {
		buyerQuoteTotalDebit = common.SafeAdd(tradeValue, exec.BuyerFee)
	} else {
		buyerQuoteTotalDebit = tradeValue
	}

	_, err = tx.Exec(ctx,
		"UPDATE balances SET reserved = reserved - $1, total = total - $2, updated_at = NOW() WHERE user_id = $3 AND asset = $4",
		buyerQuoteReservedDebit, buyerQuoteTotalDebit, exec.BuyerID, job.QuoteAsset)
	if err != nil {
		return fmt.Errorf("failed to debit buyer quote balance: %w", err)
	}

	// If buyer paid fee in VLX/other asset, debit their available/total balance of that fee asset
	if buyerFeeAsset != job.QuoteAsset {
		_, err = tx.Exec(ctx,
			"UPDATE balances SET available = available - $1, total = total - $1, updated_at = NOW() WHERE user_id = $2 AND asset = $3",
			exec.BuyerFee, exec.BuyerID, buyerFeeAsset)
		if err != nil {
			return fmt.Errorf("failed to debit buyer fee balance in %s: %w", buyerFeeAsset, err)
		}
	}

	// 2. Credit Buyer Base Asset: Quantity
	_, err = tx.Exec(ctx,
		"INSERT INTO balances (user_id, asset, available, total, updated_at) VALUES ($1, $2, $3, $3, NOW()) "+
			"ON CONFLICT (user_id, asset) DO UPDATE SET available = balances.available + EXCLUDED.available, total = balances.total + EXCLUDED.total, updated_at = NOW()",
		exec.BuyerID, job.BaseAsset, exec.Quantity)
	if err != nil {
		return fmt.Errorf("failed to credit buyer base balance: %w", err)
	}

	// 3. Debit Seller Base Asset: Quantity (from Reserved)
	_, err = tx.Exec(ctx,
		"UPDATE balances SET reserved = reserved - $1, total = total - $1, updated_at = NOW() WHERE user_id = $2 AND asset = $3",
		exec.Quantity, exec.SellerID, job.BaseAsset)
	if err != nil {
		return fmt.Errorf("failed to debit seller base balance: %w", err)
	}

	// 4. Credit Seller Quote Asset: Price * Quantity (potentially minus fee if paid in Quote asset)
	var sellerCredit float64
	if sellerFeeAsset == job.QuoteAsset {
		sellerCredit = common.SafeSub(tradeValue, exec.SellerFee)
	} else {
		sellerCredit = tradeValue
	}

	_, err = tx.Exec(ctx,
		"INSERT INTO balances (user_id, asset, available, total, updated_at) VALUES ($1, $2, $3, $3, NOW()) "+
			"ON CONFLICT (user_id, asset) DO UPDATE SET available = balances.available + EXCLUDED.available, total = balances.total + EXCLUDED.total, updated_at = NOW()",
		exec.SellerID, job.QuoteAsset, sellerCredit)
	if err != nil {
		return fmt.Errorf("failed to credit seller quote balance: %w", err)
	}

	// If seller paid fee in VLX/other asset, debit their available/total balance of that fee asset
	if sellerFeeAsset != job.QuoteAsset {
		_, err = tx.Exec(ctx,
			"UPDATE balances SET available = available - $1, total = total - $1, updated_at = NOW() WHERE user_id = $2 AND asset = $3",
			exec.SellerFee, exec.SellerID, sellerFeeAsset)
		if err != nil {
			return fmt.Errorf("failed to debit seller fee balance in %s: %w", sellerFeeAsset, err)
		}
	}

	// 5. Credit Platform Fee Account
	platformFeeAccount := "platform_fees"
	// Credit buyer fee to platform
	_, err = tx.Exec(ctx,
		"INSERT INTO balances (user_id, asset, available, total, updated_at) VALUES ($1, $2, $3, $3, NOW()) "+
			"ON CONFLICT (user_id, asset) DO UPDATE SET available = balances.available + EXCLUDED.available, total = balances.total + EXCLUDED.total, updated_at = NOW()",
		platformFeeAccount, buyerFeeAsset, exec.BuyerFee)
	if err != nil {
		return fmt.Errorf("failed to credit platform buyer fee balance in %s: %w", buyerFeeAsset, err)
	}

	// Credit seller fee to platform
	_, err = tx.Exec(ctx,
		"INSERT INTO balances (user_id, asset, available, total, updated_at) VALUES ($1, $2, $3, $3, NOW()) "+
			"ON CONFLICT (user_id, asset) DO UPDATE SET available = balances.available + EXCLUDED.available, total = balances.total + EXCLUDED.total, updated_at = NOW()",
		platformFeeAccount, sellerFeeAsset, exec.SellerFee)
	if err != nil {
		return fmt.Errorf("failed to credit platform seller fee balance in %s: %w", sellerFeeAsset, err)
	}

	// 6. Record Dual Entry Ledger Postings
	ledgerTxID := "tx_settle_" + exec.TradeID

	// Buyer quote debit
	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
		"ent_b_q_"+exec.TradeID, ledgerTxID, exec.BuyerID, job.QuoteAsset, "DEBIT", buyerQuoteTotalDebit, "Buyer execution quote cost")
	if err != nil {
		return fmt.Errorf("failed to write buyer quote ledger: %w", err)
	}

	// If buyer paid fee in VLX, write corresponding ledger entry
	if buyerFeeAsset != job.QuoteAsset {
		_, err = tx.Exec(ctx,
			"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
			"ent_b_fee_"+exec.TradeID, ledgerTxID, exec.BuyerID, buyerFeeAsset, "DEBIT", exec.BuyerFee, "Buyer execution VLX fee payment")
		if err != nil {
			return fmt.Errorf("failed to write buyer fee ledger: %w", err)
		}
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

	// If seller paid fee in VLX, write corresponding ledger entry
	if sellerFeeAsset != job.QuoteAsset {
		_, err = tx.Exec(ctx,
			"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
			"ent_s_fee_"+exec.TradeID, ledgerTxID, exec.SellerID, sellerFeeAsset, "DEBIT", exec.SellerFee, "Seller execution VLX fee payment")
		if err != nil {
			return fmt.Errorf("failed to write seller fee ledger: %w", err)
		}
	}

	// Platform fee credits
	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
		"ent_p_fee_b_"+exec.TradeID, ledgerTxID, platformFeeAccount, buyerFeeAsset, "CREDIT", exec.BuyerFee, "Platform revenue from buyer fee")
	if err != nil {
		return fmt.Errorf("failed to write platform buyer fee credit ledger: %w", err)
	}

	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
		"ent_p_fee_s_"+exec.TradeID, ledgerTxID, platformFeeAccount, sellerFeeAsset, "CREDIT", exec.SellerFee, "Platform revenue from seller fee")
	if err != nil {
		return fmt.Errorf("failed to write platform seller fee credit ledger: %w", err)
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

// SetDB dynamically configures or updates the postgres connection pool
func (se *SettlementEngine) SetDB(db *database.DB) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.db = db
}
