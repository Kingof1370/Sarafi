package engine

import (
	"sync"
	"time"
	"velyxora/packages/types"
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

type Candle struct {
	Symbol      string    `json:"symbol"`
	Interval    string    `json:"interval"` // "1m", "5m", "15m", "1h", "1d"
	OpenTime    time.Time `json:"open_time"`
	CloseTime   time.Time `json:"close_time"`
	Open        float64   `json:"open"`
	High        float64   `json:"high"`
	Low         float64   `json:"low"`
	Close       float64   `json:"close"`
	Volume      float64   `json:"volume"`
	QuoteVolume float64   `json:"quote_volume"`
	TradeCount  int       `json:"trade_count"`
}

// MarketServices processes rolling ticker and depth feeds
type MarketServices struct {
	mu      sync.RWMutex
	tickers map[string]*Ticker
	candles map[string]map[string][]*Candle // symbol -> interval -> candles
	trades  map[string][]*types.Trade      // symbol -> recent trades
}

// NewMarketServices creates a new market tracking facility
func NewMarketServices() *MarketServices {
	return &MarketServices{
		tickers: make(map[string]*Ticker),
		candles: make(map[string]map[string][]*Candle),
		trades:  make(map[string][]*types.Trade),
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

// AddTrade records real-time trade, updates ticker, and builds OHLCV candles
func (ms *MarketServices) AddTrade(symbol string, trade *types.Trade) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// 1. Update Ticker
	tick, exists := ms.tickers[symbol]
	if !exists {
		tick = &Ticker{
			Symbol:    symbol,
			LastPrice: trade.Price,
			High24H:   trade.Price,
			Low24H:    trade.Price,
			Volume24H: trade.Quantity,
		}
		ms.tickers[symbol] = tick
	} else {
		tick.LastPrice = trade.Price
		tick.Volume24H += trade.Quantity
		if trade.Price > tick.High24H {
			tick.High24H = trade.Price
		}
		if trade.Price < tick.Low24H {
			tick.Low24H = trade.Price
		}
	}

	// 2. Track Recent Trades
	ms.trades[symbol] = append(ms.trades[symbol], trade)
	if len(ms.trades[symbol]) > 100 {
		ms.trades[symbol] = ms.trades[symbol][1:]
	}

	// 3. Update OHLCV Candles
	intervals := []string{"1m", "5m", "15m", "1h", "1d"}
	for _, interval := range intervals {
		ms.updateCandle(symbol, interval, trade)
	}
}

func (ms *MarketServices) updateCandle(symbol, interval string, trade *types.Trade) {
	var duration time.Duration
	switch interval {
	case "1m":
		duration = time.Minute
	case "5m":
		duration = 5 * time.Minute
	case "15m":
		duration = 15 * time.Minute
	case "1h":
		duration = time.Hour
	case "1d":
		duration = 24 * time.Hour
	default:
		duration = time.Minute
	}

	openTime := trade.Timestamp.Truncate(duration)
	closeTime := openTime.Add(duration)

	if ms.candles[symbol] == nil {
		ms.candles[symbol] = make(map[string][]*Candle)
	}

	list := ms.candles[symbol][interval]
	var last *Candle
	if len(list) > 0 {
		last = list[len(list)-1]
	}

	if last != nil && last.OpenTime.Equal(openTime) {
		last.Close = trade.Price
		if trade.Price > last.High {
			last.High = trade.Price
		}
		if trade.Price < last.Low {
			last.Low = trade.Price
		}
		last.Volume += trade.Quantity
		last.QuoteVolume += trade.Price * trade.Quantity
		last.TradeCount++
	} else {
		newCandle := &Candle{
			Symbol:      symbol,
			Interval:    interval,
			OpenTime:    openTime,
			CloseTime:   closeTime,
			Open:        trade.Price,
			High:        trade.Price,
			Low:         trade.Price,
			Close:       trade.Price,
			Volume:      trade.Quantity,
			QuoteVolume: trade.Price * trade.Quantity,
			TradeCount:  1,
		}
		ms.candles[symbol][interval] = append(list, newCandle)
		if len(ms.candles[symbol][interval]) > 1000 {
			ms.candles[symbol][interval] = ms.candles[symbol][interval][1:]
		}
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

// GetRecentTrades fetches recent executed trades
func (ms *MarketServices) GetRecentTrades(symbol string) []*types.Trade {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	trades := ms.trades[symbol]
	if trades == nil {
		return []*types.Trade{}
	}
	return trades
}

// GetCandles fetches OHLCV candlestick interval buckets
func (ms *MarketServices) GetCandles(symbol, interval string) []*Candle {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	candles := ms.candles[symbol][interval]
	if candles == nil {
		return []*Candle{}
	}
	return candles
}
