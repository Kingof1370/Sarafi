# Prompt: P03 — Multi-Market Matching

## 1. Objective
Enable parallel, concurrent, and isolated order matching loops across multiple trading symbols using a dynamic Market Registry.

## 2. Current Problem
- Engine bootstrap was hardcoded to a single `NewMatcher("BTC-USDT")` instance, preventing multi-symbol isolation.

## 3. Root Cause
Coupled OMS matching paths lacking dynamic registry lookups.

## 4. Affected Modules
- `apps/matching-engine/`
- `apps/backend/`

## 5. Affected Files
- `apps/matching-engine/engine/oms_router.go`
- `apps/matching-engine/engine/market_registry.go`
- `apps/matching-engine/main.go`
- `apps/backend/oms_handlers.go`

## 6. Architectural & Technical Requirements
- Build thread-safe `MarketRegistry`.
- Route incoming order placement requests to isolated book matchers based on symbol.
- Maintain independent sequence counters and order events per symbol.

## 7. Migration & Backward Compatibility
- Backwards compatible with single BTC-USDT default parameters in local tests.

## 8. Security & Financial Requirements
- Ensure orders for different markets (e.g. ETH-USDT) can never match or clear against BTC-USDT price levels.

## 9. Tests & Failure Scenarios
- Validate registry parallel access.
- Test symbol isolation: resting SELL in BTC-USDT crossing with BUY in ETH-USDT must never execute trades.

## 10. Acceptance Criteria & Definition of Done
- Market registry returns isolated books.
- Trades do not cross-match different symbols.
- Completed with Git commit.
