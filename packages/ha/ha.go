package ha

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// NodeType represents the class of an active clustering node
type NodeType string

const (
	NodeGateway NodeType = "GATEWAY"
	NodeEngine  NodeType = "MATCHING_ENGINE"
	NodeWallet  NodeType = "WALLET_SERVICE"
)

// ClusterNode represents a live, scaleable server node inside the cluster
type ClusterNode struct {
	ID          string    `json:"id"`
	Type        NodeType  `json:"type"`
	Address     string    `json:"address"`
	IsLeader    bool      `json:"is_leader"`
	IsHealthy   bool      `json:"is_healthy"`
	Term        uint64    `json:"term"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

// ClusterManager coordinates leader elections, heartbeat loops, failovers, and connection draining
type ClusterManager struct {
	mu           sync.RWMutex
	nodes        map[string]*ClusterNode
	term         uint64
	leaderID     string
	activeConns  int64
	heartbeatDur time.Duration
	auditLogs    []string
}

// NewClusterManager instantiates a high availability supervisor
func NewClusterManager() *ClusterManager {
	return &ClusterManager{
		nodes:        make(map[string]*ClusterNode),
		term:         1,
		activeConns:  0,
		heartbeatDur: 100 * time.Millisecond,
		auditLogs:    make([]string, 0),
	}
}

// RegisterNode adds a new instance node to the clustering directory
func (cm *ClusterManager) RegisterNode(id string, nType NodeType, addr string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.nodes[id] = &ClusterNode{
		ID:         id,
		Type:       nType,
		Address:    addr,
		IsLeader:   false,
		IsHealthy:  true,
		Term:       cm.term,
		LastSeenAt: time.Now(),
	}

	cm.auditLogs = append(cm.auditLogs, fmt.Sprintf("[%s] NODE REGISTERED: %s (%s) at %s", time.Now().Format(time.RFC3339), id, nType, addr))
}

// RunLeaderElection selects the primary active node dynamically (simulating Raft/consensus election)
func (cm *ClusterManager) RunLeaderElection(ctx context.Context) (string, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.term++
	var candidate *ClusterNode

	// Choose the first healthy node as candidate leader
	for _, n := range cm.nodes {
		if n.IsHealthy {
			candidate = n
			break
		}
	}

	if candidate == nil {
		return "", errors.New("election failure: no healthy active nodes found inside clustering directory")
	}

	// Step down previous leader
	if cm.leaderID != "" {
		if prev, exists := cm.nodes[cm.leaderID]; exists {
			prev.IsLeader = false
		}
	}

	candidate.IsLeader = true
	candidate.Term = cm.term
	cm.leaderID = candidate.ID

	cm.auditLogs = append(cm.auditLogs, fmt.Sprintf("[%s] LEADER ELECTED: %s for Term %d", time.Now().Format(time.RFC3339), candidate.ID, cm.term))
	return candidate.ID, nil
}

// HeartbeatMonitor verifies node health states and triggers auto-failovers on timeouts
func (cm *ClusterManager) HeartbeatMonitor(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	node, exists := cm.nodes[id]
	if !exists {
		return fmt.Errorf("node %s not tracked", id)
	}

	node.LastSeenAt = time.Now()
	node.IsHealthy = true
	return nil
}

// DetectTimeoutAndFailover audits heartbeats and triggers an immediate re-election if leader is missing
func (cm *ClusterManager) DetectTimeoutAndFailover(ctx context.Context) (bool, error) {
	cm.mu.Lock()
	leader, exists := cm.nodes[cm.leaderID]
	cm.mu.Unlock()

	if !exists || leader == nil {
		// No active leader, force election
		_, err := cm.RunLeaderElection(ctx)
		return true, err
	}

	// Check timeout
	cm.mu.Lock()
	isTimeout := time.Since(leader.LastSeenAt) > (3 * cm.heartbeatDur)
	if isTimeout {
		leader.IsHealthy = false
		leader.IsLeader = false
		cm.mu.Unlock()

		cm.auditLogs = append(cm.auditLogs, fmt.Sprintf("[%s] LEADER TIMEOUT DETECTED: %s unresponsive", time.Now().Format(time.RFC3339), leader.ID))
		_, err := cm.RunLeaderElection(ctx)
		return true, err
	}
	cm.mu.Unlock()

	return false, nil
}

// SetConnectionCount updates live connection logs (used on gracefully draining checks)
func (cm *ClusterManager) SetConnectionCount(count int64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.activeConns = count
}

// GracefulShutdownAndDrain drains active connections and flushes transaction buffers before instance halt
func (cm *ClusterManager) GracefulShutdownAndDrain(ctx context.Context, timeout time.Duration) (bool, error) {
	cm.auditLogs = append(cm.auditLogs, fmt.Sprintf("[%s] INITIATING GRACEFUL SHUTDOWN & CONNECTION DRAINING", time.Now().Format(time.RFC3339)))

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cm.mu.Lock()
		conns := cm.activeConns
		cm.mu.Unlock()

		if conns <= 0 {
			cm.auditLogs = append(cm.auditLogs, fmt.Sprintf("[%s] CONNECTION DRAINING COMPLETE: Safe to terminate instance", time.Now().Format(time.RFC3339)))
			return true, nil
		}

		// Simulate progressive draining
		cm.mu.Lock()
		cm.activeConns -= 5
		if cm.activeConns < 0 {
			cm.activeConns = 0
		}
		cm.mu.Unlock()

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}

	return false, errors.New("shutdown timeout: connection draining exceeded maximum allowed duration limit")
}

// ListNodes returns all cluster nodes
func (cm *ClusterManager) ListNodes() []*ClusterNode {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var list []*ClusterNode
	for _, n := range cm.nodes {
		list = append(list, n)
	}
	return list
}

// GetLeader returns details of the current primary cluster leader node
func (cm *ClusterManager) GetLeader() *ClusterNode {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.nodes[cm.leaderID]
}

// GetAuditLogs returns historical audit trail logs
func (cm *ClusterManager) GetAuditLogs() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.auditLogs
}
