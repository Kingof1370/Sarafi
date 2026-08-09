package common

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaProducer encapsulates event publishing logic
type KafkaProducer struct {
	Writer *kafka.Writer
}

// KafkaConsumer encapsulates consumer subscription logic
type KafkaConsumer struct {
	Reader *kafka.Reader
}

// NewKafkaProducer establishes a connection to a Kafka broker list for writing events
func NewKafkaProducer(brokers []string) *KafkaProducer {
	return &KafkaProducer{
		Writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Balancer: &kafka.LeastBytes{},
			Async:    false, // Wait for confirmation for high-critical events
		},
	}
}

// Publish sends an event onto the specified Kafka topic with Trace Context injection
func (p *KafkaProducer) Publish(ctx context.Context, topic string, key string, event interface{}) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	headers := []kafka.Header{}
	om := GetObservabilityManager()
	if om != nil {
		carrier := MapCarrier{}
		om.InjectTrace(ctx, carrier)
		for k, v := range carrier {
			headers = append(headers, kafka.Header{
				Key:   k,
				Value: []byte(v),
			})
		}
	}

	err = p.Writer.WriteMessages(ctx, kafka.Message{
		Topic:   topic,
		Key:     []byte(key),
		Value:   payload,
		Headers: headers,
		Time:    time.Now(),
	})
	if err != nil {
		return fmt.Errorf("failed to publish message to topic %s: %w", topic, err)
	}

	return nil
}

// Close closes the producer connection safely
func (p *KafkaProducer) Close() error {
	return p.Writer.Close()
}

// NewKafkaConsumer initializes a reader that listens to Kafka events on a specific topic / group
func NewKafkaConsumer(brokers []string, topic string, groupID string) *KafkaConsumer {
	return &KafkaConsumer{
		Reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			GroupID:  groupID,
			Topic:    topic,
			MinBytes: 10e3, // 10KB
			MaxBytes: 10e6, // 10MB
			MaxWait:  500 * time.Millisecond,
		}),
	}
}

// Consume reads and triggers the callback for incoming Kafka messages with trace context propagation
func (c *KafkaConsumer) Consume(ctx context.Context, handler func(ctx context.Context, key string, value []byte) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := c.Reader.ReadMessage(ctx)
			if err != nil {
				return err
			}

			// Extract trace context from message headers
			msgCtx := ctx
			om := GetObservabilityManager()
			if om != nil && len(msg.Headers) > 0 {
				carrier := MapCarrier{}
				for _, h := range msg.Headers {
					carrier[h.Key] = string(h.Value)
				}
				msgCtx = om.ExtractTrace(ctx, carrier)
			}

			if err := handler(msgCtx, string(msg.Key), msg.Value); err != nil {
				// Log error, but continue consuming or retry depending on retry mechanism
				fmt.Printf("Error processing consumer callback: %v\n", err)
			}
		}
	}
}

// Close closes the consumer connection safely
func (c *KafkaConsumer) Close() error {
	return c.Reader.Close()
}
