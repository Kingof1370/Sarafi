# Velyxora Platform Disaster Recovery Specification

This document details the disaster recovery architecture and operational states.

## 1. Disaster Recovery Operational Modes

The system operates in one of the following state modes:
- **NORMAL**: System runs normally with all transactional pathways fully functional.
- **DEGRADED**: Minor component failure or high latency. Some non-critical background jobs are suspended.
- **READ_ONLY**: Trading engine, withdrawals, and balance updates are suspended. Administrative query paths are open.
- **EMERGENCY**: System is in global emergency freeze. Critical paths are completely closed to prevent loss of funds.
- **RECOVERY**: Controlled restores and reconciliations are running.

## 2. Restore Workflow

- **Integrity Validation**: The restore process validates checksums, decrypts archives, verifies schema version and database migration compatibility before modifying any databases.
- **Destructive Restore Policy**: Destructive restores are **never** executed automatically. They require manual authorized administrator authorization and MFA TOTP verification.
- **Post-Restore Health Check**: Upon restore completion, the engine executes component health probes to verify database, cache, and messaging layers are operational.

## 3. Administration REST API

- `GET /api/v1/system/recovery/status` - Fetches active recovery mode and RPO/RTO metrics.
- `POST /api/v1/system/recovery/validate` - Validates integrity and executes authorized restores if `execute: true` is passed.
- All endpoints require fine-grained RBAC `system:admin` permissions and an active MFA token.
