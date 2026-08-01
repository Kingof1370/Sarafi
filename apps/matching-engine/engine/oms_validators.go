package engine

import (
	"fmt"
	"math"
	"sync"
)

// OMSValidator performs independent, modular order validations
type OMSValidator struct {
	mu    sync.RWMutex
	pairs map[string]*TradingPairConfig
}

// NewOMSValidator creates an OMS validation pipeline coordinator
func NewOMSValidator() *OMSValidator {
	// Pre-seed common active symbols to avoid empty state issues in testing
	v := &OMSValidator{
		pairs: make(map[string]*TradingPairConfig),
	}

	v.pairs["BTC-USDT"] = &TradingPairConfig{
		Symbol:            "BTC-USDT",
		MinOrderSize:      0.0001,
		MaxOrderSize:      1000.0,
		MinNotional:       5.0,
		MaxNotional:       10000000.0,
		TickSize:          0.01,
		StepSize:          0.0001,
		PricePrecision:    2,
		QuantityPrecision: 4,
		IsActive:          true,
	}

	v.pairs["ETH-USDT"] = &TradingPairConfig{
		Symbol:            "ETH-USDT",
		MinOrderSize:      0.001,
		MaxOrderSize:      10000.0,
		MinNotional:       5.0,
		MaxNotional:       5000000.0,
		TickSize:          0.01,
		StepSize:          0.001,
		PricePrecision:    2,
		QuantityPrecision: 3,
		IsActive:          true,
	}

	return v
}

// RegisterTradingPair adds validation parameters for a market symbol
func (v *OMSValidator) RegisterTradingPair(cfg *TradingPairConfig) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.pairs[cfg.Symbol] = cfg
}

// ValidateTradingRules checks all mechanical tick/size/notional constraints
func (v *OMSValidator) ValidateTradingRules(order *AdvancedOrder) error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	cfg, exists := v.pairs[order.Symbol]
	if !exists {
		return fmt.Errorf("trading pair %s is not registered", order.Symbol)
	}

	if !cfg.IsActive {
		return fmt.Errorf("trading pair %s is currently suspended/inactive", order.Symbol)
	}

	// 1. Order Size check
	if order.Quantity < cfg.MinOrderSize || order.Quantity > cfg.MaxOrderSize {
		return fmt.Errorf("order quantity %f exceeds limits [%f, %f]", order.Quantity, cfg.MinOrderSize, cfg.MaxOrderSize)
	}

	// 2. Notional value check (Price * Quantity)
	notional := order.Price * order.Quantity
	// If it is a market order we might bypass or check using current index price, for safety check if price set
	if order.Price > 0 {
		if notional < cfg.MinNotional || notional > cfg.MaxNotional {
			return fmt.Errorf("order notional %f exceeds limits [%f, %f]", notional, cfg.MinNotional, cfg.MaxNotional)
		}
	}

	// 3. Tick Size (Price precision step checks)
	if order.Price > 0 {
		remainder := math.Mod(order.Price, cfg.TickSize)
		if remainder > 1e-9 && math.Abs(remainder-cfg.TickSize) > 1e-9 {
			return fmt.Errorf("order price %f does not conform to tick size %f", order.Price, cfg.TickSize)
		}
	}

	// 4. Step Size (Quantity precision step checks)
	remainderQty := math.Mod(order.Quantity, cfg.StepSize)
	if remainderQty > 1e-9 && math.Abs(remainderQty-cfg.StepSize) > 1e-9 {
		return fmt.Errorf("order quantity %f does not conform to step size %f", order.Quantity, cfg.StepSize)
	}

	return nil
}
