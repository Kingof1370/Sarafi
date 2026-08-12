package engine

import (
	"sync"
	"time"
	"velyxora/packages/types"
)

// Execution represents an executed transaction leg with fees
type Execution struct {
	TradeID             string    `json:"trade_id"`
	Symbol              string    `json:"symbol"`
	Price               float64   `json:"price"`
	Quantity            float64   `json:"quantity"`
	BuyerID             string    `json:"buyer_id"`
	SellerID            string    `json:"seller_id"`
	BuyerFee            float64   `json:"buyer_fee"`
	SellerFee           float64   `json:"seller_fee"`
	BuyerRawFee         float64   `json:"buyer_raw_fee"`
	SellerRawFee        float64   `json:"seller_raw_fee"`
	BuyerFeeAsset       string    `json:"buyer_fee_asset"`
	SellerFeeAsset      string    `json:"seller_fee_asset"`
	BuyerFeeDiscounted  bool      `json:"buyer_fee_discounted"`
	SellerFeeDiscounted bool      `json:"seller_fee_discounted"`
	Timestamp           time.Time `json:"timestamp"`
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
		// Calculate raw fees
		buyerRawFee := ee.fees.CalculateFee(t.BuyerID, t.Price, t.Quantity, false)  // buyer is taker
		sellerRawFee := ee.fees.CalculateFee(t.SellerID, t.Price, t.Quantity, true) // seller is maker

		// Calculate discounted fee details for both buyer and seller
		buyerFeeAsset, buyerFee, buyerDiscounted := ee.fees.CalculateFeeForUser(t.BuyerID, t.Symbol, buyerRawFee, false)
		sellerFeeAsset, sellerFee, sellerDiscounted := ee.fees.CalculateFeeForUser(t.SellerID, t.Symbol, sellerRawFee, true)

		exec := &Execution{
			TradeID:             t.ID,
			Symbol:              t.Symbol,
			Price:               t.Price,
			Quantity:            t.Quantity,
			BuyerID:             t.BuyerID,
			SellerID:            t.SellerID,
			BuyerFee:            buyerFee,
			SellerFee:           sellerFee,
			BuyerRawFee:         buyerRawFee,
			SellerRawFee:        sellerRawFee,
			BuyerFeeAsset:       buyerFeeAsset,
			SellerFeeAsset:      sellerFeeAsset,
			BuyerFeeDiscounted:  buyerDiscounted,
			SellerFeeDiscounted: sellerDiscounted,
			Timestamp:           t.Timestamp,
		}

		ee.executions = append(ee.executions, exec)
		results = append(results, exec)

		// Release Risk hold limits as matching is completed
		// Release the exact quote hold reserved during Order placement (which assumed raw fee)
		ee.risk.ReleaseHold(t.BuyerID, quoteAsset, t.Quantity*t.Price+buyerRawFee)
		// Base asset hold released for seller
		ee.risk.ReleaseHold(t.SellerID, baseAsset, t.Quantity)

		// Apply final net adjustments to the RiskEngine's fast-access available balance sheets
		// Buyer gains Base, loses Quote (without raw fee if fee is VLX), and loses Fee in FeeAsset (which might be VLX or Quote asset)
		ee.risk.UpdateBalance(t.BuyerID, baseAsset, t.Quantity)
		if buyerFeeAsset == quoteAsset {
			ee.risk.UpdateBalance(t.BuyerID, quoteAsset, -(t.Quantity*t.Price + buyerFee))
		} else {
			ee.risk.UpdateBalance(t.BuyerID, quoteAsset, -(t.Quantity*t.Price))
			ee.risk.UpdateBalance(t.BuyerID, buyerFeeAsset, -buyerFee)
		}

		// Seller gains Quote (without fee deduction if fee is VLX), loses Base, and loses Fee in FeeAsset (which might be VLX or Quote asset)
		ee.risk.UpdateBalance(t.SellerID, baseAsset, -t.Quantity)
		if sellerFeeAsset == quoteAsset {
			ee.risk.UpdateBalance(t.SellerID, quoteAsset, t.Quantity*t.Price-sellerFee)
		} else {
			ee.risk.UpdateBalance(t.SellerID, quoteAsset, t.Quantity*t.Price)
			ee.risk.UpdateBalance(t.SellerID, sellerFeeAsset, -sellerFee)
		}
	}

	return results
}

// GetExecutions returns historical trades execution records
func (ee *ExecutionEngine) GetExecutions() []*Execution {
	ee.mu.RLock()
	defer ee.mu.RUnlock()
	return ee.executions
}
