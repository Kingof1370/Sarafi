package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"velyxora/packages/common"
	"velyxora/packages/custody"
	"velyxora/packages/database"
	"velyxora/packages/logger"
	"velyxora/packages/types"
)

// ScannerState manages scanning ranges per network
type ScannerState struct {
	Network           string
	LastScannedHeight int64
}

// WalletWorkerOrchestrator manages background scanners and processors
type WalletWorkerOrchestrator struct {
	mu           sync.Mutex
	db           *database.DB
	log          *logger.Logger
	producer     *common.KafkaProducer
	custody      *custody.CustodyEngine
	scannerState map[string]int64
	nonces       map[string]uint64 // address -> nonce tracking
	localTxs     map[string]bool   // tx_hash -> detected tracker
}

func NewWalletWorkerOrchestrator(db *database.DB, prod *common.KafkaProducer, ce *custody.CustodyEngine, log *logger.Logger) *WalletWorkerOrchestrator {
	return &WalletWorkerOrchestrator{
		db:           db,
		producer:     prod,
		custody:      ce,
		log:          log,
		scannerState: make(map[string]int64),
		nonces:       make(map[string]uint64),
		localTxs:     make(map[string]bool),
	}
}

// StartWorkers launches the concurrent scanners and workers
func (w *WalletWorkerOrchestrator) StartWorkers(ctx context.Context, wg *sync.WaitGroup) {
	// 0. Recover any pending BROADCASTING or CONFIRMING withdrawals on startup (Rule 24 Recovery)
	w.recoverPendingWithdrawals(ctx)

	// 1. Run Deposit Scanner background loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.log.Info("Starting Deposit Scanner routine...")
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				w.log.Info("Deposit Scanner routine stopped.")
				return
			case <-ticker.C:
				w.scanAllNetworks(ctx)
			}
		}
	}()

	// 2. Run Withdrawal Queue Worker background loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.log.Info("Starting Withdrawal Worker routine...")
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				w.log.Info("Withdrawal Worker routine stopped.")
				return
			case <-ticker.C:
				w.processWithdrawalQueue(ctx)
			}
		}
	}()
}

// recoverPendingWithdrawals scans DB for BROADCASTING or CONFIRMING states and resumes tracking
func (w *WalletWorkerOrchestrator) recoverPendingWithdrawals(ctx context.Context) {
	if w.db == nil {
		return
	}

	rows, err := w.db.Pool.Query(ctx,
		"SELECT id, user_id, asset, amount, fee, address, tx_hash, status FROM withdrawals WHERE status IN ('BROADCASTING', 'CONFIRMING')")
	if err != nil {
		w.log.Error("Recovery: failed to query pending withdrawals on startup", "err", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var wthID, userID, asset, address, txHash, statusStr string
		var amount, fee float64

		err = rows.Scan(&wthID, &userID, &asset, &amount, &fee, &address, &txHash, &statusStr)
		if err == nil && txHash != "" {
			adapter, err := common.GetBlockchainAdapter(asset)
			if err == nil {
				count++
				w.log.Info("Recovery: Resuming confirmation tracking for broadcasted withdrawal", "id", wthID, "tx", txHash)
				go w.monitorWithdrawalConfirmations(ctx, wthID, userID, asset, amount, fee, txHash, adapter)
			}
		}
	}
	w.log.Info("Recovery complete.", "pending_withdrawals_resumed", count)
}

// scanAllNetworks scans all supported simulated and configured networks
func (w *WalletWorkerOrchestrator) scanAllNetworks(ctx context.Context) {
	networks := []string{"BTC", "ETH", "SOL"}
	for _, net := range networks {
		adapter, err := common.GetBlockchainAdapter(net)
		if err != nil {
			continue
		}

		latestHeight, err := adapter.GetBlockHeight()
		if err != nil {
			w.log.Error("Failed to get block height from adapter", "network", net, "err", err)
			continue
		}

		w.mu.Lock()
		lastScanned, ok := w.scannerState[net]
		if !ok {
			// Pull from DB if available
			if w.db != nil {
				_ = w.db.Pool.QueryRow(ctx,
					"SELECT last_scanned_height FROM blockchain_scanner_states WHERE network = $1", net).
					Scan(&lastScanned)
			}
			if lastScanned == 0 {
				lastScanned = latestHeight - 5 // scan last 5 blocks initially
			}
			w.scannerState[net] = lastScanned
		}
		w.mu.Unlock()

		if latestHeight <= lastScanned {
			// Already scanned latest blocks, mine new block in simulation to keep pipeline moving
			if adapter.GetMode() == common.ModeSimulation {
				sim := common.GetSimulator(net)
				sim.MineBlock()
				latestHeight, _ = adapter.GetBlockHeight()
			} else {
				continue
			}
		}

		// Scan block ranges
		for height := lastScanned + 1; height <= latestHeight; height++ {
			w.log.Info("Scanning block", "network", net, "height", height)
			w.scanBlock(ctx, net, height, adapter)

			w.mu.Lock()
			w.scannerState[net] = height
			if w.db != nil {
				_, _ = w.db.Pool.Exec(ctx,
					"INSERT INTO blockchain_scanner_states (network, last_scanned_height, updated_at) VALUES ($1, $2, NOW()) ON CONFLICT (network) DO UPDATE SET last_scanned_height = $2, updated_at = NOW()",
					net, height)
			}
			w.mu.Unlock()
		}
	}
}

// scanBlock fetches transactions in a block and verifies matches
func (w *WalletWorkerOrchestrator) scanBlock(ctx context.Context, network string, height int64, adapter common.BlockchainAdapter) {
	if adapter.GetMode() == common.ModeSimulation {
		sim := common.GetSimulator(network)
		block := sim.GetBlockByHeight(height)
		if block == nil {
			return
		}

		for _, tx := range block.Transactions {
			w.processDetectedTransaction(ctx, network, tx.Hash, tx.To, tx.Amount, tx.Asset, height, block.Hash, adapter)
		}

		// Check for Reorg detection
		w.checkForOrphanedTransactions(ctx, network, sim)
	}
}

// processDetectedTransaction handles a single transaction match, managing confirmations state transition
func (w *WalletWorkerOrchestrator) processDetectedTransaction(ctx context.Context, network, txHash, toAddress string, amount float64, asset string, blockHeight int64, blockHash string, adapter common.BlockchainAdapter) {
	// 1. Verify if address is registered to a user
	var userID string
	var isRegistered bool

	if w.db != nil {
		err := w.db.Pool.QueryRow(ctx,
			"SELECT user_id FROM wallet_addresses WHERE address = $1 AND asset = $2 AND is_active = TRUE",
			toAddress, asset).Scan(&userID)
		if err == nil {
			isRegistered = true
		}
	} else {
		// Mock registered address mapping fallback
		if toAddress == "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2" && asset == "BTC" {
			userID = "usr_buyer"
			isRegistered = true
		} else if toAddress == "0x71C7656EC7ab88b098defB751B7401B5f6d1476B" && asset == "ETH" {
			userID = "usr_buyer"
			isRegistered = true
		} else if toAddress == "Hxs86Xj38x8vMvVvE75A9XG9m9L9p9aBcDe" && asset == "SOL" {
			userID = "usr_buyer"
			isRegistered = true
		}
	}

	if !isRegistered {
		return // Not our address
	}

	// 2. Fetch required confirmations configuration
	requiredConfirmations := int64(1)
	switch network {
	case "BTC":
		requiredConfirmations = 2
	case "ETH":
		requiredConfirmations = 3
	case "SOL":
		requiredConfirmations = 5
	}

	// 3. Track Confirmations
	currentConf, err := adapter.GetConfirmations(txHash)
	if err != nil {
		w.log.Error("Failed to fetch confirmations", "tx", txHash, "err", err)
		return
	}

	w.mu.Lock()
	alreadyProcessed := w.localTxs[txHash]
	w.mu.Unlock()

	var depStatus types.DepositStatus
	if int64(currentConf) >= requiredConfirmations {
		depStatus = types.DepositCompleted // Credited state
	} else {
		depStatus = types.DepositConfirmed // Confirming state
	}

	depID := "dep_" + txHash[:10]

	if w.db != nil {
		var existingStatus string
		err := w.db.Pool.QueryRow(ctx,
			"SELECT status FROM deposits WHERE tx_hash = $1 AND asset = $2",
			txHash, asset).Scan(&existingStatus)

		if err != nil {
			// Insert new deposit row (first detection)
			depStatus = types.DepositPending
			w.log.Info("New deposit detected on-chain", "tx", txHash, "user", userID, "amount", amount)

			_, err = w.db.Pool.Exec(ctx,
				"INSERT INTO deposits (id, user_id, asset, amount, fee, address, tx_hash, confirmations, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())",
				depID, userID, asset, amount, 0.0, toAddress, txHash, currentConf, string(depStatus))
			if err != nil {
				w.log.Error("Failed to insert deposit log to DB", "err", err)
				return
			}

			w.publishKafkaEvent(ctx, "deposit.detected", map[string]interface{}{
				"deposit_id":  depID,
				"user_id":     userID,
				"asset":       asset,
				"amount":      amount,
				"tx_hash":     txHash,
				"status":      string(depStatus),
				"block_height": blockHeight,
			})
		} else if existingStatus == string(types.DepositPending) || existingStatus == string(types.DepositConfirmed) {
			// Update confirmation count
			if int64(currentConf) >= requiredConfirmations {
				depStatus = types.DepositCompleted
			} else {
				depStatus = types.DepositConfirmed
			}

			_, err = w.db.Pool.Exec(ctx,
				"UPDATE deposits SET confirmations = $1, status = $2, updated_at = NOW() WHERE tx_hash = $3 AND asset = $4",
				currentConf, string(depStatus), txHash, asset)
			if err != nil {
				w.log.Error("Failed to update deposit confirmations", "err", err)
				return
			}

			w.publishKafkaEvent(ctx, "deposit.confirming", map[string]interface{}{
				"deposit_id":    depID,
				"user_id":       userID,
				"asset":         asset,
				"amount":        amount,
				"confirmations": currentConf,
				"status":        string(depStatus),
			})

			// If state is COMPLETED and was not credited, process credit
			if depStatus == types.DepositCompleted {
				err = w.creditUserBalance(ctx, userID, asset, amount, depID, txHash)
				if err != nil {
					w.log.Error("Credit balance operation failed", "err", err)
					return
				}
				w.publishKafkaEvent(ctx, "deposit.credited", map[string]interface{}{
					"deposit_id": depID,
					"user_id":    userID,
					"asset":      asset,
					"amount":     amount,
					"status":     string(types.DepositCompleted),
				})
			}
		}
	} else {
		// Local mock pipeline simulation
		if !alreadyProcessed {
			w.mu.Lock()
			w.localTxs[txHash] = true
			w.mu.Unlock()

			w.log.Info("[Mock Scan] Crediting local mock user available balance on finality", "user", userID, "asset", asset, "amount", amount)
			w.publishKafkaEvent(ctx, "deposit.detected", map[string]interface{}{"deposit_id": depID, "user_id": userID, "amount": amount, "tx_hash": txHash})
			w.publishKafkaEvent(ctx, "deposit.credited", map[string]interface{}{"deposit_id": depID, "user_id": userID, "amount": amount, "tx_hash": txHash})
		}
	}
}

// creditUserBalance performs atomic database available balance adjustments and double-entry ledger inputs
func (w *WalletWorkerOrchestrator) creditUserBalance(ctx context.Context, userID, asset string, amount float64, depID, txHash string) error {
	tx, err := w.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Fetch and lock balance row
	var available, total float64
	err = tx.QueryRow(ctx,
		"SELECT available, total FROM balances WHERE user_id = $1 AND asset = $2 FOR UPDATE",
		userID, asset).Scan(&available, &total)

	newAvailable := common.RoundToPrecision(available+amount, 8)
	newTotal := common.RoundToPrecision(total+amount, 8)

	if err != nil {
		// Balance doesn't exist, create initial row
		_, err = tx.Exec(ctx,
			"INSERT INTO balances (user_id, asset, available, locked, pending, reserved, total, updated_at) VALUES ($1, $2, $3, 0.0, 0.0, 0.0, $4, NOW())",
			userID, asset, amount, amount)
		if err != nil {
			return err
		}
	} else {
		_, err = tx.Exec(ctx,
			"UPDATE balances SET available = $1, total = $2, updated_at = NOW() WHERE user_id = $3 AND asset = $4",
			newAvailable, newTotal, userID, asset)
		if err != nil {
			return err
		}
	}

	// Generate Ledger Entries
	debitEntryID := fmt.Sprintf("ent_deb_%d", time.Now().UnixNano())
	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
		debitEntryID, depID, "COINBASE", asset, "DEBIT", amount, "Simulated network coinbase block delivery")
	if err != nil {
		return err
	}

	creditEntryID := fmt.Sprintf("ent_cred_%d", time.Now().UnixNano())
	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
		creditEntryID, depID, userID, asset, "CREDIT", amount, fmt.Sprintf("Deposit credit from blockchain hash %s", txHash))
	if err != nil {
		return err
	}

	// Commit atomic credit
	return tx.Commit(ctx)
}

// checkForOrphanedTransactions detects and corrects chain reorganizations state
func (w *WalletWorkerOrchestrator) checkForOrphanedTransactions(ctx context.Context, network string, sim *common.BlockchainSimulator) {
	if w.db == nil {
		return
	}

	// Look up all deposits currently in COMPLETED status
	rows, err := w.db.Pool.Query(ctx,
		"SELECT id, user_id, asset, amount, tx_hash FROM deposits WHERE status = 'COMPLETED' AND asset = $1",
		network)
	if err != nil {
		return
	}
	defer rows.Close()

	var orphanedTxs []map[string]interface{}
	for rows.Next() {
		var depID, userID, asset, txHash string
		var amount float64
		if err := rows.Scan(&depID, &userID, &asset, &amount, &txHash); err == nil {
			tx := sim.GetTransactionByHash(txHash)
			// If tx is orphaned/pending or has status "PENDING" on simulated node, a reorg has decoupled it
			if tx == nil || tx.Status == "PENDING" || tx.BlockHeight == 0 {
				orphanedTxs = append(orphanedTxs, map[string]interface{}{
					"id":      depID,
					"user_id": userID,
					"asset":   asset,
					"amount":  amount,
					"tx_hash": txHash,
				})
			}
		}
	}
	rows.Close()

	// Reverse balance credit and ledger entries for each orphaned transaction
	for _, o := range orphanedTxs {
		depID := o["id"].(string)
		userID := o["user_id"].(string)
		asset := o["asset"].(string)
		amount := o["amount"].(float64)
		txHash := o["tx_hash"].(string)

		w.log.Warn("CHAIN REORGANIZATION DETECTED: Orphaned deposit matched", "tx", txHash, "user", userID)

		tx, err := w.db.Pool.Begin(ctx)
		if err != nil {
			continue
		}
		defer tx.Rollback(ctx)

		// 1. Revert status
		_, err = tx.Exec(ctx,
			"UPDATE deposits SET status = 'REJECTED', updated_at = NOW() WHERE id = $1",
			depID)
		if err != nil {
			continue
		}

		// 2. Debit user available balance (prevent double spend)
		var available, total float64
		err = tx.QueryRow(ctx,
			"SELECT available, total FROM balances WHERE user_id = $1 AND asset = $2 FOR UPDATE",
			userID, asset).Scan(&available, &total)
		if err != nil {
			continue
		}

		newAvailable := common.RoundToPrecision(available-amount, 8)
		newTotal := common.RoundToPrecision(total-amount, 8)

		_, err = tx.Exec(ctx,
			"UPDATE balances SET available = $1, total = $2, updated_at = NOW() WHERE user_id = $3 AND asset = $4",
			newAvailable, newTotal, userID, asset)
		if err != nil {
			continue
		}

		// 3. Compensating ledger reversal entries
		reversalID := fmt.Sprintf("rev_%d", time.Now().UnixNano())
		_, err = tx.Exec(ctx,
			"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
			fmt.Sprintf("ent_deb_%d", time.Now().UnixNano()), reversalID, userID, asset, "DEBIT", amount, fmt.Sprintf("Chain Reorg reversal debit for orphaned hash %s", txHash))
		if err != nil {
			continue
		}

		_, err = tx.Exec(ctx,
			"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
			fmt.Sprintf("ent_cred_%d", time.Now().UnixNano()), reversalID, "COINBASE", asset, "CREDIT", amount, fmt.Sprintf("Chain Reorg reversal credit for orphaned hash %s", txHash))
		if err != nil {
			continue
		}

		if err := tx.Commit(ctx); err == nil {
			w.publishKafkaEvent(ctx, "deposit.reorged", map[string]interface{}{
				"deposit_id": depID,
				"user_id":    userID,
				"asset":      asset,
				"amount":     amount,
				"tx_hash":    txHash,
				"status":     "ORPHANED_REVERTED",
			})
		}
	}
}

// processWithdrawalQueue picks up approved withdrawals and builds/signs/broadcasts them
func (w *WalletWorkerOrchestrator) processWithdrawalQueue(ctx context.Context) {
	if w.db == nil {
		return
	}

	// Fetch all APPROVED withdrawals
	rows, err := w.db.Pool.Query(ctx,
		"SELECT id, user_id, asset, amount, fee, address, status FROM withdrawals WHERE status = 'APPROVED'")
	if err != nil {
		return
	}
	defer rows.Close()

	var withdrawalsToProcess []types.Withdrawal
	for rows.Next() {
		var wth types.Withdrawal
		var statusStr string
		err := rows.Scan(&wth.ID, &wth.UserID, &wth.Asset, &wth.Amount, &wth.Fee, &wth.Address, &statusStr)
		if err == nil {
			wth.Status = types.WithdrawalStatus(statusStr)
			withdrawalsToProcess = append(withdrawalsToProcess, wth)
		}
	}
	rows.Close()

	for _, wth := range withdrawalsToProcess {
		w.log.Info("Processing approved withdrawal transfer request", "id", wth.ID, "user", wth.UserID, "amount", wth.Amount)

		// [FIXED] Balance reservation check: We DO NOT call reserveBalanceForWithdrawal again here!
		// The available balance is already reserved when the request was accepted in API Gateway.
		// We simply proceed directly with transition and broadcast!

		// 1. Update Status to BROADCASTING
		_, err = w.db.Pool.Exec(ctx,
			"UPDATE withdrawals SET status = 'BROADCASTING', updated_at = NOW() WHERE id = $1",
			wth.ID)
		if err != nil {
			w.log.Error("Failed to transition status to BROADCASTING", "id", wth.ID, "err", err)
			continue
		}
		w.publishKafkaEvent(ctx, "withdrawal.queued", map[string]interface{}{"withdrawal_id": wth.ID, "status": "QUEUED"})

		// 2. Increment Nonce (Nonce tracking for accounts-based model)
		w.mu.Lock()
		nonce := w.nonces[wth.Address]
		w.nonces[wth.Address] = nonce + 1
		w.mu.Unlock()

		// 3. Secure key-signing simulation
		w.publishKafkaEvent(ctx, "withdrawal.signed", map[string]interface{}{"withdrawal_id": wth.ID, "status": "SIGNED", "nonce": nonce})

		// 4. Broadcast to Simulated Blockchain Adapter
		adapter, err := common.GetBlockchainAdapter(wth.Asset)
		if err != nil {
			w.log.Error("Unsupported network adapter", "asset", wth.Asset, "err", err)
			w.failWithdrawal(ctx, wth.ID, "Unsupported network adapter")
			w.releaseBalanceReservation(ctx, wth.UserID, wth.Asset, wth.Amount+wth.Fee)
			continue
		}

		var txHash string
		if adapter.GetMode() == common.ModeSimulation {
			sim := common.GetSimulator(wth.Asset)
			warmVaultAddress := "warm_vault_address_" + wth.Asset
			sim.SetBalance(warmVaultAddress, wth.Asset, 500.0)

			txHash, err = sim.SubmitTransaction(warmVaultAddress, wth.Address, wth.Asset, wth.Amount)
		} else {
			txHash, err = adapter.BroadcastTransaction(fmt.Sprintf("raw_tx_payload_sign_%s", wth.ID))
		}

		if err != nil {
			w.log.Error("Broadcast transaction failed on adapter", "id", wth.ID, "err", err)
			w.failWithdrawal(ctx, wth.ID, "On-chain broadcast transmission failed")
			w.releaseBalanceReservation(ctx, wth.UserID, wth.Asset, wth.Amount+wth.Fee)
			continue
		}

		// 5. Record hash and set status to BROADCASTING/CONFIRMING
		_, err = w.db.Pool.Exec(ctx,
			"UPDATE withdrawals SET tx_hash = $1, status = 'BROADCASTING', updated_at = NOW() WHERE id = $2",
			txHash, wth.ID)
		if err != nil {
			continue
		}

		w.publishKafkaEvent(ctx, "withdrawal.broadcast", map[string]interface{}{
			"withdrawal_id": wth.ID,
			"status":        "BROADCASTING",
			"tx_hash":       txHash,
		})

		// 6. Track confirmations in background until finality
		go w.monitorWithdrawalConfirmations(ctx, wth.ID, wth.UserID, wth.Asset, wth.Amount, wth.Fee, txHash, adapter)
	}
}

// monitorWithdrawalConfirmations polls block height confirmations until withdrawal is COMPLETED
func (w *WalletWorkerOrchestrator) monitorWithdrawalConfirmations(ctx context.Context, wthID, userID, asset string, amount, fee float64, txHash string, adapter common.BlockchainAdapter) {
	requiredConfirmations := int64(1)
	switch asset {
	case "BTC":
		requiredConfirmations = 2
	case "ETH":
		requiredConfirmations = 3
	case "SOL":
		requiredConfirmations = 5
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			conf, err := adapter.GetConfirmations(txHash)
			if err != nil {
				continue
			}

			w.publishKafkaEvent(ctx, "withdrawal.confirming", map[string]interface{}{
				"withdrawal_id": wthID,
				"confirmations": conf,
				"status":        "CONFIRMING",
			})

			if int64(conf) >= requiredConfirmations {
				w.log.Info("Withdrawal transaction reached finality and is COMPLETED", "id", wthID, "tx", txHash)

				// Complete Double Entry settlement: debit reserved balance, debit from client available total
				err = w.settleCompletedWithdrawal(ctx, userID, asset, amount, fee, wthID)
				if err != nil {
					w.log.Error("Failed to settle final completed withdrawal ledger", "id", wthID, "err", err)
					return
				}

				w.publishKafkaEvent(ctx, "withdrawal.completed", map[string]interface{}{
					"withdrawal_id": wthID,
					"status":        "COMPLETED",
					"tx_hash":       txHash,
				})
				return
			}
		}
	}
}

// reserveBalanceForWithdrawal atomically locks and reserves available balances
func (w *WalletWorkerOrchestrator) reserveBalanceForWithdrawal(ctx context.Context, userID, asset string, totalAmount float64) error {
	tx, err := w.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var available, reserved, total float64
	err = tx.QueryRow(ctx,
		"SELECT available, reserved, total FROM balances WHERE user_id = $1 AND asset = $2 FOR UPDATE",
		userID, asset).Scan(&available, &reserved, &total)
	if err != nil {
		return fmt.Errorf("user balance row does not exist: %w", err)
	}

	if available < totalAmount {
		return fmt.Errorf("insufficient available balance: has %f, needs %f", available, totalAmount)
	}

	newAvailable := common.RoundToPrecision(available-totalAmount, 8)
	newReserved := common.RoundToPrecision(reserved+totalAmount, 8)

	_, err = tx.Exec(ctx,
		"UPDATE balances SET available = $1, reserved = $2, updated_at = NOW() WHERE user_id = $3 AND asset = $4",
		newAvailable, newReserved, userID, asset)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// releaseBalanceReservation reverses reservation if transaction fails
func (w *WalletWorkerOrchestrator) releaseBalanceReservation(ctx context.Context, userID, asset string, totalAmount float64) {
	tx, err := w.db.Pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)

	var available, reserved float64
	err = tx.QueryRow(ctx,
		"SELECT available, reserved FROM balances WHERE user_id = $1 AND asset = $2 FOR UPDATE",
		userID, asset).Scan(&available, &reserved)
	if err != nil {
		return
	}

	newAvailable := common.RoundToPrecision(available+totalAmount, 8)
	newReserved := common.RoundToPrecision(reserved-totalAmount, 8)
	if newReserved < 0 {
		newReserved = 0
	}

	_, _ = tx.Exec(ctx,
		"UPDATE balances SET available = $1, reserved = $2, updated_at = NOW() WHERE user_id = $3 AND asset = $4",
		newAvailable, newReserved, userID, asset)

	_ = tx.Commit(ctx)
}

// settleCompletedWithdrawal debits the reserved segment and posts final double-entry ledger entries
func (w *WalletWorkerOrchestrator) settleCompletedWithdrawal(ctx context.Context, userID, asset string, amount, fee float64, wthID string) error {
	tx, err := w.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Debit reserved balance, update total
	var available, reserved, total float64
	err = tx.QueryRow(ctx,
		"SELECT available, reserved, total FROM balances WHERE user_id = $1 AND asset = $2 FOR UPDATE",
		userID, asset).Scan(&available, &reserved, &total)
	if err != nil {
		return err
	}

	totalAmount := amount + fee
	newReserved := common.RoundToPrecision(reserved-totalAmount, 8)
	newTotal := common.RoundToPrecision(total-totalAmount, 8)
	if newReserved < 0 {
		newReserved = 0
	}

	_, err = tx.Exec(ctx,
		"UPDATE balances SET reserved = $1, total = $2, updated_at = NOW() WHERE user_id = $3 AND asset = $4",
		newReserved, newTotal, userID, asset)
	if err != nil {
		return err
	}

	// 2. Set status to COMPLETED
	_, err = tx.Exec(ctx,
		"UPDATE withdrawals SET status = 'COMPLETED', updated_at = NOW() WHERE id = $1",
		wthID)
	if err != nil {
		return err
	}

	// 3. Post Double-Entry Ledger
	debitEntryID := fmt.Sprintf("ent_deb_%d", time.Now().UnixNano())
	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
		debitEntryID, wthID, userID, asset, "DEBIT", totalAmount, fmt.Sprintf("Completed withdrawal payout of %f (fee: %f)", amount, fee))
	if err != nil {
		return err
	}

	creditEntryID := fmt.Sprintf("ent_cred_%d", time.Now().UnixNano())
	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
		creditEntryID, wthID, "EXTERNAL_NETWORK", asset, "CREDIT", amount, fmt.Sprintf("External network transfer delivery for %s", wthID))
	if err != nil {
		return err
	}

	// Fee earnings entry
	if fee > 0 {
		feeEntryID := fmt.Sprintf("ent_cred_fee_%d", time.Now().UnixNano())
		_, err = tx.Exec(ctx,
			"INSERT INTO ledger_entries (id, ledger_tx_id, user_id, asset, type, amount, description, timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())",
			feeEntryID, wthID, "REVENUE_ACCOUNT", asset, "CREDIT", fee, fmt.Sprintf("Withdrawal fee revenue for %s", wthID))
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// failWithdrawal transitions withdrawal status to FAILED
func (w *WalletWorkerOrchestrator) failWithdrawal(ctx context.Context, wthID, reason string) {
	if w.db == nil {
		return
	}
	_, _ = w.db.Pool.Exec(ctx,
		"UPDATE withdrawals SET status = 'FAILED', updated_at = NOW() WHERE id = $1",
		wthID)

	w.publishKafkaEvent(ctx, "withdrawal.failed", map[string]interface{}{
		"withdrawal_id": wthID,
		"status":        "FAILED",
		"reason":        reason,
	})
}

// publishKafkaEvent emits events safely to Kafka topics
func (w *WalletWorkerOrchestrator) publishKafkaEvent(ctx context.Context, topic string, payload interface{}) {
	if w.producer == nil {
		return
	}
	event := types.KafkaEvent{
		Type:      types.EventType(topic),
		Payload:   payload,
		Timestamp: time.Now(),
	}
	_ = w.producer.Publish(ctx, "velyxora-wallet-updates", topic, event)
}
