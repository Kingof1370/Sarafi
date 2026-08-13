package engine

import (
	"context"
	"testing"
	"time"
)

func TestMarketRegistryIsolatedMatching(t *testing.T) {
	// 1. Initialize full components with a MarketRegistry and isolated matchers
	sm := NewOMSStateMachine()
	val := NewOMSValidator()
	risk := NewRiskEngine(10.0, 100000.0)
	fees := NewFeesEngine(0.0010, 0.0020)
	exec := NewExecutionEngine(fees, risk)
	settle := NewSettlementEngine(nil)

	// Register trading pairs in validator
	val.RegisterTradingPair(&TradingPairConfig{
		Symbol:               "BTC-USDT",
		MinOrderSize:         0.001,
		MaxOrderSize:         100.0,
		MinNotional:          1.0,
		MaxNotional:          1000000.0,
		TickSize:             0.1,
		StepSize:             0.001,
		IsActive:             true,
		PriceBandPercentage:  0.20,
	})
	val.RegisterTradingPair(&TradingPairConfig{
		Symbol:               "ETH-USDT",
		MinOrderSize:         0.01,
		MaxOrderSize:         1000.0,
		MinNotional:          1.0,
		MaxNotional:          1000000.0,
		TickSize:             0.05,
		StepSize:             0.01,
		IsActive:             true,
		PriceBandPercentage:  0.20,
	})

	btcMatcher := NewMatcher("BTC-USDT")
	ethMatcher := NewMatcher("ETH-USDT")

	router := NewOMSRouter(sm, val, risk, btcMatcher, exec, settle)
	router.GetMarketRegistry().RegisterMatcher("ETH-USDT", ethMatcher)

	// Assert registry lookup works correctly
	if router.GetMatcherForSymbol("BTC-USDT") != btcMatcher {
		t.Error("Failed to look up registered BTC-USDT matcher")
	}
	if router.GetMatcherForSymbol("ETH-USDT") != ethMatcher {
		t.Error("Failed to look up registered ETH-USDT matcher")
	}

	// Pre-seed user balances in RiskEngine to pass pre-trade risk controls
	router.risk.DepositAsset("usr_seller_1", "BTC", 10.0)
	router.risk.DepositAsset("usr_buyer_1", "USDT", 1000000.0)

	// 2. Submit resting Sell order in BTC-USDT book
	btcSellOrder := &AdvancedOrder{
		ID:            "ord_btc_sell",
		UserID:        "usr_seller_1",
		Symbol:        "BTC-USDT",
		Side:          "SELL",
		Type:          "LIMIT",
		Price:         98000.0,
		Quantity:      1.0,
		Status:        StatusOMS_Created,
		TimeInForce:   TIF_GTC,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	err := router.ProcessIncomingOrder(context.Background(), btcSellOrder, "USDT", "BTC", "127.0.0.1", "TEST_RUN")
	if err != nil {
		t.Fatalf("Failed to process BTC sell order: %v", err)
	}

	// 3. Submit dynamic crossing Buy order in ETH-USDT book with same crossing price (98000.0)
	// If symbol isolation was broken, this might incorrectly match against the BTC sell order!
	ethBuyOrder := &AdvancedOrder{
		ID:            "ord_eth_buy",
		UserID:        "usr_buyer_1",
		Symbol:        "ETH-USDT",
		Side:          "BUY",
		Type:          "LIMIT",
		Price:         98000.0,
		Quantity:      1.0,
		Status:        StatusOMS_Created,
		TimeInForce:   TIF_GTC,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	err = router.ProcessIncomingOrder(context.Background(), ethBuyOrder, "USDT", "ETH", "127.0.0.1", "TEST_RUN")
	if err != nil {
		t.Fatalf("Failed to process ETH buy order: %v", err)
	}

	// 4. Assert that ETH-USDT book has the Buy order resting, and did NOT execute against BTC-USDT Sell order
	ethL2 := ethMatcher.GetL2Depth(5)
	if len(ethL2.Bids) == 0 || ethL2.Bids[0].Price != 98000.0 {
		t.Error("Expected ETH buy order to rest on ETH-USDT bid side book")
	}

	btcL2 := btcMatcher.GetL2Depth(5)
	if len(btcL2.Asks) == 0 || btcL2.Asks[0].Price != 98000.0 {
		t.Error("Expected BTC sell order to remain resting on BTC-USDT ask side book")
	}

	// Assert zero trades occurred since they are completely isolated
	if len(router.GetRecentTrades()) > 0 {
		t.Errorf("Unexpected trades matched between isolated BTC-USDT and ETH-USDT books: %+v", router.GetRecentTrades())
	}
}
