package engine

import (
	"sync"
)

// Position records current exposure for a user on a given symbol
type Position struct {
	UserID    string  `json:"user_id"`
	Symbol    string  `json:"symbol"`
	Size      float64 `json:"size"`      // positive for long, negative for short
	EntryPrice float64 `json:"entry_price"`
}

// PositionEngine tracks user spot or margin positions dynamically
type PositionEngine struct {
	mu        sync.RWMutex
	positions map[string]map[string]*Position // userID -> symbol -> Position
}

// NewPositionEngine creates a new exposure coordinator
func NewPositionEngine() *PositionEngine {
	return &PositionEngine{
		positions: make(map[string]map[string]*Position),
	}
}

// RecordExecution updates position sheets based on execution trade records
func (pe *PositionEngine) RecordExecution(userID, symbol string, sizeChange, price float64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if _, ok := pe.positions[userID]; !ok {
		pe.positions[userID] = make(map[string]*Position)
	}

	pos, exists := pe.positions[userID][symbol]
	if !exists {
		pos = &Position{
			UserID:     userID,
			Symbol:     symbol,
			Size:       0,
			EntryPrice: 0,
		}
		pe.positions[userID][symbol] = pos
	}

	if pos.Size == 0 {
		pos.Size = sizeChange
		pos.EntryPrice = price
	} else {
		newSize := pos.Size + sizeChange
		// If increasing the position in same direction, calculate new average entry price
		if (pos.Size > 0 && sizeChange > 0) || (pos.Size < 0 && sizeChange < 0) {
			pos.EntryPrice = (pos.Size*pos.EntryPrice + sizeChange*price) / newSize
		}
		pos.Size = newSize
	}
}

// GetPosition retrieves position metrics
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
