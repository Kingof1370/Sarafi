package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"

	"velyxora/apps/matching-engine/engine"
	"velyxora/packages/common"
	"velyxora/packages/database"
	"velyxora/packages/logger"
	"velyxora/packages/types"
)

func main() {
	log := logger.NewLogger(logger.Config{
		Level:       getEnv("LOG_LEVEL", "INFO"),
		Format:      getEnv("LOG_FORMAT", "JSON"),
		ServiceName: "matching-engine",
	})

	log.Info("Starting Velyxora Authoritative Matching Engine...")

	appEnv := getEnv("APP_ENV", "production")

	// 1. Initialize PostgreSQL Database Pool
	var db *database.DB
	var dbErr error
	db, dbErr = database.NewConnectionPool(database.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     5432,
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		DBName:   getEnv("DB_NAME", "velyxora"),
		SSLMode:  "disable",
	})

	// Fail Closed in Production if DB is unavailable
	if dbErr != nil {
		if appEnv == "production" {
			log.Error(fmt.Sprintf("FAIL CLOSED: PostgreSQL database pool is unavailable: %v", dbErr))
			os.Exit(1)
		}
		log.Warn(fmt.Sprintf("Failed to initialize DB pool: %v", dbErr))
	} else {
		log.Info("PostgreSQL connection pool initialized successfully.")
	}

	// 2. Initialize Kafka Consumer and Producer
	kafkaBrokers := strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ",")
	if appEnv == "production" {
		if len(kafkaBrokers) == 0 || kafkaBrokers[0] == "" {
			log.Error("FAIL CLOSED: Kafka brokers are not configured!")
			os.Exit(1)
		}
		if err := probeKafkaHealth(context.Background(), kafkaBrokers); err != nil {
			log.Error(fmt.Sprintf("FAIL CLOSED: Kafka brokers unreachable: %v", err))
			os.Exit(1)
		}
	}

	consumer := common.NewKafkaConsumer(kafkaBrokers, "velyxora-orders", "matching-engine-group")
	producer := common.NewKafkaProducer(kafkaBrokers)

	defer consumer.Close()
	defer producer.Close()

	// 3. Initialize single authoritative OMSRouter & engine stack
	sm := engine.NewOMSStateMachine()
	val := engine.NewOMSValidator()
	risk := engine.NewRiskEngine(10.0, 100000.0)
	fees := engine.NewFeesEngine(0.0010, 0.0020)
	exec := engine.NewExecutionEngine(fees, risk)
	settle := engine.NewSettlementEngine(db)

	// Markets are created on demand by the MarketRegistry; no market is hardcoded.
	omsRouter := engine.NewOMSRouter(sm, val, risk, nil, exec, settle)
	if db != nil {
		omsRouter.SetDB(db)
	}
	marketServices := engine.NewMarketServices()
	candleEngine := engine.NewCandleEngine(db, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Synchronously publish trades/depth to Kafka
	omsRouter.OnTradeMatched = func(symbol string, price, quantity float64, timestamp time.Time) {
		// The trading core owns market statistics; gateways only read them.
		marketServices.RecordTrade(symbol, price, quantity)
		tradePayload := map[string]interface{}{
			"symbol":    symbol,
			"price":     price,
			"quantity":  quantity,
			"timestamp": timestamp.UnixNano() / 1e6,
		}
		event := types.KafkaEvent{
			Type:      types.EventOrderMatch,
			Payload:   tradePayload,
			Timestamp: timestamp,
		}
		_ = producer.Publish(ctx, "velyxora-trades", symbol, event)
	}

	omsRouter.OnOrderBookChanged = func(symbol string) {
		m := omsRouter.GetMatcherForSymbol(symbol)
		if m != nil {
			depth := m.GetL2Depth(20)
			event := types.KafkaEvent{
				Type:      types.EventBalanceUpdate, // reuse or use depth type
				Payload:   depth,
				Timestamp: depth.Timestamp,
			}
			_ = producer.Publish(ctx, "velyxora-depth", symbol, event)
		}
	}

	// 3b. Expose the authoritative trading core over the internal HTTP API so
	// that the API gateway never instantiates an engine of its own.
	internalToken := getEnv("INTERNAL_SERVICE_TOKEN", "")
	if internalToken == "" {
		log.Error("FAIL CLOSED: INTERNAL_SERVICE_TOKEN is not configured; the trading core API cannot start")
		os.Exit(1)
	}
	apiAddr := getEnv("MATCHING_ENGINE_HTTP_ADDR", ":8081")
	api := newInternalAPI(omsRouter, marketServices, candleEngine, internalToken, log)
	api.serve(ctx, apiAddr)
	log.Info("Internal trading core API listening", "addr", apiAddr)

	// Handle Graceful Shutdown Signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info("Shutdown signal received. Stopping consumer...")
		cancel()
	}()

	log.Info("Authoritative Matching Engine listening to velyxora-orders from Kafka...")

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

			var legacyOrder types.Order
			if err := json.Unmarshal(orderData, &legacyOrder); err != nil {
				return err
			}

			log.Info("Authoritative Matching Engine processing order", "order_id", legacyOrder.ID, "symbol", legacyOrder.Symbol, "side", legacyOrder.Side, "price", legacyOrder.Price, "quantity", legacyOrder.Quantity)

			om := common.GetObservabilityManager()
			om.OrdersSubmitted.WithLabelValues(legacyOrder.Symbol, string(legacyOrder.Side), string(legacyOrder.Type)).Inc()
			om.OrdersAccepted.WithLabelValues(legacyOrder.Symbol, string(legacyOrder.Side)).Inc()

			// Convert to AdvancedOrder
			order := &engine.AdvancedOrder{
				ID:            legacyOrder.ID,
				UserID:        legacyOrder.UserID,
				Symbol:        legacyOrder.Symbol,
				Side:          string(legacyOrder.Side),
				Type:          string(legacyOrder.Type),
				Price:         legacyOrder.Price,
				Quantity:      legacyOrder.Quantity,
				FilledQty:     legacyOrder.FilledQty,
				Status:        engine.StatusOMS_Created,
				TimeInForce:   engine.TimeInForce(legacyOrder.TimeInForce),
				CreatedAt:     legacyOrder.CreatedAt,
				UpdatedAt:     legacyOrder.UpdatedAt,

				// Advanced fields mapping alignment
				ClientOrderID: legacyOrder.ClientOrderID,
				ExternalRefID: legacyOrder.ExternalRefID,
				ExecutionID:   legacyOrder.ExecutionID,
				CorrelationID: legacyOrder.CorrelationID,
				StopPrice:     legacyOrder.StopPrice,
				TrailingDelta: legacyOrder.TrailingDelta,
				IcebergSize:   legacyOrder.IcebergSize,
				PostOnly:      legacyOrder.PostOnly,
				ReduceOnly:    legacyOrder.ReduceOnly,
			}
			if order.TimeInForce == "" {
				order.TimeInForce = engine.TIF_GTC
			}

			startMatching := time.Now()
			err = omsRouter.ProcessIncomingOrder(ctx, order, "USDT", "BTC", "KAFKA_CONSUMER", "KAFKA")
			om.MatchingLatencySeconds.WithLabelValues("matching", order.Symbol).Observe(time.Since(startMatching).Seconds())

			if err != nil {
				log.Error("Failed to process order in authoritative OMSRouter", "order_id", order.ID, "err", err)
			}
		}

		return nil
	})

	if err != nil && err != context.Canceled {
		log.Error(fmt.Sprintf("Consumer loop exited with error: %v", err))
	}
}

func probeKafkaHealth(ctx context.Context, brokers []string) error {
	if len(brokers) == 0 {
		return fmt.Errorf("no kafka brokers configured")
	}
	dialer := &kafka.Dialer{
		Timeout:   1 * time.Second,
		DualStack: true,
	}
	conn, err := dialer.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}
