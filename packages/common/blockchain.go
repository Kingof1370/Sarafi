package common

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
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

// EthereumClient is a wrapper around go-ethereum's ethclient.Client
type EthereumClient struct {
	Client *ethclient.Client
	URL    string
}

// NewEthereumClient creates a new EthereumClient using environment variables or fallback
func NewEthereumClient() (*EthereumClient, error) {
	url := os.Getenv("ETH_RPC_URL")
	if url == "" {
		url = "https://cloudflare-eth.com"
	}
	return NewEthereumClientWithURL(url)
}

// NewEthereumClientWithURL creates an EthereumClient with a specific URL
func NewEthereumClientWithURL(url string) (*EthereumClient, error) {
	client, err := ethclient.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("FAIL CLOSED: failed to connect to Ethereum RPC node %s: %w", url, err)
	}
	return &EthereumClient{
		Client: client,
		URL:    url,
	}, nil
}

func (ec *EthereumClient) GetBalance(address string) (*big.Int, error) {
	if ec.Client == nil {
		return nil, errors.New("FAIL CLOSED: EthereumClient is not initialized")
	}
	if !common.IsHexAddress(address) {
		return nil, fmt.Errorf("FAIL CLOSED: invalid address: %s", address)
	}
	addr := common.HexToAddress(address)
	bal, err := ec.Client.BalanceAt(context.Background(), addr, nil)
	if err != nil {
		return nil, fmt.Errorf("FAIL CLOSED: failed to fetch balance: %w", err)
	}
	return bal, nil
}

func (ec *EthereumClient) GetLatestBlockNumber() (uint64, error) {
	if ec.Client == nil {
		return 0, errors.New("FAIL CLOSED: EthereumClient is not initialized")
	}
	num, err := ec.Client.BlockNumber(context.Background())
	if err != nil {
		return 0, fmt.Errorf("FAIL CLOSED: failed to fetch block number: %w", err)
	}
	return num, nil
}

func (ec *EthereumClient) GetTransactionReceipt(txHash string) (status uint64, gasUsed uint64, err error) {
	if ec.Client == nil {
		return 0, 0, errors.New("FAIL CLOSED: EthereumClient is not initialized")
	}
	hash := common.HexToHash(txHash)
	receipt, err := ec.Client.TransactionReceipt(context.Background(), hash)
	if err != nil {
		return 0, 0, fmt.Errorf("FAIL CLOSED: failed to fetch transaction receipt: %w", err)
	}
	return receipt.Status, receipt.GasUsed, nil
}

// ETHAdapter implements Ethereum and EVM RPC integration
type ETHAdapter struct {
	RPCURL string
}

func (e *ETHAdapter) ValidateAddress(address string) bool {
	return common.IsHexAddress(address)
}

func (e *ETHAdapter) GetNativeBalance(address string) (float64, error) {
	ec, err := NewEthereumClientWithURL(e.RPCURL)
	if err != nil {
		return 0, err
	}
	bal, err := ec.GetBalance(address)
	if err != nil {
		return 0, err
	}

	// Convert Wei to Ether (10^18)
	fBalance := new(big.Float).SetInt(bal)
	fEther := new(big.Float).Quo(fBalance, big.NewFloat(1e18))
	val, _ := fEther.Float64()
	return val, nil
}

func (e *ETHAdapter) BroadcastTransaction(rawTx string) (string, error) {
	ec, err := NewEthereumClientWithURL(e.RPCURL)
	if err != nil {
		return "", err
	}

	rawTx = strings.TrimPrefix(rawTx, "0x")
	rawBytes, err := hex.DecodeString(rawTx)
	if err != nil {
		// Fallback to sending standard raw JSON-RPC if it is not a pure hexadecimal tx
		var txHash string
		err = callJSONRPC(e.RPCURL, "eth_sendRawTransaction", []interface{}{rawTx}, &txHash)
		if err != nil {
			return "", err
		}
		return txHash, nil
	}

	var tx ethtypes.Transaction
	err = tx.UnmarshalBinary(rawBytes)
	if err != nil {
		// Fallback to json rpc send raw tx if unmarshal binary fails (e.g. mock raw signature strings)
		var txHash string
		err = callJSONRPC(e.RPCURL, "eth_sendRawTransaction", []interface{}{rawTx}, &txHash)
		if err != nil {
			return "", err
		}
		return txHash, nil
	}

	err = ec.Client.SendTransaction(context.Background(), &tx)
	if err != nil {
		return "", fmt.Errorf("FAIL CLOSED: failed to send transaction: %w", err)
	}

	return tx.Hash().Hex(), nil
}

func (e *ETHAdapter) GetConfirmations(txHash string) (int, error) {
	ec, err := NewEthereumClientWithURL(e.RPCURL)
	if err != nil {
		return 0, err
	}

	hash := common.HexToHash(txHash)
	_, isPending, err := ec.Client.TransactionByHash(context.Background(), hash)
	if err != nil {
		// Fallback to JSON RPC if the txn hash can't be fetched via go-ethereum (e.g. mocks or customized rpc endpoint responses)
		var tx struct {
			BlockNumber string `json:"blockNumber"`
		}
		errCall := callJSONRPC(e.RPCURL, "eth_getTransactionByHash", []interface{}{txHash}, &tx)
		if errCall != nil {
			return 0, fmt.Errorf("FAIL CLOSED: failed to query transaction by hash: %w", err)
		}
		if tx.BlockNumber == "" {
			return 0, nil
		}
		var latestBlockHex string
		errCall = callJSONRPC(e.RPCURL, "eth_blockNumber", []interface{}{}, &latestBlockHex)
		if errCall != nil {
			return 0, errCall
		}
		blockNum := new(big.Int)
		blockNum.SetString(strings.TrimPrefix(tx.BlockNumber, "0x"), 16)
		latestBlock := new(big.Int)
		latestBlock.SetString(strings.TrimPrefix(latestBlockHex, "0x"), 16)
		confirmations := new(big.Int).Sub(latestBlock, blockNum)
		return int(confirmations.Int64()) + 1, nil
	}

	if isPending {
		return 0, nil
	}

	receipt, err := ec.Client.TransactionReceipt(context.Background(), hash)
	if err != nil {
		return 0, fmt.Errorf("FAIL CLOSED: failed to query transaction receipt: %w", err)
	}

	if receipt.BlockNumber == nil {
		return 0, nil
	}

	latest, err := ec.GetLatestBlockNumber()
	if err != nil {
		return 0, err
	}

	if latest < receipt.BlockNumber.Uint64() {
		return 0, nil
	}

	confirmations := latest - receipt.BlockNumber.Uint64() + 1
	return int(confirmations), nil
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
			return nil, errors.New("FAIL CLOSED: BTC_RPC_URL is not configured in production")
		}
		return &BTCAdapter{RPCURL: url}, nil
	case "ETH":
		url := os.Getenv("ETH_RPC_URL")
		if url == "" {
			return nil, errors.New("FAIL CLOSED: ETH_RPC_URL is not configured in production")
		}
		return &ETHAdapter{RPCURL: url}, nil
	case "BNB", "BSC":
		url := os.Getenv("BSC_RPC_URL")
		if url == "" {
			return nil, errors.New("FAIL CLOSED: BSC_RPC_URL is not configured in production")
		}
		return &BSCAdapter{ETHAdapter{RPCURL: url}}, nil
	case "POLYGON", "MATIC":
		url := os.Getenv("POLYGON_RPC_URL")
		if url == "" {
			return nil, errors.New("FAIL CLOSED: POLYGON_RPC_URL is not configured in production")
		}
		return &PolygonAdapter{ETHAdapter{RPCURL: url}}, nil
	case "AVALANCHE", "AVAX":
		url := os.Getenv("AVAX_RPC_URL")
		if url == "" {
			return nil, errors.New("FAIL CLOSED: AVAX_RPC_URL is not configured in production")
		}
		return &AvalancheAdapter{ETHAdapter{RPCURL: url}}, nil
	case "SOLANA", "SOL":
		url := os.Getenv("SOLANA_RPC_URL")
		if url == "" {
			return nil, errors.New("FAIL CLOSED: SOLANA_RPC_URL is not configured in production")
		}
		return &SolanaAdapter{RPCURL: url}, nil
	case "TRON", "TRX":
		url := os.Getenv("TRON_RPC_URL")
		if url == "" {
			return nil, errors.New("FAIL CLOSED: TRON_RPC_URL is not configured in production")
		}
		return &TronAdapter{RPCURL: url}, nil
	case "LITECOIN", "LTC":
		url := os.Getenv("LTC_RPC_URL")
		if url == "" {
			return nil, errors.New("FAIL CLOSED: LTC_RPC_URL is not configured in production")
		}
		return &LitecoinAdapter{BTCAdapter{RPCURL: url}}, nil
	default:
		return nil, fmt.Errorf("FAIL CLOSED: unsupported production blockchain asset: %s", asset)
	}
}
