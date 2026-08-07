package security

import (
	"testing"
	"time"
)

func TestPolicyEngineRBAC_ABAC(t *testing.T) {
	pe := NewPolicyEngine()

	ctxNormal := &ContextAttributes{
		ClientIP:          "192.168.1.100",
		GeoLocation:       "US",
		DeviceFingerprint: "fingerprint_chrome_99",
		RequestTime:       time.Now(),
		RiskScore:         0.2,
		SessionAgeSeconds: 300,
	}

	// 1. Admin should be authorized for wallets with low risk
	allowed, err := pe.Evaluate(RoleAdmin, ResourceWallet, ActionRead, ctxNormal)
	if err != nil {
		t.Fatalf("unexpected error during evaluation: %v", err)
	}
	if !allowed {
		t.Error("expected Admin to be authorized for WALLET READ under normal context")
	}

	// 2. Guest should not be allowed to perform admin actions
	allowed, _ = pe.Evaluate(RoleGuest, ResourceKeys, ActionRotate, ctxNormal)
	if allowed {
		t.Error("expected Guest to be blocked from key rotation")
	}

	// 3. User with high risk score should be blocked
	ctxHighRisk := &ContextAttributes{
		ClientIP:          "192.168.1.100",
		GeoLocation:       "US",
		DeviceFingerprint: "fingerprint_chrome_99",
		RequestTime:       time.Now(),
		RiskScore:         0.9,
		SessionAgeSeconds: 300,
	}
	allowed, _ = pe.Evaluate(RoleAdmin, ResourceWallet, ActionRead, ctxHighRisk)
	if allowed {
		t.Error("expected high-risk Admin request to be blocked")
	}

	// 4. Blacklisted IP should be blocked
	pe.BlacklistIP("203.0.113.50")
	ctxBlockedIP := &ContextAttributes{
		ClientIP:          "203.0.113.50",
		GeoLocation:       "RU",
		DeviceFingerprint: "fingerprint_unknown",
		RequestTime:       time.Now(),
		RiskScore:         0.1,
		SessionAgeSeconds: 10,
	}
	allowed, _ = pe.Evaluate(RoleAdmin, ResourceWallet, ActionRead, ctxBlockedIP)
	if allowed {
		t.Error("expected blacklisted IP request to be blocked")
	}

	// 5. Stale session should be blocked
	ctxStale := &ContextAttributes{
		ClientIP:          "192.168.1.100",
		GeoLocation:       "US",
		DeviceFingerprint: "fingerprint_chrome_99",
		RequestTime:       time.Now(),
		RiskScore:         0.1,
		SessionAgeSeconds: 5000, // > 1 hour
	}
	allowed, _ = pe.Evaluate(RoleAdmin, ResourceWallet, ActionRead, ctxStale)
	if allowed {
		t.Error("expected stale session request to be blocked")
	}
}
