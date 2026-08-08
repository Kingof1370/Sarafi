# Velyxora Custody Manual

This document details the institutional custody and vault governance module built under P0008.

## Segregated Vaults
We support institutional segregation profiles to protect exchange liquidity:
- **HOT**: Hot wallets for immediate withdrawal processing.
- **WARM**: Warm storage for buffer transfers.
- **COLD**: Offline storage for client assets.
- **TREASURY**: Operational corporate capital pools.

## Governance & Dual Approval
The system enforces strict administrative Dual Approvals (4-eyes principle) for withdrawals exceeding configured policy limits.
- **First Signature**: Initiated by first authorized administrator.
- **Second Signature**: Must be signed by a *different* authorized administrator (`SubmitApproval` validation).
- **Emergency Controls**: A global freeze control `SetGlobalFreeze` is supported to instantly freeze all transfers in case of on-chain anomalies.
