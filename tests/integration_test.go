package tests

import (
	"testing"
	"time"

	"velyxora/packages/types"
)

// Limit represents a specific price level in the order book
type Limit struct {
	Price  float64
	Orders []*types.Order
}

// OrderBook matches bids and asks using standard Price-Time priority
type OrderBook struct {
	Symbol string
	Bids   []*Limit // Buy orders sorted high to low
	Asks   []*Limit // Sell orders sorted low to high
}

func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		Symbol: symbol,
		Bids:   make([]*Limit, 0),
		Asks:   make([]*Limit, 0),
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (ob *OrderBook) AddOrderToBook(order *types.Order, limits *[]*Limit, desc bool) {
	price := order.Price
	for i, limit := range *limits {
		if limit.Price == price {
			limit.Orders = append(limit.Orders, order)
			return
		}

		if (desc && price > limit.Price) || (!desc && price < limit.Price) {
			newLimit := &Limit{Price: price, Orders: []*types.Order{order}}
			*limits = append((*limits)[:i], append([]*Limit{newLimit}, (*limits)[i:]...)...)
			return
		}
	}

	*limits = append(*limits, &Limit{Price: price, Orders: []*types.Order{order}})
}

func (ob *OrderBook) ProcessLimitOrder(order *types.Order) []*types.Trade {
	var trades []*types.Trade

	if order.Side == types.SideBuy {
		for i := 0; i < len(ob.Asks) && order.Quantity > order.FilledQty; {
			limit := ob.Asks[i]
			if limit.Price > order.Price && order.Type == types.TypeLimit {
				break
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
					ID:          "trd_mock_123",
					Symbol:      ob.Symbol,
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
				ob.Asks = append(ob.Asks[:i], ob.Asks[i+1:]...)
			} else {
				i++
			}
		}

		if order.FilledQty < order.Quantity {
			ob.AddOrderToBook(order, &ob.Bids, true)
		}

	} else {
		for i := 0; i < len(ob.Bids) && order.Quantity > order.FilledQty; {
			limit := ob.Bids[i]
			if limit.Price < order.Price && order.Type == types.TypeLimit {
				break
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
					ID:          "trd_mock_123",
					Symbol:      ob.Symbol,
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
				ob.Bids = append(ob.Bids[:i], ob.Bids[i+1:]...)
			} else {
				i++
			}
		}

		if order.FilledQty < order.Quantity {
			ob.AddOrderToBook(order, &ob.Asks, false)
		}
	}

	return trades
}

// TestCoreOrderBookEngineIntegration executes a flow representing an end-to-end matching loop.
func TestCoreOrderBookEngineIntegration(t *testing.T) {
	ob := NewOrderBook("BTC-USDT")

	// Order 1: Seller placing BTC-USDT at 50,000 USDT for 2.0 BTC
	sellOrder := &types.Order{
		ID:        "ord_sell_001",
		UserID:    "usr_seller",
		Symbol:    "BTC-USDT",
		Side:      types.SideSell,
		Type:      types.TypeLimit,
		Price:     50000.0,
		Quantity:  2.0,
		FilledQty: 0.0,
		Status:    types.StatusNew,
	}

	ob.ProcessLimitOrder(sellOrder)

	// Order 2: Buyer placing BTC-USDT order matching that price for 1.5 BTC
	buyOrder := &types.Order{
		ID:        "ord_buy_001",
		UserID:    "usr_buyer",
		Symbol:    "BTC-USDT",
		Side:      types.SideBuy,
		Type:      types.TypeLimit,
		Price:     50000.0,
		Quantity:  1.5,
		FilledQty: 0.0,
		Status:    types.StatusNew,
	}

	trades := ob.ProcessLimitOrder(buyOrder)

	if len(trades) != 1 {
		t.Fatalf("Expected exactly 1 trade match, got %d", len(trades))
	}

	matchedTrade := trades[0]
	if matchedTrade.Quantity != 1.5 {
		t.Errorf("Expected traded quantity to be 1.5, got %f", matchedTrade.Quantity)
	}

	if buyOrder.Status != types.StatusFilled {
		t.Errorf("Expected buy order status to be FILLED, got %s", buyOrder.Status)
	}

	if sellOrder.Status != types.StatusPartiallyFilled {
		t.Errorf("Expected sell order status to be PARTIALLY_FILLED, got %s", sellOrder.Status)
	}
}
