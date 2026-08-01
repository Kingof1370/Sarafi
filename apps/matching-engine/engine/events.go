package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
	"velyxora/packages/common"
	"velyxora/packages/types"
)

// EventsEngine dispatches trade and depth updates onto Kafka topic loops
type EventsEngine struct {
	mu       sync.Mutex
	producer *common.KafkaProducer
}

// NewEventsEngine creates a new message dispatch facility
func NewEventsEngine(producer *common.KafkaProducer) *EventsEngine {
	return &EventsEngine{
		producer: producer,
	}
}

// PublishTradeMatch sends matched execution records downstream
func (ee *EventsEngine) PublishTradeMatch(ctx context.Context, exec *Execution) error {
	ee.mu.Lock()
	defer ee.mu.Unlock()

	if ee.producer != nil {
		event := types.KafkaEvent{
			Type:      types.EventOrderMatch,
			Payload:   exec,
			Timestamp: exec.Timestamp,
		}
		err := ee.producer.Publish(ctx, "velyxora-trades", exec.TradeID, event)
		if err != nil {
			return fmt.Errorf("failed to publish trade match event: %w", err)
		}
	}

	return nil
}

// PublishOMSStateChange broadcasts custom transition updates downstream
func (ee *EventsEngine) PublishOMSStateChange(ctx context.Context, orderID string, action string, payload interface{}) error {
	ee.mu.Lock()
	defer ee.mu.Unlock()

	if ee.producer != nil {
		event := types.KafkaEvent{
			Type:      types.EventType(action),
			Payload:   payload,
			Timestamp: time.Now(),
		}
		err := ee.producer.Publish(ctx, "velyxora-orders-audit", orderID, event)
		if err != nil {
			return fmt.Errorf("failed to publish audit state event: %w", err)
		}
	}

	return nil
}

// PublishDepthUpdate streams market order book snapshots to the real-time feeds
func (ee *EventsEngine) PublishDepthUpdate(ctx context.Context, l2 *types.OrderBookL2) error {
	ee.mu.Lock()
	defer ee.mu.Unlock()

	if ee.producer != nil {
		event := types.KafkaEvent{
			Type:      types.EventBalanceUpdate, // reuse type structure or customize
			Payload:   l2,
			Timestamp: l2.Timestamp,
		}
		err := ee.producer.Publish(ctx, "velyxora-depth", l2.Symbol, event)
		if err != nil {
			return fmt.Errorf("failed to publish depth update event: %w", err)
		}
	}

	return nil
}
