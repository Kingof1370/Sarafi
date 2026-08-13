# Prompt: Durable Settlement Queue & Outbox Recovery (PART-04)

## Objective
Remediet the non-durable in-memory settlement queue. Currently, `SettlementEngine` stores pending jobs inside a RAM-based slice `queue []*SettlementJob`. If the server crashes, all pending settlements in RAM are permanently lost, causing a critical discrepancy where Orders are marked FILLED and Trades are CREATED, but actual financial settlements are lost. Design a persistent, transaction-safe, and crash-resilient settlement architecture.

## Scope
- Modify `apps/matching-engine/engine/settlement.go`
- Utilize PostgreSQL table-backed persistent state tracking (`settlements_queue` and `settlements_history`).
- Implement crash-recovery, process startup reconstruction, and outbox transactional patterns.

## Requirements & Specifications

### 1. Database-Backed Persistent Queue
- Transition the `queue` mechanism inside the `SettlementEngine` from a pure in-memory structure to a transactional persistent backing.
- When an order match execution is queued, persist the settlement job within the `settlements_queue` table inside the *same* database transaction that writes/updates orders and trades (Transactional Outbox pattern).
- Change the asynchronous `ProcessQueue` loop to query pending jobs from the database using row-level locking (e.g., `SELECT ... FOR UPDATE SKIP LOCKED`), ensuring horizontal scaling safety across multiple active worker threads.

### 2. Idempotent State Transitions
- Define explicit, persistent states: `PENDING`, `PROCESSING`, `COMPLETED`, `FAILED`, and `RETRYING`.
- Once a settlement job finishes successfully, atomically move the record from `settlements_queue` to `settlements_history` or mark its status as `COMPLETED` transactionally.
- Store detailed error logs and retry metrics directly on the persisted job table to assist audit tracking.

### 3. Crash Recovery and Startup Reconstruction
- During microservice bootstrap, query the persistent queue for any unfinished jobs (status `PENDING` or `PROCESSING`) and enqueue them for immediate execution.
- Ensure that the entire startup and recovery path is completely safe against replay or concurrent duplicate executions.

## Testing Strategy
- Create crash-resilience and mock failure tests:
  - Simulate a process crash: insert multiple jobs into the persistent queue, process half of them, mock a crash (by resetting the engine), and trigger startup recovery. Verify that the remaining half are fully processed, and already-completed jobs are not processed twice.
  - Verify concurrent execution safety: spawn multiple concurrent settlement worker routines and assert that each row is claimed and processed exactly once with zero collisions or race conditions.
