package engine

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"
	"velyxora/packages/types"
)

func TestRiskEngine(t *testing.T) {
	re := NewRiskEngine(10.0, 100000.0)
	userID := "usr_risk_1"

	// Initial balance setup
	re.DepositAsset(userID, "USDT", 1000.0)
	re.DepositAsset(userID, "BTC", 1.5)

	// Test available balance
	if re.GetAvailableBalance(userID, "USDT") != 1000.0 {
		t.Errorf("Expected 1000.0 available USDT, got %f", re.GetAvailableBalance(userID, "USDT"))
	}

	// Validate buy order with sufficient funds
	buyOrder := &types.Order{
		ID:        "ord_b1",
		UserID:    userID,
		Symbol:    "BTC-USDT",
		Side:      types.SideBuy,
		Type:      types.TypeLimit,
		Price:     500.0,
		Quantity:  1.0,
		FilledQty: 0.0,
	}

	err := re.ValidateOrder(buyOrder, "USDT", "BTC", 0.002) // Fee rate 0.2%
	if err != nil {
		t.Fatalf("Validation should pass, got error: %v", err)
	}

	// Verify hold was applied (500 * 1.002 = 501 USDT)
	if re.GetAvailableBalance(userID, "USDT") != 499.0 {
		t.Errorf("Expected 499.0 available USDT after hold, got %f", re.GetAvailableBalance(userID, "USDT"))
	}

	// Validate order exceeding balance should fail
	badOrder := &types.Order{
		ID:        "ord_b2",
		UserID:    userID,
		Symbol:    "BTC-USDT",
		Side:      types.SideBuy,
		Type:      types.TypeLimit,
		Price:     600.0,
		Quantity:  1.0,
		FilledQty: 0.0,
	}
	err = re.ValidateOrder(badOrder, "USDT", "BTC", 0.002)
	if err == nil {
		t.Error("Validation should fail due to insufficient funds")
	}
}

func TestFeesEngine(t *testing.T) {
	fe := NewFeesEngine(0.0010, 0.0020)
	userID := "usr_fees_1"

	// Default fees (Tier 0)
	m, tk := fe.GetFeeRates(userID)
	if m != 0.0010 || tk != 0.0020 {
		t.Errorf("Expected default 0.1%% maker and 0.2%% taker, got %f and %f", m, tk)
	}

	// High volume VIP Tier 2 (> 1M volume)
	fe.SetUserVolume(userID, 1500000.0)
	m, tk = fe.GetFeeRates(userID)
	if m != 0.0002 || tk != 0.0010 {
		t.Errorf("Expected VIP Tier 2 fees (0.02%%, 0.10%%), got %f and %f", m, tk)
	}

	// Calculate fee
	fee := fe.CalculateFee(userID, 50000.0, 0.5, false) // 25,000 * 0.10% = 25 USDT
	if fee != 25.0 {
		t.Errorf("Expected calculated fee of 25.0, got %f", fee)
	}
}

func TestOMS(t *testing.T) {
	oms := NewOMS()
	ord := &types.Order{
		ID:        "ord_oms_1",
		UserID:    "usr_oms_1",
		Symbol:    "BTC-USDT",
		Side:      types.SideBuy,
		Type:      types.TypeLimit,
		Price:     50000.0,
		Quantity:  0.1,
		Status:    types.StatusNew,
	}

	err := oms.TrackOrder(ord)
	if err != nil {
		t.Fatalf("Failed to track order: %v", err)
	}

	// Fetch order
	fetched, err := oms.GetOrder("ord_oms_1")
	if err != nil || fetched.ID != ord.ID {
		t.Errorf("Failed to retrieve tracked order")
	}

	// Update order state
	updated, err := oms.UpdateOrderState("ord_oms_1", types.StatusPartiallyFilled, 0.05)
	if err != nil || updated.Status != types.StatusPartiallyFilled || updated.FilledQty != 0.05 {
		t.Errorf("Failed to transition order state")
	}
}

func TestMatcher(t *testing.T) {
	matcher := NewMatcher("BTC-USDT")

	sell := &types.Order{
		ID:        "s_1",
		UserID:    "usr_s",
		Symbol:    "BTC-USDT",
		Side:      types.SideSell,
		Type:      types.TypeLimit,
		Price:     50000.0,
		Quantity:  1.0,
	}

	buy := &types.Order{
		ID:        "b_1",
		UserID:    "usr_b",
		Symbol:    "BTC-USDT",
		Side:      types.SideBuy,
		Type:      types.TypeLimit,
		Price:     50000.0,
		Quantity:  0.8,
	}

	// Match sell with buy
	matcher.MatchOrder(sell)
	trades := matcher.MatchOrder(buy)

	if len(trades) != 1 {
		t.Fatalf("Expected 1 execution trade match, got %d", len(trades))
	}

	if trades[0].Quantity != 0.8 || trades[0].Price != 50000.0 {
		t.Errorf("Unexpected match price/qty")
	}

	// Export depth L2
	depth := matcher.GetL2Depth(5)
	if len(depth.Asks) != 1 {
		t.Fatalf("Expected 1 Ask, got %d", len(depth.Asks))
	}
	qtyDiff := depth.Asks[0].Quantity - 0.2
	if qtyDiff < -1e-9 || qtyDiff > 1e-9 {
		t.Errorf("L2 depth ask mismatch: quantity %f is not 0.2", depth.Asks[0].Quantity)
	}

	// Cancel remaining ask
	success := matcher.CancelOrder("s_1")
	if !success {
		t.Error("Failed to cancel active order")
	}
}

func TestExecutionEngine(t *testing.T) {
	fe := NewFeesEngine(0.0010, 0.0020)
	re := NewRiskEngine(10.0, 100000.0)
	ee := NewExecutionEngine(fe, re)

	buyerID := "usr_b_exec"
	sellerID := "usr_s_exec"

	re.DepositAsset(buyerID, "USDT", 60000.0)
	re.DepositAsset(sellerID, "BTC", 1.0)

	// Place buyer bid hold
	buyOrder := &types.Order{
		ID:        "b_exec",
		UserID:    buyerID,
		Symbol:    "BTC-USDT",
		Side:      types.SideBuy,
		Type:      types.TypeLimit,
		Price:     50000.0,
		Quantity:  1.0,
	}
	re.ValidateOrder(buyOrder, "USDT", "BTC", 0.002)

	// Create match trade
	trade := &types.Trade{
		ID:        "t_exec_1",
		Symbol:    "BTC-USDT",
		BuyerID:   buyerID,
		SellerID:  sellerID,
		Price:     50000.0,
		Quantity:  1.0,
	}

	executions := ee.ProcessTrades([]*types.Trade{trade}, "USDT", "BTC")
	if len(executions) != 1 {
		t.Fatalf("Expected 1 trade execution record")
	}

	exec := executions[0]
	if exec.BuyerFee != 100.0 || exec.SellerFee != 50.0 {
		t.Errorf("Fees calculation mismatch: buyer fee %f, seller fee %f", exec.BuyerFee, exec.SellerFee)
	}

	// Check updated available balances
	// Buyer had 60,000 USDT. Lost (50,000 + 100 fee) = 9,900 available USDT
	if re.GetAvailableBalance(buyerID, "USDT") != 9900.0 {
		t.Errorf("Buyer available USDT mismatch: got %f", re.GetAvailableBalance(buyerID, "USDT"))
	}
	if re.GetAvailableBalance(buyerID, "BTC") != 1.0 {
		t.Errorf("Buyer available BTC mismatch: got %f", re.GetAvailableBalance(buyerID, "BTC"))
	}
}

func TestPositionEngine(t *testing.T) {
	pe := NewPositionEngine()
	userID := "usr_p_1"

	// Buy 1.5 BTC at 40,000
	pe.RecordExecution(userID, "BTC-USDT", 1.5, 40000.0)
	pos := pe.GetPosition(userID, "BTC-USDT")
	if pos.Size != 1.5 || pos.EntryPrice != 40000.0 {
		t.Errorf("Position mismatch after first record: size %f, entry %f", pos.Size, pos.EntryPrice)
	}

	// Buy another 0.5 BTC at 44,000 -> entry price should be average: (1.5*40k + 0.5*44k) / 2 = 41,000
	pe.RecordExecution(userID, "BTC-USDT", 0.5, 44000.0)
	pos = pe.GetPosition(userID, "BTC-USDT")
	if pos.Size != 2.0 || pos.EntryPrice != 41000.0 {
		t.Errorf("Average entry price calculation mismatch: size %f, entry %f", pos.Size, pos.EntryPrice)
	}
}

func TestMarketServices(t *testing.T) {
	ms := NewMarketServices()
	ms.RecordTrade("BTC-USDT", 42000.0, 0.5)
	ms.RecordTrade("BTC-USDT", 43000.0, 1.0)

	tick := ms.GetTicker("BTC-USDT")
	if tick.LastPrice != 43000.0 || tick.High24H != 43000.0 || tick.Low24H != 42000.0 || tick.Volume24H != 1.5 {
		t.Errorf("Ticker stats mismatch")
	}
}

func TestSettlementEngine(t *testing.T) {
	se := NewSettlementEngine(nil) // Test offline memory tracker
	exec := &Execution{
		TradeID:   "t_se_1",
		Symbol:    "BTC-USDT",
		Price:     50000.0,
		Quantity:  1.0,
		BuyerID:   "b_se",
		SellerID:  "s_se",
		BuyerFee:  100.0,
		SellerFee: 50.0,
	}

	err := se.SettleExecution(context.Background(), exec, "BTC", "USDT")
	if err != nil {
		t.Fatalf("Settlement should pass, got: %v", err)
	}

	if se.GetSettledCount() != 1 {
		t.Errorf("Settled count mismatch, got %d", se.GetSettledCount())
	}
}

func BenchmarkMatchingEnginePipeline(b *testing.B) {
	matcher := NewMatcher("BTC-USDT")

	sellOrder := &types.Order{
		ID:       "sell_bench_1",
		UserID:   "usr_sell",
		Symbol:   "BTC-USDT",
		Side:     types.SideSell,
		Type:     types.TypeLimit,
		Price:    50000.0,
		Quantity: 1000000.0,
	}
	matcher.MatchOrder(sellOrder)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buyOrder := &types.Order{
			ID:       "buy_bench",
			UserID:   "usr_buy",
			Symbol:   "BTC-USDT",
			Side:     types.SideBuy,
			Type:     types.TypeLimit,
			Price:    50000.0,
			Quantity: 1.0,
		}
		matcher.MatchOrder(buyOrder)
	}
}

func TestMatchingLatencyPercentiles(t *testing.T) {
	// A dedicated functional test that measures and outputs exact latency percentiles (P50, P95, P99)
	matcher := NewMatcher("BTC-USDT")

	sellOrder := &types.Order{
		ID:       "sell_p_1",
		UserID:   "usr_sell",
		Symbol:   "BTC-USDT",
		Side:     types.SideSell,
		Type:     types.TypeLimit,
		Price:    50000.0,
		Quantity: 1000000.0,
	}
	matcher.MatchOrder(sellOrder)

	iterations := 1000
	durations := make([]time.Duration, iterations)

	for i := 0; i < iterations; i++ {
		buyOrder := &types.Order{
			ID:       "buy_p_" + strconv.Itoa(i),
			UserID:   "usr_buy",
			Symbol:   "BTC-USDT",
			Side:     types.SideBuy,
			Type:     types.TypeLimit,
			Price:    50000.0,
			Quantity: 1.0,
		}

		start := time.Now()
		matcher.MatchOrder(buyOrder)
		durations[i] = time.Since(start)
	}

	// Sort durations
	for i := 0; i < len(durations); i++ {
		for j := i + 1; j < len(durations); j++ {
			if durations[i] > durations[j] {
				durations[i], durations[j] = durations[j], durations[i]
			}
		}
	}

	p50 := durations[int(float64(iterations)*0.50)]
	p95 := durations[int(float64(iterations)*0.95)]
	p99 := durations[int(float64(iterations)*0.99)]

	fmt.Printf("\n--- VELYXORA LATENCY PERCENTILES ---\n")
	fmt.Printf("P50 (Median) Latency: %v\n", p50)
	fmt.Printf("P95 Latency:          %v\n", p95)
	fmt.Printf("P99 Latency:          %v\n", p99)
	fmt.Printf("------------------------------------\n")
}

func TestAdvancedRiskValidationAndProtections(t *testing.T) {
	re := NewRiskEngine(10.0, 100000.0)
	userID := "usr_abuse_1"

	re.DepositAsset(userID, "USDT", 100000.0)

	// 1. Test Trading Halt
	re.SetHaltStatus(true)
	order := &types.Order{ID: "o_halt", UserID: userID, Symbol: "BTC-USDT", Side: types.SideBuy, Type: types.TypeLimit, Price: 500.0, Quantity: 1.0}
	err := re.ValidateOrder(order, "USDT", "BTC", 0.002)
	if err == nil || err.Error() != "trading is currently halted globally" {
		t.Error("Expected trading halt validation block error")
	}
	re.SetHaltStatus(false)

	// 2. Test Blocklist
	re.BlockAccount(userID, true)
	err = re.ValidateOrder(order, "USDT", "BTC", 0.002)
	if err == nil || err.Error() != "user account usr_abuse_1 is blocked due to risk violations" {
		t.Error("Expected blocked account validation block error")
	}
	re.BlockAccount(userID, false)

	// 3. Test Spam Rate Fire Protection
	for i := 0; i < 11; i++ {
		ord := &types.Order{ID: "o_spam_" + strconv.Itoa(i), UserID: userID, Symbol: "BTC-USDT", Side: types.SideBuy, Type: types.TypeLimit, Price: 500.0, Quantity: 0.1}
		err = re.ValidateOrder(ord, "USDT", "BTC", 0.002)
		if i == 10 && err == nil {
			t.Error("Expected spam protection rate limit exceed block error")
		}
	}
}

func TestSelfTradePreventionTriggers(t *testing.T) {
	re := NewRiskEngine(10.0, 100000.0)
	re.SetSTPMode(STP_CancelNewest)

	isSTP, mode := re.VerifySelfTrade("user_a", "user_a")
	if !isSTP || mode != STP_CancelNewest {
		t.Errorf("Expected Self-Trade triggered with CANCEL_NEWEST mode")
	}
}

func TestAssetReservationsAndPnL(t *testing.T) {
	pe := NewPositionEngine()
	userID := "user_portfolio"

	pe.ReserveAsset(userID, "USDT", 1000.0)
	pe.LockAsset(userID, "USDT", 500.0)
	pe.ReleaseLockedAsset(userID, "USDT", 200.0)

	// Buy 1 BTC at 40,000
	pe.RecordExecution(userID, "BTC-USDT", 1.0, 40000.0)
	// Sell 1 BTC at 42,000 -> Realized PnL should be +2000.0
	pe.RecordExecution(userID, "BTC-USDT", -1.0, 42000.0)

	pos := pe.GetPosition(userID, "BTC-USDT")
	if pos.RealizedPnL != 2000.0 {
		t.Errorf("Expected 2000.0 realized PnL, got %f", pos.RealizedPnL)
	}
}
