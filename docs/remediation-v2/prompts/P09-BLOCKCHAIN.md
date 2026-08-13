# Prompt: P09 — Blockchain Deposit & Scanner

## 1. Objective
Harden the on-chain blockchain transaction scanner and confirmation tracking loop inside `wallet-service`.

## 2. Current Problem
- Scan and confirmation tracking relied heavily on ETH, with thin simulation fallbacks for other networks.

## 3. Root Cause
Incomplete adapter integration inside the block-scanning daemon.

## 4. Affected Modules
- `apps/wallet-service/`
- `packages/common/`

## 5. Affected Files
- `apps/wallet-service/wallet_worker.go`
- `packages/common/blockchain.go`

## 6. Architectural & Technical Requirements
- Ensure proper blockchain scanner states tracking inside `blockchain_scanner_states` tables to avoid duplicate scans or skipped block heights.
- Enforce secure confirmation depth (default 12 for ETH/ERC20) before atomically crediting deposits to users' ledger accounts.

## 7. Migration & Backward Compatibility
- Backwards compatible with standard postgres scan tables.

## 8. Security & Financial Requirements
- Rollback credit and ledger updates if database connection pool encounters issues during scanning loops.

## 9. Tests & Failure Scenarios
- Verify that simulated blocks increment and correctly trigger pending to confirmed state transitions on-chain.

## 10. Acceptance Criteria & Definition of Done
- Block scanner correctly polls, tracks scan height, and confirms on-chain transfers.
- Completed with Git commit.
