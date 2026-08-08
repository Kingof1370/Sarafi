# Real-Time Transaction Monitoring Engine

Velyxora's transaction monitoring engine actively analyzes both inbound deposit flows and outbound withdrawals.

## Withdrawal Compliance Gates
The withdrawal pipeline applies 11 layers of verification:
`Authentication` -> `RBAC` -> `MFA` -> `Wallet Ownership` -> `Balance` -> `Risk` -> `KYC Status` -> `AML Velocity` -> `Sanctions` -> `Custody` -> `Reconciliation State`

Any single compliance block immediately prevents the transaction from being signed or broadcast.

## Deposit Compliance Holds
Deposits are processed as separate concepts:
- **Blockchain Confirmation**: Validates block heights.
- **Compliance Status**: Screens sender address against watchlists.
Suspicious deposits are set to `held` or `under_review` and never automatically credited to user balances.
