package engine

import (
	"strconv"
	"sync"
	"time"
	"velyxora/packages/types"
)

// MatchLimit represents a single price level in the priority tree
type MatchLimit struct {
	Price  float64
	Orders []*types.Order
}

// Matcher implements the high-speed price-time matching logic with advanced execution modifiers
type Matcher struct {
	mu          sync.RWMutex
	Symbol      string
	Bids        []*MatchLimit // buy orders sorted high to low
	Asks        []*MatchLimit // sell orders sorted low to high
	StopOrders  []*types.Order // untriggered stop orders
	LastPrice   float64
	SequenceNum int64
}

// NewMatcher creates a book matching level
func NewMatcher(symbol string) *Matcher {
	return &Matcher{
		Symbol:     symbol,
		Bids:       make([]*MatchLimit, 0),
		Asks:       make([]*MatchLimit, 0),
		StopOrders: make([]*types.Order, 0),
	}
}

// MatchOrder matches incoming buy/sell orders and outputs executions (Price-Time Priority)
func (m *Matcher) MatchOrder(order *types.Order) []*types.Trade {
	m.mu.Lock()
	defer m.mu.Unlock()

	var trades []*types.Trade

	// Check for FOK (Fill-Or-Kill) condition before matching
	if order.Type == types.TypeLimit && order.Price > 0 {
		// FOK check: we must check if there is enough depth immediately available
		if isFOK(order, m.Bids, m.Asks) {
			// Fail/Kill the order immediately
			order.Status = types.StatusRejected
			return trades
		}
	}

	// Post Only Check
	if isPostOnlyCrossing(order, m.Bids, m.Asks) {
		order.Status = types.StatusRejected
		return trades
	}

	if order.Side == types.SideBuy {
		// Match against lowest asks first
		for i := 0; i < len(m.Asks) && order.Quantity > order.FilledQty; {
			limit := m.Asks[i]
			if order.Type == types.TypeLimit && limit.Price > order.Price {
				break // Buy price limit reached
			}

			for len(limit.Orders) > 0 && order.Quantity > order.FilledQty {
				sellOrder := limit.Orders[0]
				matchQty := min(order.Quantity-order.FilledQty, sellOrder.Quantity-sellOrder.FilledQty)

				order.FilledQty += matchQty
				sellOrder.FilledQty += matchQty

				if order.FilledQty == order.Quantity {
					order.Status = types.StatusFilled
				} else {
					order.Status = types.StatusPartiallyFilled
				}

				if sellOrder.FilledQty == sellOrder.Quantity {
					sellOrder.Status = types.StatusFilled
					limit.Orders = limit.Orders[1:]
				} else {
					sellOrder.Status = types.StatusPartiallyFilled
				}

				m.SequenceNum++
				nowNano := time.Now().UnixNano()
				tradeIDBytes := make([]byte, 0, 32)
				tradeIDBytes = append(tradeIDBytes, "trd_"...)
				tradeIDBytes = strconv.AppendInt(tradeIDBytes, nowNano, 10)
				tradeIDBytes = append(tradeIDBytes, '_')
				tradeIDBytes = strconv.AppendInt(tradeIDBytes, m.SequenceNum, 10)

				trade := &types.Trade{
					ID:          string(tradeIDBytes),
					Symbol:      m.Symbol,
					BuyerID:     order.UserID,
					SellerID:    sellOrder.UserID,
					BuyOrderID:  order.ID,
					SellOrderID: sellOrder.ID,
					Price:       limit.Price,
					Quantity:    matchQty,
					Timestamp:   time.Now(),
				}
				trades = append(trades, trade)
				m.LastPrice = limit.Price
				m.triggerStopOrders(limit.Price)
			}

			if len(limit.Orders) == 0 {
				m.Asks = append(m.Asks[:i], m.Asks[i+1:]...)
			} else {
				i++
			}
		}

		// If IOC (Immediate-Or-Cancel) and not fully matched, cancel the remaining quantity
		if order.FilledQty < order.Quantity && isIOC(order) {
			order.Status = types.StatusCancelled
			return trades
		}

		// If not fully filled, place on Bid book (except market orders)
		if order.FilledQty < order.Quantity && order.Type == types.TypeLimit {
			m.addOrderToBook(order, &m.Bids, true)
		}

	} else {
		// Match against highest bids first
		for i := 0; i < len(m.Bids) && order.Quantity > order.FilledQty; {
			limit := m.Bids[i]
			if order.Type == types.TypeLimit && limit.Price < order.Price {
				break // Sell price limit reached
			}

			for len(limit.Orders) > 0 && order.Quantity > order.FilledQty {
				buyOrder := limit.Orders[0]
				matchQty := min(order.Quantity-order.FilledQty, buyOrder.Quantity-buyOrder.FilledQty)

				order.FilledQty += matchQty
				buyOrder.FilledQty += matchQty

				if order.FilledQty == order.Quantity {
					order.Status = types.StatusFilled
				} else {
					order.Status = types.StatusPartiallyFilled
				}

				if buyOrder.FilledQty == buyOrder.Quantity {
					buyOrder.Status = types.StatusFilled
					limit.Orders = limit.Orders[1:]
				} else {
					buyOrder.Status = types.StatusPartiallyFilled
				}

				m.SequenceNum++
				nowNano := time.Now().UnixNano()
				tradeIDBytes := make([]byte, 0, 32)
				tradeIDBytes = append(tradeIDBytes, "trd_"...)
				tradeIDBytes = strconv.AppendInt(tradeIDBytes, nowNano, 10)
				tradeIDBytes = append(tradeIDBytes, '_')
				tradeIDBytes = strconv.AppendInt(tradeIDBytes, m.SequenceNum, 10)

				trade := &types.Trade{
					ID:          string(tradeIDBytes),
					Symbol:      m.Symbol,
					BuyerID:     buyOrder.UserID,
					SellerID:    order.UserID,
					BuyOrderID:  buyOrder.ID,
					SellOrderID: order.ID,
					Price:       limit.Price,
					Quantity:    matchQty,
					Timestamp:   time.Now(),
				}
				trades = append(trades, trade)
				m.LastPrice = limit.Price
				m.triggerStopOrders(limit.Price)
			}

			if len(limit.Orders) == 0 {
				m.Bids = append(m.Bids[:i], m.Bids[i+1:]...)
			} else {
				i++
			}
		}

		// If IOC (Immediate-Or-Cancel) and not fully matched, cancel the remaining quantity
		if order.FilledQty < order.Quantity && isIOC(order) {
			order.Status = types.StatusCancelled
			return trades
		}

		// If not fully filled, place on Ask book (except market orders)
		if order.FilledQty < order.Quantity && order.Type == types.TypeLimit {
			m.addOrderToBook(order, &m.Asks, false)
		}
	}

	return trades
}

// AddStopOrder registers an untriggered stop order
func (m *Matcher) AddStopOrder(order *types.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StopOrders = append(m.StopOrders, order)
}

// CancelOrder removes an active order from the book levels
func (m *Matcher) CancelOrder(orderID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Helper to search and remove
	removeFromLimits := func(limits *[]*MatchLimit) bool {
		for i, limit := range *limits {
			for j, ord := range limit.Orders {
				if ord.ID == orderID {
					limit.Orders = append(limit.Orders[:j], limit.Orders[j+1:]...)
					if len(limit.Orders) == 0 {
						*limits = append((*limits)[:i], (*limits)[i+1:]...)
					}
					return true
				}
			}
		}
		return false
	}

	// Remove from untriggered stop orders
	for i, ord := range m.StopOrders {
		if ord.ID == orderID {
			m.StopOrders = append(m.StopOrders[:i], m.StopOrders[i+1:]...)
			m.SequenceNum++
			return true
		}
	}

	if removeFromLimits(&m.Bids) || removeFromLimits(&m.Asks) {
		m.SequenceNum++
		return true
	}

	return false
}

// GetL2Depth exports the current bid/ask price depth snapshot
func (m *Matcher) GetL2Depth(maxLevels int) *types.OrderBookL2 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	l2 := &types.OrderBookL2{
		Symbol:    m.Symbol,
		Bids:      make([]types.OrderBookLevel, 0, maxLevels),
		Asks:      make([]types.OrderBookLevel, 0, maxLevels),
		Sequence:  m.SequenceNum,
		Timestamp: time.Now(),
	}

	for i := 0; i < len(m.Bids) && i < maxLevels; i++ {
		qty := 0.0
		for _, ord := range m.Bids[i].Orders {
			qty += (ord.Quantity - ord.FilledQty)
		}
		l2.Bids = append(l2.Bids, types.OrderBookLevel{Price: m.Bids[i].Price, Quantity: qty})
	}

	for i := 0; i < len(m.Asks) && i < maxLevels; i++ {
		qty := 0.0
		for _, ord := range m.Asks[i].Orders {
			qty += (ord.Quantity - ord.FilledQty)
		}
		l2.Asks = append(l2.Asks, types.OrderBookLevel{Price: m.Asks[i].Price, Quantity: qty})
	}

	return l2
}

func (m *Matcher) addOrderToBook(order *types.Order, limits *[]*MatchLimit, desc bool) {
	m.SequenceNum++
	price := order.Price
	for i, limit := range *limits {
		if limit.Price == price {
			limit.Orders = append(limit.Orders, order)
			return
		}

		if (desc && price > limit.Price) || (!desc && price < limit.Price) {
			newLimit := &MatchLimit{Price: price, Orders: []*types.Order{order}}
			*limits = append((*limits)[:i], append([]*MatchLimit{newLimit}, (*limits)[i:]...)...)
			return
		}
	}

	*limits = append(*limits, &MatchLimit{Price: price, Orders: []*types.Order{order}})
}

// triggerStopOrders monitors the price and enters stop orders into limit book when crossed
func (m *Matcher) triggerStopOrders(currentPrice float64) {
	var triggered []*types.Order
	var remaining []*types.Order

	for _, ord := range m.StopOrders {
		isTriggered := false
		isTakeProfit := (ord.Type == "TAKE_PROFIT" || ord.Type == "TAKE_PROFIT_LIMIT")

		if isTakeProfit {
			if ord.Side == types.SideBuy && currentPrice <= ord.Price {
				isTriggered = true
			} else if ord.Side == types.SideSell && currentPrice >= ord.Price {
				isTriggered = true
			}
		} else { // STOP / STOP_LIMIT / others
			if ord.Side == types.SideBuy && currentPrice >= ord.Price {
				isTriggered = true
			} else if ord.Side == types.SideSell && currentPrice <= ord.Price {
				isTriggered = true
			}
		}

		if isTriggered {
			triggered = append(triggered, ord)
		} else {
			remaining = append(remaining, ord)
		}
	}

	m.StopOrders = remaining

	// Route triggered stop orders back into match loops
	for _, ord := range triggered {
		ord.Type = types.TypeLimit // convert stop trigger to limit order
		go m.MatchOrder(ord)
	}
}

func isIOC(ord *types.Order) bool {
	return ord.TimeInForce == "IOC"
}

func isFOK(ord *types.Order, bids, asks []*MatchLimit) bool {
	if ord.TimeInForce != "FOK" {
		return false
	}

	remainingToFill := ord.Quantity - ord.FilledQty

	if ord.Side == types.SideBuy {
		cumulativeQty := 0.0
		for _, limit := range asks {
			if ord.Type == types.TypeLimit && limit.Price > ord.Price {
				break // Buy price limit reached
			}
			for _, bookOrd := range limit.Orders {
				cumulativeQty += (bookOrd.Quantity - bookOrd.FilledQty)
				if cumulativeQty >= remainingToFill {
					return false // FOK satisfies, enough liquidity available
				}
			}
		}
	} else {
		cumulativeQty := 0.0
		for _, limit := range bids {
			if ord.Type == types.TypeLimit && limit.Price < ord.Price {
				break // Sell price limit reached
			}
			for _, bookOrd := range limit.Orders {
				cumulativeQty += (bookOrd.Quantity - bookOrd.FilledQty)
				if cumulativeQty >= remainingToFill {
					return false // FOK satisfies
				}
			}
		}
	}

	return true // FOK fails, insufficient immediate liquidity
}

func isPostOnlyCrossing(ord *types.Order, bids, asks []*MatchLimit) bool {
	if !ord.PostOnly {
		return false
	}

	if ord.Side == types.SideBuy && len(asks) > 0 && ord.Price >= asks[0].Price {
		return true // would cross and execute immediately
	}
	if ord.Side == types.SideSell && len(bids) > 0 && ord.Price <= bids[0].Price {
		return true // would cross and execute immediately
	}

	return false
}
