package engine

import (
	"context"
	"fmt"
	"sync"
	"velyxora/packages/database"
	"velyxora/packages/types"
)

// OMSRouter coordinates validations, risk locks, matching execution, and cancellations
type OMSRouter struct {
	mu           sync.RWMutex
	stateMachine *OMSStateMachine
	validator    *OMSValidator
	risk         *RiskEngine
	matcher      *Matcher
	execution    *ExecutionEngine
	settlement   *SettlementEngine
}

// NewOMSRouter initializes the routing supervisor
func NewOMSRouter(sm *OMSStateMachine, val *OMSValidator, risk *RiskEngine, matcher *Matcher, exec *ExecutionEngine, settle *SettlementEngine) *OMSRouter {
	return &OMSRouter{
		stateMachine: sm,
		validator:    val,
		risk:         risk,
		matcher:      matcher,
		execution:    exec,
		settlement:   settle,
	}
}

// ProcessIncomingOrder processes orders through validation, risk hold, and matching pipelines
func (or *OMSRouter) ProcessIncomingOrder(ctx context.Context, order *AdvancedOrder, quoteAsset, baseAsset string, ip, device string) error {
	// 1. Submit to state machine
	or.stateMachine.SubmitOrder(order, ip, device)

	// 2. Transition to Pending Validation
	_, err := or.stateMachine.TransitionOrder(order.ID, StatusOMS_PendingValidation, "VALIDATE", ip, device, "Validation checks started")
	if err != nil {
		return err
	}

	// 3. Trading rules validation
	err = or.validator.ValidateTradingRules(order)
	if err != nil {
		_, _ = or.stateMachine.TransitionOrder(order.ID, StatusOMS_Rejected, "REJECT", ip, device, fmt.Sprintf("Validation failed: %v", err))
		return err
	}

	// 4. Pre-trade Risk & Balance locking check
	makerRate, takerRate := or.execution.fees.GetFeeRates(order.UserID)
	feeRate := takerRate
	if order.PostOnly {
		feeRate = makerRate
	}

	legacyOrder := &types.Order{
		ID:          order.ID,
		UserID:      order.UserID,
		Symbol:      order.Symbol,
		Side:        types.OrderSide(order.Side),
		Type:        types.OrderType(order.Type),
		Price:       order.Price,
		Quantity:    order.Quantity,
		FilledQty:   order.FilledQty,
		TimeInForce: string(order.TimeInForce),
		PostOnly:    order.PostOnly,
	}

	err = or.risk.ValidateOrder(legacyOrder, quoteAsset, baseAsset, feeRate)
	if err != nil {
		_, _ = or.stateMachine.TransitionOrder(order.ID, StatusOMS_Rejected, "REJECT", ip, device, fmt.Sprintf("Risk validation failed: %v", err))
		return err
	}

	_, _ = or.stateMachine.TransitionOrder(order.ID, StatusOMS_Validated, "VALIDATE", ip, device, "Validated successfully")

	// 5. Transition to Accepted
	_, _ = or.stateMachine.TransitionOrder(order.ID, StatusOMS_Accepted, "ACCEPT", ip, device, "Accepted by Gateway")

	// 6. Route to Matching Engine Core
	_, _ = or.stateMachine.TransitionOrder(order.ID, StatusOMS_Queued, "QUEUE", ip, device, "Queued inside the limit book")

	matches := or.matcher.MatchOrder(legacyOrder)
	if len(matches) > 0 {
		// Process Executions
		executions := or.execution.ProcessTrades(matches, quoteAsset, baseAsset)

		for _, exec := range executions {
			// Persist balance adjustments via asynchronous queue clearing
			or.settlement.QueueSettlement(exec, baseAsset, quoteAsset)
		}
		// Clear pending jobs queue
		_, _ = or.settlement.ProcessQueue(ctx)

		// Update order fill state
		order.FilledQty = legacyOrder.FilledQty
		if legacyOrder.Status == types.StatusFilled {
			_, _ = or.stateMachine.TransitionOrder(order.ID, StatusOMS_Filled, "FILL", ip, device, "Fully filled")
		} else if legacyOrder.Status == types.StatusPartiallyFilled {
			_, _ = or.stateMachine.TransitionOrder(order.ID, StatusOMS_PartiallyFilled, "FILL", ip, device, "Partially filled")
		}

		// Also update any resting (maker) orders matched during this execution leg in the OMS
		for _, t := range matches {
			makerOrderID := t.SellOrderID
			if order.Side == "SELL" {
				makerOrderID = t.BuyOrderID
			}

			makerOrder, err := or.stateMachine.GetOrder(makerOrderID)
			if err == nil {
				makerOrder.FilledQty += t.Quantity
				if makerOrder.FilledQty >= makerOrder.Quantity {
					_, _ = or.stateMachine.TransitionOrder(makerOrderID, StatusOMS_Filled, "FILL", ip, device, "Fully filled as maker")
				} else {
					_, _ = or.stateMachine.TransitionOrder(makerOrderID, StatusOMS_PartiallyFilled, "FILL", ip, device, "Partially filled as maker")
				}
			}
		}
	}

	return nil
}

// CancelOrder routes cancel requests to Matcher book and updates state
func (or *OMSRouter) CancelOrder(orderID, ip, device string) error {
	order, err := or.stateMachine.GetOrder(orderID)
	if err != nil {
		return err
	}

	// Attempt cancellation on Matcher book levels
	cancelled := or.matcher.CancelOrder(orderID)
	if !cancelled {
		return fmt.Errorf("order %s was not found active in matcher price queues", orderID)
	}

	// Update OMS state
	_, err = or.stateMachine.TransitionOrder(orderID, StatusOMS_Cancelled, "CANCEL", ip, device, "Order cancelled on request")
	if err != nil {
		return err
	}

	// Release holds for the remaining unfilled portion only (prevents negative locks/double-spend risk)
	unfilledQty := order.Quantity - order.FilledQty
	if unfilledQty > 0 {
		if order.Side == "BUY" {
			makerRate, takerRate := or.execution.fees.GetFeeRates(order.UserID)
			feeRate := takerRate
			if order.PostOnly {
				feeRate = makerRate
			}
			or.risk.ReleaseHold(order.UserID, "USDT", unfilledQty*order.Price*(1.0+feeRate))
		} else {
			or.risk.ReleaseHold(order.UserID, "BTC", unfilledQty)
		}
	}

	return nil
}

// ReplaceOrder cancels an existing order and submits a new order atomically
func (or *OMSRouter) ReplaceOrder(ctx context.Context, cancelOrderID string, newOrder *AdvancedOrder, quoteAsset, baseAsset, ip, device string) error {
	// Cancel existing order
	err := or.CancelOrder(cancelOrderID, ip, device)
	if err != nil {
		return fmt.Errorf("failed to cancel previous order %s during replacement: %w", cancelOrderID, err)
	}

	// Process new order
	return or.ProcessIncomingOrder(ctx, newOrder, quoteAsset, baseAsset, ip, device)
}

// MassCancel cancels all active orders matching filters
func (or *OMSRouter) MassCancel(symbol, userID, ip, device string) int {
	var targetIDs []string

	or.stateMachine.mu.RLock()
	for id, order := range or.stateMachine.activeOrders {
		if order.Status == StatusOMS_Queued || order.Status == StatusOMS_PartiallyFilled {
			matchSymbol := (symbol == "" || order.Symbol == symbol)
			matchUser := (userID == "" || order.UserID == userID)

			if matchSymbol && matchUser {
				targetIDs = append(targetIDs, id)
			}
		}
	}
	or.stateMachine.mu.RUnlock()

	count := 0
	for _, id := range targetIDs {
		err := or.CancelOrder(id, ip, device)
		if err == nil {
			count++
		}
	}

	return count
}

// GetUserOrders queries user orders from the underlying state machine
func (or *OMSRouter) GetUserOrders(userID string) []*AdvancedOrder {
	return or.stateMachine.GetUserOrders(userID)
}

// GetOrder retrieves a single tracked advanced order
func (or *OMSRouter) GetOrder(orderID string) (*AdvancedOrder, error) {
	return or.stateMachine.GetOrder(orderID)
}

// SetDB dynamically configures database references for settlement and risk engines
func (or *OMSRouter) SetDB(db *database.DB) {
	or.mu.Lock()
	defer or.mu.Unlock()
	or.settlement.SetDB(db)
	or.risk.SetDB(db)
}

// SetHaltStatus triggers global trading halts in the risk engine
func (or *OMSRouter) SetHaltStatus(halted bool) {
	or.mu.Lock()
	defer or.mu.Unlock()
	or.risk.SetHaltStatus(halted)
}

// BlockAccount blocks a user from trading in the risk engine
func (or *OMSRouter) BlockAccount(userID string, blocked bool) {
	or.mu.Lock()
	defer or.mu.Unlock()
	or.risk.BlockAccount(userID, blocked)
}

// SuspendMarket suspends trading for a specific symbol in the risk engine
func (or *OMSRouter) SuspendMarket(symbol string, suspended bool) {
	or.mu.Lock()
	defer or.mu.Unlock()
	or.risk.SuspendMarket(symbol, suspended)
}
