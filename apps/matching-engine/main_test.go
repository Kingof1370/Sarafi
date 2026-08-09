package main

import (
	"context"
	"testing"
	"velyxora/apps/matching-engine/engine"
	"velyxora/packages/types"
)

func TestOrderBookMatching(t *testing.T) {
	// Initialize real Matcher components
	matcher := engine.NewMatcher("BTC-USDT")

	// Limit Ask (Seller)
	sellOrder := &types.Order{
		ID:        "sell_1",
		UserID:    "user_seller",
		Symbol:    "BTC-USDT",
		Side:      types.SideSell,
		Type:      types.TypeLimit,
		Price:     50000.0,
		Quantity:  1.5,
		FilledQty: 0.0,
	}

	// Route into real Matcher
	trades1 := matcher.MatchOrder(sellOrder)
	if len(trades1) != 0 {
		t.Errorf("Expected 0 trade matches initially, got %d", len(trades1))
	}

	// Limit Bid (Buyer matches the ask)
	buyOrder := &types.Order{
		ID:        "buy_1",
		UserID:    "user_buyer",
		Symbol:    "BTC-USDT",
		Side:      types.SideBuy,
		Type:      types.TypeLimit,
		Price:     50000.0,
		Quantity:  1.0,
		FilledQty: 0.0,
	}

	trades2 := matcher.MatchOrder(buyOrder)

	if len(trades2) != 1 {
		t.Fatalf("Expected exactly 1 match trade, got %d", len(trades2))
	}

	trade := trades2[0]
	if trade.Quantity != 1.0 {
		t.Errorf("Expected matching quantity to be 1.0, got %f", trade.Quantity)
	}

	if trade.Price != 50000.0 {
		t.Errorf("Expected trade execution price to be 50000.0, got %f", trade.Price)
	}

	if sellOrder.FilledQty != 1.0 {
		t.Errorf("Expected seller order to be partially filled with 1.0, got %f", sellOrder.FilledQty)
	}

	if buyOrder.Status != types.StatusFilled {
		t.Errorf("Expected buyer order status to be FILLED, got %s", buyOrder.Status)
	}
}

func TestOMSRoutingWithPostgresMocks(t *testing.T) {
	// Setup core routing components
	sm := engine.NewOMSStateMachine()
	val := engine.NewOMSValidator()
	risk := engine.NewRiskEngine(10.0, 100000.0)
	matcher := engine.NewMatcher("BTC-USDT")
	fees := engine.NewFeesEngine(0.0010, 0.0020)
	exec := engine.NewExecutionEngine(fees, risk)
	settle := engine.NewSettlementEngine(nil)

	omsRouter := engine.NewOMSRouter(sm, val, risk, matcher, exec, settle)

	// User deposits
	risk.DepositAsset("buyer", "USDT", 60000.0)
	risk.DepositAsset("seller", "BTC", 2.0)

	// Create and process incoming order
	sell := &engine.AdvancedOrder{
		ID:            "sell_adv_1",
		ClientOrderID: "cli_sell_1",
		UserID:        "seller",
		Symbol:        "BTC-USDT",
		Side:          "SELL",
		Type:          "LIMIT",
		Price:         50000.0,
		Quantity:      1.0,
	}

	err := omsRouter.ProcessIncomingOrder(context.Background(), sell, "USDT", "BTC", "127.0.0.1", "TEST")
	if err != nil {
		t.Fatalf("Failed to process advanced sell order: %v", err)
	}

	buy := &engine.AdvancedOrder{
		ID:            "buy_adv_1",
		ClientOrderID: "cli_buy_1",
		UserID:        "buyer",
		Symbol:        "BTC-USDT",
		Side:          "BUY",
		Type:          "LIMIT",
		Price:         50000.0,
		Quantity:      1.0,
	}

	err = omsRouter.ProcessIncomingOrder(context.Background(), buy, "USDT", "BTC", "127.0.0.1", "TEST")
	if err != nil {
		t.Fatalf("Failed to process advanced buy order: %v", err)
	}

	sOrder, _ := sm.GetOrder("sell_adv_1")
	bOrder, _ := sm.GetOrder("buy_adv_1")

	if sOrder.Status != engine.StatusOMS_Filled {
		t.Errorf("Expected sell order status FILLED, got %s", sOrder.Status)
	}
	if bOrder.Status != engine.StatusOMS_Filled {
		t.Errorf("Expected buy order status FILLED, got %s", bOrder.Status)
	}
}
