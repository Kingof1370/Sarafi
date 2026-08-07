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

	"velyxora/packages/wallet"
)

// Legacy BalanceEngine Wrapper for Main CLI and Kafka Loops compatibility
type BalanceEngine struct {
	pService *wallet.PersistentWalletService
	balances map[string]*types.Balance // Legacy reference structure
}

func NewBalanceEngine(db *database.DB) *BalanceEngine {
	log := logger.NewLogger(logger.Config{Level: "INFO", Format: "JSON", ServiceName: "legacy-reconciler"})
	pService := wallet.NewPersistentWalletService(db, nil, log)
	_ = pService.Bootstrap(context.Background())

	return &BalanceEngine{
		pService: pService,
		balances: make(map[string]*types.Balance),
	}
}

func (be *BalanceEngine) ProcessDoubleEntry(ctx context.Context, txID string, debitUser, creditUser string, asset string, amount float64, description string) error {
	err := be.pService.ProcessDoubleEntry(ctx, txID, debitUser, creditUser, asset, amount, description)
	if err != nil {
		return err
	}

	debitKey := debitUser + "_" + asset
	creditKey := creditUser + "_" + asset

	wDebit, _ := be.pService.GetWalletMgr().GetWallet("wal_hot_" + debitUser)
	wCredit, _ := be.pService.GetWalletMgr().GetWallet("wal_hot_" + creditUser)

	be.balances[debitKey] = &types.Balance{
		UserID:    debitUser,
		Asset:     asset,
		Available: wDebit.Balances[asset].Available,
		Total:     wDebit.Balances[asset].Total,
	}

	be.balances[creditKey] = &types.Balance{
		UserID:    creditUser,
		Asset:     asset,
		Available: wCredit.Balances[asset].Available,
		Total:     wCredit.Balances[asset].Total,
	}

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

	if dbErr != nil {
		log.Warn(fmt.Sprintf("Failed to connect to PG pool, operating in Mock local transaction memory state: %v", dbErr))
		db = nil
	} else {
		defer db.Close()
	}

	// Initialize with active Kafka Producer
	pws := wallet.NewPersistentWalletService(db, producer, log)
	_ = pws.Bootstrap(context.Background())

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

	err := consumer.Consume(ctx, func(key string, value []byte) error {
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
			err = pws.ProcessDoubleEntry(ctx, txID, trade.BuyerID, trade.SellerID, quoteAsset, quoteAmount, fmt.Sprintf("Settled trade purchase: %s", trade.ID))
			if err != nil {
				log.Error("Failed to settle Quote ledger transfer", "err", err)
				return nil
			}

			// Settle Seller Debit (BTC) -> Credit Buyer (BTC)
			err = pws.ProcessDoubleEntry(ctx, txID, trade.SellerID, trade.BuyerID, baseAsset, trade.Quantity, fmt.Sprintf("Settled trade delivery: %s", trade.ID))
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
