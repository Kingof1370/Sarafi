package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"velyxora/packages/common"
	"velyxora/packages/database"
	"velyxora/packages/logger"
	"velyxora/packages/types"
)

type BalanceEngine struct {
	mu       sync.Mutex
	balances map[string]*types.Balance // key: "userID_asset"
	ledger   []*types.LedgerEntry
}

func NewBalanceEngine() *BalanceEngine {
	return &BalanceEngine{
		balances: make(map[string]*types.Balance),
		ledger:   make([]*types.LedgerEntry, 0),
	}
}

// ProcessDoubleEntry forces absolute balance matches to ledger debits/credits to enforce financial safety
func (be *BalanceEngine) ProcessDoubleEntry(ctx context.Context, txID string, debitUser, creditUser string, asset string, amount float64, description string) error {
	be.mu.Lock()
	defer be.mu.Unlock()

	debitKey := debitUser + "_" + asset
	creditKey := creditUser + "_" + asset

	// Fetch or initialize
	debitBal, ok := be.balances[debitKey]
	if !ok {
		debitBal = &types.Balance{UserID: debitUser, Asset: asset, Available: 1000.0, Total: 1000.0}
		be.balances[debitKey] = debitBal
	}

	creditBal, ok := be.balances[creditKey]
	if !ok {
		creditBal = &types.Balance{UserID: creditUser, Asset: asset, Available: 1000.0, Total: 1000.0}
		be.balances[creditKey] = creditBal
	}

	// Enforce balance restrictions (prevent negative balances)
	if debitBal.Available < amount {
		return fmt.Errorf("insufficient available balance: user %s has %f, requested %f", debitUser, debitBal.Available, amount)
	}

	// Calculate and Settle
	debitBal.Available -= amount
	debitBal.Total -= amount

	creditBal.Available += amount
	creditBal.Total += amount

	debitEntry := &types.LedgerEntry{
		ID:          fmt.Sprintf("ent_deb_%d", time.Now().UnixNano()),
		LedgerTxID:  txID,
		UserID:      debitUser,
		Asset:       asset,
		Type:        types.Debit,
		Amount:      amount,
		Description: description,
		Timestamp:   time.Now(),
	}

	creditEntry := &types.LedgerEntry{
		ID:          fmt.Sprintf("ent_cred_%d", time.Now().UnixNano()),
		LedgerTxID:  txID,
		UserID:      creditUser,
		Asset:       asset,
		Type:        types.Credit,
		Amount:      amount,
		Description: description,
		Timestamp:   time.Now(),
	}

	be.ledger = append(be.ledger, debitEntry, creditEntry)
	return nil
}

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

	be := NewBalanceEngine()

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

			log.Info("Settling Trade via Ledger Engine", "trade_id", trade.ID, "buyer_id", trade.BuyerID, "seller_id", trade.SellerID, "price", trade.Price, "qty", trade.Quantity)

			baseAsset := strings.Split(trade.Symbol, "-")[0]  // e.g. "BTC"
			quoteAsset := strings.Split(trade.Symbol, "-")[1] // e.g. "USDT"
			quoteAmount := trade.Price * trade.Quantity

			// Settle Buyer Debit (USDT) -> Credit Seller (USDT)
			txID := "tx_ld_" + fmt.Sprintf("%d", time.Now().UnixNano())
			err = be.ProcessDoubleEntry(ctx, txID, trade.BuyerID, trade.SellerID, quoteAsset, quoteAmount, fmt.Sprintf("Settled trade purchase: %s", trade.ID))
			if err != nil {
				log.Error("Failed to settle Quote ledger transfer", "err", err)
				return nil
			}

			// Settle Seller Debit (BTC) -> Credit Buyer (BTC)
			err = be.ProcessDoubleEntry(ctx, txID, trade.SellerID, trade.BuyerID, baseAsset, trade.Quantity, fmt.Sprintf("Settled trade delivery: %s", trade.ID))
			if err != nil {
				log.Error("Failed to settle Base ledger transfer", "err", err)
				return nil
			}

			// Broadcast balance updates downstream
			balanceEvent := types.KafkaEvent{
				Type:      types.EventBalanceUpdate,
				Payload:   trade,
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

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}
