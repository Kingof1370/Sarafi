package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"velyxora/packages/common"
	"velyxora/packages/logger"
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

func main() {
	log := logger.NewLogger(logger.Config{
		Level:       "INFO",
		Format:      "JSON",
		ServiceName: "matching-engine",
	})

	log.Info("Starting Velyxora Matching Engine...")

	kafkaBrokers := strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ",")
	consumer := common.NewKafkaConsumer(kafkaBrokers, "velyxora-orders", "matching-engine-group")
	producer := common.NewKafkaProducer(kafkaBrokers)

	defer consumer.Close()
	defer producer.Close()

	// Initializing local orderbooks for standard symbols
	books := map[string]*OrderBook{
		"BTC-USDT": NewOrderBook("BTC-USDT"),
		"ETH-USDT": NewOrderBook("ETH-USDT"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Graceful Shutdown Signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info("Shutdown signal received. Stopping consumer...")
		cancel()
	}()

	log.Info("Listening to order requests from Kafka...")

	err := consumer.Consume(ctx, func(ctx context.Context, key string, value []byte) error {
		var event types.KafkaEvent
		if err := json.Unmarshal(value, &event); err != nil {
			log.Error(fmt.Sprintf("Failed to parse Kafka event: %v", err))
			return nil
		}

		if event.Type == types.EventOrderCreated {
			// Extract order payload
			orderData, err := json.Marshal(event.Payload)
			if err != nil {
				return err
			}

			var order types.Order
			if err := json.Unmarshal(orderData, &order); err != nil {
				return err
			}

			log.Info("Processing Order", "order_id", order.ID, "symbol", order.Symbol, "side", order.Side, "price", order.Price, "quantity", order.Quantity)

			om := common.GetObservabilityManager()
			om.OrdersSubmitted.WithLabelValues(order.Symbol, string(order.Side), string(order.Type)).Inc()
			om.OrdersAccepted.WithLabelValues(order.Symbol, string(order.Side)).Inc()

			book, exists := books[order.Symbol]
			if !exists {
				book = NewOrderBook(order.Symbol)
				books[order.Symbol] = book
			}

			// Perform Limit Matching Logic (measure latency)
			startMatching := time.Now()
			matches := book.ProcessLimitOrder(&order)
			om.MatchingLatencySeconds.WithLabelValues("matching", order.Symbol).Observe(time.Since(startMatching).Seconds())

			for _, trade := range matches {
				log.Info("Match Found!", "price", trade.Price, "quantity", trade.Quantity, "buyer", trade.BuyerID, "seller", trade.SellerID)

				// Increment business SRE metrics
				om.OrdersMatched.WithLabelValues(order.Symbol).Inc()
				om.TradeCount.WithLabelValues(order.Symbol).Inc()
				om.TradingVolume.WithLabelValues(order.Symbol).Add(trade.Quantity)

				// Publish Trade Event
				tradeEvent := types.KafkaEvent{
					Type:      types.EventOrderMatch,
					Payload:   trade,
					Timestamp: time.Now(),
				}
				_ = producer.Publish(ctx, "velyxora-trades", trade.ID, tradeEvent)
			}
		}

		return nil
	})

	if err != nil && err != context.Canceled {
		log.Error(fmt.Sprintf("Consumer loop exited with error: %v", err))
	}
}

// ProcessLimitOrder processes an order and matches it with existing orders in the book
func (ob *OrderBook) ProcessLimitOrder(order *types.Order) []*types.Trade {
	var trades []*types.Trade

	if order.Side == types.SideBuy {
		// Try to match with Asks (Sell orders sorted lowest price first)
		for i := 0; i < len(ob.Asks) && order.Quantity > order.FilledQty; {
			limit := ob.Asks[i]
			if limit.Price > order.Price && order.Type == types.TypeLimit {
				break // Buy limit price is lower than sell ask price, no match
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
					limit.Orders = limit.Orders[1:] // pop
				} else {
					sellOrder.Status = types.StatusPartiallyFilled
				}

				trade := &types.Trade{
					ID:          "trd_" + strconv.FormatInt(time.Now().UnixNano(), 10),
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
				ob.Asks = append(ob.Asks[:i], ob.Asks[i+1:]...) // remove empty price level
			} else {
				i++
			}
		}

		// If not fully filled, add to Bids order book
		if order.FilledQty < order.Quantity {
			ob.AddOrderToBook(order, &ob.Bids, true)
		}

	} else {
		// Try to match with Bids (Buy orders sorted highest price first)
		for i := 0; i < len(ob.Bids) && order.Quantity > order.FilledQty; {
			limit := ob.Bids[i]
			if limit.Price < order.Price && order.Type == types.TypeLimit {
				break // Sell limit price is higher than buy bid price, no match
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
					limit.Orders = limit.Orders[1:] // pop
				} else {
					buyOrder.Status = types.StatusPartiallyFilled
				}

				trade := &types.Trade{
					ID:          "trd_" + strconv.FormatInt(time.Now().UnixNano(), 10),
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
				ob.Bids = append(ob.Bids[:i], ob.Bids[i+1:]...) // remove empty price level
			} else {
				i++
			}
		}

		// If not fully filled, add to Asks order book
		if order.FilledQty < order.Quantity {
			ob.AddOrderToBook(order, &ob.Asks, false)
		}
	}

	return trades
}

func (ob *OrderBook) AddOrderToBook(order *types.Order, limits *[]*Limit, desc bool) {
	// Simple linear insertion for price priority
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

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}
