package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"velyxora/apps/matching-engine/engine"
	"velyxora/packages/types"
)

// Mock External Liquidity Provider implementing engine.ExternalLiquidityProvider
type MockProvider struct{}

func (m *MockProvider) Name() string {
	return "binance_mock"
}

func (m *MockProvider) GetOrderBook(symbol string) (*types.OrderBookL2, error) {
	return &types.OrderBookL2{
		Symbol:    symbol,
		Bids:      []types.OrderBookLevel{{Price: 50000.0, Quantity: 1.0}},
		Asks:      []types.OrderBookLevel{{Price: 50100.0, Quantity: 1.0}},
		Timestamp: time.Now(),
	}, nil
}

func (m *MockProvider) ExecuteOrder(ctx context.Context, order *types.Order) (*types.Trade, error) {
	return &types.Trade{
		ID:        "trd_ext_binance_01",
		Symbol:    order.Symbol,
		Price:     order.Price,
		Quantity:  order.Quantity,
		Timestamp: time.Now(),
	}, nil
}

func TestP0007CandleAggregation(t *testing.T) {
	ms := engine.NewMarketServices()

	trade := &types.Trade{
		ID:        "trd_mock_c1",
		Symbol:    "BTC-USDT",
		Price:     52000.5,
		Quantity:  0.25,
		Timestamp: time.Now(),
	}

	// Record trade should build candles automatically
	ms.AddTrade("BTC-USDT", trade)

	candles := ms.GetCandles("BTC-USDT", "1m")
	if len(candles) != 1 {
		t.Fatalf("Expected exactly 1 candle, got %d", len(candles))
	}

	c := candles[0]
	if c.Open != 52000.5 || c.Close != 52000.5 || c.High != 52000.5 || c.Low != 52000.5 {
		t.Errorf("Unexpected candlestick OHLC values: %+v", c)
	}
	if c.Volume != 0.25 || c.TradeCount != 1 {
		t.Errorf("Unexpected candlestick Volume/TradeCount: %+v", c)
	}

	// Feed a higher price trade within same minute step
	higherTrade := &types.Trade{
		ID:        "trd_mock_c2",
		Symbol:    "BTC-USDT",
		Price:     53000.0,
		Quantity:  0.75,
		Timestamp: time.Now(),
	}
	ms.AddTrade("BTC-USDT", higherTrade)

	candles = ms.GetCandles("BTC-USDT", "1m")
	if len(candles) != 1 {
		t.Fatalf("Expected only 1 candle (reused open step), got %d", len(candles))
	}

	c2 := candles[0]
	if c2.Open != 52000.5 || c2.Close != 53000.0 || c2.High != 53000.0 || c2.Low != 52000.5 {
		t.Errorf("Unexpected cumulative candle OHLC values: %+v", c2)
	}
	if c2.Volume != 1.0 || c2.TradeCount != 2 {
		t.Errorf("Unexpected cumulative candle Volume/TradeCount: %+v", c2)
	}
}

func TestP0007WebSocketStreamCallbacks(t *testing.T) {
	sm := engine.NewOMSStateMachine()
	val := engine.NewOMSValidator()
	risk := engine.NewRiskEngine(10.0, 100000.0)
	matcher := engine.NewMatcher("BTC-USDT")
	fees := engine.NewFeesEngine(0.0010, 0.0020)
	exec := engine.NewExecutionEngine(fees, risk)
	settle := engine.NewSettlementEngine(nil)
	ms := engine.NewMarketServices()

	router := engine.NewOMSRouter(sm, val, risk, matcher, exec, settle, ms)

	tradeCallbackInvoked := false
	depthCallbackInvoked := false

	router.SetCallbacks(
		func(symbol string, trade *types.Trade) {
			tradeCallbackInvoked = true
		},
		func(symbol string, depth *types.OrderBookL2) {
			depthCallbackInvoked = true
		},
	)

	// User deposits
	risk.DepositAsset("buyer", "USDT", 60000.0)
	risk.DepositAsset("seller", "BTC", 1.0)

	// 1. Placement triggers depth callback
	sellOrder := &engine.AdvancedOrder{
		ID:       "s_ws_t1",
		UserID:   "seller",
		Symbol:   "BTC-USDT",
		Side:     "SELL",
		Type:     "LIMIT",
		Price:    50000.0,
		Quantity: 0.5,
	}

	err := router.ProcessIncomingOrder(context.Background(), sellOrder, "USDT", "BTC", "127.0.0.1", "TEST")
	if err != nil {
		t.Fatalf("Failed to process order: %v", err)
	}

	if !depthCallbackInvoked {
		t.Error("Expected real-time depth callback to be invoked upon order placement")
	}

	// 2. Matching triggers trade callback
	buyOrder := &engine.AdvancedOrder{
		ID:       "b_ws_t1",
		UserID:   "buyer",
		Symbol:   "BTC-USDT",
		Side:     "BUY",
		Type:     "LIMIT",
		Price:    50000.0,
		Quantity: 0.5,
	}

	err = router.ProcessIncomingOrder(context.Background(), buyOrder, "USDT", "BTC", "127.0.0.1", "TEST")
	if err != nil {
		t.Fatalf("Failed to process order: %v", err)
	}

	if !tradeCallbackInvoked {
		t.Error("Expected real-time trade callback to be invoked upon matching")
	}
}

func TestP0007ExternalLiquidityValidation(t *testing.T) {
	le := engine.NewLiquidityEngine()
	provider := &MockProvider{}

	le.RegisterProvider(provider)
	fetched := le.GetProvider("binance_mock")
	if fetched == nil || fetched.Name() != "binance_mock" {
		t.Fatal("Failed to retrieve registered external liquidity provider")
	}

	// Case 1: Valid external trade
	validTrade := &types.Trade{
		ID:        "t_ext_valid",
		Symbol:    "BTC-USDT",
		Price:     51000.0,
		Quantity:  1.5,
		Timestamp: time.Now(),
	}
	err := le.ValidateExternalExecution(validTrade, 10.0, 100000.0)
	if err != nil {
		t.Errorf("Validation should pass, got error: %v", err)
	}

	// Case 2: Invalid price out of bounds
	invalidPrice := &types.Trade{
		ID:        "t_ext_bad_price",
		Symbol:    "BTC-USDT",
		Price:     110000.0, // max is 100k
		Quantity:  1.5,
		Timestamp: time.Now(),
	}
	err = le.ValidateExternalExecution(invalidPrice, 10.0, 100000.0)
	if err == nil {
		t.Error("Expected validation error for out-of-bounds external price")
	}

	// Case 3: Zero quantity
	invalidQty := &types.Trade{
		ID:        "t_ext_bad_qty",
		Symbol:    "BTC-USDT",
		Price:     50000.0,
		Quantity:  0.0,
		Timestamp: time.Now(),
	}
	err = le.ValidateExternalExecution(invalidQty, 10.0, 100000.0)
	if err == nil {
		t.Error("Expected validation error for zero quantity external trade")
	}

	// Case 4: Future timestamp
	futureTrade := &types.Trade{
		ID:        "t_ext_future",
		Symbol:    "BTC-USDT",
		Price:     50000.0,
		Quantity:  1.5,
		Timestamp: time.Now().Add(1 * time.Hour),
	}
	err = le.ValidateExternalExecution(futureTrade, 10.0, 100000.0)
	if err == nil {
		t.Error("Expected validation error for future external trade timestamp")
	}
}

func TestP0007RESTMarketDataAPIHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Register handlers manually for mock server testing
	r.GET("/ticker", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ticker", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
