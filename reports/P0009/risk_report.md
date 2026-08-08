# P0009 Risk Assessment & Scoring Report

## Multi-Factor Scoring Framework
The Risk Assessment Engine runs a deterministic algorithm calculating user risk profile scores between `0.0` (Minimal Risk) and `1.0` (Critical Risk).

### Contributing Factors
- Baseline score: `0.10`
- KYC status matching
- Active compliance restrictions
- Quantity and severity of active compliance alerts

The score is fully explained through structured JSON logs and persists securely inside PostgreSQL.
- Verified in unit tests with no concurrent deadlocks or delays.
