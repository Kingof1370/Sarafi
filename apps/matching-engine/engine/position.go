package engine

import (
	"math"
	"sync"
)

// Position records current exposure for a user on a given symbol with PnL structures
type Position struct {
	UserID                string  `json:"user_id"`
	Symbol                string  `json:"symbol"`
	Size                  float64 `json:"size"` // positive for long, negative for short
	EntryPrice            float64 `json:"entry_price"`
	Margin                float64 `json:"margin"`                 // collateral margin assigned
	Leverage              float64 `json:"leverage"`               // leverage used (up to 100x)
	MaintenanceMarginRate float64 `json:"maintenance_margin_rate"` // e.g. 0.05 for 5%
	RealizedPnL           float64 `json:"realized_pnl"`
	UnrealizedPnL         float64 `json:"unrealized_pnl"`
}

// CalculateUnrealizedPnL updates and returns uPnL based on the current Mark Price
func (p *Position) CalculateUnrealizedPnL(markPrice float64) float64 {
	p.UnrealizedPnL = p.Size * (markPrice - p.EntryPrice)
	return p.UnrealizedPnL
}

// CalculateCollateral returns Collateral = Margin + UnrealizedPnL
func (p *Position) CalculateCollateral(markPrice float64) float64 {
	uPnL := p.CalculateUnrealizedPnL(markPrice)
	return p.Margin + uPnL
}

// CalculateMaintenanceMargin returns MaintenanceMargin = |Size| * markPrice * MaintenanceMarginRate
func (p *Position) CalculateMaintenanceMargin(markPrice float64) float64 {
	sizeAbs := p.Size
	if sizeAbs < 0 {
		sizeAbs = -sizeAbs
	}
	return sizeAbs * markPrice * p.MaintenanceMarginRate
}

// CalculateMarginRatio returns MarginRatio = MaintenanceMargin / Collateral
func (p *Position) CalculateMarginRatio(markPrice float64) float64 {
	collateral := p.CalculateCollateral(markPrice)
	if collateral <= 0 {
		return 999.0 // high margin ratio indicating default/liquidation threshold breached
	}
	mm := p.CalculateMaintenanceMargin(markPrice)
	return mm / collateral
}

// CalculateLiquidationPrice returns the liquidation price based on leverage and maintenance margin
func (p *Position) CalculateLiquidationPrice(markPrice float64) float64 {
	if p.Leverage <= 0 {
		return 0
	}
	if p.Size > 0 {
		// Static Long formula: LiquidationPrice = EntryPrice * (1.0 - 1.0/Leverage) / (1.0 - MaintenanceMarginRate)
		return p.EntryPrice * (1.0 - 1.0/p.Leverage) / (1.0 - p.MaintenanceMarginRate)
	} else if p.Size < 0 {
		// Static Short formula: LiquidationPrice = EntryPrice * (1.0 + 1.0/Leverage) / (1.0 + MaintenanceMarginRate)
		return p.EntryPrice * (1.0 + 1.0/p.Leverage) / (1.0 + p.MaintenanceMarginRate)
	}
	return 0
}

// CalculateBankruptcyPrice returns the price at which remaining collateral is zero
func (p *Position) CalculateBankruptcyPrice() float64 {
	if p.Leverage <= 0 {
		return 0
	}
	if p.Size > 0 {
		return p.EntryPrice * (1.0 - 1.0/p.Leverage)
	} else if p.Size < 0 {
		return p.EntryPrice * (1.0 + 1.0/p.Leverage)
	}
	return 0
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

// RecordExecution updates spot position sheets, EntryPrices, and computes realized PnL
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

// RecordFuturesExecution records futures positions updates with robust position flip/reversal handling
func (pe *PositionEngine) RecordFuturesExecution(userID, symbol string, sizeChange, price, leverage, margin, mmRate float64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if _, ok := pe.positions[userID]; !ok {
		pe.positions[userID] = make(map[string]*Position)
	}

	pos, exists := pe.positions[userID][symbol]
	if !exists {
		pos = &Position{
			UserID:                userID,
			Symbol:                symbol,
			Size:                  0,
			EntryPrice:            0,
			Margin:                margin,
			Leverage:              leverage,
			MaintenanceMarginRate: mmRate,
			RealizedPnL:           0,
			UnrealizedPnL:         0,
		}
		pe.positions[userID][symbol] = pos
	}

	if pos.Size == 0 {
		pos.Size = sizeChange
		pos.EntryPrice = price
		pos.Margin = margin
		pos.Leverage = leverage
		pos.MaintenanceMarginRate = mmRate
	} else {
		newSize := pos.Size + sizeChange
		// Detect realized profit/loss when closing or reducing exposure (opposite side matching)
		if (pos.Size > 0 && sizeChange < 0) || (pos.Size < 0 && sizeChange > 0) {
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

			// Release margin proportionally
			if (pos.Size > 0 && newSize < 0) || (pos.Size < 0 && newSize > 0) {
				// Reversing: fully release old margin
				pos.Margin = 0
			} else {
				if pos.Size != 0 {
					releasedMargin := (closedSize / pos.Size) * pos.Margin
					if releasedMargin < 0 {
						releasedMargin = -releasedMargin
					}
					pos.Margin -= releasedMargin
					if pos.Margin < 0 {
						pos.Margin = 0
					}
				}
			}

			// If position reverses/flips, start the opposite position with correct margin and entry price
			if (pos.Size > 0 && newSize < 0) || (pos.Size < 0 && newSize > 0) {
				remainingSize := sizeChange + pos.Size
				pos.Size = remainingSize
				pos.EntryPrice = price
				pos.Margin = (math.Abs(remainingSize) * price) / leverage
				pos.Leverage = leverage
				pos.MaintenanceMarginRate = mmRate
				return
			}
		}

		// Calculate new average entry price if increasing positions
		if (pos.Size > 0 && sizeChange > 0) || (pos.Size < 0 && sizeChange < 0) {
			pos.EntryPrice = (pos.Size*pos.EntryPrice + sizeChange*price) / newSize
			pos.Margin += margin
		}

		pos.Size = newSize
		if pos.Size == 0 {
			pos.EntryPrice = 0
			pos.Margin = 0
		}
	}
}

// ResetPosition safely resets a position to 0 size and margin under mutex lock to avoid data races
func (pe *PositionEngine) ResetPosition(userID, symbol string) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if userPos, ok := pe.positions[userID]; ok {
		if pos, ok := userPos[symbol]; ok {
			pos.Size = 0
			pos.Margin = 0
			pos.EntryPrice = 0
			pos.UnrealizedPnL = 0
		}
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

// GetPositionsMap returns copy of positions map
func (pe *PositionEngine) GetPositionsMap() map[string]map[string]*Position {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	res := make(map[string]map[string]*Position)
	for uID, symMap := range pe.positions {
		res[uID] = make(map[string]*Position)
		for sym, pos := range symMap {
			res[uID][sym] = pos
		}
	}
	return res
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
