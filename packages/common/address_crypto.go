package common

import (
	"crypto/sha256"
	"math/big"
)

// Base58 Alphabet
const b58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// EncodeBase58 encodes a byte slice into Base58
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

	// Reverse result
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

// EncodeBase58Check encodes version + payload + double SHA256 checksum
func EncodeBase58Check(version byte, payload []byte) string {
	data := append([]byte{version}, payload...)
	hash1 := sha256.Sum256(data)
	hash2 := sha256.Sum256(hash1[:])
	checksum := hash2[:4]
	full := append(data, checksum...)
	return EncodeBase58(full)
}
