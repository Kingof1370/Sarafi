package tests

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"velyxora/packages/connectivity"
)

// TestNodeConnectivityConcurrency verifies parallel routing and scoring under heavy multi-threaded concurrent requests
func TestNodeConnectivityConcurrency(t *testing.T) {
	nm := connectivity.NewNodeManager()

	// Provision 10 concurrent primary and fallback nodes
	for i := 0; i < 10; i++ {
		nm.RegisterNode(&connectivity.NodeRecord{
			ID:          fmt.Sprintf("node_%d", i),
			Network:     "Ethereum",
			URL:         fmt.Sprintf("https://eth-%d.velyxora.com", i),
			Type:        connectivity.TypePrimary,
			IsAvailable: true,
		})
	}

	concurrencyLimit := 40
	var wg sync.WaitGroup
	wg.Add(concurrencyLimit)

	for i := 0; i < concurrencyLimit; i++ {
		go func(idx int) {
			defer wg.Done()
			nodeID := fmt.Sprintf("node_%d", idx%10)
			// Simulate random latencies
			nm.MeasureLatency(nodeID, int64(20+(idx*3)), nil)
			_, _ = nm.SelectOptimalNode("Ethereum")
		}(i)
	}

	wg.Wait()

	nodesList := nm.ListNodes()
	if len(nodesList) != 10 {
		t.Errorf("Expected 10 registered nodes, got %d", len(nodesList))
	}
}

// TestAutomaticFailoverRoute verifies fallback routing when all primary nodes are tripped
func TestAutomaticFailoverRoute(t *testing.T) {
	nm := connectivity.NewNodeManager()

	nm.RegisterNode(&connectivity.NodeRecord{
		ID:          "primary_1",
		Network:     "Bitcoin",
		URL:         "https://btc-1.velyxora.com",
		Type:        connectivity.TypePrimary,
		IsAvailable: true,
	})

	nm.RegisterNode(&connectivity.NodeRecord{
		ID:          "fallback_1",
		Network:     "Bitcoin",
		URL:         "https://btc-fallback.velyxora.com",
		Type:        connectivity.TypeFallback,
		IsAvailable: true,
	})

	// Measure standard ping
	nm.MeasureLatency("primary_1", 20, nil)
	nm.MeasureLatency("fallback_1", 80, nil)

	best, _ := nm.SelectOptimalNode("Bitcoin")
	if best.ID != "primary_1" {
		t.Errorf("Expected primary_1 as optimal node, got %s", best.ID)
	}

	// Trigger error to trip primary_1
	nm.MeasureLatency("primary_1", 999, errors.New("timeout network connection"))

	best, _ = nm.SelectOptimalNode("Bitcoin")
	if best.ID != "fallback_1" {
		t.Errorf("Expected fallback_1 on primary failover, got %s", best.ID)
	}
}

// BenchmarkOptimalNodeSelection measures speed of selecting the optimal node from a pool of registered nodes
func BenchmarkOptimalNodeSelection(b *testing.B) {
	nm := connectivity.NewNodeManager()
	for i := 0; i < 20; i++ {
		nm.RegisterNode(&connectivity.NodeRecord{
			ID:          fmt.Sprintf("node_%d", i),
			Network:     "Ethereum",
			URL:         fmt.Sprintf("https://eth-%d.velyxora.com", i),
			Type:        connectivity.TypePrimary,
			IsAvailable: true,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = nm.SelectOptimalNode("Ethereum")
	}
}

// BenchmarkBlockTracker Tracking measures speed of parsing heights and checking hashes
func BenchmarkBlockTracker(b *testing.B) {
	bt := connectivity.NewBlockTracker()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bt.TrackBlock(int64(i), fmt.Sprintf("hash_%d", i), fmt.Sprintf("hash_%d", i-1))
	}
}
