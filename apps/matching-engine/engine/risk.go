package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"velyxora/packages/common"
	"velyxora/packages/database"
	"velyxora/packages/types"
)

// STPMode represents the Self-Trade Prevention configuration
type STPMode string

const (
	STP_CancelNewest STPMode = "CANCEL_NEWEST"
	STP_CancelOldest STPMode = "CANCEL_OLDEST"
	STP_CancelBoth   STPMode = "CANCEL_BOTH"
	STP_Allow        STPMode = "ALLOW"
)

// RiskEngine manages pre-trade validation, post-trade settlement checks, STP, and abuse blocklists
type RiskEngine struct {
	mu               sync.RWMutex
	db               *database.DB                  // authoritative database reference
	userBalances     map[string]map[string]float64 // userID -> asset -> available amount
	activeHolds      map[string]map[string]float64 // userID -> asset -> held amount
	dailyVolume      map[string]float64            // userID -> cumulative daily USD volume
	dailyLimits      map[string]float64            // userID -> max daily volume limits
	requestTracks    map[string][]time.Time        // userID/IP -> request timestamps
	blockedAccounts  map[string]bool               // userID -> blocked status
	suspendedMarkets map[string]bool               // symbol -> suspended status

	// Price Protections
	marketReferencePrices map[string]float64 // symbol -> last index price
	priceBandPercentage   float64            // e.g. 0.10 (10% deviation limits)
	tradingHalted         bool
	stpMode               STPMode
}

// NewRiskEngine creates an upgraded Enterprise Risk and Abuse protection engine
func NewRiskEngine(minPrice, maxPrice float64) *RiskEngine {
	return &RiskEngine{
		userBalances:          make(map[string]map[string]float64),
		activeHolds:           make(map[string]map[string]float64),
		dailyVolume:           make(map[string]float64),
		dailyLimits:           make(map[string]float64),
		requestTracks:         make(map[string][]time.Time),
		blockedAccounts:       make(map[string]bool),
		suspendedMarkets:      make(map[string]bool),
		marketReferencePrices: make(map[string]float64),
		priceBandPercentage:   0.10, // Default 10% price band
		tradingHalted:         false,
		stpMode:               STP_CancelNewest,
	}
}

// SetSTPMode configures the Self-Trade Prevention policy
func (re *RiskEngine) SetSTPMode(mode STPMode) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.stpMode = mode
}

// SetHaltStatus triggers global trading halts and persists in DB
func (re *RiskEngine) SetHaltStatus(halted bool) {
	re.mu.Lock()
	re.tradingHalted = halted
	re.mu.Unlock()

	if re.db != nil {
		ctx := context.Background()
		val := "false"
		if halted {
			val = "true"
		}
		_, _ = re.db.Pool.Exec(ctx,
			`INSERT INTO risk_controls (key, value, updated_at) VALUES ('halt', $1, NOW())
			 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, val)
	}
}

// BlockAccount blocks a user from trading and persists in DB
func (re *RiskEngine) BlockAccount(userID string, blocked bool) {
	re.mu.Lock()
	re.blockedAccounts[userID] = blocked
	re.mu.Unlock()

	if re.db != nil {
		ctx := context.Background()
		val := "false"
		if blocked {
			val = "true"
		}
		_, _ = re.db.Pool.Exec(ctx,
			`INSERT INTO risk_controls (key, value, updated_at) VALUES ($1, $2, NOW())
			 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
			"block_"+userID, val)
	}
}

// SuspendMarket suspends trading for a specific symbol and persists in DB
func (re *RiskEngine) SuspendMarket(symbol string, suspended bool) {
	re.mu.Lock()
	re.suspendedMarkets[symbol] = suspended
	re.mu.Unlock()

	if re.db != nil {
		ctx := context.Background()
		val := "false"
		if suspended {
			val = "true"
		}
		_, _ = re.db.Pool.Exec(ctx,
			`INSERT INTO risk_controls (key, value, updated_at) VALUES ($1, $2, NOW())
			 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
			"suspend_"+symbol, val)
	}
}

// SetReferencePrice registers price feeds for band protections
func (re *RiskEngine) SetReferencePrice(symbol string, price float64) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.marketReferencePrices[symbol] = price
}

// DepositAsset sets or updates a user balance
func (re *RiskEngine) DepositAsset(userID, asset string, amount float64) {
	re.mu.Lock()
	defer re.mu.Unlock()

	if _, ok := re.userBalances[userID]; !ok {
		re.userBalances[userID] = make(map[string]float64)
	}
	re.userBalances[userID][asset] += amount
}

// GetAvailableBalance returns available balance
func (re *RiskEngine) GetAvailableBalance(userID, asset string) float64 {
	re.mu.RLock()
	defer re.mu.RUnlock()

	bal := 0.0
	if userBals, ok := re.userBalances[userID]; ok {
		bal = userBals[asset]
	}

	holds := 0.0
	if userHolds, ok := re.activeHolds[userID]; ok {
		holds = userHolds[asset]
	}

	available := bal - holds
	if available < 0 {
		return 0
	}
	return available
}

// ValidateOrder performs intensive pre-trade risk and abuse checks
func (re *RiskEngine) ValidateOrder(order *types.Order, quoteAsset, baseAsset string, feeRate float64) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	// Double check state from database if connected to survive restarts/crashes
	if re.db != nil {
		ctx := context.Background()
		var haltVal, blockVal, suspendVal string
		_ = re.db.Pool.QueryRow(ctx, "SELECT value FROM risk_controls WHERE key = 'halt'").Scan(&haltVal)
		_ = re.db.Pool.QueryRow(ctx, "SELECT value FROM risk_controls WHERE key = $1", "block_"+order.UserID).Scan(&blockVal)
		_ = re.db.Pool.QueryRow(ctx, "SELECT value FROM risk_controls WHERE key = $1", "suspend_"+order.Symbol).Scan(&suspendVal)

		if haltVal == "true" {
			re.tradingHalted = true
		} else if haltVal == "false" {
			re.tradingHalted = false
		}
		if blockVal == "true" {
			re.blockedAccounts[order.UserID] = true
		} else if blockVal == "false" {
			re.blockedAccounts[order.UserID] = false
		}
		if suspendVal == "true" {
			re.suspendedMarkets[order.Symbol] = true
		} else if suspendVal == "false" {
			re.suspendedMarkets[order.Symbol] = false
		}
	}

	// 1. Trading Halt check
	if re.tradingHalted {
		return fmt.Errorf("trading is currently halted globally")
	}

	// 2. Blocklist and suspended markets check
	if re.blockedAccounts[order.UserID] {
		return fmt.Errorf("user account %s is blocked due to risk violations", order.UserID)
	}
	if re.suspendedMarkets[order.Symbol] {
		return fmt.Errorf("trading pair %s is currently suspended", order.Symbol)
	}

	// 3. Spam & Rapid Fire Detection
	now := time.Now()
	re.requestTracks[order.UserID] = append(re.requestTracks[order.UserID], now)

	// Filter requests inside the last 1 second window
	var activeReqs []time.Time
	for _, t := range re.requestTracks[order.UserID] {
		if now.Sub(t) < time.Second {
			activeReqs = append(activeReqs, t)
		}
	}
	re.requestTracks[order.UserID] = activeReqs
	if len(activeReqs) > 10 { // Max 10 order placements per second to protect against spam
		return fmt.Errorf("abuse protection: order placement rate exceeded allowed threshold")
	}

	// 4. Daily Volume Limit verification
	orderValue := common.SafeMul(order.Quantity, order.Price)
	userVol := re.dailyVolume[order.UserID]
	maxVol := re.dailyLimits[order.UserID]
	if maxVol > 0 && common.SafeAdd(userVol, orderValue) > maxVol {
		return fmt.Errorf("daily trading volume limit exceeded: allowed max %f USD", maxVol)
	}

	// 5. Price Band Protection
	if order.Type == types.TypeLimit && order.Price > 0 {
		refPrice := re.marketReferencePrices[order.Symbol]
		if refPrice > 0 {
			deviation := common.SafeDiv(common.SafeSub(order.Price, refPrice), refPrice)
			if deviation > re.priceBandPercentage || deviation < -re.priceBandPercentage {
				return fmt.Errorf("price band protection: order price %f deviates too much from reference %f", order.Price, refPrice)
			}
		}
	}

	// 6. Balance verification and atomic reservation
	var requiredAmount float64
	var targetAsset string
	if order.Side == types.SideBuy {
		requiredAmount = common.SafeMul(common.SafeMul(order.Quantity, order.Price), common.SafeAdd(1.0, feeRate))
		targetAsset = quoteAsset
	} else {
		requiredAmount = order.Quantity
		targetAsset = baseAsset
	}

	if re.db != nil {
		ctx := context.Background()
		tx, err := re.db.Pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin atomic database reservation tx: %w", err)
		}
		defer tx.Rollback(ctx)

		var available, reserved, total float64
		err = tx.QueryRow(ctx,
			"SELECT available, reserved, total FROM balances WHERE user_id = $1 AND asset = $2 FOR UPDATE",
			order.UserID, targetAsset).Scan(&available, &reserved, &total)
		if err != nil {
			return fmt.Errorf("insufficient funds: required %f %s, available 0.0", requiredAmount, targetAsset)
		}

		if available < requiredAmount {
			return fmt.Errorf("insufficient funds: required %f %s, available %f", requiredAmount, targetAsset, available)
		}

		newAvailable := common.SafeSub(available, requiredAmount)
		newReserved := common.SafeAdd(reserved, requiredAmount)

		_, err = tx.Exec(ctx,
			"UPDATE balances SET available = $1, reserved = $2, updated_at = NOW() WHERE user_id = $3 AND asset = $4",
			newAvailable, newReserved, order.UserID, targetAsset)
		if err != nil {
			return fmt.Errorf("failed to persist database reservation: %w", err)
		}

		err = tx.Commit(ctx)
		if err != nil {
			return fmt.Errorf("failed to commit database reservation: %w", err)
		}

		// Sync to in-memory state for consistency
		if _, ok := re.userBalances[order.UserID]; !ok {
			re.userBalances[order.UserID] = make(map[string]float64)
		}
		re.userBalances[order.UserID][targetAsset] = total

		if _, ok := re.activeHolds[order.UserID]; !ok {
			re.activeHolds[order.UserID] = make(map[string]float64)
		}
		re.activeHolds[order.UserID][targetAsset] = newReserved

	} else {
		// Standalone memory-only fallback
		available := 0.0
		if bals, ok := re.userBalances[order.UserID]; ok {
			available = bals[targetAsset]
		}
		holds := 0.0
		if hlds, ok := re.activeHolds[order.UserID]; ok {
			holds = hlds[targetAsset]
		}

		availableBalance := common.SafeSub(available, holds)
		if availableBalance < requiredAmount {
			return fmt.Errorf("insufficient funds: required %f %s, available %f", requiredAmount, targetAsset, availableBalance)
		}

		// Apply Hold
		if _, ok := re.activeHolds[order.UserID]; !ok {
			re.activeHolds[order.UserID] = make(map[string]float64)
		}
		re.activeHolds[order.UserID][targetAsset] = common.SafeAdd(re.activeHolds[order.UserID][targetAsset], requiredAmount)
	}

	return nil
}

// ReleaseHold clears a balance hold and releases reservation atomically in database
func (re *RiskEngine) ReleaseHold(userID, asset string, amount float64) {
	re.mu.Lock()
	defer re.mu.Unlock()

	// Update in-memory hold
	if userHolds, ok := re.activeHolds[userID]; ok {
		userHolds[asset] = common.SafeSub(userHolds[asset], amount)
		if userHolds[asset] < 0 {
			userHolds[asset] = 0
		}
	}

	// Update database reservation atomically
	if re.db != nil {
		ctx := context.Background()
		tx, err := re.db.Pool.Begin(ctx)
		if err != nil {
			return
		}
		defer tx.Rollback(ctx)

		var available, reserved float64
		err = tx.QueryRow(ctx,
			"SELECT available, reserved FROM balances WHERE user_id = $1 AND asset = $2 FOR UPDATE",
			userID, asset).Scan(&available, &reserved)
		if err == nil {
			releaseAmt := amount
			if releaseAmt > reserved {
				releaseAmt = reserved
			}
			newAvailable := common.SafeAdd(available, releaseAmt)
			newReserved := common.SafeSub(reserved, releaseAmt)

			_, _ = tx.Exec(ctx,
				"UPDATE balances SET available = $1, reserved = $2, updated_at = NOW() WHERE user_id = $3 AND asset = $4",
				newAvailable, newReserved, userID, asset)
			_ = tx.Commit(ctx)
		}
	}
}

// UpdateBalance applies actual settlement adjustments
func (re *RiskEngine) UpdateBalance(userID, asset string, balanceChange float64) {
	re.mu.Lock()
	defer re.mu.Unlock()

	if _, ok := re.userBalances[userID]; !ok {
		re.userBalances[userID] = make(map[string]float64)
	}
	re.userBalances[userID][asset] += balanceChange
	if re.userBalances[userID][asset] < 0 {
		re.userBalances[userID][asset] = 0
	}
}

// TrackVolume accumulates daily trading volumes
func (re *RiskEngine) TrackVolume(userID string, usdVolume float64) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.dailyVolume[userID] += usdVolume
}

// SetDailyLimit configures daily volume caps
func (re *RiskEngine) SetDailyLimit(userID string, limit float64) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.dailyLimits[userID] = limit
}

// VerifySelfTrade implements dynamic self-trade prevention checks
func (re *RiskEngine) VerifySelfTrade(buyerID, sellerID string) (bool, STPMode) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	if buyerID == sellerID {
		return true, re.stpMode
	}
	return false, STP_Allow
}

// LoadRiskControls loads persisted risk configurations from PostgreSQL
func (re *RiskEngine) LoadRiskControls(ctx context.Context) {
	if re.db == nil {
		return
	}

	rows, err := re.db.Pool.Query(ctx, "SELECT key, value FROM risk_controls")
	if err != nil {
		return
	}
	defer rows.Close()

	re.mu.Lock()
	defer re.mu.Unlock()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err == nil {
			if key == "halt" {
				re.tradingHalted = (value == "true")
			} else if strings.HasPrefix(key, "block_") {
				uID := strings.TrimPrefix(key, "block_")
				re.blockedAccounts[uID] = (value == "true")
			} else if strings.HasPrefix(key, "suspend_") {
				sym := strings.TrimPrefix(key, "suspend_")
				re.suspendedMarkets[sym] = (value == "true")
			}
		}
	}
}

// SetDB dynamically configures or updates the database reference
func (re *RiskEngine) SetDB(db *database.DB) {
	re.mu.Lock()
	re.db = db
	re.mu.Unlock()
	re.LoadRiskControls(context.Background())
}
