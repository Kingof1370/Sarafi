package connectivity

import (
	"errors"
	"testing"
)

func TestNodeManagerLatencyAndHealthSelection(t *testing.T) {
	nm := NewNodeManager()

	n1 := &NodeRecord{
		ID:          "eth_primary",
		Network:     "Ethereum",
		URL:         "https://eth-primary.velyxora.com",
		Type:        TypePrimary,
		IsAvailable: true,
	}

	n2 := &NodeRecord{
		ID:          "eth_secondary",
		Network:     "Ethereum",
		URL:         "https://eth-secondary.velyxora.com",
		Type:        TypeSecondary,
		IsAvailable: true,
	}

	nm.RegisterNode(n1)
	nm.RegisterNode(n2)

	// Measure Latencies: n1 has 50ms, n2 has 150ms
	nm.MeasureLatency("eth_primary", 50, nil)
	nm.MeasureLatency("eth_secondary", 150, nil)

	best, err := nm.SelectOptimalNode("Ethereum")
	if err != nil {
		t.Fatalf("SelectOptimalNode failed: %v", err)
	}

	if best.ID != "eth_primary" {
		t.Errorf("Expected eth_primary, got %s", best.ID)
	}

	// Trigger error on primary to test automatic failover
	nm.MeasureLatency("eth_primary", 999, errors.New("timeout connection"))

	best, err = nm.SelectOptimalNode("Ethereum")
	if err != nil {
		t.Fatalf("SelectOptimalNode failed after failover: %v", err)
	}

	if best.ID != "eth_secondary" {
		t.Errorf("Expected eth_secondary on failover, got %s", best.ID)
	}
}

func TestCircuitBreakerTripping(t *testing.T) {
	nm := NewNodeManager()

	node := &NodeRecord{
		ID:          "btc_primary",
		Network:     "Bitcoin",
		URL:         "https://btc-primary.velyxora.com",
		Type:        TypePrimary,
		IsAvailable: true,
	}
	nm.RegisterNode(node)

	// Simulate 3 failures to trip the circuit breaker
	errRPC := nm.ExecuteRPC("Bitcoin", func(url string) error {
		return errors.New("connection failed")
	})
	if errRPC == nil {
		t.Error("Expected error from RPC call")
	}

	_ = nm.ExecuteRPC("Bitcoin", func(url string) error { return errors.New("connection failed") })
	_ = nm.ExecuteRPC("Bitcoin", func(url string) error { return errors.New("connection failed") })

	// The circuit breaker should now be OPEN, blocking subsequent calls instantly
	errTripped := nm.ExecuteRPC("Bitcoin", func(url string) error {
		return nil
	})

	if errTripped == nil || errTripped.Error() == "connection failed" {
		t.Errorf("Expected circuit breaker open block, got: %v", errTripped)
	}
}

func TestBlockTrackerReorganization(t *testing.T) {
	bt := NewBlockTracker()

	// Normal header progression
	reorg, fork := bt.TrackBlock(100, "hash_100", "hash_99")
	if reorg {
		t.Error("Unexpected reorg on standard block track")
	}

	reorg, fork = bt.TrackBlock(101, "hash_101", "hash_100")
	if reorg {
		t.Error("Unexpected reorg on standard block track")
	}

	// Forked header block tracking (Track block 101 with a different previous hash)
	reorg, fork = bt.TrackBlock(101, "hash_101_fork", "hash_99_fork")
	if !reorg {
		t.Error("Expected reorganization trigger, got none")
	}

	if fork != 100 {
		t.Errorf("Expected fork block height 100, got %d", fork)
	}
}
