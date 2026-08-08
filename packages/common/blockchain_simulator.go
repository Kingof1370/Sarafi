package common

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// SimTransaction represents a stateful simulated transaction on our mock blockchain
type SimTransaction struct {
	Hash        string    `json:"hash"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Amount      float64   `json:"amount"`
	Asset       string    `json:"asset"`
	BlockHeight int64     `json:"block_height"`
	BlockHash   string    `json:"block_hash"`
	Status      string    `json:"status"` // "PENDING", "SUCCESS", "FAILED"
	Timestamp   time.Time `json:"timestamp"`
}

// SimBlock represents a block on our simulated chain
type SimBlock struct {
	Height       int64             `json:"height"`
	Hash         string            `json:"hash"`
	ParentHash   string            `json:"parent_hash"`
	Transactions []*SimTransaction `json:"transactions"`
	Timestamp    time.Time         `json:"timestamp"`
}

// BlockchainSimulator manages simulated blockchain state for testing and development
type BlockchainSimulator struct {
	mu           sync.RWMutex
	network      string // "BTC", "ETH", "SOL"
	currentBlock int64
	blocks       map[int64]*SimBlock
	txMap        map[string]*SimTransaction
	balances     map[string]map[string]float64 // address -> asset -> balance
	pendingTxs   []*SimTransaction
}

var (
	simulators   = make(map[string]*BlockchainSimulator)
	simulatorsMu sync.Mutex
)

// GetSimulator returns the singleton simulator instance for a given network
func GetSimulator(network string) *BlockchainSimulator {
	simulatorsMu.Lock()
	defer simulatorsMu.Unlock()

	net := network
	if sim, ok := simulators[net]; ok {
		return sim
	}

	sim := &BlockchainSimulator{
		network:      net,
		currentBlock: 100, // start at a non-zero block height
		blocks:       make(map[int64]*SimBlock),
		txMap:        make(map[string]*SimTransaction),
		balances:     make(map[string]map[string]float64),
		pendingTxs:   make([]*SimTransaction, 0),
	}

	// Create genesis/initial blocks
	parentHash := "genesis_hash_" + net
	for i := int64(1); i <= 100; i++ {
		hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%s_block_%d_%s", net, i, parentHash)))
		hash := hex.EncodeToString(hashBytes[:])
		block := &SimBlock{
			Height:       i,
			Hash:         hash,
			ParentHash:   parentHash,
			Transactions: []*SimTransaction{},
			Timestamp:    time.Now().Add(time.Duration(-101+i) * time.Minute),
		}
		sim.blocks[i] = block
		parentHash = hash
	}

	simulators[net] = sim
	return sim
}

// SetBalance sets simulated balance for an address
func (s *BlockchainSimulator) SetBalance(address string, asset string, balance float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.balances[address]; !ok {
		s.balances[address] = make(map[string]float64)
	}
	s.balances[address][asset] = RoundToPrecision(balance, 8)
}

// GetBalance returns simulated balance
func (s *BlockchainSimulator) GetBalance(address string, asset string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if assets, ok := s.balances[address]; ok {
		if bal, ok := assets[asset]; ok {
			return bal
		}
	}
	return 0.0
}

// SubmitTransaction submits a new simulated transaction to the pool
func (s *BlockchainSimulator) SubmitTransaction(from, to, asset string, amount float64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate balance in simulation
	if from != "COINBASE" {
		assets, ok := s.balances[from]
		if !ok || assets[asset] < amount {
			return "", fmt.Errorf("insufficient simulated funds: %s has %f, needs %f", from, s.balances[from][asset], amount)
		}
		// deduct immediately to prevent double spending in simulation
		s.balances[from][asset] = RoundToPrecision(s.balances[from][asset]-amount, 8)
	}

	// Create deterministic hash
	rVal := rand.Int63()
	hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%s_%s_%s_%f_%d_%d", s.network, from, to, amount, time.Now().UnixNano(), rVal)))
	txHash := hex.EncodeToString(hashBytes[:])

	tx := &SimTransaction{
		Hash:        txHash,
		From:        from,
		To:          to,
		Amount:      amount,
		Asset:       asset,
		BlockHeight: 0,
		Status:      "PENDING",
		Timestamp:   time.Now(),
	}

	s.pendingTxs = append(s.pendingTxs, tx)
	s.txMap[txHash] = tx
	return txHash, nil
}

// MineBlock mines a new block containing pending transactions
func (s *BlockchainSimulator) MineBlock() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentBlock++
	height := s.currentBlock
	parentBlock := s.blocks[height-1]
	parentHash := ""
	if parentBlock != nil {
		parentHash = parentBlock.Hash
	}

	hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%s_block_%d_%s_%d", s.network, height, parentHash, time.Now().UnixNano())))
	hash := hex.EncodeToString(hashBytes[:])

	block := &SimBlock{
		Height:       height,
		Hash:         hash,
		ParentHash:   parentHash,
		Transactions: make([]*SimTransaction, 0),
		Timestamp:    time.Now(),
	}

	// Process pending transactions
	for _, tx := range s.pendingTxs {
		tx.BlockHeight = height
		tx.BlockHash = hash
		tx.Status = "SUCCESS"

		// credit the receiver
		if _, ok := s.balances[tx.To]; !ok {
			s.balances[tx.To] = make(map[string]float64)
		}
		s.balances[tx.To][tx.Asset] = RoundToPrecision(s.balances[tx.To][tx.Asset]+tx.Amount, 8)

		block.Transactions = append(block.Transactions, tx)
	}

	s.pendingTxs = make([]*SimTransaction, 0)
	s.blocks[height] = block
	return hash
}

// GetBlockByHeight gets block by height
func (s *BlockchainSimulator) GetBlockByHeight(height int64) *SimBlock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.blocks[height]
}

// GetTransactionByHash gets transaction by hash
func (s *BlockchainSimulator) GetTransactionByHash(hash string) *SimTransaction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.txMap[hash]
}

// GetHeight returns simulated chain height
func (s *BlockchainSimulator) GetHeight() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentBlock
}

// Reorg rolls back the chain by N blocks, reversing balances, then mines alternative blocks
func (s *BlockchainSimulator) Reorg(rollbackBlocks int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rollbackBlocks >= s.currentBlock {
		rollbackBlocks = s.currentBlock - 1
	}

	targetHeight := s.currentBlock - rollbackBlocks

	// Revert transactions and balances for rolled back blocks
	for h := s.currentBlock; h > targetHeight; h-- {
		block := s.blocks[h]
		if block != nil {
			for _, tx := range block.Transactions {
				// Reverse balance adjustments
				if tx.From != "COINBASE" {
					s.balances[tx.From][tx.Asset] = RoundToPrecision(s.balances[tx.From][tx.Asset]+tx.Amount, 8)
				}
				s.balances[tx.To][tx.Asset] = RoundToPrecision(s.balances[tx.To][tx.Asset]-tx.Amount, 8)

				// Mark transaction as orphaned/pending again or failed depending on reorg
				tx.Status = "PENDING"
				tx.BlockHeight = 0
				tx.BlockHash = ""
				s.pendingTxs = append(s.pendingTxs, tx)
			}
			delete(s.blocks, h)
		}
	}

	s.currentBlock = targetHeight

	// Now mine alternative block(s) to create alternate history fork
	s.currentBlock++
	height := s.currentBlock
	parentBlock := s.blocks[height-1]
	parentHash := ""
	if parentBlock != nil {
		parentHash = parentBlock.Hash
	}

	hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%s_reorg_alt_block_%d_%s_%d", s.network, height, parentHash, time.Now().UnixNano())))
	altHash := hex.EncodeToString(hashBytes[:])

	altBlock := &SimBlock{
		Height:       height,
		Hash:         altHash,
		ParentHash:   parentHash,
		Transactions: make([]*SimTransaction, 0),
		Timestamp:    time.Now(),
	}

	// Mine only 50% of the pending txs into this alt block, leaving some orphaned or in mempool
	half := len(s.pendingTxs) / 2
	newPending := make([]*SimTransaction, 0)
	for i, tx := range s.pendingTxs {
		if i < half {
			tx.BlockHeight = height
			tx.BlockHash = altHash
			tx.Status = "SUCCESS"

			if _, ok := s.balances[tx.To]; !ok {
				s.balances[tx.To] = make(map[string]float64)
			}
			s.balances[tx.To][tx.Asset] = RoundToPrecision(s.balances[tx.To][tx.Asset]+tx.Amount, 8)

			if tx.From != "COINBASE" {
				s.balances[tx.From][tx.Asset] = RoundToPrecision(s.balances[tx.From][tx.Asset]-tx.Amount, 8)
			}

			altBlock.Transactions = append(altBlock.Transactions, tx)
		} else {
			newPending = append(newPending, tx)
		}
	}

	s.pendingTxs = newPending
	s.blocks[height] = altBlock
}

// SumAllBalances returns the sum of all simulated balances for a given asset
func (s *BlockchainSimulator) SumAllBalances(asset string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0.0
	for _, assets := range s.balances {
		total += assets[asset]
	}
	return RoundToPrecision(total, 8)
}
