package engine

import (
	"context"
	"os"
	"testing"
	"time"

	"velyxora/packages/database"
	"velyxora/packages/logger"
	"velyxora/packages/types"
)

func TestLiquidityBridge_SimulationAndAggregation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := logger.NewLogger(logger.Config{
		Level:       "DEBUG",
		Format:      "TEXT",
		ServiceName: "test-liquidity-bridge",
	})

	// 1. Initialize simulated bridge
	bridge := NewLiquidityBridge(nil, log, "BTC-USDT", "", true)
	bridge.Start(ctx)
	defer bridge.Stop()

	// Wait for simulation loop to populate initial cache
	time.Sleep(100 * time.Millisecond)

	// Verify status is CONNECTED
	if bridge.GetStatus() != "CONNECTED" {
		t.Errorf("Expected status to be CONNECTED, got %s", bridge.GetStatus())
	}

	// Retrieve order book depth
	book, err := bridge.GetOrderBook("BTC-USDT")
	if err != nil {
		t.Fatalf("Failed to retrieve order book: %v", err)
	}

	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		t.Error("Simulated order book bids or asks are empty")
	}

	// Verify IsAggregated and Source properties
	for _, bid := range book.Bids {
		if !bid.IsAggregated {
			t.Error("Expected simulated bids to be marked as aggregated")
		}
		if bid.Source != "Binance" {
			t.Errorf("Expected simulated bids to have Source 'Binance', got %s", bid.Source)
		}
	}

	// 2. Test Aggregated L2 Depth inside OMSRouter
	sm := NewOMSStateMachine()
	val := NewOMSValidator()
	risk := NewRiskEngine(10.0, 100000.0)
	matcher := NewMatcher("BTC-USDT")
	fees := NewFeesEngine(0.0010, 0.0020)
	exec := NewExecutionEngine(fees, risk)
	settle := NewSettlementEngine(nil)

	router := NewOMSRouter(sm, val, risk, matcher, exec, settle)
	router.SetLiquidityBridge(bridge)

	// Insert local bid and ask orders
	localBuy := &types.Order{
		ID:        "local_buy_1",
		UserID:    "user_1",
		Symbol:    "BTC-USDT",
		Side:      types.SideBuy,
		Type:      types.TypeLimit,
		Price:     49999.0,
		Quantity:  1.0,
		FilledQty: 0.0,
	}
	localSell := &types.Order{
		ID:        "local_sell_1",
		UserID:    "user_2",
		Symbol:    "BTC-USDT",
		Side:      types.SideSell,
		Type:      types.TypeLimit,
		Price:     50001.0,
		Quantity:  1.0,
		FilledQty: 0.0,
	}

	matcher.AddOrderToBookDirect(localBuy)
	matcher.AddOrderToBookDirect(localSell)

	// Fetch aggregated L2 depth
	aggDepth := router.GetAggregatedL2Depth(5)
	if len(aggDepth.Bids) == 0 || len(aggDepth.Asks) == 0 {
		t.Fatal("Aggregated depth levels are empty")
	}

	// First levels should be local or sorted properly
	bestBid := aggDepth.Bids[0]
	if bestBid.Price != 49999.0 {
		t.Errorf("Expected best aggregated bid price to be 49999.0, got %f", bestBid.Price)
	}
	if bestBid.IsAggregated {
		t.Error("Expected local buy order level to NOT be marked as aggregated")
	}

	// Subsequent levels should be external
	foundAggregated := false
	for _, bid := range aggDepth.Bids {
		if bid.IsAggregated {
			foundAggregated = true
			break
		}
	}
	if !foundAggregated {
		t.Error("Did not find aggregated external levels in bids list")
	}
}

func TestLiquidityBridge_DisconnectionAndSREAlerting(t *testing.T) {
	ctx := context.Background()

	log := logger.NewLogger(logger.Config{
		Level:       "DEBUG",
		Format:      "TEXT",
		ServiceName: "test-liquidity-bridge-sre",
	})

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "velyxora"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "super-secure-db-password-123"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "velyxora"
	}

	// Connect to real docker-compose postgres running locally on port 5432 (similar to sre_observability_test.go)
	db, err := database.NewConnectionPool(database.Config{
		Host:     dbHost,
		Port:     5432,
		User:     dbUser,
		Password: dbPassword,
		DBName:   dbName,
		SSLMode:  "disable",
	})

	var dbAvailable bool
	if err == nil {
		dbAvailable = true
		_ = database.RunMigrations(ctx, db)
		_, _ = db.Pool.Exec(ctx, "DELETE FROM sre_alerts WHERE metric_name = 'binance_liquidity_bridge_disconnected'")
		_, _ = db.Pool.Exec(ctx, "DELETE FROM sre_incidents WHERE service = 'binance_liquidity_bridge'")
	}

	// 1. Initialize simulated bridge with DB
	bridge := NewLiquidityBridge(db, log, "BTC-USDT", "", true)

	// Simulate connection loss -> DEGRADED
	bridge.SetStatus(ctx, "DEGRADED")

	if bridge.GetStatus() != "DEGRADED" {
		t.Errorf("Expected status to be DEGRADED, got %s", bridge.GetStatus())
	}

	// Order book should gracefully fail-closed / fallback to empty external books
	_, err = bridge.GetOrderBook("BTC-USDT")
	if err == nil {
		t.Error("Expected GetOrderBook to return error when bridge is DEGRADED")
	}

	if dbAvailable {
		// Verify SRE incident was inserted in Postgres
		var incidentCount int
		qErr := db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM sre_incidents WHERE service = 'binance_liquidity_bridge' AND status = 'OPEN'").Scan(&incidentCount)
		if qErr != nil {
			t.Fatalf("Failed to query open SRE incidents: %v", qErr)
		}
		if incidentCount != 1 {
			t.Errorf("Expected 1 open incident in DB, found %d", incidentCount)
		}

		// Reconnect -> SRE HEALED
		bridge.SetStatus(ctx, "CONNECTED")

		if bridge.GetStatus() != "CONNECTED" {
			t.Errorf("Expected status to be CONNECTED, got %s", bridge.GetStatus())
		}

		// Verify SRE incident was resolved
		var resolvedCount int
		qErr = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM sre_incidents WHERE service = 'binance_liquidity_bridge' AND status = 'RESOLVED'").Scan(&resolvedCount)
		if qErr != nil {
			t.Fatalf("Failed to query resolved SRE incidents: %v", qErr)
		}
		if resolvedCount != 1 {
			t.Errorf("Expected resolved incident in DB, found %d", resolvedCount)
		}

		// Cleanup
		_, _ = db.Pool.Exec(ctx, "DELETE FROM sre_alerts WHERE metric_name = 'binance_liquidity_bridge_disconnected'")
		_, _ = db.Pool.Exec(ctx, "DELETE FROM sre_incidents WHERE service = 'binance_liquidity_bridge'")
		db.Close()
	}
}

func TestLiquidityBridge_HedgingAndExecution(t *testing.T) {
	ctx := context.Background()

	log := logger.NewLogger(logger.Config{
		Level:       "DEBUG",
		Format:      "TEXT",
		ServiceName: "test-liquidity-bridge-hedging",
	})

	bridge := NewLiquidityBridge(nil, log, "BTC-USDT", "", true)
	bridge.SetStatus(ctx, "CONNECTED")
	// Force deterministic order book for hedging tests
	bridge.updateMockBook(50000.0) // Bids: 49990, 49980... Asks: 50010, 50020...

	sm := NewOMSStateMachine()
	val := NewOMSValidator()
	risk := NewRiskEngine(10.0, 100000.0)
	matcher := NewMatcher("BTC-USDT")
	fees := NewFeesEngine(0.0, 0.0) // Zero fees for simplicity
	exec := NewExecutionEngine(fees, risk)
	settle := NewSettlementEngine(nil)

	router := NewOMSRouter(sm, val, risk, matcher, exec, settle)
	router.SetLiquidityBridge(bridge)

	// Deposit balances for buyer
	buyerID := "buyer_1"
	risk.DepositAsset(buyerID, "USDT", 1000000.0)
	risk.DepositAsset(buyerID, "BTC", 0.0)

	// Place an order that crosses external Ask at 50010.0
	order := &AdvancedOrder{
		ID:        "user_crossing_order_1",
		UserID:    buyerID,
		Symbol:    "BTC-USDT",
		Side:      "BUY",
		Type:      "LIMIT",
		Price:     50015.0, // Crossing price
		Quantity:  1.0,
		Status:    StatusOMS_Created,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := router.ProcessIncomingOrder(ctx, order, "USDT", "BTC", "127.0.0.1", "test-device")
	if err != nil {
		t.Fatalf("Failed to process order: %v", err)
	}

	// Verify order was filled against external book
	retrievedOrder, err := router.GetOrder(order.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve order: %v", err)
	}

	if retrievedOrder.FilledQty != 1.0 {
		t.Errorf("Expected order to be fully filled (1.0), got %f", retrievedOrder.FilledQty)
	}
	if retrievedOrder.Status != StatusOMS_Filled {
		t.Errorf("Expected status to be FILLED, got %s", retrievedOrder.Status)
	}

	// Verify execution records
	trades := router.GetRecentTrades()
	if len(trades) == 0 {
		t.Fatal("No trades generated")
	}

	matchedAgainstExternal := false
	for _, tr := range trades {
		if tr.BuyerID == buyerID && tr.SellerID == "EXT_LIQUIDITY_BINANCE" {
			matchedAgainstExternal = true
			if tr.Price != 50010.0 {
				t.Errorf("Expected match price of 50010.0, got %f", tr.Price)
			}
			if tr.Quantity != 1.0 {
				t.Errorf("Expected match quantity of 1.0, got %f", tr.Quantity)
			}
		}
	}

	if !matchedAgainstExternal {
		t.Error("No trade matching local user against external liquidity provider found")
	}

	// Wait briefly for asynchronous hedging thread to execute
	time.Sleep(100 * time.Millisecond)
}
