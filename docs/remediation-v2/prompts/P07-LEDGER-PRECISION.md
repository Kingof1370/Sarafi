# Prompt: P07 — Ledger & Financial Precision

## 1. Objective
Eliminate raw floating-point arithmetic (`float64`) usage inside critical financial layers, transiting to precise rational mathematics.

## 2. Current Problem
- Critical financial operations (balances, fees, and settlements) processed using standard float64, risking rounding drift.

## 3. Root Cause
Lack of strict precision math helpers enforcement during clearing calculations.

## 4. Affected Modules
- `packages/common/`
- `apps/matching-engine/`

## 5. Affected Files
- `packages/common/precision.go`
- `apps/matching-engine/engine/settlement.go`
- `packages/common/precision_reconciliation_test.go`

## 6. Architectural & Technical Requirements
- Utilize exact rational arithmetic (`math/big.Rat`) for additions, subtractions, and multiplications.
- Match database `DECIMAL(36,18)` columns precisely.

## 7. Migration & Backward Compatibility
- Backwards compatible with decimal database schemas.

## 8. Security & Financial Requirements
- Zero-drift tolerance: sum of 1000 partial fractional matches must match exact totals with zero dust leak.

## 9. Tests & Failure Scenarios
- Confirm tick/step size validation bounds down to Wei equivalent.

## 10. Acceptance Criteria & Definition of Done
- No unsafe float calculations inside SettlementEngine.
- Rounding rules are verified.
- Completed with Git commit.
