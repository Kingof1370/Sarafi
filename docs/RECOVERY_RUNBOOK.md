# Disaster Recovery & System Failover Operational Runbook

A step-by-step guide for administrators during system failures.

## 1. Entering Emergency Mode

In case of a serious data corruption anomaly or active platform exploit, immediately toggle the platform mode:
- Method: Invoke `POST /api/v1/system/emergency/freeze` or update `dr_system_status` operational mode to `EMERGENCY`.
- Effect: Immediately blocks order placement, deposits, and withdrawal processing across all services.

## 2. Restoring to a Clean Consistent State

Once the root cause is resolved:
1. Locate the latest clean backup ID from `GET /api/v1/system/backups`.
2. Validate backup integrity:
   `POST /api/v1/system/recovery/validate` with `{"backup_id": "bak_xxx", "execute": false}`.
3. Perform the authorized restore:
   `POST /api/v1/system/recovery/validate` with `{"backup_id": "bak_xxx", "execute": true}` (Requires MFA code).
4. Monitor logs to confirm post-restore health checks pass successfully and the system transitions back to `NORMAL`.
