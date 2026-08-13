# Prompt: P05 — Canonical Settlement

## 1. Objective
Establish a single, unified canonical settlement path inside secure transaction boundaries to prevent double-settlements.

## 2. Current Problem
- Both the synchronous matching loop SettlementEngine and the background wallet-service trades consumer modified balances, risking double-clearing of trades.

## 3. Root Cause
Lack of unified database boundary ownership over matched trade clearance events.

## 4. Affected Modules
- `apps/wallet-service/`
- `apps/matching-engine/`

## 5. Affected Files
- `apps/wallet-service/main.go`
- `apps/matching-engine/engine/settlement.go`

## 6. Architectural & Technical Requirements
- Dismantle duplicate ProcessDoubleEntry loops inside `wallet-service`.
- Centralize trade clearance inside the matching engine's SettlementEngine.

## 7. Migration & Backward Compatibility
- Backward compatible with existing double-entry ledger database schemas.

## 8. Security & Financial Requirements
- Enforce database-level uniqueness constraints so a trade can only be settled exactly once.

## 9. Tests & Failure Scenarios
- Mock duplicate Kafka match events. Assert that the second processing attempt has zero side effects on user balances.

## 10. Acceptance Criteria & Definition of Done
- One execution maps to exactly one financial settlement.
- Duplicate clearings are rejected.
- Completed with Git commit.
