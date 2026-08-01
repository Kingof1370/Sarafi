package engine

import (
	"sync"
	"time"
	"velyxora/packages/types"
)

// Execution represents an executed transaction leg with fees
type Execution struct {
	TradeID      string    `json:"trade_id"`
	Symbol       string    `json:"symbol"`
	Price        float64   `json:"price"`
	Quantity     float64   `json:"quantity"`
	BuyerID      string    `json:"buyer_id"`
	SellerID     string    `json:"seller_id"`
	BuyerFee     float64   `json:"buyer_fee"`
	SellerFee    float64   `json:"seller_fee"`
	Timestamp    time.Time `json:"timestamp"`
}

// ExecutionEngine processes trade outputs and updates holds and fee balances
type ExecutionEngine struct {
	mu         sync.RWMutex
	fees       *FeesEngine
	risk       *RiskEngine
	executions []*Execution
}

// NewExecutionEngine creates a trade processor
func NewExecutionEngine(fees *FeesEngine, risk *RiskEngine) *ExecutionEngine {
	return &ExecutionEngine{
		fees:       fees,
		risk:       risk,
		executions: make([]*Execution, 0),
	}
}

// ProcessTrades registers executing trade events and computes dynamic transaction fees
func (ee *ExecutionEngine) ProcessTrades(trades []*types.Trade, quoteAsset, baseAsset string) []*Execution {
	ee.mu.Lock()
	defer ee.mu.Unlock()

	results := make([]*Execution, 0, len(trades))

	for _, t := range trades {
		// Calculate Fees
		buyerFee := ee.fees.CalculateFee(t.BuyerID, t.Price, t.Quantity, false)  // buyer is taker
		sellerFee := ee.fees.CalculateFee(t.SellerID, t.Price, t.Quantity, true) // seller is maker

		exec := &Execution{
			TradeID:   t.ID,
			Symbol:    t.Symbol,
			Price:     t.Price,
			Quantity:  t.Quantity,
			BuyerID:   t.BuyerID,
			SellerID:  t.SellerID,
			BuyerFee:  buyerFee,
			SellerFee: sellerFee,
			Timestamp: t.Timestamp,
		}

		ee.executions = append(ee.executions, exec)
		results = append(results, exec)

		// Release Risk hold limits as matching is completed
		// Quote asset hold released for buyer (including computed buy fee)
		ee.risk.ReleaseHold(t.BuyerID, quoteAsset, t.Quantity*t.Price+buyerFee)
		// Base asset hold released for seller
		ee.risk.ReleaseHold(t.SellerID, baseAsset, t.Quantity)

		// Apply final net adjustments to the RiskEngine's fast-access available balance sheets
		// Buyer gains Base, loses Quote + Fee
		ee.risk.UpdateBalance(t.BuyerID, baseAsset, t.Quantity)
		ee.risk.UpdateBalance(t.BuyerID, quoteAsset, -(t.Quantity*t.Price + buyerFee))

		// Seller gains Quote - Fee, loses Base
		ee.risk.UpdateBalance(t.SellerID, quoteAsset, t.Quantity*t.Price-sellerFee)
		ee.risk.UpdateBalance(t.SellerID, baseAsset, -t.Quantity)
	}

	return results
}

// GetExecutions returns historical trades execution records
func (ee *ExecutionEngine) GetExecutions() []*Execution {
	ee.mu.RLock()
	defer ee.mu.RUnlock()
	return ee.executions
}
