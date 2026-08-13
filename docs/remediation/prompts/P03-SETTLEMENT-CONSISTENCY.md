# Prompt: Settlement Consistency & Double Settlement Prevention (PART-03)

## Objective
Remediet the architectural risk of double-settlement. Currently, both the synchronous `SettlementEngine` (invoked after matching within the `OMSRouter` cycle) and the background worker within `wallet-service` (listening on the `velyxora-trades` Kafka topic) independently apply balance settlements for matched executions. This duplication represents an unacceptable financial risk. Establish a single canonical settlement path with strict idempotency and transaction boundaries.

## Scope
- Analyze and refactor balance mutations across `apps/matching-engine/engine/settlement.go` and `apps/wallet-service/wallet_worker.go`.
- Modify Kafka event consumer topologies.
- Enforce strict transaction boundaries and duplicate detection mechanisms in database storage layers.

## Requirements & Specifications

### 1. Unified Canonical Settlement Path
- Consolidate all trade balance mutations into **one authoritative service**.
- Define the `SettlementEngine` inside the matching engine boundary or a dedicated settlement consumer as the **single source of truth** for applying trade-related ledger entries and balance adjustments.
- Eliminate the independent and duplicated buyer/seller credit/debit loops within the wallet service's Kafka consumer. The wallet service must restrict its activities to deposits, withdrawals, and on-chain syncing; it must not independently clear matched trades unless designed as the *only* consumer of a transactionally safe settlement stream.

### 2. Strict Idempotency & Unique Identifiers
- Every match execution produces a deterministic, sequential trade ID. This ID must serve as the canonical `settlement_id` or `ledger_tx_id`.
- Enforce database-level uniqueness constraints. Balance updates and double-entry ledger postings must be scoped under a unique transaction execution key.
- Apply `ON CONFLICT DO NOTHING` or PostgreSQL SELECT FOR UPDATE locks to detect and discard duplicate settlement requests instantly.

### 3. Transaction Boundaries
- All balance adjustments (debit buyer quote, debit seller base, credit buyer base, credit seller quote, platform fees, and ledger log insertions) must execute inside a single atomic database transaction block. If any single leg fails, the entire settlement must rollback.

## Testing Strategy
- Create integration and failure tests:
  - Simulate the arrival of duplicate execution events or Kafka retries for the same trade. Verify that the second settlement attempt is rejected or ignored with zero state mutations.
  - Test transaction atomicity: intentionally trigger a database constraint violation on one leg of a settlement (e.g., credit user balance) and assert that the entire ledger entry and matching balance debits rollback fully.
  - Verify that dual paths are completely dismantled and only one service mutates database balances under matched execution events.
