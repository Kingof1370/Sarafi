package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func TestHashAndPassword(t *testing.T) {
	pw := "secure123"
	hash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if !CheckPasswordHash(pw, hash) {
		t.Error("Password check should succeed with valid password")
	}

	if CheckPasswordHash("wrongpass", hash) {
		t.Error("Password check should fail with invalid password")
	}
}

func TestJWTGenerationAndValidation(t *testing.T) {
	secret := "super-secret-key-12345"
	userID := "user_uuid_1"
	email := "user@example.com"

	access, refresh, err := GenerateJWT(userID, email, secret, 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatalf("JWT generation failed: %v", err)
	}

	if access == "" || refresh == "" {
		t.Fatal("Expected non-empty tokens")
	}

	claims, err := ValidateJWT(access, secret)
	if err != nil {
		t.Fatalf("JWT validation failed: %v", err)
	}

	if claims.UserID != userID || claims.Email != email {
		t.Errorf("Expected claims to match user, got user_id=%v, email=%v", claims.UserID, claims.Email)
	}

	_, err = ValidateJWT(access, "wrong-secret")
	if err == nil {
		t.Error("Expected validation failure with incorrect secret key")
	}
}

func TestVerifyAPIKeySignature(t *testing.T) {
	secret := "apikeysecret123"
	payload := "timestamp=1609459200&recvWindow=5000&symbol=BTC-USDT"

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))

	if !VerifyAPIKeySignature(payload, signature, secret) {
		t.Error("Signature verification failed unexpectedly")
	}

	if VerifyAPIKeySignature(payload, "invalid_sig", secret) {
		t.Error("Signature verification should have failed with invalid signature")
	}
}
