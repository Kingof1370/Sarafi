package engine

import (
	"context"
	"fmt"
	"sync"
	"velyxora/packages/database"
)

// SettlementEngine processes final fiat/crypto balance ledgering on physical databases
type SettlementEngine struct {
	mu     sync.Mutex
	db     *database.DB
	ledger []string // tracked execution trade IDs
}

// NewSettlementEngine creates a persistent trade clearing house
func NewSettlementEngine(db *database.DB) *SettlementEngine {
	return &SettlementEngine{
		db:     db,
		ledger: make([]string, 0),
	}
}

// SettleExecution atomically locks and ledger-credits balances
func (s *SettlementEngine) SettleExecution(ctx context.Context, exec *Execution, baseAsset, quoteAsset string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Track in memory
	s.ledger = append(s.ledger, exec.TradeID)

	// If real PostgreSQL database connection is available, commit ACID transactions
	if s.db != nil {
		tx, err := s.db.Pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin SQL settlement: %w", err)
		}
		defer tx.Rollback(ctx)

		// SQL updates for Buyer
		// 1. Debit Quote Asset: total cost (price * quantity) + buyer fee
		buyerCost := exec.Price*exec.Quantity + exec.BuyerFee
		_, err = tx.Exec(ctx,
			"UPDATE balances SET available = available - $1, total = total - $1, updated_at = NOW() WHERE user_id = $2 AND asset = $3",
			buyerCost, exec.BuyerID, quoteAsset)
		if err != nil {
			return fmt.Errorf("failed to debit buyer quote balance: %w", err)
		}

		// 2. Credit Base Asset: quantity
		_, err = tx.Exec(ctx,
			"INSERT INTO balances (user_id, asset, available, total, updated_at) VALUES ($1, $2, $3, $3, NOW()) "+
				"ON CONFLICT (user_id, asset) DO UPDATE SET available = balances.available + EXCLUDED.available, total = balances.total + EXCLUDED.total, updated_at = NOW()",
			exec.BuyerID, baseAsset, exec.Quantity)
		if err != nil {
			return fmt.Errorf("failed to credit buyer base balance: %w", err)
		}

		// SQL updates for Seller
		// 3. Debit Base Asset: quantity
		_, err = tx.Exec(ctx,
			"UPDATE balances SET available = available - $1, total = total - $1, updated_at = NOW() WHERE user_id = $2 AND asset = $3",
			exec.Quantity, exec.SellerID, baseAsset)
		if err != nil {
			return fmt.Errorf("failed to debit seller base balance: %w", err)
		}

		// 4. Credit Quote Asset: net proceeds (price * quantity) - seller fee
		sellerProceeds := exec.Price*exec.Quantity - exec.SellerFee
		_, err = tx.Exec(ctx,
			"INSERT INTO balances (user_id, asset, available, total, updated_at) VALUES ($1, $2, $3, $3, NOW()) "+
				"ON CONFLICT (user_id, asset) DO UPDATE SET available = balances.available + EXCLUDED.available, total = balances.total + EXCLUDED.total, updated_at = NOW()",
			exec.SellerID, quoteAsset, sellerProceeds)
		if err != nil {
			return fmt.Errorf("failed to credit seller quote balance: %w", err)
		}

		// 5. Insert Ledger Entry Logs
		ledgerTxID := "tx_settle_" + exec.TradeID
		_, err = tx.Exec(ctx,
			"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
			"ent_b_q_"+exec.TradeID, ledgerTxID, exec.BuyerID, quoteAsset, "DEBIT", buyerCost, "Buyer trade quote asset cost")
		if err != nil {
			return fmt.Errorf("failed to write buyer quote ledger: %w", err)
		}

		_, err = tx.Exec(ctx,
			"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
			"ent_b_b_"+exec.TradeID, ledgerTxID, exec.BuyerID, baseAsset, "CREDIT", exec.Quantity, "Buyer trade base asset delivery")
		if err != nil {
			return fmt.Errorf("failed to write buyer base ledger: %w", err)
		}

		err = tx.Commit(ctx)
		if err != nil {
			return fmt.Errorf("failed to commit settlement transaction: %w", err)
		}
	}

	return nil
}

// GetSettledCount returns count of settled executions
func (s *SettlementEngine) GetSettledCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.ledger)
}
