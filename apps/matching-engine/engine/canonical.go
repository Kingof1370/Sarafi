package engine

import (
	"strings"
	"time"

	"velyxora/packages/types"
)

// canonicalStatusMap maps internal OMS lifecycle states onto the canonical
// wire status used by every transport (HTTP, Kafka, WebSocket, database).
var canonicalStatusMap = map[AdvancedOrderStatus]types.OrderStatus{
	StatusOMS_Created:           types.StatusNew,
	StatusOMS_PendingValidation: types.StatusNew,
	StatusOMS_Validated:         types.StatusNew,
	StatusOMS_Accepted:          types.StatusNew,
	StatusOMS_Queued:            types.StatusNew,
	StatusOMS_PartiallyFilled:   types.StatusPartiallyFilled,
	StatusOMS_Filled:            types.StatusFilled,
	StatusOMS_Cancelled:         types.StatusCancelled,
	StatusOMS_Expired:           types.StatusCancelled,
	StatusOMS_Rejected:          types.StatusRejected,
	StatusOMS_Failed:            types.StatusRejected,
	StatusOMS_Archived:          types.StatusFilled,
}

// OrderFromCanonical converts the canonical, transport level order DTO into the
// engine's internal advanced order. It is the single conversion point used by
// every entry path (Kafka consumer, internal HTTP API, tests) so that no field
// can be silently dropped by a second, divergent mapping.
func OrderFromCanonical(o *types.Order) *AdvancedOrder {
	if o == nil {
		return nil
	}

	now := time.Now().UTC()
	adv := &AdvancedOrder{
		ID:            o.ID,
		ClientOrderID: o.ClientOrderID,
		ExternalRefID: o.ExternalRefID,
		ExecutionID:   o.ExecutionID,
		CorrelationID: o.CorrelationID,
		UserID:        o.UserID,
		Symbol:        strings.ToUpper(o.Symbol),
		Side:          strings.ToUpper(string(o.Side)),
		Type:          strings.ToUpper(string(o.Type)),
		Price:         o.Price,
		Quantity:      o.Quantity,
		FilledQty:     o.FilledQty,
		Status:        StatusOMS_Created,
		TimeInForce:   TimeInForce(strings.ToUpper(o.TimeInForce)),
		StopPrice:     o.StopPrice,
		TrailingDelta: o.TrailingDelta,
		IcebergSize:   o.IcebergSize,
		PostOnly:      o.PostOnly,
		ReduceOnly:    o.ReduceOnly,
		CreatedAt:     o.CreatedAt,
		UpdatedAt:     o.UpdatedAt,
	}

	if adv.TimeInForce == "" {
		adv.TimeInForce = TIF_GTC
	}
	if adv.CreatedAt.IsZero() {
		adv.CreatedAt = now
	}
	if adv.UpdatedAt.IsZero() {
		adv.UpdatedAt = now
	}
	return adv
}

// OrderToCanonical converts an internal advanced order back to the canonical
// DTO returned to callers of the trading core.
func OrderToCanonical(a *AdvancedOrder) *types.Order {
	if a == nil {
		return nil
	}

	status, ok := canonicalStatusMap[a.Status]
	if !ok {
		status = types.StatusNew
	}

	return &types.Order{
		ID:            a.ID,
		UserID:        a.UserID,
		Symbol:        a.Symbol,
		Side:          types.OrderSide(a.Side),
		Type:          types.OrderType(a.Type),
		Price:         a.Price,
		Quantity:      a.Quantity,
		FilledQty:     a.FilledQty,
		Status:        status,
		TimeInForce:   string(a.TimeInForce),
		PostOnly:      a.PostOnly,
		CreatedAt:     a.CreatedAt,
		UpdatedAt:     a.UpdatedAt,
		ClientOrderID: a.ClientOrderID,
		ExternalRefID: a.ExternalRefID,
		ExecutionID:   a.ExecutionID,
		CorrelationID: a.CorrelationID,
		StopPrice:     a.StopPrice,
		TrailingDelta: a.TrailingDelta,
		IcebergSize:   a.IcebergSize,
		ReduceOnly:    a.ReduceOnly,
	}
}

// OrdersToCanonical converts a slice of internal orders to canonical DTOs.
func OrdersToCanonical(in []*AdvancedOrder) []types.Order {
	out := make([]types.Order, 0, len(in))
	for _, a := range in {
		if c := OrderToCanonical(a); c != nil {
			out = append(out, *c)
		}
	}
	return out
}

// SplitSymbol derives the (quote, base) asset pair from a canonical symbol such
// as "BTC-USDT". It is the single place where symbol composition is decoded, so
// no service has to hardcode a market's assets.
func SplitSymbol(symbol string) (quote string, base string) {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	sep := strings.IndexAny(s, "-_/")
	if sep <= 0 || sep == len(s)-1 {
		return "", s
	}
	return s[sep+1:], s[:sep]
}
