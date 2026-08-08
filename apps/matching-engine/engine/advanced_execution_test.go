package engine

import (
	"context"
	"testing"
)

func TestAdvancedOrderTypesAndVal(t *testing.T) {
	val := NewOMSValidator()

	// Invalid combinations: PostOnly with Market
	ord1 := &AdvancedOrder{
		Symbol:   "BTC-USDT",
		Price:    50000.0,
		Quantity: 0.1,
		Type:     "MARKET",
		PostOnly: true,
	}
	if err := val.ValidateTradingRules(ord1); err == nil {
		t.Error("Expected validation error for PostOnly with MARKET")
	}

	// Invalid combinations: PostOnly with IOC
	ord2 := &AdvancedOrder{
		Symbol:      "BTC-USDT",
		Price:       50000.0,
		Quantity:    0.1,
		Type:        "LIMIT",
		PostOnly:    true,
		TimeInForce: TIF_IOC,
	}
	if err := val.ValidateTradingRules(ord2); err == nil {
		t.Error("Expected validation error for PostOnly with IOC")
	}

	// Invalid quantity <= 0
	ord3 := &AdvancedOrder{
		Symbol:   "BTC-USDT",
		Price:    50000.0,
		Quantity: -0.5,
		Type:     "LIMIT",
	}
	if err := val.ValidateTradingRules(ord3); err == nil {
		t.Error("Expected validation error for negative order quantity")
	}
}

func TestStopAndTakeProfitTriggers(t *testing.T) {
	sm := NewOMSStateMachine()
	val := NewOMSValidator()
	risk := NewRiskEngine(10.0, 100000.0)
	matcher := NewMatcher("BTC-USDT")
	fees := NewFeesEngine(0.0010, 0.0020)
	exec := NewExecutionEngine(fees, risk)
	settle := NewSettlementEngine(nil)

	router := NewOMSRouter(sm, val, risk, matcher, exec, settle)

	risk.DepositAsset("buyer", "USDT", 100000.0)
	risk.DepositAsset("seller", "BTC", 2.0)

	// Place STOP_LIMIT order
	stopOrder := &AdvancedOrder{
		ID:        "stop_1",
		UserID:    "buyer",
		Symbol:    "BTC-USDT",
		Side:      "BUY",
		Type:      "STOP_LIMIT",
		Price:     51000.0,
		StopPrice: 51000.0,
		Quantity:  0.1,
	}

	err := router.ProcessIncomingOrder(context.Background(), stopOrder, "USDT", "BTC", "127.0.0.1", "WEB")
	if err != nil {
		t.Fatalf("Failed to process stop order: %v", err)
	}

	// Assert registered in matcher stop orders
	if len(matcher.StopOrders) != 1 {
		t.Errorf("Expected 1 stop order registered in Matcher, got %d", len(matcher.StopOrders))
	}

	// Place a regular order to create a trade at 51500 to trigger the stop
	sell := &AdvancedOrder{
		ID:       "sell_1",
		UserID:   "seller",
		Symbol:   "BTC-USDT",
		Side:     "SELL",
		Type:     "LIMIT",
		Price:    51500.0,
		Quantity: 0.1,
	}
	_ = router.ProcessIncomingOrder(context.Background(), sell, "USDT", "BTC", "127.0.0.1", "WEB")

	buy := &AdvancedOrder{
		ID:       "buy_1",
		UserID:   "buyer",
		Symbol:   "BTC-USDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Price:    51500.0,
		Quantity: 0.1,
	}
	_ = router.ProcessIncomingOrder(context.Background(), buy, "USDT", "BTC", "127.0.0.1", "WEB")

	// The stop order should be triggered as price is now 51500 >= 51000
	// Wait a brief moment or trigger stop orders directly
	matcher.triggerStopOrders(51500.0)

	if len(matcher.StopOrders) != 0 {
		t.Errorf("Expected stop order to be triggered, but still active: %d", len(matcher.StopOrders))
	}
}
