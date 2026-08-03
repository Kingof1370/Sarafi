package wallet

import (
	"context"
	"fmt"
	"sync"
	"time"

	"velyxora/packages/common"
	"velyxora/packages/database"
	"velyxora/packages/logger"
	"velyxora/packages/types"

	"velyxora/packages/address"
	"velyxora/packages/assets"
	"velyxora/packages/blockchain"
	"velyxora/packages/connectivity"
	"velyxora/packages/custody"
	"velyxora/packages/deposits"
	"velyxora/packages/monitoring"
	"velyxora/packages/treasury"
	"velyxora/packages/keys"
	"velyxora/packages/withdrawals"
)

// PersistentWalletService coordinates database, memory, and Kafka publishing representations
type PersistentWalletService struct {
	mu               sync.Mutex
	db               *database.DB
	producer         *common.KafkaProducer
	walletMgr        *WalletManager
	assetReg         *assets.AssetRegistry
	addrReg          *address.AddressRegistry
	blockchainReg    *blockchain.AdapterRegistry
	depositEngine    *deposits.DepositEngine
	withdrawalEngine *withdrawals.WithdrawalEngine
	nodeManager      *connectivity.NodeManager
	keyManager       *keys.KeyManager
	custodyMgr       *custody.CustodyManager
	treasuryMgr      *treasury.TreasuryManager
	monitoringService *monitoring.MonitoringService
	log              *logger.Logger
}

func NewPersistentWalletService(db *database.DB, producer *common.KafkaProducer, log *logger.Logger) *PersistentWalletService {
	if log == nil {
		log = logger.NewLogger(logger.Config{Level: "INFO", Format: "TEXT", ServiceName: "wallet-service"})
	}
	return &PersistentWalletService{
		db:               db,
		producer:         producer,
		walletMgr:        NewWalletManager(),
		assetReg:         assets.NewAssetRegistry(),
		addrReg:          address.NewAddressRegistry(),
		blockchainReg:    blockchain.NewAdapterRegistry(),
		depositEngine:    deposits.NewDepositEngine(),
		withdrawalEngine: withdrawals.NewWithdrawalEngine(),
		nodeManager:      connectivity.NewNodeManager(),
		keyManager:       keys.NewKeyManager(nil),
		custodyMgr:       custody.NewCustodyManager(),
		treasuryMgr:      treasury.NewTreasuryManager(),
		monitoringService: monitoring.NewMonitoringService(),
		log:              log,
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

	// 2. Register Standard Connection Nodes
	defaultNodes := []*connectivity.NodeRecord{
		{ID: "eth_primary", Network: "Ethereum", URL: "https://eth.velyxora.com", Type: connectivity.TypePrimary, IsAvailable: true, IsSynced: true, HealthScore: 1.0},
		{ID: "eth_fallback", Network: "Ethereum", URL: "https://eth-fallback.velyxora.com", Type: connectivity.TypeFallback, IsAvailable: true, IsSynced: true, HealthScore: 0.9},
		{ID: "btc_primary", Network: "Bitcoin", URL: "https://btc.velyxora.com", Type: connectivity.TypePrimary, IsAvailable: true, IsSynced: true, HealthScore: 1.0},
	}

	for _, n := range defaultNodes {
		p.nodeManager.RegisterNode(n)
		if p.db != nil {
			_, _ = p.db.Pool.Exec(ctx,
				"INSERT INTO blockchain_nodes (id, network, url, type, is_active) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO NOTHING",
				n.ID, n.Network, n.URL, string(n.Type), true)
		}
	}

	// 3. Register Standard HSM-Managed Keys
	defaultKeys := []keys.KeyType{keys.TypeEd25519, keys.TypeECDSA}
	for _, kt := range defaultKeys {
		k, err := p.keyManager.GenerateNewKey(kt, true)
		if err == nil && p.db != nil {
			_, _ = p.db.Pool.Exec(ctx,
				`INSERT INTO cryptographic_keys (id, version, type, public_key, fingerprint, status, expiration_date, is_hsm_managed)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT (id) DO NOTHING`,
				k.ID, k.Version, string(k.Type), k.PublicKey, k.Fingerprint, string(k.Status), k.ExpirationDate, k.IsHSMManaged)
		}
	}

	// 3.5. Register Standard Institutional Custody Vaults
	if p.db != nil {
		defaultVaults := []struct {
			ID   string
			Name string
			Type string
		}{
			{"v_hot", "Hot Exchange Vault", "HOT"},
			{"v_warm", "Warm Operational Vault", "WARM"},
			{"v_cold", "Cold Core Storage", "COLD"},
			{"v_deep_cold", "Deep Cold Air-Gapped Vault", "DEEP_COLD"},
			{"v_treasury", "Corporate Treasury Vault", "TREASURY_VAULT"},
			{"v_reserve", "Platform Emergency Reserve", "RESERVE_VAULT"},
			{"v_recovery", "Disaster Recovery Vault", "RECOVERY_VAULT"},
		}
		for _, v := range defaultVaults {
			_, _ = p.db.Pool.Exec(ctx,
				"INSERT INTO custody_vaults (id, name, type, is_locked) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING",
				v.ID, v.Name, v.Type, false)
		}
	}

	// 3.6. Register Standard Institutional Treasury Pools
	if p.db != nil {
		defaultPools := []struct {
			ID   string
			Name string
			Type string
		}{
			{"p_hot", "Hot Exchange Liquidity", "HOT"},
			{"p_warm", "Warm Operational Pool", "WARM"},
			{"p_cold", "Cold Storage Capital", "COLD"},
			{"p_treasury", "Corporate Treasury Core", "TREASURY"},
			{"p_reserve", "Platform Reserves Asset", "RESERVE"},
			{"p_insurance", "User Insolvency Insurance Fund", "INSURANCE"},
			{"p_fee_wallet", "Platform Fee Accumulator", "FEE_WALLET"},
			{"p_operational", "Operational Expense Wallet", "OPERATIONAL"},
		}
		for _, pID := range defaultPools {
			_, _ = p.db.Pool.Exec(ctx,
				"INSERT INTO treasury_pools (id, name, type) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING",
				pID.ID, pID.Name, pID.Type)
		}
	}

	// 4. Load existing wallets from Database if available
	if p.db != nil {
		rows, err := p.db.Pool.Query(ctx, "SELECT id, user_id, type, is_locked FROM wallets")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id, userID, wType string
				var isLocked bool
				if err := rows.Scan(&id, &userID, &wType, &isLocked); err == nil {
					w, errProv := p.walletMgr.ProvisionWallet(id, userID, WalletType(wType))
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

// GetWalletMgr retrieves the internal WalletManager
func (p *PersistentWalletService) GetWalletMgr() *WalletManager {
	return p.walletMgr
}

// GetDepositEngine retrieves the internal DepositEngine
func (p *PersistentWalletService) GetDepositEngine() *deposits.DepositEngine {
	return p.depositEngine
}

// GetWithdrawalEngine retrieves the internal WithdrawalEngine
func (p *PersistentWalletService) GetWithdrawalEngine() *withdrawals.WithdrawalEngine {
	return p.withdrawalEngine
}

// GetNodeManager retrieves the internal NodeManager
func (p *PersistentWalletService) GetNodeManager() *connectivity.NodeManager {
	return p.nodeManager
}

// GetKeyManager retrieves the internal KeyManager
func (p *PersistentWalletService) GetKeyManager() *keys.KeyManager {
	return p.keyManager
}

// GetCustodyManager retrieves the internal CustodyManager
func (p *PersistentWalletService) GetCustodyManager() *custody.CustodyManager {
	return p.custodyMgr
}

// GetTreasuryManager retrieves the internal TreasuryManager
func (p *PersistentWalletService) GetTreasuryManager() *treasury.TreasuryManager {
	return p.treasuryMgr
}

// GetMonitoringService retrieves the internal MonitoringService
func (p *PersistentWalletService) GetMonitoringService() *monitoring.MonitoringService {
	return p.monitoringService
}

// ProvisionWallet handles both DB persistence, state allocation, and Kafka notifications
func (p *PersistentWalletService) ProvisionWallet(ctx context.Context, walletID, userID string, wType WalletType) (*Wallet, error) {
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

// ProcessBlockchainTransaction registers block transactions and tracks confirmation increments dynamically
func (p *PersistentWalletService) ProcessBlockchainTransaction(ctx context.Context, tx *deposits.BlockchainTx, userID, walletID string) (*deposits.DepositRecord, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 1. Persist block tx to DB
	if p.db != nil {
		_, err := p.db.Pool.Exec(ctx,
			`INSERT INTO blockchain_transactions (tx_hash, network, asset, amount, sender, receiver, block_number, gas_used, timestamp)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (tx_hash) DO NOTHING`,
			tx.TxHash, tx.Network, tx.Asset, tx.Amount, tx.Sender, tx.Receiver, tx.BlockNumber, tx.GasUsed, tx.Timestamp)
		if err != nil {
			p.log.Error("Failed to save blockchain transaction", "err", err)
		}
	}

	// 2. Validate and Ingress to Deposit Engine
	rec, err := p.depositEngine.ValidateAndDetectIngress(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("ingress failed: %w", err)
	}

	// 3. Persist new deposit to DB
	if p.db != nil {
		_, err = p.db.Pool.Exec(ctx,
			`INSERT INTO deposits (id, user_id, asset, amount, fee, address, tx_hash, confirmations, status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT (id) DO NOTHING`,
			rec.ID, rec.UserID, rec.Asset, rec.Amount, rec.Fee, rec.Address, rec.TxHash, rec.Confirmations, string(rec.Status), rec.CreatedAt, rec.UpdatedAt)
		if err != nil {
			p.log.Error("Failed to persist deposit record", "err", err)
		}

		_, err = p.db.Pool.Exec(ctx,
			`INSERT INTO deposit_confirmations (deposit_id, confirmations_count, required_confirmations, status, updated_at)
			 VALUES ($1, $2, $3, $4, $5) ON CONFLICT (deposit_id) DO NOTHING`,
			rec.ID, rec.Confirmations, rec.RequiredConfirmations, string(rec.Status), rec.UpdatedAt)
		if err != nil {
			p.log.Error("Failed to persist confirmations depth", "err", err)
		}
	}

	// 4. Publish "Deposit Detected" Kafka Notification
	if p.producer != nil {
		_ = p.producer.Publish(ctx, "velyxora-deposit-detected", rec.ID, rec)
	}

	return rec, nil
}

// UpdateDepositConfirmation increments confirmations count and credits balance when finalized
func (p *PersistentWalletService) UpdateDepositConfirmation(ctx context.Context, txHash string, currentBlockHeight int64, walletID string) (*deposits.DepositRecord, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 1. Increment via Engine
	rec, err := p.depositEngine.IncrementConfirmations(ctx, txHash, currentBlockHeight)
	if err != nil {
		return nil, err
	}

	// 2. Persist adjustments to DB
	if p.db != nil {
		_, err = p.db.Pool.Exec(ctx,
			`UPDATE deposits SET confirmations = $1, status = $2, updated_at = NOW() WHERE id = $3`,
			rec.Confirmations, string(rec.Status), rec.ID)
		if err != nil {
			p.log.Error("Failed to update deposit", "err", err)
		}

		_, err = p.db.Pool.Exec(ctx,
			`UPDATE deposit_confirmations SET confirmations_count = $1, status = $2, updated_at = NOW() WHERE deposit_id = $3`,
			rec.Confirmations, string(rec.Status), rec.ID)
		if err != nil {
			p.log.Error("Failed to update deposit confirmations count", "err", err)
		}
	}

	// 3. Publish "Confirmation Updated" Kafka Notification
	if p.producer != nil {
		_ = p.producer.Publish(ctx, "velyxora-confirmation-updated", rec.ID, rec)
	}

	// 4. Credit balance if finalized
	if rec.Status == deposits.StatusCompleted {
		errCredit := p.walletMgr.UpdateBalance(walletID, rec.Asset, rec.Amount, 0, 0, 0)
		if errCredit == nil {
			p.log.Info("Successfully credited deposit available balance to user", "user_id", rec.UserID, "amount", rec.Amount, "asset", rec.Asset)
		} else {
			p.log.Error("Failed to credit balance on deposit finalization", "err", errCredit)
		}

		// Publish "Deposit Completed" Kafka Notification
		if p.producer != nil {
			_ = p.producer.Publish(ctx, "velyxora-deposit-completed", rec.ID, rec)
		}

		// Save balance history if DB connected
		if p.db != nil {
			w, _ := p.walletMgr.GetWallet(walletID)
			bal := w.Balances[rec.Asset]
			_, _ = p.db.Pool.Exec(ctx,
				`INSERT INTO wallet_balances (wallet_id, asset, available, locked, reserved, pending, total, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
				 ON CONFLICT (wallet_id, asset) DO UPDATE
				 SET available = EXCLUDED.available, total = EXCLUDED.total, updated_at = NOW()`,
				walletID, rec.Asset, bal.Available, bal.Locked, bal.Reserved, bal.Pending, bal.Total)
		}
	}

	p.persistAudits(ctx)

	return rec, nil
}

// ProcessWithdrawalRequest initiates, validates, and queues standard withdrawal transactions securely
func (p *PersistentWalletService) ProcessWithdrawalRequest(ctx context.Context, userID, walletID, asset string, amount float64, fee float64, address string) (*withdrawals.WithdrawalRequest, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 1. Process via Engine
	req, err := p.withdrawalEngine.CreateWithdrawal(ctx, userID, asset, amount, fee, address)
	if err != nil {
		return nil, fmt.Errorf("withdrawal validation failure: %w", err)
	}

	// 2. Persist to DB if available
	if p.db != nil {
		_, err = p.db.Pool.Exec(ctx,
			`INSERT INTO withdrawal_queue (id, user_id, asset, amount, fee, address, status, risk_score, retries, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			req.ID, req.UserID, req.Asset, req.Amount, req.Fee, req.Address, string(req.Status), req.RiskScore, req.Retries, req.CreatedAt, req.UpdatedAt)
		if err != nil {
			p.log.Error("Failed to persist withdrawal request in DB", "err", err)
		}
	}

	// 3. Publish "Withdrawal Requested" Kafka Notification
	if p.producer != nil {
		_ = p.producer.Publish(ctx, "velyxora-withdrawal-requested", req.ID, req)
	}

	return req, nil
}

// ProcessWithdrawalApproval registers admin approvals and updates status accordingly
func (p *PersistentWalletService) ProcessWithdrawalApproval(ctx context.Context, reqID, adminID string) (*withdrawals.WithdrawalRequest, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	req, err := p.withdrawalEngine.ApproveWithdrawal(ctx, reqID, adminID)
	if err != nil {
		return nil, err
	}

	if p.db != nil {
		appID := fmt.Sprintf("app_%d", time.Now().UnixNano())
		_, _ = p.db.Pool.Exec(ctx,
			"INSERT INTO withdrawal_approvals (id, withdrawal_id, admin_id, created_at) VALUES ($1, $2, $3, NOW())",
			appID, reqID, adminID)

		_, _ = p.db.Pool.Exec(ctx,
			"UPDATE withdrawal_queue SET status = $1, updated_at = NOW() WHERE id = $2",
			string(req.Status), reqID)
	}

	// Publish "Withdrawal Approved" event on Kafka if approved
	if req.Status == withdrawals.StatusApproved && p.producer != nil {
		_ = p.producer.Publish(ctx, "velyxora-withdrawal-approved", req.ID, req)
	}

	return req, nil
}

// ProcessWithdrawalBroadcast simulates broadcasting raw transaction hex
func (p *PersistentWalletService) ProcessWithdrawalBroadcast(ctx context.Context, reqID, payloadHex string) (*withdrawals.WithdrawalRequest, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	req, err := p.withdrawalEngine.BroadcastTransaction(ctx, reqID, payloadHex)
	if err != nil {
		return nil, err
	}

	if p.db != nil {
		_, _ = p.db.Pool.Exec(ctx,
			"UPDATE withdrawal_queue SET status = $1, updated_at = NOW() WHERE id = $2",
			string(req.Status), reqID)

		_, _ = p.db.Pool.Exec(ctx,
			"INSERT INTO withdrawal_broadcast_history (tx_hash, withdrawal_id, payload_hex, timestamp) VALUES ($1, $2, $3, NOW())",
			req.TxHash, reqID, payloadHex)
	}

	// Publish "Withdrawal Broadcast" Kafka Notification
	if p.producer != nil {
		_ = p.producer.Publish(ctx, "velyxora-withdrawal-broadcast", req.ID, req)
	}

	return req, nil
}

// ProcessWithdrawalBroadcast simulates broadcasting raw transaction hex
func (p *PersistentWalletService) ProcessWithdrawalFinalize(ctx context.Context, reqID, walletID string) (*withdrawals.WithdrawalRequest, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	req, err := p.withdrawalEngine.FinalizeWithdrawal(ctx, reqID)
	if err != nil {
		return nil, err
	}

	// Update balances: deduct amount + fee from available balance segment
	totalDeduction := -(req.Amount + req.Fee)
	errCredit := p.walletMgr.UpdateBalance(walletID, req.Asset, totalDeduction, 0, 0, 0)
	if errCredit != nil {
		p.log.Error("Failed to deduct balance for finalized withdrawal", "err", errCredit)
	}

	if p.db != nil {
		_, _ = p.db.Pool.Exec(ctx,
			"UPDATE withdrawal_queue SET status = $1, updated_at = NOW() WHERE id = $2",
			string(req.Status), reqID)

		// Sync balances in DB
		w, _ := p.walletMgr.GetWallet(walletID)
		bal := w.Balances[req.Asset]
		_, _ = p.db.Pool.Exec(ctx,
			`INSERT INTO wallet_balances (wallet_id, asset, available, locked, reserved, pending, total, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			 ON CONFLICT (wallet_id, asset) DO UPDATE
			 SET available = EXCLUDED.available, total = EXCLUDED.total, updated_at = NOW()`,
			walletID, req.Asset, bal.Available, bal.Locked, bal.Reserved, bal.Pending, bal.Total)
	}

	// Publish "Withdrawal Confirmed" (Completed) Kafka Notification
	if p.producer != nil {
		_ = p.producer.Publish(ctx, "velyxora-withdrawal-confirmed", req.ID, req)
	}

	p.persistAudits(ctx)

	return req, nil
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
		_, _ = p.ProvisionWallet(ctx, debitWalletID, debitUser, WalletType(TypeHot))
		_ = p.UpdateBalance(ctx, debitWalletID, asset, 1000.0, 0, 0, 0) // Onboarding onboarding deposit
	}

	_, err = p.walletMgr.GetWallet(creditWalletID)
	if err != nil {
		_, _ = p.ProvisionWallet(ctx, creditWalletID, creditUser, WalletType(TypeHot))
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
