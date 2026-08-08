package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// BlockchainMode defines the running environment profile of the adapters
type BlockchainMode string

const (
	ModeLive       BlockchainMode = "LIVE"
	ModeTestnet    BlockchainMode = "TESTNET"
	ModeSimulation BlockchainMode = "SIMULATION"
)

// BlockchainAdapter represents generic validation, broadcasting, and verification capabilities
type BlockchainAdapter interface {
	ValidateAddress(address string) bool
	GetNativeBalance(address string) (float64, error)
	BroadcastTransaction(rawTx string) (string, error)
	GetConfirmations(txHash string) (int, error)
	GetBlockHeight() (int64, error)
	GetMode() BlockchainMode
}

// BaseAdapter implements shared configuration and RPC parameters
type BaseAdapter struct {
	Asset   string
	Network string
	Mode    BlockchainMode
	RPCURL  string
}

// BTCAdapter implements BTC validation, live RPC and simulator integration
type BTCAdapter struct {
	BaseAdapter
}

func (b *BTCAdapter) ValidateAddress(address string) bool {
	if len(address) < 26 || len(address) > 42 {
		return false
	}
	// Mainnet legacy, multisig, and Bech32 segwit
	return strings.HasPrefix(address, "1") || strings.HasPrefix(address, "3") || strings.HasPrefix(address, "bc1") || strings.HasPrefix(address, "tb1")
}

func (b *BTCAdapter) GetNativeBalance(address string) (float64, error) {
	if b.Mode == ModeSimulation {
		sim := GetSimulator("BTC")
		return sim.GetBalance(address, "BTC"), nil
	}

	if b.RPCURL == "" {
		return 0, errors.New("BTC real node RPC URL is not configured")
	}

	// Real Bitcoin RPC call: getreceivedbyaddress
	payload := map[string]interface{}{
		"jsonrpc": "1.0",
		"id":      "velyxora",
		"method":  "getreceivedbyaddress",
		"params":  []interface{}{address, 1},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest("POST", b.RPCURL, bytes.NewBuffer(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to BTC node RPC: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result float64 `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}
	if rpcResp.Error != nil {
		return 0, errors.New(rpcResp.Error.Message)
	}

	return RoundToPrecision(rpcResp.Result, 8), nil
}

func (b *BTCAdapter) BroadcastTransaction(rawTx string) (string, error) {
	if rawTx == "" {
		return "", errors.New("empty raw transaction payload")
	}

	if b.Mode == ModeSimulation {
		sim := GetSimulator("BTC")
		return sim.SubmitTransaction("COINBASE", "SIM_RECEIVER", "BTC", 1.0)
	}

	if b.RPCURL == "" {
		return "", errors.New("BTC real node RPC URL is not configured")
	}

	payload := map[string]interface{}{
		"jsonrpc": "1.0",
		"id":      "velyxora",
		"method":  "sendrawtransaction",
		"params":  []interface{}{rawTx},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(b.RPCURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to connect to BTC node RPC: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", err
	}
	if rpcResp.Error != nil {
		return "", errors.New(rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

func (b *BTCAdapter) GetConfirmations(txHash string) (int, error) {
	if b.Mode == ModeSimulation {
		sim := GetSimulator("BTC")
		tx := sim.GetTransactionByHash(txHash)
		if tx == nil {
			return 0, nil
		}
		if tx.Status == "PENDING" || tx.BlockHeight == 0 {
			return 0, nil
		}
		diff := sim.GetHeight() - tx.BlockHeight + 1
		if diff < 0 {
			return 0, nil
		}
		return int(diff), nil
	}

	if b.RPCURL == "" {
		return 0, errors.New("BTC real node RPC URL is not configured")
	}

	payload := map[string]interface{}{
		"jsonrpc": "1.0",
		"id":      "velyxora",
		"method":  "gettransaction",
		"params":  []interface{}{txHash},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	resp, err := http.Post(b.RPCURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return 0, fmt.Errorf("failed to connect to BTC node RPC: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result struct {
			Confirmations int `json:"confirmations"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}
	if rpcResp.Error != nil {
		return 0, errors.New(rpcResp.Error.Message)
	}

	return rpcResp.Result.Confirmations, nil
}

func (b *BTCAdapter) GetBlockHeight() (int64, error) {
	if b.Mode == ModeSimulation {
		sim := GetSimulator("BTC")
		return sim.GetHeight(), nil
	}

	if b.RPCURL == "" {
		return 0, errors.New("BTC real node RPC URL is not configured")
	}

	payload := map[string]interface{}{
		"jsonrpc": "1.0",
		"id":      "velyxora",
		"method":  "getblockcount",
		"params":  []interface{}{},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	resp, err := http.Post(b.RPCURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return 0, fmt.Errorf("failed to connect to BTC node RPC: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result int64 `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}
	if rpcResp.Error != nil {
		return 0, errors.New(rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

func (b *BTCAdapter) GetMode() BlockchainMode {
	return b.Mode
}

// ETHAdapter implements Ethereum and EVM-compatible address validation and live RPC/simulator calls
type ETHAdapter struct {
	BaseAdapter
}

func (e *ETHAdapter) ValidateAddress(address string) bool {
	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
	return re.MatchString(address)
}

func (e *ETHAdapter) GetNativeBalance(address string) (float64, error) {
	if e.Mode == ModeSimulation {
		sim := GetSimulator("ETH")
		return sim.GetBalance(address, "ETH"), nil
	}

	if e.RPCURL == "" {
		return 0, errors.New("ETH real node RPC URL is not configured")
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "eth_getBalance",
		"params":  []interface{}{address, "latest"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	resp, err := http.Post(e.RPCURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return 0, fmt.Errorf("failed to connect to ETH node RPC: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}
	if rpcResp.Error != nil {
		return 0, errors.New(rpcResp.Error.Message)
	}

	// Parse hex balance in Wei
	cleanHex := strings.TrimPrefix(rpcResp.Result, "0x")
	weiVal, err := strconv.ParseUint(cleanHex, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse eth_getBalance hex: %w", err)
	}

	// Convert Wei to ETH (1 ETH = 10^18 Wei)
	ethVal := float64(weiVal) / 1e18
	return RoundToPrecision(ethVal, 8), nil
}

func (e *ETHAdapter) BroadcastTransaction(rawTx string) (string, error) {
	if rawTx == "" {
		return "", errors.New("empty hex payload")
	}

	if e.Mode == ModeSimulation {
		sim := GetSimulator("ETH")
		return sim.SubmitTransaction("COINBASE", "SIM_RECEIVER", "ETH", 1.0)
	}

	if e.RPCURL == "" {
		return "", errors.New("ETH real node RPC URL is not configured")
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "eth_sendRawTransaction",
		"params":  []interface{}{rawTx},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(e.RPCURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to connect to ETH node RPC: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", err
	}
	if rpcResp.Error != nil {
		return "", errors.New(rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

func (e *ETHAdapter) GetConfirmations(txHash string) (int, error) {
	if e.Mode == ModeSimulation {
		sim := GetSimulator("ETH")
		tx := sim.GetTransactionByHash(txHash)
		if tx == nil {
			return 0, nil
		}
		if tx.Status == "PENDING" || tx.BlockHeight == 0 {
			return 0, nil
		}
		diff := sim.GetHeight() - tx.BlockHeight + 1
		if diff < 0 {
			return 0, nil
		}
		return int(diff), nil
	}

	if e.RPCURL == "" {
		return 0, errors.New("ETH real node RPC URL is not configured")
	}

	// 1. Get Tx receipt to find block number
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "eth_getTransactionReceipt",
		"params":  []interface{}{txHash},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	resp, err := http.Post(e.RPCURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return 0, fmt.Errorf("failed to connect to ETH node RPC: %w", err)
	}
	defer resp.Body.Close()

	var receiptResp struct {
		Result *struct {
			BlockNumber string `json:"blockNumber"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&receiptResp); err != nil {
		return 0, err
	}
	if receiptResp.Error != nil {
		return 0, errors.New(receiptResp.Error.Message)
	}
	if receiptResp.Result == nil {
		return 0, nil // Pending
	}

	txBlockHeightHex := strings.TrimPrefix(receiptResp.Result.BlockNumber, "0x")
	txBlockHeight, err := strconv.ParseInt(txBlockHeightHex, 16, 64)
	if err != nil {
		return 0, err
	}

	latestHeight, err := e.GetBlockHeight()
	if err != nil {
		return 0, err
	}

	confirmations := latestHeight - txBlockHeight + 1
	if confirmations < 0 {
		return 0, nil
	}
	return int(confirmations), nil
}

func (e *ETHAdapter) GetBlockHeight() (int64, error) {
	if e.Mode == ModeSimulation {
		sim := GetSimulator("ETH")
		return sim.GetHeight(), nil
	}

	if e.RPCURL == "" {
		return 0, errors.New("ETH real node RPC URL is not configured")
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	resp, err := http.Post(e.RPCURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return 0, fmt.Errorf("failed to connect to ETH node RPC: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}
	if rpcResp.Error != nil {
		return 0, errors.New(rpcResp.Error.Message)
	}

	cleanHex := strings.TrimPrefix(rpcResp.Result, "0x")
	height, err := strconv.ParseInt(cleanHex, 16, 64)
	if err != nil {
		return 0, err
	}

	return height, nil
}

func (e *ETHAdapter) GetMode() BlockchainMode {
	return e.Mode
}

// SOLAdapter implements Solana Base58 validation and simulator calls
type SOLAdapter struct {
	BaseAdapter
}

func (s *SOLAdapter) ValidateAddress(address string) bool {
	if len(address) < 32 || len(address) > 44 {
		return false
	}
	re := regexp.MustCompile("^[1-9A-HJ-NP-Za-km-z]+$")
	return re.MatchString(address)
}

func (s *SOLAdapter) GetNativeBalance(address string) (float64, error) {
	if s.Mode == ModeSimulation {
		sim := GetSimulator("SOL")
		return sim.GetBalance(address, "SOL"), nil
	}

	if s.RPCURL == "" {
		return 0, errors.New("SOL real node RPC URL is not configured")
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getBalance",
		"params":  []interface{}{address},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	resp, err := http.Post(s.RPCURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return 0, fmt.Errorf("failed to connect to SOL node RPC: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result struct {
			Value uint64 `json:"value"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}
	if rpcResp.Error != nil {
		return 0, errors.New(rpcResp.Error.Message)
	}

	solVal := float64(rpcResp.Result.Value) / 1e9 // 1 SOL = 10^9 Lamports
	return RoundToPrecision(solVal, 8), nil
}

func (s *SOLAdapter) BroadcastTransaction(rawTx string) (string, error) {
	if rawTx == "" {
		return "", errors.New("empty rawTx transaction signature")
	}

	if s.Mode == ModeSimulation {
		sim := GetSimulator("SOL")
		return sim.SubmitTransaction("COINBASE", "SIM_RECEIVER", "SOL", 1.0)
	}

	if s.RPCURL == "" {
		return "", errors.New("SOL real node RPC URL is not configured")
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params":  []interface{}{rawTx},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(s.RPCURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to connect to SOL node RPC: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", err
	}
	if rpcResp.Error != nil {
		return "", errors.New(rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

func (s *SOLAdapter) GetConfirmations(txHash string) (int, error) {
	if s.Mode == ModeSimulation {
		sim := GetSimulator("SOL")
		tx := sim.GetTransactionByHash(txHash)
		if tx == nil {
			return 0, nil
		}
		if tx.Status == "PENDING" || tx.BlockHeight == 0 {
			return 0, nil
		}
		diff := sim.GetHeight() - tx.BlockHeight + 1
		if diff < 0 {
			return 0, nil
		}
		return int(diff), nil
	}

	if s.RPCURL == "" {
		return 0, errors.New("SOL real node RPC URL is not configured")
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getSignatureStatuses",
		"params":  []interface{}{[]string{txHash}, map[string]interface{}{"searchTransactionHistory": true}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	resp, err := http.Post(s.RPCURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return 0, fmt.Errorf("failed to connect to SOL node RPC: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result struct {
			Value []interface{} `json:"value"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}
	if rpcResp.Error != nil {
		return 0, errors.New(rpcResp.Error.Message)
	}

	if len(rpcResp.Result.Value) == 0 || rpcResp.Result.Value[0] == nil {
		return 0, nil // not found or pending
	}

	// Parse custom signature status structure
	statusMap, ok := rpcResp.Result.Value[0].(map[string]interface{})
	if !ok {
		return 0, nil
	}

	confirmationsVal, exists := statusMap["confirmations"]
	if !exists || confirmationsVal == nil {
		// If confirmation is complete, confirmation value might be nil and status is "finalized"
		confirmationsVal = float64(32) // max/finalized target
	}

	switch val := confirmationsVal.(type) {
	case float64:
		return int(val), nil
	case int:
		return val, nil
	}

	return 0, nil
}

func (s *SOLAdapter) GetBlockHeight() (int64, error) {
	if s.Mode == ModeSimulation {
		sim := GetSimulator("SOL")
		return sim.GetHeight(), nil
	}

	if s.RPCURL == "" {
		return 0, errors.New("SOL real node RPC URL is not configured")
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getSlot",
		"params":  []interface{}{},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	resp, err := http.Post(s.RPCURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return 0, fmt.Errorf("failed to connect to SOL node RPC: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result int64 `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}
	if rpcResp.Error != nil {
		return 0, errors.New(rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

func (s *SOLAdapter) GetMode() BlockchainMode {
	return s.Mode
}

// GetBlockchainAdapter resolves the proper blockchain verification profile dynamically
func GetBlockchainAdapter(asset string) (BlockchainAdapter, error) {
	// 1. Resolve environment blockchain mode to ensure strictness
	rawMode := getEnv("BLOCKCHAIN_MODE", "LIVE") // Default is LIVE for security to prevent silent simulation selection in production
	mode := BlockchainMode(strings.ToUpper(rawMode))

	if mode != ModeLive && mode != ModeTestnet && mode != ModeSimulation {
		return nil, fmt.Errorf("invalid configured BLOCKCHAIN_MODE: %s", rawMode)
	}

	// Check if this is a production environment running simulation, raising explicit warnings/block
	if mode == ModeSimulation && getEnv("ENV", "development") == "production" {
		return nil, errors.New("CRITICAL FAILURE: SIMULATION MODE IS DETECTED IN PRODUCTION ENVIRONMENT")
	}

	rpcURL := getEnv(fmt.Sprintf("%s_RPC_URL", strings.ToUpper(asset)), "")

	switch strings.ToUpper(asset) {
	case "BTC":
		return &BTCAdapter{
			BaseAdapter: BaseAdapter{
				Asset:   "BTC",
				Network: "BTC",
				Mode:    mode,
				RPCURL:  rpcURL,
			},
		}, nil
	case "ETH", "USDT", "USDC", "BNB":
		return &ETHAdapter{
			BaseAdapter: BaseAdapter{
				Asset:   strings.ToUpper(asset),
				Network: "ETH",
				Mode:    mode,
				RPCURL:  rpcURL,
			},
		}, nil
	case "SOL":
		return &SOLAdapter{
			BaseAdapter: BaseAdapter{
				Asset:   "SOL",
				Network: "SOL",
				Mode:    mode,
				RPCURL:  rpcURL,
			},
		}, nil
	default:
		return nil, errors.New("unsupported blockchain asset integration")
	}
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}
