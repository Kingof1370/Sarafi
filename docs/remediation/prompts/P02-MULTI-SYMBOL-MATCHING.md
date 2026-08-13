# Prompt: Multi-Symbol Matching Engine & Market Registry (PART-02)

## Objective
Refactor the Velyxora Matching Engine to support high-performance, concurrent, and isolated multi-symbol matches. Replace the hardcoded `NewMatcher("BTC-USDT")` bootstrap model with a dynamic `MarketRegistry` that spawns, routes, sequences, and manages independent order books, risk boundaries, and matching loop routines for all registered asset pairs (e.g., `BTC-USDT`, `ETH-USDT`, `SOL-USDT`, etc.).

## Scope
- Modify `apps/matching-engine/engine/matcher.go`
- Modify `apps/matching-engine/engine/oms_router.go`
- Create `apps/matching-engine/engine/market_registry.go`
- Update `apps/matching-engine/main.go` bootstrap logic

## Requirements & Specifications

### 1. Market Registry
Implement a thread-safe `MarketRegistry` that maintains:
- A thread-safe map of active `Matcher` instances keyed by trading symbol (e.g., `BTC-USDT`).
- Dynamic addition, state tracking, and isolation of trading pairs.
- Graceful degradation if order requests are directed to unregistered symbols.

### 2. Complete Symbol Isolation
- Ensure that order processing is fully isolated by symbol.
- An incoming order for symbol `ETH-USDT` must only be processed against the `ETH-USDT` matcher. It must be impossible to mistakenly route or execute `ETH-USDT` orders within a `BTC-USDT` matching loop.
- Independent, isolated sequence numbers and order books for each symbol.

### 3. OMSRouter Refactoring
- Refactor `OMSRouter` to interact with the `MarketRegistry` rather than possessing a single `Matcher` instance.
- Update `OMSRouter` dispatch routines to lookup the correct `Matcher` from the registry, execute matching loops, track market-specific states, and broadcast isolated symbol events.

### 4. Determinism & Performance Preservation
- Retain high-precision FIFO price-time priority rules.
- Retain matching determinism using monotonic trade IDs rather than non-deterministic system clock timestamps.

## Testing Strategy
- Create `apps/matching-engine/engine/market_registry_test.go` and write tests to:
  - Verify registry insertion, lookup, and safe concurrency under high-load parallel lookups.
  - Test routing isolated symbols: ensure that placing an order for `ETH-USDT` does not mutate or trigger fills inside the `BTC-USDT` book.
  - Verify that sequence numbers are kept entirely isolated and monotonic within their respective symbolic boundaries.
  - Verify graceful rejection of orders with unsupported symbols.
