# Incident Operations Runbook
## Velyxora Enterprise Exchange

Procedures for managing SRE security and reliability incidents.

### 1. Security Alert Lifecycle
- Automated AML or compliance watchlist triggers raise alerts in `compliance_alerts`.
- Critical sanctions alerts trigger a temporary lock on the target account's capabilities (`RestrictionFullyRestricted`), blocking deposits and withdrawals.

### 2. Service Down mitigations
- If the SRE alerting loop detects database, redis, or kafka network timeouts, it automatically logs a critical event in `sre_incidents` and updates the SRE metric dashboards.
- Upon service recovery, the monitoring thread automatically updates the incident to `RESOLVED` status.
