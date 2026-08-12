package engine

import (
	"testing"
)

func TestValidateTradingRules_PrecisionAndLimits(t *testing.T) {
	val := NewOMSValidator()

	// Register a pair with custom configuration
	cfg := &TradingPairConfig{
		Symbol:               "TEST-USDT",
		MinOrderSize:         0.01,
		MaxOrderSize:         100.0,
		MinNotional:          10.0,
		MaxNotional:          100000.0,
		TickSize:             0.05,
		StepSize:             0.02,
		PricePrecision:       2,
		QuantityPrecision:    2,
		PriceBandPercentage:  0.10, // 10% band
		IsActive:             true,
	}
	val.RegisterTradingPair(cfg)

	// Set LastPrice using a mock helper or direct callback
	lastPrice := 100.0
	val.GetLastPrice = func(symbol string) float64 {
		if symbol == "TEST-USDT" {
			return lastPrice
		}
		return 0
	}

	// 1. Valid Order
	validOrder := &AdvancedOrder{
		Symbol:   "TEST-USDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Price:    100.05, // conforms to tick size 0.05 (100.05 / 0.05 = 2001)
		Quantity: 1.0,    // conforms to step size 0.02
	}
	if err := val.ValidateTradingRules(validOrder); err != nil {
		t.Errorf("Expected valid order to pass, got error: %v", err)
	}

	// 2. Invalid Tick Size
	invalidTickOrder := &AdvancedOrder{
		Symbol:   "TEST-USDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Price:    100.01, // does not conform to tick size 0.05
		Quantity: 1.0,
	}
	if err := val.ValidateTradingRules(invalidTickOrder); err == nil {
		t.Error("Expected failure for invalid tick size, but it passed")
	}

	// 3. Invalid Step Size
	invalidStepOrder := &AdvancedOrder{
		Symbol:   "TEST-USDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Price:    100.0,
		Quantity: 1.01, // does not conform to step size 0.02
	}
	if err := val.ValidateTradingRules(invalidStepOrder); err == nil {
		t.Error("Expected failure for invalid step size, but it passed")
	}

	// 4. Below Min Notional (LIMIT)
	belowNotionalLimit := &AdvancedOrder{
		Symbol:   "TEST-USDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Price:    50.0,
		Quantity: 0.1, // 50 * 0.1 = 5.0 (min is 10.0)
	}
	if err := val.ValidateTradingRules(belowNotionalLimit); err == nil {
		t.Error("Expected failure for limit order below min notional, but it passed")
	}

	// 5. Below Min Notional (MARKET)
	belowNotionalMarket := &AdvancedOrder{
		Symbol:   "TEST-USDT",
		Side:     "BUY",
		Type:     "MARKET",
		Quantity: 0.08, // Estimated: 100 * 0.08 = 8.0 (min is 10.0)
	}
	if err := val.ValidateTradingRules(belowNotionalMarket); err == nil {
		t.Error("Expected failure for market order below min notional, but it passed")
	}

	// 6. Outside custom 10% Price Band (BUY)
	outsideBandBuy := &AdvancedOrder{
		Symbol:   "TEST-USDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Price:    111.0, // LastPrice is 100, custom band is 10%, upper limit is 110.0. 111.0 is outside.
		Quantity: 1.0,
	}
	if err := val.ValidateTradingRules(outsideBandBuy); err == nil {
		t.Error("Expected failure for BUY order outside upper 10% price band, but it passed")
	}

	// 7. Outside custom 10% Price Band (SELL)
	outsideBandSell := &AdvancedOrder{
		Symbol:   "TEST-USDT",
		Side:     "SELL",
		Type:     "LIMIT",
		Price:    89.0, // LastPrice is 100, custom band is 10%, lower limit is 90.0. 89.0 is outside.
		Quantity: 1.0,
	}
	if err := val.ValidateTradingRules(outsideBandSell); err == nil {
		t.Error("Expected failure for SELL order outside lower 10% price band, but it passed")
	}

	// 8. Within custom 10% Price Band
	withinBandBuy := &AdvancedOrder{
		Symbol:   "TEST-USDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Price:    109.95, // conforms to tick size 0.05, within 10% limit 110.0
		Quantity: 1.0,
	}
	if err := val.ValidateTradingRules(withinBandBuy); err != nil {
		t.Errorf("Expected within-band BUY order to pass, got error: %v", err)
	}

	withinBandSell := &AdvancedOrder{
		Symbol:   "TEST-USDT",
		Side:     "SELL",
		Type:     "LIMIT",
		Price:    90.05, // conforms to tick size 0.05, within 10% limit 90.0
		Quantity: 1.0,
	}
	if err := val.ValidateTradingRules(withinBandSell); err != nil {
		t.Errorf("Expected within-band SELL order to pass, got error: %v", err)
	}
}

func TestValidateTradingRules_DefaultPriceBand(t *testing.T) {
	val := NewOMSValidator()

	// Register a pair with default 5% band (no price band percentage configured)
	cfg := &TradingPairConfig{
		Symbol:            "DEFAULT-USDT",
		MinOrderSize:      0.01,
		MaxOrderSize:      100.0,
		MinNotional:       1.0,
		MaxNotional:       100000.0,
		TickSize:          0.01,
		StepSize:          0.01,
		PricePrecision:    2,
		QuantityPrecision: 2,
		IsActive:          true,
	}
	val.RegisterTradingPair(cfg)

	lastPrice := 100.0
	val.GetLastPrice = func(symbol string) float64 {
		if symbol == "DEFAULT-USDT" {
			return lastPrice
		}
		return 0
	}

	// Default 5% upper band for BUY is 105.0. 106.0 should fail.
	outsideDefaultBuy := &AdvancedOrder{
		Symbol:   "DEFAULT-USDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Price:    106.0,
		Quantity: 1.0,
	}
	if err := val.ValidateTradingRules(outsideDefaultBuy); err == nil {
		t.Error("Expected failure for BUY order outside default 5% price band, but it passed")
	}

	// 104.99 should pass.
	withinDefaultBuy := &AdvancedOrder{
		Symbol:   "DEFAULT-USDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Price:    104.99,
		Quantity: 1.0,
	}
	if err := val.ValidateTradingRules(withinDefaultBuy); err != nil {
		t.Errorf("Expected within-band BUY order to pass, got error: %v", err)
	}
}
