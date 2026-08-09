# Operations Runbook
## Velyxora Enterprise Exchange

This document details day-to-day operations and procedures for Velyxora.

### 1. Database Operations
- Backups are generated daily via transaction logs (WAL Stream) with AES-256-GCM encryption.
- Verification restores are executed dynamically within temporary isolated sandbox environments to confirm ledger state balances (`debits_sum == credits_sum`).

### 2. Redis Cache Reconstruction
- In case of a Redis failover or eviction event, the cache reconstruction can be triggered.
- Cache state (sessions, active API keys, rate limits) repopulates idempotently from the authoritative PostgreSQL database.
- Request path: `POST /api/v1/system/recover` (requires admin scope and TOTP validation).

### 3. Escalating SRE Incidents
- Any infrastructure unreachable event (PostgreSQL down, Redis offline, Kafka disconnected) creates an automatic critical SRE Incident in the database and triggers real-time alerts.
- Mitigated services automatically resolve active incidents upon successful ping.
