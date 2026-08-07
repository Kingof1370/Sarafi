package tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"velyxora/packages/common"
	"velyxora/packages/database"
	"velyxora/packages/types"
)

func TestRealInfrastructureE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Real PostgreSQL Integration Test
	dbConfig := database.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "velyxora",
		Password: "super-secure-db-password-123",
		DBName:   "velyxora",
		SSLMode:  "disable",
	}

	t.Log("Connecting to PostgreSQL at localhost:5432...")
	db, err := database.NewConnectionPool(dbConfig)
	if err != nil {
		t.Fatalf("Failed to connect to real PostgreSQL database in integration test: %v", err)
	}
	defer db.Close()

	t.Log("PostgreSQL connected! Running schema migrations...")
	err = database.RunMigrations(ctx, db)
	if err != nil {
		t.Fatalf("Failed to run schema migrations on real database: %v", err)
	}

	t.Log("Migrations successfully applied! Verifying applied migrations count...")
	var migrationCount int
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount)
	if err != nil {
		t.Fatalf("Failed to query schema_migrations table: %v", err)
	}
	t.Logf("Found %d applied schema migrations in the database.", migrationCount)
	if migrationCount < 71 {
		t.Errorf("Expected at least 71 applied migrations, got %d", migrationCount)
	}

	// 2. Real PostgreSQL SELECT FOR UPDATE Transactional Integrity Test
	t.Log("Executing double-entry balance updates with row locking (SELECT FOR UPDATE)...")
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// Clean up / prep test tables
	_, _ = tx.Exec(ctx, "DELETE FROM balances WHERE user_id IN ('usr_test_buyer', 'usr_test_seller')")
	_, _ = tx.Exec(ctx, "DELETE FROM users WHERE id IN ('usr_test_buyer', 'usr_test_seller')")

	// Insert test users
	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, status, role)
		VALUES
		('usr_test_buyer', 'buyer@velyxora.test', 'hashed', 'ACTIVE', 'USER'),
		('usr_test_seller', 'seller@velyxora.test', 'hashed', 'ACTIVE', 'USER')
	`)
	if err != nil {
		t.Fatalf("Failed to insert test users: %v", err)
	}

	// Insert test balances
	_, err = tx.Exec(ctx, `
		INSERT INTO balances (user_id, asset, available, locked, pending, reserved, total)
		VALUES
		('usr_test_buyer', 'USDT', 10000.0, 0.0, 0.0, 0.0, 10000.0),
		('usr_test_seller', 'BTC', 5.0, 0.0, 0.0, 0.0, 5.0),
		('usr_test_buyer', 'BTC', 0.0, 0.0, 0.0, 0.0, 0.0),
		('usr_test_seller', 'USDT', 0.0, 0.0, 0.0, 0.0, 0.0)
	`)
	if err != nil {
		t.Fatalf("Failed to insert initial test balances: %v", err)
	}

	// Lock the rows using SELECT FOR UPDATE
	var buyerUSDTAvailable float64
	err = tx.QueryRow(ctx, "SELECT available FROM balances WHERE user_id = 'usr_test_buyer' AND asset = 'USDT' FOR UPDATE").Scan(&buyerUSDTAvailable)
	if err != nil {
		t.Fatalf("Failed to lock buyer USDT balance: %v", err)
	}

	var sellerBTCAvailable float64
	err = tx.QueryRow(ctx, "SELECT available FROM balances WHERE user_id = 'usr_test_seller' AND asset = 'BTC' FOR UPDATE").Scan(&sellerBTCAvailable)
	if err != nil {
		t.Fatalf("Failed to lock seller BTC balance: %v", err)
	}

	// Update buyer and seller balances (USDT debit buyer, USDT credit seller)
	price := 60000.0
	qty := 0.1
	cost := price * qty // 6000.0 USDT

	if buyerUSDTAvailable < cost {
		t.Fatalf("Insufficient USDT balance for buyer")
	}

	_, err = tx.Exec(ctx, "UPDATE balances SET available = available - $1, total = total - $1 WHERE user_id = 'usr_test_buyer' AND asset = 'USDT'", cost)
	if err != nil {
		t.Fatalf("Failed to debit buyer: %v", err)
	}
	_, err = tx.Exec(ctx, "UPDATE balances SET available = available + $1, total = total + $1 WHERE user_id = 'usr_test_seller' AND asset = 'USDT'", cost)
	if err != nil {
		t.Fatalf("Failed to credit seller: %v", err)
	}

	// BTC transfer (debit seller, credit buyer)
	_, err = tx.Exec(ctx, "UPDATE balances SET available = available - $1, total = total - $1 WHERE user_id = 'usr_test_seller' AND asset = 'BTC'", qty)
	if err != nil {
		t.Fatalf("Failed to debit seller: %v", err)
	}
	_, err = tx.Exec(ctx, "UPDATE balances SET available = available + $1, total = total + $1 WHERE user_id = 'usr_test_buyer' AND asset = 'BTC'", qty)
	if err != nil {
		t.Fatalf("Failed to credit buyer: %v", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}
	t.Log("Row locking balance transaction committed successfully!")

	// Verify balance changes
	var finalBuyerUSDT, finalSellerUSDT, finalBuyerBTC, finalSellerBTC float64
	err = db.Pool.QueryRow(ctx, "SELECT available FROM balances WHERE user_id = 'usr_test_buyer' AND asset = 'USDT'").Scan(&finalBuyerUSDT)
	if err != nil {
		t.Fatalf("Failed to fetch final buyer USDT: %v", err)
	}
	err = db.Pool.QueryRow(ctx, "SELECT available FROM balances WHERE user_id = 'usr_test_seller' AND asset = 'USDT'").Scan(&finalSellerUSDT)
	if err != nil {
		t.Fatalf("Failed to fetch final seller USDT: %v", err)
	}
	err = db.Pool.QueryRow(ctx, "SELECT available FROM balances WHERE user_id = 'usr_test_buyer' AND asset = 'BTC'").Scan(&finalBuyerBTC)
	if err != nil {
		t.Fatalf("Failed to fetch final buyer BTC: %v", err)
	}
	err = db.Pool.QueryRow(ctx, "SELECT available FROM balances WHERE user_id = 'usr_test_seller' AND asset = 'BTC'").Scan(&finalSellerBTC)
	if err != nil {
		t.Fatalf("Failed to fetch final seller BTC: %v", err)
	}

	if finalBuyerUSDT != 4000.0 || finalSellerUSDT != 6000.0 {
		t.Errorf("USDT balances are incorrect: buyer=%f, seller=%f", finalBuyerUSDT, finalSellerUSDT)
	}
	if finalBuyerBTC != 0.1 || finalSellerBTC != 4.9 {
		t.Errorf("BTC balances are incorrect: buyer=%f, seller=%f", finalBuyerBTC, finalSellerBTC)
	}

	// 3. Real Redis Integration Test
	t.Log("Connecting to Redis at localhost:6379...")
	redisClient, err := common.NewRedisClient(common.RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	if err != nil {
		t.Fatalf("Failed to connect to real Redis: %v", err)
	}

	t.Log("Setting cache key...")
	testKey := "velyxora:test_token_cache:1"
	testValue := "session_valid_data"
	err = redisClient.Set(ctx, testKey, testValue, 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to Set key in Redis: %v", err)
	}

	t.Log("Getting cache key...")
	fetchedValue, err := redisClient.Get(ctx, testKey)
	if err != nil {
		t.Fatalf("Failed to Get key from Redis: %v", err)
	}
	if fetchedValue != testValue {
		t.Errorf("Fetched cache value did not match. Expected %q, got %q", testValue, fetchedValue)
	}

	t.Log("Deleting cache key...")
	err = redisClient.Del(ctx, testKey)
	if err != nil {
		t.Fatalf("Failed to Del key from Redis: %v", err)
	}

	// 4. Real Kafka Integration Test
	t.Log("Connecting to Kafka at localhost:9092...")
	brokers := []string{"localhost:9092"}
	kafkaProducer := common.NewKafkaProducer(brokers)
	defer kafkaProducer.Close()

	topicName := "velyxora-orders-test-topic"
	testPayload := types.Order{
		ID:       "ord_test_kafka_999",
		UserID:   "usr_test_buyer",
		Symbol:   "BTC-USDT",
		Side:     types.SideBuy,
		Type:     types.TypeLimit,
		Price:    60000.0,
		Quantity: 1.5,
		Status:   types.StatusNew,
	}

	testEvent := types.KafkaEvent{
		Type:      types.EventOrderCreated,
		Payload:   testPayload,
		Timestamp: time.Now(),
	}

	t.Log("Publishing message to Kafka...")
	err = kafkaProducer.Publish(ctx, topicName, "ord_test_kafka_999", testEvent)
	if err != nil {
		t.Fatalf("Failed to publish message to Kafka: %v", err)
	}

	t.Log("Consuming published message from Kafka...")
	kafkaConsumer := common.NewKafkaConsumer(brokers, topicName, "test-integration-group")
	defer kafkaConsumer.Close()

	consumerCtx, consumerCancel := context.WithTimeout(ctx, 15*time.Second)
	defer consumerCancel()

	var consumedEvent types.KafkaEvent
	var consumeSuccess bool

	err = kafkaConsumer.Consume(consumerCtx, func(key string, value []byte) error {
		t.Logf("Received message from Kafka with key: %s", key)
		if key != "ord_test_kafka_999" {
			t.Errorf("Expected key 'ord_test_kafka_999', got '%s'", key)
		}

		err := json.Unmarshal(value, &consumedEvent)
		if err != nil {
			t.Errorf("Failed to unmarshal consumed message: %v", err)
			return err
		}

		if consumedEvent.Type == types.EventOrderCreated {
			consumeSuccess = true
			consumerCancel() // Stop consumer once successfully verified
		}
		return nil
	})

	if err != nil && err != context.Canceled && !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("Kafka consume failed: %v", err)
	}

	if !consumeSuccess {
		t.Errorf("Failed to successfully consume published message from Kafka")
	} else {
		t.Log("Kafka Pub/Sub transaction verified successfully!")
	}
}
