# Velyxora Penetration Testing Report

## Overview
This document covers the methodology, active testing scenarios, and defensive outcomes of penetration testing executed against Velyxora exchange instances.

## Pentest Methodology
Active testing was performed following OWASP Web Security Testing Guide (WSTG) and custom blockchain-specific attack vectors. Tests were run in isolated, sandboxed environments.

## Tested Attack Scenarios & Results

### 1. Privilege Escalation (Vertical/Horizontal)
- **Target**: GET `/wallet/balances`, POST `/wallet/withdraw`
- **Method**: Logged in as a lower-privilege `SUPPORT_AGENT` role (possessing only `wallet:read` and `support:read`), attempted to invoke `/wallet/withdraw` or allocate a mock address.
- **Defensive Behavior**: Server intercepted the request via the server-side RBAC middleware and returned `403 Forbidden` with the error `Forbidden: insufficient permissions`.

### 2. Multi-Factor Authentication Bypass
- **Target**: POST `/wallet/withdraw` for a user with MFA enabled.
- **Method**: Attempted to trigger a withdrawal request without the `X-MFA-Code` header, or providing an expired/incorrect code.
- **Defensive Behavior**: The API Gateway recognized active MFA enrollment, evaluated the missing/incorrect header, and returned `401 Unauthorized` with a description that multi-factor authentication is required. Only on submitting a cryptographically valid RFC 6238 TOTP did the route successfully proceed.

### 3. API Key Signature Manipulation & Nonce Replay
- **Target**: HMAC programmatic endpoints.
- **Method**: Replayed past timestamps and duplicate nonces, and attempted to modify request payloads without regenerating a valid HMAC signature.
- **Defensive Behavior**: API Gateway signature validation middleware enforces strict clock skew (max 300 seconds), saves used nonces in Redis to fail-close replay attempts, and strictly verifies the HMAC-SHA256 checksum, rejecting manipulated bodies immediately.

### 4. Double-Spending and Re-entrancy
- **Target**: Wallet Double-Entry ledger.
- **Method**: Fired high-concurrency requests to transfer and withdraw balances simultaneously.
- **Defensive Behavior**: PostgreSQL row-level locks (`SELECT FOR UPDATE`) serialize mutating operations. The database transaction fails to commit if the reserved amount exceeds available balance.

## Severity Summary

| Threat Scenario | Risk Level | Status | Remediation |
| :--- | :--- | :--- | :--- |
| RBAC Bypass on Wallets | High | Resolved | Added `RBACMiddleware` |
| MFA Bypass on Withdrawals | Critical | Resolved | Added `VerifyMFAProtection` |
| HMAC Signature Tampering | Medium | Resolved | Enforced strict HMAC checks |
| Nonce Replay | Medium | Resolved | Added Redis-backed nonce checks |
