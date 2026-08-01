package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestOMSValidators(t *testing.T) {
	val := NewOMSValidator()

	// Min Size violation
	orderSmall := &AdvancedOrder{
		Symbol:   "BTC-USDT",
		Price:    50000.0,
		Quantity: 0.00001, // min is 0.0001
	}
	err := val.ValidateTradingRules(orderSmall)
	if err == nil {
		t.Error("Expected validation error for too small order size")
	}

	// Min Notional violation
	orderNotional := &AdvancedOrder{
		Symbol:   "BTC-USDT",
		Price:    1000.0,
		Quantity: 0.001, // 1000 * 0.001 = 1 USDT (min is 5.0)
	}
	err = val.ValidateTradingRules(orderNotional)
	if err == nil {
		t.Error("Expected validation error for too small order notional value")
	}

	// Step size violation
	orderStep := &AdvancedOrder{
		Symbol:   "BTC-USDT",
		Price:    50000.0,
		Quantity: 0.10005, // step size is 0.0001
	}
	err = val.ValidateTradingRules(orderStep)
	if err == nil {
		t.Error("Expected validation error for non-conforming step size")
	}

	// Tick size violation
	orderTick := &AdvancedOrder{
		Symbol:   "BTC-USDT",
		Price:    50000.005, // tick size is 0.01
		Quantity: 0.1,
	}
	err = val.ValidateTradingRules(orderTick)
	if err == nil {
		t.Error("Expected validation error for non-conforming price tick size")
	}
}

func TestOMSStateMachineTransitions(t *testing.T) {
	sm := NewOMSStateMachine()
	order := &AdvancedOrder{
		ID:     "ord_t1",
		UserID: "usr_t1",
		Symbol: "BTC-USDT",
		Side:   "BUY",
		Type:   "LIMIT",
	}

	sm.SubmitOrder(order, "127.0.0.1", "WEB")
	if order.Status != StatusOMS_Created {
		t.Errorf("Expected CREATED state, got %s", order.Status)
	}

	// Transition to validation
	_, err := sm.TransitionOrder(order.ID, StatusOMS_PendingValidation, "VALIDATE", "127.0.0.1", "WEB", "validation started")
	if err != nil {
		t.Fatalf("Failed to transition: %v", err)
	}

	// Invalid transition: Created to Accepted directly (must pass validation)
	_, err = sm.TransitionOrder(order.ID, StatusOMS_Accepted, "ACCEPT", "127.0.0.1", "WEB", "bypass validation")
	if err == nil {
		t.Error("Expected transition failure bypassing validation step")
	}

	// Success flow
	_, _ = sm.TransitionOrder(order.ID, StatusOMS_Validated, "VALIDATE", "127.0.0.1", "WEB", "validated")
	_, _ = sm.TransitionOrder(order.ID, StatusOMS_Accepted, "ACCEPT", "127.0.0.1", "WEB", "accepted")
	_, _ = sm.TransitionOrder(order.ID, StatusOMS_Queued, "QUEUE", "127.0.0.1", "WEB", "queued")
	_, _ = sm.TransitionOrder(order.ID, StatusOMS_Filled, "FILL", "127.0.0.1", "WEB", "filled")

	// Finalized state should refuse further updates
	_, err = sm.TransitionOrder(order.ID, StatusOMS_Cancelled, "CANCEL", "127.0.0.1", "WEB", "try cancel filled")
	if err == nil {
		t.Error("Expected transition failure from finalised Filled state")
	}

	// Verify Audit trail
	audits := sm.GetAuditLogs("ord_t1")
	if len(audits) < 5 {
		t.Errorf("Expected detailed audit trail logs, got count: %d", len(audits))
	}
}

func TestOMSRouterWorkflows(t *testing.T) {
	sm := NewOMSStateMachine()
	val := NewOMSValidator()
	risk := NewRiskEngine(10.0, 100000.0)
	matcher := NewMatcher("BTC-USDT")
	fees := NewFeesEngine(0.0010, 0.0020)
	exec := NewExecutionEngine(fees, risk)
	settle := NewSettlementEngine(nil)

	router := NewOMSRouter(sm, val, risk, matcher, exec, settle)

	// User deposits
	risk.DepositAsset("buyer", "USDT", 30000.0)
	risk.DepositAsset("seller", "BTC", 1.0)

	// Sell Order placed first in book
	sell := &AdvancedOrder{
		ID:            "s_oms_1",
		ClientOrderID: "c_s_1",
		UserID:        "seller",
		Symbol:        "BTC-USDT",
		Side:          "SELL",
		Type:          "LIMIT",
		Price:         50000.0,
		Quantity:      0.5,
	}

	err := router.ProcessIncomingOrder(context.Background(), sell, "USDT", "BTC", "127.0.0.1", "WEB")
	if err != nil {
		t.Fatalf("Failed to process sell order: %v", err)
	}

	// Buy Order matches the sell order
	buy := &AdvancedOrder{
		ID:            "b_oms_1",
		ClientOrderID: "c_b_1",
		UserID:        "buyer",
		Symbol:        "BTC-USDT",
		Side:          "BUY",
		Type:          "LIMIT",
		Price:         50000.0,
		Quantity:      0.5,
	}

	err = router.ProcessIncomingOrder(context.Background(), buy, "USDT", "BTC", "127.0.0.1", "WEB")
	if err != nil {
		t.Fatalf("Failed to process buy order: %v", err)
	}

	// Assert orders fully filled
	sOrder, _ := sm.GetOrder("s_oms_1")
	bOrder, _ := sm.GetOrder("b_oms_1")

	if sOrder.Status != StatusOMS_Filled || bOrder.Status != StatusOMS_Filled {
		t.Errorf("Expected both orders FILLED, sell status: %s, buy status: %s", sOrder.Status, bOrder.Status)
	}

	// Test Replacement
	sell2 := &AdvancedOrder{
		ID:       "s_oms_2",
		UserID:   "seller",
		Symbol:   "BTC-USDT",
		Side:     "SELL",
		Type:     "LIMIT",
		Price:    51000.0,
		Quantity: 0.1,
	}
	router.ProcessIncomingOrder(context.Background(), sell2, "USDT", "BTC", "127.0.0.1", "WEB")

	sell3 := &AdvancedOrder{
		ID:       "s_oms_3",
		UserID:   "seller",
		Symbol:   "BTC-USDT",
		Side:     "SELL",
		Type:     "LIMIT",
		Price:    52000.0,
		Quantity: 0.1,
	}

	// Replace s_oms_2 with s_oms_3
	err = router.ReplaceOrder(context.Background(), "s_oms_2", sell3, "USDT", "BTC", "127.0.0.1", "WEB")
	if err != nil {
		t.Fatalf("Order replacement failed: %v", err)
	}

	replacedStatus, _ := sm.GetOrder("s_oms_2")
	if replacedStatus.Status != StatusOMS_Cancelled {
		t.Errorf("Expected cancelled status for replaced order, got %s", replacedStatus.Status)
	}

	// Test Mass Cancel
	cancelledCount := router.MassCancel("BTC-USDT", "seller", "127.0.0.1", "WEB")
	if cancelledCount != 1 { // s_oms_3 is cancelled, s_oms_1 and s_oms_2 are already filled/cancelled
		t.Errorf("Expected 1 order mass cancelled, got %d", cancelledCount)
	}
}

func TestOMSStressConcurrency(t *testing.T) {
	sm := NewOMSStateMachine()
	val := NewOMSValidator()
	risk := NewRiskEngine(10.0, 100000.0)
	matcher := NewMatcher("BTC-USDT")
	fees := NewFeesEngine(0.0010, 0.0020)
	exec := NewExecutionEngine(fees, risk)
	settle := NewSettlementEngine(nil)

	router := NewOMSRouter(sm, val, risk, matcher, exec, settle)

	risk.DepositAsset("concurrent_user", "USDT", 1000000.0)

	var wg sync.WaitGroup
	workers := 20
	iterations := 50

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				orderID := fmt.Sprintf("ord_con_%d_%d", workerID, j)
				order := &AdvancedOrder{
					ID:       orderID,
					UserID:   "concurrent_user",
					Symbol:   "BTC-USDT",
					Side:     "BUY",
					Type:     "LIMIT",
					Price:    500.0 + float64(workerID),
					Quantity: 0.01,
				}
				_ = router.ProcessIncomingOrder(context.Background(), order, "USDT", "BTC", "127.0.0.1", "CONCURRENT")
			}
		}(i)
	}

	wg.Wait()

	// Verify all orders exist in memory state cleanly
	for i := 0; i < workers; i++ {
		for j := 0; j < iterations; j++ {
			orderID := fmt.Sprintf("ord_con_%d_%d", i, j)
			_, err := sm.GetOrder(orderID)
			if err != nil {
				t.Fatalf("Concurrently submitted order %s was lost", orderID)
			}
		}
	}
}
