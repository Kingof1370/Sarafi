package engine

import (
	"sync"
)

// Ticker holds 24-hour pricing stats for a symbol
type Ticker struct {
	Symbol      string  `json:"symbol"`
	LastPrice   float64 `json:"last_price"`
	High24H     float64 `json:"high_24h"`
	Low24H      float64 `json:"low_24h"`
	Volume24H   float64 `json:"volume_24h"`
	Change24H   float64 `json:"change_24h"` // percent change
}

// MarketServices processes rolling ticker and depth feeds
type MarketServices struct {
	mu      sync.RWMutex
	tickers map[string]*Ticker
}

// NewMarketServices creates a new market tracking facility
func NewMarketServices() *MarketServices {
	return &MarketServices{
		tickers: make(map[string]*Ticker),
	}
}

// RecordTrade updates symbol tickers based on trade execution pricing
func (ms *MarketServices) RecordTrade(symbol string, price, quantity float64) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	tick, exists := ms.tickers[symbol]
	if !exists {
		tick = &Ticker{
			Symbol:    symbol,
			LastPrice: price,
			High24H:   price,
			Low24H:    price,
			Volume24H: quantity,
		}
		ms.tickers[symbol] = tick
		return
	}

	tick.LastPrice = price
	tick.Volume24H += quantity

	if price > tick.High24H {
		tick.High24H = price
	}
	if price < tick.Low24H {
		tick.Low24H = price
	}
}

// GetTicker retrieves rolling 24H stats
func (ms *MarketServices) GetTicker(symbol string) *Ticker {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	tick, exists := ms.tickers[symbol]
	if !exists {
		return &Ticker{Symbol: symbol}
	}
	return tick
}
