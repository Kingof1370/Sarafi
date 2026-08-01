package engine

import (
	"sync"
)

// Position records current exposure for a user on a given symbol with PnL structures
type Position struct {
	UserID        string  `json:"user_id"`
	Symbol        string  `json:"symbol"`
	Size          float64 `json:"size"` // positive for long, negative for short
	EntryPrice    float64 `json:"entry_price"`
	RealizedPnL   float64 `json:"realized_pnl"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
}

// PositionEngine tracks user spot or margin positions dynamically with asset reservations
type PositionEngine struct {
	mu             sync.RWMutex
	positions      map[string]map[string]*Position // userID -> symbol -> Position
	reservedAssets map[string]map[string]float64   // userID -> asset -> reserved quantity
	lockedAssets   map[string]map[string]float64   // userID -> asset -> locked quantity
}

// NewPositionEngine creates an upgraded Enterprise exposure and portfolio tracking core
func NewPositionEngine() *PositionEngine {
	return &PositionEngine{
		positions:      make(map[string]map[string]*Position),
		reservedAssets: make(map[string]map[string]float64),
		lockedAssets:   make(map[string]map[string]float64),
	}
}

// ReserveAsset increments active reserved assets
func (pe *PositionEngine) ReserveAsset(userID, asset string, qty float64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if _, ok := pe.reservedAssets[userID]; !ok {
		pe.reservedAssets[userID] = make(map[string]float64)
	}
	pe.reservedAssets[userID][asset] += qty
}

// LockAsset transfers reserved assets to locked assets
func (pe *PositionEngine) LockAsset(userID, asset string, qty float64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if userRes, ok := pe.reservedAssets[userID]; ok {
		userRes[asset] -= qty
		if userRes[asset] < 0 {
			userRes[asset] = 0
		}
	}

	if _, ok := pe.lockedAssets[userID]; !ok {
		pe.lockedAssets[userID] = make(map[string]float64)
	}
	pe.lockedAssets[userID][asset] += qty
}

// ReleaseLockedAsset releases asset locks
func (pe *PositionEngine) ReleaseLockedAsset(userID, asset string, qty float64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if userLocks, ok := pe.lockedAssets[userID]; ok {
		userLocks[asset] -= qty
		if userLocks[asset] < 0 {
			userLocks[asset] = 0
		}
	}
}

// RecordExecution updates spot position sheets,EntryPrices, and computes realized PnL
func (pe *PositionEngine) RecordExecution(userID, symbol string, sizeChange, price float64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if _, ok := pe.positions[userID]; !ok {
		pe.positions[userID] = make(map[string]*Position)
	}

	pos, exists := pe.positions[userID][symbol]
	if !exists {
		pos = &Position{
			UserID:        userID,
			Symbol:        symbol,
			Size:          0,
			EntryPrice:    0,
			RealizedPnL:   0,
			UnrealizedPnL: 0,
		}
		pe.positions[userID][symbol] = pos
	}

	if pos.Size == 0 {
		pos.Size = sizeChange
		pos.EntryPrice = price
	} else {
		newSize := pos.Size + sizeChange
		// Detect realized profit/loss when closing or reducing exposure (opposite side matching)
		if (pos.Size > 0 && sizeChange < 0) || (pos.Size < 0 && sizeChange > 0) {
			// Compute realized profit/loss
			closedSize := sizeChange
			if (pos.Size > 0 && newSize < 0) || (pos.Size < 0 && newSize > 0) {
				// Position reversed fully
				closedSize = -pos.Size
			}

			realized := 0.0
			if pos.Size > 0 {
				realized = -closedSize * (price - pos.EntryPrice)
			} else {
				realized = closedSize * (pos.EntryPrice - price)
			}
			pos.RealizedPnL += realized
		}

		// Calculate new average entry price if increasing positions
		if (pos.Size > 0 && sizeChange > 0) || (pos.Size < 0 && sizeChange < 0) {
			pos.EntryPrice = (pos.Size*pos.EntryPrice + sizeChange*price) / newSize
		}

		pos.Size = newSize
	}
}

// GetPosition retrieves position exposure
func (pe *PositionEngine) GetPosition(userID, symbol string) *Position {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	if userPos, ok := pe.positions[userID]; ok {
		if pos, ok := userPos[symbol]; ok {
			return pos
		}
	}

	return &Position{UserID: userID, Symbol: symbol, Size: 0, EntryPrice: 0}
}

// UpdateUnrealizedPnL calculates portfolio floating profits and losses against index price feeds
func (pe *PositionEngine) UpdateUnrealizedPnL(userID, symbol string, currentPrice float64) float64 {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if userPos, ok := pe.positions[userID]; ok {
		if pos, ok := userPos[symbol]; ok {
			if pos.Size > 0 {
				pos.UnrealizedPnL = pos.Size * (currentPrice - pos.EntryPrice)
			} else if pos.Size < 0 {
				pos.UnrealizedPnL = pos.Size * (pos.EntryPrice - currentPrice)
			} else {
				pos.UnrealizedPnL = 0
			}
			return pos.UnrealizedPnL
		}
	}
	return 0
}
