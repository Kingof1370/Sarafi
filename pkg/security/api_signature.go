package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"sync"
	"time"
)

type APISignature struct {
	timestamp  string
	nonce      string
	method     string
	path       string
	body       string
	signature  string
	targetHash string
}

type SignatureValidator struct {
	secretKey  []byte
	algorithm  string
	nonceCache map[string]time.Time
	mutex      *sync.RWMutex
}

func NewSignatureValidator(secretKey string, algorithm string) (*SignatureValidator, error) {
	if algorithm != "SHA256" && algorithm != "SHA512" {
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	return &SignatureValidator{
		secretKey:  []byte(secretKey),
		algorithm:  algorithm,
		nonceCache: make(map[string]time.Time),
		mutex:      &sync.RWMutex{},
	}, nil
}

func (sv *SignatureValidator) ComputeSignature(timestamp, nonce, method, path, body string) (string, error) {
	sv.mutex.RLock()
	defer sv.mutex.RUnlock()

	canonicalRequest := timestamp + "\n" + nonce + "\n" + method + "\n" + path + "\n" + body

	var h hash.Hash
	switch sv.algorithm {
	case "SHA256":
		h = hmac.New(sha256.New, sv.secretKey)
	case "SHA512":
		h = hmac.New(sha512.New, sv.secretKey)
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", sv.algorithm)
	}

	h.Write([]byte(canonicalRequest))
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (sv *SignatureValidator) ValidateSignature(timestamp, nonce, method, path, body, providedSignature string) (bool, error) {
	if !sv.validateTimestamp(timestamp) {
		return false, fmt.Errorf("invalid timestamp: request too old")
	}

	if sv.isNonceReplayed(nonce) {
		return false, fmt.Errorf("nonce already used: replay attack detected")
	}

	expectedSignature, err := sv.ComputeSignature(timestamp, nonce, method, path, body)
	if err != nil {
		return false, fmt.Errorf("failed to compute signature: %w", err)
	}

	isValid := hmac.Equal([]byte(providedSignature), []byte(expectedSignature))
	if !isValid {
		return false, fmt.Errorf("signature mismatch")
	}

	sv.recordNonce(nonce)
	return true, nil
}

func (sv *SignatureValidator) validateTimestamp(timestamp string) bool {
	// Ensure request timestamp is within 5 minutes
	requestTime, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return false
	}

	timeDiff := time.Since(requestTime)
	return timeDiff >= -5*time.Minute && timeDiff <= 5*time.Minute
}

func (sv *SignatureValidator) isNonceReplayed(nonce string) bool {
	sv.mutex.RLock()
	defer sv.mutex.RUnlock()

	_, exists := sv.nonceCache[nonce]
	return exists
}

func (sv *SignatureValidator) recordNonce(nonce string) {
	sv.mutex.Lock()
	defer sv.mutex.Unlock()

	sv.nonceCache[nonce] = time.Now().Add(10 * time.Minute)

	// Clean up old nonces
	for key, expiry := range sv.nonceCache {
		if time.Now().After(expiry) {
			delete(sv.nonceCache, key)
		}
	}
}

func (sv *SignatureValidator) SignRequest(timestamp, nonce, method, path, body string) (string, error) {
	return sv.ComputeSignature(timestamp, nonce, method, path, body)
}
