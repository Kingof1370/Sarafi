# Velyxora Security Specification

This document details the core security guidelines, database migrations, and structural controls of Velyxora.

## 1. Cryptographic Standard
- **Password Storage**: Argon2/Bcrypt is used for password hashing.
- **Envelope Encryption**: AES-256-GCM is used to encrypt sensitive keys inside PostgreSQL.
- **API Signing**: HMAC-SHA256 is used for request authentication and payload verification.
- **MFA standard**: RFC-6238 TOTP is verified server-side using Base32 credentials.

---

## 2. Durability & Stream Audit Trails
- **Kafka Event Stream**: Security actions and logs are broadcast over the `velyxora-security-events` topic to feed downline surveillance or real-time security alerts.
- **PostgreSQL Persistence**: In parallel with Kafka streaming, critical audit logs are permanently persisted in the append-only `security_audit_logs` table for SOC2/ISO27001 compliance and forensic auditability.
- **Auditing Schema**:
  - `id`: Unique event uuid.
  - `event_type`: "LOGIN_SUCCESS", "LOGIN_FAILURE", "SESSION_CONCURRENCY_REVOCATION", "API_KEY_CREATED", "API_KEY_REVOKED", "BRUTE_FORCE_LOCKOUT", etc.
  - `severity`: "LOW", "MEDIUM", "HIGH", "CRITICAL".
  - `user_id`, `session_id`, `api_key_id`: Relational links.
  - `ip_address`, `user_agent`: Network identifiers.
  - `details`, `metadata`: Payload details.
