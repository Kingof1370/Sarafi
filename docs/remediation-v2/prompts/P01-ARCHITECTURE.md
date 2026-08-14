# Prompt: P01 — Architecture & Structural Cleanup

## 1. Objective
Perform a comprehensive architectural cleanup of the Velyxora codebase. Remove circular dependencies, dead code, duplicate implementations, and clarify package boundaries.

## 2. Current Problem
- Duplicate balance settlement loops existed across microservices.
- Some modular structures remained in redundant development/test mock directories.

## 3. Root Cause
Lack of single-source-of-truth ownership over financial ledger and balance mutation states.

## 4. Affected Modules
- `packages/common/`
- `apps/wallet-service/`
- `apps/matching-engine/`

## 5. Affected Files
- `apps/wallet-service/main.go`
- `apps/matching-engine/engine/oms_router.go`
- `apps/matching-engine/engine/settlement.go`

## 6. Architectural & Technical Requirements
- Dismantle cross-service balance updates.
- Establish `SettlementEngine` as the sole authority for matched trade balance clearings.
- Maintain single-source-of-truth runtime wiring.

## 7. Migration & Backward Compatibility
- Zero schema modifications. All database states must remain backwards compatible.

## 8. Security & Financial Requirements
- Ensure balance equations always balance on a per-asset basis.

## 9. Tests & Failure Scenarios
- Run all workspace tests to verify no compilation or routing regressions are introduced.

## 10. Acceptance Criteria & Definition of Done
- No compile issues.
- No circular dependencies or dead balance mutation paths remain active.
- Completed with Git commit.
