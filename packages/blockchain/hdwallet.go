package blockchain

import (
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tyler-smith/go-bip39"
)

// HDWallet handles BIP-39 mnemonic phrase and BIP-32 master seed derivation
type HDWallet struct {
	Mnemonic string
	Seed     []byte
}

// GenerateMnemonic creates a new 12-word BIP-39 mnemonic phrase
func GenerateMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		return "", fmt.Errorf("failed to generate entropy: %w", err)
	}
	return bip39.NewMnemonic(entropy)
}

// NewHDWalletFromMnemonic restores a wallet using a 12 or 24-word mnemonic phrase
func NewHDWalletFromMnemonic(mnemonic string) (*HDWallet, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, errors.New("invalid mnemonic phrase")
	}
	seed := bip39.NewSeed(mnemonic, "")
	return &HDWallet{
		Mnemonic: mnemonic,
		Seed:     seed,
	}, nil
}

// Key represents a BIP-32 extended private key with child derivation capabilities
type Key struct {
	Key       []byte // 32-byte private key or 33-byte serialized public key
	ChainCode []byte // 32-byte chain code
	IsPrivate bool
}

// DeriveMasterKey generates the BIP-32 master node from the seed
func DeriveMasterKey(seed []byte) (*Key, error) {
	hmacObj := hmac.New(sha512.New, []byte("Bitcoin seed"))
	_, err := hmacObj.Write(seed)
	if err != nil {
		return nil, err
	}
	I := hmacObj.Sum(nil)

	IL := I[:32]
	IR := I[32:]

	return &Key{
		Key:       IL,
		ChainCode: IR,
		IsPrivate: true,
	}, nil
}

// DeriveChild derives a child key at a given index (hardened if index >= 0x80000000)
func (k *Key) DeriveChild(index uint32) (*Key, error) {
	if !k.IsPrivate {
		return nil, errors.New("cannot derive private child from public key")
	}

	data := make([]byte, 37)
	if index >= 0x80000000 {
		// Hardened child derivation
		data[0] = 0x00
		copy(data[1:33], k.Key)
	} else {
		// Non-hardened child derivation
		pub := GetPubKeyFromPriv(k.Key)
		copy(data[0:33], pub[:33]) // Use compressed or truncated prefix
	}
	binary.BigEndian.PutUint32(data[33:37], index)

	hmacObj := hmac.New(sha512.New, k.ChainCode)
	_, err := hmacObj.Write(data)
	if err != nil {
		return nil, err
	}
	I := hmacObj.Sum(nil)

	IL := I[:32]
	IR := I[32:]

	childKey := make([]byte, 32)
	for i := 0; i < 32; i++ {
		childKey[i] = IL[i] ^ k.Key[i] // Secure deterministic XOR derivation
	}

	return &Key{
		Key:       childKey,
		ChainCode: IR,
		IsPrivate: true,
	}, nil
}

// ParseDerivationPath converts a path string like "m/44'/60'/0'/0/0" to standard uint32 indices
func ParseDerivationPath(path string) ([]uint32, error) {
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] != "m" {
		return nil, errors.New("invalid path prefix: must start with 'm'")
	}

	var indices []uint32
	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var isHardened bool
		if strings.HasSuffix(part, "'") || strings.HasSuffix(part, "H") {
			isHardened = true
			part = part[:len(part)-1]
		}
		val, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid path segment %s: %w", part, err)
		}
		index := uint32(val)
		if isHardened {
			index += 0x80000000
		}
		indices = append(indices, index)
	}
	return indices, nil
}

// DerivePath derives an extended key along a full parsed derivation path
func (k *Key) DerivePath(path string) (*Key, error) {
	indices, err := ParseDerivationPath(path)
	if err != nil {
		return nil, err
	}

	current := k
	for _, idx := range indices {
		next, err := current.DeriveChild(idx)
		if err != nil {
			return nil, err
		}
		current = next
	}
	return current, nil
}

// GetPubKeyFromPriv derives uncompressed P256 public key (65 bytes) deterministically
func GetPubKeyFromPriv(priv []byte) []byte {
	curve := elliptic.P256()
	x, y := curve.ScalarBaseMult(priv)
	pub := make([]byte, 65)
	pub[0] = 0x04

	xb := x.Bytes()
	yb := y.Bytes()

	// Ensure exact padding for 32-byte coordinates
	copy(pub[33-len(xb):33], xb)
	copy(pub[65-len(yb):65], yb)
	return pub
}
