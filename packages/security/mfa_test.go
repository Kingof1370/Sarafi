package security

import (
	"encoding/base32"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestMFASecretGenerationAndEncryption(t *testing.T) {
	secret, err := GenerateMFASecret()
	if err != nil {
		t.Fatalf("Failed to generate MFA secret: %v", err)
	}

	if len(secret) != 32 {
		t.Errorf("Expected 32-character Base32 secret string, got length %d", len(secret))
	}

	key := []byte("my-super-secret-key-for-testing")
	encrypted, err := EncryptSecret(secret, key)
	if err != nil {
		t.Fatalf("Failed to encrypt secret: %v", err)
	}

	decrypted, err := DecryptSecret(encrypted, key)
	if err != nil {
		t.Fatalf("Failed to decrypt secret: %v", err)
	}

	if decrypted != secret {
		t.Errorf("Decrypted secret did not match original. Expected %s, got %s", secret, decrypted)
	}
}

func TestTOTPVerificationAndDrift(t *testing.T) {
	secret := "MZZG653OEBTG66KNEB2A" // Valid Base32 string

	// Standard Base32 decode
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatalf("Failed to base32 decode static secret: %v", err)
	}

	interval := int64(30)
	currentTimestamp := time.Now().Unix()
	timeStep := currentTimestamp / interval

	// Calculate code via calculateHOTP
	code := calculateHOTP(decoded, timeStep)

	// Since VerifyTOTP performs decode, let's verify
	isValid := VerifyTOTP(secret, code)
	if !isValid {
		t.Errorf("VerifyTOTP failed to validate correct standard code %s under valid static secret %s", code, secret)
	} else {
		t.Logf("VerifyTOTP successfully validated code %s", code)
	}
}

func TestReplayAttackPrevention(t *testing.T) {
	userID := "usr_replay_test"
	code := "123456"

	if IsReplayAttack(userID, code) {
		t.Error("First usage of code should not be classified as a replay attack")
	}

	if !IsReplayAttack(userID, code) {
		t.Error("Subsequent reuse of same code should trigger replay attack blocker")
	}
}

func TestMFABruteForceAndRateLimiting(t *testing.T) {
	userID := "usr_brute_test"

	locked, duration := CheckMFAVerifyRateLimit(userID)
	if locked {
		t.Errorf("New user should not be locked out. Got duration: %v", duration)
	}

	// Record 5 failed attempts
	for i := 0; i < 5; i++ {
		RecordMFAFailure(userID)
	}

	locked, duration = CheckMFAVerifyRateLimit(userID)
	if !locked {
		t.Error("User should be locked out after 5 failures")
	}
	if duration <= 14*time.Minute {
		t.Errorf("Lockout duration should be approximately 15 minutes, got %v", duration)
	}

	RecordMFASuccess(userID)
	locked, _ = CheckMFAVerifyRateLimit(userID)
	if locked {
		t.Error("User lockout should be cleared after a successful verification recording")
	}
}

func TestBackupCodesGeneration(t *testing.T) {
	raw, hashed, err := GenerateBackupCodes()
	if err != nil {
		t.Fatalf("Failed to generate backup codes: %v", err)
	}

	if len(raw) != 8 || len(hashed) != 8 {
		t.Errorf("Expected 8 codes, got raw %d and hashed %d", len(raw), len(hashed))
	}

	// Test bcrypt matching
	testRaw := raw[0]
	testHashed := hashed[0]

	err = bcrypt.CompareHashAndPassword([]byte(testHashed), []byte(testRaw))
	if err != nil {
		t.Errorf("Bcrypt failed to match backup code raw representation with hashed counterpart: %v", err)
	}
}
