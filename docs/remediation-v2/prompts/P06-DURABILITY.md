# Prompt: P06 — Durable Settlement

## 1. Objective
Replace volatile in-memory settlement queues with PostgreSQL-backed durable queue outbox persistence.

## 2. Current Problem
- `SettlementEngine` stored pending clearing jobs in volatile RAM slices, risking lost settlements on server crashes.

## 3. Root Cause
Lack of database-backed persistence of pending settlement outbox records.

## 4. Affected Modules
- `apps/matching-engine/`
- `packages/database/`

## 5. Affected Files
- `apps/matching-engine/engine/settlement.go`
- `packages/database/migrations.go`

## 6. Architectural & Technical Requirements
- Add a `payload TEXT` column to `settlements_queue` PostgreSQL tables.
- Serialize and persist jobs when enqueued.
- Pull uncompleted jobs on startup or ProcessQueue cycles.
- Run balance modifications and queue cleanups inside a single, atomic database transaction block.

## 7. Migration & Backward Compatibility
- Migrations must append cleanly to track database schema versioning.

## 8. Security & Financial Requirements
- Rollback the entire transaction if any single settlement leg fails.

## 9. Tests & Failure Scenarios
- Mock server crash: enqueue jobs, execute half, restart engine, and assert that startup recovery completes the remaining half cleanly.

## 10. Acceptance Criteria & Definition of Done
- Database tables back the settlement queue.
- Crash recovery is verified.
- Completed with Git commit.
