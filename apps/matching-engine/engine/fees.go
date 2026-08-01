package engine

import (
	"sync"
)

// FeesEngine calculates maker and taker fees based on user tiers
type FeesEngine struct {
	mu             sync.RWMutex
	userVolume30D  map[string]float64 // userID -> USD trading volume
	defaultMaker   float64            // default maker fee rate (e.g., 0.0010 = 0.10%)
	defaultTaker   float64            // default taker fee rate (e.g., 0.0020 = 0.20%)
}

// NewFeesEngine creates a new fees coordinator
func NewFeesEngine(maker, taker float64) *FeesEngine {
	return &FeesEngine{
		userVolume30D: make(map[string]float64),
		defaultMaker:  maker,
		defaultTaker:  taker,
	}
}

// SetUserVolume sets the 30D volume for tier calculations
func (fe *FeesEngine) SetUserVolume(userID string, usdVolume float64) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.userVolume30D[userID] = usdVolume
}

// GetFeeRates calculates custom maker/taker fees based on the user's trading tier
func (fe *FeesEngine) GetFeeRates(userID string) (makerRate, takerRate float64) {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	vol := fe.userVolume30D[userID]

	// Dynamic Volume Tiers
	if vol >= 10000000.0 { // VIP Tier 3: > 10M USD
		return 0.0000, 0.0005 // 0.00% Maker, 0.05% Taker
	} else if vol >= 1000000.0 { // VIP Tier 2: > 1M USD
		return 0.0002, 0.0010 // 0.02% Maker, 0.10% Taker
	} else if vol >= 100000.0 { // VIP Tier 1: > 100k USD
		return 0.0005, 0.0015 // 0.05% Maker, 0.15% Taker
	}

	return fe.defaultMaker, fe.defaultTaker
}

// CalculateFee returns the fee amount for an execution price and quantity
func (fe *FeesEngine) CalculateFee(userID string, price, quantity float64, isMaker bool) float64 {
	makerRate, takerRate := fe.GetFeeRates(userID)
	rate := takerRate
	if isMaker {
		rate = makerRate
	}
	return price * quantity * rate
}
