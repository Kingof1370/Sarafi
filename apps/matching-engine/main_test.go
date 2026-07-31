package main

import (
	"testing"
	"velyxora/packages/types"
)

func TestOrderBookMatching(t *testing.T) {
	ob := NewOrderBook("BTC-USDT")

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

	ob.ProcessLimitOrder(sellOrder)

	if len(ob.Asks) != 1 {
		t.Errorf("Expected 1 Ask limit level, got %d", len(ob.Asks))
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

	trades := ob.ProcessLimitOrder(buyOrder)

	if len(trades) != 1 {
		t.Fatalf("Expected 1 match trade, got %d", len(trades))
	}

	trade := trades[0]
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
