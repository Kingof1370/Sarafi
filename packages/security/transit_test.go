package security

import (
	"testing"
	"time"
)

func TestTransitSecurity(t *testing.T) {
	// 1. HMAC Request/Response Signing
	payload := []byte("api_request_parameters_99")
	secret := []byte("system_transit_secret_123")
	sig := SignMessageHMAC(payload, secret)

	if !VerifyMessageHMAC(payload, secret, sig) {
		t.Error("expected HMAC verification to succeed on authentic payload")
	}

	if VerifyMessageHMAC([]byte("tampered payload"), secret, sig) {
		t.Error("expected HMAC verification to fail on tampered payload")
	}

	// 2. Replay Protection
	rpt := NewReplayProtectionTracker()
	nonce := "unique_nonce_abc"
	now := time.Now()

	ok, err := rpt.VerifyRequest(nonce, now)
	if !ok || err != nil {
		t.Fatalf("expected request validation to succeed: %v", err)
	}

	// Replay attack attempt
	ok, err = rpt.VerifyRequest(nonce, now)
	if ok || err == nil {
		t.Error("expected replay attack attempt to be caught and rejected")
	}

	// Timeout replay check (too old)
	ok, err = rpt.VerifyRequest("new_nonce", now.Add(-10*time.Minute))
	if ok || err == nil {
		t.Error("expected request outside time skew window to be rejected")
	}

	// 3. Token Rotation
	rot := NewTokenRotator()
	token := "refresh_token_xyz"
	if rot.IsRefreshTokenBlacklisted(token) {
		t.Error("expected token to not be blacklisted initially")
	}

	rot.InvalidateRefreshToken(token)
	if !rot.IsRefreshTokenBlacklisted(token) {
		t.Error("expected token to be blacklisted after invalidation")
	}

	// 4. CSRF Tokens
	csrf := NewCSRFManager()
	tokenCSRF, err := csrf.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("GenerateCSRFToken failed: %v", err)
	}

	valid, err := csrf.ValidateCSRFToken(tokenCSRF)
	if !valid || err != nil {
		t.Errorf("ValidateCSRFToken failed on valid token: %v", err)
	}

	// Consume test (token should be one-time use)
	valid, err = csrf.ValidateCSRFToken(tokenCSRF)
	if valid || err == nil {
		t.Error("expected token reuse to fail (one-time use standard)")
	}
}
