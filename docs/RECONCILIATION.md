# Velyxora Reconciliation Manual

This document details the multi-layered reconciliation engine designed under P0008.

## Five-Layer Reconciliation
The reconciliation system audits the entire ledger, ensuring no balance discrepancies or double-spend cracks can exist:

- **Layer 1: Blockchain state**: Totals simulated address balances on-chain.
- **Layer 2: DB records**: Sums all completed deposits and withdrawals.
- **Layer 3: Wallet balances**: Sums all user total balances.
- **Layer 4: Double-entry ledger**: Verifies that total debits balance credits exactly.
- **Layer 5: Custody/vault state**: Checks segregated Hot, Warm, Cold, and Treasury positions.

## Scheduling and Admin APIs
The reconciliation runs as a background thread on a configurable interval (every 10 seconds), and can also be triggered manually by administrators using the `POST /api/v1/reconciliation/run` API. Discrepancies are logged as issue records with associated severities (`WARNING` / `CRITICAL`) and can be queried via `/issues`.
