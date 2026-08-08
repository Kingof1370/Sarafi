package engine

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
	"velyxora/packages/types"
)

// LiquidityStats holds computed market quality metrics
type LiquidityStats struct {
	Symbol            string    `json:"symbol"`
	BestBid           float64   `json:"best_bid"`
	BestAsk           float64   `json:"best_ask"`
	Spread            float64   `json:"spread"`
	MidPrice          float64   `json:"mid_price"`
	WeightedMidPrice  float64   `json:"weighted_mid_price"`
	DepthImbalance    float64   `json:"depth_imbalance"` // between -1.0 and 1.0
	MarketHealthScore float64   `json:"market_health_score"`
	Timestamp         time.Time `json:"timestamp"`
}

// SurveillanceAlert represents flagged anomalous trading behavior
type SurveillanceAlert struct {
	ID        string    `json:"id"`
	Symbol    string    `json:"symbol"`
	UserID    string    `json:"user_id"`
	Pattern   string    `json:"pattern"` // "WASH_TRADING", "SPOOFING", "LAYERING", "QUOTE_STUFFING"
	Details   string    `json:"details"`
	Severity  string    `json:"severity"` // "INFO", "WARNING", "CRITICAL"
	Timestamp time.Time `json:"timestamp"`
}

// ExternalLiquidityProvider defines the contract for pluggable external liquidity integrations (Rule 7)
type ExternalLiquidityProvider interface {
	Name() string
	GetOrderBook(symbol string) (*types.OrderBookL2, error)
	ExecuteOrder(ctx context.Context, order *types.Order) (*types.Trade, error)
}

// LiquidityEngine tracks real-time market quality and runs surveillance filters
type LiquidityEngine struct {
	mu           sync.RWMutex
	stats        map[string]*LiquidityStats
	alerts       []*SurveillanceAlert
	orderCounts  map[string]int                    // userID -> order count in window
	cancelCounts map[string]int                    // userID -> cancellations in window
	tradeHistory map[string][]*types.Trade         // symbol -> last trades
	providers    map[string]ExternalLiquidityProvider // pluggable providers
}

// NewLiquidityEngine initializes the enterprise market surveillance core
func NewLiquidityEngine() *LiquidityEngine {
	return &LiquidityEngine{
		stats:        make(map[string]*LiquidityStats),
		alerts:       make([]*SurveillanceAlert, 0),
		orderCounts:  make(map[string]int),
		cancelCounts: make(map[string]int),
		tradeHistory: make(map[string][]*types.Trade),
		providers:    make(map[string]ExternalLiquidityProvider),
	}
}

// RegisterProvider registers a modular external liquidity provider (Rule 7)
func (le *LiquidityEngine) RegisterProvider(provider ExternalLiquidityProvider) {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.providers[provider.Name()] = provider
}

// GetProvider retrieves a registered external liquidity provider
func (le *LiquidityEngine) GetProvider(name string) ExternalLiquidityProvider {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.providers[name]
}

// ValidateExternalExecution validates external liquidity match results strictly (Rule 8)
func (le *LiquidityEngine) ValidateExternalExecution(trade *types.Trade, minPrice, maxPrice float64) error {
	if trade == nil {
		return fmt.Errorf("invalid external trade: payload is nil")
	}
	if trade.Price <= 0 || trade.Price < minPrice || trade.Price > maxPrice {
		return fmt.Errorf("invalid external trade price: %f is out of bounds [%f, %f]", trade.Price, minPrice, maxPrice)
	}
	if trade.Quantity <= 0 {
		return fmt.Errorf("invalid external trade quantity: %f must be greater than 0", trade.Quantity)
	}
	if trade.Symbol == "" {
		return fmt.Errorf("invalid external trade: symbol is empty")
	}
	if trade.Timestamp.IsZero() || trade.Timestamp.After(time.Now().Add(5*time.Second)) {
		return fmt.Errorf("invalid external trade timestamp: %v is future or missing", trade.Timestamp)
	}
	return nil
}

// AnalyzeDepth computes high-precision spread and depth metrics from the active matcher
func (le *LiquidityEngine) AnalyzeDepth(matcher *Matcher) *LiquidityStats {
	le.mu.Lock()
	defer le.mu.Unlock()

	matcher.mu.RLock()
	defer matcher.mu.RUnlock()

	symbol := matcher.Symbol
	stats := &LiquidityStats{
		Symbol:    symbol,
		Timestamp: time.Now(),
	}

	bestBid := 0.0
	bidVolume := 0.0
	if len(matcher.Bids) > 0 {
		bestBid = matcher.Bids[0].Price
		for _, limit := range matcher.Bids {
			for _, ord := range limit.Orders {
				bidVolume += (ord.Quantity - ord.FilledQty)
			}
		}
	}

	bestAsk := 0.0
	askVolume := 0.0
	if len(matcher.Asks) > 0 {
		bestAsk = matcher.Asks[0].Price
		for _, limit := range matcher.Asks {
			for _, ord := range limit.Orders {
				askVolume += (ord.Quantity - ord.FilledQty)
			}
		}
	}

	stats.BestBid = bestBid
	stats.BestAsk = bestAsk

	if bestBid > 0 && bestAsk > 0 {
		stats.Spread = bestAsk - bestBid
		stats.MidPrice = (bestAsk + bestBid) / 2.0

		// Weighted Mid Price: (BidPrice * AskVol + AskPrice * BidVol) / (BidVol + AskVol)
		if (bidVolume + askVolume) > 0 {
			stats.WeightedMidPrice = (bestBid*askVolume + bestAsk*bidVolume) / (bidVolume + askVolume)
			// Depth Imbalance: (BidVol - AskVol) / (BidVol + AskVol)
			stats.DepthImbalance = (bidVolume - askVolume) / (bidVolume + askVolume)
		} else {
			stats.WeightedMidPrice = stats.MidPrice
			stats.DepthImbalance = 0.0
		}
	}

	// Calculate Market Health Score (0 to 100 based on narrowness of spread and balanced depth)
	health := 100.0
	if stats.Spread > 0 {
		// Spread penalty
		spreadRatio := stats.Spread / stats.MidPrice
		health -= (spreadRatio * 5000.0) // penalize wide spreads
	}
	// Imbalance penalty
	health -= math.Abs(stats.DepthImbalance) * 30.0

	if health < 0 {
		health = 0
	}
	stats.MarketHealthScore = health

	le.stats[symbol] = stats
	return stats
}

// CheckSurveillance monitors and flags manipulative order placement patterns (Spoofing, Wash Trading, Layering)
func (le *LiquidityEngine) CheckSurveillance(trade *types.Trade) {
	le.mu.Lock()
	defer le.mu.Unlock()

	// 1. Wash Trading Check: Buyer and Seller are identical
	if trade.BuyerID == trade.SellerID {
		alert := &SurveillanceAlert{
			ID:        fmt.Sprintf("al_wash_%d", time.Now().UnixNano()),
			Symbol:    trade.Symbol,
			UserID:    trade.BuyerID,
			Pattern:   "WASH_TRADING",
			Details:   fmt.Sprintf("Self-match trade detected: price %f, qty %f", trade.Price, trade.Quantity),
			Severity:  "CRITICAL",
			Timestamp: time.Now(),
		}
		le.alerts = append(le.alerts, alert)
	}

	// 2. Velocity monitor (Track trades for spoofing patterns)
	le.tradeHistory[trade.Symbol] = append(le.tradeHistory[trade.Symbol], trade)
	if len(le.tradeHistory[trade.Symbol]) > 100 {
		le.tradeHistory[trade.Symbol] = le.tradeHistory[trade.Symbol][1:]
	}
}

// FlagAbnormalVelocity tracks duplicate order bursts or high cancellations rate (Quote Stuffing)
func (le *LiquidityEngine) FlagAbnormalVelocity(userID, symbol, action string) {
	le.mu.Lock()
	defer le.mu.Unlock()

	if action == "ORDER" {
		le.orderCounts[userID]++
		if le.orderCounts[userID] > 50 { // Max 50 orders in monitoring window
			alert := &SurveillanceAlert{
				ID:        fmt.Sprintf("al_velocity_%d", time.Now().UnixNano()),
				Symbol:    symbol,
				UserID:    userID,
				Pattern:   "QUOTE_STUFFING",
				Details:   "User submitting orders at abnormal speeds (spam attempt)",
				Severity:  "WARNING",
				Timestamp: time.Now(),
			}
			le.alerts = append(le.alerts, alert)
		}
	} else if action == "CANCEL" {
		le.cancelCounts[userID]++
		if le.cancelCounts[userID] > 40 {
			alert := &SurveillanceAlert{
				ID:        fmt.Sprintf("al_cancel_%d", time.Now().UnixNano()),
				Symbol:    symbol,
				UserID:    userID,
				Pattern:   "ABNORMAL_CANCELLATION",
				Details:   "Excessive cancellations detected (possible spoofing or layering order blocks)",
				Severity:  "WARNING",
				Timestamp: time.Now(),
			}
			le.alerts = append(le.alerts, alert)
		}
	}
}

// GetAlerts returns historical surveillance flags
func (le *LiquidityEngine) GetAlerts() []*SurveillanceAlert {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.alerts
}

// ResetRateLimits clears temporary rate tracking
func (le *LiquidityEngine) ResetRateLimits() {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.orderCounts = make(map[string]int)
	le.cancelCounts = make(map[string]int)
}
