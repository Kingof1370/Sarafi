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

	"velyxora/packages/address"
	"velyxora/packages/assets"
	"velyxora/packages/blockchain"
	"velyxora/packages/wallet"
)

// PersistentWalletService coordinates database, memory, and Kafka publishing representations
type PersistentWalletService struct {
	mu            sync.Mutex
	db            *database.DB
	producer      *common.KafkaProducer
	walletMgr     *wallet.WalletManager
	assetReg      *assets.AssetRegistry
	addrReg       *address.AddressRegistry
	blockchainReg *blockchain.AdapterRegistry
	log           *logger.Logger
}

func NewPersistentWalletService(db *database.DB, producer *common.KafkaProducer, log *logger.Logger) *PersistentWalletService {
	return &PersistentWalletService{
		db:            db,
		producer:      producer,
		walletMgr:     wallet.NewWalletManager(),
		assetReg:      assets.NewAssetRegistry(),
		addrReg:       address.NewAddressRegistry(),
		blockchainReg: blockchain.NewAdapterRegistry(),
		log:           log,
	}
}

// Bootstrap loads initial assets and synchronizes wallets from PostgreSQL
func (p *PersistentWalletService) Bootstrap(ctx context.Context) error {
	p.log.Info("Bootstrapping Persistent Wallet Service configurations...")

	// 1. Register Default Assets
	defaultAssets := []*assets.Asset{
		{Symbol: "BTC", Name: "Bitcoin", Type: assets.TypeNativeCoin, Precision: 8, BaseNetwork: "Bitcoin", IsActive: true},
		{Symbol: "ETH", Name: "Ethereum", Type: assets.TypeNativeCoin, Precision: 18, BaseNetwork: "Ethereum", IsActive: true},
		{Symbol: "USDT", Name: "Tether", Type: assets.TypeStablecoin, Precision: 6, BaseNetwork: "Ethereum", IsActive: true},
		{Symbol: "USDC", Name: "USD Coin", Type: assets.TypeStablecoin, Precision: 6, BaseNetwork: "Ethereum", IsActive: true},
		{Symbol: "BNB", Name: "BNB Coin", Type: assets.TypeNativeCoin, Precision: 18, BaseNetwork: "BNB Smart Chain", IsActive: true},
		{Symbol: "SOL", Name: "Solana", Type: assets.TypeNativeCoin, Precision: 9, BaseNetwork: "Solana", IsActive: true},
		{Symbol: "TRX", Name: "Tron TRX", Type: assets.TypeNativeCoin, Precision: 6, BaseNetwork: "Tron", IsActive: true},
		{Symbol: "LTC", Name: "Litecoin", Type: assets.TypeNativeCoin, Precision: 8, BaseNetwork: "Litecoin", IsActive: true},
	}

	for _, a := range defaultAssets {
		_ = p.assetReg.RegisterAsset(a, nil, nil)
		if p.db != nil {
			// Save to SQL assets_registry
			_, _ = p.db.Pool.Exec(ctx,
				"INSERT INTO assets_registry (symbol, name, type, precision, base_network, is_active) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (symbol) DO UPDATE SET name = EXCLUDED.name, precision = EXCLUDED.precision",
				a.Symbol, a.Name, string(a.Type), a.Precision, a.BaseNetwork, a.IsActive)
		}

		// Publish Kafka event for asset registration
		if p.producer != nil {
			event := types.AssetRegisteredEvent{
				Version:     "v1",
				Symbol:      a.Symbol,
				Name:        a.Name,
				Type:        string(a.Type),
				Precision:   a.Precision,
				BaseNetwork: a.BaseNetwork,
				Timestamp:   time.Now(),
			}
			_ = p.producer.Publish(ctx, "velyxora-asset-registered", a.Symbol, event)
		}
	}

	// 2. Load existing wallets from Database if available
	if p.db != nil {
		rows, err := p.db.Pool.Query(ctx, "SELECT id, user_id, type, is_locked FROM wallets")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id, userID, wType string
				var isLocked bool
				if err := rows.Scan(&id, &userID, &wType, &isLocked); err == nil {
					w, errProv := p.walletMgr.ProvisionWallet(id, userID, wallet.WalletType(wType))
					if errProv == nil {
						w.IsLocked = isLocked
					}
				}
			}
		}

		// Load existing balances
		balRows, err := p.db.Pool.Query(ctx, "SELECT wallet_id, asset, available, locked, reserved, pending, total FROM wallet_balances")
		if err == nil {
			defer balRows.Close()
			for balRows.Next() {
				var wID, asset string
				var avail, lock, res, pend, tot float64
				if err := balRows.Scan(&wID, &asset, &avail, &lock, &res, &pend, &tot); err == nil {
					_ = p.walletMgr.UpdateBalance(wID, asset, avail, lock, res, pend)
				}
			}
		}
	}

	p.log.Info("Bootstrap phase complete. Wallet registries ready.")
	return nil
}

// ProvisionWallet handles both DB persistence, state allocation, and Kafka notifications
func (p *PersistentWalletService) ProvisionWallet(ctx context.Context, walletID, userID string, wType wallet.WalletType) (*wallet.Wallet, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	w, err := p.walletMgr.ProvisionWallet(walletID, userID, wType)
	if err != nil {
		return nil, err
	}

	if p.db != nil {
		_, err := p.db.Pool.Exec(ctx,
			"INSERT INTO wallets (id, user_id, type, is_locked, created_at, updated_at) VALUES ($1, $2, $3, $4, NOW(), NOW()) ON CONFLICT (id) DO NOTHING",
			walletID, userID, string(wType), false)
		if err != nil {
			p.log.Error("Failed to persist wallet insertion", "err", err)
		}
	}

	// Publish Kafka event
	if p.producer != nil {
		event := types.WalletCreatedEvent{
			Version:   "v1",
			WalletID:  walletID,
			UserID:    userID,
			Type:      string(wType),
			Timestamp: time.Now(),
		}
		_ = p.producer.Publish(ctx, "velyxora-wallet-created", walletID, event)
	}

	// Persist audits
	p.persistAudits(ctx)

	return w, nil
}

// AllocateAddress generates an address record via deterministic path and registers it
func (p *PersistentWalletService) AllocateAddress(ctx context.Context, userID, walletID, network string) (*address.AddressRecord, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	w, err := p.walletMgr.GetWallet(walletID)
	if err != nil {
		return nil, err
	}

	adapter, err := p.blockchainReg.Get(network)
	if err != nil {
		return nil, err
	}

	// Generate a deterministic key using key index based on address count
	keyIndex := uint32(len(w.Addresses))
	path := fmt.Sprintf("%s/%d", adapter.DerivationPath(), keyIndex)

	// Build a deterministic seed from the UserID + keyIndex
	seedBytes := make([]byte, 32)
	copy(seedBytes, []byte(fmt.Sprintf("%s-%s-%d", userID, network, keyIndex)))

	addressString, err := adapter.GenerateAddress(seedBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate address: %w", err)
	}

	pubKey := adapter.DerivePublicKey(seedBytes)

	rec := &address.AddressRecord{
		UserID:         userID,
		Network:        network,
		Address:        addressString,
		PublicKey:      pubKey,
		DerivationPath: path,
		Status:         address.StatusAllocated,
		CreatedAt:      time.Now(),
	}

	meta := &address.AddressMetadata{
		Address:          addressString,
		KeyIndex:         keyIndex,
		IsChange:         false,
		TransactionCount: 0,
	}

	err = p.addrReg.RegisterAddress(rec, meta)
	if err != nil {
		return nil, err
	}

	w.Addresses = append(w.Addresses, rec)

	if p.db != nil {
		_, err := p.db.Pool.Exec(ctx,
			"INSERT INTO wallet_addresses (address, user_id, network, public_key, derivation_path, memo, status, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW()) ON CONFLICT (address) DO NOTHING",
			addressString, userID, network, pubKey, path, "", string(address.StatusAllocated))
		if err != nil {
			p.log.Error("Failed to persist allocated address to database", "err", err)
		}
	}

	// Publish Kafka event
	if p.producer != nil {
		event := types.AddressGeneratedEvent{
			Version:        "v1",
			UserID:         userID,
			Network:        network,
			Address:        addressString,
			DerivationPath: path,
			Timestamp:      time.Now(),
		}
		_ = p.producer.Publish(ctx, "velyxora-address-generated", addressString, event)
	}

	p.persistAudits(ctx)

	return rec, nil
}

// UpdateBalance applies deltas, persists balance configurations, and emits Kafka events
func (p *PersistentWalletService) UpdateBalance(ctx context.Context, walletID string, asset string, availableDelta, lockedDelta, reservedDelta, pendingDelta float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	err := p.walletMgr.UpdateBalance(walletID, asset, availableDelta, lockedDelta, reservedDelta, pendingDelta)
	if err != nil {
		return err
	}

	w, _ := p.walletMgr.GetWallet(walletID)
	bal := w.Balances[asset]

	if p.db != nil {
		// Save to wallet_balances
		_, err := p.db.Pool.Exec(ctx,
			`INSERT INTO wallet_balances (wallet_id, asset, available, locked, reserved, pending, total, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			 ON CONFLICT (wallet_id, asset) DO UPDATE
			 SET available = EXCLUDED.available, locked = EXCLUDED.locked, reserved = EXCLUDED.reserved, pending = EXCLUDED.pending, total = EXCLUDED.total, updated_at = NOW()`,
			walletID, asset, bal.Available, bal.Locked, bal.Reserved, bal.Pending, bal.Total)
		if err != nil {
			p.log.Error("Failed to update wallet balances in DB", "err", err)
		}

		// Insert balance history entry
		histID := fmt.Sprintf("hist_%d", time.Now().UnixNano())
		_, errHist := p.db.Pool.Exec(ctx,
			"INSERT INTO wallet_balance_history (id, wallet_id, asset, available, locked, reserved, pending, total, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())",
			histID, walletID, asset, bal.Available, bal.Locked, bal.Reserved, bal.Pending, bal.Total)
		if errHist != nil {
			p.log.Error("Failed to write wallet balance history log", "err", errHist)
		}
	}

	// Publish Kafka balance update event
	if p.producer != nil {
		event := types.BalanceUpdatedEvent{
			Version:   "v1",
			WalletID:  walletID,
			Asset:     asset,
			Available: bal.Available,
			Locked:    bal.Locked,
			Reserved:  bal.Reserved,
			Pending:   bal.Pending,
			Total:     bal.Total,
			Timestamp: time.Now(),
		}
		_ = p.producer.Publish(ctx, "velyxora-balance-updated", walletID, event)
	}

	p.persistAudits(ctx)

	return nil
}

// ValidateWallet performs core checks and publishes a validation report
func (p *PersistentWalletService) ValidateWallet(ctx context.Context, walletID string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	w, err := p.walletMgr.GetWallet(walletID)
	if err != nil {
		return false, err
	}

	isValid := true
	errCount := 0
	for _, bal := range w.Balances {
		sum := bal.Available + bal.Locked + bal.Reserved + bal.Pending
		if absDiff(bal.Total, sum) > 1e-9 {
			isValid = false
			errCount++
		}
	}

	// Publish Kafka event
	if p.producer != nil {
		event := types.WalletValidatedEvent{
			Version:    "v1",
			WalletID:   walletID,
			IsValid:    isValid,
			ErrorCount: errCount,
			Timestamp:  time.Now(),
		}
		_ = p.producer.Publish(ctx, "velyxora-wallet-validated", walletID, event)
	}

	return isValid, nil
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

// persistAudits scans new audit logs, inserts them into PostgreSQL, and publishes to Kafka
func (p *PersistentWalletService) persistAudits(ctx context.Context) {
	logs := p.walletMgr.ListAuditLogs()
	if len(logs) == 0 {
		return
	}

	// Pick the last record
	rec := logs[len(logs)-1]

	// 1. Database Persistence
	if p.db != nil {
		_, errAudit := p.db.Pool.Exec(ctx,
			`INSERT INTO wallet_audits (id, user_id, wallet_id, asset, action, amount, prev_balance, new_balance, message, prev_hash, hash, timestamp)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ON CONFLICT (id) DO NOTHING`,
			rec.ID, rec.UserID, rec.WalletID, rec.Asset, string(rec.Action), rec.Amount, rec.PrevBalance, rec.NewBalance, rec.Message, rec.PrevHash, rec.Hash, rec.Timestamp)
		if errAudit != nil {
			p.log.Error("Failed to persist audit record to database", "err", errAudit)
		}
	}

	// 2. Kafka Event Notification
	if p.producer != nil {
		event := types.WalletAuditEvent{
			Version:     "v1",
			AuditID:     rec.ID,
			UserID:      rec.UserID,
			WalletID:    rec.WalletID,
			Asset:       rec.Asset,
			Action:      string(rec.Action),
			Amount:      rec.Amount,
			PrevBalance: rec.PrevBalance,
			NewBalance:  rec.NewBalance,
			Hash:        rec.Hash,
			Timestamp:   rec.Timestamp,
		}
		_ = p.producer.Publish(ctx, "velyxora-wallet-audit", rec.ID, event)
	}
}

// ProcessDoubleEntry forces absolute balance matches to ledger debits/credits to enforce financial safety
func (p *PersistentWalletService) ProcessDoubleEntry(ctx context.Context, txID string, debitUser, creditUser string, asset string, amount float64, description string) error {
	// Provision standard hot wallets for these users if they do not exist
	debitWalletID := "wal_hot_" + debitUser
	creditWalletID := "wal_hot_" + creditUser

	_, err := p.walletMgr.GetWallet(debitWalletID)
	if err != nil {
		_, _ = p.ProvisionWallet(ctx, debitWalletID, debitUser, wallet.TypeHot)
		_ = p.UpdateBalance(ctx, debitWalletID, asset, 1000.0, 0, 0, 0) // Onboarding onboarding deposit
	}

	_, err = p.walletMgr.GetWallet(creditWalletID)
	if err != nil {
		_, _ = p.ProvisionWallet(ctx, creditWalletID, creditUser, wallet.TypeHot)
		_ = p.UpdateBalance(ctx, creditWalletID, asset, 1000.0, 0, 0, 0) // Onboarding onboarding deposit
	}

	// Trigger Transfer
	_, errTransfer := p.walletMgr.InitiateTransfer(txID, debitWalletID, creditWalletID, asset, amount, debitUser)
	if errTransfer != nil {
		return errTransfer
	}

	// Persist the state change in database and trigger balance update kafka notifications
	_ = p.UpdateBalance(ctx, debitWalletID, asset, 0, 0, 0, 0)
	_ = p.UpdateBalance(ctx, creditWalletID, asset, 0, 0, 0, 0)

	// Persist transaction details in database ledger if SQL is live
	if p.db != nil {
		tx, err := p.db.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		debitEntryID := fmt.Sprintf("ent_deb_%d", time.Now().UnixNano())
		_, _ = tx.Exec(ctx,
			"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
			debitEntryID, txID, debitUser, asset, string(types.Debit), amount, description)

		creditEntryID := fmt.Sprintf("ent_cred_%d", time.Now().UnixNano())
		_, _ = tx.Exec(ctx,
			"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
			creditEntryID, txID, creditUser, asset, string(types.Credit), amount, description)

		_ = tx.Commit(ctx)
	}

	return nil
}

// Legacy BalanceEngine Wrapper for Main CLI and Kafka Loops compatibility
type BalanceEngine struct {
	pService *PersistentWalletService
	balances map[string]*types.Balance // Legacy reference structure
}

func NewBalanceEngine(db *database.DB) *BalanceEngine {
	log := logger.NewLogger(logger.Config{Level: "INFO", Format: "JSON", ServiceName: "legacy-reconciler"})
	pService := NewPersistentWalletService(db, nil, log)
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

	wDebit, _ := be.pService.walletMgr.GetWallet("wal_hot_" + debitUser)
	wCredit, _ := be.pService.walletMgr.GetWallet("wal_hot_" + creditUser)

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
	pws := NewPersistentWalletService(db, producer, log)
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
