package engine

import (
	"fmt"
	"sync"
	"time"
	"velyxora/packages/types"
)

// OMS manages order validation, lifecycle states, and index querying
type OMS struct {
	mu          sync.RWMutex
	orders      map[string]*types.Order
	userOrders  map[string][]string // userID -> orderIDs
	symbolOrders map[string][]string // symbol -> orderIDs
}

// NewOMS initializes the Order Management System
func NewOMS() *OMS {
	return &OMS{
		orders:       make(map[string]*types.Order),
		userOrders:   make(map[string][]string),
		symbolOrders: make(map[string][]string),
	}
}

// TrackOrder registers an order into active OMS tracking
func (o *OMS) TrackOrder(order *types.Order) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.orders[order.ID]; exists {
		return fmt.Errorf("order %s is already tracked", order.ID)
	}

	o.orders[order.ID] = order
	o.userOrders[order.UserID] = append(o.userOrders[order.UserID], order.ID)
	o.symbolOrders[order.Symbol] = append(o.symbolOrders[order.Symbol], order.ID)
	return nil
}

// UpdateOrderState handles lifecycle transitions safely
func (o *OMS) UpdateOrderState(orderID string, status types.OrderStatus, filledQty float64) (*types.Order, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	order, exists := o.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order %s not found in OMS", orderID)
	}

	// State validation
	if order.Status == types.StatusFilled || order.Status == types.StatusCancelled || order.Status == types.StatusRejected {
		return nil, fmt.Errorf("cannot transition order %s from finalized state %s", orderID, order.Status)
	}

	order.Status = status
	order.FilledQty = filledQty
	order.UpdatedAt = time.Now()

	return order, nil
}

// GetOrder fetches a single order by ID
func (o *OMS) GetOrder(orderID string) (*types.Order, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	order, exists := o.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order %s not found", orderID)
	}
	return order, nil
}

// GetUserOrders returns active/past orders for a user
func (o *OMS) GetUserOrders(userID string) []*types.Order {
	o.mu.RLock()
	defer o.mu.RUnlock()

	ids := o.userOrders[userID]
	orders := make([]*types.Order, 0, len(ids))
	for _, id := range ids {
		if ord, ok := o.orders[id]; ok {
			orders = append(orders, ord)
		}
	}
	return orders
}
