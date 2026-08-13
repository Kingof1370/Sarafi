# PART 02 Remediation: Multi-Symbol Matching Engine & Market Registry

Remediation was fully performed and verified to resolve **Issue 2 (Single-Symbol Matcher Bootstrap)**, **Issue 11 (Engine Integration Mismatch)**, and **Issue 12 (Simulated Liquidity Fallback)**.

## 1. Objective
Enable dynamic, concurrent, and completely isolated multi-symbol trading capabilities across independent matcher price levels and order books, replacing the single-symbol hardcoded BTC-USDT matcher with a robust, thread-safe Market Registry.

## 2. Remediation Details & Implementation
- Created `apps/matching-engine/engine/market_registry.go`: Implemented a thread-safe, lock-guarded `MarketRegistry` storing isolated `Matcher` instances keyed by symbol, allowing dynamic lazy instantiation of trading pairs.
- Modified `apps/matching-engine/engine/oms_router.go`: Added `registry *MarketRegistry` to the single authoritative `OMSRouter` structure. Refactored order placement, stop orders, cancellation, and aggregated level 2 depth retrieval methods to lookup the matching engine instance isolated by order trading symbol, ensuring that no cross-symbol execution leaks can occur.
- Modified `apps/matching-engine/main.go`: Updated the order book change callback to fetch the matcher instance for the specific symbol that changed and publish its L2 depth snapshot to Kafka.
- Modified `apps/backend/oms_handlers.go`: Updated depth and stats queries to dynamically lookup matching status, and compile L2 snapshots based on the specific queried symbol rather than a single default.

## 3. Verification & Testing
Created `apps/matching-engine/engine/market_registry_test.go` and implemented `TestMarketRegistryIsolatedMatching`. This test proves that:
- Market Registry dynamically indexes BTC-USDT and ETH-USDT matching books.
- Orders in isolated symbols (e.g. resting SELL in BTC-USDT crossing with BUY in ETH-USDT) do not match across distinct symbols, guaranteeing 100% complete symbol isolation.
All unit tests compile and execute successfully.
