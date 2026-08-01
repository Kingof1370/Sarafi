package engine

import (
	"fmt"
	"sync"
	"velyxora/packages/types"
)

// RiskEngine manages pre-trade verification and balance isolation
type RiskEngine struct {
	mu            sync.RWMutex
	userBalances  map[string]map[string]float64 // userID -> asset -> available amount
	activeHolds   map[string]map[string]float64 // userID -> asset -> total held for active orders
	maxPriceLimit float64
	minPriceLimit float64
}

// NewRiskEngine creates a new pre-trade Risk validator
func NewRiskEngine(minPrice, maxPrice float64) *RiskEngine {
	return &RiskEngine{
		userBalances:  make(map[string]map[string]float64),
		activeHolds:   make(map[string]map[string]float64),
		minPriceLimit: minPrice,
		maxPriceLimit: maxPrice,
	}
}

// DepositAsset sets or updates a user balance for risk calculations
func (re *RiskEngine) DepositAsset(userID, asset string, amount float64) {
	re.mu.Lock()
	defer re.mu.Unlock()

	if _, ok := re.userBalances[userID]; !ok {
		re.userBalances[userID] = make(map[string]float64)
	}
	re.userBalances[userID][asset] += amount
}

// GetAvailableBalance returns available balance taking locks into account
func (re *RiskEngine) GetAvailableBalance(userID, asset string) float64 {
	re.mu.RLock()
	defer re.mu.RUnlock()

	bal := 0.0
	if userBals, ok := re.userBalances[userID]; ok {
		bal = userBals[asset]
	}

	holds := 0.0
	if userHolds, ok := re.activeHolds[userID]; ok {
		holds = userHolds[asset]
	}

	available := bal - holds
	if available < 0 {
		return 0
	}
	return available
}

// ValidateOrder performs pre-trade check on price boundaries and sufficient funds
func (re *RiskEngine) ValidateOrder(order *types.Order, quoteAsset, baseAsset string, feeRate float64) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	// Price Boundary Check
	if order.Type == types.TypeLimit {
		if order.Price < re.minPriceLimit || order.Price > re.maxPriceLimit {
			return fmt.Errorf("order price %f out of permitted limits [%f, %f]", order.Price, re.minPriceLimit, re.maxPriceLimit)
		}
	}

	// Balance verification
	if order.Side == types.SideBuy {
		// Buyer must lock Quote asset = Quantity * Price * (1 + fee)
		requiredAmount := order.Quantity * order.Price
		if order.Type == types.TypeLimit {
			requiredAmount = order.Quantity * order.Price * (1.0 + feeRate)
		} else {
			// For market orders we check against the estimated current price or just verify we have funds
			requiredAmount = order.Quantity * (1.0 + feeRate)
		}

		available := 0.0
		if bals, ok := re.userBalances[order.UserID]; ok {
			available = bals[quoteAsset]
		}
		holds := 0.0
		if hlds, ok := re.activeHolds[order.UserID]; ok {
			holds = hlds[quoteAsset]
		}

		if (available - holds) < requiredAmount {
			return fmt.Errorf("insufficient funds: required %f %s, available %f", requiredAmount, quoteAsset, available-holds)
		}

		// Apply Hold
		if _, ok := re.activeHolds[order.UserID]; !ok {
			re.activeHolds[order.UserID] = make(map[string]float64)
		}
		re.activeHolds[order.UserID][quoteAsset] += requiredAmount

	} else {
		// Seller must lock Base asset = Quantity
		requiredAmount := order.Quantity

		available := 0.0
		if bals, ok := re.userBalances[order.UserID]; ok {
			available = bals[baseAsset]
		}
		holds := 0.0
		if hlds, ok := re.activeHolds[order.UserID]; ok {
			holds = hlds[baseAsset]
		}

		if (available - holds) < requiredAmount {
			return fmt.Errorf("insufficient asset balance: required %f %s, available %f", requiredAmount, baseAsset, available-holds)
		}

		// Apply Hold
		if _, ok := re.activeHolds[order.UserID]; !ok {
			re.activeHolds[order.UserID] = make(map[string]float64)
		}
		re.activeHolds[order.UserID][baseAsset] += requiredAmount
	}

	return nil
}

// ReleaseHold clears a balance hold after matching or cancellation
func (re *RiskEngine) ReleaseHold(userID, asset string, amount float64) {
	re.mu.Lock()
	defer re.mu.Unlock()

	if userHolds, ok := re.activeHolds[userID]; ok {
		userHolds[asset] -= amount
		if userHolds[asset] < 0 {
			userHolds[asset] = 0
		}
	}
}

// UpdateBalance applies actual settlement adjustments to balance sheets
func (re *RiskEngine) UpdateBalance(userID, asset string, balanceChange float64) {
	re.mu.Lock()
	defer re.mu.Unlock()

	if _, ok := re.userBalances[userID]; !ok {
		re.userBalances[userID] = make(map[string]float64)
	}
	re.userBalances[userID][asset] += balanceChange
	if re.userBalances[userID][asset] < 0 {
		re.userBalances[userID][asset] = 0
	}
}
