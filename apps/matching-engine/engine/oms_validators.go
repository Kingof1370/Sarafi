package engine

import (
	"fmt"
	"math/big"
	"sync"
	"velyxora/packages/common"
)

// OMSValidator performs independent, modular order validations
type OMSValidator struct {
	mu           sync.RWMutex
	pairs        map[string]*TradingPairConfig
	GetLastPrice func(symbol string) float64
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
	getLastPriceFn := v.GetLastPrice
	cfg, exists := v.pairs[order.Symbol]
	v.mu.RUnlock()

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

	// 2. Notional value check (Price * Quantity) using high-precision arithmetic
	var priceToUse float64
	if order.Price > 0 {
		priceToUse = order.Price
	} else if order.Type == "MARKET" {
		if getLastPriceFn != nil {
			priceToUse = getLastPriceFn(order.Symbol)
		}
	}

	if priceToUse > 0 {
		notional := common.SafeMul(priceToUse, order.Quantity)
		if notional < cfg.MinNotional || notional > cfg.MaxNotional {
			return fmt.Errorf("order notional %f exceeds limits [%f, %f]", notional, cfg.MinNotional, cfg.MaxNotional)
		}
	}

	// 3. Tick Size (Price precision step checks) using high-precision arithmetic
	if order.Price > 0 && cfg.TickSize > 0 {
		rPrice := common.ToRat(order.Price)
		rTick := common.ToRat(cfg.TickSize)
		rDiv := new(big.Rat).Quo(rPrice, rTick)
		if !rDiv.IsInt() {
			return fmt.Errorf("order price %f does not conform to tick size %f", order.Price, cfg.TickSize)
		}
	}

	// 4. Step Size (Quantity precision step checks) using high-precision arithmetic
	if order.Quantity > 0 && cfg.StepSize > 0 {
		rQty := common.ToRat(order.Quantity)
		rStep := common.ToRat(cfg.StepSize)
		rDiv := new(big.Rat).Quo(rQty, rStep)
		if !rDiv.IsInt() {
			return fmt.Errorf("order quantity %f does not conform to step size %f", order.Quantity, cfg.StepSize)
		}
	}

	// 5. Percent Price Filter (Price Band Limit)
	if order.Price > 0 && getLastPriceFn != nil {
		lastPrice := getLastPriceFn(order.Symbol)
		if lastPrice > 0 {
			pct := cfg.PriceBandPercentage
			if pct == 0 {
				pct = cfg.PriceBrandPercentage
			}
			if pct == 0 {
				pct = 0.05 // default 5%
			}

			// Upper band for BUY: LastPrice * (1 + pct)
			onePlusPct := common.SafeAdd(1.0, pct)
			upperLimit := common.SafeMul(lastPrice, onePlusPct)

			// Lower band for SELL: LastPrice * (1 - pct)
			oneMinusPct := common.SafeSub(1.0, pct)
			lowerLimit := common.SafeMul(lastPrice, oneMinusPct)

			if order.Side == "BUY" && order.Price > upperLimit {
				return fmt.Errorf("buy order price %f exceeds percent price band upper limit %f (LastPrice: %f, Band: %f%%)", order.Price, upperLimit, lastPrice, pct*100)
			}
			if order.Side == "SELL" && order.Price < lowerLimit {
				return fmt.Errorf("sell order price %f is below percent price band lower limit %f (LastPrice: %f, Band: %f%%)", order.Price, lowerLimit, lastPrice, pct*100)
			}
		}
	}

	// 6. Invalid combinations and safety validations
	if order.Quantity <= 0 {
		return fmt.Errorf("invalid order quantity %f: must be greater than 0", order.Quantity)
	}

	if order.Type == "LIMIT" && order.Price <= 0 {
		return fmt.Errorf("invalid order price %f for LIMIT order", order.Price)
	}

	if order.PostOnly {
		if order.Type == "MARKET" {
			return fmt.Errorf("invalid combination: PostOnly cannot be used with MARKET orders")
		}
		if order.TimeInForce == TIF_IOC || order.TimeInForce == TIF_FOK {
			return fmt.Errorf("invalid combination: PostOnly cannot be used with IOC or FOK")
		}
	}

	if order.TimeInForce != "" {
		if order.TimeInForce != TIF_GTC && order.TimeInForce != TIF_IOC && order.TimeInForce != TIF_FOK && order.TimeInForce != TIF_GTD && order.TimeInForce != TIF_GTT {
			return fmt.Errorf("invalid time-in-force modifier: %s", order.TimeInForce)
		}
	}

	return nil
}
