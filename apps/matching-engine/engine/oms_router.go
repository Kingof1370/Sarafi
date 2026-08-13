package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"velyxora/packages/database"
	"velyxora/packages/types"
)

// IsFuturesOrder checks if a given side or symbol is for Perpetual Futures
func IsFuturesOrder(side string, symbol string) bool {
	sideUpper := strings.ToUpper(side)
	if strings.Contains(sideUpper, "OPEN") || strings.Contains(sideUpper, "CLOSE") {
		return true
	}
	if strings.Contains(strings.ToUpper(symbol), "PERP") || strings.Contains(strings.ToUpper(symbol), "FUTURES") {
		return true
	}
	return false
}

// OMSRouter coordinates validations, risk locks, matching execution, and cancellations
type OMSRouter struct {
	mu                 sync.RWMutex
	stateMachine       *OMSStateMachine
	validator          *OMSValidator
	risk               *RiskEngine
	matcher            *Matcher // Deprecated: use registry instead
	registry           *MarketRegistry
	execution          *ExecutionEngine
	settlement         *SettlementEngine
	liquidityBridge    *LiquidityBridge
	positionEngine     *PositionEngine
	OnTradeMatched     func(symbol string, price, quantity float64, timestamp time.Time)
	OnOrderBookChanged func(symbol string)
}

// NewOMSRouter initializes the routing supervisor
func NewOMSRouter(sm *OMSStateMachine, val *OMSValidator, risk *RiskEngine, matcher *Matcher, exec *ExecutionEngine, settle *SettlementEngine) *OMSRouter {
	registry := NewMarketRegistry()
	if matcher != nil {
		registry.RegisterMatcher(matcher.Symbol, matcher)
	}

	router := &OMSRouter{
		stateMachine: sm,
		validator:    val,
		risk:         risk,
		matcher:      matcher,
		registry:     registry,
		execution:    exec,
		settlement:   settle,
	}

	if val != nil {
		val.GetLastPrice = func(symbol string) float64 {
			if registry != nil {
				m := registry.GetMatcher(symbol)
				if m != nil {
					return m.GetLastPrice()
				}
			}
			return 0
		}
	}

	return router
}

// GetMatcherForSymbol retrieves the isolated Matcher instance for a given symbol
func (or *OMSRouter) GetMatcherForSymbol(symbol string) *Matcher {
	return or.registry.GetMatcher(symbol)
}

// GetMarketRegistry returns the internal multi-symbol registry
func (or *OMSRouter) GetMarketRegistry() *MarketRegistry {
	return or.registry
}

// SetPositionEngine registers the PositionEngine reference inside the OMSRouter
func (or *OMSRouter) SetPositionEngine(pe *PositionEngine) {
	or.mu.Lock()
	defer or.mu.Unlock()
	or.positionEngine = pe
}

// SetLiquidityBridge configures the external liquidity engine bridge
func (or *OMSRouter) SetLiquidityBridge(lb *LiquidityBridge) {
	or.mu.Lock()
	defer or.mu.Unlock()
	or.liquidityBridge = lb
}

// GetAggregatedL2Depth merges the local Matcher book depth with external real-time liquidity
func (or *OMSRouter) GetAggregatedL2Depth(maxLevels int) *types.OrderBookL2 {
	sym := "BTC-USDT"
	if or.matcher != nil {
		sym = or.matcher.Symbol
	}
	return or.GetAggregatedL2DepthForSymbol(sym, maxLevels)
}

// GetAggregatedL2DepthForSymbol merges the local Matcher book depth for a specific symbol with external real-time liquidity
func (or *OMSRouter) GetAggregatedL2DepthForSymbol(symbol string, maxLevels int) *types.OrderBookL2 {
	or.mu.RLock()
	lb := or.liquidityBridge
	or.mu.RUnlock()

	m := or.registry.GetMatcher(symbol)

	// 1. Retrieve local book depth
	localL2 := m.GetL2Depth(maxLevels)
	for i := range localL2.Bids {
		localL2.Bids[i].IsAggregated = false
		localL2.Bids[i].Source = "Local"
	}
	for i := range localL2.Asks {
		localL2.Asks[i].IsAggregated = false
		localL2.Asks[i].Source = "Local"
	}

	if lb == nil {
		return localL2
	}

	// 2. Retrieve external book depth
	extL2, err := lb.GetOrderBook(symbol)
	if err != nil {
		// Degradation: Fallback to only local depth
		return localL2
	}

	// 3. Merge Bids (sorted descending)
	mergedBids := make([]types.OrderBookLevel, 0, len(localL2.Bids)+len(extL2.Bids))
	i, j := 0, 0
	for i < len(localL2.Bids) && j < len(extL2.Bids) {
		localBid := localL2.Bids[i]
		extBid := extL2.Bids[j]

		if localBid.Price > extBid.Price {
			mergedBids = append(mergedBids, localBid)
			i++
		} else if localBid.Price < extBid.Price {
			mergedBids = append(mergedBids, extBid)
			j++
		} else {
			mergedBids = append(mergedBids, localBid)
			mergedBids = append(mergedBids, extBid)
			i++
			j++
		}
	}
	for i < len(localL2.Bids) {
		mergedBids = append(mergedBids, localL2.Bids[i])
		i++
	}
	for j < len(extL2.Bids) {
		mergedBids = append(mergedBids, extL2.Bids[j])
		j++
	}

	// 4. Merge Asks (sorted ascending)
	mergedAsks := make([]types.OrderBookLevel, 0, len(localL2.Asks)+len(extL2.Asks))
	i, j = 0, 0
	for i < len(localL2.Asks) && j < len(extL2.Asks) {
		localAsk := localL2.Asks[i]
		extAsk := extL2.Asks[j]

		if localAsk.Price < extAsk.Price {
			mergedAsks = append(mergedAsks, localAsk)
			i++
		} else if localAsk.Price > extAsk.Price {
			mergedAsks = append(mergedAsks, extAsk)
			j++
		} else {
			mergedAsks = append(mergedAsks, localAsk)
			mergedAsks = append(mergedAsks, extAsk)
			i++
			j++
		}
	}
	for i < len(localL2.Asks) {
		mergedAsks = append(mergedAsks, localL2.Asks[i])
		i++
	}
	for j < len(extL2.Asks) {
		mergedAsks = append(mergedAsks, extL2.Asks[j])
		j++
	}

	// Limit to maxLevels
	if len(mergedBids) > maxLevels {
		mergedBids = mergedBids[:maxLevels]
	}
	if len(mergedAsks) > maxLevels {
		mergedAsks = mergedAsks[:maxLevels]
	}

	return &types.OrderBookL2{
		Symbol:    symbol,
		Bids:      mergedBids,
		Asks:      mergedAsks,
		Sequence:  localL2.Sequence,
		Timestamp: time.Now(),
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
	isStop := (order.Type == "STOP" || order.Type == "STOP_LIMIT" || order.Type == "TAKE_PROFIT" || order.Type == "TAKE_PROFIT_LIMIT")

	m := or.registry.GetMatcher(order.Symbol)

	if isStop {
		triggerPrice := order.StopPrice
		if triggerPrice == 0 {
			triggerPrice = order.Price
		}
		legacyOrder.Price = triggerPrice

		m.AddStopOrder(legacyOrder)
		_, _ = or.stateMachine.TransitionOrder(order.ID, StatusOMS_Queued, "QUEUE", ip, device, "Registered stop order trigger")
		if or.OnOrderBookChanged != nil {
			or.OnOrderBookChanged(order.Symbol)
		}
		return nil
	}

	_, _ = or.stateMachine.TransitionOrder(order.ID, StatusOMS_Queued, "QUEUE", ip, device, "Queued inside the limit book")

	matches := m.MatchOrder(legacyOrder)

	var externalMatches []*types.Trade
	or.mu.RLock()
	lb := or.liquidityBridge
	or.mu.RUnlock()

	// External Hedging Bridge check
	if lb != nil && legacyOrder.FilledQty < legacyOrder.Quantity && legacyOrder.Status != types.StatusCancelled && legacyOrder.Status != types.StatusRejected {
		extBook, err := lb.GetOrderBook(order.Symbol)
		if err == nil {
			var extLevels []types.OrderBookLevel
			if legacyOrder.Side == types.SideBuy {
				extLevels = extBook.Asks
			} else {
				extLevels = extBook.Bids
			}

			if len(extLevels) > 0 {
				// Remove order temporarily from local matcher book to prevent double match during calculation
				m.CancelOrder(legacyOrder.ID)

				for i := 0; i < len(extLevels) && legacyOrder.FilledQty < legacyOrder.Quantity; i++ {
					level := extLevels[i]

					// Check Limit price crosses
					if legacyOrder.Type == types.TypeLimit {
						if legacyOrder.Side == types.SideBuy && legacyOrder.Price < level.Price {
							break
						}
						if legacyOrder.Side == types.SideSell && legacyOrder.Price > level.Price {
							break
						}
					}

					matchQty := legacyOrder.Quantity - legacyOrder.FilledQty
					if matchQty > level.Quantity {
						matchQty = level.Quantity
					}

					if matchQty <= 0 {
						continue
					}

					tradeID := fmt.Sprintf("trd_ext_%d", time.Now().UnixNano())
					trade := &types.Trade{
						ID:        tradeID,
						Symbol:    order.Symbol,
						Price:     level.Price,
						Quantity:  matchQty,
						Timestamp: time.Now(),
					}

					if legacyOrder.Side == types.SideBuy {
						trade.BuyerID = legacyOrder.UserID
						trade.SellerID = "EXT_LIQUIDITY_BINANCE"
						trade.BuyOrderID = legacyOrder.ID
						trade.SellOrderID = "ext_sell_order_binance"
					} else {
						trade.BuyerID = "EXT_LIQUIDITY_BINANCE"
						trade.SellerID = legacyOrder.UserID
						trade.BuyOrderID = "ext_buy_order_binance"
						trade.SellOrderID = legacyOrder.ID
					}

					externalMatches = append(externalMatches, trade)
					legacyOrder.FilledQty += matchQty

					if legacyOrder.FilledQty >= legacyOrder.Quantity {
						legacyOrder.Status = types.StatusFilled
					} else {
						legacyOrder.Status = types.StatusPartiallyFilled
					}

					// Trigger asynchronous external hedge call to Binance
					go func(qty float64, prc float64, side types.OrderSide) {
						hedgeOrder := &types.Order{
							ID:       "hdg_" + fmt.Sprintf("%d", time.Now().UnixNano()),
							Symbol:   order.Symbol,
							Side:     side,
							Type:     types.TypeMarket,
							Price:    prc,
							Quantity: qty,
							UserID:   "PLATFORM_HEDGE_ACCOUNT",
						}
						_, hErr := lb.ExecuteOrder(hedgeOrder)
						if hErr != nil {
							lb.log.Error("Asynchronous external hedge execution failed", "err", hErr)
						} else {
							lb.log.Info("Asynchronous external hedge executed successfully", "qty", qty, "price", prc)
						}
					}(matchQty, level.Price, legacyOrder.Side)
				}

				// If still not fully filled and is a Limit order, add back to local book
				if legacyOrder.FilledQty < legacyOrder.Quantity && legacyOrder.Type == types.TypeLimit {
					m.AddOrderToBookDirect(legacyOrder)
				}
			}
		}
	}

	allMatches := append(matches, externalMatches...)

	if len(allMatches) > 0 {
		// Process Executions
		executions := or.execution.ProcessTrades(allMatches, quoteAsset, baseAsset)

		for _, exec := range executions {
			// Persist balance adjustments via asynchronous queue clearing
			or.settlement.QueueSettlement(exec, baseAsset, quoteAsset)
		}

		// Trigger Trade Callback, Position Adjustments and Liquidation Surplus checks
		for _, t := range allMatches {
			if or.positionEngine != nil {
				isFuturesSymbol := strings.Contains(strings.ToUpper(t.Symbol), "PERP") || strings.Contains(strings.ToUpper(t.Symbol), "FUTURES")

				// Update Buyer position
				if isFuturesSymbol {
					lev := or.risk.GetUserLeverage(t.BuyerID, t.Symbol)
					margin := (t.Quantity * t.Price) / lev
					or.positionEngine.RecordFuturesExecution(t.BuyerID, t.Symbol, t.Quantity, t.Price, lev, margin, 0.05)
				} else {
					or.positionEngine.RecordExecution(t.BuyerID, t.Symbol, t.Quantity, t.Price)
				}

				// Update Seller position
				if isFuturesSymbol {
					lev := or.risk.GetUserLeverage(t.SellerID, t.Symbol)
					margin := (t.Quantity * t.Price) / lev
					or.positionEngine.RecordFuturesExecution(t.SellerID, t.Symbol, -t.Quantity, t.Price, lev, margin, 0.05)
				} else {
					or.positionEngine.RecordExecution(t.SellerID, t.Symbol, -t.Quantity, t.Price)
				}
			}

			// Check for liquidation order filled surplus/deficit
			if t.BuyerID == "LIQUIDATION_ENGINE" || t.SellerID == "LIQUIDATION_ENGINE" {
				liqOrderID := t.BuyOrderID
				if t.SellerID == "LIQUIDATION_ENGINE" {
					liqOrderID = t.SellOrderID
				}

				liqOrder, err := or.stateMachine.GetOrder(liqOrderID)
				if err == nil {
					bankruptcyPrice := liqOrder.Price
					filledPrice := t.Price
					qty := t.Quantity

					if t.SellerID == "LIQUIDATION_ENGINE" {
						// Long liquidation order: SELL order at Bankruptcy Price
						if filledPrice > bankruptcyPrice {
							surplus := (filledPrice - bankruptcyPrice) * qty
							_ = or.risk.AddInsuranceFundReserves(ctx, quoteAsset, surplus)
						} else if filledPrice < bankruptcyPrice {
							loss := (bankruptcyPrice - filledPrice) * qty
							_ = or.risk.AddInsuranceFundReserves(ctx, quoteAsset, -loss)
							if or.risk.GetInsuranceFundBalance(quoteAsset) < 0 {
								or.risk.TriggerADL(t.Symbol, loss)
							}
						}
					} else {
						// Short liquidation order: BUY order at Bankruptcy Price
						if filledPrice < bankruptcyPrice {
							surplus := (bankruptcyPrice - filledPrice) * qty
							_ = or.risk.AddInsuranceFundReserves(ctx, quoteAsset, surplus)
						} else if filledPrice > bankruptcyPrice {
							loss := (filledPrice - bankruptcyPrice) * qty
							_ = or.risk.AddInsuranceFundReserves(ctx, quoteAsset, -loss)
							if or.risk.GetInsuranceFundBalance(quoteAsset) < 0 {
								or.risk.TriggerADL(t.Symbol, loss)
							}
						}
					}
				}
			}

			if or.OnTradeMatched != nil {
				or.OnTradeMatched(t.Symbol, t.Price, t.Quantity, t.Timestamp)
			}
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

		// Also update any resting (maker) orders matched during this execution leg in the OMS (from local matches only)
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

	if or.OnOrderBookChanged != nil {
		or.OnOrderBookChanged(order.Symbol)
	}

	return nil
}

// CancelOrder routes cancel requests to Matcher book and updates state
func (or *OMSRouter) CancelOrder(orderID, ip, device string) error {
	order, err := or.stateMachine.GetOrder(orderID)
	if err != nil {
		return err
	}

	m := or.registry.GetMatcher(order.Symbol)

	// Attempt cancellation on Matcher book levels
	cancelled := m.CancelOrder(orderID)
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

	if or.OnOrderBookChanged != nil {
		or.OnOrderBookChanged(order.Symbol)
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

// GetMatcher returns the active matcher
func (or *OMSRouter) GetMatcher() *Matcher {
	return or.matcher
}

// GetRecentTrades returns the trade executions
func (or *OMSRouter) GetRecentTrades() []*Execution {
	return or.execution.GetExecutions()
}

// SetDB dynamically configures database references for settlement and risk engines
func (or *OMSRouter) SetDB(db *database.DB) {
	or.mu.Lock()
	defer or.mu.Unlock()
	or.settlement.SetDB(db)
	or.risk.SetDB(db)
	or.stateMachine.SetDB(db)
}

// SetHaltStatus updates the global halt status in the RiskEngine
func (or *OMSRouter) SetHaltStatus(halted bool) {
	or.mu.Lock()
	defer or.mu.Unlock()
	or.risk.SetHaltStatus(halted)
}

// BlockAccount blocks/unblocks a user from trading in the RiskEngine
func (or *OMSRouter) BlockAccount(userID string, blocked bool) {
	or.mu.Lock()
	defer or.mu.Unlock()
	or.risk.BlockAccount(userID, blocked)
}

// SuspendMarket suspends/unsuspends a trading pair in the RiskEngine
func (or *OMSRouter) SuspendMarket(symbol string, suspended bool) {
	or.mu.Lock()
	defer or.mu.Unlock()
	or.risk.SuspendMarket(symbol, suspended)
}
