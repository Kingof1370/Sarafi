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

// Matcher implements the high-speed price-time matching logic
type Matcher struct {
	mu     sync.RWMutex
	Symbol string
	Bids   []*MatchLimit // buy orders sorted high to low
	Asks   []*MatchLimit // sell orders sorted low to high
}

// NewMatcher creates a book matching level
func NewMatcher(symbol string) *Matcher {
	return &Matcher{
		Symbol: symbol,
		Bids:   make([]*MatchLimit, 0),
		Asks:   make([]*MatchLimit, 0),
	}
}

// MatchOrder matches incoming buy/sell orders and outputs executions
func (m *Matcher) MatchOrder(order *types.Order) []*types.Trade {
	m.mu.Lock()
	defer m.mu.Unlock()

	var trades []*types.Trade

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

				trade := &types.Trade{
					ID:          "trd_" + strconv.FormatInt(time.Now().UnixNano(), 10),
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
			}

			if len(limit.Orders) == 0 {
				m.Asks = append(m.Asks[:i], m.Asks[i+1:]...)
			} else {
				i++
			}
		}

		// If there's quantity left, place on Bid book (except market orders)
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

				trade := &types.Trade{
					ID:          "trd_" + strconv.FormatInt(time.Now().UnixNano(), 10),
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
			}

			if len(limit.Orders) == 0 {
				m.Bids = append(m.Bids[:i], m.Bids[i+1:]...)
			} else {
				i++
			}
		}

		// If there's quantity left, place on Ask book (except market orders)
		if order.FilledQty < order.Quantity && order.Type == types.TypeLimit {
			m.addOrderToBook(order, &m.Asks, false)
		}
	}

	return trades
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

	return removeFromLimits(&m.Bids) || removeFromLimits(&m.Asks)
}

// GetL2Depth exports the current bid/ask price depth snapshot
func (m *Matcher) GetL2Depth(maxLevels int) *types.OrderBookL2 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	l2 := &types.OrderBookL2{
		Symbol:    m.Symbol,
		Bids:      make([]types.OrderBookLevel, 0, maxLevels),
		Asks:      make([]types.OrderBookLevel, 0, maxLevels),
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

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
