package blockchain

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"golang.org/x/crypto/ripemd160"
)

// BlockchainAdapter defines a production-grade interface for blockchain interaction
type BlockchainAdapter interface {
	NetworkName() string
	ValidateAddress(address string) bool
	GenerateAddress(privateKey []byte) (string, error)
	DerivePublicKey(privateKey []byte) []byte
	SignTransaction(privateKey []byte, txData []byte) ([]byte, error)
	VerifySignature(publicKey []byte, txData []byte, signature []byte) (bool, error)
	DerivationPath() string
}

// Base58 encoder/decoder helper to satisfy real address representations
var b58Alphabet = []byte("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz")

func EncodeBase58(input []byte) string {
	var result []byte
	x := big.NewInt(0).SetBytes(input)
	base := big.NewInt(int64(len(b58Alphabet)))
	zero := big.NewInt(0)
	mod := &big.Int{}

	for x.Cmp(zero) > 0 {
		x.DivMod(x, base, mod)
		result = append(result, b58Alphabet[mod.Int64()])
	}

	// Reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	// Add leading zeros
	for _, b := range input {
		if b == 0x00 {
			result = append([]byte{b58Alphabet[0]}, result...)
		} else {
			break
		}
	}

	return string(result)
}

// BTCAdapter implements Bitcoin HD key address generation and signature verification
type BTCAdapter struct{}

func (b *BTCAdapter) NetworkName() string { return "Bitcoin" }
func (b *BTCAdapter) DerivationPath() string { return "m/44'/0'/0'/0/0" }

func (b *BTCAdapter) ValidateAddress(address string) bool {
	if len(address) < 26 || len(address) > 35 {
		return false
	}
	return strings.HasPrefix(address, "1") || strings.HasPrefix(address, "3") || strings.HasPrefix(address, "bc1")
}

func (b *BTCAdapter) DerivePublicKey(privateKey []byte) []byte {
	return GetPubKeyFromPriv(privateKey)
}

func (b *BTCAdapter) GenerateAddress(privateKey []byte) (string, error) {
	// Standard BTC Mainnet Address Generation (P2PKH)
	pubKey := b.DerivePublicKey(privateKey)
	shaHash := sha256.Sum256(pubKey)

	rp := ripemd160.New()
	_, _ = rp.Write(shaHash[:])
	ripeHash := rp.Sum(nil)

	versionedPayload := append([]byte{0x00}, ripeHash...)

	firstSHA := sha256.Sum256(versionedPayload)
	secondSHA := sha256.Sum256(firstSHA[:])
	checksum := secondSHA[:4]

	fullPayload := append(versionedPayload, checksum...)
	return EncodeBase58(fullPayload), nil
}

func (b *BTCAdapter) SignTransaction(privateKey []byte, txData []byte) ([]byte, error) {
	if len(privateKey) != 32 {
		return nil, errors.New("invalid private key length")
	}
	hash := sha256.Sum256(txData)
	privKey, _ := btcec.PrivKeyFromBytes(privateKey)
	sig := ecdsa.Sign(privKey, hash[:])
	return sig.Serialize(), nil
}

func (b *BTCAdapter) VerifySignature(publicKey []byte, txData []byte, signature []byte) (bool, error) {
	hash := sha256.Sum256(txData)
	pubKey, err := btcec.ParsePubKey(publicKey)
	if err != nil {
		return false, err
	}
	sig, err := ecdsa.ParseSignature(signature)
	if err != nil {
		return false, err
	}
	return sig.Verify(hash[:], pubKey), nil
}

// EVMAdapter is the core engine for all EVM compatible blockchains (Ethereum, BNB, Polygon, Avalanche)
type EVMAdapter struct {
	Network string
	Path    string
}

func (e *EVMAdapter) NetworkName() string { return e.Network }
func (e *EVMAdapter) DerivationPath() string { return e.Path }

func (e *EVMAdapter) ValidateAddress(address string) bool {
	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
	return re.MatchString(address)
}

func (e *EVMAdapter) DerivePublicKey(privateKey []byte) []byte {
	return GetPubKeyFromPriv(privateKey)
}

func (e *EVMAdapter) GenerateAddress(privateKey []byte) (string, error) {
	pubKey := e.DerivePublicKey(privateKey)
	hash := sha256.Sum256(pubKey)
	addrHex := hex.EncodeToString(hash[12:]) // Last 20 bytes
	return "0x" + addrHex, nil
}

func (e *EVMAdapter) SignTransaction(privateKey []byte, txData []byte) ([]byte, error) {
	hash := sha256.Sum256(txData)
	privKey, _ := btcec.PrivKeyFromBytes(privateKey)
	sig := ecdsa.Sign(privKey, hash[:])
	return sig.Serialize(), nil
}

func (e *EVMAdapter) VerifySignature(publicKey []byte, txData []byte, signature []byte) (bool, error) {
	hash := sha256.Sum256(txData)
	pubKey, err := btcec.ParsePubKey(publicKey)
	if err != nil {
		return false, err
	}
	sig, err := ecdsa.ParseSignature(signature)
	if err != nil {
		return false, err
	}
	return sig.Verify(hash[:], pubKey), nil
}

// SOLAdapter implements Solana's Ed25519 signature algorithm and Base58 address structure
type SOLAdapter struct{}

func (s *SOLAdapter) NetworkName() string { return "Solana" }
func (s *SOLAdapter) DerivationPath() string { return "m/44'/501'/0'/0'" }

func (s *SOLAdapter) ValidateAddress(address string) bool {
	if len(address) < 32 || len(address) > 44 {
		return false
	}
	re := regexp.MustCompile("^[1-9A-HJ-NP-Za-km-z]+$")
	return re.MatchString(address)
}

func (s *SOLAdapter) DerivePublicKey(privateKey []byte) []byte {
	return ed25519.NewKeyFromSeed(privateKey).Public().(ed25519.PublicKey)
}

func (s *SOLAdapter) GenerateAddress(privateKey []byte) (string, error) {
	pubKey := s.DerivePublicKey(privateKey)
	return EncodeBase58(pubKey), nil
}

func (s *SOLAdapter) SignTransaction(privateKey []byte, txData []byte) ([]byte, error) {
	fullPriv := ed25519.NewKeyFromSeed(privateKey)
	return ed25519.Sign(fullPriv, txData), nil
}

func (s *SOLAdapter) VerifySignature(publicKey []byte, txData []byte, signature []byte) (bool, error) {
	if len(publicKey) != 32 {
		return false, errors.New("invalid Solana public key length")
	}
	return ed25519.Verify(publicKey, txData, signature), nil
}

// TronAdapter implements Tron standard address formats (Prefix T, base58 check)
type TronAdapter struct{}

func (t *TronAdapter) NetworkName() string { return "Tron" }
func (t *TronAdapter) DerivationPath() string { return "m/44'/195'/0'/0/0" }

func (t *TronAdapter) ValidateAddress(address string) bool {
	if len(address) != 34 || !strings.HasPrefix(address, "T") {
		return false
	}
	re := regexp.MustCompile("^[1-9A-HJ-NP-Za-km-z]+$")
	return re.MatchString(address)
}

func (t *TronAdapter) DerivePublicKey(privateKey []byte) []byte {
	return GetPubKeyFromPriv(privateKey)
}

func (t *TronAdapter) GenerateAddress(privateKey []byte) (string, error) {
	pubKey := t.DerivePublicKey(privateKey)
	hash := sha256.Sum256(pubKey)
	payload := append([]byte{0x41}, hash[12:]...) // 0x41 prefix + last 20 bytes

	// Checksum
	h1 := sha256.Sum256(payload)
	h2 := sha256.Sum256(h1[:])
	fullPayload := append(payload, h2[:4]...)

	return EncodeBase58(fullPayload), nil
}

func (t *TronAdapter) SignTransaction(privateKey []byte, txData []byte) ([]byte, error) {
	hash := sha256.Sum256(txData)
	privKey, _ := btcec.PrivKeyFromBytes(privateKey)
	sig := ecdsa.Sign(privKey, hash[:])
	return sig.Serialize(), nil
}

func (t *TronAdapter) VerifySignature(publicKey []byte, txData []byte, signature []byte) (bool, error) {
	hash := sha256.Sum256(txData)
	pubKey, err := btcec.ParsePubKey(publicKey)
	if err != nil {
		return false, err
	}
	sig, err := ecdsa.ParseSignature(signature)
	if err != nil {
		return false, err
	}
	return sig.Verify(hash[:], pubKey), nil
}

// LTCAdapter implements Litecoin address structures
type LTCAdapter struct{}

func (l *LTCAdapter) NetworkName() string { return "Litecoin" }
func (l *LTCAdapter) DerivationPath() string { return "m/44'/2'/0'/0/0" }

func (l *LTCAdapter) ValidateAddress(address string) bool {
	if len(address) < 26 || len(address) > 43 {
		return false
	}
	return strings.HasPrefix(address, "L") || strings.HasPrefix(address, "M") || strings.HasPrefix(address, "ltc1")
}

func (l *LTCAdapter) DerivePublicKey(privateKey []byte) []byte {
	return GetPubKeyFromPriv(privateKey)
}

func (l *LTCAdapter) GenerateAddress(privateKey []byte) (string, error) {
	pubKey := l.DerivePublicKey(privateKey)
	shaHash := sha256.Sum256(pubKey)

	rp := ripemd160.New()
	_, _ = rp.Write(shaHash[:])
	ripeHash := rp.Sum(nil)

	// Version byte (0x30 for Litecoin Mainnet 'L')
	versionedPayload := append([]byte{0x30}, ripeHash...)

	firstSHA := sha256.Sum256(versionedPayload)
	secondSHA := sha256.Sum256(firstSHA[:])
	checksum := secondSHA[:4]

	fullPayload := append(versionedPayload, checksum...)
	return EncodeBase58(fullPayload), nil
}

func (l *LTCAdapter) SignTransaction(privateKey []byte, txData []byte) ([]byte, error) {
	hash := sha256.Sum256(txData)
	privKey, _ := btcec.PrivKeyFromBytes(privateKey)
	sig := ecdsa.Sign(privKey, hash[:])
	return sig.Serialize(), nil
}

func (l *LTCAdapter) VerifySignature(publicKey []byte, txData []byte, signature []byte) (bool, error) {
	hash := sha256.Sum256(txData)
	pubKey, err := btcec.ParsePubKey(publicKey)
	if err != nil {
		return false, err
	}
	sig, err := ecdsa.ParseSignature(signature)
	if err != nil {
		return false, err
	}
	return sig.Verify(hash[:], pubKey), nil
}

// Registry handles modular registration and discovery of Blockchain Adapters
type AdapterRegistry struct {
	adapters map[string]BlockchainAdapter
}

func NewAdapterRegistry() *AdapterRegistry {
	r := &AdapterRegistry{
		adapters: make(map[string]BlockchainAdapter),
	}
	r.Register(new(BTCAdapter))
	r.Register(&EVMAdapter{Network: "Ethereum", Path: "m/44'/60'/0'/0/0"})
	r.Register(&EVMAdapter{Network: "BNB Smart Chain", Path: "m/44'/60'/0'/0/0"})
	r.Register(&EVMAdapter{Network: "Polygon", Path: "m/44'/60'/0'/0/0"})
	r.Register(&EVMAdapter{Network: "Avalanche", Path: "m/44'/60'/0'/0/0"})
	r.Register(new(SOLAdapter))
	r.Register(new(TronAdapter))
	r.Register(new(LTCAdapter))
	return r
}

func (ar *AdapterRegistry) Register(adapter BlockchainAdapter) {
	ar.adapters[strings.ToLower(adapter.NetworkName())] = adapter
}

func (ar *AdapterRegistry) Get(network string) (BlockchainAdapter, error) {
	adapter, exists := ar.adapters[strings.ToLower(network)]
	if !exists {
		return nil, fmt.Errorf("unsupported network configuration: %s", network)
	}
	return adapter, nil
}
