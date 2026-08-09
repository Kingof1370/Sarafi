# VELYXORA EXCHANGE - DISASTER RECOVERY & BACKUP RUNBOOK

This runbook describes the exact shell commands, API endpoints, and configuration parameters used for managing backups, isolated restore tests, and recovering state after a system failure.

## 1. INITIATING AN ENCRYPTED BACKUP MANUALLY
Administrators with `system:admin` permissions can trigger a secure database backup.

```bash
# REST API trigger using curl (requires authorization and MFA headers)
curl -X POST http://localhost:8080/api/v1/oms/system/backup \
  -H "Authorization: Bearer <ADMIN_JWT_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"type": "DATABASE", "mfa_code": "123456"}'
```

The server response will contain the secure filepath where the encrypted archive is stored on disk (e.g. `/tmp/velyxora_encrypted_backup_bk_db_1786268897166907769.enc`).

## 2. INITIATING AN ISOLATED RESTORE TEST MANUALLY
Administrators with `super:admin` permissions can verify backup integrity by restoring it to an isolated database.

```bash
curl -X POST http://localhost:8080/api/v1/oms/system/recover \
  -H "Authorization: Bearer <SUPER_ADMIN_JWT_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"mfa_code": "123456"}'
```

This REST API call programmatically triggers the isolated restoration test within PostgreSQL, returning a full breakdown of recovered users count, ledger balanced state, and tables structure before cleanly destroying the verification sandbox.

## 3. VERIFYING SERVICE HEALTH STATES
Verify that SRE metrics endpoints and liveness/readiness probes are fully functional.

```bash
# Check service liveness
curl http://localhost:8080/health/live

# Check service readiness (verifies Postgres, Redis, Kafka connections)
curl http://localhost:8080/health/ready

# Fetch real-time SRE prometheus metrics
curl http://localhost:8080/metrics
```
