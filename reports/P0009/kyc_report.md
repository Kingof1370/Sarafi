# P0009 KYC & Customer Verification Report

## Verification Lifecycle
- **States Covered**: `PENDING`, `IN_REVIEW`, `VERIFIED`, `REJECTED`, `EXPIRED`, `SUSPENDED`, `REQUIRES_REVIEW`
- **Verification Limits Enforced**:
  - `BASIC`: Daily limit $1,000.
  - `STANDARD`: Daily limit $10,000.
  - `ADVANCED`: Daily limit $100,000.
  - `INSTITUTIONAL`: Daily limit $1,000,000.

Successful verification automatically updates the user status/role inside database records.
- Unit testing confirms correct state progression, attempts incrementing, and expiry checks.
