package connectivity

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// NodeType defines the priority tier of a blockchain node
type NodeType string

const (
	TypePrimary   NodeType = "PRIMARY"
	TypeSecondary NodeType = "SECONDARY"
	TypeFallback  NodeType = "FALLBACK"
)

// CircuitState defines the operational threshold of a circuit breaker
type CircuitState string

const (
	StateClosed   CircuitState = "CLOSED"
	StateOpen     CircuitState = "OPEN"
	StateHalfOpen CircuitState = "HALF_OPEN"
)

// NodeRecord holds performance metrics, health scores, and statuses for each node endpoint
type NodeRecord struct {
	ID             string       `json:"id"`
	Network        string       `json:"network"`
	URL            string       `json:"url"`
	Type           NodeType     `json:"type"`
	LatencyMS      int64        `json:"latency_ms"`
	IsAvailable    bool         `json:"is_available"`
	IsSynced       bool         `json:"is_synced"`
	HealthScore    float64      `json:"health_score"` // scale 0.0 to 1.0
	FailedRequests int          `json:"failed_requests"`
	CircuitState   CircuitState `json:"circuit_state"`
	StateChangedAt time.Time    `json:"state_changed_at"`
}

// CircuitBreaker wraps RPC clients to trip connections and fail-over immediately on errors
type CircuitBreaker struct {
	mu           sync.Mutex
	state        CircuitState
	failures     int
	threshold    int
	cooldownMins int
	lastFailure  time.Time
}

func NewCircuitBreaker(threshold int, cooldownMins int) *CircuitBreaker {
	return &CircuitBreaker{
		state:        StateClosed,
		threshold:    threshold,
		cooldownMins: cooldownMins,
	}
}

func (cb *CircuitBreaker) Execute(f func() error) error {
	cb.mu.Lock()
	// Check cooldown reset
	if cb.state == StateOpen && time.Since(cb.lastFailure) > time.Duration(cb.cooldownMins)*time.Minute {
		cb.state = StateHalfOpen
	}

	if cb.state == StateOpen {
		cb.mu.Unlock()
		return errors.New("circuit breaker is tripped (OPEN state)")
	}
	cb.mu.Unlock()

	err := f()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()
		if cb.failures >= cb.threshold {
			cb.state = StateOpen
		}
		return err
	}

	// Reset on success
	cb.failures = 0
	cb.state = StateClosed
	return nil
}

// NodeManager manages primary/secondary pools, measuring performance metrics and routing to the optimal healthy endpoint
type NodeManager struct {
	mu        sync.RWMutex
	nodes     map[string][]*NodeRecord // key: Network
	breakers  map[string]*CircuitBreaker // key: NodeID
	auditLogs []string
}

// NewNodeManager instantiates the multi-chain Connectivity orchestration layer
func NewNodeManager() *NodeManager {
	return &NodeManager{
		nodes:     make(map[string][]*NodeRecord),
		breakers:  make(map[string]*CircuitBreaker),
		auditLogs: make([]string, 0),
	}
}

// RegisterNode provisions a node endpoint for health monitoring
func (nm *NodeManager) RegisterNode(node *NodeRecord) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.nodes[node.Network] = append(nm.nodes[node.Network], node)
	nm.breakers[node.ID] = NewCircuitBreaker(3, 5) // Trip on 3 errors, cooldown 5 mins

	nm.auditLogs = append(nm.auditLogs, fmt.Sprintf("[%s] NODE REGISTERED: ID %s, url %s on %s", time.Now().Format(time.RFC3339), node.ID, node.URL, node.Network))
}

// MeasureLatency simulates ping tests and recalculates the node's health score
func (nm *NodeManager) MeasureLatency(nodeID string, latency int64, err error) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	for _, list := range nm.nodes {
		for _, n := range list {
			if n.ID == nodeID {
				n.LatencyMS = latency
				if err != nil {
					n.FailedRequests++
					n.IsAvailable = false
				} else {
					n.FailedRequests = 0
					n.IsAvailable = true
				}

				// Compute Score: Latency penalty and failure penalties
				score := 1.0 - (float64(latency) / 1000.0) // Deduct score based on latency (max 1s threshold)
				if score < 0 {
					score = 0.0
				}
				if !n.IsAvailable {
					score = 0.0
				}
				n.HealthScore = score

				// Sync circuit state
				breaker := nm.breakers[nodeID]
				if breaker != nil {
					n.CircuitState = breaker.state
				}
				return
			}
		}
	}
}

// SelectOptimalNode chooses the highest health score node, failing over automatically to fallback nodes if primary trips
func (nm *NodeManager) SelectOptimalNode(network string) (*NodeRecord, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	list, exists := nm.nodes[network]
	if !exists || len(list) == 0 {
		return nil, fmt.Errorf("no registered nodes found for network %s", network)
	}

	var best *NodeRecord
	for _, n := range list {
		if n.IsAvailable && n.CircuitState != StateOpen {
			if best == nil || n.HealthScore > best.HealthScore {
				best = n
			}
		}
	}

	if best == nil {
		// Attempt fallback nodes directly
		for _, n := range list {
			if n.Type == TypeFallback && n.CircuitState != StateOpen {
				return n, nil
			}
		}
		return nil, fmt.Errorf("all nodes on network %s are offline / circuit-tripped", network)
	}

	return best, nil
}

// ExecuteRPC wraps RPC requests securely with circuit breaker logic and handles failover redirects
func (nm *NodeManager) ExecuteRPC(network string, f func(url string) error) error {
	node, err := nm.SelectOptimalNode(network)
	if err != nil {
		return err
	}

	breaker := nm.breakers[node.ID]
	errRPC := breaker.Execute(func() error {
		return f(node.URL)
	})

	if errRPC != nil {
		nm.mu.Lock()
		node.FailedRequests++
		if breaker.state == StateOpen {
			node.CircuitState = StateOpen
			nm.auditLogs = append(nm.auditLogs, fmt.Sprintf("[%s] CIRCUIT BREAKER TRIPPED FOR NODE %s on %s", time.Now().Format(time.RFC3339), node.ID, network))
		}
		nm.mu.Unlock()

		nm.MeasureLatency(node.ID, 999, errRPC)

		// Failover retry on different optimal node
		nextNode, errFailover := nm.SelectOptimalNode(network)
		if errFailover == nil && nextNode.ID != node.ID {
			nm.auditLogs = append(nm.auditLogs, fmt.Sprintf("[%s] FAILOVER RETRY: routed from %s to %s", time.Now().Format(time.RFC3339), node.ID, nextNode.ID))
			nextBreaker := nm.breakers[nextNode.ID]
			return nextBreaker.Execute(func() error {
				return f(nextNode.URL)
			})
		}
		return errRPC
	}

	return nil
}

// BlockTracker parses chain heights and detects chain reorganizations dynamically
type BlockTracker struct {
	mu           sync.Mutex
	LatestBlock  int64
	CachedBlocks map[int64]string // key: BlockNumber, value: BlockHash
	reorgChannel chan int64
}

func NewBlockTracker() *BlockTracker {
	return &BlockTracker{
		CachedBlocks: make(map[int64]string),
		reorgChannel: make(chan int64, 10),
	}
}

// TrackBlock ingresses block headers and returns the fork height if a chain reorganization is detected
func (bt *BlockTracker) TrackBlock(height int64, hash string, prevHash string) (bool, int64) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	if height > bt.LatestBlock {
		bt.LatestBlock = height
	}

	// Check if previous block hash matches cached hash
	expectedPrev, exists := bt.CachedBlocks[height-1]
	if exists && expectedPrev != prevHash {
		// Chain Reorganization detected! Let's find the fork height
		forkHeight := height - 1
		for forkHeight > 0 {
			if _, existsFork := bt.CachedBlocks[forkHeight]; !existsFork {
				break
			}
			forkHeight--
		}
		return true, forkHeight + 1
	}

	bt.CachedBlocks[height] = hash
	return false, 0
}

func (nm *NodeManager) ListNodes() []*NodeRecord {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	var all []*NodeRecord
	for _, list := range nm.nodes {
		all = append(all, list...)
	}
	return all
}
