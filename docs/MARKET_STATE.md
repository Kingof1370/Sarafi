# VELYXORA EXCHANGE - MARKET STATES & CONTROLS
Governs global and market-specific operational status.

## States
- **ACTIVE:** All trading and order operations are fully functional.
- **HALTED:** Globally suspends trading activities. Rejects new order submissions.
- **SUSPENDED:** Specific trading symbols are deactivated and reject operations.
- **MAINTENANCE:** Trading is halted for platform upgrade procedures.

## Hardened Integrations
Administrative status updates are fully connected to the `RiskEngine` inside `OMSRouter` and validated through standard JWT, RBAC permissions, and MFA code challenges.
