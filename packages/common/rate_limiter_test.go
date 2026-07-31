package common

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(2, 500*time.Millisecond)
	key := "127.0.0.1"

	if !rl.Allow(key) {
		t.Error("First request should be allowed")
	}

	if !rl.Allow(key) {
		t.Error("Second request should be allowed")
	}

	if rl.Allow(key) {
		t.Error("Third request should exceed the limit and be blocked")
	}

	time.Sleep(600 * time.Millisecond)

	if !rl.Allow(key) {
		t.Error("Request should be allowed again after window expires")
	}
}
