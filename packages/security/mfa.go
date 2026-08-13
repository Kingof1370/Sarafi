package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// ValidateTOTP verifies a 6-digit TOTP code against a Base32-encoded secret.
// Implements RFC 6238 time-based one-time password standard.
func ValidateTOTP(secret, code string) bool {
	// Clean up secret format
	secret = strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		// Try standard padding as fallback
		key, err = base32.StdEncoding.DecodeString(secret)
		if err != nil {
			return false
		}
	}

	// Validate code length
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}

	// Check current time step and adjacent windows (±1 steps) to accommodate small clock skews
	currentTime := time.Now().Unix()
	timeStep := int64(30)

	for i := -1; i <= 1; i++ {
		counter := (currentTime / timeStep) + int64(i)
		if generateHOTP(key, counter) == code {
			return true
		}
	}

	return false
}

// GenerateTOTPSecret generates a new cryptographically random 32-character Base32 secret seed and backup codes
func GenerateTOTPSecret() (string, []string, error) {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return "", nil, fmt.Errorf("failed to read secure random bytes: %w", err)
	}

	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)

	// Generate 3 unique cryptographic backup codes in "xxxx-xxxx" format
	backupCodes := make([]string, 3)
	for i := 0; i < 3; i++ {
		codeBytes := make([]byte, 8)
		if _, err := rand.Read(codeBytes); err != nil {
			return "", nil, err
		}
		hexStr := hex.EncodeToString(codeBytes)
		backupCodes[i] = fmt.Sprintf("%s-%s", hexStr[:4], hexStr[4:8])
	}

	return secret, backupCodes, nil
}

// generateHOTP generates a 6-digit HOTP code for a given counter.
func generateHOTP(key []byte, counter int64) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(counter))

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	// Dynamic truncation (RFC 4226)
	offset := sum[len(sum)-1] & 0xf
	binaryVal := binary.BigEndian.Uint32(sum[offset : offset+4])
	binaryVal = binaryVal & 0x7fffffff

	otp := binaryVal % 1000000
	return fmt.Sprintf("%06d", otp)
}
