# VELYXORA EXCHANGE - ENTERPRISE BACKUP POLICY

## 1. PURPOSE & SCOPE
This document outlines the authoritative rules and schedules governing backup retention, encrypted transport, and validation for Velyxora's production resources, protecting institutional double-entry ledgers and cryptographic key metadata from critical disaster scenarios.

## 2. RECOVERY TARGETS (RPO & RTO)

| System / Dependency | Authoritative Source | Recovery Point Objective (RPO) | Recovery Time Objective (RTO) |
| :--- | :--- | :--- | :--- |
| **PostgreSQL Database** | Persistent Volume | Near-Zero (&lt; 1s) | &lt; 10s (Auto Failover) |
| **Redis Cache** | Ephemeral State | Ephemeral State (N/A) | &lt; 60s (Auto-rebuild) |
| **Matching Engine** | Disk Sequence Journal | Near-Zero (&lt; 500ms) | &lt; 1s (Journal Replay) |
| **Wallet Service** | Double-Entry Ledger | Near-Zero (&lt; 100ms) | &lt; 10s (Single-writer loop) |

## 3. BACKUP SCHEDULES & ENCRYPTION
- **Daily Full Backups:** Completed every 24 hours at 01:00 UTC using non-blocking session-isolated `pg_dump` structures.
- **Dedicated Encryption:** Backup archives are dynamically encrypted utilizing AES-256-GCM symmetric block ciphers. The backup encryption secret (`BACKUP_ENCRYPTION_KEY`) is securely supplied through the secrets/configuration infrastructure. Operational JWT/API keys are strictly segregated and not reused.
- **WAL-Archiving:** Write-Ahead Logs (WAL) are streamed continuously to redundant persistent object storage to support point-in-time recovery capabilities.

## 4. SECURITY & COMPLIANCE CONTROLS
- **Zero Raw Secret Exposure:** Backups do not expose unhashed passwords, private keys, JWT secrets, or seed phrases.
- **Fail-Closed Permissions:** Dedicated OS/Kubernetes service accounts restrict execution permissions to system administrators with `system:admin` or `super:admin` RBAC roles.
- **Continuous Validation:** Every backup runs through an automated isolated sandbox restoration verification within 1 hour of generation to guarantee valid readability.
