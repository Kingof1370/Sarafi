package engine

import (
	"fmt"
	"sync"
	"time"
)

// OMSStateMachine implements thread-safe lifecycle transitions with immutable audit trails
type OMSStateMachine struct {
	mu           sync.RWMutex
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

// SubmitOrder records the initial "Created" order state in OMS
func (sm *OMSStateMachine) SubmitOrder(order *AdvancedOrder, ip, device string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	order.Status = StatusOMS_Created
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	sm.activeOrders[order.ID] = order

	// Add Audit trail
	sm.logAudit(order.ID, order.UserID, "CREATE", "", StatusOMS_Created, ip, device, "Order initialized")
}

// TransitionOrder validates the state transition rules and updates order parameters
func (sm *OMSStateMachine) TransitionOrder(orderID string, targetState AdvancedOrderStatus, action, ip, device, reason string) (*AdvancedOrder, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	order, exists := sm.activeOrders[orderID]
	if !exists {
		return nil, fmt.Errorf("order %s not found in state machine", orderID)
	}

	previous := order.Status

	// Enforce strict state machine transitions
	if err := validateTransition(previous, targetState); err != nil {
		sm.logAudit(orderID, order.UserID, action, previous, targetState, ip, device, fmt.Sprintf("REJECTED: %v", err))
		return nil, err
	}

	order.Status = targetState
	order.UpdatedAt = time.Now()

	sm.logAudit(orderID, order.UserID, action, previous, targetState, ip, device, reason)
	return order, nil
}

// GetOrder retrieves an advanced order from memory
func (sm *OMSStateMachine) GetOrder(orderID string) (*AdvancedOrder, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	order, exists := sm.activeOrders[orderID]
	if !exists {
		return nil, fmt.Errorf("order %s not found", orderID)
	}
	return order, nil
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
