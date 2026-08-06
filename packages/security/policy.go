package security

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Role represents standard enterprise access levels
type Role string

const (
	RoleAdmin             Role = "ADMIN"
	RoleOperator          Role = "OPERATOR"
	RoleComplianceAuditor Role = "COMPLIANCE_AUDITOR"
	RoleUser              Role = "USER"
	RoleGuest             Role = "GUEST"
)

// Resource defines tracked exchange entities
type Resource string

const (
	ResourceWallet   Resource = "WALLET"
	ResourceTreasury Resource = "TREASURY"
	ResourceKeys     Resource = "KEYS"
	ResourceUser     Resource = "USER"
	ResourceOrder    Resource = "ORDER"
	ResourceAudit    Resource = "AUDIT"
)

// Action represents operations attempted on resources
type Action string

const (
	ActionRead   Action = "READ"
	ActionWrite  Action = "WRITE"
	ActionDelete Action = "DELETE"
	ActionSign   Action = "SIGN"
	ActionRotate Action = "ROTATE"
)

// ContextAttributes holds context-specific parameters for ABAC decisions
type ContextAttributes struct {
	ClientIP          string    `json:"client_ip"`
	GeoLocation       string    `json:"geo_location"`
	DeviceFingerprint string    `json:"device_fingerprint"`
	RequestTime       time.Time `json:"request_time"`
	RiskScore         float64   `json:"risk_score"`
	SessionAgeSeconds int       `json:"session_age_seconds"`
}

// PolicyRule defines an access permission rule
type PolicyRule struct {
	ID        string                         `json:"id"`
	Role      Role                           `json:"role"`
	Resource  Resource                       `json:"resource"`
	Action    Action                         `json:"action"`
	Condition func(*ContextAttributes) bool  `json:"-"`
}

// PolicyEngine evaluates thread-safe RBAC and ABAC decisions
type PolicyEngine struct {
	mu            sync.RWMutex
	rules         map[string]*PolicyRule
	blacklistedIP map[string]bool
}

// NewPolicyEngine initializes the core Zero-Trust Policy Engine
func NewPolicyEngine() *PolicyEngine {
	pe := &PolicyEngine{
		rules:         make(map[string]*PolicyRule),
		blacklistedIP: make(map[string]bool),
	}
	pe.bootstrapDefaultRules()
	return pe
}

// bootstrapDefaultRules configures default secure access policies
func (pe *PolicyEngine) bootstrapDefaultRules() {
	// 1. Admin can perform any action on any resource, provided risk is low
	pe.AddRule(&PolicyRule{
		ID:       "rule_admin_all",
		Role:     RoleAdmin,
		Resource: "*",
		Action:   "*",
		Condition: func(ctx *ContextAttributes) bool {
			return ctx.RiskScore < 0.8 && !pe.IsIPBlacklisted(ctx.ClientIP)
		},
	})

	// 2. Compliance Auditors can only READ audits and wallets
	pe.AddRule(&PolicyRule{
		ID:       "rule_compliance_audit",
		Role:     RoleComplianceAuditor,
		Resource: ResourceAudit,
		Action:   ActionRead,
		Condition: func(ctx *ContextAttributes) bool {
			return !pe.IsIPBlacklisted(ctx.ClientIP)
		},
	})
	pe.AddRule(&PolicyRule{
		ID:       "rule_compliance_wallet",
		Role:     RoleComplianceAuditor,
		Resource: ResourceWallet,
		Action:   ActionRead,
		Condition: func(ctx *ContextAttributes) bool {
			return !pe.IsIPBlacklisted(ctx.ClientIP)
		},
	})

	// 3. Standard user can read/write their own wallet/orders
	pe.AddRule(&PolicyRule{
		ID:       "rule_user_wallet",
		Role:     RoleUser,
		Resource: ResourceWallet,
		Action:   ActionRead,
		Condition: func(ctx *ContextAttributes) bool {
			return ctx.RiskScore < 0.6 && !pe.IsIPBlacklisted(ctx.ClientIP)
		},
	})
	pe.AddRule(&PolicyRule{
		ID:       "rule_user_order",
		Role:     RoleUser,
		Resource: ResourceOrder,
		Action:   ActionWrite,
		Condition: func(ctx *ContextAttributes) bool {
			// Users cannot submit orders if risk score is elevated without step-up validation
			return ctx.RiskScore < 0.5 && !pe.IsIPBlacklisted(ctx.ClientIP)
		},
	})
}

// AddRule registers a new dynamic rule
func (pe *PolicyEngine) AddRule(rule *PolicyRule) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.rules[rule.ID] = rule
}

// BlacklistIP blocks all requests from a network address pool
func (pe *PolicyEngine) BlacklistIP(ip string) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.blacklistedIP[ip] = true
}

// RemoveBlacklistIP unblocks an IP address
func (pe *PolicyEngine) RemoveBlacklistIP(ip string) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	delete(pe.blacklistedIP, ip)
}

// IsIPBlacklisted checks if an IP is blocked
func (pe *PolicyEngine) IsIPBlacklisted(ip string) bool {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return pe.blacklistedIP[ip]
}

// Evaluate determines whether a subject with a role has authorization
func (pe *PolicyEngine) Evaluate(role Role, resource Resource, action Action, ctx *ContextAttributes) (bool, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	if ctx == nil {
		return false, errors.New("missing context attributes")
	}

	// Dynamic IP Blacklist check (Core Zero-Trust requirement)
	if pe.blacklistedIP[ctx.ClientIP] {
		return false, fmt.Errorf("request rejected: client IP %s is permanently blacklisted", ctx.ClientIP)
	}

	// Extreme session age check (prevents stale token reuse)
	if ctx.SessionAgeSeconds > 3600 {
		return false, errors.New("request rejected: session age exceeds maximum allowable lifespan (1 hour)")
	}

	authorized := false
	for _, rule := range pe.rules {
		// Evaluate Role match (RBAC)
		roleMatch := rule.Role == role || rule.Role == "*"
		if !roleMatch {
			continue
		}

		// Evaluate Resource match
		resMatch := rule.Resource == resource || rule.Resource == "*"
		if !resMatch {
			continue
		}

		// Evaluate Action match
		actMatch := rule.Action == action || rule.Action == "*"
		if !actMatch {
			continue
		}

		// Evaluate dynamic environment conditions (ABAC)
		if rule.Condition != nil {
			if rule.Condition(ctx) {
				authorized = true
				break
			}
		} else {
			authorized = true
			break
		}
	}

	return authorized, nil
}
