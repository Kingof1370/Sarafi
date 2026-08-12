package common

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// BlockchainAdapter represents generic validation, broadcasting, and verification capabilities
type BlockchainAdapter interface {
	ValidateAddress(address string) bool
	GetNativeBalance(address string) (float64, error)
	BroadcastTransaction(rawTx string) (string, error)
	GetConfirmations(txHash string) (int, error)
}

// callJSONRPC performs a standard context-aware JSON-RPC POST call
func callJSONRPC(rpcURL, method string, params []interface{}, result interface{}) error {
	if rpcURL == "" {
		return errors.New("FAIL CLOSED: RPC URL is not configured")
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", rpcURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("FAIL CLOSED: RPC node unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("FAIL CLOSED: RPC node returned HTTP status %d", resp.StatusCode)
	}

	var jsonResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		return err
	}

	if jsonResp.Error != nil {
		return fmt.Errorf("FAIL CLOSED: RPC error: %s (code %d)", jsonResp.Error.Message, jsonResp.Error.Code)
	}

	return json.Unmarshal(jsonResp.Result, result)
}

// BTCAdapter implements BTC validation and RPC integration
type BTCAdapter struct {
	RPCURL string
}

func (b *BTCAdapter) ValidateAddress(address string) bool {
	if len(address) < 26 || len(address) > 35 {
		return false
	}
	return strings.HasPrefix(address, "1") || strings.HasPrefix(address, "3") || strings.HasPrefix(address, "bc1")
}

func (b *BTCAdapter) GetNativeBalance(address string) (float64, error) {
	var bal float64
	err := callJSONRPC(b.RPCURL, "getreceivedbyaddress", []interface{}{address}, &bal)
	if err != nil {
		return 0, err
	}
	return bal, nil
}

func (b *BTCAdapter) BroadcastTransaction(rawTx string) (string, error) {
	var txHash string
	err := callJSONRPC(b.RPCURL, "sendrawtransaction", []interface{}{rawTx}, &txHash)
	if err != nil {
		return "", err
	}
	return txHash, nil
}

func (b *BTCAdapter) GetConfirmations(txHash string) (int, error) {
	var tx struct {
		Confirmations int `json:"confirmations"`
	}
	err := callJSONRPC(b.RPCURL, "gettransaction", []interface{}{txHash}, &tx)
	if err != nil {
		return 0, err
	}
	return tx.Confirmations, nil
}

// ETHAdapter implements Ethereum and EVM RPC integration
type ETHAdapter struct {
	RPCURL string
}

func (e *ETHAdapter) ValidateAddress(address string) bool {
	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
	return re.MatchString(address)
}

func (e *ETHAdapter) GetNativeBalance(address string) (float64, error) {
	var hexBal string
	err := callJSONRPC(e.RPCURL, "eth_getBalance", []interface{}{address, "latest"}, &hexBal)
	if err != nil {
		return 0, err
	}

	// Parse hexadecimal value
	hexVal := strings.TrimPrefix(hexBal, "0x")
	i := new(big.Int)
	if _, ok := i.SetString(hexVal, 16); !ok {
		return 0, fmt.Errorf("failed to parse hex balance: %s", hexBal)
	}

	// Convert Wei to Ether (10^18)
	fBalance := new(big.Float).SetInt(i)
	fEther := new(big.Float).Quo(fBalance, big.NewFloat(1e18))
	val, _ := fEther.Float64()
	return val, nil
}

func (e *ETHAdapter) BroadcastTransaction(rawTx string) (string, error) {
	var txHash string
	err := callJSONRPC(e.RPCURL, "eth_sendRawTransaction", []interface{}{rawTx}, &txHash)
	if err != nil {
		return "", err
	}
	return txHash, nil
}

func (e *ETHAdapter) GetConfirmations(txHash string) (int, error) {
	var tx struct {
		BlockNumber string `json:"blockNumber"`
	}
	err := callJSONRPC(e.RPCURL, "eth_getTransactionByHash", []interface{}{txHash}, &tx)
	if err != nil {
		return 0, err
	}

	if tx.BlockNumber == "" {
		return 0, nil // Pending transaction
	}

	var latestBlockHex string
	err = callJSONRPC(e.RPCURL, "eth_blockNumber", []interface{}{}, &latestBlockHex)
	if err != nil {
		return 0, err
	}

	blockNum := new(big.Int)
	blockNum.SetString(strings.TrimPrefix(tx.BlockNumber, "0x"), 16)

	latestBlock := new(big.Int)
	latestBlock.SetString(strings.TrimPrefix(latestBlockHex, "0x"), 16)

	confirmations := new(big.Int).Sub(latestBlock, blockNum)
	return int(confirmations.Int64()) + 1, nil
}

// BSCAdapter implements BNB Smart Chain RPC integration
type BSCAdapter struct {
	ETHAdapter
}

// PolygonAdapter implements Polygon RPC integration
type PolygonAdapter struct {
	ETHAdapter
}

// AvalancheAdapter implements Avalanche RPC integration
type AvalancheAdapter struct {
	ETHAdapter
}

// SolanaAdapter implements Solana RPC integration
type SolanaAdapter struct {
	RPCURL string
}

func (s *SolanaAdapter) ValidateAddress(address string) bool {
	if len(address) < 32 || len(address) > 44 {
		return false
	}
	re := regexp.MustCompile("^[1-9A-HJ-NP-Za-km-z]+$")
	return re.MatchString(address)
}

func (s *SolanaAdapter) GetNativeBalance(address string) (float64, error) {
	var result struct {
		Value uint64 `json:"value"`
	}
	err := callJSONRPC(s.RPCURL, "getBalance", []interface{}{address}, &result)
	if err != nil {
		return 0, err
	}
	// Convert Lamports to SOL (10^9)
	return float64(result.Value) / 1e9, nil
}

func (s *SolanaAdapter) BroadcastTransaction(rawTx string) (string, error) {
	var signature string
	err := callJSONRPC(s.RPCURL, "sendTransaction", []interface{}{rawTx}, &signature)
	if err != nil {
		return "", err
	}
	return signature, nil
}

func (s *SolanaAdapter) GetConfirmations(txHash string) (int, error) {
	var status struct {
		Value []struct {
			Confirmations *int `json:"confirmations"`
		} `json:"value"`
	}
	err := callJSONRPC(s.RPCURL, "getSignatureStatuses", []interface{}{[]string{txHash}}, &status)
	if err != nil {
		return 0, err
	}

	if len(status.Value) == 0 || status.Value[0].Confirmations == nil {
		return 0, nil
	}
	return *status.Value[0].Confirmations, nil
}

// TronAdapter implements Tron RPC integration
type TronAdapter struct {
	RPCURL string
}

func (t *TronAdapter) ValidateAddress(address string) bool {
	if len(address) != 34 || !strings.HasPrefix(address, "T") {
		return false
	}
	re := regexp.MustCompile("^[a-zA-Z0-9]+$")
	return re.MatchString(address)
}

func (t *TronAdapter) GetNativeBalance(address string) (float64, error) {
	var bal uint64
	err := callJSONRPC(t.RPCURL, "getaccount", []interface{}{address}, &bal)
	if err != nil {
		return 0, err
	}
	return float64(bal) / 1e6, nil
}

func (t *TronAdapter) BroadcastTransaction(rawTx string) (string, error) {
	var txID string
	err := callJSONRPC(t.RPCURL, "broadcasttransaction", []interface{}{rawTx}, &txID)
	if err != nil {
		return "", err
	}
	return txID, nil
}

func (t *TronAdapter) GetConfirmations(txHash string) (int, error) {
	var tx struct {
		Confirmations int `json:"confirmations"`
	}
	err := callJSONRPC(t.RPCURL, "gettransactionconfirmations", []interface{}{txHash}, &tx)
	if err != nil {
		return 0, err
	}
	return tx.Confirmations, nil
}

// LitecoinAdapter implements Litecoin RPC integration
type LitecoinAdapter struct {
	BTCAdapter
}

func (l *LitecoinAdapter) ValidateAddress(address string) bool {
	if len(address) < 26 || len(address) > 43 {
		return false
	}
	return strings.HasPrefix(address, "L") || strings.HasPrefix(address, "M") || strings.HasPrefix(address, "ltc1")
}

// SimulationBlockchainProvider is a mock provider used ONLY in development/test/simulation mode
type SimulationBlockchainProvider struct{}

func (s *SimulationBlockchainProvider) ValidateAddress(address string) bool {
	if len(address) < 26 {
		return false
	}
	return true
}

func (s *SimulationBlockchainProvider) GetNativeBalance(address string) (float64, error) {
	return 100.0, nil
}

func (s *SimulationBlockchainProvider) BroadcastTransaction(rawTx string) (string, error) {
	return "tx_sim_0x" + rawTx[:8], nil
}

func (s *SimulationBlockchainProvider) GetConfirmations(txHash string) (int, error) {
	return 12, nil
}

// GetBlockchainAdapter resolves the proper blockchain verification profile dynamically
func GetBlockchainAdapter(asset string) (BlockchainAdapter, error) {
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "production" // Default to production mode for safety
	}

	if appEnv == "development" || appEnv == "test" || appEnv == "simulation" {
		return &SimulationBlockchainProvider{}, nil
	}

	switch strings.ToUpper(asset) {
	case "BTC":
		url := os.Getenv("BTC_RPC_URL")
		if url == "" {
			// Fallback to high-reliability public Bitcoin RPC node (Blockstream or similar API gateway)
			url = "https://blockstream.info/api"
		}
		return &BTCAdapter{RPCURL: url}, nil
	case "ETH":
		url := os.Getenv("ETH_RPC_URL")
		if url == "" {
			// Fallback to high-reliability public Cloudflare Ethereum mainnet node
			url = "https://cloudflare-eth.com"
		}
		return &ETHAdapter{RPCURL: url}, nil
	case "BNB", "BSC":
		url := os.Getenv("BSC_RPC_URL")
		if url == "" {
			// Fallback to Binance Smart Chain public RPC endpoint
			url = "https://bsc-dataseed.binance.org"
		}
		return &BSCAdapter{ETHAdapter{RPCURL: url}}, nil
	case "POLYGON", "MATIC":
		url := os.Getenv("POLYGON_RPC_URL")
		if url == "" {
			// Fallback to Polygon POS network public RPC
			url = "https://polygon-rpc.com"
		}
		return &PolygonAdapter{ETHAdapter{RPCURL: url}}, nil
	case "AVALANCHE", "AVAX":
		url := os.Getenv("AVAX_RPC_URL")
		if url == "" {
			// Fallback to Avalanche C-Chain public RPC endpoint
			url = "https://api.avax.network/ext/bc/C/rpc"
		}
		return &AvalancheAdapter{ETHAdapter{RPCURL: url}}, nil
	case "SOLANA", "SOL":
		url := os.Getenv("SOLANA_RPC_URL")
		if url == "" {
			// Fallback to Solana Mainnet-Beta public RPC node
			url = "https://api.mainnet-beta.solana.com"
		}
		return &SolanaAdapter{RPCURL: url}, nil
	case "TRON", "TRX":
		url := os.Getenv("TRON_RPC_URL")
		if url == "" {
			// Fallback to public TronGrid API gateway
			url = "https://api.trongrid.io"
		}
		return &TronAdapter{RPCURL: url}, nil
	case "LITECOIN", "LTC":
		url := os.Getenv("LTC_RPC_URL")
		if url == "" {
			// Fallback to Litecoin public indexer API
			url = "https://litecoinblockexplorer.net/api"
		}
		return &LitecoinAdapter{BTCAdapter{RPCURL: url}}, nil
	default:
		return nil, fmt.Errorf("FAIL CLOSED: unsupported production blockchain asset: %s", asset)
	}
}
