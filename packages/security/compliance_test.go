package security

import (
	"context"
	"strings"
	"testing"
)

func TestComplianceEngine_KYCFlows(t *testing.T) {
	// Initialize with nil DB and nil Kafka for local unit-test isolation
	engine := NewComplianceEngine(nil, nil)

	ctx := context.Background()
	userID := "user_test_kyc_01"

	// 1. Submit KYC Basic Profile
	profile, err := engine.SubmitKYC(ctx, userID, TierStandard, "{\"document_type\":\"passport\",\"id\":\"XYZ987\"}")
	if err != nil {
		t.Fatalf("Failed to submit KYC: %v", err)
	}

	if profile.UserID != userID || profile.Tier != TierStandard {
		t.Errorf("Unexpected profile values: %+v", profile)
	}
	if profile.Status != StatusInReview {
		t.Errorf("Expected status IN_REVIEW, got %s", profile.Status)
	}
	if profile.Attempts != 1 {
		t.Errorf("Expected attempts 1, got %d", profile.Attempts)
	}

	// 2. Fetch KYC
	profileFetched, err := engine.GetKYC(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get KYC: %v", err)
	}
	if profileFetched.Status != StatusInReview {
		t.Errorf("Expected fetched status IN_REVIEW, got %s", profileFetched.Status)
	}

	// 3. Review KYC -> Reject
	profile, err = engine.ReviewKYC(ctx, userID, StatusRejected, "Blurry document selfie image", "compliance_officer_01")
	if err != nil {
		t.Fatalf("Failed to reject KYC: %v", err)
	}
	if profile.Status != StatusRejected || profile.RejectionReason != "Blurry document selfie image" {
		t.Errorf("Unexpected rejection states: %+v", profile)
	}

	// 4. Submit again -> Attempts count should increment
	profile, err = engine.SubmitKYC(ctx, userID, TierStandard, "{\"document_type\":\"passport\",\"id\":\"XYZ987_fixed\"}")
	if err != nil {
		t.Fatalf("Failed resubmission: %v", err)
	}
	if profile.Attempts != 2 {
		t.Errorf("Expected attempts 2, got %d", profile.Attempts)
	}

	// 5. Review KYC -> Verify
	profile, err = engine.ReviewKYC(ctx, userID, StatusVerified, "Validated successfully", "compliance_officer_01")
	if err != nil {
		t.Fatalf("Failed to verify KYC: %v", err)
	}
	if profile.Status != StatusVerified {
		t.Errorf("Expected status VERIFIED, got %s", profile.Status)
	}
	if profile.VerifiedAt == nil || profile.ExpiresAt == nil {
		t.Error("Verification or expiry timestamps are nil")
	}
}

func TestComplianceEngine_AML_And_Limits(t *testing.T) {
	engine := NewComplianceEngine(nil, nil)
	ctx := context.Background()
	userID := "user_test_aml"

	// 1. Evaluate normal amount (no alerts)
	alerts, err := engine.EvaluateAML(ctx, userID, 150.0, "USDT", "WITHDRAWAL", "tx_01")
	if err != nil {
		t.Fatalf("EvaluateAML failed: %v", err)
	}
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts for normal amount, got %d", len(alerts))
	}

	// 2. Evaluate suspicious amount (should trigger Alert)
	alerts, err = engine.EvaluateAML(ctx, userID, 6000.0, "USDT", "WITHDRAWAL", "tx_02")
	if err != nil {
		t.Fatalf("EvaluateAML failed: %v", err)
	}
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert for large amount, got %d", len(alerts))
	}
	if alerts[0].RuleTriggered != "aml_rule_unusual_amount" {
		t.Errorf("Expected aml_rule_unusual_amount, got %s", alerts[0].RuleTriggered)
	}
	if alerts[0].Severity != SeverityHigh {
		t.Errorf("Expected SeverityHigh, got %s", alerts[0].Severity)
	}

	// 3. Test KYC Limits daily checks
	err = engine.CheckLimits(ctx, userID, 500.0, "WITHDRAWAL")
	if err != nil {
		t.Errorf("Expected no limits error for Basic tier under $1000 limit, got: %v", err)
	}
}

func TestComplianceEngine_RiskScoring(t *testing.T) {
	engine := NewComplianceEngine(nil, nil)
	ctx := context.Background()
	userID := "user_test_risk"

	// Default unverified user risk check
	eval, err := engine.EvaluateRiskScore(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to evaluate risk score: %v", err)
	}
	if eval.Score <= 0.0 || eval.RiskLevel == "" {
		t.Errorf("Invalid risk evaluation: %+v", eval)
	}
	if !strings.Contains(eval.ContributingRules, "kyc_status") {
		t.Errorf("Expected explanation factor 'kyc_status', got %s", eval.ContributingRules)
	}

	// Restrict and re-evaluate
	_ = engine.RestrictAccount(ctx, userID, RestrictionMonitor, "Alert trigger watch status", "SYSTEM")
	eval2, err := engine.EvaluateRiskScore(ctx, userID)
	if err != nil {
		t.Fatalf("Failed second evaluation: %v", err)
	}
	if eval2.Score <= eval.Score {
		t.Errorf("Expected higher risk score for restricted account. First: %f, Second: %f", eval.Score, eval2.Score)
	}
}

func TestComplianceEngine_Sanctions_FailClosed(t *testing.T) {
	ctx := context.Background()
	userID := "user_test_sanctions"

	// 1. Simulation Mode
	engineSim := NewComplianceEngine(nil, nil)
	engineSim.mode = "simulation"
	engineSim.sanctionsProvider.(*DefaultMockSanctionsProvider).Mode = "simulation"

	status, _, err := engineSim.ScreenSanctions(ctx, "ADDRESS_WITHDRAWAL", "0xCLEAR_ADDR")
	if err != nil {
		t.Fatalf("ScreenSanctions simulation error: %v", err)
	}
	if status != SanctionsClear {
		t.Errorf("Expected CLEAR address status, got %s", status)
	}

	status, _, err = engineSim.ScreenSanctions(ctx, "ADDRESS_WITHDRAWAL", "0xBLACKLISTED")
	if err != nil {
		t.Fatalf("ScreenSanctions simulation error: %v", err)
	}
	if status != SanctionsMatch {
		t.Errorf("Expected MATCH address status, got %s", status)
	}

	// 2. Production Mode - Failure Closed validation
	engineProd := NewComplianceEngine(nil, nil)
	engineProd.mode = "production"
	engineProd.sanctionsProvider.(*DefaultMockSanctionsProvider).Mode = "production"

	status, _, err = engineProd.ScreenSanctions(ctx, "ADDRESS_WITHDRAWAL", "0xANY_ADDR")
	// The provider in production mock will timeout and return error
	if err == nil {
		t.Error("Expected sanctions provider to return error under production offline simulation")
	}
	if status == SanctionsClear {
		t.Error("FAIL-CLOSED CRITICAL BREACH: Error state translated directly into CLEAR")
	}

	// Double-check withdrawal gate blocks
	err = engineProd.CheckWithdrawalAllowed(ctx, userID, 100.0, "BTC", "0xANY_ADDR")
	if err == nil {
		t.Error("Expected withdrawal gate to BLOCK when sanctions screening fails/errors (closed loop)")
	}
}

func TestComplianceEngine_TravelRule(t *testing.T) {
	engine := NewComplianceEngine(nil, nil)
	ctx := context.Background()

	// Complete Travel info
	err := engine.RegisterTravelRule(ctx, "tx_tr_01", "Alice Smith", "123 Lane St", "ACC123", "Bob Jones", "456 Main St", "ACC456")
	if err != nil {
		t.Errorf("Expected complete travel rule to succeed, got error: %v", err)
	}

	// Missing Travel info
	err = engine.RegisterTravelRule(ctx, "tx_tr_02", "", "123 Lane St", "ACC123", "Bob Jones", "456 Main St", "ACC456")
	if err == nil {
		t.Error("Expected incomplete travel rule registration to return error")
	}
}

func TestComplianceEngine_CaseManagement(t *testing.T) {
	engine := NewComplianceEngine(nil, nil)
	ctx := context.Background()
	userID := "user_test_cases"

	// Create alerts
	alt1, _ := engine.CreateAlert(ctx, userID, "tx_c1", "aml_rule_unusual_amount", 0.65, "Evidence description", SeverityHigh)
	alt2, _ := engine.CreateAlert(ctx, userID, "tx_c2", "aml_rule_velocity_tx", 0.50, "Evidence description 2", SeverityMedium)

	// Create manual compliance case
	c, err := engine.CreateCase(ctx, userID, []string{alt1.ID, alt2.ID})
	if err != nil {
		t.Fatalf("CreateCase failed: %v", err)
	}
	if c.Status != CaseOpen {
		t.Errorf("Expected Case status OPEN, got %s", c.Status)
	}

	// Add Investigator audit Note
	note, err := engine.AddCaseNote(ctx, c.ID, "investigator_01", "Investigating funds sources and KYC file details")
	if err != nil {
		t.Fatalf("AddCaseNote failed: %v", err)
	}
	if note.Note != "Investigating funds sources and KYC file details" {
		t.Errorf("Unexpected note: %+v", note)
	}

	// Resolve Case
	cResolved, err := engine.ResolveCase(ctx, c.ID, CaseResolved, "Verified that funds are sourced from legal salary slips", "investigator_01")
	if err != nil {
		t.Fatalf("ResolveCase failed: %v", err)
	}
	if cResolved.Status != CaseResolved || cResolved.Resolution != "Verified that funds are sourced from legal salary slips" {
		t.Errorf("Unexpected resolved state: %+v", cResolved)
	}
}
