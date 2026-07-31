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

	"velyxora/packages/common"
	"velyxora/packages/database"
	"velyxora/packages/logger"
	"velyxora/packages/types"
)

func main() {
	log := logger.NewLogger(logger.Config{
		Level:       "INFO",
		Format:      "JSON",
		ServiceName: "wallet-service",
	})

	log.Info("Starting Velyxora Wallet Service...")

	kafkaBrokers := strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ",")
	consumer := common.NewKafkaConsumer(kafkaBrokers, "velyxora-trades", "wallet-service-group")
	producer := common.NewKafkaProducer(kafkaBrokers)

	defer consumer.Close()
	defer producer.Close()

	// Connect to Database
	db, err := database.NewConnectionPool(database.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     5432,
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		DBName:   getEnv("DB_NAME", "velyxora"),
		SSLMode:  "disable",
	})

	if err != nil {
		log.Warn(fmt.Sprintf("Failed to connect to PG pool, operating in Mock local transaction memory state: %v", err))
	} else {
		defer db.Close()
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

	log.Info("Listening to trade results on Kafka to settle balances...")

	// In-memory balance simulation when PG is not present
	mockBalances := make(map[string]*types.Balance)

	err = consumer.Consume(ctx, func(key string, value []byte) error {
		var event types.KafkaEvent
		if err := json.Unmarshal(value, &event); err != nil {
			log.Error(fmt.Sprintf("Failed to parse Kafka event: %v", err))
			return nil
		}

		if event.Type == types.EventOrderMatch {
			tradeData, err := json.Marshal(event.Payload)
			if err != nil {
				return err
			}

			var trade types.Trade
			if err := json.Unmarshal(tradeData, &trade); err != nil {
				return err
			}

			log.Info("Settling Trade", "trade_id", trade.ID, "buyer_id", trade.BuyerID, "seller_id", trade.SellerID, "price", trade.Price, "qty", trade.Quantity)

			// Safely execute balance adjustment with transaction principles
			if db != nil {
				tx, err := db.Pool.Begin(ctx)
				if err != nil {
					log.Error("Failed to open postgres transaction")
					return err
				}
				defer tx.Rollback(ctx)

				// Base assets
				baseAsset := strings.Split(trade.Symbol, "-")[0]  // e.g. "BTC"
				quoteAsset := strings.Split(trade.Symbol, "-")[1] // e.g. "USDT"

				quoteAmount := trade.Price * trade.Quantity

				// 1. Debit Buyer's Quote Asset (e.g., USDT)
				_, err = tx.Exec(ctx,
					"UPDATE balances SET locked = locked - $1 WHERE user_id = $2 AND asset = $3",
					quoteAmount, trade.BuyerID, quoteAsset)
				if err != nil {
					return fmt.Errorf("debit buyer failed: %w", err)
				}

				// 2. Credit Buyer's Base Asset (e.g., BTC)
				_, err = tx.Exec(ctx,
					"INSERT INTO balances (user_id, asset, free, locked, updated_at) VALUES ($1, $2, $3, 0, NOW()) "+
						"ON CONFLICT (user_id, asset) DO UPDATE SET free = balances.free + $3",
					trade.BuyerID, baseAsset, trade.Quantity)
				if err != nil {
					return fmt.Errorf("credit buyer base failed: %w", err)
				}

				// 3. Debit Seller's Base Asset (e.g., BTC, previously locked during order placement)
				_, err = tx.Exec(ctx,
					"UPDATE balances SET locked = locked - $1 WHERE user_id = $2 AND asset = $3",
					trade.Quantity, trade.SellerID, baseAsset)
				if err != nil {
					return fmt.Errorf("debit seller failed: %w", err)
				}

				// 4. Credit Seller's Quote Asset (e.g., USDT)
				_, err = tx.Exec(ctx,
					"INSERT INTO balances (user_id, asset, free, locked, updated_at) VALUES ($1, $2, $3, 0, NOW()) "+
						"ON CONFLICT (user_id, asset) DO UPDATE SET free = balances.free + $3",
					trade.SellerID, quoteAsset, quoteAmount)
				if err != nil {
					return fmt.Errorf("credit seller quote failed: %w", err)
				}

				if err := tx.Commit(ctx); err != nil {
					return fmt.Errorf("ledger tx commit failed: %w", err)
				}
			} else {
				// Standalone simulation balance allocation (In-memory update)
				baseAsset := strings.Split(trade.Symbol, "-")[0]
				quoteAsset := strings.Split(trade.Symbol, "-")[1]
				quoteAmount := trade.Price * trade.Quantity

				// Buyer
				getOrInitMockBalance(mockBalances, trade.BuyerID, baseAsset).Free += trade.Quantity
				getOrInitMockBalance(mockBalances, trade.BuyerID, quoteAsset).Locked -= quoteAmount

				// Seller
				getOrInitMockBalance(mockBalances, trade.SellerID, baseAsset).Locked -= trade.Quantity
				getOrInitMockBalance(mockBalances, trade.SellerID, quoteAsset).Free += quoteAmount
			}

			// Broadcast balance updates downstream
			balanceEvent := types.KafkaEvent{
				Type:      types.EventBalanceUpdate,
				Payload:   trade, // Send trade details indicating the trigger for balance update
				Timestamp: time.Now(),
			}
			_ = producer.Publish(ctx, "velyxora-balances", trade.BuyerID, balanceEvent)
		}

		return nil
	})

	if err != nil && err != context.Canceled {
		log.Error(fmt.Sprintf("Consumer loop exited with error: %v", err))
	}
}

func getOrInitMockBalance(m map[string]*types.Balance, userID, asset string) *types.Balance {
	key := userID + "_" + asset
	if b, ok := m[key]; ok {
		return b
	}
	m[key] = &types.Balance{
		UserID: userID,
		Asset:  asset,
		Free:   10.0, // Pre-funded with mock amounts
		Locked: 10.0,
	}
	return m[key]
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}
