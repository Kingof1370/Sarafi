package common

import (
	"errors"
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

// BTCAdapter implements BTC validation and confirmation tracking
type BTCAdapter struct{}

func (b *BTCAdapter) ValidateAddress(address string) bool {
	// Simple Bitcoin mainnet address prefix parsing validation rules
	if len(address) < 26 || len(address) > 35 {
		return false
	}
	return strings.HasPrefix(address, "1") || strings.HasPrefix(address, "3") || strings.HasPrefix(address, "bc1")
}

func (b *BTCAdapter) GetNativeBalance(address string) (float64, error) {
	return 0.523, nil // Abstract mock node integration
}

func (b *BTCAdapter) BroadcastTransaction(rawTx string) (string, error) {
	if rawTx == "" {
		return "", errors.New("empty transaction data")
	}
	return "tx_btc_" + regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(rawTx[:10], ""), nil
}

func (b *BTCAdapter) GetConfirmations(txHash string) (int, error) {
	return 6, nil
}

// ETHAdapter implements Ethereum and EVM-compatible address validation rules
type ETHAdapter struct{}

func (e *ETHAdapter) ValidateAddress(address string) bool {
	// Standard Ethereum hexadecimal format pattern validation
	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
	return re.MatchString(address)
}

func (e *ETHAdapter) GetNativeBalance(address string) (float64, error) {
	return 12.45, nil
}

func (e *ETHAdapter) BroadcastTransaction(rawTx string) (string, error) {
	if rawTx == "" {
		return "", errors.New("empty hex payload")
	}
	return "tx_eth_0x" + rawTx[:8], nil
}

func (e *ETHAdapter) GetConfirmations(txHash string) (int, error) {
	return 12, nil
}

// SOLAdapter implements Solana Base58 validation rules
type SOLAdapter struct{}

func (s *SOLAdapter) ValidateAddress(address string) bool {
	if len(address) < 32 || len(address) > 44 {
		return false
	}
	// Base58 checking character patterns
	re := regexp.MustCompile("^[1-9A-HJ-NP-Za-km-z]+$")
	return re.MatchString(address)
}

func (s *SOLAdapter) GetNativeBalance(address string) (float64, error) {
	return 150.0, nil
}

func (s *SOLAdapter) BroadcastTransaction(rawTx string) (string, error) {
	return "tx_sol_sig", nil
}

func (s *SOLAdapter) GetConfirmations(txHash string) (int, error) {
	return 32, nil
}

// GetBlockchainAdapter resolves the proper blockchain verification profile dynamically
func GetBlockchainAdapter(asset string) (BlockchainAdapter, error) {
	switch strings.ToUpper(asset) {
	case "BTC":
		return &BTCAdapter{}, nil
	case "ETH", "USDT", "USDC", "BNB":
		return &ETHAdapter{}, nil
	case "SOL":
		return &SOLAdapter{}, nil
	default:
		return nil, errors.New("unsupported blockchain asset integration")
	}
}
