package engine

import (
	"fmt"
	"time"
	"velyxora/packages/types"
)

// LiquidityProvider defines the clean abstraction for external liquidity partners
type LiquidityProvider interface {
	ID() string
	GetOrderBook(symbol string) (*types.OrderBookL2, error)
	ExecuteOrder(order *types.Order) (*types.Trade, error)
	GetStatus() string
}

// TestLiquidityProvider is an isolated mock provider used strictly for tests
type TestLiquidityProvider struct {
	id     string
	status string
}

func NewTestLiquidityProvider(id string) *TestLiquidityProvider {
	return &TestLiquidityProvider{
		id:     id,
		status: "CONNECTED",
	}
}

func (tlp *TestLiquidityProvider) ID() string {
	return tlp.id
}

func (tlp *TestLiquidityProvider) GetStatus() string {
	return tlp.status
}

func (tlp *TestLiquidityProvider) GetOrderBook(symbol string) (*types.OrderBookL2, error) {
	if tlp.status != "CONNECTED" {
		return nil, fmt.Errorf("provider %s is offline", tlp.id)
	}

	return &types.OrderBookL2{
		Symbol: symbol,
		Bids: []types.OrderBookLevel{
			{Price: 49990.0, Quantity: 1.5},
			{Price: 49980.0, Quantity: 2.0},
		},
		Asks: []types.OrderBookLevel{
			{Price: 50010.0, Quantity: 1.2},
			{Price: 50020.0, Quantity: 2.5},
		},
		Sequence:  1001,
		Timestamp: time.Now(),
	}, nil
}

func (tlp *TestLiquidityProvider) ExecuteOrder(order *types.Order) (*types.Trade, error) {
	if tlp.status != "CONNECTED" {
		return nil, fmt.Errorf("provider %s is offline", tlp.id)
	}

	// Safety Check: Never bypass risk controls.
	// External executions are treated as an untrusted boundary.
	if order.Quantity <= 0 || order.Price <= 0 {
		return nil, fmt.Errorf("invalid execution parameters")
	}

	return &types.Trade{
		ID:          fmt.Sprintf("trd_ext_%d", time.Now().UnixNano()),
		Symbol:      order.Symbol,
		BuyerID:     order.UserID,
		SellerID:    "EXT_LIQUIDITY_" + tlp.id,
		BuyOrderID:  order.ID,
		SellOrderID: "ext_sell_order_999",
		Price:       order.Price,
		Quantity:    order.Quantity,
		Timestamp:   time.Now(),
	}, nil
}
