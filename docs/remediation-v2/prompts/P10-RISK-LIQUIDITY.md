# Prompt: P10 — Risk Calculations & Liquidity

## 1. Objective
Ensure deterministic risk computations, precise margin evaluations, and safe bankruptcy liquidations inside the Perpetual Futures engine.

## 2. Current Problem
- Margin, leverage, and liquidation prices could encounter float64 rounding discrepancies or mock liquidity crossings.

## 3. Root Cause
Incomplete testing of isolated margin ratios and liquidation cascades.

## 4. Affected Modules
- `apps/matching-engine/`

## 5. Affected Files
- `apps/matching-engine/engine/risk.go`
- `apps/matching-engine/engine/position.go`

## 6. Architectural & Technical Requirements
- Enforce strict maintenance margin checks prior to any trade execution or position modification.
- Safely route bankrupt liquidation orders to insurance fund reserves.

## 7. Migration & Backward Compatibility
- Compatible with existing `user_positions` and `futures_insurance_funds` PostgreSQL schemas.

## 8. Security & Financial Requirements
- Trigger ADL (Auto-Deleveraging) immediately if insurance fund balance dips below zero.

## 9. Tests & Failure Scenarios
- Mock position flip: test long to short flip, leverage limits checks, and bankruptcy liquidations.

## 10. Acceptance Criteria & Definition of Done
- Futures positions managers correctly compute margin ratios thread-safely.
- Completed with Git commit.
