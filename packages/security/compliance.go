package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"velyxora/packages/common"
	"velyxora/packages/database"
	"velyxora/packages/logger"
)

// KYCTier represents customer tier level
type KYCTier string

const (
	TierBasic         KYCTier = "BASIC"
	TierStandard      KYCTier = "STANDARD"
	TierAdvanced      KYCTier = "ADVANCED"
	TierInstitutional KYCTier = "INSTITUTIONAL"
)

// KYCStatus represents customer verification state
type KYCStatus string

const (
	StatusPending        KYCStatus = "PENDING"
	StatusInReview       KYCStatus = "IN_REVIEW"
	StatusVerified       KYCStatus = "VERIFIED"
	StatusRejected       KYCStatus = "REJECTED"
	StatusExpired        KYCStatus = "EXPIRED"
	StatusSuspended      KYCStatus = "SUSPENDED"
	StatusRequiresReview KYCStatus = "REQUIRES_REVIEW"
)

// KYCProfile stores customer verification record
type KYCProfile struct {
	UserID           string     `json:"user_id"`
	Tier             KYCTier    `json:"tier"`
	Status           KYCStatus  `json:"status"`
	ProviderRef      string     `json:"provider_ref"`
	DocumentMetadata string     `json:"document_metadata"` // Encrypted doc refs
	RejectionReason  string     `json:"rejection_reason"`
	Attempts         int        `json:"attempts"`
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// KYCLimits stores limit values for a tier
type KYCLimits struct {
	Tier                   KYCTier `json:"tier"`
	DepositLimitDaily      float64 `json:"deposit_limit_daily"`
	DepositLimitMonthly    float64 `json:"deposit_limit_monthly"`
	WithdrawalLimitDaily   float64 `json:"withdrawal_limit_daily"`
	WithdrawalLimitMonthly float64 `json:"withdrawal_limit_monthly"`
	TradingLimitDaily      float64 `json:"trading_limit_daily"`
	APILimitRate           int     `json:"api_limit_rate"`
}

// RestrictionType defines the compliance restriction state
type RestrictionType string

const (
	RestrictionNormal               RestrictionType = "NORMAL"
	RestrictionMonitor              RestrictionType = "MONITOR"
	RestrictionWithdrawalRestricted RestrictionType = "WITHDRAWAL_RESTRICTED"
	RestrictionTradingRestricted    RestrictionType = "TRADING_RESTRICTED"
	RestrictionDepositRestricted    RestrictionType = "DEPOSIT_RESTRICTED"
	RestrictionFullyRestricted      RestrictionType = "FULLY_RESTRICTED"
	RestrictionUnderReview          RestrictionType = "UNDER_REVIEW"
)

// AlertSeverity represents compliance alert priority
type AlertSeverity string

const (
	SeverityLow      AlertSeverity = "LOW"
	SeverityMedium   AlertSeverity = "MEDIUM"
	SeverityHigh     AlertSeverity = "HIGH"
	SeverityCritical AlertSeverity = "CRITICAL"
)

// AlertStatus represents compliance alert state
type AlertStatus string

const (
	AlertOpen        AlertStatus = "OPEN"
	AlertUnderReview AlertStatus = "UNDER_REVIEW"
	AlertEscalated   AlertStatus = "ESCALATED"
	AlertResolved    AlertStatus = "RESOLVED"
	AlertDismissed   AlertStatus = "DISMISSED"
	AlertBlocked     AlertStatus = "BLOCKED"
)

// ComplianceAlert represents aml alert details
type ComplianceAlert struct {
	ID               string        `json:"id"`
	UserID           string        `json:"user_id"`
	TransactionID    string        `json:"transaction_id"`
	RuleTriggered    string        `json:"rule_triggered"`
	RiskScore        float64       `json:"risk_score"`
	Evidence         string        `json:"evidence"`
	Severity         AlertSeverity `json:"severity"`
	Status           AlertStatus   `json:"status"`
	Reviewer         string        `json:"reviewer"`
	ResolutionReason string        `json:"resolution_reason"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// RiskEvaluation stores the deterministic multi-factor score
type RiskEvaluation struct {
	ID                string    `json:"id"`
	UserID            string    `json:"user_id"`
	Score             float64   `json:"score"`
	RiskLevel         string    `json:"risk_level"`
	ContributingRules string    `json:"contributing_rules"` // JSON explanation
	EvaluationVersion string    `json:"evaluation_version"`
	CreatedAt         time.Time `json:"created_at"`
}

// CaseStatus represents compliance case state
type CaseStatus string

const (
	CaseOpen        CaseStatus = "OPEN"
	CaseUnderReview CaseStatus = "UNDER_REVIEW"
	CaseEscalated   CaseStatus = "ESCALATED"
	CaseResolved    CaseStatus = "RESOLVED"
	CaseDismissed   CaseStatus = "DISMISSED"
	CaseBlocked     CaseStatus = "BLOCKED"
)

// ComplianceCase stores investigator records
type ComplianceCase struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	Status         CaseStatus `json:"status"`
	InvestigatorID string     `json:"investigator_id"`
	Resolution     string     `json:"resolution"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ComplianceCaseNote is investigator feedback
type ComplianceCaseNote struct {
	ID        string    `json:"id"`
	CaseID    string    `json:"case_id"`
	AuthorID  string    `json:"author_id"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// SanctionsStatus defines screening response state
type SanctionsStatus string

const (
	SanctionsClear          SanctionsStatus = "CLEAR"
	SanctionsMatch          SanctionsStatus = "MATCH"
	SanctionsPotentialMatch SanctionsStatus = "POTENTIAL_MATCH"
	SanctionsUnavailable    SanctionsStatus = "UNAVAILABLE"
	SanctionsError          SanctionsStatus = "ERROR"
)

// SanctionsProvider screen targets against global lists
type SanctionsProvider interface {
	Screen(ctx context.Context, targetType string, targetValue string) (SanctionsStatus, string, error)
}

// DefaultMockSanctionsProvider for simulation or production fallback
type DefaultMockSanctionsProvider struct {
	Mode string // "simulation" or "production"
}

// Screen screens an address or user
func (p *DefaultMockSanctionsProvider) Screen(ctx context.Context, targetType string, targetValue string) (SanctionsStatus, string, error) {
	if p.Mode == "production" {
		// Simulate network failure or external provider unavailable in production mode to enforce fail closed logic
		return SanctionsUnavailable, "External Screening Provider offline", errors.New("sanctions service timed out")
	}

	// Simulation mode allows rich simulation for development & unit testing
	if targetValue == "0xBLACKLISTED" || targetValue == "OFAC_BLOCKED_ADDR" {
		return SanctionsMatch, "Listed on OFAC sanctions registry", nil
	}
	if targetValue == "0xSUSPICIOUS" {
		return SanctionsPotentialMatch, "Potential match with warning PEP registry", nil
	}
	return SanctionsClear, "No matched sanction records found", nil
}

// ComplianceEngine orchestrates all compliance pipelines
type ComplianceEngine struct {
	db                *database.DB
	producer          *common.KafkaProducer
	sanctionsProvider SanctionsProvider
	log               *logger.Logger
	mode              string // "simulation" or "production"

	// Memory state fallback for local tests without DB or Kafka
	mu           sync.RWMutex
	kycProfiles  map[string]*KYCProfile
	limits       map[KYCTier]*KYCLimits
	alerts       map[string]*ComplianceAlert
	restrictions map[string]RestrictionType
	cases        map[string]*ComplianceCase
	caseNotes    []*ComplianceCaseNote
	riskProfiles map[string]*RiskEvaluation
	travelRules  map[string]map[string]string // txID -> fields
}

// NewComplianceEngine constructs compliance instance
func NewComplianceEngine(db *database.DB, producer *common.KafkaProducer) *ComplianceEngine {
	mode := os.Getenv("COMPLIANCE_MODE")
	if mode == "" {
		mode = "simulation"
	}

	l := logger.NewLogger(logger.Config{
		Level:       "INFO",
		Format:      "JSON",
		ServiceName: "compliance-engine",
	})

	engine := &ComplianceEngine{
		db:                db,
		producer:          producer,
		sanctionsProvider: &DefaultMockSanctionsProvider{Mode: mode},
		log:               l,
		mode:              mode,
		kycProfiles:       make(map[string]*KYCProfile),
		limits:            make(map[KYCTier]*KYCLimits),
		alerts:            make(map[string]*ComplianceAlert),
		restrictions:      make(map[string]RestrictionType),
		cases:             make(map[string]*ComplianceCase),
		caseNotes:         make([]*ComplianceCaseNote, 0),
		riskProfiles:      make(map[string]*RiskEvaluation),
		travelRules:       make(map[string]map[string]string),
	}

	// Initialize default KYC limits in memory fallback
	engine.limits[TierBasic] = &KYCLimits{Tier: TierBasic, DepositLimitDaily: 1000, DepositLimitMonthly: 5000, WithdrawalLimitDaily: 1000, WithdrawalLimitMonthly: 5000, TradingLimitDaily: 5000, APILimitRate: 60}
	engine.limits[TierStandard] = &KYCLimits{Tier: TierStandard, DepositLimitDaily: 10000, DepositLimitMonthly: 50000, WithdrawalLimitDaily: 10000, WithdrawalLimitMonthly: 50000, TradingLimitDaily: 50000, APILimitRate: 120}
	engine.limits[TierAdvanced] = &KYCLimits{Tier: TierAdvanced, DepositLimitDaily: 100000, DepositLimitMonthly: 500000, WithdrawalLimitDaily: 100000, WithdrawalLimitMonthly: 500000, TradingLimitDaily: 500000, APILimitRate: 300}
	engine.limits[TierInstitutional] = &KYCLimits{Tier: TierInstitutional, DepositLimitDaily: 1000000, DepositLimitMonthly: 10000000, WithdrawalLimitDaily: 1000000, WithdrawalLimitMonthly: 10000000, TradingLimitDaily: 10000000, APILimitRate: 600}

	return engine
}

// SetSanctionsProvider registers a custom sanctions provider
func (ce *ComplianceEngine) SetSanctionsProvider(provider SanctionsProvider) {
	ce.sanctionsProvider = provider
}

// PublishEvent publishes structured compliance event
func (ce *ComplianceEngine) PublishEvent(ctx context.Context, eventType string, payload interface{}) {
	if ce.producer != nil {
		event := map[string]interface{}{
			"type":      eventType,
			"payload":   payload,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		// Publish under unified compliance topic
		_ = ce.producer.Publish(ctx, "velyxora-compliance", eventType, event)
	}
}

// SubmitKYC handles standard verification submissions
func (ce *ComplianceEngine) SubmitKYC(ctx context.Context, userID string, tier KYCTier, docMeta string) (*KYCProfile, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	profile, exists := ce.kycProfiles[userID]
	if !exists {
		profile = &KYCProfile{
			UserID:    userID,
			CreatedAt: time.Now(),
		}
	}

	profile.Tier = tier
	profile.Status = StatusInReview
	profile.DocumentMetadata = docMeta
	profile.ProviderRef = fmt.Sprintf("prov_ref_%d", time.Now().UnixNano())
	profile.Attempts++
	profile.UpdatedAt = time.Now()

	if ce.db != nil {
		_, err := ce.db.Pool.Exec(ctx,
			`INSERT INTO kyc_profiles (user_id, tier, status, provider_ref, document_metadata, rejection_reason, attempts, updated_at, created_at)
			 VALUES ($1, $2, $3, $4, $5, '', $6, $7, $8)
			 ON CONFLICT (user_id) DO UPDATE SET tier = $2, status = $3, provider_ref = $4, document_metadata = $5, attempts = $6, updated_at = $7`,
			profile.UserID, string(profile.Tier), string(profile.Status), profile.ProviderRef, profile.DocumentMetadata, profile.Attempts, profile.UpdatedAt, profile.CreatedAt)
		if err != nil {
			ce.log.Error("Failed to persist KYC submit", "err", err)
			return nil, err
		}
	}

	ce.kycProfiles[userID] = profile
	ce.PublishEvent(ctx, "kyc.created", profile)

	return profile, nil
}

// ReviewKYC transitions a profile status
func (ce *ComplianceEngine) ReviewKYC(ctx context.Context, userID string, status KYCStatus, reason string, reviewerID string) (*KYCProfile, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	profile, exists := ce.kycProfiles[userID]
	if !exists {
		// Attempt to load from database if memory empty
		if ce.db != nil {
			var p KYCProfile
			var statusStr, tierStr string
			var vTime, eTime *time.Time
			err := ce.db.Pool.QueryRow(ctx,
				"SELECT user_id, tier, status, provider_ref, document_metadata, rejection_reason, attempts, verified_at, expires_at, created_at, updated_at FROM kyc_profiles WHERE user_id = $1",
				userID).Scan(&p.UserID, &tierStr, &statusStr, &p.ProviderRef, &p.DocumentMetadata, &p.RejectionReason, &p.Attempts, &vTime, &eTime, &p.CreatedAt, &p.UpdatedAt)
			if err == nil {
				p.Tier = KYCTier(tierStr)
				p.Status = KYCStatus(statusStr)
				p.VerifiedAt = vTime
				p.ExpiresAt = eTime
				profile = &p
			}
		}
	}

	if profile == nil {
		return nil, errors.New("KYC profile not found")
	}

	profile.Status = status
	profile.RejectionReason = reason
	profile.UpdatedAt = time.Now()

	now := time.Now()
	if status == StatusVerified {
		profile.VerifiedAt = &now
		exp := now.AddDate(1, 0, 0) // Valid for 1 year
		profile.ExpiresAt = &exp
	}

	if ce.db != nil {
		_, err := ce.db.Pool.Exec(ctx,
			"UPDATE kyc_profiles SET status = $1, rejection_reason = $2, verified_at = $3, expires_at = $4, updated_at = $5 WHERE user_id = $6",
			string(profile.Status), profile.RejectionReason, profile.VerifiedAt, profile.ExpiresAt, profile.UpdatedAt, profile.UserID)
		if err != nil {
			return nil, err
		}

		// Also update the role of the user inside users table for consistency if verified
		if status == StatusVerified {
			newRole := "VERIFIED_USER"
			if profile.Tier == TierInstitutional {
				newRole = "VIP_USER"
			}
			_, _ = ce.db.Pool.Exec(ctx, "UPDATE users SET role = $1, status = 'ACTIVE' WHERE id = $2", newRole, userID)
		}
	}

	ce.kycProfiles[userID] = profile

	// Trigger specific events based on action
	if status == StatusVerified {
		ce.PublishEvent(ctx, "kyc.verified", profile)
	} else if status == StatusRejected {
		ce.PublishEvent(ctx, "kyc.rejected", profile)
	} else {
		ce.PublishEvent(ctx, "kyc.updated", profile)
	}

	return profile, nil
}

// Lockless internal helper to retrieve KYC profile
func (ce *ComplianceEngine) getKYC(ctx context.Context, userID string) (*KYCProfile, error) {
	profile, exists := ce.kycProfiles[userID]
	if exists {
		return profile, nil
	}

	if ce.db != nil {
		var p KYCProfile
		var statusStr, tierStr string
		var vTime, eTime *time.Time
		err := ce.db.Pool.QueryRow(ctx,
			"SELECT user_id, tier, status, provider_ref, document_metadata, rejection_reason, attempts, verified_at, expires_at, created_at, updated_at FROM kyc_profiles WHERE user_id = $1",
			userID).Scan(&p.UserID, &tierStr, &statusStr, &p.ProviderRef, &p.DocumentMetadata, &p.RejectionReason, &p.Attempts, &vTime, &eTime, &p.CreatedAt, &p.UpdatedAt)
		if err == nil {
			p.Tier = KYCTier(tierStr)
			p.Status = KYCStatus(statusStr)
			p.VerifiedAt = vTime
			p.ExpiresAt = eTime
			return &p, nil
		}
	}

	// Default unverified fallback
	return &KYCProfile{
		UserID:           userID,
		Tier:             TierBasic,
		Status:           StatusPending,
		DocumentMetadata: "{}",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

// GetKYC retrieves user status (locking wrapper)
func (ce *ComplianceEngine) GetKYC(ctx context.Context, userID string) (*KYCProfile, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.getKYC(ctx, userID)
}

// GetLimits retrieves the limits configuration for a tier
func (ce *ComplianceEngine) GetLimits(ctx context.Context, tier KYCTier) (*KYCLimits, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	limit, exists := ce.limits[tier]
	if exists {
		return limit, nil
	}

	if ce.db != nil {
		var lim KYCLimits
		var tierStr string
		err := ce.db.Pool.QueryRow(ctx,
			"SELECT tier, deposit_limit_daily, deposit_limit_monthly, withdrawal_limit_daily, withdrawal_limit_monthly, trading_limit_daily, api_limit_rate FROM kyc_limits WHERE tier = $1",
			string(tier)).Scan(&tierStr, &lim.DepositLimitDaily, &lim.DepositLimitMonthly, &lim.WithdrawalLimitDaily, &lim.WithdrawalLimitMonthly, &lim.TradingLimitDaily, &lim.APILimitRate)
		if err == nil {
			lim.Tier = KYCTier(tierStr)
			return &lim, nil
		}
	}

	return nil, fmt.Errorf("limits not configured for tier %s", tier)
}

// RestrictAccount applies compliance-driven restrictions
func (ce *ComplianceEngine) RestrictAccount(ctx context.Context, userID string, restriction RestrictionType, reason string, actorID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	ce.restrictions[userID] = restriction

	if ce.db != nil {
		_, err := ce.db.Pool.Exec(ctx,
			`INSERT INTO account_restrictions (user_id, restriction_type, reason, created_by, updated_at, created_at)
			 VALUES ($1, $2, $3, $4, NOW(), NOW())
			 ON CONFLICT (user_id) DO UPDATE SET restriction_type = $2, reason = $3, created_by = $4, updated_at = NOW()`,
			userID, string(restriction), reason, actorID)
		if err != nil {
			return err
		}

		// Sync with users status if fully restricted or suspended
		if restriction == RestrictionFullyRestricted {
			_, _ = ce.db.Pool.Exec(ctx, "UPDATE users SET status = 'SUSPENDED' WHERE id = $1", userID)
		} else if restriction == RestrictionNormal {
			_, _ = ce.db.Pool.Exec(ctx, "UPDATE users SET status = 'ACTIVE' WHERE id = $1", userID)
		} else {
			_, _ = ce.db.Pool.Exec(ctx, "UPDATE users SET status = 'RESTRICTED' WHERE id = $1", userID)
		}
	}

	ce.PublishEvent(ctx, "compliance.restriction.created", map[string]interface{}{
		"user_id":          userID,
		"restriction_type": restriction,
		"reason":           reason,
		"actor_id":         actorID,
	})

	return nil
}

// Lockless internal helper to retrieve restriction status
func (ce *ComplianceEngine) getRestriction(ctx context.Context, userID string) (RestrictionType, error) {
	if rest, exists := ce.restrictions[userID]; exists {
		return rest, nil
	}

	if ce.db != nil {
		var restType string
		err := ce.db.Pool.QueryRow(ctx, "SELECT restriction_type FROM account_restrictions WHERE user_id = $1", userID).Scan(&restType)
		if err == nil {
			return RestrictionType(restType), nil
		}
	}

	return RestrictionNormal, nil
}

// GetRestriction gets the current account restriction (locking wrapper)
func (ce *ComplianceEngine) GetRestriction(ctx context.Context, userID string) (RestrictionType, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.getRestriction(ctx, userID)
}

// CreateAlert stores a suspicious alert and publishes aml.alert.created
func (ce *ComplianceEngine) CreateAlert(ctx context.Context, userID string, txID string, ruleName string, risk float64, evidence string, severity AlertSeverity) (*ComplianceAlert, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	alert := &ComplianceAlert{
		ID:            fmt.Sprintf("alt_%d", time.Now().UnixNano()),
		UserID:        userID,
		TransactionID: txID,
		RuleTriggered: ruleName,
		RiskScore:     risk,
		Evidence:      evidence,
		Severity:      severity,
		Status:        AlertOpen,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if ce.db != nil {
		_, err := ce.db.Pool.Exec(ctx,
			`INSERT INTO compliance_alerts (id, user_id, transaction_id, rule_triggered, risk_score, evidence, severity, status, reviewer, resolution_reason, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '', '', $9, $10)`,
			alert.ID, alert.UserID, alert.TransactionID, alert.RuleTriggered, alert.RiskScore, alert.Evidence, string(alert.Severity), string(alert.Status), alert.CreatedAt, alert.UpdatedAt)
		if err != nil {
			return nil, err
		}
	}

	ce.alerts[alert.ID] = alert
	ce.PublishEvent(ctx, "aml.alert.created", alert)

	return alert, nil
}

// UpdateAlert updates alert review status
func (ce *ComplianceEngine) UpdateAlert(ctx context.Context, alertID string, status AlertStatus, reviewer string, reason string) (*ComplianceAlert, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	alert, exists := ce.alerts[alertID]
	if !exists {
		if ce.db != nil {
			var a ComplianceAlert
			var statusStr, severityStr string
			err := ce.db.Pool.QueryRow(ctx,
				"SELECT id, user_id, transaction_id, rule_triggered, risk_score, evidence, severity, status, reviewer, resolution_reason, created_at, updated_at FROM compliance_alerts WHERE id = $1",
				alertID).Scan(&a.ID, &a.UserID, &a.TransactionID, &a.RuleTriggered, &a.RiskScore, &a.Evidence, &severityStr, &statusStr, &a.Reviewer, &a.ResolutionReason, &a.CreatedAt, &a.UpdatedAt)
			if err == nil {
				a.Severity = AlertSeverity(severityStr)
				a.Status = AlertStatus(statusStr)
				alert = &a
			}
		}
	}

	if alert == nil {
		return nil, errors.New("alert not found")
	}

	alert.Status = status
	alert.Reviewer = reviewer
	alert.ResolutionReason = reason
	alert.UpdatedAt = time.Now()

	if ce.db != nil {
		_, err := ce.db.Pool.Exec(ctx,
			"UPDATE compliance_alerts SET status = $1, reviewer = $2, resolution_reason = $3, updated_at = $4 WHERE id = $5",
			string(alert.Status), alert.Reviewer, alert.ResolutionReason, alert.UpdatedAt, alert.ID)
		if err != nil {
			return nil, err
		}
	}

	ce.alerts[alert.ID] = alert
	ce.PublishEvent(ctx, "aml.alert.updated", alert)

	return alert, nil
}

// EvaluateAML runs transactional monitoring metrics
func (ce *ComplianceEngine) EvaluateAML(ctx context.Context, userID string, amount float64, asset string, txType string, txID string) ([]*ComplianceAlert, error) {
	var alerts []*ComplianceAlert

	// 1. Check for unusually large transactions (> 5000 USD / units)
	if amount >= 5000.0 {
		alt, err := ce.CreateAlert(ctx, userID, txID, "aml_rule_unusual_amount", 0.65, fmt.Sprintf("Transaction of %f %s exceeds threshold limits", amount, asset), SeverityHigh)
		if err == nil {
			alerts = append(alerts, alt)
		}
	}

	// 2. Velocity evaluation (check transaction count in database)
	if ce.db != nil {
		var txCount int
		// Let's count recent withdrawals/deposits of this user in the past 1 hour
		_ = ce.db.Pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM (
				SELECT id FROM withdrawals WHERE user_id = $1 AND created_at >= NOW() - INTERVAL '1 hour'
				UNION ALL
				SELECT id FROM deposits WHERE user_id = $1 AND created_at >= NOW() - INTERVAL '1 hour'
			) as combined_txs`, userID).Scan(&txCount)

		if txCount >= 5 {
			alt, err := ce.CreateAlert(ctx, userID, txID, "aml_rule_velocity_tx", 0.75, fmt.Sprintf("User triggered %d transactions in the past hour", txCount), SeverityCritical)
			if err == nil {
				alerts = append(alerts, alt)
				// Apply auto monitor restriction
				_ = ce.RestrictAccount(ctx, userID, RestrictionMonitor, "Automated high transaction velocity triggered", "SYSTEM")
			}
		}
	}

	return alerts, nil
}

// EvaluateRiskScore executes deterministic multi-factor risk rules
func (ce *ComplianceEngine) EvaluateRiskScore(ctx context.Context, userID string) (*RiskEvaluation, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	factors := make(map[string]interface{})
	var score float64 = 0.1 // Baseline risk score

	// KYC status evaluation (lockless internal call)
	kyc, err := ce.getKYC(ctx, userID)
	if err == nil {
		if kyc.Status == StatusVerified {
			factors["kyc_status"] = "Verified"
			score -= 0.05
		} else if kyc.Status == StatusPending {
			factors["kyc_status"] = "Pending"
			score += 0.1
		} else {
			factors["kyc_status"] = string(kyc.Status)
			score += 0.25
		}
	}

	// Account restrictions evaluation (lockless internal call)
	rest, _ := ce.getRestriction(ctx, userID)
	if rest != RestrictionNormal {
		factors["restriction"] = string(rest)
		score += 0.3
	}

	// Recent compliance alerts count
	if ce.db != nil {
		var alertCount int
		_ = ce.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM compliance_alerts WHERE user_id = $1 AND status != 'RESOLVED'", userID).Scan(&alertCount)
		if alertCount > 0 {
			factors["active_alerts"] = alertCount
			score += float64(alertCount) * 0.15
		}
	}

	// Keep score bounded within [0.0, 1.0]
	if score < 0.0 {
		score = 0.0
	} else if score > 1.0 {
		score = 1.0
	}

	riskLevel := "LOW"
	if score > 0.75 {
		riskLevel = "CRITICAL"
	} else if score > 0.5 {
		riskLevel = "HIGH"
	} else if score > 0.25 {
		riskLevel = "MEDIUM"
	}

	factors["evaluation_date"] = time.Now().Format(time.RFC3339)
	factors["evaluated_by"] = "Deterministic Velyxora Algorithmic Rule Engine"
	factorsJSON, _ := json.Marshal(factors)

	eval := &RiskEvaluation{
		ID:                fmt.Sprintf("risk_%d", time.Now().UnixNano()),
		UserID:            userID,
		Score:             score,
		RiskLevel:         riskLevel,
		ContributingRules: string(factorsJSON),
		EvaluationVersion: "v1.1",
		CreatedAt:         time.Now(),
	}

	if ce.db != nil {
		_, err := ce.db.Pool.Exec(ctx,
			"INSERT INTO risk_evaluations (id, user_id, score, risk_level, contributing_rules, evaluation_version, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
			eval.ID, eval.UserID, eval.Score, eval.RiskLevel, eval.ContributingRules, eval.EvaluationVersion, eval.CreatedAt)
		if err != nil {
			return nil, err
		}
	}

	ce.riskProfiles[userID] = eval
	ce.PublishEvent(ctx, "risk.evaluated", eval)

	return eval, nil
}

// ScreenSanctions screens address/user against watchlists. Failure closed!
func (ce *ComplianceEngine) ScreenSanctions(ctx context.Context, targetType string, targetValue string) (SanctionsStatus, string, error) {
	ce.PublishEvent(ctx, "sanctions.screening.requested", map[string]string{
		"target_type":  targetType,
		"target_value": targetValue,
	})

	status, details, err := ce.sanctionsProvider.Screen(ctx, targetType, targetValue)
	if err != nil {
		// Failure closed: Error states or Unavailable states must NOT become CLEAR. Fail safely!
		ce.log.Error("Sanctions Screening Provider Failed, holding operation as closed", "target", targetValue, "err", err)
		return SanctionsError, "Sanctions screening provider failure. Closed/Held by security policy.", err
	}

	if ce.db != nil {
		_, _ = ce.db.Pool.Exec(ctx,
			"INSERT INTO sanctions_screenings (id, target_type, target_value, provider, status, details, created_at) VALUES ($1, $2, $3, 'DefaultSanctionsScreener', $4, $5, NOW())",
			fmt.Sprintf("scr_%d", time.Now().UnixNano()), targetType, targetValue, string(status), details)
	}

	ce.PublishEvent(ctx, "sanctions.screening.completed", map[string]string{
		"target_type":  targetType,
		"target_value": targetValue,
		"status":       string(status),
		"details":      details,
	})

	return status, details, nil
}

// CheckLimits validates that user conforms to limits
func (ce *ComplianceEngine) CheckLimits(ctx context.Context, userID string, amount float64, txType string) error {
	profile, _ := ce.GetKYC(ctx, userID)
	tier := profile.Tier

	limit, exists := ce.limits[tier]
	if !exists {
		return fmt.Errorf("limits config not found for tier %s", tier)
	}

	var totalRecent float64 = 0.0

	if ce.db != nil {
		if txType == "WITHDRAWAL" {
			_ = ce.db.Pool.QueryRow(ctx,
				"SELECT COALESCE(SUM(amount), 0.0) FROM withdrawals WHERE user_id = $1 AND status != 'REJECTED' AND status != 'FAILED' AND created_at >= NOW() - INTERVAL '1 day'",
				userID).Scan(&totalRecent)

			if totalRecent+amount > limit.WithdrawalLimitDaily {
				return fmt.Errorf("withdrawal request of %f exceeds daily limit of %f (already withdrew %f today)", amount, limit.WithdrawalLimitDaily, totalRecent)
			}
		} else if txType == "DEPOSIT" {
			_ = ce.db.Pool.QueryRow(ctx,
				"SELECT COALESCE(SUM(amount), 0.0) FROM deposits WHERE user_id = $1 AND status != 'REJECTED' AND created_at >= NOW() - INTERVAL '1 day'",
				userID).Scan(&totalRecent)

			if totalRecent+amount > limit.DepositLimitDaily {
				return fmt.Errorf("deposit of %f exceeds daily limit of %f (already deposited %f today)", amount, limit.DepositLimitDaily, totalRecent)
			}
		}
	}

	return nil
}

// RegisterTravelRule checks and records Travel Rule originator/beneficiary info
func (ce *ComplianceEngine) RegisterTravelRule(ctx context.Context, txID string, origName, origAddr, origAcc, benefName, benefAddr, benefAcc string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	rec := map[string]string{
		"transaction_id":      txID,
		"originator_name":     origName,
		"originator_address":  origAddr,
		"originator_account":  origAcc,
		"beneficiary_name":    benefName,
		"beneficiary_address": benefAddr,
		"beneficiary_account": benefAcc,
		"provider":            "VelyxoraTravelGateway",
	}

	status := "COMPLIANT"
	if origName == "" || benefName == "" {
		status = "MISSING_INFO"
	}

	rec["status"] = status

	if ce.db != nil {
		_, err := ce.db.Pool.Exec(ctx,
			`INSERT INTO travel_rule_records (id, transaction_id, originator_name, originator_address, originator_account, beneficiary_name, beneficiary_address, beneficiary_account, provider, status, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())`,
			fmt.Sprintf("tr_%d", time.Now().UnixNano()), txID, origName, origAddr, origAcc, benefName, benefAddr, benefAcc, "VelyxoraTravelGateway", status)
		if err != nil {
			return err
		}
	}

	ce.travelRules[txID] = rec

	if status == "MISSING_INFO" {
		return fmt.Errorf("Travel Rule information missing or incomplete. Originator/Beneficiary names are mandatory.")
	}

	return nil
}

// CheckWithdrawalAllowed consolidates all gates. High security!
func (ce *ComplianceEngine) CheckWithdrawalAllowed(ctx context.Context, userID string, amount float64, asset string, address string) error {
	// 1. Account restriction checks
	rest, err := ce.GetRestriction(ctx, userID)
	if err == nil {
		if rest == RestrictionFullyRestricted || rest == RestrictionWithdrawalRestricted || rest == RestrictionUnderReview {
			return fmt.Errorf("withdrawal is blocked: account restricted status %s", rest)
		}
	}

	// 2. KYC verification status gates
	kyc, err := ce.GetKYC(ctx, userID)
	if err == nil {
		if kyc.Status == StatusSuspended || kyc.Status == StatusRejected || kyc.Status == StatusExpired {
			return fmt.Errorf("withdrawal is blocked: KYC status is %s", kyc.Status)
		}
	}

	// 3. Limits validation
	err = ce.CheckLimits(ctx, userID, amount, "WITHDRAWAL")
	if err != nil {
		return err
	}

	// 4. Sanctions / Watchlist Screening (Failure Closed!)
	sStatus, msg, err := ce.ScreenSanctions(ctx, "ADDRESS_WITHDRAWAL", address)
	if err != nil {
		return fmt.Errorf("sanctions verification unavailable (Closed/Held): %w", err)
	}
	if sStatus == SanctionsMatch || sStatus == SanctionsPotentialMatch {
		return fmt.Errorf("withdrawal is blocked: address %s matched sanctions watchlist rule (%s)", address, msg)
	}
	if sStatus == SanctionsUnavailable || sStatus == SanctionsError {
		return fmt.Errorf("withdrawal is blocked: sanctions checker unavailable")
	}

	return nil
}

// CreateCase instantiates a manual review workflow case
func (ce *ComplianceEngine) CreateCase(ctx context.Context, userID string, alertIDs []string) (*ComplianceCase, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	c := &ComplianceCase{
		ID:        fmt.Sprintf("case_%d", time.Now().UnixNano()),
		UserID:    userID,
		Status:    CaseOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if ce.db != nil {
		tx, err := ce.db.Pool.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx,
			"INSERT INTO compliance_cases (id, user_id, status, investigator_id, resolution, created_at, updated_at) VALUES ($1, $2, $3, '', '', $4, $5)",
			c.ID, c.UserID, string(c.Status), c.CreatedAt, c.UpdatedAt)
		if err != nil {
			return nil, err
		}

		for _, alertID := range alertIDs {
			_, err = tx.Exec(ctx, "INSERT INTO compliance_case_alerts (case_id, alert_id) VALUES ($1, $2)", c.ID, alertID)
			if err != nil {
				return nil, err
			}
			// Update alert status to under review
			_, _ = tx.Exec(ctx, "UPDATE compliance_alerts SET status = 'UNDER_REVIEW' WHERE id = $1", alertID)
		}

		err = tx.Commit(ctx)
		if err != nil {
			return nil, err
		}
	}

	ce.cases[c.ID] = c
	ce.PublishEvent(ctx, "case.created", c)

	return c, nil
}

// AddCaseNote inserts dynamic investigator evidence
func (ce *ComplianceEngine) AddCaseNote(ctx context.Context, caseID string, authorID string, note string) (*ComplianceCaseNote, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	n := &ComplianceCaseNote{
		ID:        fmt.Sprintf("note_%d", time.Now().UnixNano()),
		CaseID:    caseID,
		AuthorID:  authorID,
		Note:      note,
		CreatedAt: time.Now(),
	}

	if ce.db != nil {
		_, err := ce.db.Pool.Exec(ctx,
			"INSERT INTO compliance_case_notes (id, case_id, author_id, note, created_at) VALUES ($1, $2, $3, $4, $5)",
			n.ID, n.CaseID, n.AuthorID, n.Note, n.CreatedAt)
		if err != nil {
			return nil, err
		}
	}

	ce.caseNotes = append(ce.caseNotes, n)
	return n, nil
}

// ResolveCase updates state and resolution notes
func (ce *ComplianceEngine) ResolveCase(ctx context.Context, caseID string, status CaseStatus, resolution string, investigatorID string) (*ComplianceCase, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	c, exists := ce.cases[caseID]
	if !exists {
		if ce.db != nil {
			var d ComplianceCase
			var statusStr string
			err := ce.db.Pool.QueryRow(ctx, "SELECT id, user_id, status, investigator_id, resolution, created_at, updated_at FROM compliance_cases WHERE id = $1", caseID).Scan(&d.ID, &d.UserID, &statusStr, &d.InvestigatorID, &d.Resolution, &d.CreatedAt, &d.UpdatedAt)
			if err == nil {
				d.Status = CaseStatus(statusStr)
				c = &d
			}
		}
	}

	if c == nil {
		return nil, errors.New("case not found")
	}

	c.Status = status
	c.Resolution = resolution
	c.InvestigatorID = investigatorID
	c.UpdatedAt = time.Now()

	if ce.db != nil {
		tx, err := ce.db.Pool.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx,
			"UPDATE compliance_cases SET status = $1, resolution = $2, investigator_id = $3, updated_at = $4 WHERE id = $5",
			string(c.Status), c.Resolution, c.InvestigatorID, c.UpdatedAt, c.ID)
		if err != nil {
			return nil, err
		}

		// Update all associated alerts status based on case resolution
		alertStatus := AlertResolved
		if status == CaseDismissed {
			alertStatus = AlertDismissed
		} else if status == CaseBlocked {
			alertStatus = AlertBlocked
		}

		_, _ = tx.Exec(ctx,
			`UPDATE compliance_alerts SET status = $1, reviewer = $2, resolution_reason = $3, updated_at = NOW()
			 WHERE id IN (SELECT alert_id FROM compliance_case_alerts WHERE case_id = $4)`,
			string(alertStatus), investigatorID, resolution, c.ID)

		err = tx.Commit(ctx)
		if err != nil {
			return nil, err
		}
	}

	ce.cases[c.ID] = c
	ce.PublishEvent(ctx, "case.resolved", c)

	return c, nil
}
