package common

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	rate     int           // requests allowed
	window   time.Duration // timeframe window
}

func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		rate:     rate,
		window:   window,
	}
}

// Allow checks if the given key (e.g. client IP or User ID) is allowed to proceed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Prune old timestamps
	var current []time.Time
	for _, t := range rl.requests[key] {
		if t.After(cutoff) {
			current = append(current, t)
		}
	}

	if len(current) >= rl.rate {
		rl.requests[key] = current
		return false
	}

	current = append(current, now)
	rl.requests[key] = current
	return true
}
