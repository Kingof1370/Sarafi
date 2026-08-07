package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// ReplayProtectionTracker ensures no requests can be replayed by tracking nonces and request timestamps
type ReplayProtectionTracker struct {
	mu           sync.RWMutex
	seenNonces   map[string]time.Time
	cleanupTimer *time.Ticker
}

// NewReplayProtectionTracker creates a thread-safe replay attacker detector
func NewReplayProtectionTracker() *ReplayProtectionTracker {
	rpt := &ReplayProtectionTracker{
		seenNonces: make(map[string]time.Time),
	}
	// Start automatic cleanup for expired nonces
	go rpt.startCleanupLoop()
	return rpt
}

func (rpt *ReplayProtectionTracker) startCleanupLoop() {
	rpt.cleanupTimer = time.NewTicker(10 * time.Minute)
	for range rpt.cleanupTimer.C {
		rpt.mu.Lock()
		now := time.Now()
		for nonce, timestamp := range rpt.seenNonces {
			// If nonce is older than 5 minutes, we can safely delete it because the timestamp window will block it anyway
			if now.Sub(timestamp) > 5*time.Minute {
				delete(rpt.seenNonces, nonce)
			}
		}
		rpt.mu.Unlock()
	}
}

// VerifyRequest verifies the request timestamp is within 5 minutes and the nonce is unique
func (rpt *ReplayProtectionTracker) VerifyRequest(nonce string, timestamp time.Time) (bool, error) {
	rpt.mu.Lock()
	defer rpt.mu.Unlock()

	// 1. Clock skew verification (Reject if request is older than 5 minutes or in the future)
	now := time.Now()
	if now.Sub(timestamp) > 5*time.Minute || timestamp.Sub(now) > 1*time.Minute {
		return false, errors.New("request timestamp is outside the allowed skew window (5 minutes)")
	}

	// 2. Nonce reuse verification
	if _, exists := rpt.seenNonces[nonce]; exists {
		return false, errors.New("security violation: request nonce has already been seen (replay attack detected)")
	}

	rpt.seenNonces[nonce] = timestamp
	return true, nil
}

// TokenRotator manages hardened JWT Refresh Token Rotation
type TokenRotator struct {
	mu                    sync.RWMutex
	blacklistedRefreshes  map[string]time.Time
}

// NewTokenRotator initializes a thread-safe token rotation engine
func NewTokenRotator() *TokenRotator {
	return &TokenRotator{
		blacklistedRefreshes: make(map[string]time.Time),
	}
}

// InvalidateRefreshToken blacklists a refresh token (used to perform rotation or immediate logout)
func (tr *TokenRotator) InvalidateRefreshToken(token string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.blacklistedRefreshes[token] = time.Now()
}

// IsRefreshTokenBlacklisted checks if a refresh token has been used/invalidated
func (tr *TokenRotator) IsRefreshTokenBlacklisted(token string) bool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	_, exists := tr.blacklistedRefreshes[token]
	return exists
}

// CSRFManager handles Cross-Site Request Forgery state tokens
type CSRFManager struct {
	mu         sync.RWMutex
	activeCSRF map[string]time.Time
}

// NewCSRFManager initializes anti-CSRF state token validation
func NewCSRFManager() *CSRFManager {
	return &CSRFManager{
		activeCSRF: make(map[string]time.Time),
	}
}

// GenerateCSRFToken provisions a secure cryptographically random anti-CSRF token
func (cm *CSRFManager) GenerateCSRFToken() (string, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	entropy := make([]byte, 32)
	_, err := rand.Read(entropy)
	if err != nil {
		return "", err
	}

	token := hex.EncodeToString(entropy)
	cm.activeCSRF[token] = time.Now().Add(30 * time.Minute) // 30 mins validity
	return token, nil
}

// ValidateCSRFToken verifies and consumes an active anti-CSRF token
func (cm *CSRFManager) ValidateCSRFToken(token string) (bool, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	expiresAt, exists := cm.activeCSRF[token]
	if !exists {
		return false, errors.New("invalid or inactive CSRF token")
	}

	// Consume token immediately (one-time use standard)
	delete(cm.activeCSRF, token)

	if time.Now().After(expiresAt) {
		return false, errors.New("CSRF token has expired")
	}

	return true, nil
}

// SignMessageHMAC computes standard HMAC-SHA256 signature for API Request/Response signing
func SignMessageHMAC(payload []byte, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyMessageHMAC verifies a request/response data integrity signature
func VerifyMessageHMAC(payload []byte, secret []byte, providedSignature string) bool {
	expectedSig := SignMessageHMAC(payload, secret)
	return hmac.Equal([]byte(expectedSig), []byte(providedSignature))
}
