# Velyxora Withdrawal Security Manual

This document details the security hardens and permissions controls applied to withdrawals under P0008.

## Security Controls

### 1. RBAC Enforcements
The withdrawal request endpoint `/api/v1/wallet/withdraw` is protected by `RBACMiddleware("wallet:write")`, ensuring only users with active withdrawal privileges can execute transfers.

### 2. Multi-Factor Authentication (MFA)
Sensitive or large-value withdrawals exceeding policy limits require validation.
- **Trigger**: The endpoint checks if user has MFA enabled. If the request exceeds the threshold (e.g., BTC >= 0.1, ETH >= 1.0, SOL/USDT >= 10.0), it strictly validates the TOTP code under header `X-MFA-Code`.
- **Validation**: Verifies against `packages/security/mfa.go` RFC 6238 time-based tokens.

### 3. Atomic Reservation & Row-Level Locking
Withdrawal available balance reservation is done atomically inside a PostgreSQL database transaction utilizing `SELECT FOR UPDATE` to completely prevent race conditions, concurrent overspending, or double withdrawals.
