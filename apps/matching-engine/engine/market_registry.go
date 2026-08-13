package engine

import (
	"sync"
)

// MarketRegistry manages isolated, concurrent, and thread-safe Matcher order books per trading pair
type MarketRegistry struct {
	mu       sync.RWMutex
	matchers map[string]*Matcher
}

// NewMarketRegistry initializes an empty thread-safe market registry
func NewMarketRegistry() *MarketRegistry {
	return &MarketRegistry{
		matchers: make(map[string]*Matcher),
	}
}

// GetMatcher retrieves the isolated Matcher for a trading symbol, dynamically instantiating it if absent
func (mr *MarketRegistry) GetMatcher(symbol string) *Matcher {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	matcher, exists := mr.matchers[symbol]
	if !exists {
		matcher = NewMatcher(symbol)
		mr.matchers[symbol] = matcher
	}
	return matcher
}

// RegisterMatcher registers a pre-configured Matcher instance under a specific trading symbol
func (mr *MarketRegistry) RegisterMatcher(symbol string, matcher *Matcher) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.matchers[symbol] = matcher
}

// GetRegisteredSymbols returns a copy of all active symbols tracked in the registry
func (mr *MarketRegistry) GetRegisteredSymbols() []string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	symbols := make([]string, 0, len(mr.matchers))
	for sym := range mr.matchers {
		symbols = append(symbols, sym)
	}
	return symbols
}
