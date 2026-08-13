package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"velyxora/packages/common"
	"velyxora/packages/database"
	"velyxora/packages/logger"
)

type WalletWorker struct {
	db  *database.DB
	log *logger.Logger
}

func NewWalletWorker(db *database.DB, log *logger.Logger) *WalletWorker {
	return &WalletWorker{
		db:  db,
		log: log,
	}
}

// Start runs the background block scanner and deposit confirmation tracker loops
func (ww *WalletWorker) Start(ctx context.Context) {
	ww.log.Info("Starting WalletWorker deposit scanning service...")

	// Create channels/tickers
	scanTicker := time.NewTicker(10 * time.Second)
	confirmTicker := time.NewTicker(5 * time.Second)

	if os.Getenv("APP_ENV") == "test" || os.Getenv("APP_ENV") == "development" || os.Getenv("APP_ENV") == "simulation" {
		// Fast tickers for local simulation and testing
		scanTicker = time.NewTicker(1 * time.Second)
		confirmTicker = time.NewTicker(500 * time.Millisecond)
	}

	defer scanTicker.Stop()
	defer confirmTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			ww.log.Info("WalletWorker background loops stopped.")
			return
		case <-scanTicker.C:
			if err := ww.ScanBlocks(ctx); err != nil {
				ww.log.Error("WalletWorker Block Scanner error", "err", err)
			}
		case <-confirmTicker.C:
			if err := ww.ConfirmDeposits(ctx); err != nil {
				ww.log.Error("WalletWorker Deposit Confirmation error", "err", err)
			}
		}
	}
}

func (ww *WalletWorker) ScanBlocks(ctx context.Context) error {
	if ww.db == nil {
		return nil
	}

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "production"
	}

	// 1. Resolve RPC node client
	var latestBlock uint64
	var err error

	if appEnv == "test" || appEnv == "development" || appEnv == "simulation" {
		// Simulating latest block increment
		latestBlock, err = ww.getSimulatedLatestBlock(ctx)
	} else {
		ec, errClient := common.NewEthereumClient()
		if errClient != nil {
			return errClient
		}
		latestBlock, err = ec.GetLatestBlockNumber()
	}

	if err != nil {
		return fmt.Errorf("failed to fetch latest block number: %w", err)
	}

	// 2. Fetch last scanned block from database
	var lastScannedBlock int64
	err = ww.db.Pool.QueryRow(ctx,
		"SELECT last_scanned_block FROM blockchain_scanner_states WHERE network = 'ETH'").Scan(&lastScannedBlock)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			// Initialize
			lastScannedBlock = int64(latestBlock) - 1
			if lastScannedBlock < 0 {
				lastScannedBlock = 0
			}
			_, err = ww.db.Pool.Exec(ctx,
				"INSERT INTO blockchain_scanner_states (network, last_scanned_block, updated_at) VALUES ('ETH', $1, NOW())",
				lastScannedBlock)
			if err != nil {
				return fmt.Errorf("failed to initialize scanner state: %w", err)
			}
		} else {
			return fmt.Errorf("failed to fetch scanner state: %w", err)
		}
	}

	nextBlock := lastScannedBlock + 1
	if nextBlock > int64(latestBlock) {
		return nil // All caught up!
	}

	// Limit scan window to max 50 blocks per run to avoid RPC rate limits
	endBlock := int64(latestBlock)
	if endBlock-nextBlock > 50 {
		endBlock = nextBlock + 50
	}

	ww.log.Info(fmt.Sprintf("Scanning blocks %d to %d (latest: %d)", nextBlock, endBlock, latestBlock))

	// 3. Fetch tracked user deposit addresses from wallet_addresses table
	addresses, err := ww.getTrackedAddresses(ctx)
	if err != nil {
		return err
	}
	if len(addresses) == 0 {
		// Update scanned block even if we don't have registered addresses yet
		_, _ = ww.db.Pool.Exec(ctx,
			"UPDATE blockchain_scanner_states SET last_scanned_block = $1, updated_at = NOW() WHERE network = 'ETH'",
			endBlock)
		return nil
	}

	// 4. Perform Scanning
	for b := nextBlock; b <= endBlock; b++ {
		if appEnv == "test" || appEnv == "development" || appEnv == "simulation" {
			err = ww.scanSimulatedBlock(ctx, b, addresses)
		} else {
			err = ww.scanRealBlock(ctx, b, addresses)
		}
		if err != nil {
			return fmt.Errorf("failed scanning block %d: %w", b, err)
		}

		// Update block state
		_, err = ww.db.Pool.Exec(ctx,
			"UPDATE blockchain_scanner_states SET last_scanned_block = $1, updated_at = NOW() WHERE network = 'ETH'",
			b)
		if err != nil {
			return fmt.Errorf("failed to update last scanned block: %w", err)
		}
	}

	return nil
}

func (ww *WalletWorker) getTrackedAddresses(ctx context.Context) (map[string]string, error) {
	// Map address to user_id
	addrMap := make(map[string]string)
	rows, err := ww.db.Pool.Query(ctx, "SELECT address, user_id FROM wallet_addresses WHERE network = 'ETH' OR asset = 'ETH'")
	if err != nil {
		return nil, fmt.Errorf("failed to query wallet addresses: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var addr, userID string
		if err := rows.Scan(&addr, &userID); err != nil {
			return nil, err
		}
		addrMap[strings.ToLower(addr)] = userID
	}
	return addrMap, nil
}

func (ww *WalletWorker) scanRealBlock(ctx context.Context, blockNum int64, addresses map[string]string) error {
	ec, err := common.NewEthereumClient()
	if err != nil {
		return err
	}

	// Fetch full block
	block, err := ec.Client.BlockByNumber(ctx, big.NewInt(blockNum))
	if err != nil {
		return fmt.Errorf("failed to fetch block %d: %w", blockNum, err)
	}

	for _, tx := range block.Transactions() {
		if tx.To() == nil {
			continue
		}
		toAddr := strings.ToLower(tx.To().Hex())
		if userID, exists := addresses[toAddr]; exists {
			txHash := tx.Hash().Hex()

			// Check if deposit record already exists to avoid duplication
			var existingID string
			errCheck := ww.db.Pool.QueryRow(ctx, "SELECT id FROM deposits WHERE tx_hash = $1", txHash).Scan(&existingID)
			if errCheck == nil {
				continue // Deposit already tracked
			}

			// Convert amount from Wei to standard Ether (float64)
			fWei := new(big.Float).SetInt(tx.Value())
			fEth := new(big.Float).Quo(fWei, big.NewFloat(1e18))
			amount, _ := fEth.Float64()

			if amount <= 0 {
				continue // Ignore zero transfers
			}

			depositID := fmt.Sprintf("dep_%s", txHash)
			ww.log.Info("Detected incoming deposit transaction", "tx_hash", txHash, "to", toAddr, "amount", amount, "user_id", userID)

			// Store pending deposit with 0 confirmations and block number
			_, err = ww.db.Pool.Exec(ctx,
				`INSERT INTO deposits (id, user_id, asset, amount, fee, address, tx_hash, confirmations, status, created_at, updated_at)
				 VALUES ($1, $2, 'ETH', $3, 0.0, $4, $5, 0, 'PENDING', NOW(), NOW())`,
				depositID, userID, amount, toAddr, txHash)
			if err != nil {
				ww.log.Error("Failed to store pending deposit record", "err", err)
			}
		}
	}

	return nil
}

func (ww *WalletWorker) scanSimulatedBlock(ctx context.Context, blockNum int64, addresses map[string]string) error {
	// For testing, periodically generate a simulated transaction to one of the registered addresses
	// so the scan and confirmation flow can be fully verified locally.
	var pendingCount int
	_ = ww.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM deposits WHERE status = 'PENDING'").Scan(&pendingCount)

	if pendingCount == 0 {
		// Pick one address
		var targetAddress, targetUserID string
		for addr, uID := range addresses {
			targetAddress = addr
			targetUserID = uID
			break
		}

		if targetAddress != "" {
			txHash := fmt.Sprintf("sim_tx_0x%d", time.Now().UnixNano())
			depositID := "dep_" + txHash
			amount := 1.25 // mock deposit amount

			ww.log.Info("Simulating detected deposit", "tx_hash", txHash, "to", targetAddress, "amount", amount)

			_, err := ww.db.Pool.Exec(ctx,
				`INSERT INTO deposits (id, user_id, asset, amount, fee, address, tx_hash, confirmations, status, created_at, updated_at)
				 VALUES ($1, $2, 'ETH', $3, 0.0, $4, $5, 0, 'PENDING', NOW(), NOW())`,
				depositID, targetUserID, amount, targetAddress, txHash)
			if err != nil {
				ww.log.Error("Failed to store simulated deposit record", "err", err)
			}
		}
	}
	return nil
}

func (ww *WalletWorker) getSimulatedLatestBlock(ctx context.Context) (uint64, error) {
	var lastScannedBlock int64
	err := ww.db.Pool.QueryRow(ctx,
		"SELECT last_scanned_block FROM blockchain_scanner_states WHERE network = 'ETH'").Scan(&lastScannedBlock)
	if err != nil {
		return 1000, nil
	}
	return uint64(lastScannedBlock) + 2, nil
}

func (ww *WalletWorker) ConfirmDeposits(ctx context.Context) error {
	if ww.db == nil {
		return nil
	}

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "production"
	}

	// Retrieve pending and confirming deposits
	rows, err := ww.db.Pool.Query(ctx,
		"SELECT id, user_id, asset, amount, address, tx_hash, confirmations FROM deposits WHERE status = 'PENDING' OR status = 'CONFIRMING'")
	if err != nil {
		return fmt.Errorf("failed to query pending deposits: %w", err)
	}
	defer rows.Close()

	type pendingDep struct {
		ID            string
		UserID        string
		Asset         string
		Amount        float64
		Address       string
		TxHash        string
		Confirmations int
	}

	var pendingList []pendingDep
	for rows.Next() {
		var d pendingDep
		if err := rows.Scan(&d.ID, &d.UserID, &d.Asset, &d.Amount, &d.Address, &d.TxHash, &d.Confirmations); err != nil {
			return err
		}
		pendingList = append(pendingList, d)
	}
	rows.Close()

	if len(pendingList) == 0 {
		return nil
	}

	// Load confirmation threshold
	threshold := 12
	if customConf := os.Getenv("ETH_CONFIRMATIONS_REQUIRED"); customConf != "" {
		if val, errParse := fmt.Sscanf(customConf, "%d", &threshold); errParse == nil && val > 0 {
			// parsed successfully
		}
	}

	for _, d := range pendingList {
		var confirmations int
		var errConfirmations error

		if appEnv == "test" || appEnv == "development" || appEnv == "simulation" {
			confirmations = d.Confirmations + 1
		} else {
			adapter, errAdapter := common.GetBlockchainAdapter(d.Asset)
			if errAdapter == nil {
				confirmations, errConfirmations = adapter.GetConfirmations(d.TxHash)
			} else {
				errConfirmations = errAdapter
			}
		}

		if errConfirmations != nil {
			ww.log.Warn("Failed to query confirmations for deposit", "tx_hash", d.TxHash, "err", errConfirmations)
			continue
		}

		ww.log.Info("Deposit confirmations updated", "tx_hash", d.TxHash, "confirmations", confirmations, "target", threshold)

		if confirmations >= threshold {
			tx, txErr := ww.db.Pool.Begin(ctx)
			if txErr != nil {
				ww.log.Error("Failed to start balance settlement transaction", "err", txErr)
				continue
			}
			defer tx.Rollback(ctx)

			var currentAvail, currentTotal float64
			errQuery := tx.QueryRow(ctx,
				"SELECT available, total FROM balances WHERE user_id = $1 AND asset = $2 FOR UPDATE",
				d.UserID, strings.ToUpper(d.Asset)).Scan(&currentAvail, &currentTotal)

			if errQuery != nil {
				_, err = tx.Exec(ctx,
					"INSERT INTO balances (user_id, asset, available, total, locked, pending, reserved, updated_at) VALUES ($1, $2, $3, $3, 0, 0, 0, NOW())",
					d.UserID, strings.ToUpper(d.Asset), d.Amount)
			} else {
				_, err = tx.Exec(ctx,
					"UPDATE balances SET available = $1, total = $2, updated_at = NOW() WHERE user_id = $3 AND asset = $4",
					currentAvail+d.Amount, currentTotal+d.Amount, d.UserID, strings.ToUpper(d.Asset))
			}
			if err != nil {
				ww.log.Error("Failed to update user balance", "err", err)
				continue
			}

			// Insert Ledger Entry
			_, err = tx.Exec(ctx,
				"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, 'CREDIT', $5, 'Deposit allocation credited via blockchain scanner', NOW())",
				"ent_dep_"+d.ID, d.ID, d.UserID, strings.ToUpper(d.Asset), d.Amount)
			if err != nil {
				ww.log.Error("Failed to insert ledger entry", "err", err)
				continue
			}

			// Update deposit to CONFIRMED
			_, err = tx.Exec(ctx,
				"UPDATE deposits SET confirmations = $1, status = 'CONFIRMED', updated_at = NOW() WHERE id = $2",
				confirmations, d.ID)
			if err != nil {
				ww.log.Error("Failed to update deposit record to CONFIRMED", "err", err)
				continue
			}

			if errCommit := tx.Commit(ctx); errCommit != nil {
				ww.log.Error("Failed to commit balance credit transaction", "err", errCommit)
				continue
			}

			ww.log.Info("Successfully confirmed and credited deposit as CONFIRMED", "deposit_id", d.ID, "amount", d.Amount, "user_id", d.UserID)
		} else {
			// Update confirmation count in deposits table
			_, err = ww.db.Pool.Exec(ctx,
				"UPDATE deposits SET confirmations = $1, status = 'CONFIRMING', updated_at = NOW() WHERE id = $2",
				confirmations, d.ID)
			if err != nil {
				ww.log.Error("Failed to update deposit confirmations", "err", err)
			}
		}
	}

	return nil
}
