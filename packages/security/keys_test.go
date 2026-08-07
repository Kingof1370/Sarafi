package security

import (
	"testing"
	"time"
)

func TestKeyManagerAndHSM(t *testing.T) {
	hsm := NewSimulatedHSM()
	km := NewKeyManager(hsm)

	// 1. Generate Key in HSM
	pubBytes, err := hsm.GenerateHSMKey("my_hsm_key")
	if err != nil {
		t.Fatalf("GenerateHSMKey failed: %v", err)
	}
	if len(pubBytes) == 0 {
		t.Fatal("expected non-empty public key bytes")
	}

	// 2. Sign and verify with HSM
	payload := []byte("confidential_data_99")
	sig, err := hsm.SignWithHSM("my_hsm_key", payload)
	if err != nil {
		t.Fatalf("SignWithHSM failed: %v", err)
	}
	if len(sig) < 64 {
		t.Fatalf("expected signature length to be at least 64 bytes, got %d", len(sig))
	}

	verified, err := hsm.VerifyWithHSM("my_hsm_key", payload, sig)
	if err != nil {
		t.Fatalf("VerifyWithHSM failed: %v", err)
	}
	if !verified {
		t.Error("expected HSM signature verification to succeed")
	}

	// 3. API Key Lifecycle
	metadata, rawSecret, err := km.RegisterAPIKey("user_007", []string{"trade", "withdraw"}, "initial_pass", 1*time.Hour)
	if err != nil {
		t.Fatalf("RegisterAPIKey failed: %v", err)
	}

	// Validate valid API credentials
	ok, err := km.ValidateAPIKey(metadata.ID, rawSecret)
	if err != nil {
		t.Fatalf("ValidateAPIKey failed on valid credentials: %v", err)
	}
	if !ok {
		t.Error("expected API key to be valid")
	}

	// Revoke API key
	err = km.RevokeAPIKey(metadata.ID)
	if err != nil {
		t.Fatalf("RevokeAPIKey failed: %v", err)
	}

	// Validate revoked API key should fail
	ok, err = km.ValidateAPIKey(metadata.ID, rawSecret)
	if ok || err == nil {
		t.Error("expected revoked API key validation to fail")
	}

	// 4. Secret Store Rotation
	km.RotateSecret("secret_jwt_key", "old_val")
	retrieved, err := km.RetrieveSecret("secret_jwt_key")
	if err != nil || retrieved != "old_val" {
		t.Errorf("secret retrieval failed: err=%v, val=%s", err, retrieved)
	}

	km.RotateSecret("secret_jwt_key", "new_val")
	retrieved, _ = km.RetrieveSecret("secret_jwt_key")
	if retrieved != "new_val" {
		t.Errorf("secret rotation failed: expected 'new_val', got '%s'", retrieved)
	}
}

func TestAdaptiveRiskAuthentication(t *testing.T) {
	history := map[string]bool{
		"192.168.1.10": true,
	}

	// Normal check
	score := EvaluateAdaptiveRisk("192.168.1.10", "US", "chrome_fingerprint", history)
	if score > 0.3 {
		t.Errorf("expected low risk score, got %.2f", score)
	}

	// Malicious IP subnet
	score = EvaluateAdaptiveRisk("192.168.99.99", "US", "chrome_fingerprint", history)
	if score < 0.6 {
		t.Errorf("expected highly elevated risk score for blacklisted IP pool, got %.2f", score)
	}

	// Unknown IP, geo, and device
	score = EvaluateAdaptiveRisk("198.51.100.12", "UNKNOWN", "UNKNOWN", history)
	if score < 0.5 {
		t.Errorf("expected elevated risk score for unrecognized context attributes, got %.2f", score)
	}
}
