# Velyxora Incident Response Plan

## Overview
This plan describes the processes, responsibilities, and automated mechanics designed to detect, contain, and recover from security incidents within the Velyxora ecosystem.

## SRE and Alerting Integration
Velyxora employs automated background monitoring via the SRE alerting manager:
1. **Detection**: Background monitors continuously verify PostgreSQL, Redis, and Kafka cluster health status.
2. **Automated Incident Creation**: If any critical dependency goes offline, a real-time SRE Incident with status `OPEN` and severity `CRITICAL` is registered in Postgres (`sre_incidents` and `sre_alerts`).
3. **Telemetry Notification**: Alert events increment Prometheus counters and trigger log records.
4. **Self-Healing / Mitigation**: Once the health monitor detects the service is back online, it marks the incident status as `RESOLVED`.

## Escalation Path & Roles
- **Tier 1 (Support Agent)**: Initial detection of abnormalities, customer reports, or open alerts.
- **Tier 2 (Compliance/Risk Officer)**: Reviews suspicious AML activity or sanctions screens. Possesses authority to block users and halt markets.
- **Tier 3 (Security Officer / Admin)**: Investigates critical vulnerability exploitation, database compromises, and executes emergency halts.
- **Tier 4 (Super Admin)**: Authorized to execute disaster recovery procedures.

## Recovery Procedures
In the event of a catastrophic compromise or data corruption:
1. **Halt Trading**: Call `/oms/risk/halt` to suspend order execution.
2. **Execute Secure Backup**: Automated or manual encrypted backups are triggered using AES-256-GCM.
3. **Transactional Database Restoration**: Replay journals and restore clean database states inside isolated instances, validating ledger math before routing traffic back.
