package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
	"velyxora/packages/database"
)

// OMSStateMachine implements thread-safe lifecycle transitions with immutable audit trails
type OMSStateMachine struct {
	mu           sync.RWMutex
	db           *database.DB
	activeOrders map[string]*AdvancedOrder
	auditLogs    []*AuditLogEntry
}

// NewOMSStateMachine creates an order state coordinator
func NewOMSStateMachine() *OMSStateMachine {
	return &OMSStateMachine{
		activeOrders: make(map[string]*AdvancedOrder),
		auditLogs:    make([]*AuditLogEntry, 0),
	}
}

// SetDB dynamically configures database connection
func (sm *OMSStateMachine) SetDB(db *database.DB) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.db = db
}

// SubmitOrder records the initial "Created" order state in OMS
func (sm *OMSStateMachine) SubmitOrder(order *AdvancedOrder, ip, device string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	order.Status = StatusOMS_Created
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	sm.activeOrders[order.ID] = order

	if sm.db != nil {
		ctx := context.Background()
		_, err := sm.db.Pool.Exec(ctx,
			`INSERT INTO orders (id, client_order_id, external_reference_id, execution_id, correlation_id, user_id, symbol, side, type, price, quantity, filled_quantity, status, time_in_force, expire_time, stop_price, trailing_delta, iceberg_size, post_only, reduce_only, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`,
			order.ID, order.ClientOrderID, order.ExternalRefID, order.ExecutionID, order.CorrelationID, order.UserID, order.Symbol, order.Side, order.Type, order.Price, order.Quantity, order.FilledQty, string(order.Status), string(order.TimeInForce), order.ExpireTime, order.StopPrice, order.TrailingDelta, order.IcebergSize, order.PostOnly, order.ReduceOnly, order.CreatedAt, order.UpdatedAt)
		if err != nil {
			panic(fmt.Errorf("failed to persist order to database: %w", err))
		}
	}

	// Add Audit trail
	sm.logAudit(order.ID, order.UserID, "CREATE", "", StatusOMS_Created, ip, device, "Order initialized")
}

// TransitionOrder validates the state transition rules and updates order parameters
func (sm *OMSStateMachine) TransitionOrder(orderID string, targetState AdvancedOrderStatus, action, ip, device, reason string) (*AdvancedOrder, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	order, exists := sm.activeOrders[orderID]
	if !exists {
		if sm.db != nil {
			var err error
			order, err = sm.loadOrderFromDB(context.Background(), orderID)
			if err != nil {
				return nil, fmt.Errorf("order %s not found: %w", orderID, err)
			}
			sm.activeOrders[orderID] = order
		} else {
			return nil, fmt.Errorf("order %s not found in state machine", orderID)
		}
	}

	previous := order.Status

	// Enforce strict state machine transitions
	if err := validateTransition(previous, targetState); err != nil {
		sm.logAudit(orderID, order.UserID, action, previous, targetState, ip, device, fmt.Sprintf("REJECTED: %v", err))
		return nil, err
	}

	order.Status = targetState
	order.UpdatedAt = time.Now()

	if sm.db != nil {
		ctx := context.Background()
		_, err := sm.db.Pool.Exec(ctx,
			"UPDATE orders SET status = $1, filled_quantity = $2, updated_at = $3 WHERE id = $4",
			string(order.Status), order.FilledQty, order.UpdatedAt, order.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to persist order transition: %w", err)
		}
	}

	sm.logAudit(orderID, order.UserID, action, previous, targetState, ip, device, reason)
	return order, nil
}

// GetOrder retrieves an advanced order from memory or DB
func (sm *OMSStateMachine) GetOrder(orderID string) (*AdvancedOrder, error) {
	sm.mu.RLock()
	order, exists := sm.activeOrders[orderID]
	sm.mu.RUnlock()

	if exists {
		return order, nil
	}

	if sm.db != nil {
		order, err := sm.loadOrderFromDB(context.Background(), orderID)
		if err == nil {
			sm.mu.Lock()
			sm.activeOrders[orderID] = order
			sm.mu.Unlock()
			return order, nil
		}
	}

	return nil, fmt.Errorf("order %s not found", orderID)
}

// loadOrderFromDB loads a single order from Postgres
func (sm *OMSStateMachine) loadOrderFromDB(ctx context.Context, orderID string) (*AdvancedOrder, error) {
	var o AdvancedOrder
	var statusStr, tifStr string
	err := sm.db.Pool.QueryRow(ctx,
		`SELECT id, client_order_id, external_reference_id, execution_id, correlation_id, user_id, symbol, side, type, price, quantity, filled_quantity, status, time_in_force, expire_time, stop_price, trailing_delta, iceberg_size, post_only, reduce_only, created_at, updated_at
		 FROM orders WHERE id = $1`, orderID).Scan(
		&o.ID, &o.ClientOrderID, &o.ExternalRefID, &o.ExecutionID, &o.CorrelationID, &o.UserID, &o.Symbol, &o.Side, &o.Type, &o.Price, &o.Quantity, &o.FilledQty, &statusStr, &tifStr, &o.ExpireTime, &o.StopPrice, &o.TrailingDelta, &o.IcebergSize, &o.PostOnly, &o.ReduceOnly, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	o.Status = AdvancedOrderStatus(statusStr)
	o.TimeInForce = TimeInForce(tifStr)
	return &o, nil
}

// GetAuditLogs retrieves historical audit logging entries
func (sm *OMSStateMachine) GetAuditLogs(orderID string) []*AuditLogEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	logs := make([]*AuditLogEntry, 0)
	for _, log := range sm.auditLogs {
		if log.OrderID == orderID {
			logs = append(logs, log)
		}
	}
	return logs
}

// GetUserOrders returns all tracked orders for a user
func (sm *OMSStateMachine) GetUserOrders(userID string) []*AdvancedOrder {
	if sm.db != nil {
		ctx := context.Background()
		rows, err := sm.db.Pool.Query(ctx,
			`SELECT id, client_order_id, external_reference_id, execution_id, correlation_id, user_id, symbol, side, type, price, quantity, filled_quantity, status, time_in_force, expire_time, stop_price, trailing_delta, iceberg_size, post_only, reduce_only, created_at, updated_at
			 FROM orders WHERE user_id = $1 ORDER BY created_at DESC`, userID)
		if err == nil {
			defer rows.Close()
			orders := make([]*AdvancedOrder, 0)
			for rows.Next() {
				var o AdvancedOrder
				var statusStr, tifStr string
				errScan := rows.Scan(
					&o.ID, &o.ClientOrderID, &o.ExternalRefID, &o.ExecutionID, &o.CorrelationID, &o.UserID, &o.Symbol, &o.Side, &o.Type, &o.Price, &o.Quantity, &o.FilledQty, &statusStr, &tifStr, &o.ExpireTime, &o.StopPrice, &o.TrailingDelta, &o.IcebergSize, &o.PostOnly, &o.ReduceOnly, &o.CreatedAt, &o.UpdatedAt)
				if errScan == nil {
					o.Status = AdvancedOrderStatus(statusStr)
					o.TimeInForce = TimeInForce(tifStr)
					orders = append(orders, &o)
				}
			}
			return orders
		}
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	orders := make([]*AdvancedOrder, 0)
	for _, order := range sm.activeOrders {
		if order.UserID == userID {
			orders = append(orders, order)
		}
	}
	return orders
}

func (sm *OMSStateMachine) logAudit(orderID, userID, action string, prev, next AdvancedOrderStatus, ip, device, result string) {
	log := &AuditLogEntry{
		ID:            fmt.Sprintf("aud_%d", time.Now().UnixNano()),
		OrderID:       orderID,
		UserID:        userID,
		Action:        action,
		PreviousState: prev,
		NewState:      next,
		IPAddress:     ip,
		DeviceID:      device,
		Result:        result,
		Timestamp:     time.Now(),
	}
	sm.auditLogs = append(sm.auditLogs, log)
}

func validateTransition(prev, next AdvancedOrderStatus) error {
	if prev == next {
		return nil
	}

	// Final states are immutable
	if prev == StatusOMS_Filled || prev == StatusOMS_Cancelled || prev == StatusOMS_Rejected || prev == StatusOMS_Expired || prev == StatusOMS_Failed || prev == StatusOMS_Archived {
		return fmt.Errorf("cannot transition from final state %s to %s", prev, next)
	}

	switch next {
	case StatusOMS_PendingValidation:
		if prev != StatusOMS_Created {
			return fmt.Errorf("cannot transition to PENDING_VALIDATION from %s", prev)
		}
	case StatusOMS_Validated:
		if prev != StatusOMS_PendingValidation {
			return fmt.Errorf("cannot transition to VALIDATED from %s", prev)
		}
	case StatusOMS_Accepted:
		if prev != StatusOMS_Validated {
			return fmt.Errorf("cannot transition to ACCEPTED from %s", prev)
		}
	case StatusOMS_Queued:
		if prev != StatusOMS_Accepted {
			return fmt.Errorf("cannot transition to QUEUED from %s", prev)
		}
	case StatusOMS_PartiallyFilled:
		if prev != StatusOMS_Queued && prev != StatusOMS_Accepted {
			return fmt.Errorf("cannot transition to PARTIALLY_FILLED from %s", prev)
		}
	case StatusOMS_Filled:
		if prev != StatusOMS_Queued && prev != StatusOMS_Accepted && prev != StatusOMS_PartiallyFilled {
			return fmt.Errorf("cannot transition to FILLED from %s", prev)
		}
	case StatusOMS_Cancelled:
		// Active orders can be cancelled at any point
		if prev == StatusOMS_Created {
			return fmt.Errorf("cannot cancel order before validation starts")
		}
	}

	return nil
}
