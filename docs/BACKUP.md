# Velyxora Platform Database Backup & Lifecycle Guidelines

The Velyxora enterprise exchange platform implements standard cryptographic backups and lifecycle management routines.

## 1. Backup Strategy Overview

- **Storage Format**: Symmetric encryption via **AES-256-GCM** using a master key stored in safe hardware-backed HSM or secure environment configurations.
- **Consistent Dump**: Exports core business states (`users`, `balances`, `ledger_entries`, `orders`, `idempotency_records`) atomically.
- **LSN WAL Tracking**: Every backup captures the authoritative Postgres Transaction LSN / WAL position to support downstream Point-In-Time-Recovery (PITR).
- **Integrity Validation**: Computes a deterministic **SHA-256 Checksum** on the final encrypted file which is persisted directly into the database registry `dr_backups`.

## 2. Retention Policy & Lifecycle Management

- **Retention Window**: 7 days maximum retention before automatic secure file deletions.
- **Pruning Trigger**: Every new backup execution spawns an asynchronous cleanup thread (`cleanExpiredBackups`) that purges metadata and purges files beyond the 7-day threshold.

## 3. Operations & Recovery

To schedule or perform manual backups:
- Use the HTTP REST endpoint: `POST /api/v1/system/backups`
- Form Parameters: `{"backup_type": "FULL"}`
- Validation: Requires active fine-grained **RBAC permissions** (`system:admin`) and **MFA challenge verification** (`X-MFA-Code` header).
