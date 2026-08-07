# Session Security & Rotation Specification

This document details the state-of-the-art secure session lifecycle, concurrent-session restrictions, and refresh-token rotation (RTR) of Velyxora.

## 1. Active Session Management
Active sessions are persisted in the authoritative `user_sessions` PostgreSQL table and cached in Redis.
Each session stores:
- Unique session ID (`id`)
- Client identifier (`user_id`)
- Metadata: `device_id`, `ip_address`, `user_agent`
- Status: `is_revoked`, `expires_at`, `created_at`, `updated_at`

---

## 2. Concurrent Session Controls
- **Default Limit**: Max **5 active sessions** per user account.
- **Enforcement Flow**: Upon successful login, if the active session count is >= 5, the gateway automatically finds the user's oldest active session and marks it as `is_revoked = TRUE` inside PostgreSQL (and caches the revocation in Redis). A security audit event `SESSION_CONCURRENCY_REVOCATION` is logged.

---

## 3. Refresh Token Rotation (RTR) & Reuse Detection
- **Rotation**: On token refresh, the client presents their old refresh token. The server verifies its authenticity, rotates it by generating a new access/refresh pair, records the old refresh token as rotated in Redis with a 24h TTL, and updates the `refresh_token_hash` in `user_sessions`.
- **Reuse/Theft Detection**: If a previously rotated or revoked refresh token is presented, the system treats it as a credential-compromise breach attempt.
  - **Breach Mitigation**: Instantly terminates **all active sessions** and revokes all refresh tokens belonging to that user ID.
  - **Audit Logging**: Emits a `CRITICAL` severity security event over Kafka and persists it in PostgreSQL.
  - **Remediation**: The user must perform a fresh login and verify their MFA code to establish a new trusted session.
