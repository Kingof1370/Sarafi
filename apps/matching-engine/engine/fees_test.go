package engine

import (
	"context"
	"testing"
	"velyxora/packages/types"
)

func TestNativeTokenDiscountAndFallback(t *testing.T) {
	fe := NewFeesEngine(0.0010, 0.0020) // maker=0.1%, taker=0.2%
	re := NewRiskEngine(10.0, 100000.0)
	fe.SetRiskEngine(re)

	userID := "usr_discount_test"
	symbol := "BTC-USDT"

	// 1. Normal fee deduction when VLX balance is zero/not configured
	// Preference is off or VLX balance is 0.
	// Raw Taker Fee on 50,000 USDT * 1 BTC = 50,000 USDT.
	// 50,000 USDT * 0.2% = 100 USDT.
	feeAsset, feeAmt, discounted := fe.CalculateFeeForUser(userID, symbol, 100.0, false)
	if feeAsset != "USDT" {
		t.Errorf("Expected fee asset to be USDT, got %s", feeAsset)
	}
	if feeAmt != 100.0 {
		t.Errorf("Expected fee to be 100.0, got %f", feeAmt)
	}
	if discounted {
		t.Errorf("Expected discounted to be false")
	}

	// Enable preference but VLX balance is still 0
	fe.SetUserPreference(userID, &types.UserPreferences{
		UserID:       userID,
		PayFeesInVLX: true,
	})
	feeAsset, feeAmt, discounted = fe.CalculateFeeForUser(userID, symbol, 100.0, false)
	if feeAsset != "USDT" {
		t.Errorf("Expected fallback fee asset to be USDT, got %s", feeAsset)
	}
	if feeAmt != 100.0 {
		t.Errorf("Expected fallback fee to be 100.0, got %f", feeAmt)
	}
	if discounted {
		t.Errorf("Expected discounted to be false when VLX is 0")
	}

	// 2. Insufficient VLX balance fallback to native asset fee
	// VLX is priced at 2.5 USDT.
	// 100 USDT fee is converted to: 100 / 2.5 = 40 VLX.
	// Applying 25% discount gives: 40 * 0.75 = 30 VLX.
	// Deposit insufficient VLX (e.g. 29.9 VLX)
	re.DepositAsset(userID, "VLX", 29.9)
	feeAsset, feeAmt, discounted = fe.CalculateFeeForUser(userID, symbol, 100.0, false)
	if feeAsset != "USDT" {
		t.Errorf("Expected fallback to USDT due to insufficient VLX balance, got %s", feeAsset)
	}
	if feeAmt != 100.0 {
		t.Errorf("Expected fallback fee to be 100.0, got %f", feeAmt)
	}
	if discounted {
		t.Errorf("Expected discounted to be false on insufficient balance")
	}

	// 3. Discounted fee deduction in VLX when balance is sufficient
	// Deposit sufficient VLX (adding 0.1 so total VLX is 30.0)
	re.DepositAsset(userID, "VLX", 0.1)
	feeAsset, feeAmt, discounted = fe.CalculateFeeForUser(userID, symbol, 100.0, false)
	if feeAsset != "VLX" {
		t.Errorf("Expected fee asset to be VLX, got %s", feeAsset)
	}
	if feeAmt != 30.0 {
		t.Errorf("Expected discounted fee in VLX to be 30.0, got %f", feeAmt)
	}
	if !discounted {
		t.Errorf("Expected discounted to be true")
	}
}

func TestExecutionAndSettlementWithVLXDiscount(t *testing.T) {
	fe := NewFeesEngine(0.0010, 0.0020) // maker=0.1%, taker=0.2%
	re := NewRiskEngine(10.0, 100000.0)
	fe.SetRiskEngine(re)
	ee := NewExecutionEngine(fe, re)
	se := NewSettlementEngine(nil)

	buyerID := "buyer_vlx"
	sellerID := "seller_vlx"

	// Setup balances
	re.DepositAsset(buyerID, "USDT", 60000.0)
	re.DepositAsset(buyerID, "VLX", 100.0) // Has sufficient VLX
	re.DepositAsset(sellerID, "BTC", 1.0)
	re.DepositAsset(sellerID, "VLX", 5.0)  // Insufficient for maker fee in VLX (maker fee 50 USDT = 20 VLX, with 25% discount = 15 VLX)

	// Set preferences
	fe.SetUserPreference(buyerID, &types.UserPreferences{UserID: buyerID, PayFeesInVLX: true})
	fe.SetUserPreference(sellerID, &types.UserPreferences{UserID: sellerID, PayFeesInVLX: true})

	// Price conversions
	fe.SetAssetPrice("VLX", 2.5)
	fe.SetAssetPrice("USDT", 1.0)

	// Validate buy order with sufficient funds.
	// Buyer raw fee = 50,000 * 0.2% = 100 USDT.
	// So reservation should lock 50,000 + 100 = 50,100 USDT.
	buyOrder := &types.Order{
		ID:        "ord_b_vlx",
		UserID:    buyerID,
		Symbol:    "BTC-USDT",
		Side:      types.SideBuy,
		Type:      types.TypeLimit,
		Price:     50000.0,
		Quantity:  1.0,
	}
	err := re.ValidateOrder(buyOrder, "USDT", "BTC", 0.002)
	if err != nil {
		t.Fatalf("Validation should pass, got: %v", err)
	}

	// Create match trade
	trade := &types.Trade{
		ID:        "t_vlx_exec_1",
		Symbol:    "BTC-USDT",
		BuyerID:   buyerID,
		SellerID:  sellerID,
		Price:     50000.0,
		Quantity:  1.0,
	}

	executions := ee.ProcessTrades([]*types.Trade{trade}, "USDT", "BTC")
	if len(executions) != 1 {
		t.Fatalf("Expected 1 execution record")
	}

	exec := executions[0]
	// Buyer fee should be 30.0 VLX (discounted)
	if exec.BuyerFeeAsset != "VLX" || exec.BuyerFee != 30.0 || !exec.BuyerFeeDiscounted {
		t.Errorf("Buyer fee mismatch: asset %s, fee %f, discounted %t", exec.BuyerFeeAsset, exec.BuyerFee, exec.BuyerFeeDiscounted)
	}
	// Seller fee should fallback to USDT (50.0 USDT) since seller has only 5.0 VLX (needs 15.0 VLX)
	if exec.SellerFeeAsset != "USDT" || exec.SellerFee != 50.0 || exec.SellerFeeDiscounted {
		t.Errorf("Seller fee mismatch: asset %s, fee %f, discounted %t", exec.SellerFeeAsset, exec.SellerFee, exec.SellerFeeDiscounted)
	}

	// Check available balances in RiskEngine:
	// Buyer:
	// - Available USDT: 60,000 - 50,000 (reservation released fully, and 50,000 deducted for execution, fee was paid in VLX) = 10,000 USDT.
	// - Available VLX: 100 - 30 (fee) = 70 VLX.
	// - Available BTC: +1.0 = 1.0 BTC.
	if re.GetAvailableBalance(buyerID, "USDT") != 10000.0 {
		t.Errorf("Expected 10,000 USDT for buyer, got %f", re.GetAvailableBalance(buyerID, "USDT"))
	}
	if re.GetAvailableBalance(buyerID, "VLX") != 70.0 {
		t.Errorf("Expected 70.0 VLX for buyer, got %f", re.GetAvailableBalance(buyerID, "VLX"))
	}
	if re.GetAvailableBalance(buyerID, "BTC") != 1.0 {
		t.Errorf("Expected 1.0 BTC for buyer, got %f", re.GetAvailableBalance(buyerID, "BTC"))
	}

	// Seller:
	// - Available USDT: + (50,000 - 50 fee) = 49,950 USDT.
	// - Available VLX: remains 5.0 VLX.
	// - Available BTC: 0.0 (originally 1.0, locked during order placement, now settled/transferred)
	if re.GetAvailableBalance(sellerID, "USDT") != 49950.0 {
		t.Errorf("Expected 49,950 USDT for seller, got %f", re.GetAvailableBalance(sellerID, "USDT"))
	}
	if re.GetAvailableBalance(sellerID, "VLX") != 5.0 {
		t.Errorf("Expected 5.0 VLX for seller, got %f", re.GetAvailableBalance(sellerID, "VLX"))
	}

	// Settle the execution and verify multi-asset double entry validation
	job := se.QueueSettlement(exec, "BTC", "USDT")
	success, fail := se.ProcessQueue(context.Background())
	if success != 1 || fail != 0 {
		t.Fatalf("Settlement failed: success=%d, fail=%d, job_error=%s", success, fail, job.ErrorMsg)
	}
}
