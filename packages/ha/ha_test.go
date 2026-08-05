package ha

import (
	"context"
	"testing"
	"time"
)

func TestLeaderElectionAndHeartbeatFailover(t *testing.T) {
	cm := NewClusterManager()
	ctx := context.Background()

	// Register 3 Node cluster instances
	cm.RegisterNode("node_1", NodeEngine, "10.0.0.1:8081")
	cm.RegisterNode("node_2", NodeEngine, "10.0.0.2:8081")
	cm.RegisterNode("node_3", NodeEngine, "10.0.0.3:8081")

	// Trigger initial leader election
	leaderID, err := cm.RunLeaderElection(ctx)
	if err != nil {
		t.Fatalf("Leader election failed: %v", err)
	}

	leaderNode := cm.GetLeader()
	if leaderNode == nil {
		t.Fatal("Expected leader node, got nil")
	}

	if leaderNode.ID != leaderID || !leaderNode.IsLeader {
		t.Errorf("Mismatch in leader details: ID %s, statusIsLeader %v", leaderNode.ID, leaderNode.IsLeader)
	}

	// Verify heartbeats keep leader healthy (no failover)
	_ = cm.HeartbeatMonitor(leaderID)
	failedOver, err := cm.DetectTimeoutAndFailover(ctx)
	if err != nil {
		t.Fatalf("Timeout audit failed: %v", err)
	}
	if failedOver {
		t.Error("Expected no failover while leader is heartbeat active")
	}

	// Simulate leader node crash (timeout expired)
	cm.mu.Lock()
	leaderNode.LastSeenAt = time.Now().Add(-500 * time.Millisecond) // Age heartbeat
	cm.mu.Unlock()

	// Timeout audit should detect timeout and auto failover to next candidate!
	failedOver, err = cm.DetectTimeoutAndFailover(ctx)
	if err != nil {
		t.Fatalf("Timeout failover process failed: %v", err)
	}
	if !failedOver {
		t.Fatal("Expected automatic failover to trigger upon leader timeout")
	}

	newLeader := cm.GetLeader()
	if newLeader == nil {
		t.Fatal("Expected new elected leader node, got nil")
	}

	if newLeader.ID == leaderID {
		t.Errorf("Failover failure: previous crashed leader %s was re-elected", leaderID)
	}

	if !newLeader.IsLeader || newLeader.Term != 3 {
		t.Errorf("New leader state mismatch: IsLeader %v Term %d", newLeader.IsLeader, newLeader.Term)
	}
}

func TestGracefulShutdownAndConnectionDraining(t *testing.T) {
	cm := NewClusterManager()
	ctx := context.Background()

	// Simulate 15 active websockets/REST sockets connected to gateway node
	cm.SetConnectionCount(15)

	// Graceful shutdown must progressively drain active conns down to 0
	completed, err := cm.GracefulShutdownAndDrain(ctx, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("Graceful shutdown failed: %v", err)
	}

	if !completed {
		t.Error("Expected shutdown draining sequence to complete successfully")
	}

	cm.mu.Lock()
	conns := cm.activeConns
	cm.mu.Unlock()

	if conns != 0 {
		t.Errorf("Expected remaining connections to be 0, got %d", conns)
	}
}
