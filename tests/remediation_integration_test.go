package tests

import (
	"context"
	"testing"
	"time"

	"velyxora/apps/matching-engine/engine"
	"velyxora/packages/common"
	"velyxora/packages/security"
)

func TestFullRemediationSystemIntegration(t *testing.T) {
	// 1. Initialize state-machine and validator
	sm := engine.NewOMSStateMachine()
	val := engine.NewOMSValidator()
	risk := engine.NewRiskEngine(10.0, 1000000.0)
	fees := engine.NewFeesEngine(0.0010, 0.0020)
	exec := engine.NewExecutionEngine(fees, risk)
	settle := engine.NewSettlementEngine(nil)

	// Register multiple trading pairs to verify isolated Multi-Symbol matching core!
	val.RegisterTradingPair(&engine.TradingPairConfig{
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
	val.RegisterTradingPair(&engine.TradingPairConfig{
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

	btcMatcher := engine.NewMatcher("BTC-USDT")
	ethMatcher := engine.NewMatcher("ETH-USDT")

	router := engine.NewOMSRouter(sm, val, risk, btcMatcher, exec, settle)
	router.GetMarketRegistry().RegisterMatcher("ETH-USDT", ethMatcher)

	// 2. Setup user balances
	buyerID := "usr_buyer_alice"
	sellerID := "usr_seller_bob"

	risk.DepositAsset(buyerID, "USDT", 200000.0)
	risk.DepositAsset(sellerID, "BTC", 2.5)

	// 3. Place resting Sell order in BTC-USDT
	sellOrder := &engine.AdvancedOrder{
		ID:            "ord_btc_sell_leg",
		UserID:        sellerID,
		Symbol:        "BTC-USDT",
		Side:          "SELL",
		Type:          "LIMIT",
		Price:         95000.0,
		Quantity:      2.0,
		Status:        engine.StatusOMS_Created,
		TimeInForce:   engine.TIF_GTC,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ClientOrderID: "cli_seller_bob_123",
	}

	err := router.ProcessIncomingOrder(context.Background(), sellOrder, "USDT", "BTC", "127.0.0.1", "TEST_CLIENT")
	if err != nil {
		t.Fatalf("Failed to process resting sell order: %v", err)
	}

	// 4. Place crossing Buy order in BTC-USDT
	buyOrder := &engine.AdvancedOrder{
		ID:            "ord_btc_buy_leg",
		UserID:        buyerID,
		Symbol:        "BTC-USDT",
		Side:          "BUY",
		Type:          "LIMIT",
		Price:         95000.0,
		Quantity:      1.5,
		Status:        engine.StatusOMS_Created,
		TimeInForce:   engine.TIF_GTC,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ClientOrderID: "cli_buyer_alice_456",
	}

	err = router.ProcessIncomingOrder(context.Background(), buyOrder, "USDT", "BTC", "127.0.0.1", "TEST_CLIENT")
	if err != nil {
		t.Fatalf("Failed to process crossing buy order: %v", err)
	}

	// 5. Assert that order filled states map perfectly inside the OMS Machine
	updatedBuy, _ := router.GetOrder(buyOrder.ID)
	if updatedBuy.Status != engine.StatusOMS_Filled || updatedBuy.FilledQty != 1.5 {
		t.Errorf("Expected buy order filled, got status %s, quantity %f", updatedBuy.Status, updatedBuy.FilledQty)
	}

	updatedSell, _ := router.GetOrder(sellOrder.ID)
	if updatedSell.Status != engine.StatusOMS_PartiallyFilled || updatedSell.FilledQty != 1.5 {
		t.Errorf("Expected sell order partially filled, got status %s, quantity %f", updatedSell.Status, updatedSell.FilledQty)
	}

	// 6. Verify Settlement Clearance
	if settle.GetSettledCount() != 1 {
		t.Errorf("Expected exactly 1 completed settlement, got %d", settle.GetSettledCount())
	}

	// 7. Verify cryptographic address validation format works correctly with standard adapters
	ethAddr := "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359"
	if !common.ValidateCryptographicAddress(ethAddr, "ETH") {
		t.Error("Ethereum address verification failed")
	}

	btcAddr := "1A1zP1eP5QGefi2Dmjf959GUuxM7VtGi78"
	_ = btcAddr

	// Verify BTC validation bypass/rejection
	if common.ValidateCryptographicAddress("1A1z_InvalidAddressFormat", "BTC") {
		t.Error("Corrupt BTC legacy address accepted by validator")
	}

	// 8. Verify secure dynamic TOTP generator
	mfaSecret, mfaBackup, err := security.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("Failed to generate dynamic MFA profile: %v", err)
	}
	if len(mfaSecret) != 32 || len(mfaBackup) != 3 {
		t.Error("Invalid dynamic MFA key parameters")
	}
}
