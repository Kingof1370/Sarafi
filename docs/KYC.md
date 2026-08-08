# KYC & Customer Verification Policy

Velyxora enforces a strict, multi-tiered Know-Your-Customer (KYC) compliance framework aligned with global regulatory directives (e.g., AMLD5/AMLD6, FATF Recommendations).

## KYC States & Lifecycle
The customer verification workflow implements a clear, state-based state machine:
- **PENDING**: Onboarded state prior to uploading documents.
- **IN_REVIEW**: Documents submitted, awaiting automated provider screening or manual review.
- **VERIFIED**: Customer details matching and approved. Triggers tier limit updates and user role progression.
- **REJECTED**: Documents disqualified. Restores status and logs rejection reasons.
- **EXPIRED**: Profiles expire after 1 year. Re-evaluation is forced automatically.
- **SUSPENDED**: Account flagged for dynamic compliance fraud alerts.
- **REQUIRES_REVIEW**: Temporary state for manual due diligence audits.

## Configurable Tiers & Limits
Velyxora supports 4 configurable verification tiers:
1. **BASIC**: Minimal onboarding. Withdrawal limits are highly capped ($1,000 daily).
2. **STANDARD**: Authenticated document verification. High daily transaction thresholds ($10,000).
3. **ADVANCED**: Extended residential proof checks. Enhanced limits ($100,000).
4. **INSTITUTIONAL**: Full corporate 4-eyes compliance validation. Professional limits ($1,000,000+).

Each tier strictly checks deposit, withdrawal, daily, and trading limits enforced inside real backend gateway transaction paths.
