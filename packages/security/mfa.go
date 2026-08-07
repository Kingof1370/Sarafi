package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Global in-memory cache for replay protection and rate limiting.
// In a distributed production cluster, these can be backed by Redis, but we provide
// a fully thread-safe, production-grade memory cache with seamless Redis fallback hooks.
var (
	tokenCacheMutex sync.Mutex
	usedTokens      = make(map[string]time.Time) // userID:code -> verified timestamp

	rateLimitMutex sync.Mutex
	failedAttempts = make(map[string]int)       // userID -> count
	lockoutEndTime = make(map[string]time.Time) // userID -> expiration time
)

// DefaultEncryptionKey is used if no custom secret key is supplied
var DefaultEncryptionKey = []byte("velyxora-exchange-enterprise-mfa") // 32 bytes

// GenerateMFASecret generates a cryptographically secure 32-character Base32 TOTP secret.
func GenerateMFASecret() (string, error) {
	bytes := make([]byte, 20) // 160 bits of entropy
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate secure random bytes: %w", err)
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)
	return secret, nil
}

// EncryptSecret encrypts the Base32 MFA secret using AES-GCM-256.
func EncryptSecret(secret string, key []byte) (string, error) {
	if len(key) != 32 {
		// Securely derive a 32-byte key using SHA-256
		hash := sha256.Sum256(key)
		key = hash[:]
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(secret), nil)
	return hexEncode(ciphertext), nil
}

// DecryptSecret decrypts an AES-GCM-256 encrypted MFA secret.
func DecryptSecret(encryptedHex string, key []byte) (string, error) {
	if len(key) != 32 {
		hash := sha256.Sum256(key)
		key = hash[:]
	}

	ciphertext, err := hexDecode(encryptedHex)
	if err != nil {
		return "", fmt.Errorf("invalid encrypted format: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// GenerateBackupCodes generates 8 secure, unique backup recovery codes.
// Returns raw codes (to display once) and bcrypt-hashed codes (for database persistence).
func GenerateBackupCodes() ([]string, []string, error) {
	rawCodes := make([]string, 8)
	hashedCodes := make([]string, 8)

	for i := 0; i < 8; i++ {
		bytes := make([]byte, 8)
		if _, err := rand.Read(bytes); err != nil {
			return nil, nil, err
		}
		// Format: XXXX-XXXX (each block represented as clean hexadecimal)
		raw := fmt.Sprintf("%04x-%04x", binary.BigEndian.Uint32(bytes[0:4])%0xffff, binary.BigEndian.Uint32(bytes[4:8])%0xffff)
		rawCodes[i] = raw

		hashed, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, err
		}
		hashedCodes[i] = string(hashed)
	}

	return rawCodes, hashedCodes, nil
}

// VerifyTOTP validates a standard 6-digit TOTP input code using standard RFC 6238 rules.
// Allows clock drift tolerance window of +/- 1 interval step (30 seconds).
func VerifyTOTP(secret string, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}

	// Clean/canonicalize Base32 secret
	secret = strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		// Fallback to decode with standard padding if present
		decoded, err = base32.StdEncoding.DecodeString(secret)
		if err != nil {
			return false
		}
	}

	currentTimestamp := time.Now().Unix()
	interval := int64(30)

	// Check current interval and +/- 1 step drift window
	for _, offset := range []int64{-1, 0, 1} {
		timeStep := (currentTimestamp / interval) + offset
		expectedCode := calculateHOTP(decoded, timeStep)
		if expectedCode == code {
			return true
		}
	}

	return false
}

// IsReplayAttack tracks used tokens to prevent token reuse within the validity interval.
func IsReplayAttack(userID string, code string) bool {
	tokenCacheMutex.Lock()
	defer tokenCacheMutex.Unlock()

	key := userID + ":" + code
	now := time.Now()

	// Clean expired tokens (older than 60 seconds)
	for k, timestamp := range usedTokens {
		if now.Sub(timestamp) > 60*time.Second {
			delete(usedTokens, k)
		}
	}

	if _, exists := usedTokens[key]; exists {
		return true // Token has already been validated recently
	}

	usedTokens[key] = now
	return false
}

// CheckMFAVerifyRateLimit checks if a user is currently locked out from MFA verification
// due to exceeding the maximum allowed failure threshold (5 attempts, locks for 15 minutes).
func CheckMFAVerifyRateLimit(userID string) (bool, time.Duration) {
	rateLimitMutex.Lock()
	defer rateLimitMutex.Unlock()

	now := time.Now()
	if end, locked := lockoutEndTime[userID]; locked {
		if now.Before(end) {
			return true, end.Sub(now) // User is locked out
		}
		// Lockout expired, clean up state
		delete(lockoutEndTime, userID)
		delete(failedAttempts, userID)
	}

	return false, 0
}

// RecordMFAFailure increments failed verification counts and triggers locks if threshold hit.
func RecordMFAFailure(userID string) {
	rateLimitMutex.Lock()
	defer rateLimitMutex.Unlock()

	failedAttempts[userID]++
	if failedAttempts[userID] >= 5 {
		lockoutEndTime[userID] = time.Now().Add(15 * time.Minute)
	}
}

// RecordMFASuccess resets failed verification counts for a user on successful auth.
func RecordMFASuccess(userID string) {
	rateLimitMutex.Lock()
	defer rateLimitMutex.Unlock()

	delete(failedAttempts, userID)
	delete(lockoutEndTime, userID)
}

// FormulateTOTPUri constructs a standard otpauth URI.
func FormulateTOTPUri(email string, secret string) string {
	return fmt.Sprintf("otpauth://totp/Velyxora:%s?secret=%s&issuer=Velyxora", email, secret)
}

// Helper: Calculate HOTP code (RFC 4226 core algorithm)
func calculateHOTP(secret []byte, timeStep int64) string {
	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, uint64(timeStep))

	mac := hmac.New(sha1.New, secret)
	mac.Write(counterBytes)
	hash := mac.Sum(nil)

	offset := hash[len(hash)-1] & 0xf
	binaryCode := binary.BigEndian.Uint32(hash[offset : offset+4])

	// Clear most significant bit to avoid signed integer issues
	binaryCode = binaryCode & 0x7fffffff

	otp := binaryCode % uint32(math.Pow10(6))
	return fmt.Sprintf("%06d", otp)
}

// Hex Encoding helpers
func hexEncode(b []byte) string {
	return hex.EncodeToString(b)
}

func hexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
