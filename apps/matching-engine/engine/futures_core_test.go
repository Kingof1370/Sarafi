package engine

import (
	"context"
	"math"
	"testing"
	"time"
	"velyxora/packages/types"
)

func TestFuturesLeveragedMarginReservation(t *testing.T) {
	// Initialize Risk Engine
	re := NewRiskEngine(0.01, 1000000.0)
	userID := "user_1"
	symbol := "BTC-USDT-PERP"

	// Set user balance: 1000 USDT
	re.DepositAsset(userID, "USDT", 1000.0)

	// Set user leverage to 50x
	re.SetUserLeverage(userID, symbol, 50.0)

	// Validate order: BUY_TO_OPEN 10 BTC at 4000 USDT (Total Notional: 40,000 USDT)
	// With 50x leverage, required margin is 40,000 / 50 = 800 USDT
	order := &types.Order{
		ID:        "ord_1",
		UserID:    userID,
		Symbol:    symbol,
		Side:      types.OrderSide("BUY_TO_OPEN"),
		Type:      types.TypeLimit,
		Price:     4000.0,
		Quantity:  10.0,
		Status:    types.StatusNew,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := re.ValidateOrder(order, "USDT", "BTC", 0.0)
	if err != nil {
		t.Fatalf("Failed to validate leveraged order: %v", err)
	}

	// Verify locked hold is exactly 800 USDT
	available := re.GetAvailableBalance(userID, "USDT")
	if math.Abs(available-200.0) > 1e-9 {
		t.Errorf("Expected available balance to be 200.0 USDT after 800 USDT margin reservation, got %f", available)
	}
}

func TestFuturesPositionAndPnLTracking(t *testing.T) {
	// Initialize Position Engine
	pe := NewPositionEngine()
	userID := "user_1"
	symbol := "BTC-USDT-PERP"

	// Create Long position of 2 BTC at 40,000 Entry Price with 20x leverage and 5% maintenance margin rate (mmRate=0.05)
	// Initial margin = (2 * 40,000) / 20 = 4,000 USDT
	pe.RecordFuturesExecution(userID, symbol, 2.0, 40000.0, 20.0, 4000.0, 0.05)

	pos := pe.GetPosition(userID, symbol)
	if pos.Size != 2.0 {
		t.Errorf("Expected position size 2.0, got %f", pos.Size)
	}
	if pos.EntryPrice != 40000.0 {
		t.Errorf("Expected entry price 40000.0, got %f", pos.EntryPrice)
	}
	if pos.Margin != 4000.0 {
		t.Errorf("Expected margin 4000.0, got %f", pos.Margin)
	}

	// Track Unrealized PnL with Mark Price at 42,000 USDT
	// Expected uPnL = 2.0 * (42,000 - 40,000) = 4,000 USDT (profit)
	upnl := pe.UpdateUnrealizedPnL(userID, symbol, 42000.0)
	if upnl != 4000.0 {
		t.Errorf("Expected Unrealized PnL to be 4000.0, got %f", upnl)
	}

	// Track Unrealized PnL with Mark Price at 38,000 USDT
	// Expected uPnL = 2.0 * (38,000 - 40,000) = -4,000 USDT (loss)
	upnl = pe.UpdateUnrealizedPnL(userID, symbol, 38000.0)
	if upnl != -4000.0 {
		t.Errorf("Expected Unrealized PnL to be -4000.0, got %f", upnl)
	}
}

func TestFuturesMarginRatioAndLiquidationPrice(t *testing.T) {
	// Setup position: Long 1 BTC at 40,000 USDT, 10x leverage, mmRate=0.05 (5%)
	// Initial Margin = 4,000 USDT
	pos := &Position{
		UserID:                "user_1",
		Symbol:                "BTC-USDT-PERP",
		Size:                  1.0,
		EntryPrice:            40000.0,
		Margin:                4000.0,
		Leverage:              10.0,
		MaintenanceMarginRate: 0.05,
	}

	// 1. Calculate Bankruptcy Price
	// Expected Long Bankruptcy Price: 40,000 * (1 - 1/10) = 36,000 USDT
	bPrice := pos.CalculateBankruptcyPrice()
	if bPrice != 36000.0 {
		t.Errorf("Expected bankruptcy price 36000.0, got %f", bPrice)
	}

	// 2. Calculate uPnL and Collateral at Mark Price 38,000 USDT
	// uPnL = 1 * (38,000 - 40,000) = -2,000 USDT
	// Collateral = 4,000 - 2,000 = 2,000 USDT
	collateral := pos.CalculateCollateral(38000.0)
	if collateral != 2000.0 {
		t.Errorf("Expected collateral to be 2000.0 at mark price 38000, got %f", collateral)
	}

	// 3. Maintenance Margin at 38,000 USDT
	// MM = |Size| * markPrice * mmRate = 1.0 * 38,000 * 0.05 = 1,900 USDT
	mm := pos.CalculateMaintenanceMargin(38000.0)
	if mm != 1900.0 {
		t.Errorf("Expected maintenance margin to be 1900.0 at mark price 38000, got %f", mm)
	}

	// 4. Margin Ratio at 38,000 USDT
	// Margin Ratio = MM / Collateral = 1,900 / 2,000 = 0.95
	mRatio := pos.CalculateMarginRatio(38000.0)
	if mRatio != 0.95 {
		t.Errorf("Expected Margin Ratio to be 0.95, got %f", mRatio)
	}

	// 5. Calculate Liquidation Price at 38,000 USDT
	lPrice := pos.CalculateLiquidationPrice(38000.0)
	expectedLPrice := 40000.0 * (1.0 - 1.0/10.0) / (1.0 - 0.05) // 37894.736842
	if math.Abs(lPrice-expectedLPrice) > 1e-4 {
		t.Errorf("Expected liquidation price to be %f, got %f", expectedLPrice, lPrice)
	}
}

func TestForceLiquidationTriggerAndInsuranceFund(t *testing.T) {
	// Initialize components
	pe := NewPositionEngine()
	re := NewRiskEngine(0.01, 1000000.0)
	re.SetPositionEngine(pe)

	userID := "user_1"
	symbol := "BTC-USDT-PERP"

	// User position: Long 1.0 BTC at 40,000 USDT, 10x leverage, mmRate=0.05 (5%)
	// Margin = 4,000 USDT
	pe.RecordFuturesExecution(userID, symbol, 1.0, 40000.0, 10.0, 4000.0, 0.05)

	cancelCalled := false
	placeCalled := false
	var liqPrice float64
	var liqSize float64

	re.SetForceLiquidationCallbacks(func(uID, sym string) {
		if uID == userID && sym == symbol {
			cancelCalled = true
		}
	}, func(uID, sym string, bankruptcyPrice, size float64) {
		if uID == userID && sym == symbol {
			placeCalled = true
			liqPrice = bankruptcyPrice
			liqSize = size
		}
	})

	// Set Mark Price to 37,000 USDT (Collateral falls to 1,000. Maintenance margin is 1,850. Margin Ratio = 1.85 >= 1.0)
	// This should trigger Force Liquidation!
	re.SetReferencePrice(symbol, 37000.0)

	if !cancelCalled {
		t.Error("Expected cancel open orders callback to be triggered during Force Liquidation")
	}

	if !placeCalled {
		t.Error("Expected place liquidation order callback to be triggered during Force Liquidation")
	}

	// Verify Bankruptcy Price for Long 1.0 BTC at 40,000 USDT is 36,000 USDT
	if liqPrice != 36000.0 {
		t.Errorf("Expected liquidation order to be placed at Bankruptcy Price 36000.0, got %f", liqPrice)
	}

	if liqSize != 1.0 {
		t.Errorf("Expected liquidation order size 1.0, got %f", liqSize)
	}

	// Test Insurance Fund surplus transfer:
	// If the liquidation order fills at 36,500 USDT (better than Bankruptcy Price of 36,000 USDT for SELL)
	// Excess = (36,500 - 36,000) * 1.0 = 500 USDT
	// Setup standard order router to process trade matches
	sm := NewOMSStateMachine()
	val := NewOMSValidator()

	// Register Perpetual symbol in validator
	val.RegisterTradingPair(&TradingPairConfig{
		Symbol:            symbol,
		MinOrderSize:      0.0001,
		MaxOrderSize:      1000.0,
		MinNotional:       5.0,
		MaxNotional:       10000000.0,
		TickSize:          0.01,
		StepSize:          0.0001,
		PricePrecision:    2,
		QuantityPrecision: 4,
		IsActive:          true,
	})

	matcher := NewMatcher(symbol)
	fees := NewFeesEngine(0.001, 0.002)
	exec := NewExecutionEngine(fees, re)
	settle := NewSettlementEngine(nil)

	router := NewOMSRouter(sm, val, re, matcher, exec, settle)
	router.SetPositionEngine(pe)

	// Set up liquidation callbacks to route to OMSRouter
	re.SetForceLiquidationCallbacks(func(uID, sym string) {
		router.MassCancel(sym, uID, "SYSTEM", "SYSTEM")
	}, func(uID, sym string, bankruptcyPrice, size float64) {
		liqOrder := &AdvancedOrder{
			ID:            "liq_order_123",
			ClientOrderID: "liq_client",
			UserID:        "LIQUIDATION_ENGINE",
			Symbol:        sym,
			Side:          "SELL",
			Type:          "LIMIT",
			Price:         bankruptcyPrice,
			Quantity:      size,
			Status:        StatusOMS_Created,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		_ = router.ProcessIncomingOrder(context.Background(), liqOrder, "USDT", "BTC", "SYSTEM", "SYSTEM")
	})

	// Match a BUY order at 36,500 USDT to execute the liquidation order better than Bankruptcy Price
	buyOrder := &AdvancedOrder{
		ID:            "buy_order_opp",
		ClientOrderID: "buy_client_opp",
		UserID:        "user_opp",
		Symbol:        symbol,
		Side:          "BUY",
		Type:          "LIMIT",
		Price:         36500.0,
		Quantity:      1.0,
		Status:        StatusOMS_Created,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Seed balance for buyer
	re.DepositAsset("user_opp", "USDT", 100000.0)

	// Place opposition order on the book
	_ = router.ProcessIncomingOrder(context.Background(), buyOrder, "USDT", "BTC", "SYSTEM", "SYSTEM")

	// Trigger liquidation
	pe.RecordFuturesExecution(userID, symbol, 1.0, 40000.0, 10.0, 4000.0, 0.05)
	re.SetReferencePrice(symbol, 37000.0)

	// Since we execute non-blocking, wait a tiny bit for match execution
	time.Sleep(50 * time.Millisecond)

	// Verify surplus transferred to the futures insurance fund
	balance := re.GetInsuranceFundBalance("USDT")
	if balance != 500.0 {
		t.Errorf("Expected USDT futures insurance fund balance to be 500.0, got %f", balance)
	}
}

func TestFuturesPositionReversalFlip(t *testing.T) {
	pe := NewPositionEngine()
	userID := "user_1"
	symbol := "BTC-USDT-PERP"

	// 1. Create a Long position: 1.0 BTC at 40,000 with 10x leverage
	// Margin = 4,000 USDT
	pe.RecordFuturesExecution(userID, symbol, 1.0, 40000.0, 10.0, 4000.0, 0.05)

	pos := pe.GetPosition(userID, symbol)
	if pos.Size != 1.0 || pos.Margin != 4000.0 {
		t.Fatalf("Initial long position failed to setup correctly")
	}

	// 2. Flip position: sell 3.0 BTC at 41,000 USDT (Net: Short 2.0 BTC)
	// This closes the 1.0 BTC long (with 1,000 USDT profit) and opens a 2.0 BTC short.
	// Expected new Short position: Size = -2.0, EntryPrice = 41000.0, Margin = (2.0 * 41000) / 10 = 8200.0 USDT
	pe.RecordFuturesExecution(userID, symbol, -3.0, 41000.0, 10.0, 0.0, 0.05)

	flippedPos := pe.GetPosition(userID, symbol)
	if flippedPos.Size != -2.0 {
		t.Errorf("Expected flipped position size -2.0, got %f", flippedPos.Size)
	}
	if flippedPos.EntryPrice != 41000.0 {
		t.Errorf("Expected flipped position entry price 41000.0, got %f", flippedPos.EntryPrice)
	}
	if flippedPos.Margin != 8200.0 {
		t.Errorf("Expected flipped position margin 8200.0, got %f", flippedPos.Margin)
	}
	if flippedPos.RealizedPnL != 1000.0 {
		t.Errorf("Expected realized PnL of 1000.0 (from closing 1.0 BTC long at 41,000 vs entry 40,000), got %f", flippedPos.RealizedPnL)
	}
}
